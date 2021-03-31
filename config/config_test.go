package config

import (
	"fmt"
	"os"
	"testing"

	"github.com/SotirisAlfonsos/chaos-master/pkg/chaoslogger"
	"github.com/stretchr/testify/assert"
)

var (
	loggers = getLoggers()
)

func TestShouldUnmarshalSimpleConfig(t *testing.T) {
	config, err := GetConfig("test/simple_config.yml")
	if err != nil {
		t.Fatal(err.Error())
	} else if config == nil {
		t.Fatal("Config should not be nil")
	}

	assert.Equal(t, "8090", config.APIOptions.Port, "The correct port should be found from the fine")
	assert.Equal(t, true, config.HealthCheck.Report, "The health check report option should be found from the file")
	assert.Equal(t, true, config.HealthCheck.Active, "The health check active option should be found from the file")
	assert.Equal(t, 6, len(config.JobsFromConfig))
	assert.Equal(t, "zookeeper docker", config.JobsFromConfig[0].JobName, "the correct first job name from the file")
	assert.Equal(t, "zookeeper service", config.JobsFromConfig[1].JobName, "the correct second job name from the file")
	assert.Equal(t, "zookeeper service", config.JobsFromConfig[2].JobName, "the correct third job name from the file")
	assert.Equal(t, "cpu injection", config.JobsFromConfig[3].JobName, "the correct fourth job name from the file")
	assert.Equal(t, "server injection", config.JobsFromConfig[4].JobName, "the correct fifth job name from the file")
	assert.Equal(t, "network injection", config.JobsFromConfig[5].JobName, "the correct sixth job name from the file")
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
	assert.Equal(t, false, config.HealthCheck.Report, "Should have the default false value")
	assert.Equal(t, false, config.HealthCheck.Active, "Should have the default false value")
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
	_, err := GetConfig("test/unmarshalable_config.yml")
	if err != nil {
		assert.Equal(t, "could not unmarshal yml: yaml: unmarshal errors:\n  line 2: cannot unmarshal !!map into []*config.JobsFromConfig", err.Error())
	} else {
		t.Errorf("There should be an error because the file can not be unmarshaled")
	}
}

func TestShouldErrorWhenImportantFieldsAreMissing(t *testing.T) {
	config, err := GetConfig("test/missing_key_values.yml")
	if err != nil {
		assert.Equal(t, "Every job should contain a job_name and type", err.Error())
	} else {
		t.Errorf("There should be an error because the file is missing key values %v", config)
	}
}

func Test_Should_Error_When_Docker_failure_does_not_have_component_name(t *testing.T) {
	config, err := GetConfig("test/docker_no_component_name_config.yml")
	if err != nil {
		assert.Equal(t, "failure type {Docker} should have component_name", err.Error())
	} else {
		t.Errorf("There should be an error because the file is missing key values %v", config)
	}
}

func Test_Should_Error_When_CPU_failure_has_component_name(t *testing.T) {
	config, err := GetConfig("test/cpu_should_not_have_component_name.yml")
	if err != nil {
		assert.Equal(t, "job {CPU} should not have component_name", err.Error())
	} else {
		t.Errorf("There should be an error because the file is missing key values %v", config)
	}
}

func Test_Should_Error_When_Server_failure_has_component_name(t *testing.T) {
	config, err := GetConfig("test/server_should_not_have_component_name.yml")
	if err != nil {
		assert.Equal(t, "job {Server} should not have component_name", err.Error())
	} else {
		t.Errorf("There should be an error because the file is missing key values %v", config)
	}
}

func TestShouldGetJobMap(t *testing.T) {
	config, err := GetConfig("test/simple_config.yml")
	if err != nil {
		t.Fatal(err.Error())
	} else if config == nil {
		t.Fatal("Config should not be nil")
	}

	jobMap := config.GetJobMap(loggers)

	assert.Equal(t, 5, len(config.JobsFromConfig)-1)
	assert.Equal(t, "my_zoo", jobMap["zookeeper docker"].ComponentName)
	assert.Equal(t, Docker, jobMap["zookeeper docker"].FailureType)
	assert.Equal(t, 1, len(jobMap["zookeeper docker"].Target))
	assert.Equal(t, "simple", jobMap["zookeeper service"].ComponentName)
	assert.Equal(t, Service, jobMap["zookeeper service"].FailureType)
	assert.Equal(t, 2, len(jobMap["zookeeper service"].Target))
	assert.Equal(t, "", jobMap["cpu injection"].ComponentName)
	assert.Equal(t, CPU, jobMap["cpu injection"].FailureType)
	assert.Equal(t, 1, len(jobMap["cpu injection"].Target))
	assert.Equal(t, "", jobMap["server injection"].ComponentName)
	assert.Equal(t, Server, jobMap["server injection"].FailureType)
	assert.Equal(t, 1, len(jobMap["server injection"].Target))
	assert.Equal(t, Network, jobMap["network injection"].FailureType)
	assert.Equal(t, 1, len(jobMap["network injection"].Target))
}

func getLoggers() chaoslogger.Loggers {
	allowLevel := &chaoslogger.AllowedLevel{}
	if err := allowLevel.Set("debug"); err != nil {
		fmt.Printf("%v", err)
	}

	return chaoslogger.Loggers{
		OutLogger: chaoslogger.New(allowLevel, os.Stdout),
		ErrLogger: chaoslogger.New(allowLevel, os.Stderr),
	}
}
