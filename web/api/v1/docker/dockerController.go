package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"

	"github.com/SotirisAlfonsos/chaos-master/cache"

	"github.com/SotirisAlfonsos/chaos-master/config"
	"google.golang.org/grpc"

	"github.com/SotirisAlfonsos/chaos-master/network"
	"github.com/SotirisAlfonsos/chaos-slave/proto"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
)

type DController struct {
	jobs           map[string]*config.Job
	connectionPool map[string]*connection
	cacheManager   *cache.Manager
	logger         log.Logger
}

type connection struct {
	grpcConn     *grpc.ClientConn
	dockerClient proto.DockerClient
}

func NewDockerController(jobs map[string]*config.Job, connections *network.Connections,
	cache *cache.Manager, logger log.Logger) *DController {
	connPool := make(map[string]*connection)
	for target, grpcConn := range connections.Pool {
		connPool[target] = &connection{
			grpcConn:     grpcConn,
			dockerClient: proto.NewDockerClient(grpcConn),
		}
	}
	return &DController{
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
		response.badRequest("Could not decode request body", d.logger)
		setResponseInWriter(w, response, d.logger)
		return
	}

	action, err := toActionEnum(r.FormValue("action"))
	if err != nil {
		response.badRequest(err.Error(), d.logger)
		setResponseInWriter(w, response, d.logger)
		return
	}

	dockerExists := false

	_ = level.Info(d.logger).Log("msg", fmt.Sprintf("%s container with name {%s}", action, details.Container))

	if job, ok := d.jobs[details.Job]; ok {
		for _, target := range job.Target {
			if job.ComponentName == details.Container && target == details.Target {
				dockerRequest := newDockerRequest(details)
				d.performAction(action, d.connectionPool[target].dockerClient, dockerRequest, details.Target, response)
				dockerExists = true
			}
		}

		if !dockerExists {
			response.badRequest(fmt.Sprintf("Container {%s} does not exist on target {%s}", details.Container, details.Target), d.logger)
		}
	} else {
		response.badRequest(fmt.Sprintf("Could not find job {%s}", details.Job), d.logger)
	}

	setResponseInWriter(w, response, d.logger)
}

func (d *DController) performAction(action action, dockerClient proto.DockerClient,
	dockerRequest *proto.DockerRequest, target string, response *Response) {
	var statusResponse *proto.StatusResponse
	var err error

	switch action {
	case start:
		statusResponse, err = dockerClient.Start(context.Background(), dockerRequest)
	case stop:
		statusResponse, err = dockerClient.Stop(context.Background(), dockerRequest)
	default:
		response.badRequest(fmt.Sprintf("Action {%s} not allowed", action), d.logger)
		return
	}

	switch {
	case err != nil:
		err = errors.Wrap(err, fmt.Sprintf("Error response from target {%s}", target))
		response.internalServerError(err.Error(), d.logger)
	case statusResponse.Status != proto.StatusResponse_SUCCESS:
		response.internalServerError(fmt.Sprintf("Failure response from target {%s}", target), d.logger)
	default:
		response.Message = fmt.Sprintf("Response from target {%s}, {%s}, {%s}", target, statusResponse.Message, statusResponse.Status)
		_ = level.Info(d.logger).Log("msg", response.Message)
		if err = d.updateCache(dockerRequest, dockerClient, target, action); err != nil {
			_ = level.Error(d.logger).Log("msg", fmt.Sprintf("Could not update cache for operation %s", action), "err", err)
		}
	}
}

func (d *DController) randomDocker(w http.ResponseWriter, r *http.Request, do string) {
	response := getDefaultResponse()

	if do != "random" {
		response := &Response{}
		response.badRequest(fmt.Sprintf("Do query parameter {%s} not allowed", do), d.logger)
		setResponseInWriter(w, response, d.logger)
		return
	}

	details := &DetailsNoTarget{}
	err := json.NewDecoder(r.Body).Decode(&details)
	if err != nil {
		response.badRequest("Could not decode request body", d.logger)
		setResponseInWriter(w, response, d.logger)
		return
	}

	action, err := toActionEnum(r.FormValue("action"))
	if err != nil {
		response.badRequest(err.Error(), d.logger)
		setResponseInWriter(w, response, d.logger)
		return
	}

	_ = level.Info(d.logger).Log("msg", fmt.Sprintf("%s any container", action))

	if job, ok := d.jobs[details.Job]; ok {
		target := getRandomTarget(job.Target)
		if job.ComponentName == details.Container {
			dockerRequest := newDockerRequestNoTarget(details)
			d.performAction(action, d.connectionPool[target].dockerClient, dockerRequest, target, response)
		} else {
			response.badRequest(fmt.Sprintf("Could not find container name {%s}", details.Container), d.logger)
		}
	} else {
		response.badRequest(fmt.Sprintf("Could not find job {%s}", details.Job), d.logger)
	}

	setResponseInWriter(w, response, d.logger)
}

func getRandomTarget(targets []string) string {
	return targets[rand.Intn(len(targets))]
}

func (d *DController) updateCache(dockerRequest *proto.DockerRequest, dockerClient proto.DockerClient, target string, action action) error {
	uniqueName := fmt.Sprintf("%s %s %s", dockerRequest.JobName, dockerRequest.Name, target)
	switch action {
	case start:
		d.cacheManager.Delete(uniqueName)
		return nil
	case stop:
		recoveryFunc := func() (*proto.StatusResponse, error) {
			return dockerClient.Start(context.Background(), dockerRequest)
		}
		return d.cacheManager.Register(uniqueName, recoveryFunc)
	default:
		return errors.New(fmt.Sprintf("Action %s not supported for cache operation", action))
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
