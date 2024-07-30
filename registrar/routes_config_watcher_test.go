package registrar_test

import (
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"code.cloudfoundry.org/route-registrar/config"
	"code.cloudfoundry.org/route-registrar/registrar"
)

var _ = FDescribe("RoutesConfigWatcher", func() {
	var (
		routesConfigWatcher ifrit.Runner
		logger              lager.Logger

		process ifrit.Process

		route1Schema, route2Schema, route3Schema, route4Schema config.RouteSchema
		route1, route2, route3, route4                         config.Route

		routesDiscovered chan config.Route
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("Registrar test")
		glob := fmt.Sprintf("%s/config-*.yml*", os.TempDir())
		routesDiscovered = make(chan config.Route)

		routesConfigWatcher = registrar.NewRoutesConfigWatcher(logger, 100*time.Millisecond, []string{glob}, func(route config.Route) {
			routesDiscovered <- route
		})

		port := 8080
		route1 = config.Route{
			Name:                 "some-route-1",
			Port:                 &port,
			RegistrationInterval: time.Second,
			URIs:                 []string{"some-route-1.apps.com"},
			Tags:                 map[string]string{},
		}
		route1Schema = config.RouteSchema{
			Name:                 "some-route-1",
			Port:                 &port,
			RegistrationInterval: "1s",
			URIs:                 []string{"some-route-1.apps.com"},
		}

		route2 = config.Route{
			Name:                 "some-route-2",
			Port:                 &port,
			RegistrationInterval: 2 * time.Second,
			URIs:                 []string{"some-route-2.apps.com"},
			Tags:                 map[string]string{},
		}
		route2Schema = config.RouteSchema{
			Name:                 "some-route-2",
			Port:                 &port,
			RegistrationInterval: "2s",
			URIs:                 []string{"some-route-2.apps.com"},
		}

		route3 = config.Route{
			Name:                 "some-route-3",
			Port:                 &port,
			RegistrationInterval: 3 * time.Second,
			URIs:                 []string{"some-route-3.apps.com"},
			Tags:                 map[string]string{},
		}
		route3Schema = config.RouteSchema{
			Name:                 "some-route-3",
			Port:                 &port,
			RegistrationInterval: "3s",
			URIs:                 []string{"some-route-3.apps.com"},
		}

		route4 = config.Route{
			Name:                 "some-route-4",
			Port:                 &port,
			RegistrationInterval: 3 * time.Second,
			URIs:                 []string{"some-route-4.apps.com"},
			Tags:                 map[string]string{},
		}
		route4Schema = config.RouteSchema{
			Name:                 "some-route-4",
			Port:                 &port,
			RegistrationInterval: "3s",
			URIs:                 []string{"some-route-4.apps.com"},
		}
	})

	JustBeforeEach(func() {
		process = ifrit.Background(routesConfigWatcher)
	})

	AfterEach(func() {
		process.Signal(os.Interrupt)
		Eventually(process.Wait(), 5*time.Second).Should(Receive())
	})

	Context("when directory has no files", func() {})

	Context("when directory has config files already", func() {
		var (
			cfgFile1 *os.File
			cfgFile2 *os.File
		)

		BeforeEach(func() {
			var err error
			cfgFile1, err = os.CreateTemp(os.TempDir(), "config-1.yml")
			Expect(err).NotTo(HaveOccurred())

			cfgFile2, err = os.CreateTemp(os.TempDir(), "config-2.yml")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			os.Remove(cfgFile1.Name())
			os.Remove(cfgFile2.Name())
		})

		Context("when config files have routes", func() {
			BeforeEach(func() {
				routesBytes1, err := yaml.Marshal(registrar.RoutesConfigSchema{Routes: []config.RouteSchema{route1Schema, route2Schema}})
				Expect(err).NotTo(HaveOccurred())
				_, err = cfgFile1.Write(routesBytes1)
				Expect(err).NotTo(HaveOccurred())

				routesBytes2, err := yaml.Marshal(registrar.RoutesConfigSchema{Routes: []config.RouteSchema{route3Schema}})
				Expect(err).NotTo(HaveOccurred())
				_, err = cfgFile2.Write(routesBytes2)
				Expect(err).NotTo(HaveOccurred())
			})

			It("loads all routes from the config", func() {
				var receivedRoute config.Route
				Eventually(routesDiscovered).Should(Receive(&receivedRoute))
				Expect(receivedRoute).To(Equal(route1))
				Eventually(routesDiscovered).Should(Receive(&receivedRoute))
				Expect(receivedRoute).To(Equal(route2))
				Eventually(routesDiscovered).Should(Receive(&receivedRoute))
				Expect(receivedRoute).To(Equal(route3))
			})

			Context("when config file is updated and new route is added", func() {
				FIt("calls routeDiscovered on that route", func() {
					var receivedRoute config.Route
					Eventually(routesDiscovered).Should(Receive(&receivedRoute))
					Expect(receivedRoute).To(Equal(route1))
					Eventually(routesDiscovered).Should(Receive(&receivedRoute))
					Expect(receivedRoute).To(Equal(route2))
					Eventually(routesDiscovered).Should(Receive(&receivedRoute))
					Expect(receivedRoute).To(Equal(route3))

					routesBytes2, err := yaml.Marshal(registrar.RoutesConfigSchema{Routes: []config.RouteSchema{route3Schema, route4Schema}})
					Expect(err).NotTo(HaveOccurred())
					err = cfgFile2.Truncate(0)
					Expect(err).NotTo(HaveOccurred())
					_, err = cfgFile2.Seek(0, 0)
					Expect(err).NotTo(HaveOccurred())
					_, err = cfgFile2.Write(routesBytes2)
					Expect(err).NotTo(HaveOccurred())

					Eventually(routesDiscovered).Should(Receive(&receivedRoute))
					Expect(receivedRoute).To(Equal(route4))
				})
			})

			Context("when config file is removed", func() {
				It("removes routes from that config file", func() {

				})
			})
		})

		Context("when config file is in wrong format", func() {
			BeforeEach(func() {
				_, err := cfgFile1.Write([]byte(`invalid`))
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				os.Remove(cfgFile1.Name())
			})

			It("logs an error and continues scaning", func() {
				Eventually(logger).Should(gbytes.Say("invalid routes config file at %s", cfgFile1.Name()))
			})
		})
	})

	It("works", func() {

	})
})
