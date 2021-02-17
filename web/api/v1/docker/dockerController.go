package docker

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
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

func newDockerRequest(details *RequestPayload) *v1.DockerRequest {
	return &v1.DockerRequest{
		JobName:              details.Job,
		Name:                 details.Container,
		XXX_NoUnkeyedLiteral: struct{}{},
		XXX_unrecognized:     nil,
		XXX_sizecache:        0,
	}
}

func newDockerRequestNoTarget(details *RequestPayloadNoTarget) *v1.DockerRequest {
	return &v1.DockerRequest{
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

	resp := response.NewDefaultResponse()

	requestPayload := &RequestPayload{}
	err := json.NewDecoder(r.Body).Decode(&requestPayload)
	if err != nil {
		resp.BadRequest("Could not decode request body", d.logger)
		resp.SetInWriter(w, d.logger)
		return
	}

	action, err := toActionEnum(r.FormValue("action"))
	if err != nil {
		resp.BadRequest(err.Error(), d.logger)
		resp.SetInWriter(w, d.logger)
		return
	}

	_ = level.Info(d.logger).Log("msg", fmt.Sprintf("%s container with name {%s}", action, requestPayload.Container))

	job, ok := d.jobs[requestPayload.Job]
	if !ok {
		resp.BadRequest(fmt.Sprintf("Could not find job {%s}", requestPayload.Job), d.logger)
		resp.SetInWriter(w, d.logger)
		return
	}

	dockerExists := false
	for _, target := range job.Target {
		if job.ComponentName == requestPayload.Container && target == requestPayload.Target {
			dockerRequest := newDockerRequest(requestPayload)
			d.performAction(action, dockerRequest, requestPayload.Target, resp)
			dockerExists = true
		}
	}

	if !dockerExists {
		resp.BadRequest(fmt.Sprintf("Container {%s} is not registered for target {%s}", requestPayload.Container, requestPayload.Target), d.logger)
	}

	resp.SetInWriter(w, d.logger)
}

func (d *DController) performAction(
	action action,
	dockerRequest *v1.DockerRequest,
	target string,
	resp *response.Payload,
) {
	var statusResponse *v1.StatusResponse
	var err error

	dockerClient, err := d.connectionPool[target].connection.GetDockerClient(target)
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("Can not get docker connection from target {%s}", target))
		resp.InternalServerError(err.Error(), d.logger)
		return
	}

	switch action {
	case start:
		statusResponse, err = dockerClient.Start(context.Background(), dockerRequest)
	case stop:
		statusResponse, err = dockerClient.Stop(context.Background(), dockerRequest)
	default:
		resp.BadRequest(fmt.Sprintf("Action {%s} not allowed", action), d.logger)
		return
	}

	switch {
	case err != nil:
		err = errors.Wrap(err, fmt.Sprintf("Error response from target {%s}", target))
		resp.InternalServerError(err.Error(), d.logger)
	case statusResponse.Status != v1.StatusResponse_SUCCESS:
		resp.InternalServerError(fmt.Sprintf("Failure response from target {%s}", target), d.logger)
	default:
		resp.Message = fmt.Sprintf("Response from target {%s}, {%s}, {%s}", target, statusResponse.Message, statusResponse.Status)
		_ = level.Info(d.logger).Log("msg", resp.Message)
		if err = d.updateCache(dockerRequest, dockerClient, target, action); err != nil {
			_ = level.Error(d.logger).Log("msg", fmt.Sprintf("Could not update cache for operation %s", action), "err", err)
		}
	}
}

func (d *DController) randomDocker(w http.ResponseWriter, r *http.Request, do string) {
	resp := response.NewDefaultResponse()

	if do != "random" {
		resp.BadRequest(fmt.Sprintf("Do query parameter {%s} not allowed", do), d.logger)
		resp.SetInWriter(w, d.logger)
		return
	}

	requestPayloadNoTarget := &RequestPayloadNoTarget{}
	err := json.NewDecoder(r.Body).Decode(&requestPayloadNoTarget)
	if err != nil {
		resp.BadRequest("Could not decode request body", d.logger)
		resp.SetInWriter(w, d.logger)
		return
	}

	action, err := toActionEnum(r.FormValue("action"))
	if err != nil {
		resp.BadRequest(err.Error(), d.logger)
		resp.SetInWriter(w, d.logger)
		return
	}

	_ = level.Info(d.logger).Log("msg", fmt.Sprintf("%s any container", action))

	if job, ok := d.jobs[requestPayloadNoTarget.Job]; ok {
		target, err := validateAndGetTarget(requestPayloadNoTarget, *job)
		if err != nil {
			resp.BadRequest(err.Error(), d.logger)
			resp.SetInWriter(w, d.logger)
			return
		}

		dockerRequest := newDockerRequestNoTarget(requestPayloadNoTarget)
		d.performAction(action, dockerRequest, target, resp)
	} else {
		resp.BadRequest(fmt.Sprintf("Could not find job {%s}", requestPayloadNoTarget.Job), d.logger)
	}

	resp.SetInWriter(w, d.logger)
}

func validateAndGetTarget(requestPayloadNoTarget *RequestPayloadNoTarget, job config.Job) (string, error) {
	target, err := getRandomTarget(job.Target)
	if err != nil {
		reformattedErr := errors.New(fmt.Sprintf("Could not get random target for job {%s}. err: %s", requestPayloadNoTarget.Job, err.Error()))
		return "", reformattedErr
	}

	if job.ComponentName != requestPayloadNoTarget.Container {
		reformattedErr := errors.New(fmt.Sprintf("Could not find container name {%s}", requestPayloadNoTarget.Container))
		return "", reformattedErr
	}

	return target, nil
}

func getRandomTarget(targets []string) (string, error) {
	if len(targets) > 0 {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(targets))))
		if err != nil {
			return "", err
		}

		return targets[num.Int64()], nil
	}
	return "", errors.New("No targets available")
}

func (d *DController) updateCache(dockerRequest *v1.DockerRequest, dockerClient v1.DockerClient, target string, action action) error {
	key := &cache.Key{
		Job:    dockerRequest.JobName,
		Target: target,
	}

	switch action {
	case start:
		d.cacheManager.Delete(key)
		return nil
	case stop:
		recoveryFunc := func() (*v1.StatusResponse, error) {
			return dockerClient.Start(context.Background(), dockerRequest)
		}
		return d.cacheManager.Register(key, recoveryFunc)
	default:
		return errors.New(fmt.Sprintf("Action %s not supported for cache operation", action))
	}
}
