package healthchecker

import (
	"fmt"
	"regexp"
	"os/exec"
)

type RiakHealthChecker struct {
	status bool
	pidFileName string
	riakAdminProgram string
	nodeIpAddress string
}

func (checker *RiakHealthChecker) Check() bool {
	checker.status = checkPIDExist(checker.pidFileName) && checker.nodeExistsAndIsValid(checker.nodeIpAddress)
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
		fmt.Println("Error executing ", nodeValidityCheckerProgram, " : ", err)
		return false
	}

	matchesOne := regexp.MustCompile(`1`)
	return matchesOne.MatchString(string(out[:]))
}
