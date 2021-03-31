package config

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/SotirisAlfonsos/chaos-master/pkg/chaoslogger"

	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type Config struct {
	APIOptions     *RestAPIOptions   `yaml:"api_options"`
	JobsFromConfig []*JobsFromConfig `yaml:"jobs,flow"`
	Bots           *Bots             `yaml:"bots,flow"`
	HealthCheck    *HealthCheck      `yaml:"health_check,flow"`
}

type RestAPIOptions struct {
	Port   string `yaml:"port"`
	Scheme string `yaml:"scheme"`
}

type HealthCheck struct {
	Active bool `yaml:"active,flow"`
	Report bool `yaml:"report,flow"`
}

type JobsFromConfig struct {
	JobName       string      `yaml:"job_name"`
	FailureType   FailureType `yaml:"type"`
	ComponentName string      `yaml:"component_name,omitempty"`
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
	CPU     FailureType = "CPU"
	Server  FailureType = "Server"
	Network FailureType = "Network"
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
		APIOptions: DefaultRestAPI,
		HealthCheck: &HealthCheck{
			Active: false,
			Report: false,
		},
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
		err := validate(jobFromConfig)
		if err != nil {
			return err
		}
	}
	return nil
}

func validate(job *JobsFromConfig) error {
	if job.JobName == "" || job.FailureType == "" {
		return errors.New("Every job should contain a job_name and type")
	}

	if job.FailureType == Docker || job.FailureType == Service {
		if job.ComponentName == "" {
			return fmt.Errorf("failure type {%s} should have component_name", job.FailureType)
		}
	} else if job.FailureType == CPU || job.FailureType == Server || job.FailureType == Network {
		if job.ComponentName != "" {
			return fmt.Errorf("job {%s} should not have component_name", job.FailureType)
		}
	}

	if strings.Contains(job.JobName, ",") {
		return errors.New("The job name and the component name should not contain the unique operator \",\"")
	}

	return nil
}

type Job struct {
	ComponentName string
	FailureType   FailureType
	Target        []string
}

func (config *Config) GetJobMap(loggers chaoslogger.Loggers) map[string]*Job {
	jobs := make(map[string]*Job)

	for _, configJobs := range config.JobsFromConfig {
		configJobs.addToJobsMap(jobs, loggers)
	}

	showRegisteredJobs(jobs, loggers)

	return jobs
}

func (cj *JobsFromConfig) addToJobsMap(jobs map[string]*Job, loggers chaoslogger.Loggers) {
	if _, ok := jobs[cj.JobName]; ok {
		_ = level.Error(loggers.ErrLogger).Log("msg", fmt.Sprintf("The job name %s is not unique", cj.JobName))
	} else {
		jobs[cj.JobName] = &Job{
			ComponentName: cj.ComponentName,
			FailureType:   cj.FailureType,
			Target:        cj.Targets,
		}
	}
}

func showRegisteredJobs(jobsMap map[string]*Job, loggers chaoslogger.Loggers) {
	for jobName, job := range jobsMap {
		if job.ComponentName != "" {
			_ = level.Info(loggers.OutLogger).Log("msg", fmt.Sprintf("{%s} job registered for component {%s} type {%s} and targets %v",
				jobName, job.ComponentName, job.FailureType, job.Target))
		} else {
			_ = level.Info(loggers.OutLogger).Log("msg", fmt.Sprintf("{%s} job registered for type {%s} and targets %v",
				jobName, job.FailureType, job.Target))
		}
	}
}
