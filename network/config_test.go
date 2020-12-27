package network

import (
	"fmt"
	"testing"

	"github.com/SotirisAlfonsos/chaos-master/chaoslogger"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
)

var (
	logger = getLogger()
)

type TestData struct {
	input    []*config.JobsFromConfig
	expected []string
}

func TestSuccessfullySetTargetConnectionPoolForSingleJob(t *testing.T) {
	dataItems := []TestData{
		{
			input: []*config.JobsFromConfig{
				{JobName: "job name", FailureType: "failure type", ComponentName: "component name", Targets: []string{"127.0.0.1"}}},
			expected: []string{"127.0.0.1"},
		},
		{
			input: []*config.JobsFromConfig{
				{JobName: "job name", FailureType: "failure type", ComponentName: "component name", Targets: []string{"127.0.0.1", "127.0.0.2"}}},
			expected: []string{"127.0.0.1", "127.0.0.2"},
		},
		{
			input: []*config.JobsFromConfig{
				{JobName: "job name", FailureType: "failure type", ComponentName: "component name", Targets: []string{}}},
			expected: []string{},
		},
	}

	for _, dataItem := range dataItems {
		t.Logf("Correct connection pull with args %+q and expected pool for targets %+q", *dataItem.input[0], dataItem.expected)

		connectionPool := GetConnectionPool(dataItem.input, logger)

		assert.Equal(t, len(connectionPool.Pool), len(dataItem.expected))
		for _, target := range dataItem.expected {
			assert.NotNil(t, connectionPool.Pool[target])
		}
	}
}

func TestSuccessfullySetTargetConnectionPoolForMultipleJob(t *testing.T) {
	dataItems := []TestData{
		{
			input: []*config.JobsFromConfig{
				{JobName: "job name", FailureType: "failure type", ComponentName: "component name", Targets: []string{"127.0.0.1"}},
				{JobName: "job name", FailureType: "failure type", ComponentName: "component name", Targets: []string{"127.0.0.2"}}},
			expected: []string{"127.0.0.1", "127.0.0.2"},
		},
		{
			input: []*config.JobsFromConfig{
				{JobName: "job name", FailureType: "failure type", ComponentName: "component name", Targets: []string{"127.0.0.1"}},
				{JobName: "job name", FailureType: "failure type", ComponentName: "component name", Targets: []string{"127.0.0.1", "127.0.0.2"}}},
			expected: []string{"127.0.0.1", "127.0.0.2"},
		},
		{
			input: []*config.JobsFromConfig{
				{JobName: "job name", FailureType: "failure type", ComponentName: "component name", Targets: []string{}},
				{JobName: "job name", FailureType: "failure type", ComponentName: "component name", Targets: []string{"127.0.0.1"}}},
			expected: []string{"127.0.0.1"},
		},
	}

	for _, dataItem := range dataItems {
		t.Logf("Correct connection pull with args %+q and %+q and expected pool for targets %+q", *dataItem.input[0], *dataItem.input[1], dataItem.expected)

		connectionPool := GetConnectionPool(dataItem.input, logger)

		assert.Equal(t, len(connectionPool.Pool), len(dataItem.expected))
		for _, target := range dataItem.expected {
			assert.NotNil(t, connectionPool.Pool[target])
		}
	}
}

func getLogger() log.Logger {
	allowLevel := &chaoslogger.AllowedLevel{}
	if err := allowLevel.Set("debug"); err != nil {
		fmt.Printf("%v", err)
	}

	return chaoslogger.New(allowLevel)
}
