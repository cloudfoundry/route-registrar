package healthchecker_test

import (
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry-incubator/route-registrar/healthchecker"
)

var _ = Describe("Check", func() {
	var pidFilename string
	var riakAdminProgram string

	BeforeEach(func() {
		path, _ := os.Getwd()
		pidFilename = strings.Join([]string{path, "/../test_helpers/examplePidFile.pid"}, "")
		riakAdminProgram = strings.Join([]string{path, "/../test_helpers/riak-admin"}, "")
	})

	It("returns true when the PID file exists and the node is a member of the cluster", func () {
		riakHealthChecker := NewRiakHealthChecker(pidFilename, riakAdminProgram, "1.2.3.4")
		riakCsHealthChecker := NewRiakCSHealthChecker(pidFilename)

		Expect(riakHealthChecker.Check()).To(BeTrue())
		Expect(riakCsHealthChecker.Check()).To(BeTrue())

	})

	It("returns false when the PID file does not exist", func() {
		riakHealthChecker := NewRiakHealthChecker("/tmp/file-that-does-not-exist", riakAdminProgram, "1.2.3.4")
		riakCsHealthChecker := NewRiakCSHealthChecker("/tmp/file-that-does-not-exist")


		Expect(riakHealthChecker.Check()).To(BeFalse())
		Expect(riakCsHealthChecker.Check()).To(BeFalse())
	})


	It("returns false when the PID file exists but the node is not present in the cluster", func() {
		riakHealthChecker := NewRiakHealthChecker(pidFilename, riakAdminProgram, "1.2.3.99")

		Expect(riakHealthChecker.Check()).To(BeFalse())
	})

	It("returns false when the PID file exists and the node is present in the cluster but the node status is not 'valid'", func() {
		riakHealthChecker := NewRiakHealthChecker(pidFilename, riakAdminProgram, "1.2.3.5")

		Expect(riakHealthChecker.Check()).To(BeFalse())
	})

})
