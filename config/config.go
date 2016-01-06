package config

import "fmt"

type MessageBusServer struct {
	Host     string `yaml:"host"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

type HealthCheckerConf struct {
	Name              string `yaml:"name"`
	HealthcheckScript string `yaml:"healthcheck_script_path"`
}

type Config struct {
	MessageBusServers []MessageBusServer `yaml:"message_bus_servers"`
	HealthChecker     *HealthCheckerConf `yaml:"health_checker"`
	Routes            []Route            `yaml:"routes"`
	UpdateFrequency   int                `yaml:"update_frequency"`
	Host              string             `yaml:"host"`
}

type Route struct {
	Name string            `yaml:"name"`
	Port int               `yaml:"port"`
	Tags map[string]string `yaml:"tags"`
	URIs []string          `yaml:"uris"`
}

func (c Config) Validate() error {
	if c.UpdateFrequency <= 0 {
		return fmt.Errorf("Invalid update frequency: %d", c.UpdateFrequency)
	}

	if c.Host == "" {
		return fmt.Errorf("Invalid host: %s", c.Host)
	}
	return nil

}
