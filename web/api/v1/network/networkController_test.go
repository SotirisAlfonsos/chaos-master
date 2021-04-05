package network

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
	connectionPool map[string]*nConnection
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

func TestStartNetworkSuccess(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Successfully start network injection with specific job and target and add to an empty cache",
			jobMap: map[string]*config.Job{
				"job name":           newNetworkJob("network name", "127.0.0.1", "127.0.0.2"),
				"job different name": newNetworkJob("network name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*nConnection{
				"127.0.0.1": withSuccessNetworkConnection(),
				"127.0.0.2": withSuccessNetworkConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Device: "device name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 1, response: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
		{
			message: "Successfully start network injection with specific job and target and not add to the cache if already present",
			jobMap: map[string]*config.Job{
				"job name": newNetworkJob("network name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*nConnection{
				"127.0.0.1": withSuccessNetworkConnection(),
			},
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
			},
			requestPayload: &RequestPayload{Job: "job name", Device: "device name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 1, response: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
	}

	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "start")
	}
}

func TestRecoverNetworkSuccess(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Successfully recover network injection with specific job and target and dont change anything with the cache",
			jobMap: map[string]*config.Job{
				"job name":           newNetworkJob("network name", "127.0.0.1", "127.0.0.2"),
				"job different name": newNetworkJob("network name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*nConnection{
				"127.0.0.1": withSuccessNetworkConnection(),
				"127.0.0.2": withSuccessNetworkConnection(),
			},
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job name", Target: "127.0.0.2"}: functionWithSuccessResponse(),
			},
			requestPayload: &RequestPayload{Job: "job name", Device: "device name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 1, response: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
		{
			message: "Successfully recover network injection with specific job and target and remove it from the cache",
			jobMap: map[string]*config.Job{
				"job name": newNetworkJob("network name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*nConnection{
				"127.0.0.1": withSuccessNetworkConnection(),
			},
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
			},
			requestPayload: &RequestPayload{Job: "job name", Device: "device name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 0, response: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
	}

	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "recover")
	}
}

func TestNetworkActionOneOfJobContainerNameTargetDoesNotExist(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Should receive bad request and not update cache for action when job name does not exist",
			jobMap: map[string]*config.Job{
				"job name":           newNetworkJob("network name", "127.0.0.1", "127.0.0.2"),
				"job different name": newNetworkJob("network name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*nConnection{
				"127.0.0.1": withSuccessNetworkConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name does not exist", Device: "device name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 0, response: badRequestResponse("Could not find job {job name does not exist}")},
		},
		{
			message: "Should receive bad request and not update cache for action when target does not exist for the job specified",
			jobMap: map[string]*config.Job{
				"job name": newNetworkJob("network name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*nConnection{
				"127.0.0.1": withSuccessNetworkConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Device: "device name", Target: "0.0.0.0"},
			expected:       &expectedResult{cacheSize: 0, response: badRequestResponse("Target {0.0.0.0} is not registered for job {job name}")},
		},
	}

	t.Log("Action start")
	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "start")
	}

	t.Log("Action recover")
	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "recover")
	}
}

func TestNetworkActionFailure(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Should receive internal server error and not update cache if the request fails on bot",
			jobMap: map[string]*config.Job{
				"job name":           newNetworkJob("network name", "127.0.0.1", "127.0.0.2"),
				"job different name": newNetworkJob("network name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*nConnection{
				"127.0.0.1": withFailureNetworkConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Device: "device name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 0, response: internalServerErrorResponse("Failure response from target {127.0.0.1}")},
		},
		{
			message: "Should receive internal server error and not update cache if the request errors on bot",
			jobMap: map[string]*config.Job{
				"job name": newNetworkJob("network name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*nConnection{
				"127.0.0.1": withErrorNetworkConnection("error occurred"),
			},
			requestPayload: &RequestPayload{Job: "job name", Device: "device name", Target: "127.0.0.1"},
			expected: &expectedResult{
				cacheSize: 0, response: internalServerErrorResponse("Error response from target {127.0.0.1}: error occurred")},
		},
	}

	t.Log("Action start")
	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "start")
	}

	t.Log("Action recover")
	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "recover")
	}
}

func TestNetworkActionWithNoGrpcConnectionToTarget(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Should receive bad request and not update cache if the action to perform is invalid",
			jobMap: map[string]*config.Job{
				"job name": newNetworkJob("network name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*nConnection{
				"127.0.0.1": withFailureToSetNetworkConnection("Can not dial target"),
			},
			requestPayload: &RequestPayload{Job: "job name", Device: "device name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 0, response: internalServerErrorResponse("Can not get network connection from target {127.0.0.1}: Can not dial target")},
		},
	}

	t.Log("Action start")
	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "start")
	}
}

func TestNetworkWithInvalidAction(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Should receive bad request and not update cache if the action to perform is invalid",
			jobMap: map[string]*config.Job{
				"job name": newNetworkJob("network name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*nConnection{
				"127.0.0.1": withSuccessNetworkConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Device: "device name", Target: "127.0.0.1"},
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
		server, err := networkHTTPTestServerWithCacheItems(dataItem.jobMap, dataItem.connectionPool, c, dataItem.cacheItems)
		if err != nil {
			t.Fatal(err)
		}

		status, message, err := networkPostCall(server, dataItem.requestPayload, action)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, dataItem.expected.response.status, status)
		assert.Equal(t, dataItem.expected.response.message, message)
		assert.Equal(t, dataItem.expected.cacheSize, c.ItemCount())

		server.Close()
	})
}

func newNetworkJob(componentName string, targets ...string) *config.Job {
	return &config.Job{
		ComponentName: componentName,
		FailureType:   config.Network,
		Target:        targets,
	}
}

func withSuccessNetworkConnection() *nConnection {
	connection := &network.MockConnection{Status: new(v1.StatusResponse), Err: nil}
	return &nConnection{
		connection: connection,
	}
}

func withFailureNetworkConnection() *nConnection {
	statusResponse := new(v1.StatusResponse)
	statusResponse.Status = v1.StatusResponse_FAIL
	connection := &network.MockConnection{Status: statusResponse, Err: nil}
	return &nConnection{
		connection: connection,
	}
}

func withErrorNetworkConnection(errorMessage string) *nConnection {
	statusResponse := new(v1.StatusResponse)
	statusResponse.Status = v1.StatusResponse_FAIL
	connection := &network.MockConnection{Status: statusResponse, Err: errors.New(errorMessage)}
	return &nConnection{
		connection: connection,
	}
}

func withFailureToSetNetworkConnection(errorMessage string) *nConnection {
	connection := &network.MockFailedConnection{Err: errors.New(errorMessage)}
	return &nConnection{
		connection: connection,
	}
}

func networkHTTPTestServerWithCacheItems(
	jobMap map[string]*config.Job,
	connectionPool map[string]*nConnection,
	cache *gocache.Cache,
	cacheItems map[cache.Key]func() (*v1.StatusResponse, error),
) (*httptest.Server, error) {
	for key, val := range cacheItems {
		cache.Set(key, val)
	}

	nController := &NController{
		jobs:           jobMap,
		connectionPool: connectionPool,
		cache:          cache,
		loggers:        loggers,
	}

	router := mux.NewRouter()
	router.HandleFunc("/network", nController.NetworkAction).
		Queries("action", "{action}").
		Methods("POST")

	return httptest.NewServer(router), nil
}

func networkPostCall(server *httptest.Server, details *RequestPayload, action string) (int, string, error) {
	requestBody, _ := json.Marshal(details)
	url := server.URL + "/network?action=" + action

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
