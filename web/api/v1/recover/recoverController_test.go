package recover

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/cache"
	"github.com/SotirisAlfonsos/chaos-master/chaoslogger"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/response"
	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

var (
	logger = getLogger()
)

type TestData struct {
	message    string
	cacheItems map[*cache.Key]func() (*v1.StatusResponse, error)
	alerts     []*Alert
	expected   *expectedResult
}

type expectedResult struct {
	cacheSize int
	payload   *response.RecoverResponsePayload
}

func TestRecoverAllRequestSuccess(t *testing.T) { //nolint:dupl
	dataItems := []TestData{
		{
			message: "Successfully recover all items from cache",
			cacheItems: map[*cache.Key]func() (*v1.StatusResponse, error){
				&cache.Key{Job: "job", Target: "127.0.0.1"}: functionWithSuccessResponse(),
				&cache.Key{Job: "job", Target: "127.0.0.2"}: functionWithSuccessResponse(),
			},
			alerts: []*Alert{{
				Status: "firing", Labels: Labels{RecoverAll: true},
			}},
			expected: &expectedResult{cacheSize: 0, payload: okResponse("SUCCESS", "SUCCESS")},
		},
		{
			message: "Successfully recover one item from the cache and record failure on the rest",
			cacheItems: map[*cache.Key]func() (*v1.StatusResponse, error){
				&cache.Key{Job: "job_1", Target: "127.0.0.1"}: functionWithSuccessResponse(),
				&cache.Key{Job: "job", Target: "127.0.0.2"}:   functionWithFailureResponse(),
				&cache.Key{Job: "job", Target: "127.0.0.3"}:   functionWithErrorResponse(),
			},
			alerts: []*Alert{{
				Status: "firing", Labels: Labels{RecoverAll: true},
			}},
			expected: &expectedResult{cacheSize: 2, payload: okResponse("SUCCESS", "FAILURE", "FAILURE")},
		},
		{
			message:    "Should not do anything and return ok for cache already empty",
			cacheItems: map[*cache.Key]func() (*v1.StatusResponse, error){},
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
			cacheItems: map[*cache.Key]func() (*v1.StatusResponse, error){
				&cache.Key{Job: "job name", Target: "127.0.0.1"}:           functionWithSuccessResponse(),
				&cache.Key{Job: "job name", Target: "127.0.0.2"}:           functionWithSuccessResponse(),
				&cache.Key{Job: "job different name", Target: "127.0.0.2"}: functionWithSuccessResponse(),
			},
			alerts: []*Alert{{
				Status: "firing", Labels: Labels{RecoverJob: "job name"},
			}},
			expected: &expectedResult{cacheSize: 1, payload: okResponse("SUCCESS", "SUCCESS")},
		},
		{
			message: "Successfully recover one item from the cache and record failure on the rest for job {job name}, while not removing other jobs",
			cacheItems: map[*cache.Key]func() (*v1.StatusResponse, error){
				&cache.Key{Job: "job name", Target: "127.0.0.1"}:           functionWithSuccessResponse(),
				&cache.Key{Job: "job name", Target: "127.0.0.2"}:           functionWithFailureResponse(),
				&cache.Key{Job: "job name", Target: "127.0.0.3"}:           functionWithErrorResponse(),
				&cache.Key{Job: "job different name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
			},
			alerts: []*Alert{{
				Status: "firing", Labels: Labels{RecoverJob: "job name"},
			}},
			expected: &expectedResult{cacheSize: 3, payload: okResponse("SUCCESS", "FAILURE", "FAILURE")},
		},
		{
			message: "Should not do anything and return ok for job {job name} not in cache",
			cacheItems: map[*cache.Key]func() (*v1.StatusResponse, error){
				&cache.Key{Job: "job different name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
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
			cacheItems: map[*cache.Key]func() (*v1.StatusResponse, error){
				&cache.Key{Job: "job name", Target: "127.0.0.1"}:           functionWithSuccessResponse(),
				&cache.Key{Job: "job different name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
				&cache.Key{Job: "job different name", Target: "127.0.0.2"}: functionWithSuccessResponse(),
			},
			alerts: []*Alert{{
				Status: "firing", Labels: Labels{RecoverTarget: "127.0.0.1"},
			}},
			expected: &expectedResult{cacheSize: 1, payload: okResponse("SUCCESS", "SUCCESS")},
		},
		{
			message: "Successfully recover one item from the cache and record failure on the rest for target {127.0.0.1}, while not removing other jobs",
			cacheItems: map[*cache.Key]func() (*v1.StatusResponse, error){
				&cache.Key{Job: "job name", Target: "127.0.0.1"}:           functionWithSuccessResponse(),
				&cache.Key{Job: "job other name", Target: "127.0.0.1"}:     functionWithFailureResponse(),
				&cache.Key{Job: "job different name", Target: "127.0.0.1"}: functionWithErrorResponse(),
				&cache.Key{Job: "job different name", Target: "127.0.0.2"}: functionWithSuccessResponse(),
			},
			alerts: []*Alert{{
				Status: "firing", Labels: Labels{RecoverTarget: "127.0.0.1"},
			}},
			expected: &expectedResult{cacheSize: 3, payload: okResponse("SUCCESS", "FAILURE", "FAILURE")},
		},
		{
			message: "Should not do anything and return ok for target {127.0.0.1} not in cache",
			cacheItems: map[*cache.Key]func() (*v1.StatusResponse, error){
				&cache.Key{Job: "job different name", Target: "127.0.0.2"}: functionWithSuccessResponse(),
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
			cacheItems: map[*cache.Key]func() (*v1.StatusResponse, error){
				&cache.Key{Job: "job name", Target: "127.0.0.1"}:           functionWithSuccessResponse(),
				&cache.Key{Job: "job different name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
				&cache.Key{Job: "job different name", Target: "127.0.0.2"}: functionWithSuccessResponse(),
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
			cacheItems: map[*cache.Key]func() (*v1.StatusResponse, error){
				&cache.Key{Job: "job name", Target: "127.0.0.1"}:       functionWithSuccessResponse(),
				&cache.Key{Job: "job other name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
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
			cacheItems: map[*cache.Key]func() (*v1.StatusResponse, error){
				&cache.Key{Job: "job different name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
			},
			alerts: []*Alert{{
				Status: "unknown", Labels: Labels{RecoverAll: true},
			}},
			expected: &expectedResult{cacheSize: 1, payload: badRequestResponse("The status {unknown} is not supported")},
		},
	}

	for _, dataItem := range dataItems {
		assertSuccessfulRecovery(t, dataItem)
	}
}

func assertSuccessfulRecovery(t *testing.T, dataItem TestData) {
	t.Logf(dataItem.message)

	cacheManager := cache.NewCacheManager(logger)
	server, err := recoverHTTPTestServerWithCacheItems(cacheManager, dataItem.cacheItems)
	if err != nil {
		t.Fatal(err)
	}
	requestPayload := newRequestPayload(dataItem.alerts)

	responsePayload, err := restorePostCall(server, requestPayload)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, dataItem.expected.payload.Status, responsePayload.Status)
	assert.Equal(t, len(dataItem.expected.payload.RecoverMessage), len(responsePayload.RecoverMessage))
	assert.Equal(t, dataItem.expected.payload.GetSortedStatuses(), responsePayload.GetSortedStatuses())
	assert.Equal(t, dataItem.expected.cacheSize, cacheManager.ItemCount())
}

func okResponse(statuses ...string) *response.RecoverResponsePayload {
	recoverMessages := make([]*response.RecoverMessage, 0, len(statuses))
	for _, status := range statuses {
		recoverMessages = append(recoverMessages, &response.RecoverMessage{Status: status})
	}
	return &response.RecoverResponsePayload{
		RecoverMessage: recoverMessages,
		Status:         200,
	}
}

func badRequestResponse(message string) *response.RecoverResponsePayload {
	recoverMessage := &response.RecoverMessage{
		Message: message,
		Status:  "FAILURE",
	}
	recoverMessages := make([]*response.RecoverMessage, 0, 1)
	recoverMessages = append(recoverMessages, recoverMessage)
	return &response.RecoverResponsePayload{
		RecoverMessage: recoverMessages,
		Status:         400,
	}
}

func functionWithSuccessResponse() func() (*v1.StatusResponse, error) {
	return func() (*v1.StatusResponse, error) {
		return &v1.StatusResponse{Status: v1.StatusResponse_SUCCESS}, nil
	}
}

func functionWithFailureResponse() func() (*v1.StatusResponse, error) {
	return func() (*v1.StatusResponse, error) {
		return &v1.StatusResponse{Status: v1.StatusResponse_FAIL}, nil
	}
}

func functionWithErrorResponse() func() (*v1.StatusResponse, error) {
	return func() (*v1.StatusResponse, error) {
		return &v1.StatusResponse{Status: v1.StatusResponse_FAIL}, errors.New("error")
	}
}

func recoverHTTPTestServerWithCacheItems(
	cacheManager *cache.Manager,
	cacheItems map[*cache.Key]func() (*v1.StatusResponse, error),
) (*httptest.Server, error) {
	for key, val := range cacheItems {
		err := cacheManager.Register(key, val)
		if err != nil {
			return nil, err
		}
	}

	rController := &RController{
		cacheManager: cacheManager,
		logger:       logger,
	}

	router := mux.NewRouter()
	router.HandleFunc("/recover/alertmanager", rController.RecoverActionAlertmanagerWebHook).
		Methods("POST")

	return httptest.NewServer(router), nil
}

func newRequestPayload(alerts []*Alert) *requestPayload {
	return &requestPayload{
		Alerts: alerts,
	}
}

func restorePostCall(server *httptest.Server, requestPayload *requestPayload) (*response.RecoverResponsePayload, error) {
	requestBody, _ := json.Marshal(requestPayload)
	url := server.URL + "/recover/alertmanager"

	return post(requestBody, url)
}

func post(requestBody []byte, url string) (*response.RecoverResponsePayload, error) {
	resp, err := http.Post(url, "", bytes.NewReader(requestBody)) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respPayload := &response.RecoverResponsePayload{}
	err = json.NewDecoder(resp.Body).Decode(&respPayload)
	if err != nil {
		return &response.RecoverResponsePayload{Status: resp.StatusCode}, err
	}
	return respPayload, nil
}

func getLogger() log.Logger {
	allowLevel := &chaoslogger.AllowedLevel{}
	if err := allowLevel.Set("debug"); err != nil {
		fmt.Printf("%v", err)
	}

	return chaoslogger.New(allowLevel)
}
