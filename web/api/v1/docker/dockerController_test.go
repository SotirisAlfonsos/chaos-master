package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SotirisAlfonsos/chaos-master/cache"
	"github.com/SotirisAlfonsos/chaos-master/chaoslogger"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/SotirisAlfonsos/chaos-slave/proto"
	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

var (
	logger = getLogger()
)

type mockDockerClient struct {
	Status *proto.StatusResponse
	Error  error
}

func GetMockDockerClient(status *proto.StatusResponse, err error) proto.DockerClient {
	return &mockDockerClient{Status: status, Error: err}
}

func (msc *mockDockerClient) Start(ctx context.Context, in *proto.DockerRequest, opts ...grpc.CallOption) (*proto.StatusResponse, error) {
	return msc.Status, msc.Error
}

func (msc *mockDockerClient) Stop(ctx context.Context, in *proto.DockerRequest, opts ...grpc.CallOption) (*proto.StatusResponse, error) {
	return msc.Status, msc.Error
}

func TestStartDockerSuccess(t *testing.T) {
	var jobName = "jobName"
	var containerName = "containerName"
	var target = "target"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*dConnection)

	connectionPool[target] = withSuccessDockerConnection()
	connectionPool["wrong target"] = withFailureDockerConnection()
	jobMap[jobName] = newDockerJob(containerName, target, "wrong target")

	cacheManager := cache.NewCacheManager(logger)

	server := dockerHTTPTestServer(jobMap, connectionPool, cacheManager)
	defer server.Close()

	details := newDockerRequestPayload(jobName, target, containerName)

	response, err := dockerPostCall(server, details, "start")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 200, response.Status)
	assert.Equal(t, fmt.Sprintf("Response from target {%s}, {}, {SUCCESS}", target), response.Message)
	assert.Equal(t, "", response.Error)
	assert.Equal(t, 0, cacheManager.ItemCount())
}

func TestStopDockerSuccess(t *testing.T) {
	var jobName = "jobName"
	var dockerName = "dockerName"
	var target = "target"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*dConnection)

	connectionPool[target] = withSuccessDockerConnection()
	connectionPool["wrong target"] = withErrorDockerConnection("error message")
	jobMap[jobName] = newDockerJob(dockerName, target, "wrong target")

	cacheManager := cache.NewCacheManager(logger)

	server := dockerHTTPTestServer(jobMap, connectionPool, cacheManager)
	defer server.Close()

	details := newDockerRequestPayload(jobName, target, dockerName)

	response, err := dockerPostCall(server, details, "stop")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 200, response.Status)
	assert.Equal(t, fmt.Sprintf("Response from target {%s}, {}, {SUCCESS}", target), response.Message)
	assert.Equal(t, "", response.Error)
	assert.Equal(t, 1, cacheManager.ItemCount())
}

func TestStartDockerOneOfContainerNameTargetNotExist(t *testing.T) {
	var jobName = "jobName"
	var dockerName = "dockerName"
	var dockerToStart = "different name"
	var target = "target"
	var differentTarget = "different target"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*dConnection)

	connectionPool[target] = withSuccessDockerConnection()
	jobMap[jobName] = newDockerJob(dockerName, target)

	cacheManager := cache.NewCacheManager(logger)

	server := dockerHTTPTestServer(jobMap, connectionPool, cache.NewCacheManager(logger))
	defer server.Close()

	details := newDockerRequestPayload(jobName, target, dockerToStart)

	responseStart, err := dockerPostCall(server, details, "start")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 400, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("Container {%s} does not exist on target {%s}", dockerToStart, target), responseStart.Error)
	assert.Equal(t, 0, cacheManager.ItemCount())

	details = newDockerRequestPayload(jobName, differentTarget, dockerName)

	responseStop, err := dockerPostCall(server, details, "stop")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 400, responseStop.Status)
	assert.Equal(t, fmt.Sprintf("Container {%s} does not exist on target {%s}", dockerName, differentTarget), responseStop.Error)
	assert.Equal(t, 0, cacheManager.ItemCount())
}

func TestStartStopDockerJobDoesNotExist(t *testing.T) {
	var jobName = "jobName"
	var dockerName = "dockerName"
	var jobToStart = "different target"
	var target = "target"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*dConnection)

	connectionPool[target] = withSuccessDockerConnection()
	jobMap[jobName] = newDockerJob(dockerName, target)

	cacheManager := cache.NewCacheManager(logger)

	server := dockerHTTPTestServer(jobMap, connectionPool, cache.NewCacheManager(logger))
	defer server.Close()

	details := newDockerRequestPayload(jobToStart, target, dockerName)

	responseStart, err := dockerPostCall(server, details, "start")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 400, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("Could not find job {%s}", jobToStart), responseStart.Error)
	assert.Equal(t, 0, cacheManager.ItemCount())

	responseStop, err := dockerPostCall(server, details, "stop")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 400, responseStop.Status)
	assert.Equal(t, fmt.Sprintf("Could not find job {%s}", jobToStart), responseStop.Error)
	assert.Equal(t, 0, cacheManager.ItemCount())
}

func TestStartStopDockerWithFailureResponseFromSlave(t *testing.T) {
	var jobName = "jobName"
	var dockerName = "dockerName"
	var target = "target"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*dConnection)

	connectionPool[target] = withFailureDockerConnection()
	jobMap[jobName] = newDockerJob(dockerName, target)

	cacheManager := cache.NewCacheManager(logger)

	server := dockerHTTPTestServer(jobMap, connectionPool, cache.NewCacheManager(logger))
	defer server.Close()

	details := newDockerRequestPayload(jobName, target, dockerName)

	responseStart, err := dockerPostCall(server, details, "start")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 500, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("Failure response from target {%s}", target), responseStart.Error)
	assert.Equal(t, 0, cacheManager.ItemCount())

	responseStop, err := dockerPostCall(server, details, "stop")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 500, responseStop.Status)
	assert.Equal(t, fmt.Sprintf("Failure response from target {%s}", target), responseStop.Error)
	assert.Equal(t, 0, cacheManager.ItemCount())
}

func TestStartStopDockerWithErrorResponseFromSlave(t *testing.T) {
	var jobName = "jobName"
	var dockerName = "dockerName"
	var target = "target"
	var errorMessage = "error message"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*dConnection)

	connectionPool[target] = withErrorDockerConnection(errorMessage)
	jobMap[jobName] = newDockerJob(dockerName, target)

	cacheManager := cache.NewCacheManager(logger)

	server := dockerHTTPTestServer(jobMap, connectionPool, cache.NewCacheManager(logger))
	defer server.Close()

	details := newDockerRequestPayload(jobName, target, dockerName)

	responseStart, err := dockerPostCall(server, details, "start")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 500, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("Error response from target {%s}: %s", target, errorMessage), responseStart.Error)
	assert.Equal(t, 0, cacheManager.ItemCount())

	responseStop, err := dockerPostCall(server, details, "stop")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 500, responseStop.Status)
	assert.Equal(t, fmt.Sprintf("Error response from target {%s}: %s", target, errorMessage), responseStop.Error)
	assert.Equal(t, 0, cacheManager.ItemCount())
}

func TestDockerWithInvalidAction(t *testing.T) {
	var jobName = "jobName"
	var dockerName = "dockerName"
	var target = "target"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*dConnection)

	connectionPool[target] = withSuccessDockerConnection()
	jobMap[jobName] = newDockerJob(dockerName, target)

	cacheManager := cache.NewCacheManager(logger)

	server := dockerHTTPTestServer(jobMap, connectionPool, cache.NewCacheManager(logger))
	defer server.Close()

	details := newDockerRequestPayload(jobName, target, dockerName)

	responseStart, err := dockerPostCall(server, details, "invalidAction")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 400, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("The action {%s} is not supported", "invalidAction"), responseStart.Error)
	assert.Equal(t, 0, cacheManager.ItemCount())
}

func TestRandomDockerWithInvalidAction(t *testing.T) {
	var jobName = "jobName"
	var dockerName = "dockerName"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*dConnection)

	connectionPool["target"] = withSuccessDockerConnection()
	jobMap[jobName] = newDockerJob(dockerName, "target")

	cacheManager := cache.NewCacheManager(logger)

	server := dockerHTTPTestServer(jobMap, connectionPool, cache.NewCacheManager(logger))
	defer server.Close()

	details := newDockerRequestPayloadNoTarget(jobName, dockerName)

	responseStart, err := dockerPostCallNoTarget(server, details, "invalidAction")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 400, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("The action {%s} is not supported", "invalidAction"), responseStart.Error)
	assert.Equal(t, 0, cacheManager.ItemCount())
}

func TestStartStopRandomDockerOneOfJobOrDockerNameNotExist(t *testing.T) {
	var jobName = "jobName"
	var differentJobName = "different jobName"
	var dockerName = "dockerName"
	var dockerToStart = "different name"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*dConnection)

	connectionPool["target"] = withSuccessDockerConnection()
	jobMap[jobName] = newDockerJob(dockerName, "target")

	cacheManager := cache.NewCacheManager(logger)

	server := dockerHTTPTestServer(jobMap, connectionPool, cacheManager)
	defer server.Close()

	details := newDockerRequestPayloadNoTarget(jobName, dockerToStart)

	responseStart, err := dockerPostCallNoTarget(server, details, "start")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 400, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("Could not find container name {%s}", dockerToStart), responseStart.Error)

	details = newDockerRequestPayloadNoTarget(differentJobName, dockerName)

	responseStop, err := dockerPostCallNoTarget(server, details, "stop")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 400, responseStop.Status)
	assert.Equal(t, fmt.Sprintf("Could not find job {%s}", differentJobName), responseStop.Error)
	assert.Equal(t, 0, cacheManager.ItemCount())
}

func TestStartStopRandomDockerDoActionNotImplemented(t *testing.T) {
	var jobName = "jobName"
	var dockerName = "dockerName"
	var dockerToStart = "different name"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*dConnection)

	connectionPool["target"] = withSuccessDockerConnection()
	jobMap[jobName] = newDockerJob(dockerName, "target")

	cacheManager := cache.NewCacheManager(logger)

	server := dockerHTTPTestServer(jobMap, connectionPool, cacheManager)
	defer server.Close()

	details := newDockerRequestPayloadNoTarget(jobName, dockerToStart)

	responseStart, err := dockerPostCallNotImplementedDo(server, details, "<random>!")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 400, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("Do query parameter {%s} not allowed", "<random>!"), responseStart.Error)
}

func withSuccessDockerConnection() *dConnection {
	return &dConnection{
		grpcConn:     nil,
		dockerClient: GetMockDockerClient(new(proto.StatusResponse), nil),
	}
}

func withFailureDockerConnection() *dConnection {
	statusResponse := new(proto.StatusResponse)
	statusResponse.Status = proto.StatusResponse_FAIL
	return &dConnection{
		grpcConn:     nil,
		dockerClient: GetMockDockerClient(statusResponse, nil),
	}
}

func withErrorDockerConnection(errorMessage string) *dConnection {
	statusResponse := new(proto.StatusResponse)
	statusResponse.Status = proto.StatusResponse_FAIL
	return &dConnection{
		grpcConn:     nil,
		dockerClient: GetMockDockerClient(statusResponse, errors.New(errorMessage)),
	}
}

func newDockerRequestPayload(jobName string, target string, containerName string) *RequestPayload {
	return &RequestPayload{
		Job:       jobName,
		Container: containerName,
		Target:    target,
	}
}

func newDockerRequestPayloadNoTarget(jobName string, containerName string) *RequestPayloadNoTarget {
	return &RequestPayloadNoTarget{
		Job:       jobName,
		Container: containerName,
	}
}

func dockerPostCall(server *httptest.Server, details *RequestPayload, action string) (*ResponsePayload, error) {
	requestBody, _ := json.Marshal(details)
	url := server.URL + "/docker?action=" + action

	return post(requestBody, url)
}

func dockerPostCallNoTarget(server *httptest.Server, details *RequestPayloadNoTarget, action string) (*ResponsePayload, error) {
	requestBody, _ := json.Marshal(details)
	url := server.URL + "/docker?do=random&action=" + action

	return post(requestBody, url)
}

func dockerPostCallNotImplementedDo(server *httptest.Server, details *RequestPayloadNoTarget, do string) (*ResponsePayload, error) {
	requestBody, _ := json.Marshal(details)
	url := server.URL + "/docker?do=" + do + "&action=start"

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
		FailureType:   config.Service,
		Target:        targets,
	}
}

func dockerHTTPTestServer(jobMap map[string]*config.Job, connectionPool map[string]*dConnection, cache *cache.Manager) *httptest.Server {
	dController := &DController{
		jobs:           jobMap,
		connectionPool: connectionPool,
		cacheManager:   cache,
		logger:         logger,
	}

	router := mux.NewRouter()
	router.HandleFunc("/docker", dController.DockerAction).
		Queries("action", "{action}").
		Methods("POST")
	return httptest.NewServer(router)
}

func getLogger() log.Logger {
	allowLevel := &chaoslogger.AllowedLevel{}
	if err := allowLevel.Set("debug"); err != nil {
		fmt.Printf("%v", err)
	}

	return chaoslogger.New(allowLevel)
}
