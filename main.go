package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/SotirisAlfonsos/chaos-master/chaoslogger"
	"github.com/SotirisAlfonsos/chaos-master/config"
	_ "github.com/SotirisAlfonsos/chaos-master/docs"
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

	connections := network.GetConnectionPool(conf, logger)
	jobMap := conf.GetJobMap(logger)

	var healthChecker *healthcheck.HealthChecker

	if conf.HealthCheck.Active {
		healthChecker := healthcheck.Register(connections, logger)
		healthChecker.Start(conf.HealthCheck.Report)
	}
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
