package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

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

type ResponsePayload struct {
	Message string `json:"message"`
	Error   string `json:"error"`
	Status  int    `json:"status"`
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
	response := newDefaultResponse()

	requestPayload := &RequestPayload{}
	err := json.NewDecoder(r.Body).Decode(&requestPayload)
	if err != nil {
		response.badRequest("Could not decode request body", s.logger)
		setResponseInWriter(w, response, s.logger)
		return
	}

	action, err := toActionEnum(r.FormValue("action"))
	if err != nil {
		response.badRequest(err.Error(), s.logger)
		setResponseInWriter(w, response, s.logger)
		return
	}

	serviceExists := false

	_ = level.Info(s.logger).Log("msg", fmt.Sprintf("%s service with name {%s}", action, requestPayload.ServiceName))

	if job, ok := s.jobs[requestPayload.Job]; ok {
		for _, target := range job.Target {
			if target == requestPayload.Target && job.ComponentName == requestPayload.ServiceName {
				serviceRequest := newServiceRequest(requestPayload)
				s.performAction(action, s.connectionPool[target], serviceRequest, requestPayload, response)
				serviceExists = true
			}
		}

		if !serviceExists {
			response.badRequest(fmt.Sprintf("Service {%s} is not registered for target {%s}", requestPayload.ServiceName, requestPayload.Target), s.logger)
		}
	} else {
		response.badRequest(fmt.Sprintf("Could not find job {%s}", requestPayload.Job), s.logger)
	}

	setResponseInWriter(w, response, s.logger)
}

func (s *SController) performAction(
	action action,
	sConnection *sConnection,
	serviceRequest *v1.ServiceRequest,
	details *RequestPayload,
	response *ResponsePayload,
) {
	var statusResponse *v1.StatusResponse
	var err error

	serviceClient, err := sConnection.connection.GetServiceClient(details.Target)
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("Can not get service connection from target {%s}", details.Target))
		response.internalServerError(err.Error(), s.logger)
		return
	}

	switch action {
	case start:
		statusResponse, err = serviceClient.Start(context.Background(), serviceRequest)
	case stop:
		statusResponse, err = serviceClient.Stop(context.Background(), serviceRequest)
	default:
		response.badRequest(fmt.Sprintf("Action {%s} not allowed", action), s.logger)
		return
	}

	switch {
	case err != nil:
		err = errors.Wrap(err, fmt.Sprintf("Error response from target {%s}", details.Target))
		response.internalServerError(err.Error(), s.logger)
	case statusResponse.Status != v1.StatusResponse_SUCCESS:
		response.internalServerError(fmt.Sprintf("Failure response from target {%s}", details.Target), s.logger)
	default:
		response.Message = fmt.Sprintf("Response from target {%s}, {%s}, {%s}", details.Target, statusResponse.Message, statusResponse.Status)
		_ = level.Info(s.logger).Log("msg", response.Message)
		if err = s.updateCache(serviceRequest, serviceClient, details.Target, action); err != nil {
			_ = level.Error(s.logger).Log("msg", fmt.Sprintf("Could not update cache for operation %s", action), "err", err)
		}
	}
}

func (s *SController) updateCache(serviceRequest *v1.ServiceRequest, serviceClient v1.ServiceClient, target string, action action) error {
	uniqueName := fmt.Sprintf("%s,%s", serviceRequest.JobName, target)
	switch action {
	case start:
		s.cacheManager.Delete(uniqueName)
		return nil
	case stop:
		recoveryFunc := func() (*v1.StatusResponse, error) {
			return serviceClient.Start(context.Background(), serviceRequest)
		}
		return s.cacheManager.Register(uniqueName, recoveryFunc)
	default:
		return errors.New(fmt.Sprintf("Action %s not supported for cache operation", action))
	}
}

func newDefaultResponse() *ResponsePayload {
	return &ResponsePayload{
		Message: "",
		Error:   "",
		Status:  200,
	}
}

func (r *ResponsePayload) internalServerError(message string, logger log.Logger) {
	r.Error = message
	r.Status = 500
	_ = level.Error(logger).Log("msg", "Internal server error", "err", message)
}

func (r *ResponsePayload) badRequest(message string, logger log.Logger) {
	r.Error = message
	r.Status = 400
	_ = level.Warn(logger).Log("msg", "Bad request", "warn", message)
}

func setResponseInWriter(w http.ResponseWriter, resp *ResponsePayload, logger log.Logger) {
	reqBodyBytes := new(bytes.Buffer)
	err := json.NewEncoder(reqBodyBytes).Encode(resp)
	if err != nil {
		_ = level.Error(logger).Log("msg", "Error when trying to encode response to byte array", "err", err)
		w.WriteHeader(500)
		return
	}

	w.WriteHeader(resp.Status)
	_, err = w.Write(reqBodyBytes.Bytes())
	if err != nil {
		_ = level.Error(logger).Log("msg", "Error when trying to write response to byte array", "err", err)
		w.WriteHeader(500)
		return
	}
}
