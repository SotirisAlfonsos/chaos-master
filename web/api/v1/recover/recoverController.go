package recover

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/pkg/cache"
	"github.com/SotirisAlfonsos/chaos-master/pkg/chaoslogger"
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

// RecoverActionAlertmanagerWebHook godoc
// @Summary recover from failures
// @Description Alertmanager webhook to recover from failures
// @Tags Recover
// @Accept json
// @Produce json
// @Param RequestPayload body RequestPayload true "Create request payload that contains the recovery details"
// @Success 200 {object} response.RecoverResponsePayload
// @Failure 400 {string} http.Error
// @Router /recover/alertmanager [post]
func (rController *RController) RecoverActionAlertmanagerWebHook(w http.ResponseWriter, r *http.Request) {
	recoverMessages := make([]*response.RecoverMessage, 0)

	requestPayload := &RequestPayload{}
	err := json.NewDecoder(r.Body).Decode(&requestPayload)
	if err != nil {
		response.BadRequest(w, "Could not decode request body", rController.loggers)
		return
	}

	for _, alert := range requestPayload.Alerts {
		status, err := toStatusEnum(alert.Status)
		if err != nil {
			response.BadRequest(w, err.Error(), rController.loggers)
			return
		} else if status == firing {
			recoverMessages = rController.performActionBasedOnLabels(alert.Labels)
		}
	}

	response.RecoverResponse(w, recoverMessages, rController.loggers)
}

type RequestPayload struct {
	Alerts []*Alert `json:"alerts"`
}

type Alert struct {
	Status string `json:"status"`
	Labels Labels `json:"labels"`
}

type Labels struct {
	RecoverJob    string `json:"recoverJob,omitempty"`
	RecoverTarget string `json:"recoverTarget,omitempty"`
	RecoverAll    bool   `json:"recoverAll,omitempty"`
}

func (rController *RController) performActionBasedOnLabels(labels Labels) []*response.RecoverMessage {
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

func (rController *RController) recoverJob(items []gocache.Item, labels Labels, wg *sync.WaitGroup) []*response.RecoverMessage {
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

func (rController *RController) recoverTarget(items []gocache.Item, labels Labels, wg *sync.WaitGroup) []*response.RecoverMessage {
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

type alertStatus int

const (
	firing alertStatus = iota
	resolved
	invalid
)

func (s alertStatus) String() string {
	return [...]string{"firing", "resolved"}[s]
}

func toStatusEnum(value string) (alertStatus, error) {
	switch value {
	case firing.String():
		return firing, nil
	case resolved.String():
		return resolved, nil
	}
	return invalid, errors.New(fmt.Sprintf("The status {%s} is not supported", value))
}
