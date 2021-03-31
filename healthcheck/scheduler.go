package healthcheck

import (
	"context"
	"fmt"

	"github.com/SotirisAlfonsos/chaos-master/pkg/chaoslogger"

	v1 "github.com/SotirisAlfonsos/chaos-bot/proto/grpc/v1"
	"github.com/SotirisAlfonsos/chaos-master/pkg/network"
	"github.com/go-kit/kit/log/level"
	"github.com/robfig/cron/v3"
)

type HealthChecker struct {
	DetailsMap map[string]*Details
	loggers    chaoslogger.Loggers
}

type Details struct {
	Status     v1.HealthCheckResponse_ServingStatus
	connection network.Connection
}

func Register(connections *network.Connections, loggers chaoslogger.Loggers) *HealthChecker {
	healthChecker := &HealthChecker{DetailsMap: make(map[string]*Details), loggers: loggers}
	for target, connection := range connections.Pool {
		healthChecker.DetailsMap[target] = &Details{
			Status:     v1.HealthCheckResponse_UNKNOWN,
			connection: connection,
		}
	}

	return healthChecker
}

func (hch *HealthChecker) Start(report bool) {
	c := cron.New()
	id, err := c.AddFunc("@every 1m", func() {
		for target, details := range hch.DetailsMap {
			client, err := details.connection.GetHealthClient()
			if err != nil {
				_ = level.Error(hch.loggers.ErrLogger).Log(
					"msg", fmt.Sprintf("Can not get healthcheck connection for target {%s}", target),
					"err", err)
				continue
			}

			resp, err := client.Check(context.Background(), &v1.HealthCheckRequest{})
			if err != nil {
				_ = level.Error(hch.loggers.ErrLogger).Log(
					"msg", fmt.Sprintf("Failed to get valid response when health-checking target %s", target),
					"err", err)
				hch.DetailsMap[target].Status = v1.HealthCheckResponse_NOT_SERVING
			} else {
				hch.DetailsMap[target].Status = resp.Status
			}
		}

		_ = level.Debug(hch.loggers.OutLogger).Log("msg", "checking status of bots")

		if report {
			for target, details := range hch.DetailsMap {
				_ = level.Info(hch.loggers.OutLogger).Log("msg", fmt.Sprintf("Status of bot %s is %s", target, details.Status))
			}
		}
	})

	if err != nil {
		_ = level.Error(hch.loggers.ErrLogger).Log("msg", "could not create scheduling task for automated health-checks")
	}

	_ = level.Info(hch.loggers.OutLogger).Log("msg", fmt.Sprintf("starting automated health-check scheduler with id %d", id))

	c.Start()
}
