package main

import (
	"flag"
	"io/ioutil"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/cloudfoundry-incubator/cf-lager"
	"github.com/cloudfoundry-incubator/route-registrar/config"
	"github.com/cloudfoundry-incubator/route-registrar/healthchecker"
	"github.com/cloudfoundry-incubator/route-registrar/registrar"
	"github.com/pivotal-cf-experimental/service-config"
	"github.com/pivotal-golang/lager"
)

func main() {
	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	pidfile := flags.String("pidfile", "", "Path to pid file")
	cf_lager.AddFlags(flags)

	serviceConfig := service_config.New()
	serviceConfig.AddFlags(flags)
	flags.Set("configPath", "registrar_settings.yml")

	flags.Parse(os.Args[1:])

	logger, _ := cf_lager.New("Route Registrar")

	logger.Info("Route Registrar started")

	var registrarConfig config.Config
	err := serviceConfig.Read(&registrarConfig)
	if err != nil {
		logger.Fatal("error parsing file: %s\n", err)
	}

	r := registrar.NewRegistrar(registrarConfig, logger)
	//add health check handler
	checker := healthchecker.InitHealthChecker(registrarConfig, logger)
	if checker != nil {
		r.AddHealthCheckHandler(checker)
	}

	if *pidfile != "" {
		pid := strconv.Itoa(os.Getpid())
		err := ioutil.WriteFile(*pidfile, []byte(pid), 0644)
		if err != nil {
			logger.Fatal(
				"error writing pid to pidfile",
				err,
				lager.Data{
					"pid":     pid,
					"pidfile": *pidfile},
			)
		}
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan struct{}, 1)
	go func() {
		r.Run(signals)
		close(done)
	}()
	<-done
	os.Exit(1)
}
