package main

import (
	"flag"
	"fmt"

	"github.com/go-kit/kit/log/level"

	"github.com/SotirisAlfonsos/chaos-master/healthcheck"

	"github.com/SotirisAlfonsos/chaos-master/network"

	"github.com/SotirisAlfonsos/chaos-master/chaoslogger"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/go-kit/kit/log"
)

func main() {
	configFile := flag.String("config.file", "", "the file that contains the configuration for the chaos master")
	debugLevel := flag.String("debug.level", "info", "the debug level for the chaos master")
	flag.Parse()

	logger := createLogger(*debugLevel)

	conf := config.GetConfig(*configFile, logger)

	clientConnections := network.GetConnectionsFromSlaveList(conf.ChaosSlaves, logger)
	showRegisteredJobs(clientConnections, logger)

	healthChecker := healthcheck.Register(clientConnections.Health, logger)
	healthChecker.Start(conf.HealthCheckReport)

	conf.APIConfig.RunAPIController(clientConnections, healthChecker, logger)
}

func createLogger(debugLevel string) log.Logger {
	allowLevel := &chaoslogger.AllowedLevel{}
	if err := allowLevel.Set(debugLevel); err != nil {
		fmt.Printf("%v", err)
	}

	return chaoslogger.New(allowLevel)
}

func showRegisteredJobs(clientConnections *network.Clients, logger log.Logger) {
	showDockerJobs(clientConnections.Docker, logger)
	showServiceJobs(clientConnections.Service, logger)
}

func showDockerJobs(dockerJobs map[string][]network.DockerClientConnection, logger log.Logger) {
	jobList := make([]string, 0)
	for job := range dockerJobs {
		jobList = append(jobList, job)
	}

	_ = level.Info(logger).Log("msg", fmt.Sprintf("docker jobs registered %v", jobList))
}

func showServiceJobs(serviceJobs map[string][]network.ServiceClientConnection, logger log.Logger) {
	jobList := make([]string, 0)
	for job := range serviceJobs {
		jobList = append(jobList, job)
	}

	_ = level.Info(logger).Log("msg", fmt.Sprintf("service jobs registered %v", jobList))
}
