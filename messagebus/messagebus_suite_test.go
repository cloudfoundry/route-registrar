package messagebus_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	natsPort int
)

func TestMessagebus(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Messagebus Suite")
}

var _ = BeforeSuite(func() {
	natsPort = 20000 + GinkgoParallelNode()
})
