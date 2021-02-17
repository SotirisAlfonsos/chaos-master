package response

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sort"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

type Payload struct {
	Message string `json:"message"`
	Error   string `json:"error"`
	Status  int    `json:"status"`
}

func NewDefaultResponse() *Payload {
	return &Payload{
		Message: "",
		Error:   "",
		Status:  200,
	}
}

func (p *Payload) BadRequest(message string, logger log.Logger) {
	p.Error = message
	p.Status = 400
	_ = level.Warn(logger).Log("msg", "Bad request", "warn", message)
}

func (p *Payload) InternalServerError(message string, logger log.Logger) {
	p.Error = message
	p.Status = 500
	_ = level.Error(logger).Log("msg", "Internal server error", "err", message)
}

func (p *Payload) SetInWriter(w http.ResponseWriter, logger log.Logger) {
	reqBodyBytes := new(bytes.Buffer)
	err := json.NewEncoder(reqBodyBytes).Encode(p)
	if err != nil {
		_ = level.Error(logger).Log("msg", "Error when trying to encode response to byte array", "err", err)
		w.WriteHeader(500)
		return
	}

	w.WriteHeader(p.Status)
	_, err = w.Write(reqBodyBytes.Bytes())
	if err != nil {
		_ = level.Error(logger).Log("msg", "Error when trying to write response to byte array", "err", err)
		w.WriteHeader(500)
		return
	}
}

type RecoverResponsePayload struct {
	RecoverMessage []*RecoverMessage `json:"recoverMessages"`
	Status         int               `json:"status"`
}

func (recoverR *RecoverResponsePayload) BadRequest(message string, logger log.Logger) {
	recoverR.RecoverMessage = append(recoverR.RecoverMessage, FailureRecoverResponse(message))
	recoverR.Status = 400
	_ = level.Warn(logger).Log("msg", "Bad request", "warn", message)
}

func (recoverR *RecoverResponsePayload) GetSortedStatuses() []string {
	statuses := make([]string, 0, len(recoverR.RecoverMessage))
	for _, recoverMessage := range recoverR.RecoverMessage {
		statuses = append(statuses, recoverMessage.Status)
	}
	sort.Strings(statuses)
	return statuses
}

func (recoverR *RecoverResponsePayload) SetInWriter(w http.ResponseWriter, logger log.Logger) {
	reqBodyBytes := new(bytes.Buffer)
	err := json.NewEncoder(reqBodyBytes).Encode(recoverR)
	if err != nil {
		_ = level.Error(logger).Log("msg", "Error when trying to encode response to byte array", "err", err)
		w.WriteHeader(500)
		return
	}

	w.WriteHeader(recoverR.Status)
	_, err = w.Write(reqBodyBytes.Bytes())
	if err != nil {
		_ = level.Error(logger).Log("msg", "Error when trying to write response to byte array", "err", err)
		w.WriteHeader(500)
		return
	}
}

type RecoverMessage struct {
	Message string `json:"message"`
	Error   string `json:"error"`
	Status  string `json:"status"`
}

func NewDefaultRecoverResponse() *RecoverResponsePayload {
	return &RecoverResponsePayload{
		RecoverMessage: make([]*RecoverMessage, 0),
		Status:         200,
	}
}

func FailureRecoverResponse(message string) *RecoverMessage {
	return &RecoverMessage{
		Error:  message,
		Status: "FAILURE",
	}
}

func SuccessRecoverResponse(message string) *RecoverMessage {
	return &RecoverMessage{
		Message: message,
		Status:  "SUCCESS",
	}
}
