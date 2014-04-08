package healthchecker

import (
	"os"
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

type RiakCSHealthChecker struct {
	status bool
	pidFileName string
}

func (checker *RiakCSHealthChecker) Check() bool {
	//do the check of riak-cs process
	checker.status = checkPIDExist(checker.pidFileName)
	return checker.status
}

func (checker *RiakHealthChecker) Check() bool {
	//do the check of riak process and cluster status
	checker.status = checkPIDExist(checker.pidFileName) && checker.nodeExistsAndIsValid(checker.nodeIpAddress)
	return checker.status
}

func checkPIDExist(pidFileName string) bool {
	_, err := os.Stat(pidFileName)
	if err == nil {
		return true
	} else {
		fmt.Println("Not Found PID file", err)
		return false
	}
}

func NewRiakHealthChecker(pidFileName string, riakAdminProgram string, nodeIpAddress string) *RiakHealthChecker{
	return &RiakHealthChecker{
		status: false,
		pidFileName: pidFileName,
		riakAdminProgram: riakAdminProgram,
		nodeIpAddress: nodeIpAddress,
	}
}

func NewRiakCSHealthChecker(pidFileName string) *RiakCSHealthChecker{
	return &RiakCSHealthChecker{
		status: false,
		pidFileName: pidFileName,
	}
}

func (checker *RiakHealthChecker)nodeExistsAndIsValid(nodeIp string) (result bool) {
	nodeValidityCheckerProgram := "./check_node_validity.sh"

	cmd := exec.Command(nodeValidityCheckerProgram, checker.riakAdminProgram, nodeIp)
	cmd.Env = os.Environ()
//	for _, v := range cmd.Env {
//		println("Env Variable:", v)
//	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + string(out))
		return false
	}

//	out, err := cmd.Output()
//
//	if err != nil {
//		// MAYBE PANIC HERE??
//		fmt.Println("err", err)
//	}

	matchesOne := regexp.MustCompile(`1`)
	return matchesOne.MatchString(string(out[:]))
}
