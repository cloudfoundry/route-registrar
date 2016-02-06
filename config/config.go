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

func (c ConfigSchema) ToConfig() (*Config, error) {
	if c.Host == "" {
		return nil, fmt.Errorf("Host required")
	}

	messageBusServers, err := messageBusServersFromSchema(c.MessageBusServers)
	if err != nil {
		return nil, err
	}

	routes := []Route{}
	for _, r := range c.Routes {
		route, err := routeFromSchema(r)
		if err != nil {
			return nil, err
		}

		routes = append(routes, *route)
	}

	config := Config{
		Host:              c.Host,
		MessageBusServers: messageBusServers,
		Routes:            routes,
	}

	return &config, nil
}

func routeFromSchema(r RouteSchema) (*Route, error) {
	if r.RegistrationInterval == "" {
		return nil, fmt.Errorf("registration_interval not provided")
	}

	if r.Name == "" {
		return nil, fmt.Errorf("name for route must be provided")
	}

	registrationInterval, err := time.ParseDuration(r.RegistrationInterval)
	if err != nil {
		return nil, fmt.Errorf("Invalid registration_interval: %s", err.Error())
	}

	if registrationInterval <= 0 {
		return nil, fmt.Errorf("Invalid registration_interval: %d", registrationInterval)
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
