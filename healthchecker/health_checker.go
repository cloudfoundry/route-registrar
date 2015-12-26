package healthchecker

import (
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
