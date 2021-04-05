package recover

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/SotirisAlfonsos/gocache"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/pkg/cache"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/response"
	"github.com/stretchr/testify/assert"
)

type RecoverTestData struct {
	message    string
	cacheItems map[cache.Key]func() (*v1.StatusResponse, error)
	options    *Options
	expected   *expectedResult
}

func TestRecoverAllRequestSuccess(t *testing.T) { //nolint:dupl
	dataItems := []RecoverTestData{
		{
			message: "Successfully recover all items from cache",
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job", Target: "127.0.0.1"}: functionWithSuccessResponse(),
				cache.Key{Job: "job", Target: "127.0.0.2"}: functionWithSuccessResponse(),
			},
			options:  &Options{RecoverAll: true},
			expected: &expectedResult{cacheSize: 0, response: recoverResponse(200, "SUCCESS", "SUCCESS")},
		},
		{
			message: "Successfully recover one item from the cache and record failure on the rest",
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job_1", Target: "127.0.0.1"}: functionWithSuccessResponse(),
				cache.Key{Job: "job", Target: "127.0.0.2"}:   functionWithFailureResponse(),
				cache.Key{Job: "job", Target: "127.0.0.3"}:   functionWithErrorResponse(),
			},
			options:  &Options{RecoverAll: true},
			expected: &expectedResult{cacheSize: 2, response: recoverResponse(500, "SUCCESS", "FAILURE", "FAILURE")},
		},
		{
			message:    "Should not do anything and return ok for cache already empty",
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){},
			options:    &Options{RecoverAll: true},
			expected:   &expectedResult{cacheSize: 0, response: recoverResponse(200)},
		},
	}

	for _, dataItem := range dataItems {
		assertSuccessfulRecovery(t, dataItem)
	}
}

func TestRecoverJobRequestSuccess(t *testing.T) { //nolint:dupl
	dataItems := []RecoverTestData{
		{
			message: "Successfully recover all items from cache for job {job name}, while not removing other jobs",
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job name", Target: "127.0.0.1"}:           functionWithSuccessResponse(),
				cache.Key{Job: "job name", Target: "127.0.0.2"}:           functionWithSuccessResponse(),
				cache.Key{Job: "job different name", Target: "127.0.0.2"}: functionWithSuccessResponse(),
			},
			options:  &Options{RecoverJob: "job name"},
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
			options:  &Options{RecoverJob: "job name"},
			expected: &expectedResult{cacheSize: 3, response: recoverResponse(500, "SUCCESS", "FAILURE", "FAILURE")},
		},
		{
			message: "Should not do anything and return ok for job {job name} not in cache",
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job different name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
			},
			options:  &Options{RecoverJob: "job name"},
			expected: &expectedResult{cacheSize: 1, response: recoverResponse(200)},
		},
	}

	for _, dataItem := range dataItems {
		assertSuccessfulRecovery(t, dataItem)
	}
}

func TestRecoverTargetRequestSuccess(t *testing.T) { //nolint:dupl
	dataItems := []RecoverTestData{
		{
			message: "Successfully recover all items from cache for target {127.0.0.1}, while not removing other jobs",
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job name", Target: "127.0.0.1"}:           functionWithSuccessResponse(),
				cache.Key{Job: "job different name", Target: "127.0.0.1"}: functionWithSuccessResponse(),
				cache.Key{Job: "job different name", Target: "127.0.0.2"}: functionWithSuccessResponse(),
			},
			options:  &Options{RecoverTarget: "127.0.0.1"},
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
			options:  &Options{RecoverTarget: "127.0.0.1"},
			expected: &expectedResult{cacheSize: 3, response: recoverResponse(500, "SUCCESS", "FAILURE", "FAILURE")},
		},
		{
			message: "Should not do anything and return ok for target {127.0.0.1} not in cache",
			cacheItems: map[cache.Key]func() (*v1.StatusResponse, error){
				cache.Key{Job: "job different name", Target: "127.0.0.2"}: functionWithSuccessResponse(),
			},
			options:  &Options{RecoverTarget: "127.0.0.1"},
			expected: &expectedResult{cacheSize: 1, response: recoverResponse(200)},
		},
	}

	for _, dataItem := range dataItems {
		assertSuccessfulRecovery(t, dataItem)
	}
}

func assertSuccessfulRecovery(t *testing.T, dataItem RecoverTestData) {
	t.Run(dataItem.message, func(t *testing.T) {
		cacheManager := gocache.New(0)
		server, err := recoverHTTPTestServerWithCacheItems(cacheManager, dataItem.cacheItems)
		if err != nil {
			t.Fatal(err)
		}

		status, httpErrorMessage, recoverMessages, err := restorePostCall(server, dataItem.options)
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

func restorePostCall(server *httptest.Server, options *Options) (int, string, []*response.RecoverMessage, error) {
	request, _ := json.Marshal(options)
	url := server.URL + "/recover"

	return post(request, url)
}
