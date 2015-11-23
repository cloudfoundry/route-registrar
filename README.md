route-registrar
===============

A standalone executable written in golang that continuously broadcasts a route using NATS to the CF router.

This uses [yagnats](https://github.com/cloudfoundry/yagnats) for connecting to the NATS bus and [gibson](https://github.com/cloudfoundry/gibson) for registering routes with the CloudFoundry router.

## Usage

### BOSH release

You can colocate `route-registrar` into any BOSH deployment using https://github.com/cloudfoundry-community/route-registrar-boshrelease BOSH release.

### Executing tests
1. Run the following commands to install ginkgo and gomega

 ```
 go get github.com/onsi/ginkgo/ginkgo  # installs the ginkgo CLI
 go install -v github.com/onsi/ginkgo/ginkgo
 go get github.com/onsi/gomega
 go install -v github.com/onsi/gomega
 ```

1. Run tests, by running the following command from root of this reposirtory
```
bin/test
```

### Installation
1. Run
	```
	git clone https://github.com/cloudfoundry-incubator/route-registrar
	```
1. Run the following command to install route-registrar
	```go install github.com/cloudfoundry-incubator/route-registrar```

1. The route-registrar expects a configuration YAML file like the one below:

	```
	message_bus_servers:
     - host: REPLACE_WITH_NATS_URL
       user: REPLACE_WITH_NATS_USERNAME
       password: REPLACE_WITH_NATS_PASSWORD
	external_host: REPLACE_WITH_ROUTE_TO_REGISTER
	external_ip: REPLACE_WITH_VM_IP
	port: REPLACE_WITH_VM_PORT
	```

1. Run route-registrar binaries using the following command

```
./bin/route-registrar -configPath=FILE_PATH_TO_CONFIG_YML --pidFile=PATH_TO_PIDFILE

```

