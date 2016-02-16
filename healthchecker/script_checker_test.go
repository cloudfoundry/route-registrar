package healthchecker_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/cloudfoundry-incubator/route-registrar/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry-incubator/route-registrar/Godeps/_workspace/src/github.com/onsi/gomega"
	"github.com/cloudfoundry-incubator/route-registrar/Godeps/_workspace/src/github.com/onsi/gomega/gbytes"
	"github.com/cloudfoundry-incubator/route-registrar/commandrunner/fakes"
	"github.com/cloudfoundry-incubator/route-registrar/healthchecker"

	"github.com/cloudfoundry-incubator/route-registrar/Godeps/_workspace/src/github.com/pivotal-golang/lager"
	"github.com/cloudfoundry-incubator/route-registrar/Godeps/_workspace/src/github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("ScriptHealthChecker", func() {
	var (
		logger     lager.Logger
		runner     *fakes.FakeRunner
		tmpDir     string
		scriptPath string
		timeout    time.Duration

		h healthchecker.HealthChecker
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir(os.TempDir(), "healthchecker-test")
		Expect(err).ToNot(HaveOccurred())

		scriptPath = filepath.Join(tmpDir, "healthchecker.sh")

		logger = lagertest.NewTestLogger("Script healthchecker test")

		runner = new(fakes.FakeRunner) //commandrunner.NewRunner(scriptPath)
		runner.RunStub = func(outbuf, errbuf *bytes.Buffer) error {
			outbuf.WriteString("my-stdout")
			errbuf.WriteString("my-stderr")
			return nil
		}

		runner.CommandErrorChannelStub = func() chan error {
			errChan := make(chan error, 1)
			errChan <- nil
			return errChan
		}

		h = healthchecker.NewHealthChecker(logger)
	})

	It("logs stdout and stderr from the runner", func() {
		_, _ = h.Check(runner, scriptPath, timeout)

		Expect(logger).Should(gbytes.Say("stderr\":\"my-stderr"))
		Expect(logger).Should(gbytes.Say("stdout\":\"my-stdout"))
	})

	Context("When the runner returns no errors", func() {
		It("returns true without error", func() {
			result, err := h.Check(runner, scriptPath, timeout)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(result).To(BeTrue())
		})
	})

	Context("When the runner returns an error on the execution channel", func() {
		BeforeEach(func() {
			runner.CommandErrorChannelStub = func() chan error {
				errChan := make(chan error, 1)
				errChan <- &exec.ExitError{}
				return errChan
			}
		})

		It("returns false without error", func() {
			result, err := h.Check(runner, scriptPath, timeout)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(result).To(BeFalse())
		})
	})

	Context("when the runner returns an immediate error", func() {
		BeforeEach(func() {
			runner.RunStub = func(outbuf, errbuf *bytes.Buffer) error {
				return errors.New("BOO")
			}
		})

		It("returns error", func() {
			_, err := h.Check(runner, scriptPath, timeout)
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("when the timeout is positive", func() {
		BeforeEach(func() {
			timeout = 2 * time.Second
		})

		Context("when the runner exits within timeout", func() {
			BeforeEach(func() {
				runner.CommandErrorChannelStub = func() chan error {
					errChan := make(chan error, 1)
					go func() {
						time.Sleep(1 * time.Second)
						errChan <- nil
					}()
					return errChan
				}
			})

			It("returns true without error", func() {
				result, err := h.Check(runner, scriptPath, timeout)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(result).To(BeTrue())
			})
		})

		Context("when the runner does not exit within the timeout", func() {
			BeforeEach(func() {
				runner.CommandErrorChannelStub = func() chan error {
					errChan := make(chan error, 1)
					go func() {
						time.Sleep(5 * time.Second)
						errChan <- nil
					}()
					return errChan
				}
			})

			It("returns error", func() {
				_, err := h.Check(runner, scriptPath, timeout)
				Expect(err).Should(HaveOccurred())
			})

			It("kills the healthcheck process", func() {
				h.Check(runner, scriptPath, timeout)
				Expect(runner.KillCallCount()).To(Equal(1))
			})
		})
	})
})
