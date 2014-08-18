package healthchecker

import (
	. "github.com/cloudfoundry-incubator/route-registrar/logger"
)

type RiakCSHealthChecker struct {
	status      bool
	pidFileName string
}

func (checker *RiakCSHealthChecker) Check() bool {
	pidFileExists := checkPIDExist(checker.pidFileName)

	if !pidFileExists {
		LogWithTimestamp("RiakCSHealthChecker: pidFile does not exist: %s\n", checker.pidFileName)
	}

	checker.status = pidFileExists
	return checker.status
}

func NewRiakCSHealthChecker(pidFileName string) *RiakCSHealthChecker {
	return &RiakCSHealthChecker{
		status:      false,
		pidFileName: pidFileName,
	}
}
