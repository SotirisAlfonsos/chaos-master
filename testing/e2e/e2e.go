package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/server"

	"github.com/SotirisAlfonsos/chaos-master/pkg/chaoslogger"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/cpu"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/docker"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/network"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/recover"
	"github.com/SotirisAlfonsos/chaos-master/web/api/v1/service"
	"github.com/go-kit/kit/log/level"
)

var loggers = getLoggers()

func main() {
	_ = level.Info(loggers.OutLogger).Log("msg", "------------ Init start service and container ------------")
	setUp()

	_ = level.Info(loggers.OutLogger).Log("msg", "------------ Start CPU failure ------------")
	status := cpuFailure("start")
	if status != http.StatusOK {
		_ = level.Error(loggers.ErrLogger).Log("err", fmt.Sprintf("failed to start cpu failure with status %d", status))
		os.Exit(1)
	}

	_ = level.Info(loggers.OutLogger).Log("msg", "------------ Start Network failure ------------")
	networkFailure()

	_ = level.Info(loggers.OutLogger).Log("msg", "------------ Sleep for 30 seconds ------------")
	time.Sleep(30 * time.Second)

	_ = level.Info(loggers.OutLogger).Log("msg", "------------ Start non existing service ------------")
	status = serviceFailure("test", "recover")
	if status != http.StatusBadRequest {
		_ = level.Error(loggers.ErrLogger).Log("err", fmt.Sprintf("should not be able to recover service {test} with status %d", status))
		os.Exit(1)
	}

	_ = level.Info(loggers.OutLogger).Log("msg", "------------ Start non existing container ------------")
	status = dockerFailure("test", "recover")
	if status != http.StatusBadRequest {
		_ = level.Error(loggers.ErrLogger).Log("err", fmt.Sprintf("should not be able to recover conatiner {test} with status %d", status))
		os.Exit(1)
	}

	_ = level.Info(loggers.OutLogger).Log("msg", "------------ Kill service------------")
	status = serviceFailure("simple", "kill")
	if status != http.StatusOK {
		_ = level.Error(loggers.ErrLogger).Log("err", fmt.Sprintf("failed to kill service {simple} with status %d", status))
		os.Exit(1)
	}

	_ = level.Info(loggers.OutLogger).Log("msg", "------------ Kill container ------------")
	status = dockerFailure("zookeeper", "kill")
	if status != http.StatusOK {
		_ = level.Error(loggers.ErrLogger).Log("err", fmt.Sprintf("failed to kill container {zookeeper} with status %d", status))
		os.Exit(1)
	}

	recoverTarget("127.0.0.1:8081")

	_ = level.Info(loggers.OutLogger).Log("msg", "------------ Kill container ------------")
	status = dockerFailure("zookeeper", "kill")
	if status != http.StatusOK {
		_ = level.Error(loggers.ErrLogger).Log("err", fmt.Sprintf("failed to kill container {zookeeper} with status %d", status))
		os.Exit(1)
	}

	_ = level.Info(loggers.OutLogger).Log("msg", "------------ Recover CPU that has already recovered ------------")
	status = cpuFailure("recover")
	if status != http.StatusInternalServerError {
		_ = level.Error(loggers.ErrLogger).Log("err", "should not be able to recover already recovered cpu failure")
		os.Exit(1)
	}

	recoverAlertmanagerAll()

	//_ = level.Info(loggers.OutLogger).Log("msg", "------------ Kill server ------------")
	//serverFailure()

	defer cleanUp()

	_ = level.Info(loggers.OutLogger).Log("msg", "------------ E2e test SUCCESS ------------")
}

func setUp() {
	status := serviceFailure("simple", "recover")
	if status != http.StatusOK {
		_ = level.Error(loggers.ErrLogger).Log("err", fmt.Sprintf("failed to recover service {simple} with status %d", status))
		os.Exit(1)
	}

	status = dockerFailure("zookeeper", "recover")
	if status != http.StatusOK {
		_ = level.Error(loggers.ErrLogger).Log("err", fmt.Sprintf("failed to recover container {zookeeper} with status %d", status))
		os.Exit(1)
	}
}

func cleanUp() {
	status := serviceFailure("simple", "kill")
	if status != http.StatusOK {
		_ = level.Error(loggers.ErrLogger).Log("err", "failed to kill service {simple}")
		os.Exit(1)
	}

	status = dockerFailure("zookeeper", "kill")
	if status != http.StatusOK {
		_ = level.Error(loggers.ErrLogger).Log("err", "failed to kill container {zookeeper}")
		os.Exit(1)
	}
}

func serviceFailure(serviceName string, action string) int {
	request := &service.RequestPayload{
		Job:         "zookeeper service",
		ServiceName: serviceName,
		Target:      "127.0.0.1:8081",
	}
	status, err := serviceAction(action, request)
	if err != nil {
		_ = level.Error(loggers.ErrLogger).Log("err", err)
		os.Exit(1)
	}

	return status
}

func dockerFailure(containerName string, action string) int {
	request := &docker.RequestPayload{
		Job:       "zookeeper docker",
		Container: containerName,
		Target:    "127.0.0.1:8081",
	}
	status, err := dockerAction(action, request)
	if err != nil {
		_ = level.Error(loggers.ErrLogger).Log("err", err)
		os.Exit(1)
	}

	return status
}

func cpuFailure(action string) int {
	request := &cpu.RequestPayload{
		Job:        "cpu_injection",
		Percentage: 10,
		Target:     "127.0.0.1:8081",
	}
	status, err := cpuAction(action, request)
	if err != nil {
		_ = level.Error(loggers.ErrLogger).Log("err", err)
		os.Exit(1)
	}

	return status
}

func networkFailure() {
	request := &network.RequestPayload{
		Job:      "network injection",
		Device:   "lo",
		Loss:     10.5,
		LossCorr: 10.5,
		Target:   "127.0.0.1:8081",
	}
	err := networkOKAction("start", request)
	if err != nil {
		_ = level.Error(loggers.ErrLogger).Log("err", err)
		os.Exit(1)
	}
}

func serverFailure() {
	request := &server.RequestPayload{
		Job:    "server_injection",
		Target: "127.0.0.1:8081",
	}
	err := serverAction("kill", request)
	if err != nil {
		_ = level.Error(loggers.ErrLogger).Log("err", err)
		os.Exit(1)
	}
}

func serviceAction(action string, request *service.RequestPayload) (int, error) {
	_ = level.Info(loggers.OutLogger).Log("msg", fmt.Sprintf("%sing service {%s}", action, request.ServiceName))

	url := "http://127.0.0.1:8090/chaos/api/v1/service?action=" + action
	requestBody, _ := json.Marshal(request)

	resp, err := http.Post(url, "application/json", bytes.NewReader(requestBody))
	if err != nil {
		return 0, fmt.Errorf(err.Error())
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}

func dockerAction(action string, request *docker.RequestPayload) (int, error) {
	_ = level.Info(loggers.OutLogger).Log("msg", fmt.Sprintf("%sing docker {%s}", action, request.Container))

	url := "http://127.0.0.1:8090/chaos/api/v1/docker?action=" + action
	requestBody, _ := json.Marshal(request)

	resp, err := http.Post(url, "application/json", bytes.NewReader(requestBody))
	if err != nil {
		return 0, fmt.Errorf(err.Error())
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}

func recoverTarget(target string) {
	_ = level.Info(loggers.OutLogger).Log("msg", "------------ Recover target ------------")

	request := &recover.Options{RecoverTarget: target}

	err := recoverAllOK(request)
	if err != nil {
		_ = level.Error(loggers.ErrLogger).Log("err", err)
		os.Exit(1)
	}
}

func recoverAlertmanagerAll() {
	_ = level.Info(loggers.OutLogger).Log("msg", "------------ Recover alertmanager all ------------")

	request := &recover.RequestPayload{
		Alerts: []*recover.Alert{
			{Status: "firing", Labels: recover.Options{RecoverAll: true}},
		},
	}

	err := recoverAlertmanagerAllOK(request)
	if err != nil {
		_ = level.Error(loggers.ErrLogger).Log("err", err)
		os.Exit(1)
	}
}

func cpuAction(action string, request *cpu.RequestPayload) (int, error) {
	_ = level.Info(loggers.OutLogger).Log("msg", fmt.Sprintf("%sing cpu injection", action))

	url := "http://127.0.0.1:8090/chaos/api/v1/cpu?action=" + action
	requestBody, _ := json.Marshal(request)

	resp, err := http.Post(url, "application/json", bytes.NewReader(requestBody))
	if err != nil {
		return 0, fmt.Errorf(err.Error())
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}

func networkOKAction(action string, request *network.RequestPayload) error {
	_ = level.Info(loggers.OutLogger).Log("msg", fmt.Sprintf("%sing network injection", action))

	url := "http://127.0.0.1:8090/chaos/api/v1/network?action=" + action
	requestBody, _ := json.Marshal(request)

	resp, err := http.Post(url, "application/json", bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to %s network injection", action)
	}

	return nil
}

func serverAction(action string, request *server.RequestPayload) error {
	_ = level.Info(loggers.OutLogger).Log("msg", fmt.Sprintf("%sing server", action))

	url := "http://127.0.0.1:8090/chaos/api/v1/server?action=" + action
	requestBody, _ := json.Marshal(request)

	resp, err := http.Post(url, "application/json", bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to %s server", action)
	}

	return nil
}

func recoverAlertmanagerAllOK(request *recover.RequestPayload) error {
	_ = level.Info(loggers.OutLogger).Log("msg", "Recovering alertmanager all failures")

	url := "http://127.0.0.1:8090/chaos/api/v1/recover/alertmanager"
	requestBody, _ := json.Marshal(request)

	resp, err := http.Post(url, "application/json", bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to recover failures")
	}

	return nil
}

func recoverAllOK(request *recover.Options) error {
	_ = level.Info(loggers.OutLogger).Log("msg", "Recovering all failures")

	url := "http://127.0.0.1:8090/chaos/api/v1/recover"
	requestBody, _ := json.Marshal(request)

	resp, err := http.Post(url, "application/json", bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf(err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to recover failures")
	}

	return nil
}

func getLoggers() chaoslogger.Loggers {
	allowLevel := &chaoslogger.AllowedLevel{}
	if err := allowLevel.Set("debug"); err != nil {
		fmt.Printf("%v", err)
	}

	return chaoslogger.Loggers{
		ErrLogger: chaoslogger.New(allowLevel, os.Stderr),
		OutLogger: chaoslogger.New(allowLevel, os.Stdout),
	}
}
