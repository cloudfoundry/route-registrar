package healthchecker

import (
	"fmt"
	"os/exec"
	"regexp"

	"github.com/pivotal-golang/lager"
)

type RiakHealthChecker struct {
	logger lager.Logger
	status           bool
	pidFileName      string
	riakAdminProgram string
	nodeIpAddress    string
}

func (checker *RiakHealthChecker) Check() bool {
	pidFileExists := checkPIDExist(checker.pidFileName)
	checker.status = pidFileExists

	if pidFileExists {
		nodeExistsAndIsValid := checker.nodeExistsAndIsValid(checker.nodeIpAddress)
		checker.status = checker.status && nodeExistsAndIsValid

		if !nodeExistsAndIsValid {
			checker.logger.Info("RiakHealthChecker: Node is not a valid member of the cluster\n")
		}
	} else {
		checker.logger.Info(fmt.Sprintf("RiakHealthChecker: pidFile does not exist: %s\n", checker.pidFileName))
	}

	return checker.status
}

func NewRiakHealthChecker(pidFileName string, riakAdminProgram string, nodeIpAddress string, logger lager.Logger) *RiakHealthChecker {
	return &RiakHealthChecker{
		status:           false,
		pidFileName:      pidFileName,
		riakAdminProgram: riakAdminProgram,
		nodeIpAddress:    nodeIpAddress,
		logger: logger,
	}
}

func (checker *RiakHealthChecker) nodeExistsAndIsValid(nodeIp string) (result bool) {
	nodeValidityCheckerProgram := "./check_node_validity.sh"

	cmd := exec.Command(nodeValidityCheckerProgram, checker.riakAdminProgram, nodeIp)

	out, err := cmd.CombinedOutput()
	if err != nil {
		checker.logger.Info(fmt.Sprintf("RiakHealthChecker: Error executing %s : %s\n", nodeValidityCheckerProgram, err))
		return false
	}

	matchesOne := regexp.MustCompile(`1`)
	return matchesOne.MatchString(string(out[:]))
}
