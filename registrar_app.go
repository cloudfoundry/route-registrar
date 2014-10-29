package main

import (
	"flag"
	"io/ioutil"
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
		err := ioutil.WriteFile(*pidfile, []byte(strconv.Itoa(os.Getpid())), 0644)
		if err != nil {
			panic(err)
		}
	}

	config := config.InitConfigFromFile("registrar_settings.yml")
	registrar := NewRegistrar(config)
	//add health check handler
	checker := InitHealthChecker(config)
	if checker != nil {
		registrar.AddHealthCheckHandler(checker)
	}
	registrar.RegisterRoutes()
	os.Exit(1)
}
