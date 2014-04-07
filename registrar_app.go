package main

import (
	"os"

	. "github.com/cloudfoundry-incubator/route-registrar/registrar"
	"github.com/cloudfoundry-incubator/route-registrar/config"
	. "github.com/cloudfoundry-incubator/route-registrar/healthchecker"
)

func main() {
	config := config.InitConfigFromFile("registrar_settings.yml")
	registrar := NewRegistrar(config)
	//add health check handler
	checker := InitHealthChecker(config)
	registrar.AddHealthCheckHandler(checker)
	registrar.RegisterRoutes()
	os.Exit(1)
}
