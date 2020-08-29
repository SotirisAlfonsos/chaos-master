package network

import (
	"fmt"

	"github.com/SotirisAlfonsos/chaos-slave/proto"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"google.golang.org/grpc"
)

type ChaosSlave struct {
	JobName       string       `yaml:"job_name,omitempty"`
	Instance      InstanceType `yaml:"instance,omitempty"`
	ComponentName string       `yaml:"component_name,omitempty"`
	Targets       []string     `yaml:"targets,omitempty"`
}

type InstanceType string

const (
	Docker  InstanceType = "Docker"
	Service InstanceType = "Service"
)

type Clients struct {
	Health  map[string]HealthClientConnection
	Service map[string][]ServiceClientConnection
	Docker  map[string][]DockerClientConnection
}

type HealthClientConnection struct {
	Client proto.HealthClient
}

type ServiceClientConnection struct {
	Name   string
	Target string
	Client proto.ServiceClient
}

type DockerClientConnection struct {
	Name   string
	Target string
	Client proto.DockerClient
}

func GetConnectionsFromSlaveList(chaosSlaves []*ChaosSlave, logger log.Logger) *Clients {
	clientConnections := &Clients{
		Health:  make(map[string]HealthClientConnection),
		Service: make(map[string][]ServiceClientConnection),
		Docker:  make(map[string][]DockerClientConnection),
	}

	for _, chaosSlave := range chaosSlaves {
		chaosSlave.updateClientConnections(clientConnections, logger)
	}

	return clientConnections
}

func (cs *ChaosSlave) updateClientConnections(clientConnections *Clients, logger log.Logger) {
	switch cs.Instance {
	case Docker:
		if _, ok := clientConnections.Docker[cs.JobName]; ok {
			_ = level.Error(logger).Log("msg", fmt.Sprintf("The job name %s is not unique", cs.JobName))
		} else {
			clientConnections.Docker[cs.JobName] = make([]DockerClientConnection, 0)
			cs.setDockerClientConnection(clientConnections, logger)
		}
	case Service:
		if _, ok := clientConnections.Service[cs.JobName]; ok {
			_ = level.Error(logger).Log("msg", fmt.Sprintf("The job name %s is not unique", cs.JobName))
		} else {
			clientConnections.Service[cs.JobName] = make([]ServiceClientConnection, 0)
			cs.setServiceClientConnection(clientConnections, logger)
		}
	default:
		_ = level.Error(logger).Log("msg", fmt.Sprintf("instance registration for %s has not been implemented", cs.Instance))
		return
	}

	cs.setHealthClientConnection(clientConnections, logger)
}

func (cs *ChaosSlave) setDockerClientConnection(clientConnections *Clients, logger log.Logger) {
	for _, target := range cs.Targets {
		clientConn, err := withTestGrpcClientConn(target)
		if err != nil {
			_ = level.Error(logger).Log("msg", fmt.Sprintf("failed to dial server address for slave %s", target), "err", err)
		}

		dockerClientConnection := &DockerClientConnection{
			Name:   cs.ComponentName,
			Target: target,
			Client: proto.NewDockerClient(clientConn),
		}
		clientConnections.Docker[cs.JobName] = append(clientConnections.Docker[cs.JobName], *dockerClientConnection)
	}
}

func (cs *ChaosSlave) setServiceClientConnection(clientConnections *Clients, logger log.Logger) {
	for _, target := range cs.Targets {
		clientConn, err := withTestGrpcClientConn(target)
		if err != nil {
			_ = level.Error(logger).Log("msg", fmt.Sprintf("failed to dial server address for slave %s", target), "err", err)
		}

		serverClientConnection := &ServiceClientConnection{
			Name:   cs.ComponentName,
			Target: target,
			Client: proto.NewServiceClient(clientConn),
		}
		clientConnections.Service[cs.JobName] = append(clientConnections.Service[cs.JobName], *serverClientConnection)
	}
}

func (cs *ChaosSlave) setHealthClientConnection(clientConnections *Clients, logger log.Logger) {
	for _, target := range cs.Targets {
		if _, ok := clientConnections.Health[target]; !ok {
			clientConn, err := withTestGrpcClientConn(target)
			if err != nil {
				_ = level.Error(logger).Log("msg", fmt.Sprintf("failed to dial server address for slave %s", target), "err", err)
			}

			clientConnections.Health[target] = HealthClientConnection{Client: proto.NewHealthClient(clientConn)}
			_ = level.Info(logger).Log("msg", fmt.Sprintf("Target %s register as slave", target))
		}
	}
}

func withTestGrpcClientConn(target string) (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(target, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	return conn, nil
}
