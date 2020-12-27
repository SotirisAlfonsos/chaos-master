package v1

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/SotirisAlfonsos/chaos-master/config"

	"github.com/SotirisAlfonsos/chaos-master/healthcheck"
	"github.com/SotirisAlfonsos/chaos-master/network"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/docker"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/service"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/status"
	"github.com/go-kit/kit/log"
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

type options struct {
	restAPIOptions *config.RestAPIOptions
	jobMap         map[string]*config.Job
	connections    *network.Connections
}

func NewAPIOptions(restAPIOptions *config.RestAPIOptions, jobMap map[string]*config.Job,
	connections *network.Connections) *options {
	return &options{
		restAPIOptions: restAPIOptions,
		jobMap:         jobMap,
		connections:    connections,
	}
}

func NewRestAPI(opt *options, healthChecker *healthcheck.HealthChecker, logger log.Logger) *RestAPI {
	base := "/chaos/api/v1"

	router := mux.NewRouter()
	setSlaveRouters(router, opt, base, logger)
	setStatusRouter(healthChecker, router, base, logger)
	router.Schemes(opt.restAPIOptions.Scheme)

	return &RestAPI{
		Router: router,
		Logger: logger,
		Port:   opt.restAPIOptions.Port,
	}
}

func setSlaveRouters(router *mux.Router, opt *options, base string, logger log.Logger) {
	jobRouter := router.PathPrefix(base).Subrouter()
	serviceControllerRouter(jobRouter, opt, logger)
	dockerControllerRouter(jobRouter, opt, logger)
}

func setStatusRouter(healthChecker *healthcheck.HealthChecker, router *mux.Router, base string, logger log.Logger) {
	statusController := &status.Slaves{StatusMap: healthChecker.DetailsMap, Logger: logger}
	serviceRouter := router.PathPrefix(base + "/master").Subrouter()
	serviceRouter.HandleFunc("/status", statusController.Status).Methods("GET")
}

func serviceControllerRouter(router *mux.Router, options *options, logger log.Logger) {
	sController := service.NewServiceController(filterJobsOnType(options.jobMap, config.Service), options.connections, logger)
	router.HandleFunc("/service", sController.ServiceAction).
		Queries("action", "{action}").
		Methods("POST")
}

func dockerControllerRouter(router *mux.Router, options *options, logger log.Logger) {
	dController := docker.NewDockerController(filterJobsOnType(options.jobMap, config.Docker), options.connections, logger)
	router.HandleFunc("/docker", dController.DockerAction).
		Queries("action", "{action}").
		Methods("POST")
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

func filterJobsOnType(jobMap map[string]*config.Job, failureType config.FailureType) map[string]*config.Job {
	newJobMap := make(map[string]*config.Job)
	for target, job := range jobMap {
		if job.FailureType == failureType {
			newJobMap[target] = job
		}
	}
	return newJobMap
}
