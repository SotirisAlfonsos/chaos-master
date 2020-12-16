package v1

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"time"

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

type RestAPIOptions struct {
	Port   string `yaml:"port"`
	Scheme string `yaml:"scheme"`
}

func (restAPIOpt *RestAPIOptions) NewRestAPI(clientConnections *network.Clients,
	healthChecker *healthcheck.HealthChecker, logger log.Logger) *RestAPI {
	base := "/chaos/api/v1"

	router := mux.NewRouter()
	setSlaveRouters(clientConnections, router, base, logger)
	setStatusRouter(healthChecker, router, base, logger)
	router.Schemes(restAPIOpt.Scheme)

	return &RestAPI{
		Router: router,
		Logger: logger,
		Port:   restAPIOpt.Port,
	}
}

func setSlaveRouters(clientConnections *network.Clients, router *mux.Router, base string, logger log.Logger) {
	jobRouter := router.PathPrefix(base).Subrouter()
	serviceControllerRouter(jobRouter, clientConnections, logger)
	dockerControllerRouter(jobRouter, clientConnections, logger)
}

func setStatusRouter(healthChecker *healthcheck.HealthChecker, router *mux.Router, base string, logger log.Logger) {
	statusController := &status.Slaves{StatusMap: healthChecker.DetailsMap, Logger: logger}
	serviceRouter := router.PathPrefix(base + "/master").Subrouter()
	serviceRouter.HandleFunc("/status", statusController.Status).Methods("GET")
}

func serviceControllerRouter(router *mux.Router, clientConnections *network.Clients, logger log.Logger) {
	sController := &service.SController{ServiceClients: clientConnections.Service, Logger: logger}
	router.HandleFunc("/service", sController.ServiceAction).
		Queries("action", "{action}").
		Methods("POST")
}

func dockerControllerRouter(router *mux.Router, clientConnections *network.Clients, logger log.Logger) {
	dController := &docker.DController{DockerClients: clientConnections.Docker, Logger: logger}
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
