package cpu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/cache"
	"github.com/SotirisAlfonsos/chaos-master/chaoslogger"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/response"
	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

var (
	logger = getLogger()
)

type TestData struct {
	message        string
	jobMap         map[string]*config.Job
	connectionPool map[string]*cConnection
	cacheItems     map[*cache.Key]func() (*v1.StatusResponse, error)
	requestPayload *RequestPayload
	expected       *expectedResult
}

type expectedResult struct {
	cacheSize int
	payload   *response.Payload
}

type mockCPUClient struct {
	Status *v1.StatusResponse
	Error  error
}

func GetMockCPUClient(status *v1.StatusResponse, err error) v1.CPUClient {
	return &mockCPUClient{Status: status, Error: err}
}

func (mcc *mockCPUClient) Start(ctx context.Context, in *v1.CPURequest, opts ...grpc.CallOption) (*v1.StatusResponse, error) {
	return mcc.Status, mcc.Error
}

func (mcc *mockCPUClient) Stop(ctx context.Context, in *v1.CPURequest, opts ...grpc.CallOption) (*v1.StatusResponse, error) {
	return mcc.Status, mcc.Error
}

type connection struct {
	status *v1.StatusResponse
	err    error
}

func (connection *connection) GetServiceClient(target string) (v1.ServiceClient, error) {
	return nil, nil
}

func (connection *connection) GetDockerClient(target string) (v1.DockerClient, error) {
	return nil, nil
}

func (connection *connection) GetCPUClient(target string) (v1.CPUClient, error) {
	return GetMockCPUClient(connection.status, connection.err), nil
}

func (connection *connection) GetHealthClient(target string) (v1.HealthClient, error) {
	return nil, nil
}

type failedConnection struct {
	err error
}

func (failedConnection *failedConnection) GetServiceClient(target string) (v1.ServiceClient, error) {
	return nil, failedConnection.err
}

func (failedConnection *failedConnection) GetDockerClient(target string) (v1.DockerClient, error) {
	return nil, nil
}

func (failedConnection *failedConnection) GetCPUClient(target string) (v1.CPUClient, error) {
	return nil, failedConnection.err
}

func (failedConnection *failedConnection) GetHealthClient(target string) (v1.HealthClient, error) {
	return nil, nil
}

func TestStartCPUSuccess(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Successfully start cpu injection with specific job and target and add it in cache",
			jobMap: map[string]*config.Job{
				"job name":           newCPUJob("127.0.0.1", "127.0.0.2"),
				"job different name": newCPUJob("127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*cConnection{
				"127.0.0.1": withSuccessCPUConnection(),
				"127.0.0.2": withSuccessCPUConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Percentage: 100, Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 1, payload: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
		{
			message: "Successfully start cpu injection with specific job and target and dont add it in cache if already exists",
			jobMap: map[string]*config.Job{
				"job name": newCPUJob("127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*cConnection{
				"127.0.0.1": withSuccessCPUConnection(),
			},
			cacheItems: map[*cache.Key]func() (*v1.StatusResponse, error){
				&cache.Key{Job: "job name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
			},
			requestPayload: &RequestPayload{Job: "job name", Percentage: 100, Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 1, payload: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
	}

	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "start")
	}
}

func TestStopServiceSuccess(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Successfully stop CPU injection with specific job and target and dont add it in cache",
			jobMap: map[string]*config.Job{
				"job name":           newCPUJob("127.0.0.1", "127.0.0.2"),
				"job different name": newCPUJob("127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*cConnection{
				"127.0.0.1": withSuccessCPUConnection(),
				"127.0.0.2": withSuccessCPUConnection(),
			},
			cacheItems: map[*cache.Key]func() (*v1.StatusResponse, error){
				&cache.Key{Job: "job name", Target: "127.0.0.2"}: functionWithSuccessResponse(),
			},
			requestPayload: &RequestPayload{Job: "job name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 1, payload: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
		{
			message: "Successfully stop CPU injection with specific job and target and remove it from cache",
			jobMap: map[string]*config.Job{
				"job name": newCPUJob("service name", "127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*cConnection{
				"127.0.0.1": withSuccessCPUConnection(),
			},
			cacheItems: map[*cache.Key]func() (*v1.StatusResponse, error){
				&cache.Key{Job: "job name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
			},
			requestPayload: &RequestPayload{Job: "job name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 0, payload: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
	}

	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "stop")
	}
}

func TestCPUActionOneOfJobTargetDoesNotExist(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Should receive bad request and not update cache for action when job name does not exist",
			jobMap: map[string]*config.Job{
				"job name":           newCPUJob("127.0.0.1", "127.0.0.2"),
				"job different name": newCPUJob("127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*cConnection{
				"127.0.0.1": withSuccessCPUConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name does not exist", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 0, payload: badRequestResponse("Could not find job {job name does not exist}")},
		},
		{
			message: "Should receive bad request and not update cache for action when target name does not exist for job",
			jobMap: map[string]*config.Job{
				"job name": newCPUJob("127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*cConnection{
				"127.0.0.1": withSuccessCPUConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Target: "127.0.0.3"},
			expected: &expectedResult{
				cacheSize: 0,
				payload:   badRequestResponse("Target {127.0.0.3} is not registered for job {job name}"),
			},
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

func TestCPUActionFailure(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Should receive internal server error and not update cache if the request fails on bot",
			jobMap: map[string]*config.Job{
				"job name":           newCPUJob("127.0.0.1", "127.0.0.2"),
				"job different name": newCPUJob("127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*cConnection{
				"127.0.0.1": withFailureCPUConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 0, payload: internalServerErrorResponse("Failure response from target {127.0.0.1}")},
		},
		{
			message: "Should receive internal server error and not update cache if the request errors on bot",
			jobMap: map[string]*config.Job{
				"job name": newCPUJob("127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*cConnection{
				"127.0.0.1": withErrorCPUConnection("error occurred"),
			},
			requestPayload: &RequestPayload{Job: "job name", Target: "127.0.0.1"},
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

func TestCPUActionWithNoGrpcConnectionToTarget(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Should receive bad request and not update cache if the action to perform is invalid",
			jobMap: map[string]*config.Job{
				"job name": newCPUJob("127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*cConnection{
				"127.0.0.1": withFailureToSetCPUConnection("Can not dial target"),
			},
			requestPayload: &RequestPayload{Job: "job name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 0, payload: internalServerErrorResponse("Can not get cpu connection for target {127.0.0.1}: Can not dial target")},
		},
	}

	t.Log("Action start")
	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "start")
	}
}

func TestCPUWithInvalidAction(t *testing.T) {
	dataItems := []TestData{
		{
			message: "Should receive bad request and not update cache if the action to perform is invalid",
			jobMap: map[string]*config.Job{
				"job name": newCPUJob("127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*cConnection{
				"127.0.0.1": withSuccessCPUConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Target: "127.0.0.1"},
			expected:       &expectedResult{cacheSize: 0, payload: badRequestResponse("The action {invalidAction} is not supported")},
		},
	}

	t.Log("Action invalidAction")
	for _, dataItem := range dataItems {
		assertActionPerformed(t, dataItem, "invalidAction")
	}
}

func assertActionPerformed(t *testing.T, dataItem TestData, action string) {
	t.Run(dataItem.message, func(t *testing.T) {
		cacheManager := cache.NewCacheManager(logger)
		server, err := cpuHTTPTestServerWithCacheItems(dataItem.jobMap, dataItem.connectionPool, cacheManager, dataItem.cacheItems)
		if err != nil {
			t.Fatal(err)
		}

		resp, err := cpuPostCall(server, dataItem.requestPayload, action)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, dataItem.expected.payload.Status, resp.Status)
		assert.Equal(t, dataItem.expected.payload.Message, resp.Message)
		assert.Equal(t, dataItem.expected.payload.Error, resp.Error)
		assert.Equal(t, dataItem.expected.cacheSize, cacheManager.ItemCount())

		server.Close()
	})
}

func newCPUJob(targets ...string) *config.Job {
	return &config.Job{
		FailureType: config.Service,
		Target:      targets,
	}
}

func withSuccessCPUConnection() *cConnection {
	connection := &connection{status: new(v1.StatusResponse), err: nil}
	return &cConnection{
		connection: connection,
	}
}

func withFailureCPUConnection() *cConnection {
	statusResponse := new(v1.StatusResponse)
	statusResponse.Status = v1.StatusResponse_FAIL
	connection := &connection{status: statusResponse, err: nil}
	return &cConnection{
		connection: connection,
	}
}

func withErrorCPUConnection(errorMessage string) *cConnection {
	statusResponse := new(v1.StatusResponse)
	statusResponse.Status = v1.StatusResponse_FAIL
	connection := &connection{status: statusResponse, err: errors.New(errorMessage)}
	return &cConnection{
		connection: connection,
	}
}

func withFailureToSetCPUConnection(errorMessage string) *cConnection {
	connection := &failedConnection{err: errors.New(errorMessage)}
	return &cConnection{
		connection: connection,
	}
}

func cpuHTTPTestServerWithCacheItems(
	jobMap map[string]*config.Job,
	connectionPool map[string]*cConnection,
	cacheManager *cache.Manager,
	cacheItems map[*cache.Key]func() (*v1.StatusResponse, error),
) (*httptest.Server, error) {
	for key, val := range cacheItems {
		err := cacheManager.Register(key, val)
		if err != nil {
			return nil, err
		}
	}

	cController := &CController{
		jobs:           jobMap,
		connectionPool: connectionPool,
		cacheManager:   cacheManager,
		logger:         logger,
	}

	router := mux.NewRouter()
	router.HandleFunc("/cpu", cController.CPUAction).
		Queries("action", "{action}").
		Methods("POST")

	return httptest.NewServer(router), nil
}

func cpuPostCall(server *httptest.Server, details *RequestPayload, action string) (*response.Payload, error) {
	requestBody, _ := json.Marshal(details)
	url := server.URL + "/cpu?action=" + action

	return post(requestBody, url)
}

func post(requestBody []byte, url string) (*response.Payload, error) {
	resp, err := http.Post(url, "", bytes.NewReader(requestBody)) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respPayload := &response.Payload{}
	err = json.NewDecoder(resp.Body).Decode(&respPayload)
	if err != nil {
		return &response.Payload{Status: resp.StatusCode}, err
	}
	return respPayload, nil
}

func okResponse(message string) *response.Payload {
	return &response.Payload{
		Message: message,
		Status:  200,
	}
}

func badRequestResponse(error string) *response.Payload {
	return &response.Payload{
		Error:  error,
		Status: 400,
	}
}

func internalServerErrorResponse(error string) *response.Payload {
	return &response.Payload{
		Error:  error,
		Status: 500,
	}
}

func functionWithSuccessResponse() func() (*v1.StatusResponse, error) {
	return func() (*v1.StatusResponse, error) {
		return &v1.StatusResponse{Status: v1.StatusResponse_SUCCESS}, nil
	}
}

func getLogger() log.Logger {
	allowLevel := &chaoslogger.AllowedLevel{}
	if err := allowLevel.Set("debug"); err != nil {
		fmt.Printf("%v", err)
	}

	return chaoslogger.New(allowLevel)
}
