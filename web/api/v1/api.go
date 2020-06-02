package v1

import (
	"net/http"
	"time"

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

func (restAPI *RestAPI) RunAPIController(clientConnections *network.Clients, logger log.Logger) {
	base := "/chaos/api/v1"

	router := getRouter(clientConnections, base, restAPI.Scheme, logger)
	server := getServer(router, restAPI.Port)

	_ = level.Info(logger).Log("msg", "starting web server on port "+restAPI.Port)
	if err := server.ListenAndServe(); err != nil {
		_ = level.Error(logger).Log("msg", "Can not start web server", "err", err)
	}
}

func getRouter(clientConnections *network.Clients, base string, scheme string, logger log.Logger) *mux.Router {
	router := mux.NewRouter()

	serviceControllerRouter(router, clientConnections, base, logger)
	dockerControllerRouter(router, clientConnections, base, logger)
	router.Schemes(scheme)

	return router
}

func serviceControllerRouter(router *mux.Router, clientConnections *network.Clients, base string, logger log.Logger) {
	sController := &service.SController{ServiceClients: clientConnections.Service, Logger: logger}
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
