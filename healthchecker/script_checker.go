package healthchecker

import (
	"os/exec"
	"regexp"
	"github.com/pivotal-golang/lager"
	"fmt"
)

type ScriptHealthChecker struct {
	logger lager.Logger
	scriptPath string
}

func NewScriptHealthChecker(scriptPath string, logger lager.Logger) *ScriptHealthChecker {
	return &ScriptHealthChecker{
		scriptPath: scriptPath,
		logger: logger,
	}
}

func (checker *ScriptHealthChecker) Check() bool {
	cmd := exec.Command(checker.scriptPath)
	checker.logger.Info(fmt.Sprintf("Script Path: %s\n", checker.scriptPath))

	out, err := cmd.CombinedOutput()
	if err != nil {
		checker.logger.Info(fmt.Sprintf("Error executing %s : %s\n", checker.scriptPath, err))
		return false
	}

	matchesOne := regexp.MustCompile(`1`)
	return matchesOne.MatchString(string(out[:]))
	// return false
}
