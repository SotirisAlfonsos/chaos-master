package recover

import (
	"encoding/json"
	"net/http"

	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/response"
)

// RecoverActionAlertmanagerWebHook godoc
// @Summary recover from failures
// @Description Alertmanager webhook to recover from failures
// @Tags Recover
// @Accept json
// @Produce json
// @Param RequestPayload body RequestPayload true "Create request payload that contains the recovery details"
// @Success 200 {object} response.RecoverResponsePayload
// @Failure 400 {string} http.Error
// @Router /recover/alertmanager [post]
func (rController *RController) RecoverActionAlertmanagerWebHook(w http.ResponseWriter, r *http.Request) {
	recoverMessages := make([]*response.RecoverMessage, 0)

	requestPayload := &RequestPayload{}
	err := json.NewDecoder(r.Body).Decode(&requestPayload)
	if err != nil {
		response.BadRequest(w, "Could not decode request body", rController.loggers)
		return
	}

	for _, alert := range requestPayload.Alerts {
		status, err := toStatusEnum(alert.Status)
		if err != nil {
			response.BadRequest(w, err.Error(), rController.loggers)
			return
		} else if status == firing {
			recoverMessages = rController.performActionBasedOnOptions(alert.Labels)
		}
	}

	response.RecoverResponse(w, recoverMessages, rController.loggers)
}
