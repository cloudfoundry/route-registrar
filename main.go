package main

import (
	"flag"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/cloudfoundry-incubator/cf-lager"
	"github.com/cloudfoundry-incubator/route-registrar/config"
	. "github.com/cloudfoundry-incubator/route-registrar/healthchecker"
	. "github.com/cloudfoundry-incubator/route-registrar/registrar"
	"github.com/pivotal-golang/lager"
)

func main() {
	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	pidfile := flags.String("pidfile", "", "Path to pid file")
	cf_lager.AddFlags(flags)
	flags.Parse(os.Args[1:])

	logger, _ := cf_lager.New("Route Registrar")

	logger.Info("Route Registrar started")

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

	config, err := config.InitConfigFromFile("registrar_settings.yml")
	if err != nil {
		logger.Fatal("error parsing file: %s\n", err)
	}
	registrar := NewRegistrar(config, logger)
	//add health check handler
	checker := InitHealthChecker(config, logger)
	if checker != nil {
		registrar.AddHealthCheckHandler(checker)
	}
	registrar.RegisterRoutes()
	os.Exit(1)
}
