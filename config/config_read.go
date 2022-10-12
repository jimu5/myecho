package config

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type YAMLConfig struct {
	Database *Database `yaml:"database"`
}

type Database struct {
	Type     string `yaml:"type"`
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"db_name"`
}

var Yaml YAMLConfig

func ReadYAMLConfig() {
	yamlFile, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(yamlFile, &Yaml)
	if err != nil {
		panic(err)
	}
}
