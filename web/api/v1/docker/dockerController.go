package docker

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

func StartDockerWithName(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	fmt.Println("Starting docker with name " + name)
}

func StopDockerWithName(w http.ResponseWriter, r *http.Request) {
	fmt.Print("Stopping docker")
}

func StopAnyDocker(w http.ResponseWriter, r *http.Request) {
	fmt.Print("Stopping any docker")
}
