package v1

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/SotirisAlfonsos/chaos-master/healthcheck"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

type Bots struct {
	StatusMap map[string]*healthcheck.Details
	Logger    log.Logger
}

// CalcExample godoc
// @Summary get master status
// @Description Bot status
// @Tags example
// @Accept json
// @Produce json
// @Success 200 {string} string "Bots status"
// @Failure 500 {string} string "ok"
// @Router /master/status [get]
func (bots *Bots) Status(w http.ResponseWriter, r *http.Request) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintln("Bots status:"))
	for botHost, botDetails := range bots.StatusMap {
		sb.WriteString(fmt.Sprintln(botHost, botDetails.Status.String()))
	}

	response, err := w.Write([]byte(sb.String()))
	if err != nil {
		_ = level.Error(bots.Logger).Log("msg",
			fmt.Sprintf("Error when trying to write the status %d", response), "err", err)
	}
}
