package network

import (
	"fmt"

	"github.com/SotirisAlfonsos/chaos-slave/proto"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"google.golang.org/grpc"
)

type Clients struct {
	Health  []proto.HealthClient
	Service []proto.ServiceClient
	Docker  map[string]proto.DockerClient
}

func GetClientConnections(slaves []string, logger log.Logger) *Clients {
	clientConnections := &Clients{Docker: make(map[string]proto.DockerClient)}

	for _, slave := range slaves {
		updateClientConnection(clientConnections, slave, logger)
	}

	return clientConnections
}

func updateClientConnection(clientConnections *Clients, slave string, logger log.Logger) {
	clientConn, err := withTestGrpcClientConn(slave, logger)
	if err != nil {
		_ = level.Error(logger).Log("msg", fmt.Sprintf("failed to dial server address for slave %s", slave), "err", err)
	}

	clientConnections.Health = append(clientConnections.Health, proto.NewHealthClient(clientConn))
	clientConnections.Service = append(clientConnections.Service, proto.NewServiceClient(clientConn))
	clientConnections.Docker[slave] = proto.NewDockerClient(clientConn)
}

func withTestGrpcClientConn(slave string, logger log.Logger) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(slave, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	return conn, nil
}
