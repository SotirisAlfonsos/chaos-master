package response

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/SotirisAlfonsos/chaos-master/pkg/chaoslogger"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

type Payload struct {
	Message string `json:"message"`
	Status  int    `json:"status"`
}

func NewDefaultResponse() *Payload {
	return &Payload{
		Message: "",
		Status:  200,
	}
}

// The serverError helper writes an error message and stack trace to the errorLog,
// then sends a generic 500 Internal Server Error response to the user.
func serverError(w http.ResponseWriter, loggers chaoslogger.Loggers, message string) {
	statusText := http.StatusText(http.StatusInternalServerError)
	_ = level.Error(loggers.ErrLogger).Log("msg", statusText, "err", message)

	http.Error(w, message, http.StatusInternalServerError)
}

// The clientError helper sends a specific status code and corresponding description
// to the user. We'll use this later in the book to send responses like 400 "Bad
// Request" when there's a problem with the request that the user sent.
func clientError(w http.ResponseWriter, loggers chaoslogger.Loggers, message string, status int) {
	_ = level.Info(loggers.OutLogger).Log("msg", http.StatusText(status), "warn", message)
	http.Error(w, message, status)
}

func BadRequest(w http.ResponseWriter, message string, loggers chaoslogger.Loggers) {
	status := http.StatusBadRequest
	clientError(w, loggers, message, status)
}

func InternalServerError(w http.ResponseWriter, message string, loggers chaoslogger.Loggers) {
	serverError(w, loggers, message)
}

func OkResponse(w http.ResponseWriter, message string, loggers chaoslogger.Loggers) {
	resp := &Payload{
		Message: message,
		Status:  200,
	}
	resp.SetInWriter(w, loggers)
}

func (p *Payload) SetInWriter(w http.ResponseWriter, loggers chaoslogger.Loggers) {
	reqBodyBytes := new(bytes.Buffer)
	err := json.NewEncoder(reqBodyBytes).Encode(p)
	if err != nil {
		_ = level.Error(loggers.ErrLogger).Log("msg", "Error when trying to encode response to byte array", "err", err)
		w.WriteHeader(500)
		return
	}

	w.WriteHeader(p.Status)
	_, err = w.Write(reqBodyBytes.Bytes())
	if err != nil {
		_ = level.Error(loggers.ErrLogger).Log("msg", "Error when trying to write response to byte array", "err", err)
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

func OkRecoverResponse(w http.ResponseWriter, messages []*RecoverMessage, loggers chaoslogger.Loggers) {
	resp := &RecoverResponsePayload{
		RecoverMessage: messages,
		Status:         200,
	}
	resp.SetInWriter(w, loggers)
}

func (recoverR *RecoverResponsePayload) SetInWriter(w http.ResponseWriter, loggers chaoslogger.Loggers) {
	reqBodyBytes := new(bytes.Buffer)
	err := json.NewEncoder(reqBodyBytes).Encode(recoverR)
	if err != nil {
		_ = level.Error(loggers.ErrLogger).Log("msg", "Error when trying to encode response to byte array", "err", err)
		w.WriteHeader(500)
		return
	}

	w.WriteHeader(recoverR.Status)
	_, err = w.Write(reqBodyBytes.Bytes())
	if err != nil {
		_ = level.Error(loggers.ErrLogger).Log("msg", "Error when trying to write response to byte array", "err", err)
		w.WriteHeader(500)
		return
	}
}

type RecoverMessage struct {
	Message string `json:"message"`
	Error   string `json:"error"`
	Status  string `json:"status"`
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
