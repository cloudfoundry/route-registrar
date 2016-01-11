package healthchecker_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/cloudfoundry-incubator/route-registrar/healthchecker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("ScriptHealthChecker", func() {
	var (
		logger     lager.Logger
		tmpDir     string
		scriptPath string
		scriptText string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir(os.TempDir(), "healthchecker-test")
		Expect(err).ToNot(HaveOccurred())
		scriptPath = filepath.Join(tmpDir, "healthchecker.sh")
		logger = lagertest.NewTestLogger("Script healthchecker test")
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Context("When the script's stdout says 1", func() {
		BeforeEach(func() {
			scriptText = "#!/bin/bash\necho 1\n"
			ioutil.WriteFile(scriptPath, []byte(scriptText), 0777)
		})

		It("returns true", func() {
			healthChecker := healthchecker.NewHealthChecker(logger)
			Expect(healthChecker.Check(scriptPath)).To(BeTrue(), "Expected Check to return true when stdout is 1")
		})
	})

	Context("When the script's stdout says anything else", func() {
		BeforeEach(func() {
			scriptText = "#!/bin/bash\necho 0\n"
			ioutil.WriteFile(scriptPath, []byte(scriptText), 0777)
		})

		It("returns false", func() {
			healthChecker := healthchecker.NewHealthChecker(logger)
			Expect(healthChecker.Check(scriptPath)).To(BeFalse(), "Expected Check to return false when stdout is not 1")
		})
	})
})
