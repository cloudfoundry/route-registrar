package config_test

import (
	"github.com/cloudfoundry-incubator/route-registrar/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	Describe("Validate", func() {
		var (
			c config.Config
		)

		BeforeEach(func() {
			registrationInterval := 20
			c = config.Config{
				Routes: []config.Route{
					{
						RegistrationInterval: &registrationInterval,
					},
				},
				Host: "127.0.0.1",
			}
		})

		It("returns without error for all valid values", func() {
			err := c.Validate()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("The registration interval is nil", func() {
			BeforeEach(func() {
				c.Routes[0].RegistrationInterval = nil
			})

			It("returns an error", func() {
				err := c.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Update frequency not provided"))
			})
		})

		Context("The registration interval is zero", func() {
			BeforeEach(func() {
				*c.Routes[0].RegistrationInterval = 0
			})

			It("returns an error", func() {
				err := c.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Invalid update frequency"))
			})
		})

		Context("The host is empty", func() {
			BeforeEach(func() {
				c.Host = ""
			})

			It("returns an error", func() {
				err := c.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Invalid host"))
			})
		})

		Context("healthcheck is provided", func() {
			BeforeEach(func() {
				c.Routes[0].HealthCheck = &config.HealthCheck{
					Name:       "my healthcheck",
					ScriptPath: "/some/script/path",
				}
			})

			Context("healthcheck timeout is not provided", func() {
				BeforeEach(func() {
					c.Routes[0].HealthCheck.Timeout = nil
				})

				It("defaults to half of the registration interval", func() {
					err := c.Validate()
					Expect(err).NotTo(HaveOccurred())

					expectedTimeout := *c.Routes[0].RegistrationInterval / 2
					Expect(*c.Routes[0].HealthCheck.Timeout).To(Equal(expectedTimeout))
				})
			})
		})
	})
})
