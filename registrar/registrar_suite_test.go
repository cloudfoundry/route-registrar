package registrar_test

import (
	"fmt"
	"testing"

	"github.com/cloudfoundry-incubator/route-registrar/test_helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRoute_register(t *testing.T) {
	RegisterFailHandler(Fail)

	fmt.Println("starting gnatsd...")
	natsCmd := test_helpers.StartNats(4222)

	RunSpecs(t, "Registrar Suite")

	test_helpers.StopCmd(natsCmd)
}
