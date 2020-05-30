package worker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/supremind/container-snapshot/version"
	corev1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("container snapshot worker").WithValues("version", version.Version)

type Worker struct {
	client DockerClient
	auths  mergedDockerAuth
}

// DockerClient is a subset of docker CommonAPIClient, to make the worker interface simpler
type DockerClient interface {
	ContainerCommit(ctx context.Context, container string, options types.ContainerCommitOptions) (types.IDResponse, error)
	ImagePush(ctx context.Context, ref string, options types.ImagePushOptions) (io.ReadCloser, error)
}

func New(cli DockerClient, authpath string) (*Worker, error) {
	auths, e := loadDockerAuths(authpath)
	if e != nil {
		return nil, fmt.Errorf("load docker auths: %w", e)
	}

	return &Worker{client: cli, auths: auths}, nil
}

type SnapshotOptions struct {
	// docker registry auth header, from imagePushSecret
	// see: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/#inspecting-the-secret-regcred
	Container string `json:"container,omitempty"`
	Image     string `json:"image,omitempty"` // image full name: host/path/image:tag
	Author    string `json:"author,omitempty"`
	Comment   string `json:"comment,omitempty"`
}

func (c *Worker) TakeSnapshot(ctx context.Context, opt *SnapshotOptions) error {
	log = log.WithValues("container", opt.Container, "image", opt.Image, "author", opt.Author)
	log.Info("taking snapshot")

	ref, e := reference.ParseNormalizedNamed(opt.Image)
	if e != nil {
		log.Error(e, "parse image name failed")
		return errInvalidImage(opt.Image)
	}

	id, e := c.client.ContainerCommit(ctx, opt.Container, types.ContainerCommitOptions{
		Reference: ref.String(),
		Author:    opt.Author,
		Comment:   opt.Comment,
		Config: &container.Config{
			Image: opt.Image,
		},
	})
	if e != nil {
		log.Error(e, "container commit failed")
		return errCommit(opt.Container)
	}
	log.WithValues("id", id.ID).Info("container committed")

	for _, auth := range c.auths[reference.Domain(ref)] {
		e = c.push(ctx, &auth, ref)
		if e == nil {
			goto succeed
		}
	}
	e = c.push(ctx, nil, ref)
	if e != nil {
		log.Error(e, "push image")
		return errPush(ref.Name())
	}

succeed:
	log.Info("image push succeed")
	return nil
}

func (c *Worker) push(ctx context.Context, auth *types.AuthConfig, ref reference.Reference) error {
	image := reference.FamiliarString(ref)

	var coded string
	if auth != nil {
		var e error
		coded, e = formatAuth(*auth)
		if e != nil {
			return e
		}
	}

	resp, e := c.client.ImagePush(ctx, image, types.ImagePushOptions{
		RegistryAuth: coded,
	})
	if e != nil {
		return e
	}

	return c.printPushMessage(resp)
}

func (c *Worker) printPushMessage(r io.ReadCloser) error {
	dec := json.NewDecoder(r)

	for {
		var jm jsonmessage.JSONMessage
		if e := dec.Decode(&jm); e != nil {
			if e == io.EOF {
				return nil
			}
			return e
		}

		log.Info(jm.ProgressMessage, "id", jm.ID, "status", jm.Status, "stream", jm.Stream, "from", jm.From, "error message", jm.ErrorMessage)
	}
}

func formatAuth(auth types.AuthConfig) (string, error) {
	buf, e := json.Marshal(auth)
	if e != nil {
		return "", fmt.Errorf("marshal auth: %w", e)
	}

	return base64.URLEncoding.EncodeToString(buf), nil
}

type mergedDockerAuth map[string][]types.AuthConfig
type dockerAuth map[string]types.AuthConfig

func loadDockerAuths(configRoot string) (mergedDockerAuth, error) {
	merged := make(mergedDockerAuth)

	filepath.Walk(configRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("walking docker config files in %s: %w", path, err)
		}

		if info.IsDir() {
			return nil
		}

		f, e := os.Open(path)
		if e != nil {
			return fmt.Errorf("open docker config file %s: %w", path, e)
		}
		defer f.Close()

		switch filepath.Base(path) {
		case corev1.DockerConfigKey:
			content := make(dockerAuth)
			if e := json.NewDecoder(f).Decode(&content); e != nil {
				return fmt.Errorf("unmarshal docker config %s: %w", path, e)
			}
			merged.merge(content)

		case corev1.DockerConfigJsonKey:
			content := struct {
				Auths dockerAuth `json:"auths"`
			}{}
			if e := json.NewDecoder(f).Decode(&content); e != nil {
				return fmt.Errorf("unmarshal docker config %s: %w", path, e)
			}
			merged.merge(content.Auths)
		}

		return nil
	})

	return merged, nil
}

func (m mergedDockerAuth) merge(singleAuth dockerAuth) {
	for reg, auth := range singleAuth {
		if _, ok := m[reg]; !ok {
			m[reg] = make([]types.AuthConfig, 0, 1)
		}
		m[reg] = append(m[reg], auth)
	}
}
