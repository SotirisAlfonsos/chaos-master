package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"

	"github.com/stretchr/testify/assert"

	"github.com/SotirisAlfonsos/chaos-master/network"

	"google.golang.org/grpc"

	"github.com/SotirisAlfonsos/chaos-master/chaoslogger"

	"github.com/SotirisAlfonsos/chaos-slave/proto"

	"github.com/go-kit/kit/log"
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

	response := servicePostCall(t, server, jobName, target, serviceName, "start")

	assert.Equal(t, 200, response.Status)
	assert.Equal(t, fmt.Sprintf("Response from target <%s>, <>, <SUCCESS>", target), response.Message)
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

	response := servicePostCall(t, server, jobName, target, serviceName, "stop")

	assert.Equal(t, 200, response.Status)
	assert.Equal(t, fmt.Sprintf("Response from target <%s>, <>, <SUCCESS>", target), response.Message)
	assert.Equal(t, "", response.Error)
}

func TestStartServiceNameDoesNotExist(t *testing.T) {
	var jobName = "jobName"
	var serviceName = "serviceName"
	var serviceToStart = "different name"
	var target = "target"

	serviceClientConnection := withSuccessServiceConnection(serviceName, target)

	serviceClients := setClients(jobName, serviceClientConnection)
	server := httpTestServer(serviceClients)
	defer server.Close()

	responseStart := servicePostCall(t, server, jobName, target, serviceToStart, "start")

	assert.Equal(t, 400, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("Service <%s> does not exist on target <%s>", serviceToStart, target), responseStart.Message)
	assert.Equal(t, "", responseStart.Error)

	responseStop := servicePostCall(t, server, jobName, target, serviceToStart, "stop")

	assert.Equal(t, 400, responseStop.Status)
	assert.Equal(t, fmt.Sprintf("Service <%s> does not exist on target <%s>", serviceToStart, target), responseStop.Message)
	assert.Equal(t, "", responseStop.Error)
}

func TestStartStopServiceTargetDoesNotExist(t *testing.T) {
	var jobName = "jobName"
	var serviceName = "serviceName"
	var targetToStart = "different target"
	var target = "target"

	serviceClientConnection := withSuccessServiceConnection(serviceName, target)

	serviceClients := setClients(jobName, serviceClientConnection)
	server := httpTestServer(serviceClients)
	defer server.Close()

	responseStart := servicePostCall(t, server, jobName, targetToStart, serviceName, "start")

	assert.Equal(t, 400, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("Service <%s> does not exist on target <%s>", serviceName, targetToStart), responseStart.Message)
	assert.Equal(t, "", responseStart.Error)

	responseStop := servicePostCall(t, server, jobName, targetToStart, serviceName, "stop")

	assert.Equal(t, 400, responseStop.Status)
	assert.Equal(t, fmt.Sprintf("Service <%s> does not exist on target <%s>", serviceName, targetToStart), responseStop.Message)
	assert.Equal(t, "", responseStop.Error)
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

	responseStart := servicePostCall(t, server, jobToStart, target, serviceName, "start")

	assert.Equal(t, 400, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("Could not find job <%s>", jobToStart), responseStart.Message)
	assert.Equal(t, "", responseStart.Error)

	responseStop := servicePostCall(t, server, jobToStart, target, serviceName, "stop")

	assert.Equal(t, 400, responseStop.Status)
	assert.Equal(t, fmt.Sprintf("Could not find job <%s>", jobToStart), responseStop.Message)
	assert.Equal(t, "", responseStop.Error)
}

func TestStartStopServiceWithFailureResponseFromSlave(t *testing.T) {
	var jobName = "jobName"
	var serviceName = "serviceName"
	var target = "target"

	serviceClientConnection := withFailureServiceConnection(serviceName, target)

	serviceClients := setClients(jobName, serviceClientConnection)
	server := httpTestServer(serviceClients)
	defer server.Close()

	responseStart := servicePostCall(t, server, jobName, target, serviceName, "start")

	assert.Equal(t, 500, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("Failure response from target <%s>", target), responseStart.Message)
	assert.Equal(t, "", responseStart.Error)

	responseStop := servicePostCall(t, server, jobName, target, serviceName, "stop")

	assert.Equal(t, 500, responseStop.Status)
	assert.Equal(t, fmt.Sprintf("Failure response from target <%s>", target), responseStop.Message)
	assert.Equal(t, "", responseStop.Error)
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

	responseStart := servicePostCall(t, server, jobName, target, serviceName, "start")

	assert.Equal(t, 500, responseStart.Status)
	assert.Equal(t, fmt.Sprintf("Error response from target <%s>", target), responseStart.Message)
	assert.Equal(t, errorMessage, responseStart.Error)

	responseStop := servicePostCall(t, server, jobName, target, serviceName, "stop")

	assert.Equal(t, 500, responseStop.Status)
	assert.Equal(t, fmt.Sprintf("Error response from target <%s>", target), responseStop.Message)
	assert.Equal(t, errorMessage, responseStop.Error)
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

func servicePostCall(t *testing.T, server *httptest.Server, jobName string, target string, serviceName string, action string) *Response {
	url := server.URL + "/job/" + jobName + "/target/" + target + "/service/" + serviceName + "/" + action
	resp, err := http.Post(url, "", nil) //nolint:gosec
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	response := &Response{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}
	return response
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
	router.HandleFunc("/job/{jobName}/target/{target}/service/{name}/start", sController.StartService)
	router.HandleFunc("/job/{jobName}/target/{target}/service/{name}/stop", sController.StopService)
	return httptest.NewServer(router)
}

func getLogger() log.Logger {
	allowLevel := &chaoslogger.AllowedLevel{}
	if err := allowLevel.Set("debug"); err != nil {
		fmt.Printf("%v", err)
	}

	return chaoslogger.New(allowLevel)
}
