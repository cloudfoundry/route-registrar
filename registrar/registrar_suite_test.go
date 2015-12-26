package registrar_test

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"testing"
	"time"

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

func startNats(port int) *exec.Cmd {
	fmt.Fprintf(GinkgoWriter, "Starting gnatsd on port %d", port)

	cmd := exec.Command(
		"gnatsd",
		"-p", strconv.Itoa(port),
		"--user", "nats",
		"--pass", "nats")

	err := cmd.Start()
	if err != nil {
		fmt.Printf("gnatsd failed to start: %v\n", err)
	}

	err = waitUntilNatsUp(port)
	if err != nil {
		panic("Cannot connect to gnatsd")
	}
	fmt.Fprintf(GinkgoWriter, "gnatsd running on port %d\n", port)
	return cmd
}

func waitUntilNatsUp(port int) error {
	maxWait := 10
	for i := 0; i < maxWait; i++ {
		time.Sleep(500 * time.Millisecond)
		_, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			return nil
		}
	}
	return errors.New("Waited too long for NATS to start")
}

func stopCmd(cmd *exec.Cmd) {
	cmd.Process.Kill()
	cmd.Wait()
}
