package worker

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/docker/docker/api/types"
)

func TestWorker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Worker Suite")
}

var _ = Describe("container snapshot worker", func() {
	var (
		ctx    = context.Background()
		worker = Worker{
			client: &mockDockerClient{},
			auths:  make(mergedDockerAuth),
		}
		options = SnapshotOptions{
			Container: "container-id",
			Image:     "image-name",
		}
	)

	Context("when docker steps all good", func() {
		It("should succeed", func() {
			Expect(worker.TakeSnapshot(ctx, &options)).Should(Succeed())
		})
	})

	Context("when image name is invalid", func() {
		opts := SnapshotOptions{
			Container: "container-id",
			Image:     "invalid image",
		}

		It("should fail", func() {
			Expect(worker.TakeSnapshot(ctx, &opts)).Should(MatchError(ErrInvalidImage))
		})
	})

	Context("when container commit fails", func() {
		BeforeEach(func() {
			worker.client = &mockDockerClient{
				badCommit: true,
			}
		})

		It("should fail", func() {
			Expect(worker.TakeSnapshot(ctx, &options)).Should(MatchError(ErrCommit))
		})
	})

	Context("when docker push fails", func() {
		BeforeEach(func() {
			worker.client = &mockDockerClient{
				badPush: true,
			}
		})

		It("should fail", func() {
			Expect(worker.TakeSnapshot(ctx, &options)).Should(MatchError(ErrPush))
		})
	})
})

type mockDockerClient struct {
	badCommit bool
	badPush   bool
}

func (c *mockDockerClient) ContainerCommit(ctx context.Context, container string, options types.ContainerCommitOptions) (types.IDResponse, error) {
	if c.badCommit {
		return types.IDResponse{}, errors.New("can not do container commit")
	}

	return types.IDResponse{ID: "mock id"}, nil
}

func (c *mockDockerClient) ImagePush(ctx context.Context, ref string, options types.ImagePushOptions) (io.ReadCloser, error) {
	if c.badPush {
		return nil, errors.New("can not do image push")
	}

	return ioutil.NopCloser(strings.NewReader("{}")), nil
}
