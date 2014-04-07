package config

import (
	"io/ioutil"
	"gopkg.in/v1/yaml"
)

type MessageBusServer struct {
	Host		string
	User		string
	Password    string
}

type HealthCheckerConf struct {
	Name		string		"name"
}

type Config struct {
	MessageBusServer	MessageBusServer	"message_bus_server"
	ExternalHost		string		"external_host"
	ExternalIp			string		"external_ip"
	Port				int
	HealthChecker		*HealthCheckerConf "health_checker"
}

func Initialize(configYAML []byte, c *Config) error {
	return yaml.Unmarshal(configYAML, c)
}

func InitConfigFromFile(path string) Config {
	c := new(Config)
	var e error

	b, e := ioutil.ReadFile(path)
	if e != nil {
		panic(e.Error())
	}

	e = Initialize(b, c)
	if e != nil {
		panic(e.Error())
	}

	return *c
}
