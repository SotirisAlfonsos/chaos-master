package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/cache"
	"github.com/SotirisAlfonsos/chaos-master/chaoslogger"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

var (
	logger = getLogger()
)

type TestData struct {
	message        string
	jobMap         map[string]*config.Job
	connectionPool map[string]*dConnection
	cacheItems     map[string]func() (*v1.StatusResponse, error)
	requestPayload *RequestPayload
	expected       *expectedResult
}

type expectedResult struct {
	cacheSize int
	payload   *ResponsePayload
}

type mockDockerClient struct {
	Status *v1.StatusResponse
	Error  error
}

func GetMockDockerClient(status *v1.StatusResponse, err error) v1.DockerClient {
	return &mockDockerClient{Status: status, Error: err}
}

func (msc *mockDockerClient) Start(ctx context.Context, in *v1.DockerRequest, opts ...grpc.CallOption) (*v1.StatusResponse, error) {
	return msc.Status, msc.Error
}

func (msc *mockDockerClient) Stop(ctx context.Context, in *v1.DockerRequest, opts ...grpc.CallOption) (*v1.StatusResponse, error) {
	return msc.Status, msc.Error
}

type connection struct {
	status *v1.StatusResponse
	err    error
}

func (connection *connection) GetServiceClient(target string) (v1.ServiceClient, error) {
	return nil, nil
}

func (connection *connection) GetDockerClient(target string) (v1.DockerClient, error) {
	return GetMockDockerClient(connection.status, connection.err), nil
}

func (connection *connection) GetCPUClient(target string) (v1.CPUClient, error) {
	return nil, nil
}

func (connection *connection) GetHealthClient(target string) (v1.HealthClient, error) {
	return nil, nil
}

type failedConnection struct {
	err error
}

func (failedConnection *failedConnection) GetServiceClient(target string) (v1.ServiceClient, error) {
	return nil, nil
}

func (failedConnection *failedConnection) GetDockerClient(target string) (v1.DockerClient, error) {
	return nil, failedConnection.err
}

func (failedConnection *failedConnection) GetCPUClient(target string) (v1.CPUClient, error) {
	return nil, failedConnection.err
}

func (failedConnection *failedConnection) GetHealthClient(target string) (v1.HealthClient, error) {
	return nil, nil
}

func TestStartDockerSuccess(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Successfully start single container with specific job, name and target",
			jobMap: map[string]*config.Job{
				"job name":           newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
				"job different name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
				"127.0.0.2": withSuccessDockerConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 0, payload: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
		{
			message: "Successfully start single container with specific job, name and target and remove it from the cache",
			jobMap: map[string]*config.Job{
				"job name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
			},
			cacheItems: map[string]func() (*v1.StatusResponse, error){
				"job name,127.0.0.1": functionWithSuccessResponse(),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 0, payload: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
	}

	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "start")
	}
}

func TestStopDockerSuccess(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Successfully stop single container with specific job, name and target and add it in cache",
			jobMap: map[string]*config.Job{
				"job name":           newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
				"job different name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
				"127.0.0.2": withSuccessDockerConnection(),
			},
			cacheItems: map[string]func() (*v1.StatusResponse, error){
				"job name,127.0.0.2": functionWithSuccessResponse(),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 2, payload: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
		{
			message: "Successfully stop single container with specific job, name and target and dont add it in cache if already exists",
			jobMap: map[string]*config.Job{
				"job name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
			},
			cacheItems: map[string]func() (*v1.StatusResponse, error){
				"job name,127.0.0.1": functionWithSuccessResponse(),
			},
			requestPayload: &RequestPayload{Job: "job name", Container: "container name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 1, payload: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
	}

	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "stop")
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
			expected:       &expectedResult{cacheSize: 0, payload: badRequestResponse("Could not find job {job name does not exist}")},
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
				payload:   badRequestResponse("Container {container name does not exist} is not registered for target {127.0.0.1}"),
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
			expected:       &expectedResult{cacheSize: 0, payload: badRequestResponse("Container {container name} is not registered for target {0.0.0.0}")},
		},
	}

	t.Log("Action start")
	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "start")
	}

	t.Log("Action stop")
	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "stop")
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
			expected:       &expectedResult{cacheSize: 0, payload: internalServerErrorResponse("Failure response from target {127.0.0.1}")},
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
				cacheSize: 0, payload: internalServerErrorResponse("Error response from target {127.0.0.1}: error occurred")},
		},
	}

	t.Log("Action start")
	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "start")
	}

	t.Log("Action stop")
	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "stop")
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
			expected:       &expectedResult{cacheSize: 0, payload: internalServerErrorResponse("Can not get docker connection from target {127.0.0.1}: Can not dial target")},
		},
	}

	t.Log("Action start")
	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "start")
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
			expected:       &expectedResult{cacheSize: 0, payload: badRequestResponse("The action {invalidAction} is not supported")},
		},
	}

	t.Log("Action invalidAction")
	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "invalidAction")
	}
}

func assertActionPerformed(t *testing.T, dataItem TestData, action string) {
	t.Log(dataItem.message)

	cacheManager := cache.NewCacheManager(logger)
	server, err := dockerHTTPTestServerWithCacheItems(dataItem.jobMap, dataItem.connectionPool, cacheManager, dataItem.cacheItems)
	if err != nil {
		t.Fatal(err)
	}

	response, err := dockerPostCall(server, dataItem.requestPayload, action)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, dataItem.expected.payload.Status, response.Status)
	assert.Equal(t, dataItem.expected.payload.Message, response.Message)
	assert.Equal(t, dataItem.expected.payload.Error, response.Error)
	assert.Equal(t, dataItem.expected.cacheSize, cacheManager.ItemCount())

	server.Close()
}

type TestDataForRandomDocker struct {
	message        string
	jobMap         map[string]*config.Job
	connectionPool map[string]*dConnection
	cacheItems     map[string]func() (*v1.StatusResponse, error)
	requestPayload *RequestPayloadNoTarget
	expected       *expectedResult
}

func TestStartRandomDockerSuccess(t *testing.T) {
	dataItems := []TestDataForRandomDocker{
		{
			message: "Successfully start random container with specific job and name",
			jobMap: map[string]*config.Job{
				"job name":           newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
				"job different name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
				"127.0.0.2": withSuccessDockerConnection(),
			},
			requestPayload: &RequestPayloadNoTarget{Job: "job name", Container: "container name"},
			expected:       &expectedResult{cacheSize: 0, payload: okResponse("Response from target {127.0.0.\\d}, {}, {SUCCESS}")},
		},
		{
			message: "Successfully start random container with specific job and name and remove it from the cache",
			jobMap: map[string]*config.Job{
				"job name": newDockerJob("container name", "127.0.0.1"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
			},
			cacheItems: map[string]func() (*v1.StatusResponse, error){
				"job name,127.0.0.1": functionWithSuccessResponse(),
			},
			requestPayload: &RequestPayloadNoTarget{Job: "job name", Container: "container name"},
			expected:       &expectedResult{cacheSize: 0, payload: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
	}

	for _, dataItem := range dataItems {
		assertRandomActionPerformed(t, dataItem, "random", "start")
	}
}

func TestStopRandomDockerSuccess(t *testing.T) {
	dataItems := []TestDataForRandomDocker{
		{
			message: "Successfully stop random container with specific job and name and add it in cache",
			jobMap: map[string]*config.Job{
				"job name":           newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
				"job different name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
				"127.0.0.2": withSuccessDockerConnection(),
			},
			requestPayload: &RequestPayloadNoTarget{Job: "job name", Container: "container name"},
			expected:       &expectedResult{cacheSize: 1, payload: okResponse("Response from target {127.0.0.\\d}, {}, {SUCCESS}")},
		},
		{
			message: "Successfully stop random container with specific job and name and dont add it in cache if it is already present",
			jobMap: map[string]*config.Job{
				"job name": newDockerJob("container name", "127.0.0.1"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
			},
			cacheItems: map[string]func() (*v1.StatusResponse, error){
				"job name,127.0.0.1": functionWithSuccessResponse(),
			},
			requestPayload: &RequestPayloadNoTarget{Job: "job name", Container: "container name"},
			expected:       &expectedResult{cacheSize: 1, payload: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
	}

	for _, dataItem := range dataItems {
		assertRandomActionPerformed(t, dataItem, "random", "stop")
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
			requestPayload: &RequestPayloadNoTarget{Job: "job name does not exist", Container: "container name"},
			expected:       &expectedResult{cacheSize: 0, payload: badRequestResponse("Could not find job {job name does not exist}")},
		},
		{
			message: "Should receive bad request and not update cache for action when container name does not exist for target",
			jobMap: map[string]*config.Job{
				"job name": newDockerJob("container name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*dConnection{
				"127.0.0.1": withSuccessDockerConnection(),
			},
			requestPayload: &RequestPayloadNoTarget{Job: "job name", Container: "container name does not exist"},
			expected: &expectedResult{
				cacheSize: 0,
				payload:   badRequestResponse("Could not find container name {container name does not exist}"),
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
			requestPayload: &RequestPayloadNoTarget{Job: "job name", Container: "container name"},
			expected:       &expectedResult{cacheSize: 0, payload: badRequestResponse("Could not get random target for job {job name}. err: No targets available")},
		},
	}

	t.Log("Action start")
	for _, dataItem := range dataItems {
		assertRandomActionPerformed(t, dataItem, "random", "start")
	}

	t.Log("Action stop")
	for _, dataItem := range dataItems {
		assertRandomActionPerformed(t, dataItem, "random", "stop")
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
			requestPayload: &RequestPayloadNoTarget{Job: "job name", Container: "container name"},
			expected:       &expectedResult{cacheSize: 0, payload: internalServerErrorResponse("Failure response from target {127.0.0.\\d}")},
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
			requestPayload: &RequestPayloadNoTarget{Job: "job name", Container: "container name"},
			expected: &expectedResult{
				cacheSize: 0,
				payload:   internalServerErrorResponse("Error response from target {127.0.0.\\d}: error message"),
			},
		},
	}

	t.Log("Action start")
	for _, dataItem := range dataItems {
		assertRandomActionPerformed(t, dataItem, "random", "start")
	}

	t.Log("Action stop")
	for _, dataItem := range dataItems {
		assertRandomActionPerformed(t, dataItem, "random", "stop")
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
			requestPayload: &RequestPayloadNoTarget{Job: "job name", Container: "container name"},
			expected:       &expectedResult{cacheSize: 0, payload: badRequestResponse("The action {invalidAction} is not supported")},
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
			requestPayload: &RequestPayloadNoTarget{Job: "job name", Container: "container name"},
			expected:       &expectedResult{cacheSize: 0, payload: badRequestResponse("Do query parameter {invalidDo} not allowed")},
		},
	}

	t.Log("Action invalidAction")
	for _, dataItem := range dataItems {
		assertRandomActionPerformed(t, dataItem, "invalidDo", "start")
	}
}

func okResponse(message string) *ResponsePayload {
	return &ResponsePayload{
		Message: message,
		Status:  200,
	}
}

func badRequestResponse(error string) *ResponsePayload {
	return &ResponsePayload{
		Error:  error,
		Status: 400,
	}
}

func internalServerErrorResponse(error string) *ResponsePayload {
	return &ResponsePayload{
		Error:  error,
		Status: 500,
	}
}

func functionWithSuccessResponse() func() (*v1.StatusResponse, error) {
	return func() (*v1.StatusResponse, error) {
		return &v1.StatusResponse{Status: v1.StatusResponse_SUCCESS}, nil
	}
}

func assertRandomActionPerformed(t *testing.T, dataItem TestDataForRandomDocker, do string, action string) {
	t.Log(dataItem.message)

	cacheManager := cache.NewCacheManager(logger)
	server, err := dockerHTTPTestServerWithCacheItems(dataItem.jobMap, dataItem.connectionPool, cacheManager, dataItem.cacheItems)
	if err != nil {
		t.Fatal(err)
	}

	response, err := dockerPostCallNoTarget(server, dataItem.requestPayload, do, action)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, dataItem.expected.payload.Status, response.Status)
	assert.Regexp(t, regexp.MustCompile(dataItem.expected.payload.Message), response.Message)
	assert.Regexp(t, regexp.MustCompile(dataItem.expected.payload.Error), response.Error)
	assert.Equal(t, dataItem.expected.cacheSize, cacheManager.ItemCount())

	server.Close()
}

func withSuccessDockerConnection() *dConnection {
	connection := &connection{status: new(v1.StatusResponse), err: nil}
	return &dConnection{
		connection: connection,
	}
}

func withFailureDockerConnection() *dConnection {
	statusResponse := new(v1.StatusResponse)
	statusResponse.Status = v1.StatusResponse_FAIL
	connection := &connection{status: statusResponse, err: nil}
	return &dConnection{
		connection: connection,
	}
}

func withErrorDockerConnection(errorMessage string) *dConnection {
	statusResponse := new(v1.StatusResponse)
	statusResponse.Status = v1.StatusResponse_FAIL
	connection := &connection{status: statusResponse, err: errors.New(errorMessage)}
	return &dConnection{
		connection: connection,
	}
}

func withFailureToSetDockerConnection(errorMessage string) *dConnection {
	connection := &failedConnection{err: errors.New(errorMessage)}
	return &dConnection{
		connection: connection,
	}
}

func dockerPostCall(server *httptest.Server, details *RequestPayload, action string) (*ResponsePayload, error) {
	requestBody, _ := json.Marshal(details)
	url := server.URL + "/docker?action=" + action

	return post(requestBody, url)
}

func dockerPostCallNoTarget(server *httptest.Server, details *RequestPayloadNoTarget, do string, action string) (*ResponsePayload, error) {
	requestBody, _ := json.Marshal(details)
	var url string
	if do != "" {
		url = fmt.Sprintf("%s/docker?do=%s&action=%s", server.URL, do, action)
	} else {
		url = fmt.Sprintf("%s/docker?action=%s", server.URL, action)
	}

	return post(requestBody, url)
}

func post(requestBody []byte, url string) (*ResponsePayload, error) {
	resp, err := http.Post(url, "", bytes.NewReader(requestBody)) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	response := &ResponsePayload{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return &ResponsePayload{Status: resp.StatusCode}, err
	}
	return response, nil
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
	cacheManager *cache.Manager,
	cacheItems map[string]func() (*v1.StatusResponse, error),
) (*httptest.Server, error) {
	for key, val := range cacheItems {
		err := cacheManager.Register(key, val)
		if err != nil {
			return nil, err
		}
	}

	dController := &DController{
		jobs:           jobMap,
		connectionPool: connectionPool,
		cacheManager:   cacheManager,
		logger:         logger,
	}

	router := mux.NewRouter()
	router.HandleFunc("/docker", dController.DockerAction).
		Queries("action", "{action}").
		Methods("POST")

	return httptest.NewServer(router), nil
}

func getLogger() log.Logger {
	allowLevel := &chaoslogger.AllowedLevel{}
	if err := allowLevel.Set("debug"); err != nil {
		fmt.Printf("%v", err)
	}

	return chaoslogger.New(allowLevel)
}
