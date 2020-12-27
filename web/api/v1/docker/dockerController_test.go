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

	"github.com/SotirisAlfonsos/chaos-master/config"

	"github.com/SotirisAlfonsos/chaos-master/chaoslogger"
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
	connectionPool := make(map[string]*connection)

	connectionPool[target] = withSuccessDockerConnection()
	connectionPool["wrong target"] = withFailureDockerConnection()

	jobMap[jobName] = newJob(containerName, target, "wrong target")
	server := httpTestServer(jobMap, connectionPool)
	defer server.Close()

	details := newDetails(jobName, target, containerName)

	response, err := dockerPostCall(server, details, "start")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 200, response.Status)
	assert.Equal(t, fmt.Sprintf("Response from target {%s}, {}, {SUCCESS}", target), response.Message)
	assert.Equal(t, "", response.Error)
}

func TestStopDockerSuccess(t *testing.T) {
	var jobName = "jobName"
	var dockerName = "dockerName"
	var target = "target"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*connection)

	connectionPool[target] = withSuccessDockerConnection()
	connectionPool["wrong target"] = withErrorDockerConnection("error message")
	jobMap[jobName] = newJob(dockerName, target, "wrong target")

	server := httpTestServer(jobMap, connectionPool)
	defer server.Close()

	details := newDetails(jobName, target, dockerName)

	response, err := dockerPostCall(server, details, "stop")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 200, response.Status)
	assert.Equal(t, fmt.Sprintf("Response from target {%s}, {}, {SUCCESS}", target), response.Message)
	assert.Equal(t, "", response.Error)
}

func TestStartDockerOneOfContainerNameTargetNotExist(t *testing.T) {
	var jobName = "jobName"
	var dockerName = "dockerName"
	var dockerToStart = "different name"
	var target = "target"
	var differentTarget = "different target"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*connection)

	connectionPool[target] = withSuccessDockerConnection()
	jobMap[jobName] = newJob(dockerName, target)

	server := httpTestServer(jobMap, connectionPool)
	defer server.Close()

	details := newDetails(jobName, target, dockerToStart)

	responseStart, err := dockerPostCall(server, details, "start")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 400, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("Container {%s} does not exist on target {%s}", dockerToStart, target), responseStart.Error)

	details = newDetails(jobName, differentTarget, dockerName)

	responseStop, err := dockerPostCall(server, details, "stop")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 400, responseStop.Status)
	assert.Equal(t, fmt.Sprintf("Container {%s} does not exist on target {%s}", dockerName, differentTarget), responseStop.Error)
}

func TestStartStopDockerJobDoesNotExist(t *testing.T) {
	var jobName = "jobName"
	var dockerName = "dockerName"
	var jobToStart = "different target"
	var target = "target"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*connection)

	connectionPool[target] = withSuccessDockerConnection()
	jobMap[jobName] = newJob(dockerName, target)

	server := httpTestServer(jobMap, connectionPool)
	defer server.Close()

	details := newDetails(jobToStart, target, dockerName)

	responseStart, err := dockerPostCall(server, details, "start")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 400, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("Could not find job {%s}", jobToStart), responseStart.Error)

	responseStop, err := dockerPostCall(server, details, "stop")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 400, responseStop.Status)
	assert.Equal(t, fmt.Sprintf("Could not find job {%s}", jobToStart), responseStop.Error)
}

func TestStartStopDockerWithFailureResponseFromSlave(t *testing.T) {
	var jobName = "jobName"
	var dockerName = "dockerName"
	var target = "target"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*connection)

	connectionPool[target] = withFailureDockerConnection()
	jobMap[jobName] = newJob(dockerName, target)

	server := httpTestServer(jobMap, connectionPool)
	defer server.Close()

	details := newDetails(jobName, target, dockerName)

	responseStart, err := dockerPostCall(server, details, "start")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 500, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("Failure response from target {%s}", target), responseStart.Error)

	responseStop, err := dockerPostCall(server, details, "stop")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 500, responseStop.Status)
	assert.Equal(t, fmt.Sprintf("Failure response from target {%s}", target), responseStop.Error)
}

func TestStartStopDockerWithErrorResponseFromSlave(t *testing.T) {
	var jobName = "jobName"
	var dockerName = "dockerName"
	var target = "target"
	var errorMessage = "error message"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*connection)

	connectionPool[target] = withErrorDockerConnection(errorMessage)
	jobMap[jobName] = newJob(dockerName, target)

	server := httpTestServer(jobMap, connectionPool)
	defer server.Close()

	details := newDetails(jobName, target, dockerName)

	responseStart, err := dockerPostCall(server, details, "start")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 500, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("Error response from target {%s}: %s", target, errorMessage), responseStart.Error)

	responseStop, err := dockerPostCall(server, details, "stop")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 500, responseStop.Status)
	assert.Equal(t, fmt.Sprintf("Error response from target {%s}: %s", target, errorMessage), responseStop.Error)
}

func TestDockerWithInvalidAction(t *testing.T) {
	var jobName = "jobName"
	var dockerName = "dockerName"
	var target = "target"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*connection)

	connectionPool[target] = withSuccessDockerConnection()
	jobMap[jobName] = newJob(dockerName, target)

	server := httpTestServer(jobMap, connectionPool)
	defer server.Close()

	details := newDetails(jobName, target, dockerName)

	responseStart, err := dockerPostCall(server, details, "invalidAction")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 400, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("The action {%s} is not supported", "invalidAction"), responseStart.Error)
}

func TestRandomDockerWithInvalidAction(t *testing.T) {
	var jobName = "jobName"
	var dockerName = "dockerName"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*connection)

	connectionPool["target"] = withSuccessDockerConnection()
	jobMap[jobName] = newJob(dockerName, "target")

	server := httpTestServer(jobMap, connectionPool)
	defer server.Close()

	details := newDetailsNoTarget(jobName, dockerName)

	responseStart, err := dockerPostCallNoTarget(server, details, "invalidAction")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 400, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("The action {%s} is not supported", "invalidAction"), responseStart.Error)
}

func TestStartStopRandomDockerOneOfJobOrDockerNameNotExist(t *testing.T) {
	var jobName = "jobName"
	var differentJobName = "different jobName"
	var dockerName = "dockerName"
	var dockerToStart = "different name"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*connection)

	connectionPool["target"] = withSuccessDockerConnection()
	jobMap[jobName] = newJob(dockerName, "target")

	server := httpTestServer(jobMap, connectionPool)
	defer server.Close()

	details := newDetailsNoTarget(jobName, dockerToStart)

	responseStart, err := dockerPostCallNoTarget(server, details, "start")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 400, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("Could not find container name {%s}", dockerToStart), responseStart.Error)

	details = newDetailsNoTarget(differentJobName, dockerName)

	responseStop, err := dockerPostCallNoTarget(server, details, "stop")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 400, responseStop.Status)
	assert.Equal(t, fmt.Sprintf("Could not find job {%s}", differentJobName), responseStop.Error)
}

func withSuccessDockerConnection() *connection {
	return &connection{
		grpcConn:     nil,
		dockerClient: GetMockDockerClient(new(proto.StatusResponse), nil),
	}
}

func withFailureDockerConnection() *connection {
	statusResponse := new(proto.StatusResponse)
	statusResponse.Status = proto.StatusResponse_FAIL
	return &connection{
		grpcConn:     nil,
		dockerClient: GetMockDockerClient(statusResponse, nil),
	}
}

func withErrorDockerConnection(errorMessage string) *connection {
	statusResponse := new(proto.StatusResponse)
	statusResponse.Status = proto.StatusResponse_FAIL
	return &connection{
		grpcConn:     nil,
		dockerClient: GetMockDockerClient(statusResponse, errors.New(errorMessage)),
	}
}

func newDetails(jobName string, target string, containerName string) *Details {
	return &Details{
		Job:       jobName,
		Container: containerName,
		Target:    target,
	}
}

func newDetailsNoTarget(jobName string, containerName string) *DetailsNoTarget {
	return &DetailsNoTarget{
		Job:       jobName,
		Container: containerName,
	}
}

func dockerPostCall(server *httptest.Server, details *Details, action string) (*Response, error) {
	requestBody, _ := json.Marshal(details)
	url := server.URL + "/docker?action=" + action

	return post(requestBody, url)
}

func dockerPostCallNoTarget(server *httptest.Server, details *DetailsNoTarget, action string) (*Response, error) {
	requestBody, _ := json.Marshal(details)
	url := server.URL + "/docker?do=random&action=" + action

	return post(requestBody, url)
}

func post(requestBody []byte, url string) (*Response, error) {
	resp, err := http.Post(url, "", bytes.NewReader(requestBody)) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	response := &Response{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return &Response{Status: resp.StatusCode}, err
	}
	return response, nil
}

func newJob(componentName string, targets ...string) *config.Job {
	return &config.Job{
		ComponentName: componentName,
		FailureType:   config.Service,
		Target:        targets,
	}
}

func httpTestServer(jobMap map[string]*config.Job, connectionPool map[string]*connection) *httptest.Server {
	dController := &DController{
		jobs:           jobMap,
		connectionPool: connectionPool,
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
