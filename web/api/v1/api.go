package v1

import (
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/docker"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

var base = "/chaos/api/v1"

type RestApi struct {
	Port   string `yaml:"port"`
	Scheme string `yaml:"scheme"`
}

func GetConfig() {

}

func (restApi *RestApi) RunApiController() {
	router := mux.NewRouter().StrictSlash(true)
	router.Schemes(restApi.Scheme)

	dockerRouter := router.PathPrefix(base + "/docker").Subrouter()
	dockerRouter.HandleFunc("{name}/start", docker.StartDocker).Methods("POST")
	dockerRouter.HandleFunc("{name}/stop", docker.StopDocker).Methods("POST")

	serviceRouter := router.PathPrefix(base + "/service").Subrouter()
	serviceRouter.HandleFunc("{name}/start", docker.StartDocker).Methods("POST")
	serviceRouter.HandleFunc("{name}/stop", docker.StartDocker).Methods("POST")

	log.Fatal(http.ListenAndServe(restApi.Port, router))
}
