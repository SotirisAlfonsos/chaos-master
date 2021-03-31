package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/SotirisAlfonsos/chaos-master/config"
	_ "github.com/SotirisAlfonsos/chaos-master/docs"
	"github.com/SotirisAlfonsos/chaos-master/healthcheck"
	"github.com/SotirisAlfonsos/chaos-master/pkg/chaoslogger"
	"github.com/SotirisAlfonsos/chaos-master/pkg/network"
	"github.com/SotirisAlfonsos/chaos-master/web/api"
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

	loggers := createLoggers(*debugLevel)

	conf, err := config.GetConfig(*configFile)
	if err != nil {
		_ = level.Error(loggers.ErrLogger).Log("err", err)
		os.Exit(1)
	}

	connections := network.GetConnectionPool(conf, loggers)
	jobMap := conf.GetJobMap(loggers)

	var healthChecker *healthcheck.HealthChecker

	if conf.HealthCheck.Active {
		healthChecker := healthcheck.Register(connections, loggers)
		healthChecker.Start(conf.HealthCheck.Report)
	}
	options := api.NewAPIOptions(conf.APIOptions, jobMap, connections, loggers)
	restAPI := api.NewRestAPI(options, healthChecker)
	restAPI.RunAPIController()
}

func createLoggers(debugLevel string) chaoslogger.Loggers {
	allowLevel := &chaoslogger.AllowedLevel{}
	if err := allowLevel.Set(debugLevel); err != nil {
		fmt.Printf("%v", err)
	}

	return chaoslogger.Loggers{
		OutLogger: chaoslogger.New(allowLevel, os.Stdout),
		ErrLogger: chaoslogger.New(allowLevel, os.Stderr),
	}
}
