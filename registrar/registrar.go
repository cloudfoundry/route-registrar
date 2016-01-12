package registrar

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/nats-io/nats"
	"github.com/nu7hatch/gouuid"

	"github.com/cloudfoundry-incubator/route-registrar/config"
	"github.com/cloudfoundry-incubator/route-registrar/healthchecker"

	"github.com/pivotal-golang/lager"
)

type Registrar interface {
	Run(signals <-chan os.Signal, ready chan<- struct{}) error
}

type Message struct {
	URIs              []string `json:"uris"`
	Host              string   `json:"host"`
	Port              int      `json:"port"`
	PrivateInstanceId string   `json:"private_instance_id"`
}

type registrar struct {
	logger            lager.Logger
	config            config.Config
	healthChecker     healthchecker.HealthChecker
	wasHealthy        bool
	messageBus        *nats.Conn
	privateInstanceId string

	lock sync.RWMutex
}

func NewRegistrar(
	clientConfig config.Config,
	healthChecker healthchecker.HealthChecker,
	logger lager.Logger,
) Registrar {
	aUUID, err := uuid.NewV4()
	if err != nil {
		panic(err)
	}
	return &registrar{
		config:            clientConfig,
		logger:            logger,
		wasHealthy:        false,
		privateInstanceId: aUUID.String(),
		healthChecker:     healthChecker,
	}
}

func (r *registrar) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	var err error

	r.logger.Info("creating nats connection", lager.Data{"config": r.config})
	r.messageBus, err = buildMessageBus(r)
	if err != nil {
		panic(err)
	}
	defer r.messageBus.Close()

	close(ready)

	duration := time.Duration(r.config.UpdateFrequency) * time.Second
	ticker := time.NewTicker(duration)

	for {
		select {
		case <-ticker.C:
			for _, route := range r.config.Routes {
				if route.HealthCheck == nil || route.HealthCheck.ScriptPath == "" {
					r.logger.Info("no healthchecker found for route", lager.Data{"route": route})

					err := r.registerRoutes(route)
					if err != nil {
						return err
					}
				} else {
					ok, err := r.healthChecker.Check(route.HealthCheck.ScriptPath, route.HealthCheck.Timeout)
					if err != nil {
						r.logger.Info("healthchecker errored for route", lager.Data{"route": route})
						err := r.unregisterRoutes(route)
						if err != nil {
							panic(err)
						}
					} else if ok {
						r.logger.Info("healthchecker returned healthy for route", lager.Data{"route": route})
						err := r.registerRoutes(route)
						if err != nil {
							panic(err)
						}
					} else {
						r.logger.Info("healthchecker returned unhealthy for route", lager.Data{"route": route})
						err := r.unregisterRoutes(route)
						if err != nil {
							panic(err)
						}
					}
				}
			}
		case <-signals:
			r.logger.Info("Received signal; shutting down")

			for _, route := range r.config.Routes {
				err := r.unregisterRoutes(route)
				if err != nil {
					return err
				}
				return nil
			}
		}
	}
}

func (r registrar) registerRoutes(route config.Route) error {
	r.logger.Info(
		"Registering routes",
		lager.Data{
			"port": route.Port,
			"uris": route.URIs,
		},
	)

	err := r.sendMessage("router.register", r.config.Host, route)
	if err != nil {
		return err
	}

	return nil
}

func (r registrar) unregisterRoutes(route config.Route) error {
	r.logger.Info(
		"Unregistering routes",
		lager.Data{
			"port": route.Port,
			"uris": route.URIs,
		},
	)

	err := r.sendMessage("router.unregister", r.config.Host, route)
	if err != nil {
		return err
	}

	return nil
}

func (r registrar) sendMessage(subject string, host string, route config.Route) error {
	msg := &Message{
		URIs:              route.URIs,
		Host:              host,
		Port:              route.Port,
		PrivateInstanceId: r.privateInstanceId,
	}

	json, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return r.messageBus.Publish(subject, json)
}

func buildMessageBus(r *registrar) (*nats.Conn, error) {
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

	opts := nats.DefaultOptions
	opts.Servers = natsServers

	return opts.Connect()
}
