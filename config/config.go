package config

import "fmt"

type MessageBusServer struct {
	Host     string `yaml:"host"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

type HealthCheck struct {
	Name       string `yaml:"name"`
	ScriptPath string `yaml:"script_path"`
	Timeout    int    `yaml:"timeout"`
}

type Config struct {
	MessageBusServers []MessageBusServer `yaml:"message_bus_servers"`
	Routes            []Route            `yaml:"routes"`
	Host              string             `yaml:"host"`
}

type Route struct {
	Name                 string            `yaml:"name"`
	Port                 int               `yaml:"port"`
	Tags                 map[string]string `yaml:"tags"`
	URIs                 []string          `yaml:"uris"`
	RegistrationInterval *int              `yaml:"registration_interval,omitempty"`
	HealthCheck          *HealthCheck      `yaml:"health_check,omitempty"`
}

func (c Config) Validate() error {
	for _, r := range c.Routes {
		if r.RegistrationInterval == nil {
			return fmt.Errorf("Update frequency not provided")
		}

		if *r.RegistrationInterval <= 0 {
			return fmt.Errorf("Invalid update frequency: %d", r.RegistrationInterval)
		}
	}

	if c.Host == "" {
		return fmt.Errorf("Invalid host: %s", c.Host)
	}

	return nil
}
