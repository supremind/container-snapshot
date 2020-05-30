package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/docker/docker/client"
	"github.com/operator-framework/operator-sdk/pkg/log/zap"
	"github.com/spf13/pflag"
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
	// Add the zap logger flag set to the CLI. The flag set must
	// be added before calling pflag.Parse().
	pflag.CommandLine.AddFlagSet(zap.FlagSet())

	// Add flags registered by imported packages (e.g. glog and
	// controller-runtime)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

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
	pflag.StringVarP(&opt.Container, "container", "c", "", "required, docker name of the container going to take a snapshot")
	pflag.StringVarP(&opt.Image, "image", "i", "", "required, name of the snapshot image")
	pflag.StringVar(&opt.Author, "author", "", "snapshot author")
	pflag.StringVar(&opt.Comment, "comment", "", "comment")

	var configRoot string
	var snapshot string
	pflag.StringVar(&configRoot, "config", defaultConfigRoot, "root path of docker config files, default is /config")
	pflag.StringVar(&snapshot, "snapshot", "", "required, snapshot name")
	pflag.Parse()

	namespace := os.Getenv(envNamespace)
	if opt.Container == "" || opt.Image == "" || snapshot == "" || namespace == "" {
		return errors.New("invalid arguments")
	}

	cli, e := client.NewEnvClient()
	if e != nil {
		return fmt.Errorf("create docker client: %w", e)
	}

	log = log.WithValues("namespace", namespace, "snapshot", snapshot, "container", opt.Container, "image", opt.Image)

	c, e := worker.New(cli, configRoot)
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
