route-registrar
===============

A standalone executable written in golang that continuously broadcasts a route using NATS to the CF router.

This uses [yagnats](https://github.com/cloudfoundry/yagnats) for connecting to the NATS bus and [gibson](https://github.com/cloudfoundry/gibson) for registering routes with the CloudFoundry router.

## Usage

### Executing tests

To run tests, run `bin/test` from the root of this repository.
