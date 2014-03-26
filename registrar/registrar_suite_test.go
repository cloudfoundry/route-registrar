package registrar_test

import (
	"testing"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry-incubator/route-registrar/test_helpers"
)

func TestRoute_register(t *testing.T) {
	RegisterFailHandler(Fail)

	fmt.Println("starting gnatsd...")
	natsCmd := StartNats(4222)

	RunSpecs(t, "Route_register Suite")

	StopCmd(natsCmd)
}
