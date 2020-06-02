package service

import (
	"fmt"
	"net/http"

	"github.com/SotirisAlfonsos/chaos-slave/proto"
	"github.com/go-kit/kit/log"
)

type SController struct {
	ServiceClients []proto.ServiceClient
	Logger         log.Logger
}

func (s *SController) StartService(w http.ResponseWriter, r *http.Request) {
	fmt.Print("Starting service")
}

func (s *SController) StopService(w http.ResponseWriter, r *http.Request) {
	fmt.Print("Stopping service")
}
