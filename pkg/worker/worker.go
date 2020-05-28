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
	"github.com/docker/docker/pkg/term"
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

	auths := c.auths[reference.Domain(ref)]
	image := reference.FamiliarString(ref)
	authIdx := 0
	resp, e := c.client.ImagePush(ctx, image, types.ImagePushOptions{
		// PrivilegeFunc may be called more than once to retry on authentication errors
		PrivilegeFunc: func() (string, error) {
			if authIdx < len(auths) {
				authIdx++
				auth, e := formatAuth(auths[authIdx])
				if e != nil {
					return "", e
				}
				return auth, nil
			}
			return "", io.EOF
		},
	})
	if e != nil {
		log.Error(e, "image push failed")
		return errPush(image)
	}

	fd, isTerm := term.GetFdInfo(resp)
	if e := jsonmessage.DisplayJSONMessagesStream(resp, os.Stdout, fd, isTerm, nil); e != nil {
		log.Error(e, "display push output")
		return errPush("display push output")
	}
	log.Info("push complete")

	return nil
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
