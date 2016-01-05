package main_test

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/cloudfoundry-incubator/route-registrar/config"
	"github.com/fraenkel/candiedyaml"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Main", func() {
	var natsCmd *exec.Cmd

	BeforeEach(func() {
		initConfig()
		writeConfig()

		natsCmd = exec.Command(
			"gnatsd",
			"-p", strconv.Itoa(natsPort),
			"--user", "nats",
			"--pass", "nats")
		err := natsCmd.Start()
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		natsCmd.Process.Kill()
		natsCmd.Wait()
	})

	It("Starts correctly and exits 1 on SIGTERM", func() {
		command := exec.Command(
			routeRegistrarBinPath,
			fmt.Sprintf("-pidfile=%s", pidFile),
			fmt.Sprintf("-configPath=%s", configFile),
		)
		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(session.Out).Should(gbytes.Say("Initializing"))
		Eventually(session.Out).Should(gbytes.Say("Writing pid"))
		Eventually(session.Out).Should(gbytes.Say("Running"))

		time.Sleep(500 * time.Millisecond)

		session.Terminate().Wait()
		Eventually(session).Should(gexec.Exit())
		Expect(session.ExitCode()).ToNot(BeZero())
	})

	Context("When the config validatation fails", func() {
		BeforeEach(func() {
			rootConfig.UpdateFrequency = 0
			writeConfig()
		})

		It("exits with error", func() {
			command := exec.Command(
				routeRegistrarBinPath,
				fmt.Sprintf("-configPath=%s", configFile),
			)
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(session.Out).Should(gbytes.Say("Initializing"))
			Eventually(session.Err).Should(gbytes.Say("Invalid update frequency"))

			Eventually(session).Should(gexec.Exit())
			Expect(session.ExitCode()).ToNot(BeZero())
		})
	})
})

func initConfig() {
	natsPort = 42222 + GinkgoParallelNode()

	messageBusServers := []config.MessageBusServer{
		config.MessageBusServer{
			Host:     fmt.Sprintf("127.0.0.1:%d", natsPort),
			User:     "nats",
			Password: "nats",
		},
	}

	healthCheckerConfig := &config.HealthCheckerConf{
		Name: "a health-checkable",
	}

	rootConfig = config.Config{
		MessageBusServers: messageBusServers,
		HealthChecker:     healthCheckerConfig,
		UpdateFrequency:   1,
	}
}

func writeConfig() {
	fileToWrite, err := os.Create(configFile)
	Expect(err).ShouldNot(HaveOccurred())

	encoder := candiedyaml.NewEncoder(fileToWrite)
	err = encoder.Encode(rootConfig)
	Expect(err).ShouldNot(HaveOccurred())
}
