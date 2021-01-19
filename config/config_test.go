package config

import (
	"fmt"
	"testing"

	"github.com/SotirisAlfonsos/chaos-master/chaoslogger"
	"github.com/go-kit/kit/log"

	"github.com/stretchr/testify/assert"
)

var (
	logger = getLogger()
)

func TestShouldUnmarshalSimpleConfig(t *testing.T) {
	config, err := GetConfig("test/simple_config.yml")
	if err != nil {
		t.Fatal(err.Error())
	} else if config == nil {
		t.Fatal("Config should not be nil")
	}

	assert.Equal(t, "8090", config.APIOptions.Port, "The correct port should be found from the fine")
	assert.Equal(t, true, config.HealthCheckReport, "The health check report option should be found from the file")
	assert.Equal(t, 3, len(config.JobsFromConfig))
	assert.Equal(t, "zookeeper docker", config.JobsFromConfig[0].JobName, "the correct first job name from the file")
	assert.Equal(t, "zookeeper service", config.JobsFromConfig[1].JobName, "the correct second job name from the file")
	assert.Equal(t, "zookeeper service", config.JobsFromConfig[2].JobName, "the correct third job name from the file")
	assert.Equal(t, "1234", config.Bots.PeerToken, "the peer token for the communication with the bots")
}

func TestShouldUnmarshalConfigWIthMissingDefaultValues(t *testing.T) {
	config, err := GetConfig("test/missing_defaults_config.yml")
	if err != nil {
		t.Fatal(err.Error())
	} else if config == nil {
		t.Fatal("Config should not be nil")
	}

	assert.Equal(t, "8080", config.APIOptions.Port, "Should have the default port")
	assert.Equal(t, false, config.HealthCheckReport, "Should have the default false value")
	assert.Equal(t, 1, len(config.JobsFromConfig))
}

func TestShouldErrorWhenCanNotFindConfigFile(t *testing.T) {
	_, err := GetConfig("test/non_existent_file.yml")
	if err != nil {
		assert.Equal(t, "could not read yml: open test/non_existent_file.yml: no such file or directory", err.Error())
	} else {
		t.Errorf("There should be an error because the file does not exist")
	}
}

func TestShouldErrorWhenCanNotUnmarshalFile(t *testing.T) {
	config, err := GetConfig("test/unmarshalable_config.yml")
	if err != nil {
		assert.Equal(t, "could not unmarshal yml: yaml: unmarshal errors:\n  line 2: cannot unmarshal !!map into []*config.JobsFromConfig", err.Error())
	} else {
		t.Errorf("There should be an error because the file does not exist %v", config)
	}
}

func TestShouldErrorWhenImportantFieldsAreMissing(t *testing.T) {
	config, err := GetConfig("test/missing_key_values.yml")
	if err != nil {
		assert.Equal(t, "Every job should contain a job_name, type and component_name", err.Error())
	} else {
		t.Errorf("There should be an error because the file does not exist %v", config)
	}
}

func TestShouldGetJobMap(t *testing.T) {
	config, err := GetConfig("test/simple_config.yml")
	if err != nil {
		t.Fatal(err.Error())
	} else if config == nil {
		t.Fatal("Config should not be nil")
	}

	jobMap := config.GetJobMap(logger)

	assert.Equal(t, 2, len(config.JobsFromConfig)-1)
	assert.Equal(t, "my_zoo", jobMap["zookeeper docker"].ComponentName)
	assert.Equal(t, Docker, jobMap["zookeeper docker"].FailureType)
	assert.Equal(t, 1, len(jobMap["zookeeper docker"].Target))
	assert.Equal(t, "simple", jobMap["zookeeper service"].ComponentName)
	assert.Equal(t, Service, jobMap["zookeeper service"].FailureType)
	assert.Equal(t, 2, len(jobMap["zookeeper service"].Target))
}

func getLogger() log.Logger {
	allowLevel := &chaoslogger.AllowedLevel{}
	if err := allowLevel.Set("debug"); err != nil {
		fmt.Printf("%v", err)
	}

	return chaoslogger.New(allowLevel)
}
