package api

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/SotirisAlfonsos/chaos-master/cache"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/SotirisAlfonsos/chaos-master/healthcheck"
	"github.com/SotirisAlfonsos/chaos-master/network"
	v1 "github.com/SotirisAlfonsos/chaos-master/web/api/v1"
	"github.com/go-kit/kit/log/level"
	"github.com/gorilla/mux"
)

type RestAPI struct {
	Router *mux.Router
	Logger log.Logger
	Port   string
}

func (restAPI *RestAPI) RunAPIController() {
	server := getServer(restAPI.Router, restAPI.Port)

	_ = level.Info(restAPI.Logger).Log("msg", "starting web server on port "+restAPI.Port)

	go func() {
		if err := server.ListenAndServe(); err != nil {
			_ = level.Error(restAPI.Logger).Log("msg", "Can not start web server", "err", err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c

	ctx, cancel := context.WithTimeout(context.Background(), 15)

	err := server.Shutdown(ctx)
	if err != nil {
		_ = level.Info(restAPI.Logger).Log("msg", "Gracefully shutting down server")
	}
	cancel()
	os.Exit(0)
}

type Options struct {
	restAPIOptions *config.RestAPIOptions
	jobMap         map[string]*config.Job
	connections    *network.Connections
	cache          *cache.Manager
	logger         log.Logger
}

func NewAPIOptions(
	restAPIOptions *config.RestAPIOptions,
	jobMap map[string]*config.Job,
	connections *network.Connections,
	logger log.Logger,
) *Options {
	return &Options{
		restAPIOptions: restAPIOptions,
		jobMap:         jobMap,
		connections:    connections,
		cache:          cache.NewCacheManager(logger),
		logger:         logger,
	}
}

func NewRestAPI(opt *Options, healthChecker *healthcheck.HealthChecker) *RestAPI {
	router := mux.NewRouter()
	apiRouter := v1.NewAPIRouter(opt.jobMap, opt.connections, opt.cache, opt.logger)
	router = apiRouter.AddRoutes(healthChecker, router)
	router.Schemes(opt.restAPIOptions.Scheme)

	return &RestAPI{
		Router: router,
		Logger: opt.logger,
		Port:   opt.restAPIOptions.Port,
	}
}

func getServer(router http.Handler, port string) *http.Server {
	server := &http.Server{
		Handler:      router,
		Addr:         ":" + port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	return server
}
