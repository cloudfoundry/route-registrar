package integration

import (
	"code.cloudfoundry.org/routing-api/test_helpers"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

const routeRegistrarPackage = "code.cloudfoundry.org/route-registrar/"

var (
	routeRegistrarBinPath string
	pidFile               string
	configFile            string
	natsPort              int

	tempDir string
)

func TestIntegration(test *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(test, "Integration Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	path, err := gexec.Build(routeRegistrarPackage, "-race")
	Expect(err).ShouldNot(HaveOccurred())

	return []byte(path)
}, func(data []byte) {
	routeRegistrarBinPath = string(data)

	tempDir, err := os.MkdirTemp(os.TempDir(), "route-registrar")
	Expect(err).ToNot(HaveOccurred())

	pidFile = filepath.Join(tempDir, "route-registrar.pid")

	natsPort = test_helpers.NextAvailPort()

	configFile = filepath.Join(tempDir, "registrar_settings.json")
})

var _ = SynchronizedAfterSuite(func() {
}, func() {
	err := os.RemoveAll(tempDir)
	Expect(err).ShouldNot(HaveOccurred())

	gexec.CleanupBuildArtifacts()
})
