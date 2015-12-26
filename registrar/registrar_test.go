package registrar_test

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/cloudfoundry/gibson"
	"github.com/cloudfoundry/yagnats"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry-incubator/route-registrar/config"
	"github.com/cloudfoundry-incubator/route-registrar/healthchecker/fakes"
	"github.com/cloudfoundry-incubator/route-registrar/registrar"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Registrar.RegisterRoutes", func() {
	var (
		natsCmd      *exec.Cmd
		natsHost     string
		natsUsername string
		natsPassword string

		rrConfig      config.Config
		testSpyClient *yagnats.Client

		logger           lager.Logger
		messageBusServer config.MessageBusServer

		signals chan os.Signal
		ready   chan struct{}

		r registrar.Registrar
	)

	BeforeEach(func() {
		natsUsername = "nats-user"
		natsPassword = "nats-pw"
		natsHost = "127.0.0.1"

		natsCmd = startNats(natsHost, natsPort, natsUsername, natsPassword)

		messageBusServer = config.MessageBusServer{
			fmt.Sprintf("%s:%d", natsHost, natsPort),
			natsUsername,
			natsPassword,
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

		rrConfig = config.Config{
			// doesn't matter if these are the same, just want to send a slice
			MessageBusServers: []config.MessageBusServer{messageBusServer, messageBusServer},
		}

		signals = make(chan os.Signal, 1)
		ready = make(chan struct{}, 1)
	})

	AfterEach(func() {
		testSpyClient.Disconnect()

		natsCmd.Process.Kill()
		natsCmd.Wait()
	})

	Context("When single external host is provided", func() {
		BeforeEach(func() {
			rrConfig.ExternalHost = "some-external-host"
			rrConfig.ExternalIP = "127.0.0.1"
			rrConfig.Port = 8080

			r = registrar.NewRegistrar(rrConfig, logger)
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
				r.Run(signals, ready)
			}()
			<-ready

			// Assert that we got the right router.register message
			var receivedMessage string
			Eventually(registered, 2).Should(Receive(&receivedMessage))

			expectedRegistryMessage := gibson.RegistryMessage{
				URIs: []string{rrConfig.ExternalHost},
				Host: rrConfig.ExternalIP,
				Port: rrConfig.Port,
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

		verifySignalTriggersUnregister := func(signal os.Signal) {
			r := registrar.NewRegistrar(rrConfig, logger)

			unregistered := make(chan string)

			// Send a signal after a successful router.register message
			subscribeToRegisterEvents(testSpyClient, func(msg *yagnats.Message) {
				signals <- signal
			})

			// Detect when a router.unregister message gets sent
			subscribeToUnregisterEvents(testSpyClient, func(msg *yagnats.Message) {
				unregistered <- string(msg.Payload)
			})

			ready := make(chan struct{}, 1)
			r.Run(signals, ready)
			<-ready

			// Assert that we got the right router.unregister message as a result of the signal
			var receivedMessage string
			Eventually(unregistered, 2).Should(Receive(&receivedMessage))

			expectedRegistryMessage := gibson.RegistryMessage{
				URIs: []string{rrConfig.ExternalHost},
				Host: rrConfig.ExternalIP,
				Port: rrConfig.Port,
			}

			var registryMessage gibson.RegistryMessage
			err := json.Unmarshal([]byte(receivedMessage), &registryMessage)

			Expect(err).ShouldNot(HaveOccurred())
			Expect(registryMessage.URIs).To(Equal(expectedRegistryMessage.URIs))
			Expect(registryMessage.Host).To(Equal(expectedRegistryMessage.Host))
			Expect(registryMessage.Port).To(Equal(expectedRegistryMessage.Port))
		}
		It("Emits a router.unregister message when SIGINT is sent to the registrar's signal channel", func() {
			verifySignalTriggersUnregister(syscall.SIGINT)
		})

		It("Emits a router.unregister message when SIGTERM is sent to the registrar's signal channel", func() {
			verifySignalTriggersUnregister(syscall.SIGTERM)
		})

		Context("When the registrar has a healthchecker", func() {
			BeforeEach(func() {
				healthCheckerConfig := config.HealthCheckerConf{
					Name:     "a_useful_health_checker",
					Interval: 1,
				}

				rrConfig.HealthChecker = &healthCheckerConfig
			})

			It("Emits a router.unregister message when registrar's health check fails, and emits a router.register message when registrar's health check back to normal", func() {
				r = registrar.NewRegistrar(rrConfig, logger)

				healthy := fakes.NewFakeHealthChecker()
				healthy.CheckReturns(true)

				r.AddHealthCheckHandler(healthy)

				unregistered := make(chan string)
				registered := make(chan string)

				// Listen for a router.unregister event, then set health status to true,
				// then listen for a router.register event
				subscribeToRegisterEvents(testSpyClient, func(msg *yagnats.Message) {
					registered <- string(msg.Payload)

					healthy.CheckReturns(false)

					subscribeToUnregisterEvents(testSpyClient, func(msg *yagnats.Message) {
						unregistered <- string(msg.Payload)
					})
				})

				go func() {
					r.Run(signals, ready)
				}()
				<-ready

				var receivedMessage string
				testTimeout := rrConfig.HealthChecker.Interval * 3

				expectedRegistryMessage := gibson.RegistryMessage{
					URIs: []string{rrConfig.ExternalHost},
					Host: rrConfig.ExternalIP,
					Port: rrConfig.Port,
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

	Context("When backing cf-release style route registration", func() {
		BeforeEach(func() {
			rrConfig.Host = "my host"
			rrConfig.RefreshInterval = 100 * time.Millisecond
		})

		Context("one route, multiple URIs", func() {
			BeforeEach(func() {
				rrConfig.Routes = []config.Route{
					{
						Name: "my route",
						Port: 8080,
						URIs: []string{
							"my uri 1",
							"my uri 2",
						},
					},
				}

				r = registrar.NewRegistrar(rrConfig, logger)
			})

			It("periodically registers all URIs for all URIs associated with the route", func() {
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
					r.Run(signals, ready)
				}()
				<-ready

				// Assert that we got the right router.register message
				var receivedMessage string
				Eventually(registered, 2).Should(Receive(&receivedMessage))

				var registryMessage gibson.RegistryMessage
				err := json.Unmarshal([]byte(receivedMessage), &registryMessage)
				Expect(err).ShouldNot(HaveOccurred())

				expectedRegistryMessage := gibson.RegistryMessage{
					URIs: rrConfig.Routes[0].URIs,
					Host: rrConfig.Host,
					Port: rrConfig.Routes[0].Port,
				}

				Expect(registryMessage.URIs).To(Equal(expectedRegistryMessage.URIs))
				Expect(registryMessage.Host).To(Equal(expectedRegistryMessage.Host))
				Expect(registryMessage.Port).To(Equal(expectedRegistryMessage.Port))

				// Assert that we never got a router.unregister message
				Consistently(unregistered, 2).ShouldNot(Receive())
			})
		})

		Context("multiple routes, each with multiple URIs", func() {
			BeforeEach(func() {
				rrConfig.Routes = []config.Route{
					{
						Name: "my route 1",
						Port: 8080,
						URIs: []string{
							"my uri 1.1",
							"my uri 1.2",
						},
					},
					{
						Name: "my route 2",
						Port: 8081,
						URIs: []string{
							"my uri 2.1",
							"my uri 2.2",
						},
					},
				}

				r = registrar.NewRegistrar(rrConfig, logger)
			})

			It("periodically registers all URIs for all routes", func() {
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
					r.Run(signals, ready)
				}()
				<-ready

				expectedRegistryMessages := []gibson.RegistryMessage{
					{
						URIs: rrConfig.Routes[0].URIs,
						Host: rrConfig.Host,
						Port: rrConfig.Routes[0].Port,
					},
					{
						URIs: rrConfig.Routes[1].URIs,
						Host: rrConfig.Host,
						Port: rrConfig.Routes[1].Port,
					},
				}

				for i := 0; i < len(expectedRegistryMessages); i++ {
					// Assert that we got the right router.register message
					var receivedMessage string
					Eventually(registered, 2).Should(Receive(&receivedMessage))

					var registryMessage gibson.RegistryMessage
					err := json.Unmarshal([]byte(receivedMessage), &registryMessage)
					Expect(err).ShouldNot(HaveOccurred())

					switch registryMessage.Port {
					case expectedRegistryMessages[0].Port:
						Expect(registryMessage.URIs).To(Equal(expectedRegistryMessages[0].URIs))
						break
					case expectedRegistryMessages[1].Port:
						Expect(registryMessage.URIs).To(Equal(expectedRegistryMessages[1].URIs))
						break
					default:
						Fail("Unexpected port in nats message")
					}
				}

				// Assert that we never got a router.unregister message
				Consistently(unregistered, 2).ShouldNot(Receive())
			})
		})
	})
})

func subscribeToRegisterEvents(
	testSpyClient *yagnats.Client,
	callback func(msg *yagnats.Message),
) (registerChannel chan string) {
	registerChannel = make(chan string)
	go testSpyClient.Subscribe("router.register", callback)

	return
}

func subscribeToUnregisterEvents(
	testSpyClient *yagnats.Client,
	callback func(msg *yagnats.Message),
) (unregisterChannel chan bool) {
	unregisterChannel = make(chan bool)
	go testSpyClient.Subscribe("router.unregister", callback)

	return
}

func startNats(host string, port int, username, password string) *exec.Cmd {
	fmt.Fprintf(GinkgoWriter, "Starting gnatsd on port %d\n", port)

	cmd := exec.Command(
		"gnatsd",
		"-p", strconv.Itoa(port),
		"--user", username,
		"--pass", password)

	err := cmd.Start()
	if err != nil {
		fmt.Printf("gnatsd failed to start: %v\n", err)
	}

	natsTimeout := 10 * time.Second
	natsPollingInterval := 20 * time.Millisecond
	Eventually(func() error {
		_, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
		return err
	}, natsTimeout, natsPollingInterval).Should(Succeed())

	fmt.Fprintf(GinkgoWriter, "gnatsd running on port %d\n", port)
	return cmd
}
