route-registrar
===============

A standalone executable written in golang that continuously broadcasts a route using NATS to the CF router.

This uses [nats-io/nats](https://github.com/nats-io/nats) for connecting to the NATS bus.

## Usage

### BOSH release

You can colocate `route-registrar` into any BOSH deployment using https://github.com/cloudfoundry-community/route-registrar-boshrelease BOSH release.

### Executing tests
1. Install the ginkgo binary with `go get`:
  ```
  go get github.com/onsi/ginkgo/ginkgo
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
  ```
  go install github.com/cloudfoundry-incubator/route-registrar
  ```

1. The route-registrar expects a configuration YAML file like the one below:
  ```yaml
  message_bus_servers:
  - host: REPLACE_WITH_NATS_URL
    user: REPLACE_WITH_NATS_USERNAME
    password: REPLACE_WITH_NATS_PASSWORD
  update_frequency: UPDATE_FREQUENCY_IN_SECONDS
  host: HOSTNAME_OR_IP_OF_ROUTE_DESTINATION
  routes:
  - name: SOME_ROUTE_NAME
    port: REPLACE_WITH_VM_PORT
    tags:
      optional_tag_field: some_tag_value
      another_tag_field: some_other_value
    uris:
    - some_uri_for_the_router_should_listen_on
    - some_other_uri_for_the_router_to_listen_on
  ```

1. Run route-registrar binaries using the following command
  ```
  ./bin/route-registrar -configPath=FILE_PATH_TO_CONFIG_YML --pidFile=PATH_TO_PIDFILE
  ```
