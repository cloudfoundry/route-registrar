package healthchecker

import (
	"os"
	"fmt"
	. "github.com/cloudfoundry-incubator/route-registrar/config"
)

type HealthChecker interface {
	Check() bool
}

func InitHealthChecker(clientConfig Config) *CompositeChecker{
	if(clientConfig.HealthChecker != nil){
		if(clientConfig.HealthChecker.Name == "riak-cs-cluster") {
			//TODO: these should be passed in via registrar_settings.yml
			riak_pidfile := "/var/vcap/sys/run/riak/riak.pid"
			riak_admin_program := "/var/vcap/packages/riak/rel/bin/riak-admin"
			riak_cs_pidfile := "/var/vcap/sys/run/riak-cs/riak-cs.pid"

			riakChecker := NewRiakHealthChecker(riak_pidfile, riak_admin_program, clientConfig.ExternalIp)
			riakCSChecker := NewRiakCSHealthChecker(riak_cs_pidfile)
			checkers := []HealthChecker{riakChecker, riakCSChecker}

			checker := NewCompositeChecker(checkers)
			return checker
		}
	}
	return nil
}

func checkPIDExist(pidFileName string) bool {
	_, err := os.Stat(pidFileName)
	if err == nil {
		return true
	} else {
		fmt.Println("PID file not found : ", pidFileName)
		fmt.Println("Error : ", err)
		return false
	}
}
