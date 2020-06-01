package config

import (
	api "github.com/SotirisAlfonsos/chaos-master/web/api/v1"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

var (
	DefaultConfig = Config {
		ApiConfig: DefaultRestApi,
	}

	DefaultRestApi = &api.RestApi{
		Port:   "8080",
		Scheme: "http",
	}
)

type Config struct {
	ApiConfig *api.RestApi `yaml:"api_specs"`
	Slaves    *[]string   `yaml:"scan_date,omitempty"`
}

func GetConfig(file string, logger log.Logger) Config{
	return unmarshalConfFromFile(file, logger)
}

func unmarshalConfFromFile(file string, logger log.Logger) Config {
	config := DefaultConfig
	if file != "" {
		yamlFile, err := ioutil.ReadFile(file)
		if err != nil {
			_ = level.Error(logger).Log("msg", "could not read yml", "err", err)
			os.Exit(1)
		}

		if err = yaml.Unmarshal(yamlFile, config); err != nil {
			_ = level.Error(logger).Log("msg", "could not unmarshal yml", "err", err)
			os.Exit(1)
		}
	}

	return config
}