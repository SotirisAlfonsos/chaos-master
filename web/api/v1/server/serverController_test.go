package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/SotirisAlfonsos/chaos-master/network"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/chaoslogger"
	"github.com/go-kit/kit/log"

	"github.com/gorilla/mux"

	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/response"
)

var (
	logger = getLogger()
)

type testData struct {
	message        string
	jobMap         map[string]*config.Job
	connectionPool map[string]*sConnection
	requestPayload *RequestPayload
	expected       *expectedResult
}

type expectedResult struct {
	payload *response.Payload
}

func TestSuccessServerStop(t *testing.T) {
	td := []testData{
		{
			message: "Successfully stop server for job and target",
			jobMap: map[string]*config.Job{
				"job name": newServerJob("127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*sConnection{
				"127.0.0.1": withSuccessServerConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Target: "127.0.0.1"},
			expected:       &expectedResult{payload: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
	}

	for _, test := range td {
		assertActionPerformed(t, test, "stop")
	}
}

func TestServerInvalidInputTargetAndJob(t *testing.T) {
	td := []testData{
		{
			message: "Bad request for job not defined",
			jobMap: map[string]*config.Job{
				"job name": newServerJob("127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*sConnection{
				"127.0.0.1": withSuccessServerConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name not existing", Target: "127.0.0.1"},
			expected:       &expectedResult{payload: badRequestResponse("Could not find job {job name not existing}")},
		},
		{
			message: "Bad request for target not provided in request",
			jobMap: map[string]*config.Job{
				"job name": newServerJob("127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*sConnection{
				"127.0.0.1": withSuccessServerConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name"},
			expected:       &expectedResult{payload: badRequestResponse("Target {} is not registered for job {job name}")},
		},
		{
			message: "Bad request for job not provided in request",
			jobMap: map[string]*config.Job{
				"job name": newServerJob("127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*sConnection{
				"127.0.0.1": withSuccessServerConnection(),
			},
			requestPayload: &RequestPayload{Target: "127.0.0.1"},
			expected:       &expectedResult{payload: badRequestResponse("Could not find job {}")},
		},
		{
			message: "Bad request for target not in job",
			jobMap: map[string]*config.Job{
				"job name":     newServerJob("127.0.0.1", "127.0.0.2"),
				"job name new": newServerJob("127.0.0.3"),
			},
			connectionPool: map[string]*sConnection{
				"127.0.0.3": withSuccessServerConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Target: "127.0.0.3"},
			expected:       &expectedResult{payload: badRequestResponse("Target {127.0.0.3} is not registered for job {job name}")},
		},
	}

	for _, test := range td {
		assertActionPerformed(t, test, "stop")
	}
}

func TestServerInvalidAction(t *testing.T) {
	td := []testData{
		{
			message: "Bad request action start is not supported for server",
			jobMap: map[string]*config.Job{
				"job name": newServerJob("127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*sConnection{
				"127.0.0.1": withSuccessServerConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Target: "127.0.0.1"},
			expected:       &expectedResult{payload: badRequestResponse("The action {start} is not supported")},
		},
	}

	for _, test := range td {
		assertActionPerformed(t, test, "start")
	}
}

func TestServerInvalidResponseFromBot(t *testing.T) {
	td := []testData{
		{
			message: "Internal server error for failure server response from bot",
			jobMap: map[string]*config.Job{
				"job name": newServerJob("127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*sConnection{
				"127.0.0.1": withFailureServerConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Target: "127.0.0.1"},
			expected:       &expectedResult{payload: internalServerErrorResponse("Failure response from target {127.0.0.1}")},
		},
		{
			message: "Internal server error for failure to connect to bot",
			jobMap: map[string]*config.Job{
				"job name": newServerJob("127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*sConnection{
				"127.0.0.1": withErrorServerConnection("can not connect"),
			},
			requestPayload: &RequestPayload{Job: "job name", Target: "127.0.0.1"},
			expected:       &expectedResult{payload: internalServerErrorResponse("Error response from target {127.0.0.1}: can not connect")},
		},
		{
			message: "Internal server error for failure to establish connection with bot",
			jobMap: map[string]*config.Job{
				"job name": newServerJob("127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*sConnection{
				"127.0.0.1": withFailureToSetServerConnection("can not set connection"),
			},
			requestPayload: &RequestPayload{Job: "job name", Target: "127.0.0.1"},
			expected:       &expectedResult{payload: internalServerErrorResponse("Can not get server connection from target {127.0.0.1}: can not set connection")},
		},
	}

	for _, test := range td {
		assertActionPerformed(t, test, "stop")
	}
}

func newServerJob(targets ...string) *config.Job {
	return &config.Job{
		FailureType: config.Server,
		Target:      targets,
	}
}

func assertActionPerformed(t *testing.T, dataItem testData, action string) {
	t.Run(dataItem.message, func(t *testing.T) {
		server, err := serverHTTPTestServer(dataItem.jobMap, dataItem.connectionPool)
		if err != nil {
			t.Fatal(err)
		}

		resp, err := serverPostCall(server, dataItem.requestPayload, action)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, dataItem.expected.payload.Status, resp.Status)
		assert.Equal(t, dataItem.expected.payload.Message, resp.Message)
		assert.Equal(t, dataItem.expected.payload.Error, resp.Error)

		server.Close()
	})
}

func serverHTTPTestServer(
	jobMap map[string]*config.Job,
	connectionPool map[string]*sConnection,
) (*httptest.Server, error) {
	sController := &SController{
		jobs:           jobMap,
		connectionPool: connectionPool,
		logger:         logger,
	}

	router := mux.NewRouter()
	router.HandleFunc("/server", sController.ServerAction).
		Queries("action", "{action}").
		Methods("POST")

	return httptest.NewServer(router), nil
}

func serverPostCall(server *httptest.Server, details *RequestPayload, action string) (*response.Payload, error) {
	requestBody, _ := json.Marshal(details)
	url := server.URL + "/server?action=" + action

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

func withSuccessServerConnection() *sConnection {
	connection := &network.MockConnection{Status: new(v1.StatusResponse), Err: nil}
	return &sConnection{
		connection: connection,
	}
}

func withFailureServerConnection() *sConnection {
	statusResponse := new(v1.StatusResponse)
	statusResponse.Status = v1.StatusResponse_FAIL
	connection := &network.MockConnection{Status: statusResponse, Err: nil}
	return &sConnection{
		connection: connection,
	}
}

func withErrorServerConnection(errorMessage string) *sConnection {
	statusResponse := new(v1.StatusResponse)
	statusResponse.Status = v1.StatusResponse_FAIL
	connection := &network.MockConnection{Status: statusResponse, Err: errors.New(errorMessage)}
	return &sConnection{
		connection: connection,
	}
}

func withFailureToSetServerConnection(errorMessage string) *sConnection {
	connection := &network.MockFailedConnection{Err: errors.New(errorMessage)}
	return &sConnection{
		connection: connection,
	}
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

func getLogger() log.Logger {
	allowLevel := &chaoslogger.AllowedLevel{}
	if err := allowLevel.Set("debug"); err != nil {
		fmt.Printf("%v", err)
	}

	return chaoslogger.New(allowLevel)
}
