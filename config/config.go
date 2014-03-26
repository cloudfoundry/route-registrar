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

type Config struct {
	MessageBusServer	MessageBusServer	"message_bus_server"
	ExternalHost		string		"external_host"
	ExternalIp			string		"external_ip"
	Port				int
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
