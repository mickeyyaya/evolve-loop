package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

var errEnforceLaneNoPool = errors.New("fleet.integration.stage=enforce needs a pool id (--pool), and this fleet lane has none; start the lane through evolve loop")

func shipIntegrationOf(pol policy.Policy, fleetLane bool, scope []string, pool int, warn io.Writer) (*ship.Integration, error) {
	if !fleetLane {
		return nil, nil
	}
	stage, err := pol.FleetIntegrationStage()
	if err != nil {
		return nil, err
	}
	if pool > 0 {
		return &ship.Integration{Stage: stage, Wave: strconv.Itoa(pool), Scope: scope}, nil
	}
	if stage == policy.FleetIntegrationEnforce {
		return nil, errEnforceLaneNoPool
	}
	fmt.Fprintln(warn, "[integration] WARN: this fleet lane has no pool id (--pool), so it writes no deliverable record; the ship lands as before")
	return nil, nil
}
