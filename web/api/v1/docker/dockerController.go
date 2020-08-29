package docker

import (
	"context"
	"fmt"
	"net/http"

	"github.com/SotirisAlfonsos/chaos-master/network"

	"github.com/SotirisAlfonsos/chaos-slave/proto"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gorilla/mux"
)

type DController struct {
	DockerClients map[string][]network.DockerClientConnection
	Logger        log.Logger
}

func (d *DController) StartDockerWithName(w http.ResponseWriter, r *http.Request) {
	_ = level.Info(d.Logger).Log("msg", "Start docker controller")
	//vars := mux.Vars(r)
	////jobName := vars["jobName"]
	//name := vars["name"]
	//
	//_ = level.Info(d.Logger).Log("msg", fmt.Sprintf("Starting docker with name %s", name))
	//
	//for slave, cli := range d.DockerClients {
	//	statusResponse, err := cli.Start(context.Background(), &proto.DockerRequest{
	//		Name:                 name,
	//		XXX_NoUnkeyedLiteral: struct{}{},
	//		XXX_unrecognized:     nil,
	//		XXX_sizecache:        0,
	//	})
	//	if err != nil {
	//		_ = level.Error(d.Logger).Log("msg", fmt.Sprintf("Error response from slave %s,", slave), "err", err)
	//	}
	//
	//	_ = level.Info(d.Logger).Log("msg",
	//		fmt.Sprintf("Response from slave %s, %s, %s", slave, statusResponse.Message, statusResponse.Status))
	//}
}

func (d *DController) StopDockerWithName(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	_ = level.Info(d.Logger).Log("msg", fmt.Sprintf("Stopping docker with name %s", name))

	for slave, cli := range d.DockerClients {
		statusResponse, err := cli[0].Client.Stop(context.Background(), &proto.DockerRequest{
			Name:                 name,
			XXX_NoUnkeyedLiteral: struct{}{},
			XXX_unrecognized:     nil,
			XXX_sizecache:        0,
		})
		if err != nil {
			_ = level.Error(d.Logger).Log("msg", fmt.Sprintf("Error response from slave %s,", slave), "err", err)
		}

		_ = level.Info(d.Logger).Log("msg",
			fmt.Sprintf("Response from slave %s, %s, %s", slave, statusResponse.Message, statusResponse.Status))
	}
}

func (d *DController) StopAnyDocker(w http.ResponseWriter, r *http.Request) {
	_ = level.Info(d.Logger).Log("msg", "Stopping any docker")
}
