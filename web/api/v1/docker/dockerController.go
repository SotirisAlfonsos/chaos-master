package docker

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/cache"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/SotirisAlfonsos/chaos-master/network"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/response"
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
		Name: details.Container,
	}
}

func newDockerRequestNoTarget(details *RequestPayloadNoTarget) *v1.DockerRequest {
	return &v1.DockerRequest{
		Name: details.Container,
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
// @Success 200 {object} response.Payload
// @Failure 400 {object} response.Payload
// @Failure 500 {object} response.Payload
// @Router /docker [post]
func (d *DController) DockerAction(w http.ResponseWriter, r *http.Request) {
	do := r.FormValue("do")
	if do != "" {
		d.randomDocker(w, r, do)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	target, err := getTargetIfExists(d.jobs, requestPayload)
	if err != nil {
		resp.BadRequest(err.Error(), d.logger)
		resp.SetInWriter(w, d.logger)
		return
	}

	serverRequest := newDockerRequest(requestPayload)
	d.performAction(ctx, action, serverRequest, target, requestPayload.Job, resp)

	resp.SetInWriter(w, d.logger)
}

func (d *DController) randomDocker(w http.ResponseWriter, r *http.Request, do string) {
	resp := response.NewDefaultResponse()

	if do != "random" {
		resp.BadRequest(fmt.Sprintf("Do query parameter {%s} not allowed", do), d.logger)
		resp.SetInWriter(w, d.logger)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	target, err := getRandomTargetIfExists(d.jobs, requestPayloadNoTarget)
	if err != nil {
		resp.BadRequest(err.Error(), d.logger)
		resp.SetInWriter(w, d.logger)
		return
	}

	serverRequest := newDockerRequestNoTarget(requestPayloadNoTarget)
	d.performAction(ctx, action, serverRequest, target, requestPayloadNoTarget.Job, resp)

	resp.SetInWriter(w, d.logger)
}

func getTargetIfExists(jobMap map[string]*config.Job, requestPayload *RequestPayload) (string, error) {
	job, ok := jobMap[requestPayload.Job]
	if !ok {
		return "", errors.New(fmt.Sprintf("Could not find job {%s}", requestPayload.Job))
	}

	target, ok := getTargetIfExistsWithContainer(job, requestPayload.Target, requestPayload.Container)
	if !ok {
		return "", errors.New(fmt.Sprintf("Container {%s} is not registered for target {%s}", requestPayload.Container, requestPayload.Target))
	}

	return target, nil
}

func getTargetIfExistsWithContainer(job *config.Job, requestTarget string, requestContainer string) (string, bool) {
	for _, target := range job.Target {
		if job.ComponentName == requestContainer && target == requestTarget {
			return target, true
		}
	}

	return "", false
}

func getRandomTargetIfExists(jobMap map[string]*config.Job, requestPayloadNoTarget *RequestPayloadNoTarget) (string, error) {
	job, ok := jobMap[requestPayloadNoTarget.Job]
	if !ok {
		return "", errors.New(fmt.Sprintf("Could not find job {%s}", requestPayloadNoTarget.Job))
	}

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

func (d *DController) performAction(
	ctx context.Context,
	action action,
	dockerRequest *v1.DockerRequest,
	target string,
	jobName string,
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
		statusResponse, err = dockerClient.Start(ctx, dockerRequest)
	case stop:
		statusResponse, err = dockerClient.Stop(ctx, dockerRequest)
	default:
		resp.BadRequest(fmt.Sprintf("Action {%s} not allowed", action), d.logger)
		return
	}

	message, err := d.handleBotResponse(statusResponse, err, target)
	if err != nil {
		resp.InternalServerError(err.Error(), d.logger)
		return
	}

	resp.Message = message
	_ = level.Info(d.logger).Log("msg", resp.Message)

	key := &cache.Key{
		Job:    jobName,
		Target: target,
	}

	if err = d.updateCache(dockerRequest, dockerClient, key, action); err != nil {
		_ = level.Error(d.logger).Log("msg", fmt.Sprintf("Could not update cache for operation %s", action), "err", err)
	}
}

func (d *DController) handleBotResponse(
	statusResponse *v1.StatusResponse,
	err error,
	target string,
) (string, error) {
	switch {
	case err != nil:
		return "", errors.Wrap(err, fmt.Sprintf("Error response from target {%s}", target))
	case statusResponse.Status != v1.StatusResponse_SUCCESS:
		return "", errors.New(fmt.Sprintf("Failure response from target {%s}", target))
	default:
		return fmt.Sprintf("Response from target {%s}, {%s}, {%s}", target, statusResponse.Message, statusResponse.Status), nil
	}
}

func (d *DController) updateCache(dockerRequest *v1.DockerRequest, dockerClient v1.DockerClient, key *cache.Key, action action) error {
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
