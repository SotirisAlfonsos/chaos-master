package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/SotirisAlfonsos/chaos-master/network"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/response"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
)

type RequestPayload struct {
	Job    string `json:"job"`
	Target string `json:"target"`
}

func newServerRequest() *v1.ServerRequest {
	return &v1.ServerRequest{}
}

type SController struct {
	logger         log.Logger
	jobs           jobs
	connectionPool map[string]*sConnection
}

type sConnection struct {
	connection network.Connection
}

type jobs map[string]*config.Job

func NewServerController(
	jobs map[string]*config.Job,
	connections *network.Connections,
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
		logger:         logger,
	}
}

// CalcExample godoc
// @Summary Inject Server failures
// @Description Perform Server fault injection. Supports action to stop the server specified. A shutdown will be executed after 1 minute.
// @Tags Failure injections
// @Accept json
// @Produce json
// @Param action query string true "Specify to perform a stop action on the server"
// @Param requestPayload body RequestPayload true "Specify the job name and target"
// @Success 200 {object} response.Payload
// @Failure 400 {object} response.Payload
// @Failure 500 {object} response.Payload
// @Router /server [post]
func (sc *SController) ServerAction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resp := response.NewDefaultResponse()

	requestPayload := &RequestPayload{}
	err := json.NewDecoder(r.Body).Decode(&requestPayload)
	if err != nil {
		resp.BadRequest("Could not decode request body", sc.logger)
		resp.SetInWriter(w, sc.logger)
		return
	}

	action, err := toActionEnum(r.FormValue("action"))
	if err != nil {
		resp.BadRequest(err.Error(), sc.logger)
		resp.SetInWriter(w, sc.logger)
		return
	}

	_ = level.Info(sc.logger).Log("msg", fmt.Sprintf("%s target with name {%s}", action, requestPayload.Target))

	job, ok := sc.jobs[requestPayload.Job]
	if !ok {
		resp.BadRequest(fmt.Sprintf("Could not find job {%s}", requestPayload.Job), sc.logger)
		resp.SetInWriter(w, sc.logger)
		return
	}

	target, ok := getTargetIfExists(job, requestPayload.Target)
	if !ok {
		resp.BadRequest(fmt.Sprintf("Target {%s} is not registered for job {%s}", requestPayload.Target, requestPayload.Job), sc.logger)
		resp.SetInWriter(w, sc.logger)
		return
	}

	serverRequest := newServerRequest()
	sc.performAction(ctx, action, serverRequest, target, resp)

	resp.SetInWriter(w, sc.logger)
}

func (sc *SController) performAction(
	ctx context.Context,
	action action,
	serverRequest *v1.ServerRequest,
	target string,
	resp *response.Payload,
) {
	var statusResponse *v1.StatusResponse
	var err error

	serverClient, err := sc.connectionPool[target].connection.GetServerClient(target)
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("Can not get server connection from target {%s}", target))
		resp.InternalServerError(err.Error(), sc.logger)
		return
	}

	switch action {
	case stop:
		statusResponse, err = serverClient.Stop(ctx, serverRequest)
	default:
		resp.BadRequest(fmt.Sprintf("Action {%s} not allowed", action), sc.logger)
		return
	}

	switch {
	case err != nil:
		err = errors.Wrap(err, fmt.Sprintf("Error response from target {%s}", target))
		resp.InternalServerError(err.Error(), sc.logger)
	case statusResponse.Status != v1.StatusResponse_SUCCESS:
		resp.InternalServerError(fmt.Sprintf("Failure response from target {%s}", target), sc.logger)
	default:
		resp.Message = fmt.Sprintf("Response from target {%s}, {%s}, {%s}", target, statusResponse.Message, statusResponse.Status)
		_ = level.Info(sc.logger).Log("msg", resp.Message)
	}
}

func getTargetIfExists(job *config.Job, requestTarget string) (string, bool) {
	for _, target := range job.Target {
		if target == requestTarget {
			return target, true
		}
	}

	return "", false
}

type action int

const (
	stop action = iota
	notImplemented
)

func (a action) String() string {
	return [...]string{"stop"}[a]
}

func toActionEnum(value string) (action, error) {
	switch value {
	case stop.String():
		return stop, nil
	}
	return notImplemented, errors.New(fmt.Sprintf("The action {%s} is not supported", value))
}
