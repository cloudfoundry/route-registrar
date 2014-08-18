package main

import (
	"os"

	"github.com/cloudfoundry-incubator/route-registrar/config"
	. "github.com/cloudfoundry-incubator/route-registrar/healthchecker"
	. "github.com/cloudfoundry-incubator/route-registrar/registrar"
)

func main() {
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
