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

	r.logger.Debug("creating client", lager.Data{"config": r.config})
	client := gibson.NewCFRouterClient(r.config.Host, messageBus)
	client.Greet()

	go func() {
		<-signals
		close(done)
	}()
	close(ready)

	ticker := time.NewTicker(r.config.RefreshInterval)

	for {
		select {
		case <-ticker.C:
			for _, route := range r.config.Routes {
				r.logger.Debug(
					"registering routes",
					lager.Data{
						"port": route.Port,
						"uris": route.URIs,
					},
				)
				client.RegisterAll(
					route.Port,
					route.URIs,
				)
			}
		case <-done:
			r.logger.Info("Received signal; shutting down")
			for _, route := range r.config.Routes {
				r.logger.Debug(
					"deregistering routes",
					lager.Data{
						"port": route.Port,
						"uris": route.URIs,
					},
				)
				client.UnregisterAll(
					route.Port,
					route.URIs,
				)
				return nil
			}
		}
	}
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
