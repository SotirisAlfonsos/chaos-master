package main

import (
	"flag"
	"fmt"

	"github.com/SotirisAlfonsos/chaos-master/chaoslogger"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/SotirisAlfonsos/chaos-master/network"
	"github.com/go-kit/kit/log"
)

func main() {
	configFile := flag.String("config.file", "", "the file that contains the configuration for the chaos master")
	debugLevel := flag.String("debug.level", "info", "the debug level for the chaos master")
	flag.Parse()

	logger := createLogger(*debugLevel)

	conf := config.GetConfig(*configFile, logger)

	clientConnections := network.GetClientConnections(conf.ChaosSlaves, logger)

	conf.APIConfig.RunAPIController(clientConnections, logger)
}

func createLogger(debugLevel string) log.Logger {
	allowLevel := &chaoslogger.AllowedLevel{}
	if err := allowLevel.Set(debugLevel); err != nil {
		fmt.Printf("%v", err)
	}

	return chaoslogger.New(allowLevel)
}
