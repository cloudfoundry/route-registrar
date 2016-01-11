package registrar_test

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/apcera/nats"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry-incubator/route-registrar/config"
	healthchecker_fakes "github.com/cloudfoundry-incubator/route-registrar/healthchecker/fakes"
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
		testSpyClient *nats.Conn

		logger lager.Logger

		signals chan os.Signal
		ready   chan struct{}

		r registrar.Registrar

		fakeHealthChecker *healthchecker_fakes.FakeHealthChecker
	)

	BeforeEach(func() {
		natsUsername = "nats-user"
		natsPassword = "nats-pw"
		natsHost = "127.0.0.1"

		natsCmd = startNats(natsHost, natsPort, natsUsername, natsPassword)

		logger = lagertest.NewTestLogger("Registrar test")
		var err error
		servers := []string{
			fmt.Sprintf(
				"nats://%s:%s@%s:%d",
				natsUsername,
				natsPassword,
				natsHost,
				natsPort,
			),
		}

		opts := nats.DefaultOptions
		opts.Servers = servers

		testSpyClient, err = opts.Connect()
		Expect(err).ShouldNot(HaveOccurred())

		messageBusServer := config.MessageBusServer{
			fmt.Sprintf("%s:%d", natsHost, natsPort),
			natsUsername,
			natsPassword,
		}

		rrConfig = config.Config{
			// doesn't matter if these are the same, just want to send a slice
			MessageBusServers: []config.MessageBusServer{messageBusServer, messageBusServer},
			Host:              "my host",
			UpdateFrequency:   1,
		}

		signals = make(chan os.Signal, 1)
		ready = make(chan struct{}, 1)

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

		fakeHealthChecker = &healthchecker_fakes.FakeHealthChecker{}

		r = registrar.NewRegistrar(rrConfig, fakeHealthChecker, logger)
	})

	AfterEach(func() {
		testSpyClient.Close()

		err := natsCmd.Process.Kill()
		Expect(err).NotTo(HaveOccurred())
		// err = natsCmd.Wait()
		// Expect(err).NotTo(HaveOccurred())
	})

	It("periodically registers all URIs for all routes", func() {
		// Detect when a router.register message gets sent
		var registered chan (string)
		registered = subscribeToRegisterEvents(testSpyClient, func(msg *nats.Msg) {
			registered <- string(msg.Data)
		})

		// Detect when an unregister message gets sent
		var unregistered chan (string)
		unregistered = subscribeToUnregisterEvents(testSpyClient, func(msg *nats.Msg) {
			close(unregistered)
		})

		runStatus := make(chan error)
		go func() {
			runStatus <- r.Run(signals, ready)
		}()
		<-ready

		expectedRegistryMessages := []registrar.Message{
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

			var registryMessage registrar.Message
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

		close(signals)
		err := <-runStatus
		Expect(err).ShouldNot(HaveOccurred())
	})

	Context("given a healthcheck", func() {
		var scriptPath string

		BeforeEach(func() {
			scriptPath = "/path/to/some/script/"

			rrConfig.Routes[0].HealthChecker = &config.HealthChecker{
				Name:       "My Healthcheck process",
				ScriptPath: scriptPath,
			}

			r = registrar.NewRegistrar(rrConfig, fakeHealthChecker, logger)
		})

		Context("and the healthcheck succeeds", func() {
			BeforeEach(func() {
				fakeHealthChecker.CheckReturns(true, nil)

				r = registrar.NewRegistrar(rrConfig, fakeHealthChecker, logger)
			})

			It("registers routes", func() {
				// Detect when a router.register message gets sent
				var registered chan (string)
				registered = subscribeToRegisterEvents(testSpyClient, func(msg *nats.Msg) {
					registered <- string(msg.Data)
				})

				// Detect when an unregister message gets sent
				var unregistered chan (string)
				unregistered = subscribeToUnregisterEvents(testSpyClient, func(msg *nats.Msg) {
					unregistered <- string(msg.Data)
				})

				runStatus := make(chan error)
				go func() {
					runStatus <- r.Run(signals, ready)
				}()
				<-ready

				expectedRegistryMessage := registrar.Message{
					URIs: rrConfig.Routes[0].URIs,
					Host: rrConfig.Host,
					Port: rrConfig.Routes[0].Port,
				}

				// Assert that we got the right router.register message
				var receivedMessage string
				Eventually(registered, 2).Should(Receive(&receivedMessage))

				var registryMessage registrar.Message
				err := json.Unmarshal([]byte(receivedMessage), &registryMessage)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(registryMessage.URIs).To(Equal(expectedRegistryMessage.URIs))
				Expect(registryMessage.Port).To(Equal(expectedRegistryMessage.Port))

				// Assert that we never got a router.unregister message
				Consistently(unregistered, 2).ShouldNot(Receive())

				close(signals)
				err = <-runStatus
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when the healthcheck fails", func() {
			BeforeEach(func() {
				fakeHealthChecker.CheckReturns(false, nil)

				r = registrar.NewRegistrar(rrConfig, fakeHealthChecker, logger)
			})

			It("unregisters routes", func() {
				// Detect when a router.register message gets sent
				var registered chan (string)
				registered = subscribeToRegisterEvents(testSpyClient, func(msg *nats.Msg) {
					registered <- string(msg.Data)
				})

				// Detect when an unregister message gets sent
				var unregistered chan (string)
				unregistered = subscribeToUnregisterEvents(testSpyClient, func(msg *nats.Msg) {
					unregistered <- string(msg.Data)
				})

				runStatus := make(chan error)
				go func() {
					runStatus <- r.Run(signals, ready)
				}()
				<-ready

				expectedUnregisterMessage := registrar.Message{
					URIs: rrConfig.Routes[0].URIs,
					Host: rrConfig.Host,
					Port: rrConfig.Routes[0].Port,
				}

				// Assert that we got the right router.unregister message
				var receivedUnregisterMessage string
				Eventually(unregistered, 2).Should(Receive(&receivedUnregisterMessage))

				var unregisterMessage registrar.Message
				err := json.Unmarshal([]byte(receivedUnregisterMessage), &unregisterMessage)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(unregisterMessage.URIs).To(Equal(expectedUnregisterMessage.URIs))
				Expect(unregisterMessage.Port).To(Equal(expectedUnregisterMessage.Port))

				// register
				var receivedRegisterMessage string
				Eventually(registered, 2).Should(Receive(&receivedRegisterMessage))

				expectedRegisterMessage := registrar.Message{
					URIs: rrConfig.Routes[1].URIs,
					Host: rrConfig.Host,
					Port: rrConfig.Routes[1].Port,
				}

				var registerMessage registrar.Message
				err = json.Unmarshal([]byte(receivedRegisterMessage), &registerMessage)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(registerMessage.URIs).To(Equal(expectedRegisterMessage.URIs))
				Expect(registerMessage.Port).To(Equal(expectedRegisterMessage.Port))

				close(signals)
				err = <-runStatus
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Context("given an errored healthcheck", func() {
			var expectedErr error

			BeforeEach(func() {
				expectedErr = fmt.Errorf("boom")
				fakeHealthChecker.CheckReturns(true, expectedErr)

				r = registrar.NewRegistrar(rrConfig, fakeHealthChecker, logger)
			})

			It("unregisters routes", func() {

				// Detect when a router.register message gets sent
				var registered chan (string)
				registered = subscribeToRegisterEvents(testSpyClient, func(msg *nats.Msg) {
					registered <- string(msg.Data)
				})

				// Detect when an unregister message gets sent
				var unregistered chan (string)
				unregistered = subscribeToUnregisterEvents(testSpyClient, func(msg *nats.Msg) {
					unregistered <- string(msg.Data)
				})

				runStatus := make(chan error)
				go func() {
					runStatus <- r.Run(signals, ready)
				}()
				<-ready

				expectedUnregisterMessage := registrar.Message{
					URIs: rrConfig.Routes[0].URIs,
					Host: rrConfig.Host,
					Port: rrConfig.Routes[0].Port,
				}

				// Assert that we got the right router.unregister message
				var receivedUnregisterMessage string
				Eventually(unregistered, 2).Should(Receive(&receivedUnregisterMessage))

				var unregisterMessage registrar.Message
				err := json.Unmarshal([]byte(receivedUnregisterMessage), &unregisterMessage)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(unregisterMessage.URIs).To(Equal(expectedUnregisterMessage.URIs))
				Expect(unregisterMessage.Port).To(Equal(expectedUnregisterMessage.Port))

				// register
				var receivedRegisterMessage string
				Eventually(registered, 2).Should(Receive(&receivedRegisterMessage))

				expectedRegisterMessage := registrar.Message{
					URIs: rrConfig.Routes[1].URIs,
					Host: rrConfig.Host,
					Port: rrConfig.Routes[1].Port,
				}

				var registerMessage registrar.Message
				err = json.Unmarshal([]byte(receivedRegisterMessage), &registerMessage)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(registerMessage.URIs).To(Equal(expectedRegisterMessage.URIs))
				Expect(registerMessage.Port).To(Equal(expectedRegisterMessage.Port))

				close(signals)
				err = <-runStatus
				Expect(err).ShouldNot(HaveOccurred())
			})
		})
	})
})

func subscribeToRegisterEvents(
	testSpyClient *nats.Conn,
	callback func(msg *nats.Msg),
) chan string {
	registerChannel := make(chan string)
	go testSpyClient.Subscribe("router.register", callback)

	return registerChannel
}

func subscribeToUnregisterEvents(
	testSpyClient *nats.Conn,
	callback func(msg *nats.Msg),
) chan string {
	unregisterChannel := make(chan string)
	go testSpyClient.Subscribe("router.unregister", callback)

	return unregisterChannel
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
