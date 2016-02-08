package config

import (
	"fmt"
	"time"
)

type MessageBusServerSchema struct {
	Host     string `yaml:"host"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

type HealthCheckSchema struct {
	Name       string `yaml:"name"`
	ScriptPath string `yaml:"script_path"`
	Timeout    string `yaml:"timeout"`
}

type ConfigSchema struct {
	MessageBusServers []MessageBusServerSchema `yaml:"message_bus_servers"`
	Routes            []RouteSchema            `yaml:"routes"`
	Host              string                   `yaml:"host"`
}

type RouteSchema struct {
	Name                 string             `yaml:"name"`
	Port                 int                `yaml:"port"`
	Tags                 map[string]string  `yaml:"tags"`
	URIs                 []string           `yaml:"uris"`
	RegistrationInterval string             `yaml:"registration_interval,omitempty"`
	HealthCheck          *HealthCheckSchema `yaml:"health_check,omitempty"`
}

type MessageBusServer struct {
	Host     string
	User     string
	Password string
}

type HealthCheck struct {
	Name       string
	ScriptPath string
	Timeout    time.Duration
}

type Config struct {
	MessageBusServers []MessageBusServer
	Routes            []Route
	Host              string
}

type Route struct {
	Name                 string
	Port                 int
	Tags                 map[string]string
	URIs                 []string
	RegistrationInterval time.Duration
	HealthCheck          *HealthCheck
}

type multiError struct {
	errors []error
}

func (e multiError) Error() string {
	var errStr string
	for _, err := range e.errors {
		errStr = fmt.Sprintf("%s%s\n", errStr, err.Error())
	}
	return errStr
}

func (e *multiError) add(err error) {
	errors, ok := err.(multiError)
	if ok {
		e.errors = append(e.errors, errors.errors...)
	} else {
		e.errors = append(e.errors, err)
	}
}

func (e multiError) hasAny() bool {
	return len(e.errors) > 0
}

func (c ConfigSchema) ToConfig() (*Config, error) {
	errors := multiError{}

	if c.Host == "" {
		errors.add(fmt.Errorf("host required"))
	}

	messageBusServers, err := messageBusServersFromSchema(c.MessageBusServers)
	if err != nil {
		errors.add(err)
	}

	routes := []Route{}
	for index, r := range c.Routes {
		route, err := routeFromSchema(r, index)
		if err != nil {
			errors.add(err)
			continue
		}

		routes = append(routes, *route)
	}

	if errors.hasAny() {
		return nil, errors
	}

	config := Config{
		Host:              c.Host,
		MessageBusServers: messageBusServers,
		Routes:            routes,
	}

	return &config, nil
}

func parseRegistrationInterval(registrationInterval string, index int) (time.Duration, error) {
	var duration time.Duration

	if registrationInterval == "" {
		return duration, fmt.Errorf("registration_interval not provided for route %d", index)
	}

	var err error
	duration, err = time.ParseDuration(registrationInterval)
	if err != nil {
		return duration, fmt.Errorf("route %d has invalid registration_interval: %s", index, err.Error())
	}

	if duration <= 0 {
		return duration, fmt.Errorf("route %d has invalid registration_interval: interval must be greater than 0", index)
	}

	return duration, nil
}

func routeFromSchema(r RouteSchema, index int) (*Route, error) {
	errors := multiError{}

	if r.Name == "" {
		errors.add(fmt.Errorf("name must be provided for route %d", index))
	}

	registrationInterval, err := parseRegistrationInterval(r.RegistrationInterval, index)
	if err != nil {
		errors.add(err)
	}

	if errors.hasAny() {
		return nil, errors
	}

	var healthCheck *HealthCheck
	if r.HealthCheck != nil {
		healthCheck, err = healthCheckFromSchema(r.HealthCheck, registrationInterval)
		if err != nil {
			return nil, err
		}
	}

	route := Route{
		Name:                 r.Name,
		Port:                 r.Port,
		Tags:                 r.Tags,
		URIs:                 r.URIs,
		RegistrationInterval: registrationInterval,
		HealthCheck:          healthCheck,
	}
	return &route, nil
}

func healthCheckFromSchema(healthCheckSchema *HealthCheckSchema, registrationInterval time.Duration) (*HealthCheck, error) {
	healthCheck := &HealthCheck{
		Name:       healthCheckSchema.Name,
		ScriptPath: healthCheckSchema.ScriptPath,
	}

	if healthCheckSchema.Timeout == "" {
		healthCheck.Timeout = registrationInterval / 2
		return healthCheck, nil
	}

	var err error
	healthCheck.Timeout, err = time.ParseDuration(healthCheckSchema.Timeout)
	if err != nil {
		return nil, fmt.Errorf("Invalid healthcheck timeout: %s", err.Error())
	}

	if healthCheck.Timeout <= 0 {
		return nil, fmt.Errorf("Invalid healthcheck timeout: %s", healthCheck.Timeout)
	}

	if healthCheck.Timeout >= registrationInterval {
		return nil, fmt.Errorf(
			"Invalid healthcheck timeout: %v must be less than registration interval: %v",
			healthCheck.Timeout,
			registrationInterval,
		)
	}
	return healthCheck, nil
}

func messageBusServersFromSchema(servers []MessageBusServerSchema) ([]MessageBusServer, error) {
	messageBusServers := []MessageBusServer{}
	if len(servers) < 1 {
		return nil, fmt.Errorf("message_bus_servers must have at least one entry")
	}

	for _, m := range servers {
		messageBusServers = append(
			messageBusServers,
			MessageBusServer{
				Host:     m.Host,
				User:     m.User,
				Password: m.Password,
			},
		)
	}

	return messageBusServers, nil
}
