package registrar

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/cloudfoundry/gibson"
	"github.com/cloudfoundry/yagnats"

	"github.com/cloudfoundry-incubator/route-registrar/config"
	"github.com/cloudfoundry-incubator/route-registrar/healthchecker"

	"github.com/pivotal-golang/lager"
)

type Registrar interface {
	AddHealthCheckHandler(handler healthchecker.HealthChecker)
	Run(signals <-chan os.Signal, ready chan<- struct{}) error
}

type registrar struct {
	logger        lager.Logger
	config        config.Config
	healthChecker healthchecker.HealthChecker
	wasHealthy    bool

	lock sync.RWMutex
}

func NewRegistrar(clientConfig config.Config, logger lager.Logger) Registrar {
	return &registrar{
		config:     clientConfig,
		logger:     logger,
		wasHealthy: false,
	}
}

func (r *registrar) AddHealthCheckHandler(handler healthchecker.HealthChecker) {
	r.healthChecker = handler
}

func (r *registrar) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	messageBus := buildMessageBus(r)

	done := make(chan bool)

	close(ready)

	if len(r.config.Routes) > 0 {
		r.logger.Debug("creating client", lager.Data{"config": r.config})

		client := gibson.NewCFRouterClient(r.config.Host, messageBus)
		client.Greet()
		r.registerSignalHandler(signals, done, client)

		ticker := time.NewTicker(r.config.RefreshInterval)

		for {
			select {
			case <-ticker.C:
				r.logger.Debug(
					"registering routes",
					lager.Data{
						"port": r.config.Routes[0].Port,
						"uris": r.config.Routes[0].URIs,
					},
				)
				client.RegisterAll(
					r.config.Routes[0].Port,
					r.config.Routes[0].URIs,
				)
			case <-done:
				r.logger.Debug(
					"deregistering routes",
					lager.Data{
						"port": r.config.Routes[0].Port,
						"uris": r.config.Routes[0].URIs,
					},
				)
				client.UnregisterAll(
					r.config.Routes[0].Port,
					r.config.Routes[0].URIs,
				)
				return nil
			}
		}
	}

	client := gibson.NewCFRouterClient(r.config.ExternalIp, messageBus)
	client.Greet()
	r.registerSignalHandler(signals, done, client)

	if r.healthChecker != nil {
		callbackInterval := time.Duration(r.config.HealthChecker.Interval) * time.Second
		callbackPeriodically(
			callbackInterval,
			func() { r.updateRegistrationBasedOnHealthCheck(client) },
			done)
	} else {
		client.Register(r.config.Port, r.config.ExternalHost)

		select {
		case <-done:
			return nil
		}
	}
	return nil
}

func buildMessageBus(r *registrar) yagnats.NATSConn {
	var natsServers []string

	for _, server := range r.config.MessageBusServers {
		r.logger.Info(
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

func callbackPeriodically(duration time.Duration, callback func(), done chan bool) {
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

func (r *registrar) updateRegistrationBasedOnHealthCheck(client *gibson.CFRouterClient) {
	r.lock.Lock()
	defer r.lock.Unlock()

	current := r.healthChecker.Check()
	if (current) && r.wasHealthy {
		r.logger.Debug("still healthy")
	} else if (!current) && !r.wasHealthy {
		r.logger.Debug("still unhealthy")
	} else if (!current) && r.wasHealthy {
		r.logger.Info("Health check status changed to unavailable; unregistering the route")
		client.Unregister(r.config.Port, r.config.ExternalHost)
	} else if current && (!r.wasHealthy) {
		r.logger.Info("Health check status changed to available; registering the route")
		client.Register(r.config.Port, r.config.ExternalHost)
	}
	r.wasHealthy = current
}

func (r *registrar) registerSignalHandler(
	signals <-chan os.Signal,
	done chan bool,
	client *gibson.CFRouterClient,
) {
	go func() {
		select {
		case <-signals:
			r.logger.Info("Received signal; unregistering the route")
			client.Unregister(r.config.Port, r.config.ExternalHost)
			done <- true
		}
	}()
}
