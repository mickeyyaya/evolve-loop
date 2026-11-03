package main

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const shipIntegrationWave = "current"

func shipIntegrationOf(pol policy.Policy, fleetLane bool, scope []string) (*ship.Integration, error) {
	if !fleetLane {
		return nil, nil
	}
	stage, err := pol.FleetIntegrationStage()
	if err != nil {
		return nil, err
	}
	return &ship.Integration{Stage: stage, Wave: shipIntegrationWave, Scope: scope}, nil
}
