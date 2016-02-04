package healthchecker_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/cloudfoundry-incubator/route-registrar/healthchecker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("ScriptHealthChecker", func() {
	var (
		logger     lager.Logger
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

		h = healthchecker.NewHealthChecker(logger)
	})

	AfterEach(func() {
		// os.RemoveAll(tmpDir)
	})

	Context("When the writes to stdout and stderr", func() {
		BeforeEach(func() {
			scriptText := "#!/bin/bash\necho 'my-stdout'; >&2 echo 'my-stderr'; exit 0\n"
			ioutil.WriteFile(scriptPath, []byte(scriptText), os.ModePerm)
		})

		It("writes stdout and stderr to the logs", func() {
			_, _ = h.Check(scriptPath, timeout)

			Expect(logger).Should(gbytes.Say("stderr\":\"my-stderr"))
			Expect(logger).Should(gbytes.Say("stdout\":\"my-stdout"))
		})
	})

	Context("When the script exits 0", func() {
		BeforeEach(func() {
			scriptText := "#!/bin/bash\nexit 0\n"
			ioutil.WriteFile(scriptPath, []byte(scriptText), os.ModePerm)
		})

		It("returns true without error", func() {
			result, err := h.Check(scriptPath, timeout)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(result).To(BeTrue())
		})
	})

	Context("When the script exits non-zero", func() {
		BeforeEach(func() {
			scriptText := "#!/bin/bash\nexit 127"
			ioutil.WriteFile(scriptPath, []byte(scriptText), os.ModePerm)
		})

		It("returns false without error", func() {
			result, err := h.Check(scriptPath, timeout)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(result).To(BeFalse())
		})
	})

	Context("when the script fails to start", func() {
		BeforeEach(func() {
			ioutil.WriteFile(scriptPath, []byte(""), 0666)
		})

		It("returns error", func() {
			_, err := h.Check(scriptPath, timeout)
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("when the timeout is positive", func() {
		BeforeEach(func() {
			timeout = 2 * time.Second
		})

		Context("when the script exits within timeout", func() {
			BeforeEach(func() {
				scriptText := "#!/bin/bash\nsleep 1"
				ioutil.WriteFile(scriptPath, []byte(scriptText), os.ModePerm)
			})

			It("returns true without error", func() {
				result, err := h.Check(scriptPath, timeout)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(result).To(BeTrue())
			})
		})

		Context("when the script does not exit within the timeout", func() {
			BeforeEach(func() {
				scriptText := "#!/bin/bash\nsleep 5"
				ioutil.WriteFile(scriptPath, []byte(scriptText), os.ModePerm)
			})

			It("returns error", func() {
				_, err := h.Check(scriptPath, timeout)
				Expect(err).Should(HaveOccurred())
			})
		})
	})
})
