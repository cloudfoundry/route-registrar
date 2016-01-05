package config_test

import (
	"github.com/cloudfoundry-incubator/route-registrar/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	Describe("Update frequency validations", func() {
		var (
			c   config.Config
			err error
		)
		BeforeEach(func() {
			c = config.Config{
				UpdateFrequency: 20,
			}
		})

		It("validates the correct values", func() {
			err = c.Validate()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("The update frequency is unset", func() {
			BeforeEach(func() {
				c = config.Config{}
			})
			It("returns an error", func() {
				err = c.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Invalid update frequency"))
			})
		})
	})
})
