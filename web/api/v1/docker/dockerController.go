package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"

	"github.com/SotirisAlfonsos/chaos-master/network"
	"github.com/SotirisAlfonsos/chaos-slave/proto"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
)

type DController struct {
	DockerClients map[string][]network.DockerClientConnection
	Logger        log.Logger
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
	Job       string `json:"job"`
	Container string `json:"containerName"`
	Target    string `json:"target"`
}

type DetailsNoTarget struct {
	Job       string `json:"job"`
	Container string `json:"containerName"`
}

func getDefaultResponse() *Response {
	return &Response{
		Message: "",
		Error:   "",
		Status:  200,
	}
}

func newDockerRequest(details *Details) *proto.DockerRequest {
	return &proto.DockerRequest{
		JobName:              details.Job,
		Name:                 details.Container,
		XXX_NoUnkeyedLiteral: struct{}{},
		XXX_unrecognized:     nil,
		XXX_sizecache:        0,
	}
}

func newDockerRequestNoTarget(details *DetailsNoTarget) *proto.DockerRequest {
	return &proto.DockerRequest{
		JobName:              details.Job,
		Name:                 details.Container,
		XXX_NoUnkeyedLiteral: struct{}{},
		XXX_unrecognized:     nil,
		XXX_sizecache:        0,
	}
}

func (d *DController) DockerAction(w http.ResponseWriter, r *http.Request) {
	do := r.FormValue("do")
	if do != "" {
		d.randomDocker(w, r, do)
		return
	}

	response := getDefaultResponse()

	details := &Details{}
	err := json.NewDecoder(r.Body).Decode(&details)
	if err != nil {
		response.badRequest("Could not decode request body", d.Logger)
		setResponseInWriter(w, response, d.Logger)
		return
	}

	action, err := toActionEnum(r.FormValue("action"))
	if err != nil {
		response.badRequest(err.Error(), d.Logger)
		setResponseInWriter(w, response, d.Logger)
		return
	}

	dockerExists := false

	_ = level.Info(d.Logger).Log("msg", fmt.Sprintf("%s container with name {%s}", action, details.Container))

	if clients, ok := d.DockerClients[details.Job]; ok {
		for i := 0; i < len(clients) && !dockerExists; i++ {
			if clients[i].Name == details.Container && clients[i].Target == details.Target {
				dockerRequest := newDockerRequest(details)
				d.performAction(action, &clients[i], dockerRequest, details.Target, response)
				dockerExists = true
			}
		}

		if !dockerExists {
			response.badRequest(fmt.Sprintf("Container {%s} does not exist on target {%s}", details.Container, details.Target), d.Logger)
		}
	} else {
		response.badRequest(fmt.Sprintf("Could not find job {%s}", details.Job), d.Logger)
	}

	setResponseInWriter(w, response, d.Logger)
}

func (d *DController) performAction(action action, dockerConnection *network.DockerClientConnection,
	dockerRequest *proto.DockerRequest, target string, response *Response) {
	var statusResponse *proto.StatusResponse
	var err error

	switch action {
	case start:
		statusResponse, err = dockerConnection.Client.Start(context.Background(), dockerRequest)
	case stop:
		statusResponse, err = dockerConnection.Client.Stop(context.Background(), dockerRequest)
	default:
		response.badRequest(fmt.Sprintf("Action {%s} not allowed", action), d.Logger)
		return
	}

	switch {
	case err != nil:
		err = errors.Wrap(err, fmt.Sprintf("Error response from target {%s}", target))
		response.internalServerError(err.Error(), d.Logger)
	case statusResponse.Status != proto.StatusResponse_SUCCESS:
		response.internalServerError(fmt.Sprintf("Failure response from target {%s}", target), d.Logger)
	default:
		response.Message = fmt.Sprintf("Response from target {%s}, {%s}, {%s}", target, statusResponse.Message, statusResponse.Status)
		_ = level.Info(d.Logger).Log("msg", response.Message)
	}
}

func (d *DController) randomDocker(w http.ResponseWriter, r *http.Request, do string) {
	response := getDefaultResponse()

	if do != "random" {
		response := &Response{}
		response.badRequest(fmt.Sprintf("Do query parameter {%s} not allowed", do), d.Logger)
		setResponseInWriter(w, response, d.Logger)
		return
	}

	details := &DetailsNoTarget{}
	err := json.NewDecoder(r.Body).Decode(&details)
	if err != nil {
		response.badRequest("Could not decode request body", d.Logger)
		setResponseInWriter(w, response, d.Logger)
		return
	}

	action, err := toActionEnum(r.FormValue("action"))
	if err != nil {
		response.badRequest(err.Error(), d.Logger)
		setResponseInWriter(w, response, d.Logger)
		return
	}

	_ = level.Info(d.Logger).Log("msg", fmt.Sprintf("%s any container", action))

	if clients, ok := d.DockerClients[details.Job]; ok {
		client := getRandomClient(clients)
		if client.Name == details.Container {
			dockerRequest := newDockerRequestNoTarget(details)
			d.performAction(action, &client, dockerRequest, client.Target, response)
		} else {
			response.badRequest(fmt.Sprintf("Could not find container name {%s}", details.Container), d.Logger)
		}
	} else {
		response.badRequest(fmt.Sprintf("Could not find job {%s}", details.Job), d.Logger)
	}

	setResponseInWriter(w, response, d.Logger)
}

func getRandomClient(clients []network.DockerClientConnection) network.DockerClientConnection {
	return clients[rand.Intn(len(clients))]
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
