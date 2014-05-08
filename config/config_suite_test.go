package config_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRoute_register(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Config Suite")
}
