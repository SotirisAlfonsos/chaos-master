package network

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/SotirisAlfonsos/gocache"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/SotirisAlfonsos/chaos-master/pkg/cache"
	"github.com/SotirisAlfonsos/chaos-master/pkg/chaoslogger"
	"github.com/SotirisAlfonsos/chaos-master/pkg/network"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/response"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
)

type NController struct {
	jobs           map[string]*config.Job
	connectionPool map[string]*nConnection
	cache          *gocache.Cache
	loggers        chaoslogger.Loggers
}

type nConnection struct {
	connection network.Connection
}

func NewNetworkController(
	jobs map[string]*config.Job,
	connections *network.Connections,
	cache *gocache.Cache,
	loggers chaoslogger.Loggers,
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
		cache:          cache,
		loggers:        loggers,
	}
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

// NetworkAction godoc
// @Summary Inject network failures
// @Description Start and stop network failures
// @Tags Failure injections
// @Accept json
// @Produce json
// @Param action query string true "Specify to perform a start or recover for a network failure injection" Enums(start, recover)
// @Param requestPayload body RequestPayload true "Specify the job name, device name, target and netem injection arguments"
// @Success 200 {object} response.Payload
// @Failure 400 {string} http.Error
// @Failure 500 {string} http.Error
// @Router /network [post]
func (n *NController) NetworkAction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	requestPayload := &RequestPayload{}
	err := json.NewDecoder(r.Body).Decode(&requestPayload)
	if err != nil {
		response.BadRequest(w, "Could not decode request body", n.loggers)
		return
	}

	action, err := toActionEnum(r.FormValue("action"))
	if err != nil {
		response.BadRequest(w, err.Error(), n.loggers)
		return
	}

	err = checkIfTargetExists(n.jobs, requestPayload)
	if err != nil {
		response.BadRequest(w, err.Error(), n.loggers)
		return
	}

	_ = level.Info(n.loggers.OutLogger).Log("msg",
		fmt.Sprintf("%s network injection for device {%s} on target {%s}", action, requestPayload.Device, requestPayload.Target))

	message, err := n.performAction(ctx, action, requestPayload)
	if err != nil {
		response.InternalServerError(w, err.Error(), n.loggers)
		return
	}

	_ = level.Info(n.loggers.OutLogger).Log("msg", message)

	response.OkResponse(w, message, n.loggers)
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

func (n *NController) performAction(
	ctx context.Context,
	action action,
	request *RequestPayload,
) (string, error) {
	var statusResponse *v1.StatusResponse
	var err error
	connection := n.connectionPool[request.Target].connection

	networkClient, err := connection.GetNetworkClient()
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("Can not get network connection from target {%s}", request.Target))
	}

	switch action {
	case start:
		statusResponse, err = networkClient.Start(ctx, newNetworkRequest(request))
	case recoverFailure:
		statusResponse, err = networkClient.Recover(ctx, newNetworkRequest(request))
	}

	switch {
	case err != nil:
		return "", errors.Wrap(err, fmt.Sprintf("Error response from target {%s}", request.Target))
	case statusResponse.Status != v1.StatusResponse_SUCCESS:
		return "", errors.New(fmt.Sprintf("Failure response from target {%s}", request.Target))
	default:
		if err = n.updateCache(connection, request, action); err != nil {
			_ = level.Error(n.loggers.ErrLogger).Log("msg", fmt.Sprintf("Could not update cache for operation %s", action), "err", err)
		}
	}

	return fmt.Sprintf("Response from target {%s}, {%s}, {%s}", request.Target, statusResponse.Message, statusResponse.Status), nil
}

func (n *NController) updateCache(connection network.Connection, request *RequestPayload, action action) error {
	key := cache.Key{
		Job:    request.Job,
		Target: request.Target,
	}

	switch action {
	case start:
		recoveryFunc := func() (*v1.StatusResponse, error) {
			networkClient, err := connection.GetNetworkClient()
			if err != nil {
				return nil, errors.New(fmt.Sprintf("Could not recover network failure for job {%s} and target {%s}", request.Job, request.Target))
			}
			return networkClient.Recover(context.Background(), &v1.NetworkRequest{Device: request.Device})
		}
		n.cache.Set(key, recoveryFunc)
		return nil
	case recoverFailure:
		n.cache.Delete(key)
		return nil
	default:
		return errors.New(fmt.Sprintf("Action %s not supported for cache operation", action))
	}
}
