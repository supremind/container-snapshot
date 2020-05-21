package worker

import (
	"context"
	"errors"
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

	envTimeout   = "TIMEOUT"
	envNamespace = "NAMESPACE"
)

var log = logf.Log.WithName("container snapshot worker").WithValues("version", version.Version)

func main() {
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
	flag.StringVarP(&opt.Container, "container", "c", "", "required, docker name of the container going to take a snapshot")
	flag.StringVarP(&opt.Image, "image", "i", "", "required, name of the snapshot image")
	flag.StringVar(&opt.Author, "author", "", "snapshot author")
	flag.StringVar(&opt.Comment, "comment", "", "comment")

	var configRoot string
	var snapshot string
	flag.StringVar(&configRoot, "config", defaultConfigRoot, "root path of docker config files, default is /config")
	flag.StringVar(&snapshot, "snapshot", "", "required, snapshot name")
	flag.Parse()

	namespace := os.Getenv(envNamespace)
	if opt.Container == "" || opt.Image == "" || snapshot == "" || namespace == "" {
		return errors.New("invalid arguments")
	}

	log = log.WithValues("namespace", namespace, "snapshot", snapshot, "container", opt.Container, "image", opt.Image)

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

	e = c.TakeSnapshot(ctx, opt)
	if e != nil {
		log.Error(e, "take snapshot failed")
		c, e := worker.NewController(namespace, snapshot)
		if e != nil {
			log.Error(e, "create snapshot controller")
			return e
		}

		if e = c.UpdateCondition(ctx, snapshot, e); e != nil {
			log.Error(e, "update snapshot condition")
			return e
		}
	}

	return e
}
