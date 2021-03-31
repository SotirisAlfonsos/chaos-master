package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/SotirisAlfonsos/chaos-master/pkg/chaoslogger"
	"github.com/SotirisAlfonsos/chaos-master/pkg/network"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/response"
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
	loggers        chaoslogger.Loggers
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
		loggers:        loggers,
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

	requestPayload := &RequestPayload{}
	err := json.NewDecoder(r.Body).Decode(&requestPayload)
	if err != nil {
		response.BadRequest(w, "Could not decode request body", sc.loggers)
		return
	}

	action, err := toActionEnum(r.FormValue("action"))
	if err != nil {
		response.BadRequest(w, err.Error(), sc.loggers)
		return
	}

	err = checkIfTargetExists(sc.jobs, requestPayload)
	if err != nil {
		response.BadRequest(w, err.Error(), sc.loggers)
		return
	}

	_ = level.Info(sc.loggers.OutLogger).Log("msg", fmt.Sprintf("%s target with name {%s}", action, requestPayload.Target))

	message, err := sc.performAction(ctx, action, requestPayload)
	if err != nil {
		response.InternalServerError(w, err.Error(), sc.loggers)
		return
	}

	_ = level.Info(sc.loggers.OutLogger).Log("msg", message)

	response.OkResponse(w, message, sc.loggers)
}

func (sc *SController) performAction(
	ctx context.Context,
	action action,
	request *RequestPayload,
) (string, error) {
	var statusResponse *v1.StatusResponse
	var err error

	serverClient, err := sc.connectionPool[request.Target].connection.GetServerClient()
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("Can not get server connection from target {%s}", request.Target))
	}

	if action == stop {
		statusResponse, err = serverClient.Stop(ctx, newServerRequest())
	}

	switch {
	case err != nil:
		return "", errors.Wrap(err, fmt.Sprintf("Error response from target {%s}", request.Target))
	case statusResponse.Status != v1.StatusResponse_SUCCESS:
		return "", errors.New(fmt.Sprintf("Failure response from target {%s}", request.Target))
	}

	return fmt.Sprintf("Response from target {%s}, {%s}, {%s}", request.Target, statusResponse.Message, statusResponse.Status), nil
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
