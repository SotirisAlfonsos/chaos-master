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

	"github.com/gorilla/mux"

	"github.com/SotirisAlfonsos/chaos-master/chaoslogger"
	"github.com/SotirisAlfonsos/chaos-master/network"
	"github.com/SotirisAlfonsos/chaos-slave/proto"
	"github.com/go-kit/kit/log"
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

	sConn1 := withSuccessDockerConnection(containerName, target)
	sConn2 := withSuccessDockerConnection("wrong name", "wrong target")
	sConn3 := withFailureDockerConnection(containerName, "wrong target")
	sConn4 := withErrorDockerConnection("wrong name", target, "error message")

	dockerClients := setClients(jobName, sConn1, sConn2, sConn3, sConn4)
	server := httpTestServer(dockerClients)
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

	sConn1 := withFailureDockerConnection("wrong name", target)
	sConn2 := withErrorDockerConnection(dockerName, "wrong target", "error message")
	sConn3 := withSuccessDockerConnection("wrong name", "wrong target")
	sConn4 := withSuccessDockerConnection(dockerName, target)

	dockerClients := setClients(jobName, sConn1, sConn2, sConn3, sConn4)
	server := httpTestServer(dockerClients)
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

func TestStartDockerOneOfJobDockerTargetNotExist(t *testing.T) {
	var jobName = "jobName"
	var dockerName = "dockerName"
	var dockerToStart = "different name"
	var target = "target"
	var differentTarget = "different target"

	dockerClientConnection := withSuccessDockerConnection(dockerName, target)

	dockerClients := setClients(jobName, dockerClientConnection)
	server := httpTestServer(dockerClients)
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

	dockerClientConnection := withSuccessDockerConnection(dockerName, target)

	dockerClients := setClients(jobName, dockerClientConnection)
	server := httpTestServer(dockerClients)
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

	dockerClientConnection := withFailureDockerConnection(dockerName, target)

	dockerClients := setClients(jobName, dockerClientConnection)
	server := httpTestServer(dockerClients)
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

	dockerClientConnection := withErrorDockerConnection(dockerName, target, errorMessage)

	dockerClients := setClients(jobName, dockerClientConnection)
	server := httpTestServer(dockerClients)
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

	dockerClientConnection := withSuccessDockerConnection(dockerName, target)

	dockerClients := setClients(jobName, dockerClientConnection)
	server := httpTestServer(dockerClients)
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

	dockerClientConnection := withSuccessDockerConnection(dockerName, "target")

	dockerClients := setClients(jobName, dockerClientConnection)
	server := httpTestServer(dockerClients)
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

	dockerClientConnection := withSuccessDockerConnection(dockerName, "target")

	dockerClients := setClients(jobName, dockerClientConnection)
	server := httpTestServer(dockerClients)
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

func withSuccessDockerConnection(container string, target string) *network.DockerClientConnection {
	return &network.DockerClientConnection{
		Name:   container,
		Target: target,
		Client: GetMockDockerClient(new(proto.StatusResponse), nil),
	}
}

func withFailureDockerConnection(container string, target string) *network.DockerClientConnection {
	statusResponse := new(proto.StatusResponse)
	statusResponse.Status = proto.StatusResponse_FAIL
	return &network.DockerClientConnection{
		Name:   container,
		Target: target,
		Client: GetMockDockerClient(statusResponse, nil),
	}
}

func withErrorDockerConnection(container string, target string, errorMessage string) *network.DockerClientConnection {
	statusResponse := new(proto.StatusResponse)
	statusResponse.Status = proto.StatusResponse_FAIL
	return &network.DockerClientConnection{
		Name:   container,
		Target: target,
		Client: GetMockDockerClient(statusResponse, errors.New(errorMessage)),
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

func setClients(jobName string, dockerClientConnections ...*network.DockerClientConnection) map[string][]network.DockerClientConnection {
	dockerClients := make(map[string][]network.DockerClientConnection)
	for _, dockerClientConnection := range dockerClientConnections {
		dockerClients[jobName] = append(dockerClients[jobName], *dockerClientConnection)
	}

	return dockerClients
}

func httpTestServer(dockerClients map[string][]network.DockerClientConnection) *httptest.Server {
	sController := &DController{
		DockerClients: dockerClients,
		Logger:        logger,
	}

	router := mux.NewRouter()
	router.HandleFunc("/docker", sController.DockerAction).
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
