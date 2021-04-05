package recover

import (
	"fmt"
	"sync"

	"github.com/SotirisAlfonsos/chaos-master/pkg/chaoslogger"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/pkg/cache"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/response"
	"github.com/SotirisAlfonsos/gocache"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
)

type RController struct {
	cache   *gocache.Cache
	loggers chaoslogger.Loggers
}

func NewRecoverController(cache *gocache.Cache, loggers chaoslogger.Loggers) *RController {
	return &RController{
		cache:   cache,
		loggers: loggers,
	}
}

func (rController *RController) performActionBasedOnOptions(labels Options) []*response.RecoverMessage {
	items := rController.cache.GetAll()
	var wg sync.WaitGroup

	switch {
	case labels.RecoverAll:
		return rController.recoverAll(items, &wg)
	case labels.RecoverJob != "":
		return rController.recoverJob(items, labels, &wg)
	case labels.RecoverTarget != "":
		return rController.recoverTarget(items, labels, &wg)
	}

	return make([]*response.RecoverMessage, 0)
}

func (rController *RController) recoverAll(items []gocache.Item, wg *sync.WaitGroup) []*response.RecoverMessage {
	messages := make([]*response.RecoverMessage, 0)
	for _, item := range items {
		wg.Add(1)
		key := item.Key.(cache.Key)
		val := item.Value.(func() (*v1.StatusResponse, error))
		go func() {
			defer wg.Done()
			messages = append(messages, rController.action(&key, val))
		}()
	}

	wg.Wait()
	return messages
}

func (rController *RController) recoverJob(items []gocache.Item, labels Options, wg *sync.WaitGroup) []*response.RecoverMessage {
	messages := make([]*response.RecoverMessage, 0)
	for _, item := range items {
		if item.Key.(cache.Key).Job == labels.RecoverJob {
			wg.Add(1)
			key := item.Key.(cache.Key)
			val := item.Value.(func() (*v1.StatusResponse, error))
			go func() {
				defer wg.Done()
				messages = append(messages, rController.action(&key, val))
			}()
		}
	}

	wg.Wait()
	return messages
}

func (rController *RController) recoverTarget(items []gocache.Item, labels Options, wg *sync.WaitGroup) []*response.RecoverMessage {
	messages := make([]*response.RecoverMessage, 0)
	for _, item := range items {
		if item.Key.(cache.Key).Target == labels.RecoverTarget {
			wg.Add(1)
			key := item.Key.(cache.Key)
			val := item.Value.(func() (*v1.StatusResponse, error))
			go func() {
				defer wg.Done()
				messages = append(messages, rController.action(&key, val))
			}()
		}
	}

	wg.Wait()
	return messages
}

func (rController *RController) action(key *cache.Key, function func() (*v1.StatusResponse, error)) *response.RecoverMessage {
	statusResponse, err := function()
	_ = level.Info(rController.loggers.OutLogger).Log("msg", fmt.Sprintf("recover job item {%s} from cache on target {%s}", key.Job, key.Target))

	switch {
	case err != nil:
		return response.FailureRecoverResponse(errors.Wrap(err, fmt.Sprintf("Error response from target {%s}", key.Target)).Error())
	case statusResponse.Status != v1.StatusResponse_SUCCESS:
		return response.FailureRecoverResponse(fmt.Sprintf("Failure response from target {%s}", key.Target))
	}
	rController.cache.Delete(key)
	message := fmt.Sprintf("Response from target {%s}, {%s}, {%s}", key.Target, statusResponse.Message, statusResponse.Status)
	return response.SuccessRecoverResponse(message)
}
