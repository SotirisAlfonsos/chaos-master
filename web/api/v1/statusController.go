package v1

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"

	"github.com/SotirisAlfonsos/chaos-master/healthcheck"
)

type Slaves struct {
	StatusMap map[string]*healthcheck.Details
	Logger    log.Logger
}

func (slaves *Slaves) Status(w http.ResponseWriter, r *http.Request) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintln("Slaves status:"))
	for slaveHost, slaveDetails := range slaves.StatusMap {
		sb.WriteString(fmt.Sprintln(slaveHost, slaveDetails.Status.String()))
	}

	response, err := w.Write([]byte(sb.String()))
	if err != nil {
		_ = level.Error(slaves.Logger).Log("msg",
			fmt.Sprintf("Error when trying to write the status %d", response), "err", err)
	}
}
