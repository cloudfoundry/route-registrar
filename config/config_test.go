package config_test

import (
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry-incubator/route-registrar/config"
)

var _ = Describe("Config", func() {
	var configFile string

	BeforeEach(func() {
		path, _ := os.Getwd()
		configFile = strings.Join([]string{path, "/../registrar_settings.yml"}, "")
	})

	It("Initializes a configuration from file", func() {
		cfg := InitConfigFromFile(configFile)

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
