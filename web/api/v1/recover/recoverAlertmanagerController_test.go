package recover

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"

	"github.com/SotirisAlfonsos/gocache"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/pkg/cache"
	"github.com/SotirisAlfonsos/chaos-master/pkg/chaoslogger"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/response"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

var (
	loggers = getLogger()
)

type TestData struct {
	message    string
	cacheItems map[cache.Key]func() (*v1.StatusResponse, error)
	alerts     []*Alert
	expected   *expectedResult
}

type expectedResult struct {
	cacheSize int
	response  *responseWrapper
}

type responseWrapper struct {
	status          int
	httpErrMessage  string
	recoverMessages []*response.RecoverMessage
	err             error
}

func TestRecoverAllRequestSuccessWithAlertmanagerWebhook(t *testing.T) { //nolint:dupl
	dataItems := []TestData{
		{
			message: "Successfully recover all items from cache",
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job", Target: "127.0.0.1"}: functionWithSuccessResponse(),
				cache.Key{Job: "job", Target: "127.0.0.2"}: functionWithSuccessResponse(),
			},
			alerts: []*Alert{{
				Status: "firing", Labels: Options{RecoverAll: true},
			}},
			expected: &expectedResult{cacheSize: 0, response: recoverResponse(200, "SUCCESS", "SUCCESS")},
		},
		{
			message: "Successfully recover one item from the cache and record failure on the rest",
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job_1", Target: "127.0.0.1"}: functionWithSuccessResponse(),
				cache.Key{Job: "job", Target: "127.0.0.2"}:   functionWithFailureResponse(),
				cache.Key{Job: "job", Target: "127.0.0.3"}:   functionWithErrorResponse(),
			},
			alerts: []*Alert{{
				Status: "firing", Labels: Options{RecoverAll: true},
			}},
			expected: &expectedResult{cacheSize: 2, response: recoverResponse(500, "SUCCESS", "FAILURE", "FAILURE")},
		},
		{
			message:    "Should not do anything and return ok for cache already empty",
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){},
			alerts: []*Alert{{
				Status: "firing", Labels: Options{RecoverAll: true},
			}},
			expected: &expectedResult{cacheSize: 0, response: recoverResponse(200)},
		},
	}

	for _, dataItem := range dataItems {
		assertSuccessfulRecoveryWithAlertmanagerWebhook(t, dataItem)
	}
}

func TestRecoverJobRequestSuccessWithAlertmanagerWebhook(t *testing.T) { //nolint:dupl
	dataItems := []TestData{
		{
			message: "Successfully recover all items from cache for job {job name}, while not removing other jobs",
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job name", Target: "127.0.0.1"}:           functionWithSuccessResponse(),
				cache.Key{Job: "job name", Target: "127.0.0.2"}:           functionWithSuccessResponse(),
				cache.Key{Job: "job different name", Target: "127.0.0.2"}: functionWithSuccessResponse(),
			},
			alerts: []*Alert{{
				Status: "firing", Labels: Options{RecoverJob: "job name"},
			}},
			expected: &expectedResult{cacheSize: 1, response: recoverResponse(200, "SUCCESS", "SUCCESS")},
		},
		{
			message: "Successfully recover one item from the cache and record failure on the rest for job {job name}, while not removing other jobs",
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job name", Target: "127.0.0.1"}:           functionWithSuccessResponse(),
				cache.Key{Job: "job name", Target: "127.0.0.2"}:           functionWithFailureResponse(),
				cache.Key{Job: "job name", Target: "127.0.0.3"}:           functionWithErrorResponse(),
				cache.Key{Job: "job different name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
			},
			alerts: []*Alert{{
				Status: "firing", Labels: Options{RecoverJob: "job name"},
			}},
			expected: &expectedResult{cacheSize: 3, response: recoverResponse(500, "SUCCESS", "FAILURE", "FAILURE")},
		},
		{
			message: "Should not do anything and return ok for job {job name} not in cache",
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job different name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
			},
			alerts: []*Alert{{
				Status: "firing", Labels: Options{RecoverJob: "job name"},
			}},
			expected: &expectedResult{cacheSize: 1, response: recoverResponse(200)},
		},
	}

	for _, dataItem := range dataItems {
		assertSuccessfulRecoveryWithAlertmanagerWebhook(t, dataItem)
	}
}

func TestRecoverTargetRequestSuccessWithAlertmanagerWebhook(t *testing.T) { //nolint:dupl
	dataItems := []TestData{
		{
			message: "Successfully recover all items from cache for target {127.0.0.1}, while not removing other jobs",
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job name", Target: "127.0.0.1"}:           functionWithSuccessResponse(),
				cache.Key{Job: "job different name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
				cache.Key{Job: "job different name", Target: "127.0.0.2"}: functionWithSuccessResponse(),
			},
			alerts: []*Alert{{
				Status: "firing", Labels: Options{RecoverTarget: "127.0.0.1"},
			}},
			expected: &expectedResult{cacheSize: 1, response: recoverResponse(200, "SUCCESS", "SUCCESS")},
		},
		{
			message: "Successfully recover one item from the cache and record failure on the rest for target {127.0.0.1}, while not removing other jobs",
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job name", Target: "127.0.0.1"}:           functionWithSuccessResponse(),
				cache.Key{Job: "job other name", Target: "127.0.0.1"}:     functionWithFailureResponse(),
				cache.Key{Job: "job different name", Target: "127.0.0.1"}: functionWithErrorResponse(),
				cache.Key{Job: "job different name", Target: "127.0.0.2"}: functionWithSuccessResponse(),
			},
			alerts: []*Alert{{
				Status: "firing", Labels: Options{RecoverTarget: "127.0.0.1"},
			}},
			expected: &expectedResult{cacheSize: 3, response: recoverResponse(500, "SUCCESS", "FAILURE", "FAILURE")},
		},
		{
			message: "Should not do anything and return ok for target {127.0.0.1} not in cache",
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job different name", Target: "127.0.0.2"}: functionWithSuccessResponse(),
			},
			alerts: []*Alert{{
				Status: "firing", Labels: Options{RecoverTarget: "127.0.0.1"},
			}},
			expected: &expectedResult{cacheSize: 1, response: recoverResponse(200)},
		},
	}

	for _, dataItem := range dataItems {
		assertSuccessfulRecoveryWithAlertmanagerWebhook(t, dataItem)
	}
}

func TestRecoverRequestForNotFiringAlertsWithAlertmanagerWebhook(t *testing.T) { //nolint:dupl
	dataItems := []TestData{
		{
			message: "Successfully recover items on firing and ignore resolved alerts, resulting in two jobs recovered",
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job name", Target: "127.0.0.1"}:           functionWithSuccessResponse(),
				cache.Key{Job: "job different name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
				cache.Key{Job: "job different name", Target: "127.0.0.2"}: functionWithSuccessResponse(),
			},
			alerts: []*Alert{
				{Status: "firing", Labels: Options{RecoverTarget: "127.0.0.1"}},
				{Status: "resolved", Labels: Options{RecoverTarget: "127.0.0.1"}},
				{Status: "resolved", Labels: Options{RecoverAll: true}},
				{Status: "resolved", Labels: Options{RecoverJob: "job name"}},
			},
			expected: &expectedResult{cacheSize: 1, response: recoverResponse(200, "SUCCESS", "SUCCESS")},
		},
		{
			message: "Should not do anything for resolved alerts, no items should be removed from cache",
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job name", Target: "127.0.0.1"}:       functionWithSuccessResponse(),
				cache.Key{Job: "job other name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
			},
			alerts: []*Alert{
				{Status: "resolved", Labels: Options{RecoverTarget: "127.0.0.1"}},
				{Status: "resolved", Labels: Options{RecoverAll: true}},
				{Status: "resolved", Labels: Options{RecoverJob: "job name"}},
			},
			expected: &expectedResult{cacheSize: 2, response: recoverResponse(200)},
		},
		{
			message: "Should not do anything for unknown alert status, no items should be removed from cache",
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job different name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
			},
			alerts: []*Alert{{
				Status: "unknown", Labels: Options{RecoverAll: true},
			}},
			expected: &expectedResult{cacheSize: 1, response: badRequestResponse("The status {unknown} is not supported")},
		},
	}

	for _, dataItem := range dataItems {
		assertSuccessfulRecoveryWithAlertmanagerWebhook(t, dataItem)
	}
}

func assertSuccessfulRecoveryWithAlertmanagerWebhook(t *testing.T, dataItem TestData) {
	t.Run(dataItem.message, func(t *testing.T) {
		cacheManager := gocache.New(0)
		server, err := recoverHTTPTestServerWithCacheItems(cacheManager, dataItem.cacheItems)
		if err != nil {
			t.Fatal(err)
		}
		requestPayload := newRequestPayload(dataItem.alerts)

		status, httpErrorMessage, recoverMessages, err := restoreAlertmanagerWebhookPostCall(server, requestPayload)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, dataItem.expected.response.status, status)
		assert.Equal(t, len(dataItem.expected.response.recoverMessages), len(recoverMessages))
		assert.Equal(t, dataItem.expected.response.httpErrMessage, httpErrorMessage)
		assert.Equal(t, getSortedStatuses(dataItem.expected.response.recoverMessages), getSortedStatuses(recoverMessages))
		assert.Equal(t, dataItem.expected.cacheSize, cacheManager.ItemCount())
	})
}

func recoverResponse(status int, statuses ...string) *responseWrapper {
	recoverMessages := make([]*response.RecoverMessage, 0, len(statuses))
	for _, status := range statuses {
		recoverMessages = append(recoverMessages, &response.RecoverMessage{Status: status})
	}
	return &responseWrapper{
		recoverMessages: recoverMessages,
		status:          status,
	}
}

func badRequestResponse(message string) *responseWrapper {
	return &responseWrapper{
		httpErrMessage: message + "\n",
		status:         400,
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
	cache *gocache.Cache,
	cacheItems map[cache.Key]func() (*v1.StatusResponse, error),
) (*httptest.Server, error) {
	for key, val := range cacheItems {
		cache.Set(key, val)
	}

	rController := &RController{
		cache:   cache,
		loggers: loggers,
	}

	router := mux.NewRouter()
	router.HandleFunc("/recover", rController.RecoverAction).
		Methods("POST")
	router.HandleFunc("/recover/alertmanager", rController.RecoverActionAlertmanagerWebHook).
		Methods("POST")

	return httptest.NewServer(router), nil
}

func newRequestPayload(alerts []*Alert) *RequestPayload {
	return &RequestPayload{
		Alerts: alerts,
	}
}

func restoreAlertmanagerWebhookPostCall(server *httptest.Server, requestPayload *RequestPayload) (int, string, []*response.RecoverMessage, error) {
	requestBody, _ := json.Marshal(requestPayload)
	url := server.URL + "/recover/alertmanager"

	return post(requestBody, url)
}

func post(requestBody []byte, url string) (int, string, []*response.RecoverMessage, error) {
	resp, err := http.Post(url, "", bytes.NewReader(requestBody)) //nolint:gosec
	if err != nil {
		return 0, "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		b, _ := ioutil.ReadAll(resp.Body)
		return resp.StatusCode, string(b), nil, nil
	}

	respPayload := &response.RecoverResponsePayload{}
	err = json.NewDecoder(resp.Body).Decode(&respPayload)
	if err != nil {
		return resp.StatusCode, "", nil, err
	}
	return respPayload.Status, "", respPayload.RecoverMessage, nil
}

func getSortedStatuses(recoverMessages []*response.RecoverMessage) []string {
	statuses := make([]string, 0, len(recoverMessages))
	for _, recoverMessage := range recoverMessages {
		statuses = append(statuses, recoverMessage.Status)
	}
	sort.Strings(statuses)
	return statuses
}

func getLogger() chaoslogger.Loggers {
	allowLevel := &chaoslogger.AllowedLevel{}
	if err := allowLevel.Set("debug"); err != nil {
		fmt.Printf("%v", err)
	}

	return chaoslogger.Loggers{
		OutLogger: chaoslogger.New(allowLevel, os.Stdout),
		ErrLogger: chaoslogger.New(allowLevel, os.Stderr),
	}
}
