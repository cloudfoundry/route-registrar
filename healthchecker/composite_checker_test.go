package healthchecker_test

import (
	"github.com/cloudfoundry-incubator/route-registrar/healthchecker"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type FakeHealthChecker struct {
	status bool
}

func (handler *FakeHealthChecker) Check() bool {
	return handler.status
}

func NewFakeHealthChecker() *FakeHealthChecker {
	return &FakeHealthChecker{
		status: false,
	}
}

var _ = Describe("Check", func() {
	It("returns true when all checks return true", func() {
		checker1 := NewFakeHealthChecker()
		checker1.status = true
		checker2 := NewFakeHealthChecker()
		checker2.status = true
		checkerArray := []healthchecker.HealthChecker{checker1, checker2}

		compositeChecker := healthchecker.NewCompositeChecker(checkerArray)
		Expect(compositeChecker.Check()).To(Equal(true))
	})

	It("returns false if any checks return false", func() {
		checker1 := NewFakeHealthChecker()
		checker1.status = true
		checker2 := NewFakeHealthChecker()
		checkerArray := []healthchecker.HealthChecker{checker1, checker2}

		compositeChecker := healthchecker.NewCompositeChecker(checkerArray)
		Expect(compositeChecker.Check()).To(Equal(false))
	})
})
