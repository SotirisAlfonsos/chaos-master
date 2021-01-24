package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/cache"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
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
	response := newDefaultRecoverResponse()

	requestPayload := &requestPayload{}
	err := json.NewDecoder(r.Body).Decode(&requestPayload)
	if err != nil {
		response.badRequest("Could not decode request body", rController.logger)
		setRecoverResponseInWriter(w, response, rController.logger)
		return
	}

	for _, alert := range requestPayload.Alerts {
		if status, err := toStatusEnum(alert.Status); status == firing && err == nil {
			rController.performActionBasedOnLabels(alert.Labels, response)
		} else if err != nil {
			_ = level.Error(rController.logger).Log("msg", "Could not get status of incoming firing alert", "err", err)
		}
	}

	setRecoverResponseInWriter(w, response, rController.logger)
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

func (rController *RController) performActionBasedOnLabels(labels Labels, response *RecoverResponsePayload) {
	var wg sync.WaitGroup
	items := rController.cacheManager.GetAll()
	switch {
	case labels.RecoverAll:
		for key, val := range items {
			rController.performAsync(key, val.Object.(func() (*v1.StatusResponse, error)), response, &wg)
		}
	case labels.RecoverJob != "":
		for key, val := range items {
			jobNameTarget := strings.Split(key, ",")
			if jobNameTarget[0] == labels.RecoverJob {
				rController.performAsync(key, val.Object.(func() (*v1.StatusResponse, error)), response, &wg)
			}
		}
	case labels.RecoverTarget != "":
		for key, val := range items {
			jobNameTarget := strings.Split(key, ",")
			if jobNameTarget[2] == labels.RecoverTarget {
				rController.performAsync(key, val.Object.(func() (*v1.StatusResponse, error)), response, &wg)
			}
		}
	}

	wg.Wait()
}

func (rController *RController) performAsync(key string, val func() (*v1.StatusResponse, error), response *RecoverResponsePayload, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		recoverMsg := action(key, val, rController.cacheManager)
		response.RecoverMessage = append(response.RecoverMessage, recoverMsg)
	}()
}

func action(uniqueName string, function func() (*v1.StatusResponse, error), cache *cache.Manager) *RecoverMessage {
	jobTarget := strings.Split(uniqueName, ",")
	statusResponse, err := function()

	switch {
	case err != nil:
		return failureRecoverResponse(errors.Wrap(err, fmt.Sprintf("Error response from target {%s}", jobTarget[1])).Error())
	case statusResponse.Status != v1.StatusResponse_SUCCESS:
		return failureRecoverResponse(fmt.Sprintf("Failure response from target {%s}", jobTarget[1]))
	}
	cache.Delete(uniqueName)
	message := fmt.Sprintf("Response from target {%s}, {%s}, {%s}", jobTarget[1], statusResponse.Message, statusResponse.Status)
	return successRecoverResponse(message)
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
