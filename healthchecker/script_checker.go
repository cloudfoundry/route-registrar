package healthchecker

import (
	"os/exec"
	"regexp"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . HealthChecker

type HealthChecker interface {
	Check(string) (bool, error)
}

type healthChecker struct {
	logger lager.Logger
}

func NewHealthChecker(logger lager.Logger) HealthChecker {
	return &healthChecker{
		logger: logger,
	}
}

func (checker healthChecker) Check(scriptPath string) (bool, error) {
	cmd := exec.Command(scriptPath)
	checker.logger.Info(
		"Executing script",
		lager.Data{"scriptPath": scriptPath},
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		checker.logger.Info(
			"Error executing script",
			lager.Data{"script": scriptPath,
				"error": err.Error(),
			},
		)
		return false, nil
	}

	matchesOne := regexp.MustCompile(`1`)
	return matchesOne.MatchString(string(out[:])), nil
}
