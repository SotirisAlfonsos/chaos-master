package service

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

	"github.com/gorilla/mux"

	"github.com/SotirisAlfonsos/chaos-master/chaoslogger"
	"github.com/SotirisAlfonsos/chaos-slave/proto"
	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

var (
	logger = getLogger()
)

type mockServiceClient struct {
	Status *proto.StatusResponse
	Error  error
}

func GetMockServiceClient(status *proto.StatusResponse, err error) proto.ServiceClient {
	return &mockServiceClient{Status: status, Error: err}
}

func (msc *mockServiceClient) Start(ctx context.Context, in *proto.ServiceRequest, opts ...grpc.CallOption) (*proto.StatusResponse, error) {
	return msc.Status, msc.Error
}

func (msc *mockServiceClient) Stop(ctx context.Context, in *proto.ServiceRequest, opts ...grpc.CallOption) (*proto.StatusResponse, error) {
	return msc.Status, msc.Error
}

func TestStartServiceSuccess(t *testing.T) {
	var jobName = "jobName"
	var serviceName = "serviceName"
	var target = "target"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*connection)

	connectionPool[target] = withSuccessServiceConnection()
	connectionPool["wrong target"] = withFailureServiceConnection()

	jobMap[jobName] = newJob(serviceName, target, "wrong target")
	server := httpTestServer(jobMap, connectionPool)
	defer server.Close()

	details := newDetails(jobName, target, serviceName)

	response, err := servicePostCall(server, details, "start")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 200, response.Status)
	assert.Equal(t, fmt.Sprintf("Response from target {%s}, {}, {SUCCESS}", target), response.Message)
	assert.Equal(t, "", response.Error)
}

func TestStopServiceSuccess(t *testing.T) {
	var jobName = "jobName"
	var serviceName = "serviceName"
	var target = "target"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*connection)

	connectionPool[target] = withSuccessServiceConnection()
	connectionPool["wrong target"] = withErrorServiceConnection("error message")
	jobMap[jobName] = newJob(serviceName, target, "wrong target")

	server := httpTestServer(jobMap, connectionPool)
	defer server.Close()

	details := newDetails(jobName, target, serviceName)

	response, err := servicePostCall(server, details, "stop")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 200, response.Status)
	assert.Equal(t, fmt.Sprintf("Response from target {%s}, {}, {SUCCESS}", target), response.Message)
	assert.Equal(t, "", response.Error)
}

func TestStartServiceOneOfServiceNameOrTargetNotExist(t *testing.T) {
	var jobName = "jobName"
	var serviceName = "serviceName"
	var differentServiceName = "different name"
	var target = "target"
	var differentTarget = "different target"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*connection)

	connectionPool[target] = withSuccessServiceConnection()
	jobMap[jobName] = newJob(serviceName, target)

	server := httpTestServer(jobMap, connectionPool)
	defer server.Close()

	details := newDetails(jobName, target, differentServiceName)

	responseStart, err := servicePostCall(server, details, "start")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 400, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("Service {%s} does not exist on target {%s}", differentServiceName, target), responseStart.Error)

	details = newDetails(jobName, differentTarget, serviceName)

	responseStop, err := servicePostCall(server, details, "stop")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 400, responseStop.Status)
	assert.Equal(t, fmt.Sprintf("Service {%s} does not exist on target {%s}", serviceName, differentTarget), responseStop.Error)
}

func TestStartStopServiceJobDoesNotExist(t *testing.T) {
	var jobName = "jobName"
	var serviceName = "serviceName"
	var jobToStart = "different target"
	var target = "target"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*connection)

	connectionPool[target] = withSuccessServiceConnection()
	jobMap[jobName] = newJob(serviceName, target)

	server := httpTestServer(jobMap, connectionPool)
	defer server.Close()

	details := newDetails(jobToStart, target, serviceName)

	responseStart, err := servicePostCall(server, details, "start")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 400, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("Could not find job {%s}", jobToStart), responseStart.Error)

	responseStop, err := servicePostCall(server, details, "stop")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 400, responseStop.Status)
	assert.Equal(t, fmt.Sprintf("Could not find job {%s}", jobToStart), responseStop.Error)
}

func TestStartStopServiceWithFailureResponseFromSlave(t *testing.T) {
	var jobName = "jobName"
	var serviceName = "serviceName"
	var target = "target"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*connection)

	connectionPool[target] = withFailureServiceConnection()
	jobMap[jobName] = newJob(serviceName, target)

	server := httpTestServer(jobMap, connectionPool)
	defer server.Close()

	details := newDetails(jobName, target, serviceName)

	responseStart, err := servicePostCall(server, details, "start")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 500, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("Failure response from target {%s}", target), responseStart.Error)

	responseStop, err := servicePostCall(server, details, "stop")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 500, responseStop.Status)
	assert.Equal(t, fmt.Sprintf("Failure response from target {%s}", target), responseStop.Error)
}

func TestStartStopServiceWithErrorResponseFromSlave(t *testing.T) {
	var jobName = "jobName"
	var serviceName = "serviceName"
	var target = "target"
	var errorMessage = "error message"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*connection)

	connectionPool[target] = withErrorServiceConnection(errorMessage)
	jobMap[jobName] = newJob(serviceName, target)

	server := httpTestServer(jobMap, connectionPool)
	defer server.Close()

	details := newDetails(jobName, target, serviceName)

	responseStart, err := servicePostCall(server, details, "start")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 500, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("Error response from target {%s}: %s", target, errorMessage), responseStart.Error)

	responseStop, err := servicePostCall(server, details, "stop")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 500, responseStop.Status)
	assert.Equal(t, fmt.Sprintf("Error response from target {%s}: %s", target, errorMessage), responseStop.Error)
}

func TestServiceWithInvalidAction(t *testing.T) {
	var jobName = "jobName"
	var serviceName = "serviceName"
	var target = "target"

	jobMap := make(map[string]*config.Job)
	connectionPool := make(map[string]*connection)

	connectionPool[target] = withSuccessServiceConnection()
	jobMap[jobName] = newJob(serviceName, target)

	server := httpTestServer(jobMap, connectionPool)
	defer server.Close()

	details := newDetails(jobName, target, serviceName)

	responseStart, err := servicePostCall(server, details, "invalidAction")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 400, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("The action {%s} is not supported", "invalidAction"), responseStart.Error)
}

func withSuccessServiceConnection() *connection {
	return &connection{
		grpcConn:      nil,
		serviceClient: GetMockServiceClient(new(proto.StatusResponse), nil),
	}
}

func withFailureServiceConnection() *connection {
	statusResponse := new(proto.StatusResponse)
	statusResponse.Status = proto.StatusResponse_FAIL
	return &connection{
		grpcConn:      nil,
		serviceClient: GetMockServiceClient(statusResponse, nil),
	}
}

func withErrorServiceConnection(errorMessage string) *connection {
	statusResponse := new(proto.StatusResponse)
	statusResponse.Status = proto.StatusResponse_FAIL
	return &connection{
		grpcConn:      nil,
		serviceClient: GetMockServiceClient(statusResponse, errors.New(errorMessage)),
	}
}

func newDetails(jobName string, target string, serviceName string) *Details {
	return &Details{
		Job:         jobName,
		ServiceName: serviceName,
		Target:      target,
	}
}

func servicePostCall(server *httptest.Server, details *Details, action string) (*Response, error) {
	requestBody, _ := json.Marshal(details)
	url := server.URL + "/service?action=" + action

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
	sController := &SController{
		jobs:           jobMap,
		connectionPool: connectionPool,
		logger:         logger,
	}

	router := mux.NewRouter()
	router.HandleFunc("/service", sController.ServiceAction).
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
