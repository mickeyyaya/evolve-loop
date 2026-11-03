package main

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestShipIntegrationOf_ASoloCycleHasNoIntegration(t *testing.T) {
	t.Parallel()
	pol := policy.Policy{Fleet: &policy.FleetPolicy{Integration: &policy.FleetIntegrationPolicy{Stage: policy.FleetIntegrationEnforce}}}
	got, err := shipIntegrationOf(pol, false, []string{"a"})
	if err != nil || got != nil {
		t.Fatalf("solo cycle: got %+v, %v, want nil, nil", got, err)
	}
}

func TestShipIntegrationOf_AFleetLaneCarriesTheStageAndItsScope(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"", policy.FleetIntegrationShadow, policy.FleetIntegrationEnforce} {
		want := stage
		if want == "" {
			want = policy.FleetIntegrationShadow
		}
		pol := policy.Policy{Fleet: &policy.FleetPolicy{Integration: &policy.FleetIntegrationPolicy{Stage: stage}}}
		got, err := shipIntegrationOf(pol, true, []string{"task-a", "task-b"})
		if err != nil {
			t.Fatalf("stage %q: %v", stage, err)
		}
		if got == nil || got.Stage != want || got.Wave != shipIntegrationWave || !reflect.DeepEqual(got.Scope, []string{"task-a", "task-b"}) {
			t.Errorf("stage %q: got %+v, want stage %q, wave %q and the lane scope", stage, got, want, shipIntegrationWave)
		}
	}
}

func TestShipIntegrationOf_AWrongStageFailsTheLane(t *testing.T) {
	t.Parallel()
	pol := policy.Policy{Fleet: &policy.FleetPolicy{Integration: &policy.FleetIntegrationPolicy{Stage: "on"}}}
	got, err := shipIntegrationOf(pol, true, nil)
	if err == nil || got != nil || !strings.Contains(err.Error(), "fleet.integration.stage") {
		t.Fatalf("got %+v, %v, want an error naming the key", got, err)
	}
}

func TestWireOrchestratorDeps_AWrongIntegrationStageStopsAFleetLaneBeforeAnyDispatch(t *testing.T) {
	t.Setenv(ipcenv.FleetKey, "1")
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"fleet":{"integration":{"stage":"on"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{})
	if deps.CloseSignals != nil {
		t.Cleanup(deps.CloseSignals)
	}
	if deps.RoutingErr == nil || !strings.Contains(deps.RoutingErr.Error(), "fleet.integration.stage") || deps.Orchestrator != nil {
		t.Fatalf("deps = err %v orchestrator %v, want no orchestrator and an error naming fleet.integration.stage", deps.RoutingErr, deps.Orchestrator)
	}
}
