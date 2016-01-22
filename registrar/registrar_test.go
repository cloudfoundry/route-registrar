package registrar_test

import (
	"fmt"
	"os"

	"github.com/apcera/nats"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry-incubator/route-registrar/config"
	healthchecker_fakes "github.com/cloudfoundry-incubator/route-registrar/healthchecker/fakes"
	messagebus_fakes "github.com/cloudfoundry-incubator/route-registrar/messagebus/fakes"
	"github.com/cloudfoundry-incubator/route-registrar/registrar"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Registrar.RegisterRoutes", func() {
	var (
		fakeMessageBus *messagebus_fakes.FakeMessageBus

		natsHost     string
		natsUsername string
		natsPassword string

		rrConfig config.Config

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

		logger = lagertest.NewTestLogger("Registrar test")
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

		fakeHealthChecker = new(healthchecker_fakes.FakeHealthChecker)
		fakeMessageBus = new(messagebus_fakes.FakeMessageBus)

		r = registrar.NewRegistrar(rrConfig, fakeHealthChecker, logger, fakeMessageBus)
	})

	It("connects to messagebus", func() {
		runStatus := make(chan error)
		go func() {
			runStatus <- r.Run(signals, ready)
		}()
		<-ready

		Expect(fakeMessageBus.ConnectCallCount()).To(Equal(1))
	})

	It("unregisters on shutdown", func() {
		runStatus := make(chan error)
		go func() {
			runStatus <- r.Run(signals, ready)
		}()
		<-ready

		close(signals)
		err := <-runStatus
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(fakeMessageBus.SendMessageCallCount, 3).Should(BeNumerically(">", 1))

		subject, host, route, privateInstanceId := fakeMessageBus.SendMessageArgsForCall(0)
		Expect(subject).To(Equal("router.unregister"))
		Expect(host).To(Equal(rrConfig.Host))
		Expect(route.Name).To(Equal(rrConfig.Routes[0].Name))
		Expect(route.URIs).To(Equal(rrConfig.Routes[0].URIs))
		Expect(route.Port).To(Equal(rrConfig.Routes[0].Port))
		Expect(privateInstanceId).NotTo(Equal(""))

		subject, host, route, privateInstanceId = fakeMessageBus.SendMessageArgsForCall(1)
		Expect(subject).To(Equal("router.unregister"))
		Expect(host).To(Equal(rrConfig.Host))
		Expect(route.Name).To(Equal(rrConfig.Routes[1].Name))
		Expect(route.URIs).To(Equal(rrConfig.Routes[1].URIs))
		Expect(route.Port).To(Equal(rrConfig.Routes[1].Port))
		Expect(privateInstanceId).NotTo(Equal(""))
	})

	It("periodically registers all URIs for all routes", func() {
		runStatus := make(chan error)
		go func() {
			runStatus <- r.Run(signals, ready)
		}()
		<-ready

		Eventually(fakeMessageBus.SendMessageCallCount, 3).Should(BeNumerically(">", 1))

		subject, host, route, privateInstanceId := fakeMessageBus.SendMessageArgsForCall(0)
		Expect(subject).To(Equal("router.register"))
		Expect(host).To(Equal(rrConfig.Host))
		Expect(route.Name).To(Equal(rrConfig.Routes[0].Name))
		Expect(route.URIs).To(Equal(rrConfig.Routes[0].URIs))
		Expect(route.Port).To(Equal(rrConfig.Routes[0].Port))
		Expect(privateInstanceId).NotTo(Equal(""))

		subject, host, route, privateInstanceId = fakeMessageBus.SendMessageArgsForCall(1)
		Expect(subject).To(Equal("router.register"))
		Expect(host).To(Equal(rrConfig.Host))
		Expect(route.Name).To(Equal(rrConfig.Routes[1].Name))
		Expect(route.URIs).To(Equal(rrConfig.Routes[1].URIs))
		Expect(route.Port).To(Equal(rrConfig.Routes[1].Port))
		Expect(privateInstanceId).NotTo(Equal(""))
	})

	Context("given a healthcheck", func() {
		var scriptPath string

		BeforeEach(func() {
			scriptPath = "/path/to/some/script/"

			rrConfig.Routes[0].HealthCheck = &config.HealthCheck{
				Name:       "My Healthcheck process",
				ScriptPath: scriptPath,
			}
			rrConfig.Routes[1].HealthCheck = &config.HealthCheck{
				Name:       "My Healthcheck process 2",
				ScriptPath: scriptPath,
			}

			r = registrar.NewRegistrar(rrConfig, fakeHealthChecker, logger, fakeMessageBus)
		})

		Context("and the healthcheck succeeds", func() {
			BeforeEach(func() {
				fakeHealthChecker.CheckReturns(true, nil)

				r = registrar.NewRegistrar(rrConfig, fakeHealthChecker, logger, fakeMessageBus)
			})

			It("registers routes", func() {
				runStatus := make(chan error)
				go func() {
					runStatus <- r.Run(signals, ready)
				}()
				<-ready

				Eventually(fakeMessageBus.SendMessageCallCount, 3).Should(BeNumerically(">", 1))

				subject, host, route, privateInstanceId := fakeMessageBus.SendMessageArgsForCall(0)
				Expect(subject).To(Equal("router.register"))
				Expect(host).To(Equal(rrConfig.Host))
				Expect(route.Name).To(Equal(rrConfig.Routes[0].Name))
				Expect(route.URIs).To(Equal(rrConfig.Routes[0].URIs))
				Expect(route.Port).To(Equal(rrConfig.Routes[0].Port))
				Expect(privateInstanceId).NotTo(Equal(""))

				subject, host, route, privateInstanceId = fakeMessageBus.SendMessageArgsForCall(1)
				Expect(subject).To(Equal("router.register"))
				Expect(host).To(Equal(rrConfig.Host))
				Expect(route.Name).To(Equal(rrConfig.Routes[1].Name))
				Expect(route.URIs).To(Equal(rrConfig.Routes[1].URIs))
				Expect(route.Port).To(Equal(rrConfig.Routes[1].Port))
				Expect(privateInstanceId).NotTo(Equal(""))
			})
		})

		Context("when the healthcheck fails", func() {
			BeforeEach(func() {
				fakeHealthChecker.CheckReturns(false, nil)

				r = registrar.NewRegistrar(rrConfig, fakeHealthChecker, logger, fakeMessageBus)
			})

			It("unregisters routes", func() {
				runStatus := make(chan error)
				go func() {
					runStatus <- r.Run(signals, ready)
				}()
				<-ready

				Eventually(fakeMessageBus.SendMessageCallCount, 3).Should(BeNumerically(">", 1))

				subject, host, route, privateInstanceId := fakeMessageBus.SendMessageArgsForCall(0)
				Expect(subject).To(Equal("router.unregister"))
				Expect(host).To(Equal(rrConfig.Host))
				Expect(route.Name).To(Equal(rrConfig.Routes[0].Name))
				Expect(route.URIs).To(Equal(rrConfig.Routes[0].URIs))
				Expect(route.Port).To(Equal(rrConfig.Routes[0].Port))
				Expect(privateInstanceId).NotTo(Equal(""))

				subject, host, route, privateInstanceId = fakeMessageBus.SendMessageArgsForCall(1)
				Expect(subject).To(Equal("router.unregister"))
				Expect(host).To(Equal(rrConfig.Host))
				Expect(route.Name).To(Equal(rrConfig.Routes[1].Name))
				Expect(route.URIs).To(Equal(rrConfig.Routes[1].URIs))
				Expect(route.Port).To(Equal(rrConfig.Routes[1].Port))
				Expect(privateInstanceId).NotTo(Equal(""))
			})
		})

		Context("when the healthcheck errors", func() {
			var expectedErr error

			BeforeEach(func() {
				expectedErr = fmt.Errorf("boom")
				fakeHealthChecker.CheckReturns(true, expectedErr)

				r = registrar.NewRegistrar(rrConfig, fakeHealthChecker, logger, fakeMessageBus)
			})

			It("unregisters routes", func() {
				runStatus := make(chan error)
				go func() {
					runStatus <- r.Run(signals, ready)
				}()
				<-ready

				Eventually(fakeMessageBus.SendMessageCallCount, 3).Should(BeNumerically(">", 1))

				subject, host, route, privateInstanceId := fakeMessageBus.SendMessageArgsForCall(0)
				Expect(subject).To(Equal("router.unregister"))
				Expect(host).To(Equal(rrConfig.Host))
				Expect(route.Name).To(Equal(rrConfig.Routes[0].Name))
				Expect(route.URIs).To(Equal(rrConfig.Routes[0].URIs))
				Expect(route.Port).To(Equal(rrConfig.Routes[0].Port))
				Expect(privateInstanceId).NotTo(Equal(""))

				subject, host, route, privateInstanceId = fakeMessageBus.SendMessageArgsForCall(1)
				Expect(subject).To(Equal("router.unregister"))
				Expect(host).To(Equal(rrConfig.Host))
				Expect(route.Name).To(Equal(rrConfig.Routes[1].Name))
				Expect(route.URIs).To(Equal(rrConfig.Routes[1].URIs))
				Expect(route.Port).To(Equal(rrConfig.Routes[1].Port))
				Expect(privateInstanceId).NotTo(Equal(""))
			})
		})
	})
})
