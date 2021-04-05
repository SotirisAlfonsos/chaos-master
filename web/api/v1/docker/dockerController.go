package docker

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"

	"github.com/SotirisAlfonsos/chaos-master/pkg/chaoslogger"
	"github.com/SotirisAlfonsos/gocache"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/SotirisAlfonsos/chaos-master/pkg/cache"
	"github.com/SotirisAlfonsos/chaos-master/pkg/network"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/response"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
)

type DController struct {
	jobs           map[string]*config.Job
	connectionPool map[string]*dConnection
	cache          *gocache.Cache
	loggers        chaoslogger.Loggers
}

type dConnection struct {
	connection network.Connection
}

type action int

const (
	kill action = iota
	recoverContainer
	notImplemented
)

func (a action) String() string {
	return [...]string{"kill", "recover"}[a]
}

func toActionEnum(value string) (action, error) {
	switch value {
	case recoverContainer.String():
		return recoverContainer, nil
	case kill.String():
		return kill, nil
	}
	return notImplemented, errors.New(fmt.Sprintf("The action {%s} is not supported", value))
}

func NewDockerController(
	jobs map[string]*config.Job,
	connections *network.Connections,
	cache *gocache.Cache,
	loggers chaoslogger.Loggers,
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
		cache:          cache,
		loggers:        loggers,
	}
}

type RequestPayload struct {
	Job       string `json:"job"`
	Container string `json:"containerName"`
	Target    string `json:"target"`
}

func newDockerRequest(details *RequestPayload) *v1.DockerRequest {
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
// @Param do query string false "Specify to perform action for container on random target" Enums(random)
// @Param action query string true "Specify to perform a recover or a kill on the specified container" Enums(kill, recover)
// @Param requestPayload body RequestPayload true "Specify the job name, container name and target"
// @Success 200 {object} response.Payload
// @Failure 400 {string} http.Error
// @Failure 500 {string} http.Error
// @Router /docker [post]
func (d *DController) DockerAction(w http.ResponseWriter, r *http.Request) {
	do := r.FormValue("do")
	if do != "" {
		d.randomDocker(w, r, do)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	requestPayload := &RequestPayload{}
	err := json.NewDecoder(r.Body).Decode(&requestPayload)
	if err != nil {
		response.BadRequest(w, "Could not decode request body", d.loggers)
		return
	}

	action, err := toActionEnum(r.FormValue("action"))
	if err != nil {
		response.BadRequest(w, err.Error(), d.loggers)
		return
	}

	_ = level.Info(d.loggers.OutLogger).Log("msg", fmt.Sprintf("%s container with name {%s}", action, requestPayload.Container))

	err = checkIfTargetExists(d.jobs, requestPayload)
	if err != nil {
		response.BadRequest(w, err.Error(), d.loggers)
		return
	}

	message, err := d.performAction(ctx, action, requestPayload)
	if err != nil {
		response.InternalServerError(w, err.Error(), d.loggers)
		return
	}

	_ = level.Info(d.loggers.OutLogger).Log("msg", message)

	response.OkResponse(w, message, d.loggers)
}

func (d *DController) randomDocker(w http.ResponseWriter, r *http.Request, do string) {
	if do != "random" {
		response.BadRequest(w, fmt.Sprintf("Do query parameter {%s} not allowed", do), d.loggers)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	requestPayload := &RequestPayload{}
	err := json.NewDecoder(r.Body).Decode(&requestPayload)
	if err != nil {
		response.BadRequest(w, "Could not decode request body", d.loggers)
		return
	}

	action, err := toActionEnum(r.FormValue("action"))
	if err != nil {
		response.BadRequest(w, err.Error(), d.loggers)
		return
	}

	_ = level.Info(d.loggers.OutLogger).Log("msg", fmt.Sprintf("%s any container", action))

	err = setRandomTargetIfExists(d.jobs, requestPayload)
	if err != nil {
		response.BadRequest(w, err.Error(), d.loggers)
		return
	}

	message, err := d.performAction(ctx, action, requestPayload)
	if err != nil {
		response.InternalServerError(w, err.Error(), d.loggers)
		return
	}

	_ = level.Info(d.loggers.OutLogger).Log("msg", message)

	response.OkResponse(w, message, d.loggers)
}

func checkIfTargetExists(jobMap map[string]*config.Job, requestPayload *RequestPayload) error {
	job, ok := jobMap[requestPayload.Job]
	if !ok {
		return errors.New(fmt.Sprintf("Could not find job {%s}", requestPayload.Job))
	}

	ok = checkTargetIfExistsWithContainer(job, requestPayload.Target, requestPayload.Container)
	if !ok {
		return errors.New(fmt.Sprintf("Container {%s} is not registered for target {%s}", requestPayload.Container, requestPayload.Target))
	}

	return nil
}

func checkTargetIfExistsWithContainer(job *config.Job, requestTarget string, requestContainer string) bool {
	for _, target := range job.Target {
		if job.ComponentName == requestContainer && target == requestTarget {
			return true
		}
	}

	return false
}

func setRandomTargetIfExists(jobMap map[string]*config.Job, requestPayload *RequestPayload) error {
	job, ok := jobMap[requestPayload.Job]
	if !ok {
		return errors.New(fmt.Sprintf("Could not find job {%s}", requestPayload.Job))
	}

	target, err := getRandomTarget(job.Target)
	if err != nil {
		reformattedErr := errors.New(fmt.Sprintf("Could not get random target for job {%s}. err: %s", requestPayload.Job, err.Error()))
		return reformattedErr
	}

	if job.ComponentName != requestPayload.Container {
		reformattedErr := errors.New(fmt.Sprintf("Could not find container name {%s}", requestPayload.Container))
		return reformattedErr
	}

	requestPayload.Target = target
	return nil
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
	request *RequestPayload,
) (string, error) {
	var statusResponse *v1.StatusResponse
	var err error
	connection := d.connectionPool[request.Target].connection

	dockerClient, err := connection.GetDockerClient()
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("Can not get docker connection from target {%s}", request.Target))
	}

	switch action {
	case recoverContainer:
		statusResponse, err = dockerClient.Recover(ctx, newDockerRequest(request))
	case kill:
		statusResponse, err = dockerClient.Kill(ctx, newDockerRequest(request))
	}

	switch {
	case err != nil:
		return "", errors.Wrap(err, fmt.Sprintf("Error response from target {%s}", request.Target))
	case statusResponse.Status != v1.StatusResponse_SUCCESS:
		return "", errors.New(fmt.Sprintf("Failure response from target {%s}", request.Target))
	default:
		if err = d.updateCache(connection, request, action); err != nil {
			_ = level.Error(d.loggers.ErrLogger).Log("msg", fmt.Sprintf("Could not update cache for operation %s", action), "err", err)
		}
	}

	return fmt.Sprintf("Response from target {%s}, {%s}, {%s}", request.Target, statusResponse.Message, statusResponse.Status), nil
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

func (d *DController) updateCache(connection network.Connection, request *RequestPayload, action action) error {
	key := cache.Key{
		Job:    request.Job,
		Target: request.Target,
	}

	switch action {
	case recoverContainer:
		d.cache.Delete(key)
		return nil
	case kill:
		recoveryFunc := func() (*v1.StatusResponse, error) {
			dockerClient, err := connection.GetDockerClient()
			if err != nil {
				return nil, errors.New(fmt.Sprintf("Could not recover container for job {%s} and target {%s}", request.Job, request.Target))
			}
			return dockerClient.Recover(context.Background(), &v1.DockerRequest{Name: request.Container})
		}
		d.cache.Set(key, recoveryFunc)
		return nil
	default:
		return errors.New(fmt.Sprintf("Action %s not supported for cache operation", action))
	}
}
