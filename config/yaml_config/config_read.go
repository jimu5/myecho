package yaml_config

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type YAMLConfig struct {
	Database  *Database  `yaml:"database"`
	APPConfig *APPConfig `yaml:"app_config"`
}

type Database struct {
	Type     string `yaml:"type"`
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"db_name"`
}

type APPConfig struct {
	AllowRegister bool `yaml:"allow_register"`
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
