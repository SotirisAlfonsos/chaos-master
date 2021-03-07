package network

import (
	"context"
	"encoding/json"
	"fmt"
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

type NController struct {
	jobs           map[string]*config.Job
	connectionPool map[string]*nConnection
	cacheManager   *cache.Manager
	logger         log.Logger
}

type nConnection struct {
	connection network.Connection
}

func NewNetworkController(
	jobs map[string]*config.Job,
	connections *network.Connections,
	cache *cache.Manager,
	logger log.Logger,
) *NController {
	connPool := make(map[string]*nConnection)
	for target, connection := range connections.Pool {
		connPool[target] = &nConnection{
			connection: connection,
		}
	}
	return &NController{
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
	Job           string  `json:"job"`
	Device        string  `json:"device"`
	Target        string  `json:"target"`
	Latency       uint32  `json:"latency"`
	DelayCorr     float32 `json:"delay correlation"`
	Limit         uint32  `json:"limit"`
	Loss          float32 `json:"loss"`
	LossCorr      float32 `json:"loss correlation"`
	Gap           uint32  `json:"gap"`
	Duplicate     float32 `json:"duplicate"`
	DuplicateCorr float32 `json:"duplicate correlation"`
	Jitter        uint32  `json:"jitter"`
	ReorderProb   float32 `json:"reorder probability"`
	ReorderCorr   float32 `json:"reorder correlation"`
	CorruptProb   float32 `json:"corrupt probability"`
	CorruptCorr   float32 `json:"corrupt correlation"`
}

func newNetworkRequest(details *RequestPayload) *v1.NetworkRequest {
	return &v1.NetworkRequest{
		Device:        details.Device,
		Latency:       details.Latency,
		DelayCorr:     details.DelayCorr,
		Limit:         details.Limit,
		Loss:          details.Loss,
		LossCorr:      details.LossCorr,
		Gap:           details.Gap,
		Duplicate:     details.Duplicate,
		DuplicateCorr: details.DuplicateCorr,
		Jitter:        details.Jitter,
		ReorderProb:   details.ReorderProb,
		ReorderCorr:   details.ReorderCorr,
		CorruptProb:   details.CorruptProb,
		CorruptCorr:   details.CorruptCorr,
	}
}

// CalcExample godoc
// @Summary Inject network failures
// @Description Start and stop network failures
// @Tags Failure injections
// @Accept json
// @Produce json
// @Param action query string true "Specify to perform a start or a stop for a network failure injection"
// @Param requestPayload body RequestPayload true "Specify the job name, device name, target and netem injection arguments"
// @Success 200 {object} response.Payload
// @Failure 400 {object} response.Payload
// @Failure 500 {object} response.Payload
// @Router /network [post]
func (n *NController) NetworkAction(w http.ResponseWriter, r *http.Request) {
	resp := response.NewDefaultResponse()

	requestPayload := &RequestPayload{}
	err := json.NewDecoder(r.Body).Decode(&requestPayload)
	if err != nil {
		resp.BadRequest("Could not decode request body", n.logger)
		resp.SetInWriter(w, n.logger)
		return
	}

	action, err := toActionEnum(r.FormValue("action"))
	if err != nil {
		resp.BadRequest(err.Error(), n.logger)
		resp.SetInWriter(w, n.logger)
		return
	}

	_ = level.Info(n.logger).Log("msg",
		fmt.Sprintf("%s network injection for device {%s} on target {%s}", action, requestPayload.Device, requestPayload.Target))

	target, err := getTargetIfExists(n.jobs, requestPayload)
	if err != nil {
		resp.BadRequest(err.Error(), n.logger)
		resp.SetInWriter(w, n.logger)
		return
	}

	networkRequest := newNetworkRequest(requestPayload)
	n.performAction(action, n.connectionPool[target], networkRequest, requestPayload, resp)

	resp.SetInWriter(w, n.logger)
}

func getTargetIfExists(jobMap map[string]*config.Job, requestPayload *RequestPayload) (string, error) {
	job, ok := jobMap[requestPayload.Job]
	if !ok {
		return "", errors.New(fmt.Sprintf("Could not find job {%s}", requestPayload.Job))
	}

	for _, target := range job.Target {
		if target == requestPayload.Target {
			return target, nil
		}
	}

	return "", errors.New(fmt.Sprintf("Target {%s} is not registered for job {%s}", requestPayload.Target, requestPayload.Job))
}

func (n *NController) performAction(
	action action,
	sConnection *nConnection,
	networkRequest *v1.NetworkRequest,
	requestPayload *RequestPayload,
	resp *response.Payload,
) {
	var statusResponse *v1.StatusResponse
	var err error

	networkClient, err := sConnection.connection.GetNetworkClient(requestPayload.Target)
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("Can not get network connection from target {%s}", requestPayload.Target))
		resp.InternalServerError(err.Error(), n.logger)
		return
	}

	switch action {
	case start:
		statusResponse, err = networkClient.Start(context.Background(), networkRequest)
	case stop:
		statusResponse, err = networkClient.Stop(context.Background(), networkRequest)
	default:
		resp.BadRequest(fmt.Sprintf("Action {%s} not allowed", action), n.logger)
		return
	}

	message, err := handleGRPCResponse(statusResponse, err, requestPayload.Target)
	if err != nil {
		resp.InternalServerError(err.Error(), n.logger)
		return
	}

	resp.Message = message
	_ = level.Info(n.logger).Log("msg", resp.Message)

	key := &cache.Key{
		Job:    requestPayload.Job,
		Target: requestPayload.Target,
	}

	if err = n.updateCache(networkRequest, networkClient, key, action); err != nil {
		_ = level.Error(n.logger).Log("msg", fmt.Sprintf("Could not update cache for operation %s", action), "err", err)
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

func (n *NController) updateCache(networkRequest *v1.NetworkRequest, networkClient v1.NetworkClient, key *cache.Key, action action) error {
	switch action {
	case start:
		recoveryFunc := func() (*v1.StatusResponse, error) {
			return networkClient.Stop(context.Background(), networkRequest)
		}
		return n.cacheManager.Register(key, recoveryFunc)
	case stop:
		n.cacheManager.Delete(key)
		return nil
	default:
		return errors.New(fmt.Sprintf("Action %s not supported for cache operation", action))
	}
}
