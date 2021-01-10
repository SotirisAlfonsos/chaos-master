package v1

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/SotirisAlfonsos/chaos-master/chaoslogger"
	"github.com/go-kit/kit/log"

	"github.com/SotirisAlfonsos/chaos-master/cache"
	"github.com/gorilla/mux"

	"github.com/SotirisAlfonsos/chaos-bot/proto"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/SotirisAlfonsos/chaos-master/healthcheck"
	"github.com/SotirisAlfonsos/chaos-master/network"
	"github.com/stretchr/testify/assert"
)

var (
	logger = getLogger()
)

type TestData struct {
	message    string
	cacheItems map[string]func() (*proto.StatusResponse, error)
	alerts     []*Alert
	expected   *expectedResult
}

type expectedResult struct {
	cacheSize int
	payload   *RecoverResponsePayload
}

func TestRecoverAllRequestSuccess(t *testing.T) { //nolint:dupl
	dataItems := []TestData{
		{
			message: "Successfully recover all items from cache",
			cacheItems: map[string]func() (*proto.StatusResponse, error){
				"job,container,127.0.0.1":   functionWithSuccessResponse(),
				"job,container_2,127.0.0.2": functionWithSuccessResponse(),
			},
			alerts: []*Alert{{
				Status: "firing", Labels: Labels{RecoverAll: true},
			}},
			expected: &expectedResult{cacheSize: 0, payload: okResponse("SUCCESS", "SUCCESS")},
		},
		{
			message: "Successfully recover one item from the cache and record failure on the rest",
			cacheItems: map[string]func() (*proto.StatusResponse, error){
				"job_1,container,127.0.0.1": functionWithSuccessResponse(),
				"job,container_2,127.0.0.2": functionWithFailureResponse(),
				"job,container,127.0.0.2":   functionWithErrorResponse(),
			},
			alerts: []*Alert{{
				Status: "firing", Labels: Labels{RecoverAll: true},
			}},
			expected: &expectedResult{cacheSize: 2, payload: okResponse("SUCCESS", "FAILURE", "FAILURE")},
		},
		{
			message:    "Should not do anything and return ok for cache already empty",
			cacheItems: map[string]func() (*proto.StatusResponse, error){},
			alerts: []*Alert{{
				Status: "firing", Labels: Labels{RecoverAll: true},
			}},
			expected: &expectedResult{cacheSize: 0, payload: okResponse()},
		},
	}

	for _, dataItem := range dataItems {
		assertSuccessfulRecovery(t, dataItem)
	}
}

func TestRecoverJobRequestSuccess(t *testing.T) { //nolint:dupl
	dataItems := []TestData{
		{
			message: "Successfully recover all items from cache for job {job name}, while not removing other jobs",
			cacheItems: map[string]func() (*proto.StatusResponse, error){
				"job name,container,127.0.0.1":             functionWithSuccessResponse(),
				"job name,container_2,127.0.0.2":           functionWithSuccessResponse(),
				"job different name,container_2,127.0.0.2": functionWithSuccessResponse(),
			},
			alerts: []*Alert{{
				Status: "firing", Labels: Labels{RecoverJob: "job name"},
			}},
			expected: &expectedResult{cacheSize: 1, payload: okResponse("SUCCESS", "SUCCESS")},
		},
		{
			message: "Successfully recover one item from the cache and record failure on the rest for job {job name}, while not removing other jobs",
			cacheItems: map[string]func() (*proto.StatusResponse, error){
				"job name,container,127.0.0.1":           functionWithSuccessResponse(),
				"job name,container_2,127.0.0.2":         functionWithFailureResponse(),
				"job name,container,127.0.0.2":           functionWithErrorResponse(),
				"job different name,container,127.0.0.1": functionWithSuccessResponse(),
			},
			alerts: []*Alert{{
				Status: "firing", Labels: Labels{RecoverJob: "job name"},
			}},
			expected: &expectedResult{cacheSize: 3, payload: okResponse("SUCCESS", "FAILURE", "FAILURE")},
		},
		{
			message: "Should not do anything and return ok for job {job name} not in cache",
			cacheItems: map[string]func() (*proto.StatusResponse, error){
				"job different name,container,127.0.0.1": functionWithSuccessResponse(),
			},
			alerts: []*Alert{{
				Status: "firing", Labels: Labels{RecoverJob: "job name"},
			}},
			expected: &expectedResult{cacheSize: 1, payload: okResponse()},
		},
	}

	for _, dataItem := range dataItems {
		assertSuccessfulRecovery(t, dataItem)
	}
}

func TestRecoverTargetRequestSuccess(t *testing.T) { //nolint:dupl
	dataItems := []TestData{
		{
			message: "Successfully recover all items from cache for target {127.0.0.1}, while not removing other jobs",
			cacheItems: map[string]func() (*proto.StatusResponse, error){
				"job name,container,127.0.0.1":             functionWithSuccessResponse(),
				"job different name,container_2,127.0.0.1": functionWithSuccessResponse(),
				"job different name,container_2,127.0.0.2": functionWithSuccessResponse(),
			},
			alerts: []*Alert{{
				Status: "firing", Labels: Labels{RecoverTarget: "127.0.0.1"},
			}},
			expected: &expectedResult{cacheSize: 1, payload: okResponse("SUCCESS", "SUCCESS")},
		},
		{
			message: "Successfully recover one item from the cache and record failure on the rest for target {127.0.0.1}, while not removing other jobs",
			cacheItems: map[string]func() (*proto.StatusResponse, error){
				"job name,container,127.0.0.1":           functionWithSuccessResponse(),
				"job other name,container_2,127.0.0.1":   functionWithFailureResponse(),
				"job different name,container,127.0.0.1": functionWithErrorResponse(),
				"job different name,container,127.0.0.2": functionWithSuccessResponse(),
			},
			alerts: []*Alert{{
				Status: "firing", Labels: Labels{RecoverTarget: "127.0.0.1"},
			}},
			expected: &expectedResult{cacheSize: 3, payload: okResponse("SUCCESS", "FAILURE", "FAILURE")},
		},
		{
			message: "Should not do anything and return ok for target {127.0.0.1} not in cache",
			cacheItems: map[string]func() (*proto.StatusResponse, error){
				"job different name,container,127.0.0.2": functionWithSuccessResponse(),
			},
			alerts: []*Alert{{
				Status: "firing", Labels: Labels{RecoverTarget: "127.0.0.1"},
			}},
			expected: &expectedResult{cacheSize: 1, payload: okResponse()},
		},
	}

	for _, dataItem := range dataItems {
		assertSuccessfulRecovery(t, dataItem)
	}
}

func TestRecoverRequestForNotFiringAlerts(t *testing.T) { //nolint:dupl
	dataItems := []TestData{
		{
			message: "Successfully recover items on firing and ignore resolved alerts, resulting in two jobs recovered",
			cacheItems: map[string]func() (*proto.StatusResponse, error){
				"job name,container,127.0.0.1":             functionWithSuccessResponse(),
				"job different name,container_2,127.0.0.1": functionWithSuccessResponse(),
				"job different name,container_2,127.0.0.2": functionWithSuccessResponse(),
			},
			alerts: []*Alert{
				{Status: "firing", Labels: Labels{RecoverTarget: "127.0.0.1"}},
				{Status: "resolved", Labels: Labels{RecoverTarget: "127.0.0.1"}},
				{Status: "resolved", Labels: Labels{RecoverAll: true}},
				{Status: "resolved", Labels: Labels{RecoverJob: "job name"}},
			},
			expected: &expectedResult{cacheSize: 1, payload: okResponse("SUCCESS", "SUCCESS")},
		},
		{
			message: "Should not do anything for resolved alerts, no items should be removed from cache",
			cacheItems: map[string]func() (*proto.StatusResponse, error){
				"job name,container,127.0.0.1":         functionWithSuccessResponse(),
				"job other name,container_2,127.0.0.1": functionWithSuccessResponse(),
			},
			alerts: []*Alert{
				{Status: "resolved", Labels: Labels{RecoverTarget: "127.0.0.1"}},
				{Status: "resolved", Labels: Labels{RecoverAll: true}},
				{Status: "resolved", Labels: Labels{RecoverJob: "job name"}},
			},
			expected: &expectedResult{cacheSize: 2, payload: okResponse()},
		},
		{
			message: "Should not do anything for unknown alert status, no items should be removed from cache",
			cacheItems: map[string]func() (*proto.StatusResponse, error){
				"job different name,container,127.0.0.1": functionWithSuccessResponse(),
			},
			alerts: []*Alert{{
				Status: "unknown", Labels: Labels{RecoverAll: true},
			}},
			expected: &expectedResult{cacheSize: 1, payload: okResponse()},
		},
	}

	for _, dataItem := range dataItems {
		assertSuccessfulRecovery(t, dataItem)
	}
}

func assertSuccessfulRecovery(t *testing.T, dataItem TestData) {
	t.Logf(dataItem.message)

	apiRouter, err := defaultAPIRouterWithCacheItems(make(map[string]*config.Job), &network.Connections{}, dataItem.cacheItems)
	if err != nil {
		t.Fatalf("Could not prepare cache for test. %s", err)
	}

	server := recoverHTTPTestServer(apiRouter)
	requestPayload := newRequestPayload(dataItem.alerts)

	responsePayload, err := restorePostCall(server, requestPayload)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, dataItem.expected.payload.Status, responsePayload.Status)
	assert.Equal(t, len(dataItem.expected.payload.RecoverMessage), len(responsePayload.RecoverMessage))
	assert.Equal(t, dataItem.expected.payload.getSortedStatuses(), responsePayload.getSortedStatuses())
	assert.Equal(t, dataItem.expected.cacheSize, apiRouter.cache.ItemCount())
}

func (recoverR *RecoverResponsePayload) getSortedStatuses() []string {
	statuses := make([]string, 0)
	for _, recoverMessage := range recoverR.RecoverMessage {
		statuses = append(statuses, recoverMessage.Status)
	}
	sort.Strings(statuses)
	return statuses
}

func defaultAPIRouterWithCacheItems(jobMap map[string]*config.Job, connections *network.Connections,
	cacheItems map[string]func() (*proto.StatusResponse, error)) (*APIRouter, error) {
	apiRouter := NewAPIRouter(jobMap, connections, cache.NewCacheManager(logger), logger)

	for key, val := range cacheItems {
		err := apiRouter.cache.Register(key, val)
		if err != nil {
			return nil, err
		}
	}

	return apiRouter, nil
}

func okResponse(statuses ...string) *RecoverResponsePayload {
	recoverMessages := make([]*RecoverMessage, 0)
	for _, status := range statuses {
		recoverMessages = append(recoverMessages, &RecoverMessage{Status: status})
	}
	return &RecoverResponsePayload{
		RecoverMessage: recoverMessages,
		Status:         200,
	}
}

func functionWithSuccessResponse() func() (*proto.StatusResponse, error) {
	return func() (*proto.StatusResponse, error) {
		return &proto.StatusResponse{Status: proto.StatusResponse_SUCCESS}, nil
	}
}

func functionWithFailureResponse() func() (*proto.StatusResponse, error) {
	return func() (*proto.StatusResponse, error) {
		return &proto.StatusResponse{Status: proto.StatusResponse_FAIL}, nil
	}
}

func functionWithErrorResponse() func() (*proto.StatusResponse, error) {
	return func() (*proto.StatusResponse, error) {
		return &proto.StatusResponse{Status: proto.StatusResponse_FAIL}, errors.New("error")
	}
}

func recoverHTTPTestServer(apiRouter *APIRouter) *httptest.Server {
	router := mux.NewRouter()
	router = apiRouter.AddRoutes(&healthcheck.HealthChecker{}, router)
	router.Schemes("http")
	return httptest.NewServer(router)
}

func newRequestPayload(alerts []*Alert) *requestPayload {
	return &requestPayload{
		Alerts: alerts,
	}
}

func restorePostCall(server *httptest.Server, requestPayload *requestPayload) (*RecoverResponsePayload, error) {
	requestBody, _ := json.Marshal(requestPayload)
	url := server.URL + "/chaos/api/v1/recover/alertmanager"

	return post(requestBody, url)
}

func post(requestBody []byte, url string) (*RecoverResponsePayload, error) {
	resp, err := http.Post(url, "", bytes.NewReader(requestBody)) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	response := &RecoverResponsePayload{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return &RecoverResponsePayload{Status: resp.StatusCode}, err
	}
	return response, nil
}

func getLogger() log.Logger {
	allowLevel := &chaoslogger.AllowedLevel{}
	if err := allowLevel.Set("debug"); err != nil {
		fmt.Printf("%v", err)
	}

	return chaoslogger.New(allowLevel)
}
