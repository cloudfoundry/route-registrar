package config_test

import (
	"fmt"
	"time"

	"github.com/cloudfoundry-incubator/route-registrar/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	var (
		configSchema config.ConfigSchema

		registrationInterval0String string
		registrationInterval1String string

		registrationInterval0 time.Duration
		registrationInterval1 time.Duration

		routeName1 string
		routeName2 string
	)

	BeforeEach(func() {
		registrationInterval0String = "20s"
		registrationInterval1String = "10s"

		registrationInterval0 = 20 * time.Second
		registrationInterval1 = 10 * time.Second

		routeName1 = "route-1"
		routeName2 = "route-2"

		configSchema = config.ConfigSchema{
			MessageBusServers: []config.MessageBusServerSchema{
				config.MessageBusServerSchema{
					Host:     "some-host",
					User:     "some-user",
					Password: "some-password",
				},
				config.MessageBusServerSchema{
					Host:     "another-host",
					User:     "another-user",
					Password: "another-password",
				},
			},
			Routes: []config.RouteSchema{
				{
					Name:                 routeName1,
					Port:                 3000,
					RegistrationInterval: registrationInterval0String,
					URIs:                 []string{"my-app.my-domain.com"},
				},
				{
					Name:                 routeName2,
					Port:                 3001,
					RegistrationInterval: registrationInterval1String,
					URIs:                 []string{"my-other-app.my-domain.com"},
				},
			},
			Host: "127.0.0.1",
		}
	})

	Describe("ToConfig", func() {
		It("returns a Config object and no error", func() {
			c, err := configSchema.ToConfig()
			Expect(err).ToNot(HaveOccurred())

			expectedC := &config.Config{
				Host: configSchema.Host,
				MessageBusServers: []config.MessageBusServer{
					{
						Host:     configSchema.MessageBusServers[0].Host,
						User:     configSchema.MessageBusServers[0].User,
						Password: configSchema.MessageBusServers[0].Password,
					},
					{
						Host:     configSchema.MessageBusServers[1].Host,
						User:     configSchema.MessageBusServers[1].User,
						Password: configSchema.MessageBusServers[1].Password,
					},
				},
				Routes: []config.Route{
					{
						Name:                 routeName1,
						Port:                 configSchema.Routes[0].Port,
						RegistrationInterval: registrationInterval0,
						URIs:                 configSchema.Routes[0].URIs,
					},
					{
						Name:                 routeName2,
						Port:                 configSchema.Routes[1].Port,
						RegistrationInterval: registrationInterval1,
						URIs:                 configSchema.Routes[1].URIs,
					},
				},
			}

			Expect(c).To(Equal(expectedC))
		})

		Describe("Routes", func() {

			Context("when the config input includes tags", func() {
				BeforeEach(func() {
					configSchema.Routes[0].Tags = map[string]string{"key": "value"}
					configSchema.Routes[1].Tags = map[string]string{"key": "value2"}
				})

				It("includes them in the config", func() {
					c, err := configSchema.ToConfig()
					Expect(err).ToNot(HaveOccurred())

					Expect(c.Routes[0].Tags).Should(Equal(configSchema.Routes[0].Tags))
					Expect(c.Routes[1].Tags).Should(Equal(configSchema.Routes[1].Tags))
				})
			})

			Context("healthcheck is provided", func() {
				BeforeEach(func() {
					configSchema.Routes[0].HealthCheck = &config.HealthCheckSchema{
						Name:       "my healthcheck",
						ScriptPath: "/some/script/path",
					}
				})

				Context("The healthcheck timeout is empty", func() {
					BeforeEach(func() {
						configSchema.Routes[0].HealthCheck.Timeout = ""
					})

					It("defaults the healthcheck timeout to half the registration interval", func() {
						c, err := configSchema.ToConfig()
						Expect(err).NotTo(HaveOccurred())

						Expect(c.Routes[0].HealthCheck.Timeout).To(Equal(registrationInterval0 / 2))
					})
				})

				Context("and the healthcheck timeout is provided", func() {
					BeforeEach(func() {
						configSchema.Routes[0].HealthCheck.Timeout = "11s"
					})

					It("sets the healthcheck timeout on the config", func() {
						c, err := configSchema.ToConfig()
						Expect(err).NotTo(HaveOccurred())

						Expect(err).NotTo(HaveOccurred())
						Expect(c.Routes[0].HealthCheck.Timeout).To(Equal(11 * time.Second))
					})
				})
			})
		})
	})

	Describe("Handling errors", func() {
		Describe("on the registration interval", func() {
			Context("The registration interval is empty", func() {
				BeforeEach(func() {
					configSchema.Routes[0].RegistrationInterval = ""
				})

				It("returns an error", func() {
					c, err := configSchema.ToConfig()
					Expect(c).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("registration_interval not provided"))
				})
			})

			Context("The registration interval is zero", func() {
				BeforeEach(func() {
					configSchema.Routes[0].RegistrationInterval = "0s"
				})

				It("returns an error", func() {
					c, err := configSchema.ToConfig()
					Expect(c).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("route 0 has invalid registration_interval: interval must be greater than 0"))
				})
			})

			Context("The registration interval is negative", func() {
				BeforeEach(func() {
					configSchema.Routes[0].RegistrationInterval = "-1s"
				})

				It("returns an error", func() {
					_, err := configSchema.ToConfig()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("route 0 has invalid registration_interval: interval must be greater than 0"))
				})
			})

			Context("When the registration interval has no units", func() {
				BeforeEach(func() {
					configSchema.Routes[0].RegistrationInterval = "1"
				})

				It("returns an error", func() {
					c, err := configSchema.ToConfig()
					Expect(c).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("route 0 has invalid registration_interval: time: missing unit in duration 1"))
				})
			})

			Context("When the registration interval is not parsable", func() {
				BeforeEach(func() {
					configSchema.Routes[0].RegistrationInterval = "asdf"
				})

				It("returns an error", func() {
					c, err := configSchema.ToConfig()
					Expect(c).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("route 0 has invalid registration_interval: time: invalid duration asdf"))
				})
			})
		})

		Describe("on route names", func() {
			Context("when the config input does not include a name", func() {
				BeforeEach(func() {
					configSchema.Routes[0].Name = ""
				})

				It("returns an error", func() {
					c, err := configSchema.ToConfig()
					Expect(c).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("name must be provided for route 0"))
				})
			})
		})

		Describe("on the healthcheck timeout, assuming healthcheck is provided", func() {
			BeforeEach(func() {
				configSchema.Routes[0].HealthCheck = &config.HealthCheckSchema{
					Name:       "my healthcheck",
					ScriptPath: "/some/script/path",
				}
			})

			Context("The healthcheck timeout is zero", func() {
				BeforeEach(func() {
					configSchema.Routes[0].HealthCheck.Timeout = "0s"
				})

				It("returns an error", func() {
					c, err := configSchema.ToConfig()
					Expect(c).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Invalid healthcheck timeout"))
				})
			})

			Context("The healthcheck timeout is negative", func() {
				BeforeEach(func() {
					configSchema.Routes[0].HealthCheck.Timeout = "-1s"
				})

				It("returns an error", func() {
					_, err := configSchema.ToConfig()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Invalid healthcheck timeout: -1s"))
				})
			})

			Context("When the healthcheck timeout has no units", func() {
				BeforeEach(func() {
					configSchema.Routes[0].HealthCheck.Timeout = "1"
				})

				It("returns an error", func() {
					c, err := configSchema.ToConfig()
					Expect(c).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Invalid healthcheck timeout: time: missing unit in duration 1"))
				})
			})

			Context("When the healthcheck is equal to the registration interval", func() {
				BeforeEach(func() {
					configSchema.Routes[0].HealthCheck.Timeout = configSchema.Routes[0].RegistrationInterval
				})

				It("returns an error", func() {
					c, err := configSchema.ToConfig()
					Expect(err).To(HaveOccurred())
					Expect(c).To(BeNil())
					Expect(err.Error()).To(ContainSubstring(
						fmt.Sprintf(
							"Invalid healthcheck timeout: %s must be less than registration interval: %s",
							configSchema.Routes[0].HealthCheck.Timeout,
							configSchema.Routes[0].RegistrationInterval,
						),
					))
				})
			})

			Context("When the healthcheck is greater than the registration interval", func() {
				BeforeEach(func() {
					configSchema.Routes[0].HealthCheck.Timeout = "59s"
				})

				It("returns an error", func() {
					c, err := configSchema.ToConfig()
					Expect(err).To(HaveOccurred())
					Expect(c).To(BeNil())
					Expect(err.Error()).To(ContainSubstring(
						fmt.Sprintf(
							"Invalid healthcheck timeout: %s must be less than registration interval: %s",
							configSchema.Routes[0].HealthCheck.Timeout,
							configSchema.Routes[0].RegistrationInterval,
						),
					))
				})
			})

			Context("When the healthcheck timeout is not parsable", func() {
				BeforeEach(func() {
					configSchema.Routes[0].HealthCheck.Timeout = "asdf"
				})

				It("returns an error", func() {
					c, err := configSchema.ToConfig()
					Expect(c).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Invalid healthcheck timeout: time: invalid duration asdf"))
				})
			})
		})

		Describe("on the host", func() {
			Context("when the host is empty", func() {
				BeforeEach(func() {
					configSchema.Host = ""
				})

				It("returns an error", func() {
					c, err := configSchema.ToConfig()
					Expect(c).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("host required"))
				})
			})
		})

		Describe("on the message bus servers", func() {
			Context("when message bus servers are empty", func() {
				BeforeEach(func() {
					configSchema.MessageBusServers = []config.MessageBusServerSchema{}
				})

				It("returns an error", func() {
					c, err := configSchema.ToConfig()
					Expect(c).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("message_bus_servers must have at least one entry"))
				})
			})
		})

		Context("when there are many errors", func() {
			BeforeEach(func() {
				configSchema.Host = ""
				configSchema.MessageBusServers = []config.MessageBusServerSchema{}
				configSchema.Routes[0].RegistrationInterval = ""
				configSchema.Routes[0].Name = ""
				configSchema.Routes[1].RegistrationInterval = "asdf"
				configSchema.Routes[1].Name = ""

				configSchema.Routes = append(configSchema.Routes, config.RouteSchema{
					RegistrationInterval: "10",
					Name:                 "",
				})

				configSchema.Routes = append(configSchema.Routes, config.RouteSchema{
					RegistrationInterval: "-10s",
					Name:                 "",
				})
			})

			It("aggregates the errors (except on the healthcheck)", func() {
				c, err := configSchema.ToConfig()
				Expect(c).To(BeNil())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("host required"))
				Expect(err.Error()).To(ContainSubstring("message_bus_servers must have at least one entry"))
				Expect(err.Error()).To(ContainSubstring("registration_interval not provided for route 0"))
				Expect(err.Error()).To(ContainSubstring("name must be provided for route 0"))
				Expect(err.Error()).To(ContainSubstring("route 1 has invalid registration_interval: time: invalid duration asdf"))
				Expect(err.Error()).To(ContainSubstring("name must be provided for route 1"))
				Expect(err.Error()).To(ContainSubstring("route 2 has invalid registration_interval: time: missing unit in duration 10"))
				Expect(err.Error()).To(ContainSubstring("route 3 has invalid registration_interval: interval must be greater than 0"))

				Expect(err.Error()).NotTo(ContainSubstring("route 0 has invalid registration_interval: time: invalid duration"))
				Expect(err.Error()).NotTo(ContainSubstring("route 2 has invalid registration_interval: interval must be greater than 0"))
			})
		})
	})
})
