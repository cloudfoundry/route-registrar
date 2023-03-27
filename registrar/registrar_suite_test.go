package registrar_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	natsPort int
)

func TestRoute_register(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Registrar Suite")
}

var _ = BeforeSuite(func() {
	natsPort = 40000 + GinkgoParallelNode()
})
