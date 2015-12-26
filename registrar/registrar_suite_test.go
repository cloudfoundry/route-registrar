package registrar_test

import (
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
)

var (
	natsPort int
	natsCmd  *exec.Cmd
)

func TestRoute_register(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Registrar Suite")
}

var _ = BeforeSuite(func() {
	natsPort = 40000 + config.GinkgoConfig.ParallelNode
})
