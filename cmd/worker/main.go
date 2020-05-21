package worker

import (
	"context"
	"os"
	"time"

	flag "github.com/spf13/pflag"
	"github.com/supremind/container-snapshot/pkg/worker"
	"github.com/supremind/container-snapshot/version"
	"github.com/supremind/pkg/shutdown"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	defaultConfigRoot = "/config"
	defaultTimeout    = 30 * time.Minute

	envTimeout = "TIMEOUT"
)

func main() {
	log := logf.Log.WithName("container snapshot worker").WithValues("version", version.Version)

	go func() {
		e := run()
		if e != nil {
			log.Error(e, "snapshot worker failed")
			os.Exit(1)
		}
	}()

	shutdown.BornToDie(context.Background())
	log.Info("exit")
}

func run() error {
	opt := &worker.SnapshotOptions{}
	flag.StringVarP(&opt.Container, "container", "c", "", "docker if of the container going to take a snapshot")
	flag.StringVarP(&opt.Image, "image", "i", "", "name of the snapshot image")
	flag.StringVar(&opt.Author, "author", "", "snapshot author")
	flag.StringVar(&opt.Comment, "comment", "", "comment")

	var configRoot string
	flag.StringVar(&configRoot, "config", defaultConfigRoot, "root path of docker config files")

	flag.Parse()

	c, e := worker.NewDockerClient(configRoot)
	if e != nil {
		return e
	}

	timeout := defaultTimeout
	if t := os.Getenv(envTimeout); t != "" {
		d, _ := time.ParseDuration(t)
		if d > 0 {
			timeout = d
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return c.TakeSnapshot(ctx, opt)
}
