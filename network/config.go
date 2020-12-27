package network

import (
	"fmt"

	"github.com/SotirisAlfonsos/chaos-master/config"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"google.golang.org/grpc"
)

type Connections struct {
	Pool map[string]*grpc.ClientConn
}

//
//type Clients struct {
//	Health  map[string]HealthClientConnection
//	Service map[string][]ServiceClientConnection
//	Docker  map[string][]DockerClientConnection
//}
//
//type HealthClientConnection struct {
//	Client proto.HealthClient
//}
//
//type ServiceClientConnection struct {
//	Name   string
//	Target string
//	Client proto.ServiceClient
//}
//
//type DockerClientConnection struct {
//	Name   string
//	Target string
//	Client proto.DockerClient
//}

func GetConnectionPool(jobsFromConfig []*config.JobsFromConfig, logger log.Logger) *Connections {
	connections := &Connections{
		Pool: make(map[string]*grpc.ClientConn),
	}

	for _, jobFromConfig := range jobsFromConfig {
		addConnectionForTargets(connections, jobFromConfig.Targets, logger)
	}

	return connections
}

func addConnectionForTargets(connections *Connections, targets []string, logger log.Logger) {
	for _, target := range targets {
		if err := addConnectionForTarget(connections, target); err != nil {
			_ = level.Error(logger).Log("msg", fmt.Sprintf("failed to add connection to target %s, to connection pool", target), "err", err)
		}
	}
}

func addConnectionForTarget(connections *Connections, target string) error {
	if _, ok := connections.Pool[target]; !ok {
		clientConn, err := grpc.Dial(target, grpc.WithInsecure())
		if err != nil {
			connections.Pool[target] = nil
			return err
		}
		connections.Pool[target] = clientConn
	}
	return nil
}
