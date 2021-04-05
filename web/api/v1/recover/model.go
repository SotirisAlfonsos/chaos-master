package recover

import (
	"errors"
	"fmt"
)

type RequestPayload struct {
	Alerts []*Alert `json:"alerts"`
}

type Alert struct {
	Status string  `json:"status"`
	Labels Options `json:"labels"`
}

type Options struct {
	RecoverJob    string `json:"recoverJob,omitempty"`
	RecoverTarget string `json:"recoverTarget,omitempty"`
	RecoverAll    bool   `json:"recoverAll,omitempty"`
}

type alertStatus int

const (
	firing alertStatus = iota
	resolved
	invalid
)

func (s alertStatus) String() string {
	return [...]string{"firing", "resolved"}[s]
}

func toStatusEnum(value string) (alertStatus, error) {
	switch value {
	case firing.String():
		return firing, nil
	case resolved.String():
		return resolved, nil
	}
	return invalid, errors.New(fmt.Sprintf("The status {%s} is not supported", value))
}
