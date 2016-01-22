package nats_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
)

var (
	natsPort int
)

func TestNats(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Nats Suite")
}

var _ = BeforeSuite(func() {
	natsPort = 40000 + config.GinkgoConfig.ParallelNode
})
