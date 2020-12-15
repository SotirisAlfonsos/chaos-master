package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pkg/errors"

	"github.com/SotirisAlfonsos/chaos-master/network"
	"github.com/SotirisAlfonsos/chaos-slave/proto"
	"github.com/go-kit/kit/log/level"

	"github.com/go-kit/kit/log"
)

type SController struct {
	ServiceClients map[string][]network.ServiceClientConnection
	Logger         log.Logger
}

type Response struct {
	Message string `json:"message"`
	Error   string `json:"error,"`
	Status  int    `json:"status"`
}

type Details struct {
	Job     string `json:"job"`
	Service string `json:"service"`
	Target  string `json:"target"`
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
		Name:                 details.Service,
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
		response.badRequest("Could not decode request body", s.Logger)
		setResponseInWriter(w, response, s.Logger)
		return
	}

	action := r.URL.Query().Get("action")
	if action != "start" && action != "stop" {
		response.badRequest(fmt.Sprintf("The action {%s} is not supported", action), s.Logger)
		setResponseInWriter(w, response, s.Logger)
		return
	}

	serviceExists := false

	_ = level.Info(s.Logger).Log("msg", fmt.Sprintf("%s service with name {%s}", action, details.Service))

	if clients, ok := s.ServiceClients[details.Job]; ok {
		for i := 0; i < len(clients) && !serviceExists; i++ {
			if clients[i].Name == details.Service && clients[i].Target == details.Target {
				serviceRequest := newServiceRequest(details)
				s.performAction(action, &clients[i], serviceRequest, details, response)
				serviceExists = true
			}
		}

		if !serviceExists {
			response.badRequest(fmt.Sprintf("Service {%s} does not exist on target {%s}", details.Service, details.Target), s.Logger)
		}
	} else {
		response.badRequest(fmt.Sprintf("Could not find job {%s}", details.Job), s.Logger)
	}

	setResponseInWriter(w, response, s.Logger)
}

func (s *SController) performAction(action string, serviceConnection *network.ServiceClientConnection,
	serviceRequest *proto.ServiceRequest, details *Details, response *Response) {
	var statusResponse *proto.StatusResponse
	var err error

	switch {
	case action == "start":
		statusResponse, err = serviceConnection.Client.Start(context.Background(), serviceRequest)
	case action == "stop":
		statusResponse, err = serviceConnection.Client.Stop(context.Background(), serviceRequest)
	default:
		response.badRequest(fmt.Sprintf("Action {%s} not allowed", action), s.Logger)
		return
	}

	switch {
	case err != nil:
		err = errors.Wrap(err, fmt.Sprintf("Error response from target {%s}", details.Target))
		response.internalServerError(err.Error(), s.Logger)
	case statusResponse.Status != proto.StatusResponse_SUCCESS:
		response.internalServerError(fmt.Sprintf("Failure response from target {%s}", details.Target), s.Logger)
	default:
		response.Message = fmt.Sprintf("Response from target {%s}, {%s}, {%s}", details.Target, statusResponse.Message, statusResponse.Status)
		_ = level.Info(s.Logger).Log("msg", response.Message)
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
