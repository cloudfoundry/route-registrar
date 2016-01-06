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
			c = config.Config{
				UpdateFrequency: 20,
				Host:            "127.0.0.1",
			}
		})

		It("returns without error for all valid values", func() {
			err := c.Validate()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("The update frequency is zero", func() {
			BeforeEach(func() {
				c.UpdateFrequency = 0
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
	})
})
