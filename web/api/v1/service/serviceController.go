package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/response"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/cache"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/SotirisAlfonsos/chaos-master/network"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
)

type SController struct {
	jobs           map[string]*config.Job
	connectionPool map[string]*sConnection
	cacheManager   *cache.Manager
	logger         log.Logger
}

type sConnection struct {
	connection network.Connection
}

func NewServiceController(
	jobs map[string]*config.Job,
	connections *network.Connections,
	cache *cache.Manager,
	logger log.Logger,
) *SController {
	connPool := make(map[string]*sConnection)
	for target, connection := range connections.Pool {
		connPool[target] = &sConnection{
			connection: connection,
		}
	}
	return &SController{
		jobs:           jobs,
		connectionPool: connPool,
		cacheManager:   cache,
		logger:         logger,
	}
}

type action int

const (
	stop action = iota
	start
	notImplemented
)

func (a action) String() string {
	return [...]string{"stop", "start"}[a]
}

func toActionEnum(value string) (action, error) {
	switch value {
	case start.String():
		return start, nil
	case stop.String():
		return stop, nil
	}
	return notImplemented, errors.New(fmt.Sprintf("The action {%s} is not supported", value))
}

type RequestPayload struct {
	Job         string `json:"job"`
	ServiceName string `json:"serviceName"`
	Target      string `json:"target"`
}

func newServiceRequest(details *RequestPayload) *v1.ServiceRequest {
	return &v1.ServiceRequest{
		JobName:              details.Job,
		Name:                 details.ServiceName,
		XXX_NoUnkeyedLiteral: struct{}{},
		XXX_unrecognized:     nil,
		XXX_sizecache:        0,
	}
}

// CalcExample godoc
// @Summary Inject service failures
// @Description Perform start or stop action on a service
// @Tags Failure injections
// @Accept json
// @Produce json
// @Param action query string true "Specify to perform a start or a stop on the specified service"
// @Param requestPayload body RequestPayload true "Specify the job name, service name and target"
// @Success 200 {object} ResponsePayload
// @Failure 400 {object} ResponsePayload
// @Failure 500 {object} ResponsePayload
// @Router /service [post]
func (s *SController) ServiceAction(w http.ResponseWriter, r *http.Request) {
	resp := response.NewDefaultResponse()

	requestPayload := &RequestPayload{}
	err := json.NewDecoder(r.Body).Decode(&requestPayload)
	if err != nil {
		resp.BadRequest("Could not decode request body", s.logger)
		resp.SetInWriter(w, s.logger)
		return
	}

	action, err := toActionEnum(r.FormValue("action"))
	if err != nil {
		resp.BadRequest(err.Error(), s.logger)
		resp.SetInWriter(w, s.logger)
		return
	}

	_ = level.Info(s.logger).Log("msg", fmt.Sprintf("%s service with name {%s}", action, requestPayload.ServiceName))

	job, ok := s.jobs[requestPayload.Job]
	if !ok {
		resp.BadRequest(fmt.Sprintf("Could not find job {%s}", requestPayload.Job), s.logger)
		resp.SetInWriter(w, s.logger)
		return
	}

	serviceExists := false
	for _, target := range job.Target {
		if target == requestPayload.Target && job.ComponentName == requestPayload.ServiceName {
			serviceRequest := newServiceRequest(requestPayload)
			s.performAction(action, s.connectionPool[target], serviceRequest, requestPayload.Target, resp)
			serviceExists = true
		}
	}

	if !serviceExists {
		resp.BadRequest(fmt.Sprintf("Service {%s} is not registered for target {%s}", requestPayload.ServiceName, requestPayload.Target), s.logger)
	}

	resp.SetInWriter(w, s.logger)
}

func (s *SController) performAction(
	action action,
	sConnection *sConnection,
	serviceRequest *v1.ServiceRequest,
	target string,
	resp *response.Payload,
) {
	var statusResponse *v1.StatusResponse
	var err error

	serviceClient, err := sConnection.connection.GetServiceClient(target)
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("Can not get service connection from target {%s}", target))
		resp.InternalServerError(err.Error(), s.logger)
		return
	}

	switch action {
	case start:
		statusResponse, err = serviceClient.Start(context.Background(), serviceRequest)
	case stop:
		statusResponse, err = serviceClient.Stop(context.Background(), serviceRequest)
	default:
		resp.BadRequest(fmt.Sprintf("Action {%s} not allowed", action), s.logger)
		return
	}

	message, err := handleGRPCResponse(statusResponse, err, target)
	if err != nil {
		resp.InternalServerError(err.Error(), s.logger)
		return
	}

	resp.Message = message
	_ = level.Info(s.logger).Log("msg", resp.Message)

	if err = s.updateCache(serviceRequest, serviceClient, target, action); err != nil {
		_ = level.Error(s.logger).Log("msg", fmt.Sprintf("Could not update cache for operation %s", action), "err", err)
	}
}

func handleGRPCResponse(statusResponse *v1.StatusResponse, err error, target string) (string, error) {
	switch {
	case err != nil:
		return "", errors.Wrap(err, fmt.Sprintf("Error response from target {%s}", target))
	case statusResponse.Status != v1.StatusResponse_SUCCESS:
		return "", errors.New(fmt.Sprintf("Failure response from target {%s}", target))
	default:
		return fmt.Sprintf("Response from target {%s}, {%s}, {%s}", target, statusResponse.Message, statusResponse.Status), nil
	}
}

func (s *SController) updateCache(serviceRequest *v1.ServiceRequest, serviceClient v1.ServiceClient, target string, action action) error {
	key := &cache.Key{
		Job:    serviceRequest.JobName,
		Target: target,
	}

	switch action {
	case start:
		s.cacheManager.Delete(key)
		return nil
	case stop:
		recoveryFunc := func() (*v1.StatusResponse, error) {
			return serviceClient.Start(context.Background(), serviceRequest)
		}
		return s.cacheManager.Register(key, recoveryFunc)
	default:
		return errors.New(fmt.Sprintf("Action %s not supported for cache operation", action))
	}
}
