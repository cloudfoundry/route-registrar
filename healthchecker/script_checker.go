package healthchecker

import (
	"bytes"
	"fmt"
	"os/exec"
	"time"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . HealthChecker

type HealthChecker interface {
	Check(scriptPath string, timeout time.Duration) (bool, error)
}

type healthChecker struct {
	logger lager.Logger
}

func NewHealthChecker(logger lager.Logger) HealthChecker {
	return &healthChecker{
		logger: logger,
	}
}

func (h healthChecker) Check(scriptPath string, timeout time.Duration) (bool, error) {
	cmd := exec.Command(scriptPath)
	h.logger.Info(
		"Executing script",
		lager.Data{"scriptPath": scriptPath},
	)

	var outbuf, errbuf bytes.Buffer
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	err := cmd.Start()
	if err != nil {
		h.logger.Info(
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

	cmdErrChan := make(chan error)
	go func() {
		cmdErrChan <- cmd.Wait()
	}()

	if timeout <= 0 {
		err := <-cmdErrChan
		return h.handleOutput(scriptPath, err, outbuf, errbuf)
	}

	select {
	case <-time.After(timeout):
		h.logger.Info(
			"Script failed to exit within timeout",
			lager.Data{
				"script":  scriptPath,
				"stdout":  outbuf.String(),
				"stderr":  errbuf.String(),
				"timeout": timeout,
			},
		)
		return false, fmt.Errorf("Script failed to exit within %v", timeout)

	case err := <-cmdErrChan:
		return h.handleOutput(scriptPath, err, outbuf, errbuf)
	}
}

func (h healthChecker) handleOutput(scriptPath string, err error, outbuf, errbuf bytes.Buffer) (bool, error) {
	if err != nil {
		h.logger.Info(
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

	h.logger.Info(
		"Script exited without error",
		lager.Data{
			"script": scriptPath,
			"stdout": outbuf.String(),
			"stderr": errbuf.String(),
		},
	)
	return true, nil
}
