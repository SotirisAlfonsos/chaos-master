package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"

	"github.com/SotirisAlfonsos/chaos-bot/proto"
	"github.com/SotirisAlfonsos/chaos-master/cache"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/SotirisAlfonsos/chaos-master/network"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
)

type DController struct {
	jobs           map[string]*config.Job
	connectionPool map[string]*dConnection
	cacheManager   *cache.Manager
	logger         log.Logger
}

type dConnection struct {
	connection network.Connection
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

func NewDockerController(
	jobs map[string]*config.Job,
	connections *network.Connections,
	cache *cache.Manager,
	logger log.Logger,
) *DController {
	connPool := make(map[string]*dConnection)
	for target, connection := range connections.Pool {
		connPool[target] = &dConnection{
			connection: connection,
		}
	}
	return &DController{
		jobs:           jobs,
		connectionPool: connPool,
		cacheManager:   cache,
		logger:         logger,
	}
}

type RequestPayload struct {
	Job       string `json:"job"`
	Container string `json:"containerName"`
	Target    string `json:"target"`
}

type RequestPayloadNoTarget struct {
	Job       string `json:"job"`
	Container string `json:"containerName"`
}

type ResponsePayload struct {
	Message string `json:"message"`
	Error   string `json:"error"`
	Status  int    `json:"status"`
}

func newDockerRequest(details *RequestPayload) *proto.DockerRequest {
	return &proto.DockerRequest{
		JobName:              details.Job,
		Name:                 details.Container,
		XXX_NoUnkeyedLiteral: struct{}{},
		XXX_unrecognized:     nil,
		XXX_sizecache:        0,
	}
}

func newDockerRequestNoTarget(details *RequestPayloadNoTarget) *proto.DockerRequest {
	return &proto.DockerRequest{
		JobName:              details.Job,
		Name:                 details.Container,
		XXX_NoUnkeyedLiteral: struct{}{},
		XXX_unrecognized:     nil,
		XXX_sizecache:        0,
	}
}

// CalcExample godoc
// @Summary Inject docker failures
// @Description Perform start or stop action on a container. If random is specified you do not have to provide a target
// @Tags Failure injections
// @Accept json
// @Produce json
// @Param do query string false "Specify to perform action for container on random target"
// @Param action query string true "Specify to perform a start or a stop on the specified container"
// @Param requestPayload body RequestPayload true "Specify the job name, container name and target"
// @Success 200 {object} ResponsePayload
// @Failure 400 {object} ResponsePayload
// @Failure 500 {object} ResponsePayload
// @Router /docker [post]
func (d *DController) DockerAction(w http.ResponseWriter, r *http.Request) {
	do := r.FormValue("do")
	if do != "" {
		d.randomDocker(w, r, do)
		return
	}

	response := newDefaultResponse()

	requestPayload := &RequestPayload{}
	err := json.NewDecoder(r.Body).Decode(&requestPayload)
	if err != nil {
		response.badRequest("Could not decode request body", d.logger)
		setResponseInWriter(w, response, d.logger)
		return
	}

	action, err := toActionEnum(r.FormValue("action"))
	if err != nil {
		response.badRequest(err.Error(), d.logger)
		setResponseInWriter(w, response, d.logger)
		return
	}

	dockerExists := false

	_ = level.Info(d.logger).Log("msg", fmt.Sprintf("%s container with name {%s}", action, requestPayload.Container))

	if job, ok := d.jobs[requestPayload.Job]; ok {
		for _, target := range job.Target {
			if job.ComponentName == requestPayload.Container && target == requestPayload.Target {
				dockerRequest := newDockerRequest(requestPayload)
				d.performAction(action, d.connectionPool[target], dockerRequest, requestPayload.Target, response)
				dockerExists = true
			}
		}

		if !dockerExists {
			response.badRequest(fmt.Sprintf("Container {%s} is not registered for target {%s}", requestPayload.Container, requestPayload.Target), d.logger)
		}
	} else {
		response.badRequest(fmt.Sprintf("Could not find job {%s}", requestPayload.Job), d.logger)
	}

	setResponseInWriter(w, response, d.logger)
}

func (d *DController) performAction(
	action action,
	dConnection *dConnection,
	dockerRequest *proto.DockerRequest,
	target string,
	response *ResponsePayload,
) {
	var statusResponse *proto.StatusResponse
	var err error

	dockerClient, err := dConnection.connection.GetDockerClient(target)
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("Can not get docker connection from target {%s}", target))
		response.internalServerError(err.Error(), d.logger)
		return
	}

	switch action {
	case start:
		statusResponse, err = dockerClient.Start(context.Background(), dockerRequest)
	case stop:
		statusResponse, err = dockerClient.Stop(context.Background(), dockerRequest)
	default:
		response.badRequest(fmt.Sprintf("Action {%s} not allowed", action), d.logger)
		return
	}

	switch {
	case err != nil:
		err = errors.Wrap(err, fmt.Sprintf("Error response from target {%s}", target))
		response.internalServerError(err.Error(), d.logger)
	case statusResponse.Status != proto.StatusResponse_SUCCESS:
		response.internalServerError(fmt.Sprintf("Failure response from target {%s}", target), d.logger)
	default:
		response.Message = fmt.Sprintf("Response from target {%s}, {%s}, {%s}", target, statusResponse.Message, statusResponse.Status)
		_ = level.Info(d.logger).Log("msg", response.Message)
		if err = d.updateCache(dockerRequest, dockerClient, target, action); err != nil {
			_ = level.Error(d.logger).Log("msg", fmt.Sprintf("Could not update cache for operation %s", action), "err", err)
		}
	}
}

func (d *DController) randomDocker(w http.ResponseWriter, r *http.Request, do string) {
	response := newDefaultResponse()

	if do != "random" {
		response := &ResponsePayload{}
		response.badRequest(fmt.Sprintf("Do query parameter {%s} not allowed", do), d.logger)
		setResponseInWriter(w, response, d.logger)
		return
	}

	requestPayloadNoTarget := &RequestPayloadNoTarget{}
	err := json.NewDecoder(r.Body).Decode(&requestPayloadNoTarget)
	if err != nil {
		response.badRequest("Could not decode request body", d.logger)
		setResponseInWriter(w, response, d.logger)
		return
	}

	action, err := toActionEnum(r.FormValue("action"))
	if err != nil {
		response.badRequest(err.Error(), d.logger)
		setResponseInWriter(w, response, d.logger)
		return
	}

	_ = level.Info(d.logger).Log("msg", fmt.Sprintf("%s any container", action))

	if job, ok := d.jobs[requestPayloadNoTarget.Job]; ok {
		target, err := getRandomTarget(job.Target)
		if err != nil {
			response.badRequest(fmt.Sprintf("Could not get random target for job {%s}. err: %s", requestPayloadNoTarget.Job, err.Error()), d.logger)
			setResponseInWriter(w, response, d.logger)
			return
		}

		if job.ComponentName == requestPayloadNoTarget.Container {
			dockerRequest := newDockerRequestNoTarget(requestPayloadNoTarget)
			d.performAction(action, d.connectionPool[target], dockerRequest, target, response)
		} else {
			response.badRequest(fmt.Sprintf("Could not find container name {%s}", requestPayloadNoTarget.Container), d.logger)
		}
	} else {
		response.badRequest(fmt.Sprintf("Could not find job {%s}", requestPayloadNoTarget.Job), d.logger)
	}

	setResponseInWriter(w, response, d.logger)
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

func getRandomTarget(targets []string) (string, error) {
	if len(targets) > 0 {
		return targets[rand.Intn(len(targets))], nil
	}
	return "", errors.New("No targets available")
}

func (d *DController) updateCache(dockerRequest *proto.DockerRequest, dockerClient proto.DockerClient, target string, action action) error {
	uniqueName := fmt.Sprintf("%s,%s,%s", dockerRequest.JobName, dockerRequest.Name, target)
	switch action {
	case start:
		d.cacheManager.Delete(uniqueName)
		return nil
	case stop:
		recoveryFunc := func() (*proto.StatusResponse, error) {
			return dockerClient.Start(context.Background(), dockerRequest)
		}
		return d.cacheManager.Register(uniqueName, recoveryFunc)
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

func (r *ResponsePayload) badRequest(message string, logger log.Logger) {
	r.Error = message
	r.Status = 400
	_ = level.Warn(logger).Log("msg", "Bad request", "warn", message)
}

func (r *ResponsePayload) internalServerError(message string, logger log.Logger) {
	r.Error = message
	r.Status = 500
	_ = level.Error(logger).Log("msg", "Internal server error", "err", message)
}
