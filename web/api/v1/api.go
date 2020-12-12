package v1

import (
	"net/http"
	"time"

	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/status"

	"github.com/SotirisAlfonsos/chaos-master/healthcheck"

	"github.com/SotirisAlfonsos/chaos-master/network"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/docker"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/service"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/gorilla/mux"
)

type RestAPI struct {
	Port   string `yaml:"port"`
	Scheme string `yaml:"scheme"`
}

func (restAPI *RestAPI) RunAPIController(clientConnections *network.Clients,
	healthChecker *healthcheck.HealthChecker, logger log.Logger) {
	base := "/chaos/api/v1"

	router := mux.NewRouter()
	setSlaveRouters(clientConnections, router, base, logger)
	setStatusRouter(healthChecker, router, base, logger)
	router.Schemes(restAPI.Scheme)
	server := getServer(router, restAPI.Port)

	_ = level.Info(logger).Log("msg", "starting web server on port "+restAPI.Port)
	if err := server.ListenAndServe(); err != nil {
		_ = level.Error(logger).Log("msg", "Can not start web server", "err", err)
	}
}

func setSlaveRouters(clientConnections *network.Clients, router *mux.Router, base string, logger log.Logger) {
	jobRouter := router.PathPrefix(base + "/job/{jobName}").Subrouter()
	serviceControllerRouter(jobRouter, clientConnections, base, logger)
	dockerControllerRouter(jobRouter, clientConnections, base, logger)
}

func setStatusRouter(healthChecker *healthcheck.HealthChecker, router *mux.Router, base string, logger log.Logger) {
	statusController := &status.Slaves{StatusMap: healthChecker.DetailsMap, Logger: logger}
	serviceRouter := router.PathPrefix(base + "/master").Subrouter()
	serviceRouter.HandleFunc("/status", statusController.Status).Methods("GET")
}

func serviceControllerRouter(router *mux.Router, clientConnections *network.Clients, base string, logger log.Logger) {
	sController := &service.SController{ServiceClientConnections: clientConnections.Service, Logger: logger}
	serviceRouter := router.PathPrefix(base + "/service").Subrouter()
	serviceRouter.HandleFunc("/{name}/start", sController.StartService).Methods("POST")
	serviceRouter.HandleFunc("/{name}/stop", sController.StopService).Methods("POST")
}

func dockerControllerRouter(router *mux.Router, clientConnections *network.Clients, base string, logger log.Logger) {
	dController := &docker.DController{DockerClients: clientConnections.Docker, Logger: logger}
	dockerRouter := router.PathPrefix(base + "/docker").Subrouter()
	dockerRouter.HandleFunc("/{name}/start", dController.StartDockerWithName).Methods("POST")
	dockerRouter.HandleFunc("/{name}/stop", dController.StopDockerWithName).Methods("POST")
	dockerRouter.HandleFunc("/stop", dController.StopAnyDocker).Methods("POST")
}

func getServer(router http.Handler, port string) *http.Server {
	server := &http.Server{
		Handler:      router,
		Addr:         "127.0.0.1:" + port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	return server
}
