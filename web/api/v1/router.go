package v1

import (
	"github.com/SotirisAlfonsos/chaos-master/cache"
	"github.com/SotirisAlfonsos/chaos-master/config"
	"github.com/SotirisAlfonsos/chaos-master/healthcheck"
	"github.com/SotirisAlfonsos/chaos-master/network"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/cpu"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/docker"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/service"
	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
	httpSwagger "github.com/swaggo/http-swagger"
)

type APIRouter struct {
	jobMap      map[string]*config.Job
	connections *network.Connections
	cache       *cache.Manager
	logger      log.Logger
}

func NewAPIRouter(
	jobMap map[string]*config.Job,
	connections *network.Connections,
	cache *cache.Manager,
	logger log.Logger,
) *APIRouter {
	return &APIRouter{
		jobMap:      jobMap,
		connections: connections,
		cache:       cache,
		logger:      logger,
	}
}

func (r *APIRouter) AddRoutes(healthChecker *healthcheck.HealthChecker, router *mux.Router) *mux.Router {
	base := "/chaos/api/v1"

	router = router.PathPrefix(base).Subrouter()
	setBotRouters(router, r)
	setRecoverRouter(router, r)
	setStatusRouter(healthChecker, router, r.logger)
	setSwaggerRouter(router)

	return router
}

func setBotRouters(router *mux.Router, r *APIRouter) {
	serviceControllerRouter(router, r)
	dockerControllerRouter(router, r)
	cpuControllerRouter(router, r)
}

func setRecoverRouter(router *mux.Router, r *APIRouter) {
	rController := NewRecoverController(r.cache, r.logger)
	router.HandleFunc("/recover/alertmanager", rController.RecoverActionAlertmanagerWebHook).
		Methods("POST")
}

func setStatusRouter(healthChecker *healthcheck.HealthChecker, router *mux.Router, logger log.Logger) {
	statusController := &Bots{StatusMap: healthChecker.DetailsMap, Logger: logger}
	router.HandleFunc("/master/status", statusController.Status).Methods("GET")
}

func serviceControllerRouter(router *mux.Router, r *APIRouter) {
	sController := service.NewServiceController(filterJobsOnType(r.jobMap, config.Service), r.connections, r.cache, r.logger)
	router.HandleFunc("/service", sController.ServiceAction).
		Queries("action", "{action}").
		Methods("POST")
}

func dockerControllerRouter(router *mux.Router, r *APIRouter) {
	dController := docker.NewDockerController(filterJobsOnType(r.jobMap, config.Docker), r.connections, r.cache, r.logger)
	router.HandleFunc("/docker", dController.DockerAction).
		Queries("action", "{action}").
		Methods("POST")
}

func cpuControllerRouter(router *mux.Router, r *APIRouter) {
	cController := cpu.NewCPUController(filterJobsOnType(r.jobMap, config.CPU), r.connections, r.cache, r.logger)
	router.HandleFunc("/cpu", cController.CPUAction).
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
