package healthchecker

import (
	"os"

	"github.com/cloudfoundry-incubator/route-registrar/config"
	"github.com/pivotal-golang/lager"
)

type HealthChecker interface {
	Check() bool
}

func InitHealthChecker(clientConfig config.Config, logger lager.Logger) HealthChecker {
	if clientConfig.HealthChecker != nil {
		if clientConfig.HealthChecker.Name == "script" {
			return NewScriptHealthChecker(clientConfig.HealthChecker.HealthcheckScript, logger)
		}
	}
	return nil
}

func checkPIDExist(pidFileName string) bool {
	_, err := os.Stat(pidFileName)
	return nil == err
}
