package recover

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/cache"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/response"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
)

type RController struct {
	cacheManager *cache.Manager
	logger       log.Logger
}

func NewRecoverController(cache *cache.Manager, logger log.Logger) *RController {
	return &RController{
		cacheManager: cache,
		logger:       logger,
	}
}

// CalcExample godoc
// @Summary recover from failures
// @Description Alertmanager webhook to recover from failures
// @Tags Recover
// @Accept json
// @Produce json
// @Param requestPayload body requestPayload true "Create request payload that contains the recovery details"
// @Success 200 {object} RecoverResponsePayload
// @Failure 400 {object} RecoverResponsePayload
// @Router /recover/alertmanager [post]
func (rController *RController) RecoverActionAlertmanagerWebHook(w http.ResponseWriter, r *http.Request) {
	resp := response.NewDefaultRecoverResponse()

	requestPayload := &requestPayload{}
	err := json.NewDecoder(r.Body).Decode(&requestPayload)
	if err != nil {
		resp.BadRequest("Could not decode request body", rController.logger)
		resp.SetInWriter(w, rController.logger)
		return
	}

	for _, alert := range requestPayload.Alerts {
		if status, err := toStatusEnum(alert.Status); status == firing && err == nil {
			rController.performActionBasedOnLabels(alert.Labels, resp)
		} else if err != nil {
			resp.BadRequest(err.Error(), rController.logger)
		}
	}

	resp.SetInWriter(w, rController.logger)
}

type requestPayload struct {
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

func (rController *RController) performActionBasedOnLabels(labels Labels, resp *response.RecoverResponsePayload) {
	items := rController.cacheManager.GetAll()

	switch {
	case labels.RecoverAll:
		rController.recoverAll(items, resp)
	case labels.RecoverJob != "":
		rController.recoverJob(items, labels, resp)
	case labels.RecoverTarget != "":
		rController.recoverTarget(items, labels, resp)
	}
}

func (rController *RController) recoverAll(items cache.Items, resp *response.RecoverResponsePayload) {
	var wg sync.WaitGroup
	for key, val := range items {
		wg.Add(1)
		rController.performAsync(key, val.Object.(func() (*v1.StatusResponse, error)), resp, &wg)
	}
	wg.Wait()
}

func (rController *RController) recoverJob(items cache.Items, labels Labels, resp *response.RecoverResponsePayload) {
	var wg sync.WaitGroup
	for key, val := range items {
		if key.Job == labels.RecoverJob {
			wg.Add(1)
			rController.performAsync(key, val.Object.(func() (*v1.StatusResponse, error)), resp, &wg)
		}
	}
	wg.Wait()
}

func (rController *RController) recoverTarget(items cache.Items, labels Labels, resp *response.RecoverResponsePayload) {
	var wg sync.WaitGroup
	for key, val := range items {
		if key.Target == labels.RecoverTarget {
			wg.Add(1)
			rController.performAsync(key, val.Object.(func() (*v1.StatusResponse, error)), resp, &wg)
		}
	}
	wg.Wait()
}

func (rController *RController) performAsync(
	key cache.Key,
	val func() (*v1.StatusResponse, error),
	resp *response.RecoverResponsePayload,
	wg *sync.WaitGroup,
) {
	go func() {
		defer wg.Done()
		recoverMsg := action(&key, val, rController.cacheManager)
		resp.RecoverMessage = append(resp.RecoverMessage, recoverMsg)
	}()
}

func action(key *cache.Key, function func() (*v1.StatusResponse, error), cache *cache.Manager) *response.RecoverMessage {
	statusResponse, err := function()

	switch {
	case err != nil:
		return response.FailureRecoverResponse(errors.Wrap(err, fmt.Sprintf("Error response from target {%s}", key.Target)).Error())
	case statusResponse.Status != v1.StatusResponse_SUCCESS:
		return response.FailureRecoverResponse(fmt.Sprintf("Failure response from target {%s}", key.Target))
	}
	cache.Delete(key)
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
