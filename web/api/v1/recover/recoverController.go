package recover

import (
	"encoding/json"
	"net/http"

	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/response"
)

// RecoverAction godoc
// @Summary recover from failures
// @Description Recover from failures endpoint
// @Tags Recover
// @Accept json
// @Produce json
// @Param Options body Options true "Create request payload that contains the recovery details"
// @Success 200 {object} response.RecoverResponsePayload
// @Failure 400 {string} http.Error
// @Router /recover [post]
func (rController *RController) RecoverAction(w http.ResponseWriter, r *http.Request) {
	recoverMessages := make([]*response.RecoverMessage, 0)

	request := &Options{}
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		response.BadRequest(w, "Could not decode request body", rController.loggers)
		return
	}

	recoverMessages = rController.performActionBasedOnOptions(*request)

	response.RecoverResponse(w, recoverMessages, rController.loggers)
}
