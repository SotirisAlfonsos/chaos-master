package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/SotirisAlfonsos/chaos-master/pkg/chaoslogger"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/SotirisAlfonsos/chaos-master/pkg/cache"
	"github.com/SotirisAlfonsos/chaos-master/pkg/network"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/response"
	"github.com/SotirisAlfonsos/gocache"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
)

type SController struct {
	jobs           map[string]*config.Job
	connectionPool map[string]*sConnection
	cache          *gocache.Cache
	loggers        chaoslogger.Loggers
}

type sConnection struct {
	connection network.Connection
}

func NewServiceController(
	jobs map[string]*config.Job,
	connections *network.Connections,
	cache *gocache.Cache,
	loggers chaoslogger.Loggers,
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
		cache:          cache,
		loggers:        loggers,
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
		Name: details.ServiceName,
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
// @Success 200 {object} response.Payload
// @Failure 400 {object} response.Payload
// @Failure 500 {object} response.Payload
// @Router /service [post]
func (s *SController) ServiceAction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	requestPayload := &RequestPayload{}
	err := json.NewDecoder(r.Body).Decode(&requestPayload)
	if err != nil {
		response.BadRequest(w, "Could not decode request body", s.loggers)
		return
	}

	action, err := toActionEnum(r.FormValue("action"))
	if err != nil {
		response.BadRequest(w, err.Error(), s.loggers)
		return
	}

	err = checkIfTargetExists(s.jobs, requestPayload)
	if err != nil {
		response.BadRequest(w, err.Error(), s.loggers)
		return
	}

	_ = level.Info(s.loggers.OutLogger).Log("msg", fmt.Sprintf("%s service with name {%s}", action, requestPayload.ServiceName))

	message, err := s.performAction(ctx, action, requestPayload)
	if err != nil {
		response.InternalServerError(w, err.Error(), s.loggers)
		return
	}

	_ = level.Info(s.loggers.OutLogger).Log("msg", message)

	response.OkResponse(w, message, s.loggers)
}

func checkIfTargetExists(jobMap map[string]*config.Job, requestPayload *RequestPayload) error {
	job, ok := jobMap[requestPayload.Job]
	if !ok {
		return errors.New(fmt.Sprintf("Could not find job {%s}", requestPayload.Job))
	}

	ok = checkTargetIfExistsWithService(job, requestPayload.Target, requestPayload.ServiceName)
	if !ok {
		return errors.New(fmt.Sprintf("Service {%s} is not registered for target {%s}", requestPayload.ServiceName, requestPayload.Target))
	}

	return nil
}

func checkTargetIfExistsWithService(job *config.Job, requestTarget string, requestService string) bool {
	for _, target := range job.Target {
		if job.ComponentName == requestService && target == requestTarget {
			return true
		}
	}

	return false
}

func (s *SController) performAction(
	ctx context.Context,
	action action,
	request *RequestPayload,
) (string, error) {
	var statusResponse *v1.StatusResponse
	var err error

	serviceClient, err := s.connectionPool[request.Target].connection.GetServiceClient()
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("Can not get service connection from target {%s}", request.Target))
	}

	switch action {
	case start:
		statusResponse, err = serviceClient.Start(ctx, newServiceRequest(request))
	case stop:
		statusResponse, err = serviceClient.Stop(ctx, newServiceRequest(request))
	}

	switch {
	case err != nil:
		return "", errors.Wrap(err, fmt.Sprintf("Error response from target {%s}", request.Target))
	case statusResponse.Status != v1.StatusResponse_SUCCESS:
		return "", errors.New(fmt.Sprintf("Failure response from target {%s}", request.Target))
	default:
		if err = s.updateCache(serviceClient, request, action); err != nil {
			_ = level.Error(s.loggers.ErrLogger).Log("msg", fmt.Sprintf("Could not update cache for operation %s", action), "err", err)
		}
	}

	return fmt.Sprintf("Response from target {%s}, {%s}, {%s}", request.Target, statusResponse.Message, statusResponse.Status), nil
}

func (s *SController) updateCache(serviceClient v1.ServiceClient, request *RequestPayload, action action) error {
	key := cache.Key{
		Job:    request.Job,
		Target: request.Target,
	}

	switch action {
	case start:
		s.cache.Delete(key)
		return nil
	case stop:
		recoveryFunc := func() (*v1.StatusResponse, error) {
			return serviceClient.Start(context.Background(), newServiceRequest(request))
		}
		s.cache.Set(key, recoveryFunc)
		return nil
	default:
		return errors.New(fmt.Sprintf("Action %s not supported for cache operation", action))
	}
}
