package docker

import (
	"context"
	"fmt"
	"math/rand"
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
	vars := mux.Vars(r)
	jobName := vars["jobName"]
	name := vars["name"]

	_ = level.Info(d.Logger).Log("msg", fmt.Sprintf("Starting docker with name %s", name))

	if clients, ok := d.DockerClients[jobName]; ok {
		statusResponse, err := clients[0].Client.Start(context.Background(), &proto.DockerRequest{
			Name:                 name,
			XXX_NoUnkeyedLiteral: struct{}{},
			XXX_unrecognized:     nil,
			XXX_sizecache:        0,
		})
		if err != nil {
			_ = level.Error(d.Logger).Log("msg", fmt.Sprintf("Error response from target %s,", clients[0].Target), "err", err)
		}

		_ = level.Info(d.Logger).Log("msg",
			fmt.Sprintf("Response from target %s, %s, %s", clients[0].Target, statusResponse.Message, statusResponse.Status))
	}
}

func (d *DController) StopDockerWithName(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobName := vars["jobName"]
	name := vars["name"]

	_ = level.Info(d.Logger).Log("msg", fmt.Sprintf("Stopping docker with name %s", name))

	if clients, ok := d.DockerClients[jobName]; ok {
		statusResponse, err := clients[0].Client.Stop(context.Background(), &proto.DockerRequest{
			Name:                 name,
			XXX_NoUnkeyedLiteral: struct{}{},
			XXX_unrecognized:     nil,
			XXX_sizecache:        0,
		})
		if err != nil {
			_ = level.Error(d.Logger).Log("msg", fmt.Sprintf("Error response from target %s,", clients[0].Target), "err", err)
		}

		_ = level.Info(d.Logger).Log("msg",
			fmt.Sprintf("Response from target %s, %s, %s", clients[0].Target, statusResponse.Message, statusResponse.Status))
	}
}

func (d *DController) StopAnyDocker(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobName := vars["jobName"]
	name := ""
	_ = level.Info(d.Logger).Log("msg", "Stopping any docker")

	if clients, ok := d.DockerClients[jobName]; ok {
		client := getRandomClient(clients)

		statusResponse, err := client.Client.Stop(context.Background(), &proto.DockerRequest{
			Name:                 name,
			XXX_NoUnkeyedLiteral: struct{}{},
			XXX_unrecognized:     nil,
			XXX_sizecache:        0,
		})
		if err != nil {
			_ = level.Error(d.Logger).Log("msg", fmt.Sprintf("Error response from target %s,", client.Target), "err", err)
		}

		_ = level.Info(d.Logger).Log("msg",
			fmt.Sprintf("Response from target %s, %s, %s", client.Target, statusResponse.Message, statusResponse.Status))
	}
}

func getRandomClient(clients []network.DockerClientConnection) network.DockerClientConnection {
	return clients[rand.Intn(len(clients))]
}
