package healthchecker_test

import (
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry-incubator/route-registrar/healthchecker"
)

var _ = Describe("RiakCSHealthChecker", func() {
    Describe("Check", func() {
			var pidFilename string
			var riakAdminProgram string

			BeforeEach(func() {
				path, _ := os.Getwd()
				pidFilename = strings.Join([]string{path, "/../test_helpers/examplePidFile.pid"}, "")
				riakAdminProgram = strings.Join([]string{path, "/../test_helpers/riak-admin"}, "")
			})

			It("returns true when the PID file exists", func () {
				riakCsHealthChecker := NewRiakCSHealthChecker(pidFilename)

				Expect(riakCsHealthChecker.Check()).To(BeTrue())
			})

			It("returns false when the PID file does not exist", func() {
				riakCsHealthChecker := NewRiakCSHealthChecker("/tmp/file-that-does-not-exist")

				Expect(riakCsHealthChecker.Check()).To(BeFalse())
			})
		})
})
