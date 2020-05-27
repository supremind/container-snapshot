package worker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	flag "github.com/spf13/pflag"
	"github.com/supremind/container-snapshot/pkg/constants"
	"github.com/supremind/container-snapshot/pkg/worker"
	"github.com/supremind/container-snapshot/version"
	"github.com/supremind/pkg/shutdown"
	corev1 "k8s.io/api/core/v1"
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

		if e := writeTerminationLog(e); e != nil {
			log.Error(e, "write termination log failed")
		}

		var code int32 = 127
		if errors.Is(e, worker.ErrInvalidImage) {
			code = constants.ExitCodeInvalidImage
		} else if errors.Is(e, worker.ErrCommit) {
			code = constants.ExitCodeDockerCommit
		} else if errors.Is(e, worker.ErrPush) {
			code = constants.ExitCodeDockerPush
		}
		os.Exit(int(code))
	}

	return e
}

func writeTerminationLog(e error) error {
	f, e := os.Create(corev1.TerminationMessagePathDefault)
	if e != nil {
		return fmt.Errorf("open ternination message file: %w", e)
	}
	defer f.Close()

	_, e = f.WriteString(e.Error())
	return e
}
