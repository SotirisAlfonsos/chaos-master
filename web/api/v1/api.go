package v1

import (
	"net/http"
	"time"

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

func (restAPI *RestAPI) RunAPIController(logger log.Logger) {
	base := "/chaos/api/v1"

	router := getRouter(base, restAPI.Scheme)
	server := getServer(router, restAPI.Port)

	_ = level.Info(logger).Log("msg", "starting web server on port "+restAPI.Port)
	if err := server.ListenAndServe(); err != nil {
		_ = level.Error(logger).Log("msg", "Can not start web server", "err", err)
	}
}

func getRouter(base string, scheme string) *mux.Router {
	router := mux.NewRouter()

	dockerRouter := router.PathPrefix(base + "/docker").Subrouter()
	dockerRouter.HandleFunc("/{name}/start", docker.StartDockerWithName).Methods("POST")
	dockerRouter.HandleFunc("/{name}/stop", docker.StopDockerWithName).Methods("POST")
	dockerRouter.HandleFunc("/stop", docker.StopAnyDocker).Methods("POST")

	serviceRouter := router.PathPrefix(base + "/service").Subrouter()
	serviceRouter.HandleFunc("/{name}/start", service.StartService).Methods("POST")
	serviceRouter.HandleFunc("/{name}/stop", service.StopService).Methods("POST")
	router.Schemes(scheme)

	return router
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
