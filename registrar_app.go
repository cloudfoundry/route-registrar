package main

import (
	"os"

	. "github.com/cloudfoundry-incubator/route-registrar/registrar"
	"github.com/cloudfoundry-incubator/route-registrar/config"
)

func main() {
	config := config.InitConfigFromFile("registrar_settings.yml")
	registrar := NewRegistrar(config)
	registrar.RegisterRoutes()
	os.Exit(1)
}
