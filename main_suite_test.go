package main_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

const (
	routeRegistrarPackage = "github.com/cloudfoundry-incubator/route-registrar/"
)

var (
	routeRegistrarBinPath string
	pidFile               string
	configFile            string
)

func TestRouteRegistrar(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Main Suite")
}

var _ = BeforeSuite(func() {
	var err error
	routeRegistrarBinPath, err = gexec.Build(routeRegistrarPackage, "-race")
	Ω(err).ShouldNot(HaveOccurred())

	tempDir, err := ioutil.TempDir(os.TempDir(), "route-registrar")
	Ω(err).NotTo(HaveOccurred())

	pidFile = filepath.Join(tempDir, "route-registrar.pid")

	configFile = filepath.Join(tempDir, "registrar_settings.yml")
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})
