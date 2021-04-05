package v1

import (
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/SotirisAlfonsos/chaos-master/healthcheck"
	"github.com/SotirisAlfonsos/chaos-master/pkg/chaoslogger"
	"github.com/SotirisAlfonsos/chaos-master/pkg/network"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/cpu"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/docker"
	apiNetwork "github.com/SotirisAlfonsos/chaos-master/web/api/v1/network"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/recover"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/server"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/service"
	"github.com/SotirisAlfonsos/gocache"
	"github.com/gorilla/mux"
	httpSwagger "github.com/swaggo/http-swagger"
)

type APIRouter struct {
	jobMap      map[string]*config.Job
	connections *network.Connections
	Cache       *gocache.Cache
	loggers     chaoslogger.Loggers
}

func NewAPIRouter(
	jobMap map[string]*config.Job,
	connections *network.Connections,
	cache *gocache.Cache,
	loggers chaoslogger.Loggers,
) *APIRouter {
	return &APIRouter{
		jobMap:      jobMap,
		connections: connections,
		Cache:       cache,
		loggers:     loggers,
	}
}

func (r *APIRouter) AddRoutes(healthChecker *healthcheck.HealthChecker, router *mux.Router) *mux.Router {
	base := "/chaos/api/v1"

	router = router.PathPrefix(base).Subrouter()
	setBotRouters(router, r)
	setRecoverRouter(router, r)
	if healthChecker != nil {
		setStatusRouter(healthChecker, router, r.loggers)
	}
	setSwaggerRouter(router)

	return router
}

func setBotRouters(router *mux.Router, r *APIRouter) {
	serviceControllerRouter(router, r)
	dockerControllerRouter(router, r)
	cpuControllerRouter(router, r)
	serverControllerRouter(router, r)
	networkControllerRouter(router, r)
}

func setRecoverRouter(router *mux.Router, r *APIRouter) {
	rController := recover.NewRecoverController(r.Cache, r.loggers)
	router.HandleFunc("/recover", rController.RecoverAction).
		Methods("POST")
	router.HandleFunc("/recover/alertmanager", rController.RecoverActionAlertmanagerWebHook).
		Methods("POST")
}

func setStatusRouter(healthChecker *healthcheck.HealthChecker, router *mux.Router, loggers chaoslogger.Loggers) {
	statusController := &Bots{StatusMap: healthChecker.DetailsMap, Loggers: loggers}
	router.HandleFunc("/master/status", statusController.Status).Methods("GET")
}

func serviceControllerRouter(router *mux.Router, r *APIRouter) {
	sController := service.NewServiceController(filterJobsOnType(r.jobMap, config.Service), r.connections, r.Cache, r.loggers)
	router.HandleFunc("/service", sController.ServiceAction).
		Queries("action", "{action}").
		Methods("POST")
}

func dockerControllerRouter(router *mux.Router, r *APIRouter) {
	dController := docker.NewDockerController(filterJobsOnType(r.jobMap, config.Docker), r.connections, r.Cache, r.loggers)
	router.HandleFunc("/docker", dController.DockerAction).
		Queries("action", "{action}").
		Methods("POST")
}

func cpuControllerRouter(router *mux.Router, r *APIRouter) {
	cController := cpu.NewCPUController(filterJobsOnType(r.jobMap, config.CPU), r.connections, r.Cache, r.loggers)
	router.HandleFunc("/cpu", cController.CPUAction).
		Queries("action", "{action}").
		Methods("POST")
}

func serverControllerRouter(router *mux.Router, r *APIRouter) {
	s := server.NewServerController(filterJobsOnType(r.jobMap, config.Server), r.connections, r.loggers)
	router.HandleFunc("/server", s.ServerAction).
		Queries("action", "{action}").
		Methods("POST")
}

func networkControllerRouter(router *mux.Router, r *APIRouter) {
	n := apiNetwork.NewNetworkController(filterJobsOnType(r.jobMap, config.Network), r.connections, r.Cache, r.loggers)
	router.HandleFunc("/network", n.NetworkAction).
		Queries("action", "{action}").
		Methods("POST")
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

func setSwaggerRouter(router *mux.Router) {
	router.PathPrefix("/swagger").Handler(httpSwagger.WrapHandler)
}
