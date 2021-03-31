package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/SotirisAlfonsos/gocache"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/SotirisAlfonsos/chaos-master/pkg/cache"
	"github.com/SotirisAlfonsos/chaos-master/pkg/chaoslogger"
	"github.com/SotirisAlfonsos/chaos-master/pkg/network"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/response"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

var (
	loggers = getLoggers()
)

type TestData struct {
	message        string
	jobMap         map[string]*config.Job
	connectionPool map[string]*sConnection
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

func TestStartServiceSuccess(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Successfully start single container with specific job, name and target",
			jobMap: map[string]*config.Job{
				"job name":           newServiceJob("service name", "127.0.0.1", "127.0.0.2"),
				"job different name": newServiceJob("service name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*sConnection{
				"127.0.0.1": withSuccessServiceConnection(),
				"127.0.0.2": withSuccessServiceConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", ServiceName: "service name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 0, response: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
		{
			message: "Successfully start single container with specific job, name and target and remove it from the cache",
			jobMap: map[string]*config.Job{
				"job name": newServiceJob("service name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*sConnection{
				"127.0.0.1": withSuccessServiceConnection(),
			},
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
			},
			requestPayload: &RequestPayload{Job: "job name", ServiceName: "service name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 0, response: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
	}

	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "start")
	}
}

func TestStopServiceSuccess(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Successfully stop single service with specific job, name and target and add it in cache",
			jobMap: map[string]*config.Job{
				"job name":           newServiceJob("service name", "127.0.0.1", "127.0.0.2"),
				"job different name": newServiceJob("service name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*sConnection{
				"127.0.0.1": withSuccessServiceConnection(),
				"127.0.0.2": withSuccessServiceConnection(),
			},
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job name", Target: "127.0.0.2"}: functionWithSuccessResponse(),
			},
			requestPayload: &RequestPayload{Job: "job name", ServiceName: "service name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 2, response: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
		{
			message: "Successfully stop single service with specific job, name and target and dont add it in cache if already exists",
			jobMap: map[string]*config.Job{
				"job name": newServiceJob("service name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*sConnection{
				"127.0.0.1": withSuccessServiceConnection(),
			},
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
			},
			requestPayload: &RequestPayload{Job: "job name", ServiceName: "service name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 1, response: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
	}

	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "stop")
	}
}

func TestServiceActionOneOfJobContainerNameTargetDoesNotExist(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Should receive bad request and not update cache for action when job name does not exist",
			jobMap: map[string]*config.Job{
				"job name":           newServiceJob("service name", "127.0.0.1", "127.0.0.2"),
				"job different name": newServiceJob("service name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*sConnection{
				"127.0.0.1": withSuccessServiceConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name does not exist", ServiceName: "service name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 0, response: badRequestResponse("Could not find job {job name does not exist}")},
		},
		{
			message: "Should receive bad request and not update cache for action when service name does not exist for target",
			jobMap: map[string]*config.Job{
				"job name": newServiceJob("service name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*sConnection{
				"127.0.0.1": withSuccessServiceConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", ServiceName: "service name does not exist", Target: "127.0.0.1"},
			expected: &expectedResult{
				cacheSize: 0,
				response:  badRequestResponse("Service {service name does not exist} is not registered for target {127.0.0.1}"),
			},
		},
		{
			message: "Should receive bad request and not update cache for action when target does not exist for the job and service specified",
			jobMap: map[string]*config.Job{
				"job name": newServiceJob("service name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*sConnection{
				"127.0.0.1": withSuccessServiceConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", ServiceName: "service name", Target: "0.0.0.0"},
			expected:       &expectedResult{cacheSize: 0, response: badRequestResponse("Service {service name} is not registered for target {0.0.0.0}")},
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

func TestServiceActionFailure(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Should receive internal server error and not update cache if the request fails on bot",
			jobMap: map[string]*config.Job{
				"job name":           newServiceJob("service name", "127.0.0.1", "127.0.0.2"),
				"job different name": newServiceJob("service name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*sConnection{
				"127.0.0.1": withFailureServiceConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", ServiceName: "service name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 0, response: internalServerErrorResponse("Failure response from target {127.0.0.1}")},
		},
		{
			message: "Should receive internal server error and not update cache if the request errors on bot",
			jobMap: map[string]*config.Job{
				"job name": newServiceJob("service name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*sConnection{
				"127.0.0.1": withErrorServiceConnection("error occurred"),
			},
			requestPayload: &RequestPayload{Job: "job name", ServiceName: "service name", Target: "127.0.0.1"},
			expected: &expectedResult{
				cacheSize: 0, response: internalServerErrorResponse("Error response from target {127.0.0.1}: error occurred")},
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
				"job name": newServiceJob("service name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*sConnection{
				"127.0.0.1": withFailureToSetServiceConnection("Can not dial target"),
			},
			requestPayload: &RequestPayload{Job: "job name", ServiceName: "service name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 0, response: internalServerErrorResponse("Can not get service connection from target {127.0.0.1}: Can not dial target")},
		},
	}

	t.Log("Action start")
	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "start")
	}
}

func TestServiceWithInvalidAction(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Should receive bad request and not update cache if the action to perform is invalid",
			jobMap: map[string]*config.Job{
				"job name": newServiceJob("service name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*sConnection{
				"127.0.0.1": withSuccessServiceConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", ServiceName: "service name", Target: "127.0.0.1"},
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
		cacheManager := gocache.New(0)
		server, err := serviceHTTPTestServerWithCacheItems(dataItem.jobMap, dataItem.connectionPool, cacheManager, dataItem.cacheItems)
		if err != nil {
			t.Fatal(err)
		}

		status, message, err := servicePostCall(server, dataItem.requestPayload, action)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, dataItem.expected.response.status, status)
		assert.Equal(t, dataItem.expected.response.message, message)
		assert.Equal(t, dataItem.expected.cacheSize, cacheManager.ItemCount())

		server.Close()
	})
}

func newServiceJob(componentName string, targets ...string) *config.Job {
	return &config.Job{
		ComponentName: componentName,
		FailureType:   config.Service,
		Target:        targets,
	}
}

func withSuccessServiceConnection() *sConnection {
	connection := &network.MockConnection{Status: new(v1.StatusResponse), Err: nil}
	return &sConnection{
		connection: connection,
	}
}

func withFailureServiceConnection() *sConnection {
	statusResponse := new(v1.StatusResponse)
	statusResponse.Status = v1.StatusResponse_FAIL
	connection := &network.MockConnection{Status: statusResponse, Err: nil}
	return &sConnection{
		connection: connection,
	}
}

func withErrorServiceConnection(errorMessage string) *sConnection {
	statusResponse := new(v1.StatusResponse)
	statusResponse.Status = v1.StatusResponse_FAIL
	connection := &network.MockConnection{Status: statusResponse, Err: errors.New(errorMessage)}
	return &sConnection{
		connection: connection,
	}
}

func withFailureToSetServiceConnection(errorMessage string) *sConnection {
	connection := &network.MockFailedConnection{Err: errors.New(errorMessage)}
	return &sConnection{
		connection: connection,
	}
}

func serviceHTTPTestServerWithCacheItems(
	jobMap map[string]*config.Job,
	connectionPool map[string]*sConnection,
	cache *gocache.Cache,
	cacheItems map[cache.Key]func() (*v1.StatusResponse, error),
) (*httptest.Server, error) {
	for key, val := range cacheItems {
		cache.Set(key, val)
	}

	sController := &SController{
		jobs:           jobMap,
		connectionPool: connectionPool,
		cache:          cache,
		loggers:        loggers,
	}

	router := mux.NewRouter()
	router.HandleFunc("/service", sController.ServiceAction).
		Queries("action", "{action}").
		Methods("POST")

	return httptest.NewServer(router), nil
}

func servicePostCall(server *httptest.Server, details *RequestPayload, action string) (int, string, error) {
	requestBody, _ := json.Marshal(details)
	url := server.URL + "/service?action=" + action

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

func getLoggers() chaoslogger.Loggers {
	allowLevel := &chaoslogger.AllowedLevel{}
	if err := allowLevel.Set("debug"); err != nil {
		fmt.Printf("%v", err)
	}

	return chaoslogger.Loggers{
		OutLogger: chaoslogger.New(allowLevel, os.Stdout),
		ErrLogger: chaoslogger.New(allowLevel, os.Stderr),
	}
}
