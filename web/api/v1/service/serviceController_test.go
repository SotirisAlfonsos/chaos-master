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

	sConn1 := withSuccessServiceConnection(serviceName, target)
	sConn2 := withSuccessServiceConnection("wrong name", "wrong target")
	sConn3 := withFailureServiceConnection(serviceName, "wrong target")
	sConn4 := withErrorServiceConnection("wrong name", target, "error message")

	serviceClients := setClients(jobName, sConn1, sConn2, sConn3, sConn4)
	server := httpTestServer(serviceClients)
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

	sConn1 := withFailureServiceConnection("wrong name", target)
	sConn2 := withErrorServiceConnection(serviceName, "wrong target", "error message")
	sConn3 := withSuccessServiceConnection("wrong name", "wrong target")
	sConn4 := withSuccessServiceConnection(serviceName, target)

	serviceClients := setClients(jobName, sConn1, sConn2, sConn3, sConn4)
	server := httpTestServer(serviceClients)
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

	serviceClientConnection := withSuccessServiceConnection(serviceName, target)

	serviceClients := setClients(jobName, serviceClientConnection)
	server := httpTestServer(serviceClients)
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

	serviceClientConnection := withSuccessServiceConnection(serviceName, target)

	serviceClients := setClients(jobName, serviceClientConnection)
	server := httpTestServer(serviceClients)
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

	serviceClientConnection := withFailureServiceConnection(serviceName, target)

	serviceClients := setClients(jobName, serviceClientConnection)
	server := httpTestServer(serviceClients)
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

	serviceClientConnection := withErrorServiceConnection(serviceName, target, errorMessage)

	serviceClients := setClients(jobName, serviceClientConnection)
	server := httpTestServer(serviceClients)
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

	serviceClientConnection := withSuccessServiceConnection(serviceName, target)

	serviceClients := setClients(jobName, serviceClientConnection)
	server := httpTestServer(serviceClients)
	defer server.Close()

	details := newDetails(jobName, target, serviceName)

	responseStart, err := servicePostCall(server, details, "invalidAction")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 400, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("The action {%s} is not supported", "invalidAction"), responseStart.Error)
}

func withSuccessServiceConnection(service string, target string) *network.ServiceClientConnection {
	return &network.ServiceClientConnection{
		Name:   service,
		Target: target,
		Client: GetMockServiceClient(new(proto.StatusResponse), nil),
	}
}

func withFailureServiceConnection(service string, target string) *network.ServiceClientConnection {
	statusResponse := new(proto.StatusResponse)
	statusResponse.Status = proto.StatusResponse_FAIL
	return &network.ServiceClientConnection{
		Name:   service,
		Target: target,
		Client: GetMockServiceClient(statusResponse, nil),
	}
}

func withErrorServiceConnection(service string, target string, errorMessage string) *network.ServiceClientConnection {
	statusResponse := new(proto.StatusResponse)
	statusResponse.Status = proto.StatusResponse_FAIL
	return &network.ServiceClientConnection{
		Name:   service,
		Target: target,
		Client: GetMockServiceClient(statusResponse, errors.New(errorMessage)),
	}
}

func newDetails(jobName string, target string, serviceName string) *Details {
	return &Details{
		Job:     jobName,
		Service: serviceName,
		Target:  target,
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

func setClients(jobName string, serviceClientConnections ...*network.ServiceClientConnection) map[string][]network.ServiceClientConnection {
	serviceClients := make(map[string][]network.ServiceClientConnection)
	for _, serviceClientConnection := range serviceClientConnections {
		serviceClients[jobName] = append(serviceClients[jobName], *serviceClientConnection)
	}

	return serviceClients
}

func httpTestServer(serviceClients map[string][]network.ServiceClientConnection) *httptest.Server {
	sController := &SController{
		ServiceClients: serviceClients,
		Logger:         logger,
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
