package main_test

import (
	"fmt"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Main", func() {

	It("Starts correctly and exits 1 on SIGTERM", func() {
		command := exec.Command(routeRegistrarBinPath, fmt.Sprintf("-pidfile=%s", pidFile))
		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		Eventually(session.Out).Should(gbytes.Say("Route Registrar"))

		time.Sleep(100 * time.Millisecond)

		session.Terminate().Wait()
		Eventually(session).Should(gexec.Exit(1))
	})
})
