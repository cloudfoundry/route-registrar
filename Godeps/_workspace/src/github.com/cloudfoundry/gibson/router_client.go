package gibson

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/apcera/nats"
	"github.com/cloudfoundry/yagnats"
	"github.com/nu7hatch/gouuid"
)

type RouterClient interface {
	Greet() error
	Register(port int, uri string) error
	RegisterAll(port int, uris []string) error
	Unregister(port int, uri string) error
	UnregisterAll(port int, uris []string) error
}

type CFRouterClient struct {
	Host              string
	PrivateInstanceId string
	messageBus        yagnats.NATSConn

	registry *Registry

	stopPeriodicCallback chan bool

	lock sync.RWMutex
}

type RegistryMessage struct {
	URIs              []string `json:"uris"`
	Host              string   `json:"host"`
	Port              int      `json:"port"`
	PrivateInstanceId string   `json:"private_instance_id"`
}

type RouterGreetingMessage struct {
	MinimumRegisterInterval int `json:"minimumRegisterIntervalInSeconds"`
}

func NewCFRouterClient(host string, messageBus yagnats.NATSConn) *CFRouterClient {
	u4, err := uuid.NewV4()
	if err != nil {
		log.Printf("failed to create UUID as private instance id: %s\n", err)
	}
	return &CFRouterClient{
		Host:              host,
		PrivateInstanceId: u4.String(),

		registry: NewRegistry(),

		messageBus: messageBus,
	}
}

func (r *CFRouterClient) Greet() error {
	_, err := r.messageBus.Subscribe("router.start", r.handleGreeting)
	if err != nil {
		return err
	}

	uuid, err := uuid.NewV4()
	if err != nil {
		return err
	}

	replyTo := uuid.String()

	r.messageBus.Subscribe(replyTo, r.handleGreeting)

	return r.messageBus.PublishRequest("router.greet", replyTo, []byte{})
}

func (r *CFRouterClient) Register(port int, uri string) error {
	r.registry.Register(port, uri)
	return r.sendRegistryMessage("router.register", port, []string{uri})
}

func (r *CFRouterClient) Unregister(port int, uri string) error {
	r.registry.Unregister(port, uri)
	return r.sendRegistryMessage("router.unregister", port, []string{uri})
}

func (r *CFRouterClient) RegisterAll(port int, uris []string) error {
	r.registry.RegisterAll(port, uris)
	return r.sendRegistryMessage("router.register", port, uris)
}

func (r *CFRouterClient) UnregisterAll(port int, uris []string) error {
	r.registry.UnregisterAll(port, uris)
	return r.sendRegistryMessage("router.unregister", port, uris)
}

func (r *CFRouterClient) handleGreeting(greeting *nats.Msg) {
	interval, err := r.intervalFrom(greeting.Data)
	if err != nil {
		log.Printf("failed to parse router.start: %s\n", err)
		return
	}

	go r.callbackPeriodically(time.Duration(interval) * time.Second)
}

func (r *CFRouterClient) callbackPeriodically(interval time.Duration) {
	if r.stopPeriodicCallback != nil {
		r.stopPeriodicCallback <- true
	}

	cancel := make(chan bool)

	r.stopPeriodicCallback = cancel

	for stop := false; !stop; {
		select {
		case <-time.After(interval):
			r.registerAllRoutes()
		case stop = <-cancel:
		}
	}
}

func (r *CFRouterClient) sendRegistryMessage(subject string, port int, uris []string) error {
	msg := &RegistryMessage{
		URIs:              uris,
		Host:              r.Host,
		Port:              port,
		PrivateInstanceId: r.PrivateInstanceId,
	}

	json, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return r.messageBus.Publish(subject, json)
}

func (r *CFRouterClient) intervalFrom(greetingPayload []byte) (int, error) {
	var greeting RouterGreetingMessage

	err := json.Unmarshal(greetingPayload, &greeting)
	if err != nil {
		return 0, err
	}

	return greeting.MinimumRegisterInterval, nil
}

func (r *CFRouterClient) registerAllRoutes() {
	r.registry.InParallel(func(port int, uris []string) {
		r.sendRegistryMessage("router.register", port, uris)
	})
}
