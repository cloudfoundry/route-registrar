package healthchecker_test

import (
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry-incubator/route-registrar/healthchecker"
)

var _ = Describe("ScriptHealthChecker", func() {
	var scriptPath = "/tmp/healthcheck_script.sh"
	var scriptText string

	AfterEach(func() {
		os.Remove(scriptPath)
	})

	Context("When the script's stdout says 1", func() {
		BeforeEach(func() {
			scriptText = "#!/bin/bash\necho 1\n"
			ioutil.WriteFile(scriptPath, []byte(scriptText), 0777)
		})

		It("returns true", func() {
			scriptHealthChecker := NewScriptHealthChecker(scriptPath)
			Expect(scriptHealthChecker.Check()).To(BeTrue())
		})
	})

	Context("When the script's stdout says anything else", func() {
		BeforeEach(func() {
			scriptText = "#!/bin/bash\necho 0\n"
			ioutil.WriteFile(scriptPath, []byte(scriptText), 0777)
		})

		It("returns false", func() {
			scriptHealthChecker := NewScriptHealthChecker(scriptPath)
			Expect(scriptHealthChecker.Check()).To(BeFalse())
		})
	})
})
