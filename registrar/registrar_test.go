package registrar_test

import (
	"encoding/json"
	"os"
	"syscall"

	"github.com/cloudfoundry/gibson"
	"github.com/cloudfoundry/yagnats"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry-incubator/route-registrar/config"
	"github.com/cloudfoundry-incubator/route-registrar/healthchecker/fakes"
	. "github.com/cloudfoundry-incubator/route-registrar/registrar"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Registrar.RegisterRoutes", func() {
	var (
		config        Config
		testSpyClient *yagnats.Client

		logger           lager.Logger
		messageBusServer MessageBusServer
	)

	BeforeEach(func() {
		messageBusServer = MessageBusServer{
			"127.0.0.1:4222",
			"nats",
			"nats",
		}

		logger = lagertest.NewTestLogger("Registrar test")
		testSpyClient = yagnats.NewClient()
		connectionInfo := yagnats.ConnectionInfo{
			messageBusServer.Host,
			messageBusServer.User,
			messageBusServer.Password,
			nil,
		}

		err := testSpyClient.Connect(&connectionInfo)
		Expect(err).NotTo(HaveOccurred())

		config = Config{
			MessageBusServers: []MessageBusServer{messageBusServer, messageBusServer}, // doesn't matter if these are the same, just want to send a slice
		}
	})

	AfterEach(func() {
		testSpyClient.Disconnect()
	})

	Context("When single external host is provided", func() {
		BeforeEach(func() {

			healthCheckerConfig := HealthCheckerConf{
				Name:     "a_useful_health_checker",
				Interval: 1,
			}

			config.ExternalHost = "riakcs.vcap.me"
			config.ExternalIp = "127.0.0.1"
			config.Port = 8080
			config.HealthChecker = &healthCheckerConfig
		})

		It("Sends a router.register message and does not send a router.unregister message", func() {
			// Detect when a router.register message gets sent
			var registered chan (string)
			registered = subscribeToRegisterEvents(testSpyClient, func(msg *yagnats.Message) {
				registered <- string(msg.Payload)
			})

			// Detect when an unregister message gets sent
			var unregistered chan (bool)
			unregistered = subscribeToUnregisterEvents(testSpyClient, func(msg *yagnats.Message) {
				unregistered <- true
			})

			go func() {
				registrar := NewRegistrar(config, logger)
				registrar.RegisterRoutes()
			}()

			// Assert that we got the right router.register message
			var receivedMessage string
			Eventually(registered, 2).Should(Receive(&receivedMessage))

			expectedRegistryMessage := gibson.RegistryMessage{
				URIs: []string{"riakcs.vcap.me"},
				Host: "127.0.0.1",
				Port: 8080,
			}

			var registryMessage gibson.RegistryMessage
			err := json.Unmarshal([]byte(receivedMessage), &registryMessage)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(registryMessage.URIs).To(Equal(expectedRegistryMessage.URIs))
			Expect(registryMessage.Host).To(Equal(expectedRegistryMessage.Host))
			Expect(registryMessage.Port).To(Equal(expectedRegistryMessage.Port))

			// Assert that we never got a router.unregister message
			Consistently(unregistered, 2).ShouldNot(Receive())
		})

		It("Emits a router.unregister message when SIGINT is sent to the registrar's signal channel", func() {
			verifySignalTriggersUnregister(
				config,
				syscall.SIGINT,
				logger,
				testSpyClient,
			)
		})

		It("Emits a router.unregister message when SIGTERM is sent to the registrar's signal channel", func() {
			verifySignalTriggersUnregister(
				config,
				syscall.SIGTERM,
				logger,
				testSpyClient,
			)
		})

		Context("When the registrar has a healthchecker", func() {
			It("Emits a router.unregister message when registrar's health check fails, and emits a router.register message when registrar's health check back to normal", func() {
				healthy := fakes.NewFakeHealthChecker()
				healthy.CheckReturns(true)

				unregistered := make(chan string)
				registered := make(chan string)
				var registrar *Registrar

				// Listen for a router.unregister event, then set health status to true, then listen for a router.register event
				subscribeToRegisterEvents(testSpyClient, func(msg *yagnats.Message) {
					registered <- string(msg.Payload)

					healthy.CheckReturns(false)

					subscribeToUnregisterEvents(testSpyClient, func(msg *yagnats.Message) {
						unregistered <- string(msg.Payload)
					})
				})

				go func() {
					registrar = NewRegistrar(config, logger)
					registrar.AddHealthCheckHandler(healthy)
					registrar.RegisterRoutes()
				}()

				var receivedMessage string
				testTimeout := config.HealthChecker.Interval * 3

				expectedRegistryMessage := gibson.RegistryMessage{
					URIs: []string{"riakcs.vcap.me"},
					Host: "127.0.0.1",
					Port: 8080,
				}

				var registryMessage gibson.RegistryMessage

				Eventually(registered, testTimeout).Should(Receive(&receivedMessage))
				err := json.Unmarshal([]byte(receivedMessage), &registryMessage)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(registryMessage.URIs).To(Equal(expectedRegistryMessage.URIs))
				Expect(registryMessage.Host).To(Equal(expectedRegistryMessage.Host))
				Expect(registryMessage.Port).To(Equal(expectedRegistryMessage.Port))

				Eventually(unregistered, testTimeout).Should(Receive(&receivedMessage))
				err = json.Unmarshal([]byte(receivedMessage), &registryMessage)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(registryMessage.URIs).To(Equal(expectedRegistryMessage.URIs))
				Expect(registryMessage.Host).To(Equal(expectedRegistryMessage.Host))
				Expect(registryMessage.Port).To(Equal(expectedRegistryMessage.Port))
			})
		})

	})
})

func verifySignalTriggersUnregister(
	config Config,
	signal os.Signal,
	logger lager.Logger,
	testSpyClient *yagnats.Client,
) {
	unregistered := make(chan string)
	returned := make(chan bool)

	var registrar *Registrar

	// Trigger a SIGINT after a successful router.register message
	subscribeToRegisterEvents(testSpyClient, func(msg *yagnats.Message) {
		registrar.SignalChannel <- signal
	})

	// Detect when a router.unregister message gets sent
	subscribeToUnregisterEvents(testSpyClient, func(msg *yagnats.Message) {
		unregistered <- string(msg.Payload)
	})

	go func() {
		registrar = NewRegistrar(config, logger)
		registrar.RegisterRoutes()

		// Set up a channel to wait for RegisterRoutes to return
		returned <- true
	}()

	// Assert that we got the right router.unregister message as a result of the signal
	var receivedMessage string
	Eventually(unregistered, 2).Should(Receive(&receivedMessage))

	expectedRegistryMessage := gibson.RegistryMessage{
		URIs: []string{"riakcs.vcap.me"},
		Host: "127.0.0.1",
		Port: 8080,
	}

	var registryMessage gibson.RegistryMessage
	err := json.Unmarshal([]byte(receivedMessage), &registryMessage)

	Expect(err).ShouldNot(HaveOccurred())
	Expect(registryMessage.URIs).To(Equal(expectedRegistryMessage.URIs))
	Expect(registryMessage.Host).To(Equal(expectedRegistryMessage.Host))
	Expect(registryMessage.Port).To(Equal(expectedRegistryMessage.Port))

	// Assert that RegisterRoutes returned
	Expect(returned).To(Receive())
}

func subscribeToRegisterEvents(testSpyClient *yagnats.Client, callback func(msg *yagnats.Message)) (registerChannel chan string) {
	registerChannel = make(chan string)
	go testSpyClient.Subscribe("router.register", callback)

	return
}

func subscribeToUnregisterEvents(testSpyClient *yagnats.Client, callback func(msg *yagnats.Message)) (unregisterChannel chan bool) {
	unregisterChannel = make(chan bool)
	go testSpyClient.Subscribe("router.unregister", callback)

	return
}
