package api

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SotirisAlfonsos/chaos-master/pkg/chaoslogger"
	"github.com/SotirisAlfonsos/gocache"

	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/SotirisAlfonsos/chaos-master/healthcheck"
	"github.com/SotirisAlfonsos/chaos-master/pkg/network"
	v1 "github.com/SotirisAlfonsos/chaos-master/web/api/v1"
	"github.com/go-kit/kit/log/level"
	"github.com/gorilla/mux"
)

type RestAPI struct {
	Router  *mux.Router
	Loggers chaoslogger.Loggers
	Port    string
}

func (restAPI *RestAPI) RunAPIController() {
	server := getServer(restAPI.Router, restAPI.Port)

	_ = level.Info(restAPI.Loggers.OutLogger).Log("msg", "starting web server on port "+restAPI.Port)

	c := make(chan os.Signal, 1)
	e := make(chan error)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		e <- server.ListenAndServe()
	}()

	select {
	case err := <-e:
		_ = level.Error(restAPI.Loggers.ErrLogger).Log("msg", "server error", "err", err)
		os.Exit(1)
	case <-c:
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		_ = level.Info(restAPI.Loggers.OutLogger).Log("msg", "Gracefully shutting down server")

		err := server.Shutdown(ctx)
		if err != nil {
			_ = level.Error(restAPI.Loggers.ErrLogger).Log("msg", "could not gracefully shut down server", "err", err)
		}
		cancel()
		os.Exit(0)
	}
}

type Options struct {
	restAPIOptions *config.RestAPIOptions
	jobMap         map[string]*config.Job
	connections    *network.Connections
	cache          *gocache.Cache
	loggers        chaoslogger.Loggers
}

func NewAPIOptions(
	restAPIOptions *config.RestAPIOptions,
	jobMap map[string]*config.Job,
	connections *network.Connections,
	loggers chaoslogger.Loggers,
) *Options {
	return &Options{
		restAPIOptions: restAPIOptions,
		jobMap:         jobMap,
		connections:    connections,
		cache:          gocache.New(0),
		loggers:        loggers,
	}
}

func NewRestAPI(opt *Options, healthChecker *healthcheck.HealthChecker) *RestAPI {
	router := mux.NewRouter()
	apiRouter := v1.NewAPIRouter(opt.jobMap, opt.connections, opt.cache, opt.loggers)
	router = apiRouter.AddRoutes(healthChecker, router)
	router.Schemes(opt.restAPIOptions.Scheme)

	return &RestAPI{
		Router:  router,
		Loggers: opt.loggers,
		Port:    opt.restAPIOptions.Port,
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
