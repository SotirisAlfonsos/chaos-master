package cpu

import "C"
import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/SotirisAlfonsos/chaos-master/pkg/cache"
	"github.com/SotirisAlfonsos/chaos-master/pkg/chaoslogger"
	"github.com/SotirisAlfonsos/chaos-master/pkg/network"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/response"
	"github.com/SotirisAlfonsos/gocache"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
)

type CController struct {
	jobs           map[string]*config.Job
	connectionPool map[string]*cConnection
	cache          *gocache.Cache
	loggers        chaoslogger.Loggers
}

type cConnection struct {
	connection network.Connection
}

type action int

const (
	recoverFailure action = iota
	start
	notImplemented
)

func (a action) String() string {
	return [...]string{"recover", "start"}[a]
}

func toActionEnum(value string) (action, error) {
	switch value {
	case start.String():
		return start, nil
	case recoverFailure.String():
		return recoverFailure, nil
	}
	return notImplemented, errors.New(fmt.Sprintf("The action {%s} is not supported", value))
}

func NewCPUController(
	jobs map[string]*config.Job,
	connections *network.Connections,
	cache *gocache.Cache,
	loggers chaoslogger.Loggers,
) *CController {
	connPool := make(map[string]*cConnection)
	for target, connection := range connections.Pool {
		connPool[target] = &cConnection{
			connection: connection,
		}
	}
	return &CController{
		jobs:           jobs,
		connectionPool: connPool,
		cache:          cache,
		loggers:        loggers,
	}
}

type RequestPayload struct {
	Job        string `json:"job"`
	Percentage int32  `json:"percentage"`
	Target     string `json:"target"`
}

func newCPURequest(details *RequestPayload) *v1.CPURequest {
	return &v1.CPURequest{
		Percentage: details.Percentage,
	}
}

// CPUAction godoc
// @Summary Inject CPU failures
// @Description Perform CPU spike injection. Provide a percentage and the cpu usage will increase based on it
// @Tags Failure injections
// @Accept json
// @Produce json
// @Param action query string true "Specify to perform a start or a recover for the CPU injection" Enums(start, recover)
// @Param requestPayload body RequestPayload true "Specify the job name, percentage and target"
// @Success 200 {object} response.Payload
// @Failure 400 {string} http.Error
// @Failure 500 {string} http.Error
// @Router /cpu [post]
func (c *CController) CPUAction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	requestPayload := &RequestPayload{}
	err := json.NewDecoder(r.Body).Decode(&requestPayload)
	if err != nil {
		response.BadRequest(w, "Could not decode request body", c.loggers)
		return
	}

	action, err := toActionEnum(r.FormValue("action"))
	if err != nil {
		response.BadRequest(w, err.Error(), c.loggers)
		return
	}

	err = checkIfTargetExists(c.jobs, requestPayload)
	if err != nil {
		response.BadRequest(w, err.Error(), c.loggers)
		return
	}

	_ = level.Info(c.loggers.OutLogger).Log("msg", fmt.Sprintf("%s CPU injection on targets {%s}", action, requestPayload.Target))

	message, err := c.performAction(ctx, action, requestPayload)
	if err != nil {
		response.InternalServerError(w, err.Error(), c.loggers)
		return
	}

	_ = level.Info(c.loggers.OutLogger).Log("msg", message)

	response.OkResponse(w, message, c.loggers)
}

func checkIfTargetExists(jobMap map[string]*config.Job, requestPayload *RequestPayload) error {
	job, ok := jobMap[requestPayload.Job]
	if !ok {
		return errors.New(fmt.Sprintf("Could not find job {%s}", requestPayload.Job))
	}

	ok = checkIfTargetExistsForJob(job, requestPayload.Target)
	if !ok {
		return errors.New(fmt.Sprintf("Target {%s} is not registered for job {%s}", requestPayload.Target, requestPayload.Job))
	}

	return nil
}

func checkIfTargetExistsForJob(job *config.Job, requestTarget string) bool {
	for _, target := range job.Target {
		if target == requestTarget {
			return true
		}
	}

	return false
}

func (c *CController) performAction(
	ctx context.Context,
	action action,
	request *RequestPayload,
) (string, error) {
	var statusResponse *v1.StatusResponse
	var err error
	connection := c.connectionPool[request.Target].connection

	cpuClient, err := connection.GetCPUClient()
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("Can not get cpu connection for target {%s}", request.Target))
	}

	switch action {
	case start:
		statusResponse, err = cpuClient.Start(ctx, newCPURequest(request))
	case recoverFailure:
		statusResponse, err = cpuClient.Recover(ctx, newCPURequest(request))
	}

	switch {
	case err != nil:
		return "", errors.Wrap(err, fmt.Sprintf("Error response from target {%s}", request.Target))
	case statusResponse.Status != v1.StatusResponse_SUCCESS:
		return "", errors.New(fmt.Sprintf("Failure response from target {%s}", request.Target))
	default:
		if err = c.updateCache(connection, request, action); err != nil {
			_ = level.Error(c.loggers.ErrLogger).Log("msg", fmt.Sprintf("Could not update cache for operation cpu injection %s", action), "err", err)
		}
	}

	return fmt.Sprintf("Response from target {%s}, {%s}, {%s}", request.Target, statusResponse.Message, statusResponse.Status), nil
}

func (c *CController) updateCache(connection network.Connection, request *RequestPayload, action action) error {
	key := cache.Key{
		Job:    request.Job,
		Target: request.Target,
	}

	switch action {
	case start:
		recoveryFunc := func() (*v1.StatusResponse, error) {
			cpuClient, err := connection.GetCPUClient()
			if err != nil {
				return nil, errors.New(fmt.Sprintf("Could not recover cpu failure for job {%s} and target {%s}", request.Job, request.Target))
			}
			return cpuClient.Recover(context.Background(), &v1.CPURequest{})
		}
		c.cache.Set(key, recoveryFunc)
		return nil
	case recoverFailure:
		c.cache.Delete(key)
		return nil
	default:
		return errors.New(fmt.Sprintf("Action %s not supported for cache operation", action))
	}
}
