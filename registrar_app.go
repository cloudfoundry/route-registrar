package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"strconv"

	"github.com/cloudfoundry-incubator/route-registrar/config"
	. "github.com/cloudfoundry-incubator/route-registrar/healthchecker"
	. "github.com/cloudfoundry-incubator/route-registrar/registrar"
)

var (
	pidfile = flag.String("pidfile", "", "Location of file to write to.")
)

func main() {
	flag.Parse()
	if *pidfile != "" {
		pid := strconv.Itoa(os.Getpid())
		err := ioutil.WriteFile(*pidfile, []byte(pid), 0644)
		if err != nil {
			log.Fatal("error writing pid %s to file: %s\n", pid, err)
		}
	}

	config, err := config.InitConfigFromFile("registrar_settings.yml")
	if err != nil {
		log.Fatal("error parsing file: %s\n", err)
	}
	registrar := NewRegistrar(config)
	//add health check handler
	checker := InitHealthChecker(config)
	if checker != nil {
		registrar.AddHealthCheckHandler(checker)
	}
	registrar.RegisterRoutes()
	os.Exit(1)
}
