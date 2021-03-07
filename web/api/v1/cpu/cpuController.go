package cpu

import "C"
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

type CController struct {
	jobs           map[string]*config.Job
	connectionPool map[string]*cConnection
	cacheManager   *cache.Manager
	logger         log.Logger
}

type cConnection struct {
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

func NewCPUController(
	jobs map[string]*config.Job,
	connections *network.Connections,
	cache *cache.Manager,
	logger log.Logger,
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
		cacheManager:   cache,
		logger:         logger,
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

// CalcExample godoc
// @Summary Inject CPU failures
// @Description Perform CPU spike injection. Provide a percentage that is proportionate to your logical CPUs.
// E.g. With 4 logical CPUs if you want to inject 40% CPU failure we calculate that as ( numCpu * percentage / 100 )
// so we will block 1 CPU, effectively injecting failure 25%
// @Tags Failure injections
// @Accept json
// @Produce json
// @Param action query string true "Specify to perform a start or a stop for the CPU injection"
// @Param requestPayload body RequestPayload true "Specify the job name, percentage and target"
// @Success 200 {object} response.Payload
// @Failure 400 {object} response.Payload
// @Failure 500 {object} response.Payload
// @Router /cpu [post]
func (c *CController) CPUAction(w http.ResponseWriter, r *http.Request) {
	resp := response.NewDefaultResponse()

	requestPayload := &RequestPayload{}
	err := json.NewDecoder(r.Body).Decode(&requestPayload)
	if err != nil {
		resp.BadRequest("Could not decode request body", c.logger)
		resp.SetInWriter(w, c.logger)
		return
	}

	action, err := toActionEnum(r.FormValue("action"))
	if err != nil {
		resp.BadRequest(err.Error(), c.logger)
		resp.SetInWriter(w, c.logger)
		return
	}

	_ = level.Info(c.logger).Log("msg", fmt.Sprintf("%s CPU injection on targets {%s}", action, requestPayload.Target))

	job, ok := c.jobs[requestPayload.Job]
	if !ok {
		resp.BadRequest(fmt.Sprintf("Could not find job {%s}", requestPayload.Job), c.logger)
		resp.SetInWriter(w, c.logger)
		return
	}

	targetExists := false
	for _, target := range job.Target {
		if target == requestPayload.Target {
			c.performAction(action, c.connectionPool[target], requestPayload, resp)
			targetExists = true
		}
	}

	if !targetExists {
		resp.BadRequest(fmt.Sprintf("Target {%s} is not registered for job {%s}", requestPayload.Target, requestPayload.Job), c.logger)
	}

	resp.SetInWriter(w, c.logger)
}

func (c *CController) performAction(
	action action,
	cConnection *cConnection,
	request *RequestPayload,
	resp *response.Payload,
) {
	var statusResponse *v1.StatusResponse
	var err error

	cpuClient, err := cConnection.connection.GetCPUClient(request.Target)
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("Can not get cpu connection for target {%s}", request.Target))
		resp.InternalServerError(err.Error(), c.logger)
		return
	}

	switch action {
	case start:
		statusResponse, err = cpuClient.Start(context.Background(), newCPURequest(request))
	case stop:
		statusResponse, err = cpuClient.Stop(context.Background(), newCPURequest(request))
	default:
		resp.BadRequest(fmt.Sprintf("Action {%s} not allowed", action), c.logger)
		return
	}

	switch {
	case err != nil:
		err = errors.Wrap(err, fmt.Sprintf("Error response from target {%s}", request.Target))
		resp.InternalServerError(err.Error(), c.logger)
	case statusResponse.Status != v1.StatusResponse_SUCCESS:
		resp.InternalServerError(fmt.Sprintf("Failure response from target {%s}", request.Target), c.logger)
	default:
		resp.Message = fmt.Sprintf("Response from target {%s}, {%s}, {%s}", request.Target, statusResponse.Message, statusResponse.Status)
		_ = level.Info(c.logger).Log("msg", resp.Message)
		if err = c.updateCache(cpuClient, request, action); err != nil {
			_ = level.Error(c.logger).Log("msg", fmt.Sprintf("Could not update cache for operation %s", action), "err", err)
		}
	}
}

func (c *CController) updateCache(cpuClient v1.CPUClient, request *RequestPayload, action action) error {
	key := &cache.Key{
		Job:    request.Job,
		Target: request.Target,
	}

	switch action {
	case start:
		recoveryFunc := func() (*v1.StatusResponse, error) {
			return cpuClient.Stop(context.Background(), newCPURequest(request))
		}
		return c.cacheManager.Register(key, recoveryFunc)
	case stop:
		c.cacheManager.Delete(key)
		return nil
	default:
		return errors.New(fmt.Sprintf("Action %s not supported for cache operation", action))
	}
}
