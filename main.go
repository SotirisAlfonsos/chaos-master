package main

import (
	"flag"
	"fmt"
	"os"

	_ "github.com/SotirisAlfonsos/chaos-master/docs"

	"github.com/SotirisAlfonsos/chaos-master/chaoslogger"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/SotirisAlfonsos/chaos-master/healthcheck"
	"github.com/SotirisAlfonsos/chaos-master/network"
	"github.com/SotirisAlfonsos/chaos-master/web/api"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

// @title Chaos Master API
// @version 1.0
// @description This is the chaos master API.

// @host localhost:8090
// @BasePath /chaos/api/v1
func main() {
	configFile := flag.String("config.file", "", "the file that contains the configuration for the chaos master")
	debugLevel := flag.String("debug.level", "info", "the debug level for the chaos master")
	flag.Parse()

	logger := createLogger(*debugLevel)

	conf, err := config.GetConfig(*configFile)
	if err != nil {
		_ = level.Error(logger).Log("err", err)
		os.Exit(1)
	}

	connections := network.GetConnectionPool(conf.JobsFromConfig, logger)
	jobMap := conf.GetJobMap(logger)
	showRegisteredJobs(jobMap, logger)

	healthChecker := healthcheck.Register(connections, logger)
	healthChecker.Start(conf.HealthCheckReport)

	options := api.NewAPIOptions(conf.APIOptions, jobMap, connections, logger)
	restAPI := api.NewRestAPI(options, healthChecker)
	restAPI.RunAPIController()
}

func createLogger(debugLevel string) log.Logger {
	allowLevel := &chaoslogger.AllowedLevel{}
	if err := allowLevel.Set(debugLevel); err != nil {
		fmt.Printf("%v", err)
	}

	return chaoslogger.New(allowLevel)
}

func showRegisteredJobs(jobsMap map[string]*config.Job, logger log.Logger) {
	for jobName, job := range jobsMap {
		_ = level.Info(logger).Log("msg", fmt.Sprintf("{%s} job registered for component {%s} type {%s} and targets %v",
			jobName, job.ComponentName, job.FailureType, job.Target))
	}
}
