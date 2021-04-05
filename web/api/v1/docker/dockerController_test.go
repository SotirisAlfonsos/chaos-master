package docker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"

	"github.com/SotirisAlfonsos/gocache"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/SotirisAlfonsos/chaos-master/pkg/cache"
	"github.com/SotirisAlfonsos/chaos-master/pkg/chaoslogger"
	"github.com/SotirisAlfonsos/chaos-master/pkg/network"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/response"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

var (
	loggers = getLoggers()
)

type TestData struct {
	message        string
	jobMap         map[string]*config.Job
	connectionPool map[string]*dConnection
	cacheItems     map[cache.Key]func() (*v1.StatusResponse, error)
	requestPayload *RequestPayload
	expected       *expectedResult
}

type expectedResult struct {
	cacheSize int
	response  *responseWrapper
}

type responseWrapper struct {
	status  int
	message string
	err     error
}

func TestRecoverDockerSuccess(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Successfully recover single container with specific job, name and target",
			jobMap: map[string]*config.Job{
				"job name":           newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
				"job different name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
				"127.0.0.2": withSuccessDockerConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 0, response: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
		{
			message: "Successfully recover single container with specific job, name and target and remove it from the cache",
			jobMap: map[string]*config.Job{
				"job name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
			},
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 0, response: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
	}

	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "recover")
	}
}

func TestKillDockerSuccess(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Successfully kill single container with specific job, name and target and add it in cache",
			jobMap: map[string]*config.Job{
				"job name":           newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
				"job different name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
				"127.0.0.2": withSuccessDockerConnection(),
			},
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job name", Target: "127.0.0.2"}: functionWithSuccessResponse(),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 2, response: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
		{
			message: "Successfully kill single container with specific job, name and target and dont add it in cache if already exists",
			jobMap: map[string]*config.Job{
				"job name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
			},
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 1, response: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
	}

	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "kill")
	}
}

func TestDockerActionOneOfJobContainerNameTargetDoesNotExist(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Should receive bad request and not update cache for action when job name does not exist",
			jobMap: map[string]*config.Job{
				"job name":           newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
				"job different name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name does not exist", Container: "container name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 0, response: badRequestResponse("Could not find job {job name does not exist}")},
		},
		{
			message: "Should receive bad request and not update cache for action when container name does not exist for target",
			jobMap: map[string]*config.Job{
				"job name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name does not exist", Target: "127.0.0.1"},
			expected: &expectedResult{
				cacheSize: 0,
				response:  badRequestResponse("Container {container name does not exist} is not registered for target {127.0.0.1}"),
			},
		},
		{
			message: "Should receive bad request and not update cache for action when target does not exist for the job and container specified",
			jobMap: map[string]*config.Job{
				"job name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name", Target: "0.0.0.0"},
			expected:       &expectedResult{cacheSize: 0, response: badRequestResponse("Container {container name} is not registered for target {0.0.0.0}")},
		},
	}

	t.Log("Action recover")
	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "recover")
	}

	t.Log("Action kill")
	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "kill")
	}
}

func TestDockerActionFailure(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Should receive internal server error and not update cache if the request fails on bot",
			jobMap: map[string]*config.Job{
				"job name":           newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
				"job different name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withFailureDockerConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 0, response: internalServerErrorResponse("Failure response from target {127.0.0.1}")},
		},
		{
			message: "Should receive internal server error and not update cache if the request errors on bot",
			jobMap: map[string]*config.Job{
				"job name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withErrorDockerConnection("error occurred"),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name", Target: "127.0.0.1"},
			expected: &expectedResult{
				cacheSize: 0, response: internalServerErrorResponse("Error response from target {127.0.0.1}: error occurred")},
		},
	}

	t.Log("Action recover")
	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "recover")
	}

	t.Log("Action kill")
	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "kill")
	}
}

func TestServiceActionWithNoGrpcConnectionToTarget(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Should receive bad request and not update cache if the action to perform is invalid",
			jobMap: map[string]*config.Job{
				"job name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withFailureToSetDockerConnection("Can not dial target"),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 0, response: internalServerErrorResponse("Can not get docker connection from target {127.0.0.1}: Can not dial target")},
		},
	}

	t.Log("Action recover")
	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "recover")
	}
}

func TestDockerWithInvalidAction(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Should receive bad request and not update cache if the action to perform is invalid",
			jobMap: map[string]*config.Job{
				"job name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 0, response: badRequestResponse("The action {invalidAction} is not supported")},
		},
	}

	t.Log("Action invalidAction")
	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "invalidAction")
	}
}

func assertActionPerformed(t *testing.T, dataItem TestData, action string) {
	t.Run(dataItem.message, func(t *testing.T) {
		c := gocache.New(0)
		server, err := dockerHTTPTestServerWithCacheItems(dataItem.jobMap, dataItem.connectionPool, c, dataItem.cacheItems)
		if err != nil {
			t.Fatal(err)
		}

		status, message, err := dockerPostCall(server, dataItem.requestPayload, action)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, dataItem.expected.response.status, status)
		assert.Equal(t, dataItem.expected.response.message, message)
		assert.Equal(t, dataItem.expected.cacheSize, c.ItemCount())

		server.Close()
	})
}

type TestDataForRandomDocker struct {
	message        string
	jobMap         map[string]*config.Job
	connectionPool map[string]*dConnection
	cacheItems     map[cache.Key]func() (*v1.StatusResponse, error)
	requestPayload *RequestPayload
	expected       *expectedResult
}

func TestRecoverRandomDockerSuccess(t *testing.T) {
	dataItems := []TestDataForRandomDocker{
		{
			message: "Successfully recover random container with specific job and name",
			jobMap: map[string]*config.Job{
				"job name":           newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
				"job different name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
				"127.0.0.2": withSuccessDockerConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name"},
			expected:       &expectedResult{cacheSize: 0, response: okResponse("Response from target {127.0.0.\\d}, {}, {SUCCESS}")},
		},
		{
			message: "Successfully recover random container with specific job and name and remove it from the cache",
			jobMap: map[string]*config.Job{
				"job name": newDockerJob("container name", "127.0.0.1"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
			},
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name"},
			expected:       &expectedResult{cacheSize: 0, response: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
	}

	for _, dataItem := range dataItems {
		assertRandomActionPerformed(t, dataItem, "random", "recover")
	}
}

func TestKillRandomDockerSuccess(t *testing.T) {
	dataItems := []TestDataForRandomDocker{
		{
			message: "Successfully kill random container with specific job and name and add it in cache",
			jobMap: map[string]*config.Job{
				"job name":           newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
				"job different name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
				"127.0.0.2": withSuccessDockerConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name"},
			expected:       &expectedResult{cacheSize: 1, response: okResponse("Response from target {127.0.0.\\d}, {}, {SUCCESS}")},
		},
		{
			message: "Successfully kill random container with specific job and name and dont add it in cache if it is already present",
			jobMap: map[string]*config.Job{
				"job name": newDockerJob("container name", "127.0.0.1"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
			},
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name"},
			expected:       &expectedResult{cacheSize: 1, response: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
	}

	for _, dataItem := range dataItems {
		assertRandomActionPerformed(t, dataItem, "random", "kill")
	}
}

func TestDockerActionForRandomTargetOneOfJobContainerNameTargetDoesNotExist(t *testing.T) {
	dataItems := []TestDataForRandomDocker{
		{
			message: "Should receive bad request and not update cache for action when job name does not exist",
			jobMap: map[string]*config.Job{
				"job name":           newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
				"job different name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name does not exist", Container: "container name"},
			expected:       &expectedResult{cacheSize: 0, response: badRequestResponse("Could not find job {job name does not exist}")},
		},
		{
			message: "Should receive bad request and not update cache for action when container name does not exist for target",
			jobMap: map[string]*config.Job{
				"job name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name does not exist"},
			expected: &expectedResult{
				cacheSize: 0,
				response:  badRequestResponse("Could not find container name {container name does not exist}"),
			},
		},
		{
			message: "Should receive bad request and not update cache for action when job has no targets defined",
			jobMap: map[string]*config.Job{
				"job name": newDockerJob("container name"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name"},
			expected:       &expectedResult{cacheSize: 0, response: badRequestResponse("Could not get random target for job {job name}. err: No targets available")},
		},
	}

	t.Log("Action recover")
	for _, dataItem := range dataItems {
		assertRandomActionPerformed(t, dataItem, "random", "recover")
	}

	t.Log("Action kill")
	for _, dataItem := range dataItems {
		assertRandomActionPerformed(t, dataItem, "random", "kill")
	}
}

func TestDockerActionForRandomTargetFailure(t *testing.T) {
	dataItems := []TestDataForRandomDocker{
		{
			message: "Should receive internal server error and not update cache for failure response from bot on action",
			jobMap: map[string]*config.Job{
				"job name":           newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
				"job different name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withFailureDockerConnection(),
				"127.0.0.2": withFailureDockerConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name"},
			expected:       &expectedResult{cacheSize: 0, response: internalServerErrorResponse("Failure response from target {127.0.0.\\d}")},
		},
		{
			message: "Should receive internal server error and not update cache for error response from bot on action",
			jobMap: map[string]*config.Job{
				"job name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withErrorDockerConnection("error message"),
				"127.0.0.2": withErrorDockerConnection("error message"),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name"},
			expected: &expectedResult{
				cacheSize: 0,
				response:  internalServerErrorResponse("Error response from target {127.0.0.\\d}: error message"),
			},
		},
	}

	t.Log("Action recover")
	for _, dataItem := range dataItems {
		assertRandomActionPerformed(t, dataItem, "random", "recover")
	}

	t.Log("Action kill")
	for _, dataItem := range dataItems {
		assertRandomActionPerformed(t, dataItem, "random", "kill")
	}
}

func TestDockerActionForRandomTargetWithInvalidAction(t *testing.T) {
	dataItems := []TestDataForRandomDocker{
		{
			message: "Should receive bad request and not update cache if the action to perform is invalid",
			jobMap: map[string]*config.Job{
				"job name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name"},
			expected:       &expectedResult{cacheSize: 0, response: badRequestResponse("The action {invalidAction} is not supported")},
		},
	}

	t.Log("Action invalidAction")
	for _, dataItem := range dataItems {
		assertRandomActionPerformed(t, dataItem, "random", "invalidAction")
	}
}

func TestDockerActionForRandomTargetWithInvalidDo(t *testing.T) {
	dataItems := []TestDataForRandomDocker{
		{
			message: "Should receive bad request and not update cache if the action to perform is invalid",
			jobMap: map[string]*config.Job{
				"job name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name"},
			expected:       &expectedResult{cacheSize: 0, response: badRequestResponse("Do query parameter {invalidDo} not allowed")},
		},
	}

	t.Log("Action invalidAction")
	for _, dataItem := range dataItems {
		assertRandomActionPerformed(t, dataItem, "invalidDo", "recover")
	}
}

func okResponse(message string) *responseWrapper {
	return &responseWrapper{
		message: message,
		status:  200,
	}
}

func badRequestResponse(error string) *responseWrapper {
	return &responseWrapper{
		message: error + "\n",
		status:  400,
	}
}

func internalServerErrorResponse(error string) *responseWrapper {
	return &responseWrapper{
		message: error + "\n",
		status:  500,
	}
}

func functionWithSuccessResponse() func() (*v1.StatusResponse, error) {
	return func() (*v1.StatusResponse, error) {
		return &v1.StatusResponse{Status: v1.StatusResponse_SUCCESS}, nil
	}
}

func assertRandomActionPerformed(t *testing.T, dataItem TestDataForRandomDocker, do string, action string) {
	t.Run(dataItem.message, func(t *testing.T) {
		c := gocache.New(0)
		server, err := dockerHTTPTestServerWithCacheItems(dataItem.jobMap, dataItem.connectionPool, c, dataItem.cacheItems)
		if err != nil {
			t.Fatal(err)
		}

		status, message, err := dockerPostCallNoTarget(server, dataItem.requestPayload, do, action)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, dataItem.expected.response.status, status)
		assert.Regexp(t, regexp.MustCompile(dataItem.expected.response.message), message)
		assert.Equal(t, dataItem.expected.cacheSize, c.ItemCount())

		server.Close()
	})
}

func withSuccessDockerConnection() *dConnection {
	connection := &network.MockConnection{Status: new(v1.StatusResponse), Err: nil}
	return &dConnection{
		connection: connection,
	}
}

func withFailureDockerConnection() *dConnection {
	statusResponse := new(v1.StatusResponse)
	statusResponse.Status = v1.StatusResponse_FAIL
	connection := &network.MockConnection{Status: statusResponse, Err: nil}
	return &dConnection{
		connection: connection,
	}
}

func withErrorDockerConnection(errorMessage string) *dConnection {
	statusResponse := new(v1.StatusResponse)
	statusResponse.Status = v1.StatusResponse_FAIL
	connection := &network.MockConnection{Status: statusResponse, Err: errors.New(errorMessage)}
	return &dConnection{
		connection: connection,
	}
}

func withFailureToSetDockerConnection(errorMessage string) *dConnection {
	connection := &network.MockFailedConnection{Err: errors.New(errorMessage)}
	return &dConnection{
		connection: connection,
	}
}

func dockerPostCall(server *httptest.Server, details *RequestPayload, action string) (int, string, error) {
	requestBody, _ := json.Marshal(details)
	url := server.URL + "/docker?action=" + action

	return post(requestBody, url)
}

func dockerPostCallNoTarget(server *httptest.Server, details *RequestPayload, do string, action string) (int, string, error) {
	requestBody, _ := json.Marshal(details)
	var url string
	if do != "" {
		url = fmt.Sprintf("%s/docker?do=%s&action=%s", server.URL, do, action)
	} else {
		url = fmt.Sprintf("%s/docker?action=%s", server.URL, action)
	}

	return post(requestBody, url)
}

func post(requestBody []byte, url string) (int, string, error) {
	resp, err := http.Post(url, "", bytes.NewReader(requestBody)) //nolint:gosec
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := ioutil.ReadAll(resp.Body)
		return resp.StatusCode, string(b), nil
	}

	respPayload := &response.Payload{}
	err = json.NewDecoder(resp.Body).Decode(&respPayload)
	if err != nil {
		return resp.StatusCode, "", err
	}
	return respPayload.Status, respPayload.Message, nil
}

func newDockerJob(componentName string, targets ...string) *config.Job {
	return &config.Job{
		ComponentName: componentName,
		FailureType:   config.Docker,
		Target:        targets,
	}
}

func dockerHTTPTestServerWithCacheItems(
	jobMap map[string]*config.Job,
	connectionPool map[string]*dConnection,
	cache *gocache.Cache,
	cacheItems map[cache.Key]func() (*v1.StatusResponse, error),
) (*httptest.Server, error) {
	for key, val := range cacheItems {
		cache.Set(key, val)
	}

	dController := &DController{
		jobs:           jobMap,
		connectionPool: connectionPool,
		cache:          cache,
		loggers:        loggers,
	}

	router := mux.NewRouter()
	router.HandleFunc("/docker", dController.DockerAction).
		Queries("action", "{action}").
		Methods("POST")

	return httptest.NewServer(router), nil
}

func getLoggers() chaoslogger.Loggers {
	allowLevel := &chaoslogger.AllowedLevel{}
	if err := allowLevel.Set("debug"); err != nil {
		fmt.Printf("%v", err)
	}

	return chaoslogger.Loggers{
		ErrLogger: chaoslogger.New(allowLevel, os.Stderr),
		OutLogger: chaoslogger.New(allowLevel, os.Stdout),
	}
}
