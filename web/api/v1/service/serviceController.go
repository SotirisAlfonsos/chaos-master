package service

import (
	"fmt"
	"net/http"
)

func StartService(w http.ResponseWriter, r *http.Request) {
	fmt.Print("Starting service")
}

func StopService(w http.ResponseWriter, r *http.Request) {
	fmt.Print("Stopping service")
}
