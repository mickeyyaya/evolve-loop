package policy

import (
	"errors"
	"fmt"
	"time"
)

const (
	FleetIntegrationShadow  = "shadow"
	FleetIntegrationEnforce = "enforce"
	DefaultPullRefWaitS     = 5
)

var errInvalidPullRefWait = errors.New("policy: fleet.integration.pull_ref_wait_s must be >= 0")

type FleetIntegrationPolicy struct {
	Stage        string `json:"stage,omitempty"`
	PullRefWaitS *int   `json:"pull_ref_wait_s,omitempty"`
}

func (p Policy) FleetIntegrationStage() (string, error) {
	if p.Fleet == nil || p.Fleet.Integration == nil || p.Fleet.Integration.Stage == "" {
		return FleetIntegrationShadow, nil
	}
	switch v := p.Fleet.Integration.Stage; v {
	case FleetIntegrationShadow, FleetIntegrationEnforce:
		return v, nil
	default:
		return "", fmt.Errorf("policy: fleet.integration.stage must be shadow or enforce, got %q", v)
	}
}

func (p Policy) PullRefWait() (time.Duration, error) {
	if p.Fleet == nil || p.Fleet.Integration == nil || p.Fleet.Integration.PullRefWaitS == nil {
		return DefaultPullRefWaitS * time.Second, nil
	}
	v := *p.Fleet.Integration.PullRefWaitS
	if v < 0 {
		return 0, fmt.Errorf("%w, got %d", errInvalidPullRefWait, v)
	}
	return time.Duration(v) * time.Second, nil
}
