package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pkg/errors"

	"github.com/SotirisAlfonsos/chaos-slave/proto"
	"github.com/go-kit/kit/log/level"
	"github.com/gorilla/mux"

	"github.com/SotirisAlfonsos/chaos-master/network"

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

func getDefaultResponse() *Response {
	return &Response{
		Message: "",
		Error:   "",
		Status:  200,
	}
}

func (s *SController) StartService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobName := vars["jobName"]
	name := vars["name"]
	target := vars["target"]
	serviceExists := false
	response := getDefaultResponse()

	_ = level.Info(s.Logger).Log("msg", fmt.Sprintf("Starting service with name <%s>", name))

	if clients, ok := s.ServiceClients[jobName]; ok {
		for i := 0; i < len(clients) && !serviceExists; i++ {
			if clients[i].Name == name && clients[i].Target == target {
				serviceRequest := &proto.ServiceRequest{
					JobName:              jobName,
					Name:                 name,
					XXX_NoUnkeyedLiteral: struct{}{},
					XXX_unrecognized:     nil,
					XXX_sizecache:        0,
				}
				statusResponse, err := clients[i].Client.Start(context.Background(), serviceRequest)
				response.setFromSlave(clients[i].Target, statusResponse, err, s.Logger)
				serviceExists = true
			}
		}

		if !serviceExists {
			response.Message = fmt.Sprintf("Service <%s> does not exist on target <%s>", name, target)
			response.Status = 400
		}
	} else {
		response.Message = fmt.Sprintf("Could not find job <%s>", jobName)
		response.Status = 400
	}

	err := setResponseInWriter(w, response)
	if err != nil {
		_ = level.Error(s.Logger).Log("msg", "Error when trying to write start service response", "err", err)
	}
}

func (s *SController) StopService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobName := vars["jobName"]
	name := vars["name"]
	target := vars["target"]
	serviceExists := false
	response := getDefaultResponse()

	_ = level.Info(s.Logger).Log("msg", fmt.Sprintf("Stopping service with name <%s>", name))

	if clients, ok := s.ServiceClients[jobName]; ok {
		for i := 0; i < len(clients) && !serviceExists; i++ {
			if clients[i].Name == name && clients[i].Target == target {
				serviceRequest := &proto.ServiceRequest{
					JobName:              jobName,
					Name:                 name,
					XXX_NoUnkeyedLiteral: struct{}{},
					XXX_unrecognized:     nil,
					XXX_sizecache:        0,
				}
				statusResponse, err := clients[i].Client.Stop(context.Background(), serviceRequest)
				response.setFromSlave(clients[i].Target, statusResponse, err, s.Logger)
				serviceExists = true
			}
		}

		if !serviceExists {
			response.Message = fmt.Sprintf("Service <%s> does not exist on target <%s>", name, target)
			response.Status = 400
		}
	} else {
		response.Message = fmt.Sprintf("Could not find job <%s>", jobName)
		response.Status = 400
	}

	err := setResponseInWriter(w, response)
	if err != nil {
		_ = level.Error(s.Logger).Log("msg", "Error when trying to write stop service response", "err", err)
	}
}

func (resp *Response) setFromSlave(target string, statusResponse *proto.StatusResponse, err error, logger log.Logger) {
	switch {
	case err != nil:
		resp.Message = fmt.Sprintf("Error response from target <%s>", target)
		resp.Error = err.Error()
		resp.Status = 500
		_ = level.Error(logger).Log("msg", resp.Message, "err", err)
	case statusResponse.Status != proto.StatusResponse_SUCCESS:
		resp.Message = fmt.Sprintf("Failure response from target <%s>", target)
		resp.Status = 500
		_ = level.Error(logger).Log("msg", resp.Message, "err", err)
	default:
		resp.Message = fmt.Sprintf("Response from target <%s>, <%s>, <%s>", target, statusResponse.Message, statusResponse.Status)
		_ = level.Info(logger).Log("msg", resp.Message)
	}
}

func setResponseInWriter(w http.ResponseWriter, resp *Response) error {
	reqBodyBytes := new(bytes.Buffer)
	err := json.NewEncoder(reqBodyBytes).Encode(resp)
	if err != nil {
		err = errors.Wrap(err, "Error when trying to encode response to byte array")
		return err
	}

	w.WriteHeader(resp.Status)
	_, err = w.Write(reqBodyBytes.Bytes())
	if err != nil {
		return err
	}

	return nil
}
