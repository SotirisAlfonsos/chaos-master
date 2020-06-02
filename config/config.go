package config

import (
	"io/ioutil"
	"os"

	api "github.com/SotirisAlfonsos/chaos-master/web/api/v1"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"gopkg.in/yaml.v2"
)

type Config struct {
	APIConfig   *api.RestAPI `yaml:"api_config"`
	ChaosSlaves []string     `yaml:"chaos_slaves,flow"`
}

func GetConfig(file string, logger log.Logger) Config {
	return unmarshalConfFromFile(file, logger)
}

func unmarshalConfFromFile(file string, logger log.Logger) Config {
	DefaultRestAPI := &api.RestAPI{
		Port:   "8080",
		Scheme: "http",
	}
	DefaultConfig := Config{
		APIConfig: DefaultRestAPI,
	}

	config := DefaultConfig

	if file != "" {
		yamlFile, err := ioutil.ReadFile(file)
		if err != nil {
			_ = level.Error(logger).Log("msg", "could not read yml", "err", err)

			os.Exit(1)
		}

		if err = yaml.Unmarshal(yamlFile, &config); err != nil {
			_ = level.Error(logger).Log("msg", "could not unmarshal yml", "err", err)

			os.Exit(1)
		}
	}

	return config
}
