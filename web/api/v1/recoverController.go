package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/SotirisAlfonsos/chaos-bot/proto"
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

type requestPayload struct {
	Alerts []*Alert `json:"alerts"`
}

type Alert struct {
	Status string `json:"status"`
	Labels Labels `json:"labels"`
}

type Labels struct {
	RecoverJob    string `json:"recoverJob"`
	RecoverTarget string `json:"recoverTarget"`
	RecoverAll    bool   `json:"recoverAll"`
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

func (rController *RController) performActionBasedOnLabels(labels Labels, response *RecoverResponsePayload) {
	var wg sync.WaitGroup
	items := rController.cacheManager.GetAll()
	switch {
	case labels.RecoverAll:
		for key, val := range items {
			wg.Add(1)
			go func(key string, val func() (*proto.StatusResponse, error)) {
				defer wg.Done()
				recoverMsg := performAction(key, val, rController.cacheManager)
				response.RecoverMessage = append(response.RecoverMessage, recoverMsg)
			}(key, val.Object.(func() (*proto.StatusResponse, error)))
		}
	case labels.RecoverJob != "":
		for key, val := range items {
			jobNameTarget := strings.Split(key, ",")
			if jobNameTarget[0] == labels.RecoverJob {
				wg.Add(1)
				go func(key string, val func() (*proto.StatusResponse, error)) {
					defer wg.Done()
					recoverMsg := performAction(key, val, rController.cacheManager)
					response.RecoverMessage = append(response.RecoverMessage, recoverMsg)
				}(key, val.Object.(func() (*proto.StatusResponse, error)))
			}
		}
	case labels.RecoverTarget != "":
		for key, val := range items {
			jobNameTarget := strings.Split(key, ",")
			if jobNameTarget[2] == labels.RecoverTarget {
				wg.Add(1)
				go func(key string, val func() (*proto.StatusResponse, error)) {
					defer wg.Done()
					recoverMsg := performAction(key, val, rController.cacheManager)
					response.RecoverMessage = append(response.RecoverMessage, recoverMsg)
				}(key, val.Object.(func() (*proto.StatusResponse, error)))
			}
		}
	}

	wg.Wait()
}

func performAction(uniqueName string, function func() (*proto.StatusResponse, error), cache *cache.Manager) *RecoverMessage {
	jobNameTarget := strings.Split(uniqueName, ",")
	statusResponse, err := function()

	switch {
	case err != nil:
		return failureRecoverResponse(errors.Wrap(err, fmt.Sprintf("Error response from target {%s}", jobNameTarget[2])).Error())
	case statusResponse.Status != proto.StatusResponse_SUCCESS:
		return failureRecoverResponse(fmt.Sprintf("Failure response from target {%s}", jobNameTarget[2]))
	}
	cache.Delete(uniqueName)
	message := fmt.Sprintf("Response from target {%s}, {%s}, {%s}", jobNameTarget[2], statusResponse.Message, statusResponse.Status)
	return successRecoverResponse(message)
}
