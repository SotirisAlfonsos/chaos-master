package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/SotirisAlfonsos/chaos-master/network"
	"github.com/SotirisAlfonsos/chaos-slave/proto"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

type SController struct {
	jobs           map[string]*config.Job
	connectionPool map[string]*connection
	logger         log.Logger
}

type connection struct {
	grpcConn      *grpc.ClientConn
	serviceClient proto.ServiceClient
}

func NewServiceController(jobs map[string]*config.Job, connections *network.Connections, logger log.Logger) *SController {
	connPool := make(map[string]*connection)
	for target, grpcConn := range connections.Pool {
		connPool[target] = &connection{
			grpcConn:      grpcConn,
			serviceClient: proto.NewServiceClient(grpcConn),
		}
	}
	return &SController{
		jobs:           jobs,
		connectionPool: connPool,
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

type Response struct {
	Message string `json:"message"`
	Error   string `json:"error,"`
	Status  int    `json:"status"`
}

type Details struct {
	Job         string `json:"job"`
	ServiceName string `json:"serviceName"`
	Target      string `json:"target"`
}

func getDefaultResponse() *Response {
	return &Response{
		Message: "",
		Error:   "",
		Status:  200,
	}
}

func newServiceRequest(details *Details) *proto.ServiceRequest {
	return &proto.ServiceRequest{
		JobName:              details.Job,
		Name:                 details.ServiceName,
		XXX_NoUnkeyedLiteral: struct{}{},
		XXX_unrecognized:     nil,
		XXX_sizecache:        0,
	}
}

func (s *SController) ServiceAction(w http.ResponseWriter, r *http.Request) {
	response := getDefaultResponse()

	details := &Details{}
	err := json.NewDecoder(r.Body).Decode(&details)
	if err != nil {
		response.badRequest("Could not decode request body", s.logger)
		setResponseInWriter(w, response, s.logger)
		return
	}

	action, err := toActionEnum(r.FormValue("action"))
	if err != nil {
		response.badRequest(err.Error(), s.logger)
		setResponseInWriter(w, response, s.logger)
		return
	}

	serviceExists := false

	_ = level.Info(s.logger).Log("msg", fmt.Sprintf("%s service with name {%s}", action, details.ServiceName))

	if job, ok := s.jobs[details.Job]; ok {
		for _, target := range job.Target {
			if target == details.Target && job.ComponentName == details.ServiceName {
				serviceRequest := newServiceRequest(details)
				s.performAction(action, s.connectionPool[target].serviceClient, serviceRequest, details, response)
				serviceExists = true
			}
		}

		if !serviceExists {
			response.badRequest(fmt.Sprintf("Service {%s} does not exist on target {%s}", details.ServiceName, details.Target), s.logger)
		}
	} else {
		response.badRequest(fmt.Sprintf("Could not find job {%s}", details.Job), s.logger)
	}

	setResponseInWriter(w, response, s.logger)
}

func (s *SController) performAction(action action, serviceClient proto.ServiceClient,
	serviceRequest *proto.ServiceRequest, details *Details, response *Response) {
	var statusResponse *proto.StatusResponse
	var err error

	switch action {
	case start:
		statusResponse, err = serviceClient.Start(context.Background(), serviceRequest)
	case stop:
		statusResponse, err = serviceClient.Stop(context.Background(), serviceRequest)
	default:
		response.badRequest(fmt.Sprintf("Action {%s} not allowed", action), s.logger)
		return
	}

	switch {
	case err != nil:
		err = errors.Wrap(err, fmt.Sprintf("Error response from target {%s}", details.Target))
		response.internalServerError(err.Error(), s.logger)
	case statusResponse.Status != proto.StatusResponse_SUCCESS:
		response.internalServerError(fmt.Sprintf("Failure response from target {%s}", details.Target), s.logger)
	default:
		response.Message = fmt.Sprintf("Response from target {%s}, {%s}, {%s}", details.Target, statusResponse.Message, statusResponse.Status)
		_ = level.Info(s.logger).Log("msg", response.Message)
	}
}

func (r *Response) internalServerError(message string, logger log.Logger) {
	r.Error = message
	r.Status = 500
	_ = level.Error(logger).Log("msg", "Internal server error", "err", message)
}

func (r *Response) badRequest(message string, logger log.Logger) {
	r.Error = message
	r.Status = 400
	_ = level.Warn(logger).Log("msg", "Bad request", "warn", message)
}

func setResponseInWriter(w http.ResponseWriter, resp *Response, logger log.Logger) {
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
