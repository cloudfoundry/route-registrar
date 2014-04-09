package healthchecker

import (
	"regexp"
	"os/exec"
	. "github.com/cloudfoundry-incubator/route-registrar/logger"
)

type RiakHealthChecker struct {
	status bool
	pidFileName string
	riakAdminProgram string
	nodeIpAddress string
}

func (checker *RiakHealthChecker) Check() bool {
	pidFileExists := checkPIDExist(checker.pidFileName)
	nodeExistsAndIsValid := checker.nodeExistsAndIsValid(checker.nodeIpAddress)

	if(!pidFileExists) {
		LogWithTimestamp("RiakHealthChecker: pidFile does not exist: %s\n", checker.pidFileName)
	}
	if(!nodeExistsAndIsValid) {
		LogWithTimestamp("RiakHealthChecker: Node is not a valid member of the cluster\n")
	}

	checker.status = pidFileExists && nodeExistsAndIsValid
	return checker.status
}

func NewRiakHealthChecker(pidFileName string, riakAdminProgram string, nodeIpAddress string) *RiakHealthChecker{
	return &RiakHealthChecker{
		status: false,
		pidFileName: pidFileName,
		riakAdminProgram: riakAdminProgram,
		nodeIpAddress: nodeIpAddress,
	}
}

func (checker *RiakHealthChecker)nodeExistsAndIsValid(nodeIp string) (result bool) {
	nodeValidityCheckerProgram := "./check_node_validity.sh"

	cmd := exec.Command(nodeValidityCheckerProgram, checker.riakAdminProgram, nodeIp)

	out, err := cmd.CombinedOutput()
	if err != nil {
		LogWithTimestamp("RiakHealthChecker: Error executing %s : %s\n", nodeValidityCheckerProgram, err)
		return false
	}

	matchesOne := regexp.MustCompile(`1`)
	return matchesOne.MatchString(string(out[:]))
}
