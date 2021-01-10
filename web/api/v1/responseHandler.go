package v1

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

type ResponsePayload struct {
	Message string `json:"message"`
	Error   string `json:"error"`
	Status  int    `json:"status"`
}

type RecoverResponsePayload struct {
	RecoverMessage []*RecoverMessage `json:"recoverMessages"`
	Status         int               `json:"status"`
}

type RecoverMessage struct {
	Message string `json:"message"`
	Error   string `json:"error"`
	Status  string `json:"status"`
}

func newDefaultRecoverResponse() *RecoverResponsePayload {
	return &RecoverResponsePayload{
		RecoverMessage: make([]*RecoverMessage, 0),
		Status:         200,
	}
}

func (recoverR *RecoverResponsePayload) badRequest(message string, logger log.Logger) {
	recoverR.RecoverMessage = append(recoverR.RecoverMessage, failureRecoverResponse(message))
	recoverR.Status = 400
	_ = level.Warn(logger).Log("msg", "Bad request", "warn", message)
}

func failureRecoverResponse(message string) *RecoverMessage {
	return &RecoverMessage{
		Error:  message,
		Status: "FAILURE",
	}
}

func successRecoverResponse(message string) *RecoverMessage {
	return &RecoverMessage{
		Message: message,
		Status:  "SUCCESS",
	}
}

func setRecoverResponseInWriter(w http.ResponseWriter, resp *RecoverResponsePayload, logger log.Logger) {
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
