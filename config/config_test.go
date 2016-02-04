package config_test

import (
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

	Describe("Validate", func() {
		It("returns a Config object and no error", func() {
			c, err := configSchema.Validate()
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
					c, err := configSchema.Validate()
					Expect(err).ToNot(HaveOccurred())

					Expect(c.Routes[0].Tags).Should(Equal(configSchema.Routes[0].Tags))
					Expect(c.Routes[1].Tags).Should(Equal(configSchema.Routes[1].Tags))
				})
			})

			Context("when the config input does not include a name", func() {
				BeforeEach(func() {
					configSchema.Routes[0].Name = ""
				})

				It("includes them in the config", func() {
					c, err := configSchema.Validate()
					Expect(c).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("name for route must be provided"))
				})
			})

			Context("The registration interval is empty", func() {
				BeforeEach(func() {
					configSchema.Routes[0].RegistrationInterval = ""
				})

				It("returns an error", func() {
					c, err := configSchema.Validate()
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
					c, err := configSchema.Validate()
					Expect(c).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Invalid registration_interval"))
				})
			})

			Context("The registration interval is negative", func() {
				BeforeEach(func() {
					configSchema.Routes[0].RegistrationInterval = "-1s"
				})

				It("returns an error", func() {
					_, err := configSchema.Validate()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Invalid registration_interval: -1"))
				})
			})

			Context("When the registration interval has no units", func() {
				BeforeEach(func() {
					configSchema.Routes[0].RegistrationInterval = "1"
				})

				It("returns an error", func() {
					c, err := configSchema.Validate()
					Expect(c).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Invalid registration_interval: time: missing unit in duration 1"))
				})
			})

			Context("When the registration interval is not parsable", func() {
				BeforeEach(func() {
					configSchema.Routes[0].RegistrationInterval = "asdf"
				})

				It("returns an error", func() {
					c, err := configSchema.Validate()
					Expect(c).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Invalid registration_interval: time: invalid duration asdf"))
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
						c, err := configSchema.Validate()
						Expect(err).NotTo(HaveOccurred())

						Expect(c.Routes[0].HealthCheck.Timeout).To(Equal(registrationInterval0 / 2))
					})
				})

				Context("and the healthcheck timeout is provided", func() {
					BeforeEach(func() {
						configSchema.Routes[0].HealthCheck.Timeout = "11s"
					})

					It("sets the healthcheck timeout on the config", func() {
						c, err := configSchema.Validate()
						Expect(err).NotTo(HaveOccurred())

						Expect(err).NotTo(HaveOccurred())
						Expect(c.Routes[0].HealthCheck.Timeout).To(Equal(11 * time.Second))
					})

					Context("The healthcheck timeout is zero", func() {
						BeforeEach(func() {
							configSchema.Routes[0].HealthCheck.Timeout = "0s"
						})

						It("returns an error", func() {
							c, err := configSchema.Validate()
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
							_, err := configSchema.Validate()
							Expect(err).To(HaveOccurred())
							Expect(err.Error()).To(ContainSubstring("Invalid healthcheck timeout: -1"))
						})
					})

					Context("When the healthcheck timeout has no units", func() {
						BeforeEach(func() {
							configSchema.Routes[0].HealthCheck.Timeout = "1"
						})

						It("returns an error", func() {
							c, err := configSchema.Validate()
							Expect(c).To(BeNil())
							Expect(err).To(HaveOccurred())
							Expect(err.Error()).To(ContainSubstring("Invalid healthcheck timeout: time: missing unit in duration 1"))
						})
					})

					Context("When the healthcheck timeout is not parsable", func() {
						BeforeEach(func() {
							configSchema.Routes[0].HealthCheck.Timeout = "asdf"
						})

						It("returns an error", func() {
							c, err := configSchema.Validate()
							Expect(c).To(BeNil())
							Expect(err).To(HaveOccurred())
							Expect(err.Error()).To(ContainSubstring("Invalid healthcheck timeout: time: invalid duration asdf"))
						})
					})
				})
			})
		})

		Context("The host is empty", func() {
			BeforeEach(func() {
				configSchema.Host = ""
			})

			It("returns an error", func() {
				c, err := configSchema.Validate()
				Expect(c).To(BeNil())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Host required"))
			})
		})

		Context("when message bus servers are empty", func() {
			BeforeEach(func() {
				configSchema.MessageBusServers = []config.MessageBusServerSchema{}
			})

			It("returns an error", func() {
				c, err := configSchema.Validate()
				Expect(c).To(BeNil())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("message_bus_servers must have at least one entry"))
			})
		})
	})
})
