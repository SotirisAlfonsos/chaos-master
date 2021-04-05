package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/SotirisAlfonsos/chaos-master/pkg/chaoslogger"
	"github.com/SotirisAlfonsos/chaos-master/pkg/network"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/response"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

var (
	loggers = getLoggers()
)

type testData struct {
	message        string
	jobMap         map[string]*config.Job
	connectionPool map[string]*sConnection
	requestPayload *RequestPayload
	expected       *expectedResult
}

type expectedResult struct {
	response *responseWrapper
}

type responseWrapper struct {
	status  int
	message string
	err     error
}

func TestSuccessServerKill(t *testing.T) {
	td := []testData{
		{
			message: "Successfully kill server for job and target",
			jobMap: map[string]*config.Job{
				"job name": newServerJob("127.0.0.1", "127.0.0.2"),
			},
			connectionPool: map[string]*sConnection{
				"127.0.0.1": withSuccessServerConnection(),
			},
			requestPayload: &RequestPayload{Job: "job name", Target: "127.0.0.1"},
			expected:       &expectedResult{response: okResponse("Response from target {127.0.0.1}, {}, {SUCCESS}")},
		},
	}

	for _, test := range td {
		assertActionPerformed(t, test, "kill")
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
			expected:       &expectedResult{response: badRequestResponse("Could not find job {job name not existing}")},
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
			expected:       &expectedResult{response: badRequestResponse("Target {} is not registered for job {job name}")},
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
			expected:       &expectedResult{response: badRequestResponse("Could not find job {}")},
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
			expected:       &expectedResult{response: badRequestResponse("Target {127.0.0.3} is not registered for job {job name}")},
		},
	}

	for _, test := range td {
		assertActionPerformed(t, test, "kill")
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
			expected:       &expectedResult{response: badRequestResponse("The action {start} is not supported")},
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
			expected:       &expectedResult{response: internalServerErrorResponse("Failure response from target {127.0.0.1}")},
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
			expected:       &expectedResult{response: internalServerErrorResponse("Error response from target {127.0.0.1}: can not connect")},
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
			expected:       &expectedResult{response: internalServerErrorResponse("Can not get server connection from target {127.0.0.1}: can not set connection")},
		},
	}

	for _, test := range td {
		assertActionPerformed(t, test, "kill")
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

		status, message, err := serverPostCall(server, dataItem.requestPayload, action)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, dataItem.expected.response.status, status)
		assert.Equal(t, dataItem.expected.response.message, message)

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
		loggers:        loggers,
	}

	router := mux.NewRouter()
	router.HandleFunc("/server", sController.ServerAction).
		Queries("action", "{action}").
		Methods("POST")

	return httptest.NewServer(router), nil
}

func serverPostCall(server *httptest.Server, details *RequestPayload, action string) (int, string, error) {
	requestBody, _ := json.Marshal(details)
	url := server.URL + "/server?action=" + action

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
