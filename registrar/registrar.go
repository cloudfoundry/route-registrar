package registrar

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudfoundry/gibson"
	"github.com/cloudfoundry/yagnats"

	"github.com/cloudfoundry-incubator/route-registrar/config"
	"github.com/cloudfoundry-incubator/route-registrar/healthchecker"

	"github.com/pivotal-golang/lager"
)

type Registrar struct {
	logger               lager.Logger
	Config               config.Config
	SignalChannel        chan os.Signal
	HealthChecker        healthchecker.HealthChecker
	previousHealthStatus bool
}

func NewRegistrar(clientConfig config.Config, logger lager.Logger) *Registrar {
	return &Registrar{
		Config:               clientConfig,
		logger:               logger,
		SignalChannel:        make(chan os.Signal, 1),
		previousHealthStatus: false,
	}
}

func (registrar *Registrar) AddHealthCheckHandler(handler healthchecker.HealthChecker) {
	registrar.HealthChecker = handler
}

type callbackFunction func()

func (registrar *Registrar) RegisterRoutes() {
	messageBus := buildMessageBus(registrar)

	done := make(chan bool)

	if len(registrar.Config.Routes) > 0 {
		registrar.logger.Debug("creating client", lager.Data{"config": registrar.Config})

		client := gibson.NewCFRouterClient(registrar.Config.Host, messageBus)
		client.Greet()
		registrar.registerSignalHandler(done, client)

		ticker := time.NewTicker(registrar.Config.RefreshInterval)

		for {
			select {
			case <-ticker.C:
				registrar.logger.Debug(
					"registering routes",
					lager.Data{
						"port": registrar.Config.Routes[0].Port,
						"uris": registrar.Config.Routes[0].URIs,
					},
				)
				client.RegisterAll(
					registrar.Config.Routes[0].Port,
					registrar.Config.Routes[0].URIs,
				)
			case <-done:
				registrar.logger.Debug(
					"deregistering routes",
					lager.Data{
						"port": registrar.Config.Routes[0].Port,
						"uris": registrar.Config.Routes[0].URIs,
					},
				)
				client.UnregisterAll(
					registrar.Config.Routes[0].Port,
					registrar.Config.Routes[0].URIs,
				)
				return
			}
		}
	}

	client := gibson.NewCFRouterClient(registrar.Config.ExternalIp, messageBus)
	client.Greet()
	registrar.registerSignalHandler(done, client)

	if registrar.HealthChecker != nil {
		callbackInterval := time.Duration(registrar.Config.HealthChecker.Interval) * time.Second
		callbackPeriodically(callbackInterval,
			func() { registrar.updateRegistrationBasedOnHealthCheck(client) },
			done)
	} else {
		client.Register(registrar.Config.Port, registrar.Config.ExternalHost)

		select {
		case <-done:
			return
		}
	}
}

func buildMessageBus(registrar *Registrar) yagnats.NATSConn {
	var natsServers []string

	for _, server := range registrar.Config.MessageBusServers {
		registrar.logger.Info(
			"Adding NATS server",
			lager.Data{"server": server},
		)
		natsServers = append(
			natsServers,
			fmt.Sprintf("nats://%s:%s@%s", server.User, server.Password, server.Host),
		)
	}
	messageBus, err := yagnats.Connect(natsServers)
	if err != nil {
		panic(err)
	}
	return messageBus
}

func callbackPeriodically(duration time.Duration, callback callbackFunction, done chan bool) {
	interval := time.NewTicker(duration)
	for stop := false; !stop; {
		select {
		case <-interval.C:
			callback()
		case stop = <-done:
			return
		}
	}
}

func (registrar *Registrar) updateRegistrationBasedOnHealthCheck(client *gibson.CFRouterClient) {
	current := registrar.HealthChecker.Check()
	if (!current) && registrar.previousHealthStatus {
		registrar.logger.Info("Health check status changed to unavailabile; unregistering the route")
		client.Unregister(registrar.Config.Port, registrar.Config.ExternalHost)
	} else if current && (!registrar.previousHealthStatus) {
		registrar.logger.Info("Health check status changed to availabile; registering the route")
		client.Register(registrar.Config.Port, registrar.Config.ExternalHost)
	}
	registrar.previousHealthStatus = current
}

func (registrar *Registrar) registerSignalHandler(done chan bool, client *gibson.CFRouterClient) {
	go func() {
		select {
		case <-registrar.SignalChannel:
			registrar.logger.Info("Received SIGTERM or SIGINT; unregistering the route")
			client.Unregister(registrar.Config.Port, registrar.Config.ExternalHost)
			done <- true
		}
	}()

	signal.Notify(registrar.SignalChannel, syscall.SIGINT, syscall.SIGTERM)
}
