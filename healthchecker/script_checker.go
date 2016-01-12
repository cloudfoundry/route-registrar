package healthchecker

import (
	"bytes"
	"os/exec"

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

	var outbuf, errbuf bytes.Buffer
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	err := cmd.Start()
	if err != nil {
		checker.logger.Info(
			"Error starting script",
			lager.Data{
				"script": scriptPath,
				"error":  err.Error(),
				"stdout": outbuf.String(),
				"stderr": errbuf.String(),
			},
		)
		return false, err
	}

	err = cmd.Wait()

	if err != nil {
		checker.logger.Info(
			"Script exited with error",
			lager.Data{
				"script": scriptPath,
				"error":  err.Error(),
				"stdout": outbuf.String(),
				"stderr": errbuf.String(),
			},
		)

		// If the script exited non-zero then we do not consider that an error
		_, ok := err.(*exec.ExitError)
		if ok {
			return false, nil
		}

		// Untested due to difficulty of reproducing this case under test
		// E.g. this path would be encountered for I/O errors between the script
		// and the golang parent process which we cannot force in a test.
		return false, err
	}

	checker.logger.Info(
		"Script exited without error",
		lager.Data{
			"script": scriptPath,
			"stdout": outbuf.String(),
			"stderr": errbuf.String(),
		},
	)
	return true, nil
}
