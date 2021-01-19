package network

import (
	"testing"

	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/stretchr/testify/assert"
)

type TestData struct {
	message    string
	jobsConfig []*config.JobsFromConfig
	bots       *config.Bots
	expected   []string
}

func TestSuccessfullySetTargetConnectionPoolForSingleJob(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Should add single target to connection pool",
			jobsConfig: []*config.JobsFromConfig{
				{JobName: "job name", FailureType: "failure type", ComponentName: "component name", Targets: []string{"127.0.0.1"}}},
			expected: []string{"127.0.0.1"},
		},
		{
			message: "Should add both targets to new connection pool",
			jobsConfig: []*config.JobsFromConfig{
				{JobName: "job name", FailureType: "failure type", ComponentName: "component name", Targets: []string{"127.0.0.1", "127.0.0.2"}}},
			expected: []string{"127.0.0.1", "127.0.0.2"},
		},
		{
			message: "Should create new connection pool with no targets if no targets where provided",
			jobsConfig: []*config.JobsFromConfig{
				{JobName: "job name", FailureType: "failure type", ComponentName: "component name", Targets: []string{}}},
			expected: []string{},
		},
	}

	for _, dataItem := range dataItems {
		t.Logf(dataItem.message)

		conf := &config.Config{
			JobsFromConfig:    dataItem.jobsConfig,
			HealthCheckReport: false,
		}

		connectionPool := GetConnectionPool(conf, logger)

		assert.Equal(t, len(dataItem.expected), len(connectionPool.Pool))
		for _, target := range dataItem.expected {
			assert.NotNil(t, connectionPool.Pool[target])
		}
	}
}

func TestSuccessfullySetTargetConnectionPoolForMultipleJob(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Should create new connection pool for targets from multiple jobs",
			jobsConfig: []*config.JobsFromConfig{
				{JobName: "job name", FailureType: "failure type", ComponentName: "component name", Targets: []string{"127.0.0.1"}},
				{JobName: "job name", FailureType: "failure type", ComponentName: "component name", Targets: []string{"127.0.0.2"}}},
			expected: []string{"127.0.0.1", "127.0.0.2"},
		},
		{
			message: "Should create new connection pool removing duplicate targets for multiple jobs",
			jobsConfig: []*config.JobsFromConfig{
				{JobName: "job name", FailureType: "failure type", ComponentName: "component name", Targets: []string{"127.0.0.1", "127.0.0.1"}},
				{JobName: "job name", FailureType: "failure type", ComponentName: "component name", Targets: []string{"127.0.0.1", "127.0.0.2"}}},
			expected: []string{"127.0.0.1", "127.0.0.2"},
		},
		{
			message: "Should create new connection pool and include targets only if job has targets",
			jobsConfig: []*config.JobsFromConfig{
				{JobName: "job name", FailureType: "failure type", ComponentName: "component name", Targets: []string{}},
				{JobName: "job name", FailureType: "failure type", ComponentName: "component name", Targets: []string{"127.0.0.1"}}},
			expected: []string{"127.0.0.1"},
		},
	}

	for _, dataItem := range dataItems {
		t.Logf(dataItem.message)

		conf := &config.Config{
			JobsFromConfig:    dataItem.jobsConfig,
			HealthCheckReport: false,
		}

		connectionPool := GetConnectionPool(conf, logger)

		assert.Equal(t, len(dataItem.expected), len(connectionPool.Pool))
		for _, target := range dataItem.expected {
			assert.NotNil(t, connectionPool.Pool[target])
		}
	}
}

func TestSuccessfullySetTargetConnectionPoolWithTls(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Should create connection with no cert and no peer token",
			jobsConfig: []*config.JobsFromConfig{
				{JobName: "job name", FailureType: "failure type", ComponentName: "component name", Targets: []string{"127.0.0.1:8081"}}},
			bots:     &config.Bots{},
			expected: []string{"127.0.0.1:8081"},
		},
		{
			message: "Should create connections with valid public certs and token provided",
			jobsConfig: []*config.JobsFromConfig{
				{JobName: "job name", FailureType: "failure type", ComponentName: "component name", Targets: []string{"127.0.0.1:8081", "127.0.0.2:8081"}}},
			bots:     &config.Bots{PublicCert: "../config/test/certs/server-cert.pem", PeerToken: "12345"},
			expected: []string{"127.0.0.1:8081", "127.0.0.2:8081"},
		},
		{
			message: "Should create connections with valid ca certs and token provided",
			jobsConfig: []*config.JobsFromConfig{
				{JobName: "job name", FailureType: "failure type", ComponentName: "component name", Targets: []string{"127.0.0.1:8081"}}},
			bots:     &config.Bots{CACert: "../config/test/certs/ca-cert.pem", PeerToken: "12345"},
			expected: []string{"127.0.0.1:8081"},
		},
	}

	for _, dataItem := range dataItems {
		t.Logf(dataItem.message)

		conf := &config.Config{
			JobsFromConfig: dataItem.jobsConfig,
			Bots:           dataItem.bots,
		}

		connectionPool := GetConnectionPool(conf, logger)

		assert.Equal(t, len(dataItem.expected), len(connectionPool.Pool))
		for _, target := range dataItem.expected {
			if _, err := connectionPool.Pool[target].GetHealthClient(target); err != nil {
				t.Errorf("Connection redial should not have error when getting the health client")
			}
			if _, err := connectionPool.Pool[target].GetDockerClient(target); err != nil {
				t.Errorf("Connection redial should not have error when getting the docker client")
			}
			if _, err := connectionPool.Pool[target].GetServiceClient(target); err != nil {
				t.Errorf("Connection redial should not have error when getting the service client")
			}
			assert.NotNil(t, connectionPool.Pool[target])
		}
	}
}

func TestRedialFailureWithTls(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Should not create connection for invalid path to file",
			jobsConfig: []*config.JobsFromConfig{
				{JobName: "job name", FailureType: "failure type", ComponentName: "component name", Targets: []string{"127.0.0.1:8081", "127.0.0.2:8081"}}},
			bots:     &config.Bots{PublicCert: "../path/to/file", PeerToken: "12345"},
			expected: []string{"127.0.0.1:8081", "127.0.0.2:8081"},
		},
		{
			message: "Should not create connection with peer token but no certs",
			jobsConfig: []*config.JobsFromConfig{
				{JobName: "job name", FailureType: "failure type", ComponentName: "component name", Targets: []string{"127.0.0.1:8081", "127.0.0.2:8081"}}},
			bots:     &config.Bots{PeerToken: "12345"},
			expected: []string{"127.0.0.1:8081", "127.0.0.2:8081"},
		},
	}

	for _, dataItem := range dataItems {
		t.Logf(dataItem.message)

		conf := &config.Config{
			JobsFromConfig: dataItem.jobsConfig,
			Bots:           dataItem.bots,
		}

		connectionPool := GetConnectionPool(conf, logger)

		assert.Equal(t, len(dataItem.expected), len(connectionPool.Pool))
		for _, target := range dataItem.expected {
			if _, err := connectionPool.Pool[target].GetHealthClient(target); err == nil {
				t.Errorf("Connection redial should have error when getting the health client")
			}
			if _, err := connectionPool.Pool[target].GetDockerClient(target); err == nil {
				t.Errorf("Connection redial should have error when getting the docker client")
			}
			if _, err := connectionPool.Pool[target].GetServiceClient(target); err == nil {
				t.Errorf("Connection redial should have error when getting the service client")
			}
		}
	}
}
