package config_test

import (
	"os"
	"path"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry-incubator/route-registrar/config"
)

var _ = Describe("Config", func() {
	Context("when the config file does not exist", func() {
		It("returns an error", func() {
			_, err := InitConfigFromFile("")

			Expect(err).To(HaveOccurred())
		})

		It("returns an empty config file", func() {
			cfg, _ := InitConfigFromFile("")

			Expect(cfg).To(Equal(Config{}))
		})
	})

	Context("when the config file exists", func() {
		Context("when the config file contains invalid yaml", func() {
			var configFilePath string

			BeforeEach(func() {
				configFilePath = path.Join(os.TempDir(), "tmp-route-registrar.yml")
				f, err := os.Create(configFilePath)
				Expect(err).NotTo(HaveOccurred())
				_, err = f.WriteString("invalid yaml")
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error", func() {
				_, err := InitConfigFromFile(configFilePath)

				Expect(err).To(HaveOccurred())
			})

			It("returns an empty config file", func() {
				cfg, _ := InitConfigFromFile(configFilePath)

				Expect(cfg).To(Equal(Config{}))
			})
		})

		Context("when the config file contains valid yaml", func() {
			var configFile string

			BeforeEach(func() {
				currentDir, _ := os.Getwd()
				configFile = path.Join(currentDir, "..", "registrar_settings.yml")
			})

			It("Initializes a configuration from file without error", func() {
				cfg, err := InitConfigFromFile(configFile)

				Expect(err).NotTo(HaveOccurred())

				Expect(cfg.ExternalHost).To(Equal("riakcs.vcap.me"))
				Expect(cfg.ExternalIp).To(Equal("127.0.0.1"))
				Expect(cfg.Port).To(Equal(8080))

				Expect(cfg.MessageBusServers[0].Host).To(Equal("10.244.0.6:4222"))
				Expect(cfg.MessageBusServers[0].User).To(Equal("nats"))
				Expect(cfg.MessageBusServers[0].Password).To(Equal("nats"))

				Expect(cfg.HealthChecker.Name).To(Equal("riak-cs-cluster"))
				Expect(cfg.HealthChecker.Interval).To(Equal(float64(10)))
			})
		})
	})
})
