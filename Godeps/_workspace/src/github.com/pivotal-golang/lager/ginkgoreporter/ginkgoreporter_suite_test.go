package ginkgoreporter_test

import (
	. "github.com/cloudfoundry-incubator/route-registrar/Godeps/_workspace/src/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry-incubator/route-registrar/Godeps/_workspace/src/github.com/onsi/gomega"

	"testing"
)

func TestGinkgoReporter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GinkgoReporter Suite")
}
