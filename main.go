package main

import (
	"flag"
	"fmt"
	"github.com/SotirisAlfonsos/chaos-master/chaoslogger"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/go-kit/kit/log"
)

func main() {
	configFile := flag.String("config.file", "", "the file that contains the configuration for oscap scan")
	debugLevel := flag.String("debug.level", "info", "the debug level for the exporter. Could be debug, info, warn, error.")
	flag.Parse()

	logger := createLogger(*debugLevel)

	conf := config.GetConfig(*configFile, logger)

	conf.ApiConfig.RunApiController()
}

func createLogger(debugLevel string) log.Logger {
	allowLevel := &chaoslogger.AllowedLevel{}
	if err := allowLevel.Set(debugLevel); err != nil {
		fmt.Printf("%v", err)
	}

	return chaoslogger.New(allowLevel)
}
