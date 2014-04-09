package healthchecker

import (
)

type RiakCSHealthChecker struct {
	status bool
	pidFileName string
}

func (checker *RiakCSHealthChecker) Check() bool {
	checker.status = checkPIDExist(checker.pidFileName)
	return checker.status
}

func NewRiakCSHealthChecker(pidFileName string) *RiakCSHealthChecker{
	return &RiakCSHealthChecker{
		status: false,
		pidFileName: pidFileName,
	}
}
