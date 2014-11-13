package healthchecker

import (
	"github.com/cloudfoundry-incubator/route-registrar/logger"
	"os/exec"
	"regexp"
)

type ScriptHealthChecker struct {
	scriptPath string
}

func NewScriptHealthChecker(scriptPath string) *ScriptHealthChecker {
	return &ScriptHealthChecker{
		scriptPath: scriptPath,
	}
}

func (checker *ScriptHealthChecker) Check() bool {
	cmd := exec.Command(checker.scriptPath)
	logger.LogWithTimestamp("Script Path: %s\n", checker.scriptPath)

	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.LogWithTimestamp("Error executing %s : %s\n", checker.scriptPath, err)
		return false
	}

	matchesOne := regexp.MustCompile(`1`)
	return matchesOne.MatchString(string(out[:]))
	// return false
}
