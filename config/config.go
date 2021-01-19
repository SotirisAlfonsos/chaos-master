package config

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type Config struct {
	APIOptions        *RestAPIOptions   `yaml:"api_options"`
	JobsFromConfig    []*JobsFromConfig `yaml:"jobs,flow"`
	Bots              *Bots             `yaml:"bots,flow"`
	HealthCheckReport bool              `yaml:"health_check_report,flow"`
}

type RestAPIOptions struct {
	Port   string `yaml:"port"`
	Scheme string `yaml:"scheme"`
}

type JobsFromConfig struct {
	JobName       string      `yaml:"job_name"`
	FailureType   FailureType `yaml:"type"`
	ComponentName string      `yaml:"component_name"`
	Targets       []string    `yaml:"targets,omitempty"`
}

type Bots struct {
	CACert     string `yaml:"ca_cert,omitempty"`
	PublicCert string `yaml:"public_cert,omitempty"`
	PeerToken  string `yaml:"peer_token"`
}

type FailureType string

const (
	Docker  FailureType = "Docker"
	Service FailureType = "Service"
)

func GetConfig(file string) (*Config, error) {
	return unmarshalConfFromFile(file)
}

func unmarshalConfFromFile(file string) (*Config, error) {
	DefaultRestAPI := &RestAPIOptions{
		Port:   "8080",
		Scheme: "http",
	}
	DefaultConfig := Config{
		APIOptions:        DefaultRestAPI,
		HealthCheckReport: false,
	}

	config := DefaultConfig

	if file != "" {
		yamlFile, err := ioutil.ReadFile(file)
		if err != nil {
			err = errors.Wrap(err, "could not read yml")
			return nil, err
		}

		if err = yaml.Unmarshal(yamlFile, &config); err != nil {
			err = errors.Wrap(err, "could not unmarshal yml")
			return nil, err
		}
	}

	if err := config.validate(); err != nil {
		return nil, err
	}

	return &config, nil
}

func (config *Config) validate() error {
	for _, jobFromConfig := range config.JobsFromConfig {
		if jobFromConfig.JobName == "" || jobFromConfig.ComponentName == "" || jobFromConfig.FailureType == "" {
			return errors.New("Every job should contain a job_name, type and component_name")
		}
		if strings.Contains(jobFromConfig.JobName, ",") || strings.Contains(jobFromConfig.ComponentName, ",") {
			return errors.New("The job name and the component name should not contain the unique operator \",\"")
		}
	}
	return nil
}

type Job struct {
	ComponentName string
	FailureType   FailureType
	Target        []string
}

func (config *Config) GetJobMap(logger log.Logger) map[string]*Job {
	jobs := make(map[string]*Job)

	for _, configJobs := range config.JobsFromConfig {
		configJobs.addToJobsMap(jobs, logger)
	}

	showRegisteredJobs(jobs, logger)

	return jobs
}

func (cj *JobsFromConfig) addToJobsMap(jobs map[string]*Job, logger log.Logger) {
	if _, ok := jobs[cj.JobName]; ok {
		_ = level.Error(logger).Log("msg", fmt.Sprintf("The job name %s is not unique", cj.JobName))
	} else {
		jobs[cj.JobName] = &Job{
			ComponentName: cj.ComponentName,
			FailureType:   cj.FailureType,
			Target:        cj.Targets,
		}
	}
}

func showRegisteredJobs(jobsMap map[string]*Job, logger log.Logger) {
	for jobName, job := range jobsMap {
		_ = level.Info(logger).Log("msg", fmt.Sprintf("{%s} job registered for component {%s} type {%s} and targets %v",
			jobName, job.ComponentName, job.FailureType, job.Target))
	}
}
