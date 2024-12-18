package messagebus_test

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"

	tls_helpers "code.cloudfoundry.org/cf-routing-test-helpers/tls"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/route-registrar/config"
	"code.cloudfoundry.org/route-registrar/messagebus"
	"code.cloudfoundry.org/tlsconfig"
	"github.com/nats-io/nats.go"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Messagebus test Suite", func() {
	var (
		natsCmd      *exec.Cmd
		natsHost     string
		natsUsername string
		natsPassword string

		testSpyClient *nats.Conn

		logger            lager.Logger
		messageBusServers []config.MessageBusServer
		messageBus        messagebus.MessageBus
	)

	BeforeEach(func() {
		natsUsername = "nats-user"
		natsPassword = "nats-pw"
		natsHost = "127.0.0.1"

		natsCmd = startNats(natsHost, natsPort, natsUsername, natsPassword)

		logger = lagertest.NewTestLogger("nats-test")
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

		opts := nats.GetDefaultOptions()
		opts.Servers = servers

		testSpyClient, err = opts.Connect()
		Expect(err).ToNot(HaveOccurred())

		// Ensure nats server is listening before tests
		Eventually(func() string {
			connStatus := testSpyClient.Status()
			return fmt.Sprintf("%v", connStatus)
		}, 5*time.Second).Should(Equal("CONNECTED"))

		Expect(err).ShouldNot(HaveOccurred())

		messageBusServer := config.MessageBusServer{
			Host:     fmt.Sprintf("%s:%d", natsHost, natsPort),
			User:     natsUsername,
			Password: natsPassword,
		}

		messageBusServers = []config.MessageBusServer{messageBusServer}

		messageBus = messagebus.NewMessageBus(logger, "some-az")
	})

	AfterEach(func() {
		testSpyClient.Close()

		err := natsCmd.Process.Kill()
		Expect(err).NotTo(HaveOccurred())
		_, err = natsCmd.Process.Wait()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Connect", func() {
		It("connects without error", func() {
			err := messageBus.Connect(messageBusServers, nil)
			Expect(err).ShouldNot(HaveOccurred())
		})

		Context("when tls config is provided", func() {
			var (
				natsTlsHost            string
				natsTlsPort            int
				natsTlsCmd             *exec.Cmd
				tlsMessageBusServers   []config.MessageBusServer
				natsCAPath             string
				mtlsNATSServerCertPath string
				mtlsNATSServerKeyPath  string
				mtlsNATSClientCert     tls.Certificate
			)
			BeforeEach(func() {
				natsTlsHost = "127.0.0.1"
				natsTlsPort = natsPort + 1000
				natsCAPath, mtlsNATSServerCertPath, mtlsNATSServerKeyPath, mtlsNATSClientCert = tls_helpers.GenerateCaAndMutualTlsCerts()

				natsTlsCmd = startNatsTls(natsTlsHost, natsTlsPort, natsCAPath, mtlsNATSServerCertPath, mtlsNATSServerKeyPath, "testuser", "testpw")

				tlsServers := []string{
					fmt.Sprintf(
						"nats://%s:%d",
						natsTlsHost,
						natsTlsPort,
					),
				}

				tlsOpts := nats.GetDefaultOptions()
				tlsOpts.Servers = tlsServers
				tlsOpts.User = "testuser"
				tlsOpts.Password = "testpw"

				spyClientTlsConfig, err := tlsconfig.Build(
					tlsconfig.WithInternalServiceDefaults(),
					tlsconfig.WithIdentity(mtlsNATSClientCert),
				).Client(
					tlsconfig.WithAuthorityFromFile(natsCAPath),
				)
				Expect(err).NotTo(HaveOccurred())

				tlsOpts.TLSConfig = spyClientTlsConfig

				tlsTestSpyClient, err := tlsOpts.Connect()
				Expect(err).ToNot(HaveOccurred())

				// Ensure nats server is listening before tests
				Eventually(func() string {
					connStatus := tlsTestSpyClient.Status()
					return fmt.Sprintf("%v", connStatus)
				}, 5*time.Second).Should(Equal("CONNECTED"))

				Expect(err).ShouldNot(HaveOccurred())

				tlsMessageBusServer := config.MessageBusServer{
					Host:     fmt.Sprintf("%s:%d", natsTlsHost, natsTlsPort),
					User:     "testuser",
					Password: "testpw",
				}

				tlsMessageBusServers = []config.MessageBusServer{tlsMessageBusServer}

				tlsTestSpyClient.Close()

				messageBusServers = []config.MessageBusServer{}
			})
			AfterEach(func() {

				err := natsTlsCmd.Process.Kill()
				Expect(err).NotTo(HaveOccurred())
				_, err = natsTlsCmd.Process.Wait()
				Expect(err).NotTo(HaveOccurred())
			})

			It("connects without error", func() {
				var (
					err             error
					clientTlsConfig *tls.Config
				)

				clientTlsConfig, err = tlsconfig.Build(
					tlsconfig.WithInternalServiceDefaults(),
					tlsconfig.WithIdentity(mtlsNATSClientCert),
				).Client(
					tlsconfig.WithAuthorityFromFile(natsCAPath),
				)
				Expect(err).NotTo(HaveOccurred())

				err = messageBus.Connect(tlsMessageBusServers, clientTlsConfig)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when no servers are provided", func() {
			BeforeEach(func() {
				messageBusServers = []config.MessageBusServer{}
			})

			It("returns error", func() {
				err := messageBus.Connect(messageBusServers, nil)
				Expect(err).Should(HaveOccurred())
			})
		})

		Context("when nats connection is successful", func() {
			BeforeEach(func() {
				err := messageBus.Connect(messageBusServers, nil)
				Expect(err).ShouldNot(HaveOccurred())
			})
			It("logs a message", func() {
				Eventually(logger).Should(gbytes.Say(`nats-connection-successful`))
				Eventually(logger).Should(gbytes.Say(natsHost))
			})
		})

		Context("when nats connection closes", func() {
			BeforeEach(func() {
				err := messageBus.Connect(messageBusServers, nil)
				Expect(err).ShouldNot(HaveOccurred())
				messageBus.Close()
			})

			It("logs a message", func() {
				Eventually(logger).Should(gbytes.Say(`nats-connection-disconnected`))
				Eventually(logger).Should(gbytes.Say(natsHost))
				Eventually(logger).Should(gbytes.Say(`nats-connection-closed`))
				Eventually(logger).Should(gbytes.Say(natsHost))
			})
		})
	})

	Describe("SendMessage", func() {
		const (
			topic             = "router.registrar"
			privateInstanceId = "some_id"
		)

		var (
			route config.Route
		)

		BeforeEach(func() {
			err := messageBus.Connect(messageBusServers, nil)
			Expect(err).ShouldNot(HaveOccurred())

			port := uint16(12345)

			route = config.Route{
				Name:                "some_name",
				Host:                "some_host",
				Port:                &port,
				TLSPort:             &port,
				URIs:                []string{"uri1", "uri2"},
				RouteServiceUrl:     "https://rs.example.com",
				Tags:                map[string]string{"tag1": "val1", "tag2": "val2"},
				ServerCertDomainSAN: "cf.cert.internal",
			}
		})

		It("send messages", func() {
			registered := make(chan string)
			testSpyClient.Subscribe(topic, func(msg *nats.Msg) {
				registered <- string(msg.Data)
			})

			// Wait for the nats library to register our callback.
			// We use a sleep because there's no way to know that the callback was
			// registered successfully (e.g. they don't provide a channel)
			time.Sleep(20 * time.Millisecond)

			err := messageBus.SendMessage(topic, route, privateInstanceId)
			Expect(err).ShouldNot(HaveOccurred())

			// Assert that we got the right message
			var receivedMessage string
			Eventually(registered, 2).Should(Receive(&receivedMessage))

			expectedRegistryMessage := messagebus.Message{
				URIs:                route.URIs,
				Host:                route.Host,
				Port:                route.Port,
				TLSPort:             route.TLSPort,
				RouteServiceUrl:     route.RouteServiceUrl,
				Tags:                route.Tags,
				ServerCertDomainSAN: "cf.cert.internal",
				AvailabilityZone:    "some-az",
			}

			var registryMessage messagebus.Message
			err = json.Unmarshal([]byte(receivedMessage), &registryMessage)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(registryMessage.URIs).To(Equal(expectedRegistryMessage.URIs))
			Expect(registryMessage.Port).To(Equal(expectedRegistryMessage.Port))
			Expect(registryMessage.Host).To(Equal(expectedRegistryMessage.Host))
			Expect(registryMessage.Protocol).To(BeEmpty())
			Expect(registryMessage.RouteServiceUrl).To(Equal(expectedRegistryMessage.RouteServiceUrl))
			Expect(registryMessage.Tags).To(Equal(expectedRegistryMessage.Tags))
			Expect(registryMessage.AvailabilityZone).To(Equal(expectedRegistryMessage.AvailabilityZone))
		})

		Context("when the connection is already closed", func() {
			BeforeEach(func() {
				err := messageBus.Connect(messageBusServers, nil)
				Expect(err).ShouldNot(HaveOccurred())

				messageBus.Close()
			})

			It("returns error", func() {
				err := messageBus.SendMessage(topic, route, privateInstanceId)
				Expect(err).Should(HaveOccurred())
			})
		})
	})

	Describe("SendMessage for h2 route", func() {
		const (
			topic             = "router.registrar"
			privateInstanceId = "some_id"
		)

		var (
			route config.Route
		)

		BeforeEach(func() {
			err := messageBus.Connect(messageBusServers, nil)
			Expect(err).ShouldNot(HaveOccurred())

			port := uint16(12345)

			route = config.Route{
				Name:                "some_name",
				Port:                &port,
				Host:                "some_host",
				TLSPort:             &port,
				Protocol:            "http2",
				URIs:                []string{"uri1", "uri2"},
				RouteServiceUrl:     "https://rs.example.com",
				Tags:                map[string]string{"tag1": "val1", "tag2": "val2"},
				ServerCertDomainSAN: "cf.cert.internal",
			}
		})

		It("send messages", func() {
			registered := make(chan string)
			testSpyClient.Subscribe(topic, func(msg *nats.Msg) {
				registered <- string(msg.Data)
			})

			// Wait for the nats library to register our callback.
			// We use a sleep because there's no way to know that the callback was
			// registered successfully (e.g. they don't provide a channel)
			time.Sleep(20 * time.Millisecond)

			err := messageBus.SendMessage(topic, route, privateInstanceId)
			Expect(err).ShouldNot(HaveOccurred())

			// Assert that we got the right message
			var receivedMessage string
			Eventually(registered, 2).Should(Receive(&receivedMessage))

			expectedRegistryMessage := messagebus.Message{
				URIs:                route.URIs,
				Host:                route.Host,
				Port:                route.Port,
				Protocol:            route.Protocol,
				TLSPort:             route.TLSPort,
				RouteServiceUrl:     route.RouteServiceUrl,
				Tags:                route.Tags,
				ServerCertDomainSAN: "cf.cert.internal",
				AvailabilityZone:    "some-az",
			}

			var registryMessage messagebus.Message
			err = json.Unmarshal([]byte(receivedMessage), &registryMessage)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(registryMessage.URIs).To(Equal(expectedRegistryMessage.URIs))
			Expect(registryMessage.Port).To(Equal(expectedRegistryMessage.Port))
			Expect(registryMessage.Host).To(Equal(expectedRegistryMessage.Host))
			Expect(registryMessage.Protocol).To(Equal(expectedRegistryMessage.Protocol))
			Expect(registryMessage.RouteServiceUrl).To(Equal(expectedRegistryMessage.RouteServiceUrl))
			Expect(registryMessage.Tags).To(Equal(expectedRegistryMessage.Tags))
		})
	})
	Describe("SendMessage with per-route options", func() {
		const (
			topic             = "router.registrar"
			privateInstanceId = "some_id"
		)

		var (
			route config.Route
		)

		BeforeEach(func() {
			err := messageBus.Connect(messageBusServers, nil)
			Expect(err).ShouldNot(HaveOccurred())

			port := uint16(12345)

			route = config.Route{
				Name:                "some_name",
				TLSPort:             &port,
				Host:                "some_host",
				URIs:                []string{"uri1", "uri2"},
				ServerCertDomainSAN: "cf.cert.internal",
				Options:             &config.Options{LoadBalancingAlgorithm: config.LeastConns},
			}
		})

		It("sends messages", func() {
			registered := make(chan string)
			testSpyClient.Subscribe(topic, func(msg *nats.Msg) {
				registered <- string(msg.Data)
			})

			// Wait for the nats library to register our callback.
			// We use a sleep because there's no way to know that the callback was
			// registered successfully (e.g. they don't provide a channel)
			time.Sleep(20 * time.Millisecond)

			err := messageBus.SendMessage(topic, route, privateInstanceId)
			Expect(err).ShouldNot(HaveOccurred())

			// Assert that we got the right message
			var receivedMessage string
			Eventually(registered, 2).Should(Receive(&receivedMessage))

			expectedRegistryMessage := messagebus.Message{
				URIs:                route.URIs,
				Host:                route.Host,
				TLSPort:             route.TLSPort,
				ServerCertDomainSAN: "cf.cert.internal",
				AvailabilityZone:    "some-az",
				Options:             map[string]string{"loadbalancing": string(route.Options.LoadBalancingAlgorithm)},
			}

			var registryMessage messagebus.Message
			err = json.Unmarshal([]byte(receivedMessage), &registryMessage)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(registryMessage.URIs).To(Equal(expectedRegistryMessage.URIs))
			Expect(registryMessage.Protocol).To(BeEmpty())
			Expect(registryMessage.AvailabilityZone).To(Equal(expectedRegistryMessage.AvailabilityZone))

			Expect(registryMessage.Options).To(Equal(expectedRegistryMessage.Options))
		})

		Context("when the connection is already closed", func() {
			BeforeEach(func() {
				err := messageBus.Connect(messageBusServers, nil)
				Expect(err).ShouldNot(HaveOccurred())

				messageBus.Close()
			})

			It("returns error", func() {
				err := messageBus.SendMessage(topic, route, privateInstanceId)
				Expect(err).Should(HaveOccurred())
			})
		})
	})
})

func startNats(host string, port int, username, password string) *exec.Cmd {
	fmt.Fprintf(GinkgoWriter, "Starting nats-server on port %d\n", port)

	natsServer, exists := os.LookupEnv("NATS_SERVER_BINARY")
	if !exists {
		fmt.Println("You need nats-server installed and set NATS_SERVER_BINARY env variable")
		os.Exit(1)
	}

	cmd := exec.Command(
		natsServer,
		"-p", strconv.Itoa(port),
		"--user", username,
		"--pass", password)

	err := cmd.Start()
	if err != nil {
		fmt.Printf("nats-server failed to start: %v\n", err)
	}

	natsTimeout := 10 * time.Second
	natsPollingInterval := 20 * time.Millisecond
	Eventually(func() error {
		_, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
		return err
	}, natsTimeout, natsPollingInterval).Should(Succeed())

	fmt.Fprintf(GinkgoWriter, "nats-server running on port %d\n", port)
	return cmd
}

func startNatsTls(host string, port int, caFile, certFile, keyFile, username, password string) *exec.Cmd {
	fmt.Fprintf(GinkgoWriter, "Starting TLS nats-server on port %d\n", port)

	natsServer, exists := os.LookupEnv("NATS_SERVER_BINARY")
	if !exists {
		fmt.Println("You need nats-server installed and set NATS_SERVER_BINARY env variable")
		os.Exit(1)
	}
	cmd := exec.Command(
		natsServer,
		"-p", strconv.Itoa(port),
		"--tlsverify",
		"--tlscacert", caFile,
		"--tlscert", certFile,
		"--tlskey", keyFile,
		"--user", username,
		"--pass", password,
	)

	err := cmd.Start()
	if err != nil {
		fmt.Printf("TLS nats-server failed to start: %v\n", err)
	}

	natsTimeout := 10 * time.Second
	natsPollingInterval := 20 * time.Millisecond
	Eventually(func() error {
		_, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
		return err
	}, natsTimeout, natsPollingInterval).Should(Succeed())

	fmt.Fprintf(GinkgoWriter, "TLS nats-server running on port %d\n", port)
	return cmd
}
