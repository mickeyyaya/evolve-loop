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
	got, err := shipIntegrationOf(pol, false, []string{"a"}, 0, io.Discard)
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
		got, err := shipIntegrationOf(pol, true, []string{"task-a", "task-b"}, 41, io.Discard)
		if err != nil {
			t.Fatalf("stage %q: %v", stage, err)
		}
		if got == nil || got.Stage != want || got.Wave != "41" || !reflect.DeepEqual(got.Scope, []string{"task-a", "task-b"}) {
			t.Errorf("stage %q: got %+v, want stage %q, wave 41 and the lane scope", stage, got, want)
		}
	}
}

func TestShipIntegrationOf_AShadowLaneWithNoPoolWritesNoRecordAndWarnsOnce(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"", policy.FleetIntegrationShadow} {
		pol := policy.Policy{Fleet: &policy.FleetPolicy{Integration: &policy.FleetIntegrationPolicy{Stage: stage}}}
		var warn strings.Builder
		got, err := shipIntegrationOf(pol, true, []string{"a"}, 0, &warn)
		if err != nil || got != nil {
			t.Fatalf("stage %q: got %+v, %v, want nil, nil", stage, got, err)
		}
		want := "[integration] WARN: this fleet lane has no pool id (--pool), so it writes no deliverable record; the ship lands as before\n"
		if warn.String() != want {
			t.Errorf("stage %q: warning = %q, want %q", stage, warn.String(), want)
		}
	}
}

func TestShipIntegrationOf_AnEnforceLaneWithNoPoolFailsTheLane(t *testing.T) {
	t.Parallel()
	pol := policy.Policy{Fleet: &policy.FleetPolicy{Integration: &policy.FleetIntegrationPolicy{Stage: policy.FleetIntegrationEnforce}}}
	var warn strings.Builder
	got, err := shipIntegrationOf(pol, true, nil, 0, &warn)
	want := "fleet.integration.stage=enforce needs a pool id (--pool), and this fleet lane has none; start the lane through evolve loop"
	if got != nil || err == nil || err.Error() != want {
		t.Fatalf("got %+v, %v, want nil and the error %q", got, err, want)
	}
	if warn.Len() != 0 {
		t.Errorf("an enforce refusal wrote a warning: %q", warn.String())
	}
}

func TestShipIntegrationOf_ANoPoolEnforceLaneStopsAtStartupWithTheWholeText(t *testing.T) {
	t.Setenv(ipcenv.FleetKey, "1")
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"fleet":{"integration":{"stage":"enforce"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{})
	if deps.CloseSignals != nil {
		t.Cleanup(deps.CloseSignals)
	}
	want := "fleet.integration.stage=enforce needs a pool id (--pool), and this fleet lane has none; start the lane through evolve loop"
	if deps.RoutingErr == nil || deps.RoutingErr.Error() != want || deps.Orchestrator != nil {
		t.Fatalf("deps = err %v orchestrator %v, want no orchestrator and %q", deps.RoutingErr, deps.Orchestrator, want)
	}
}

func TestShipIntegrationOf_AWrongStageFailsTheLane(t *testing.T) {
	t.Parallel()
	pol := policy.Policy{Fleet: &policy.FleetPolicy{Integration: &policy.FleetIntegrationPolicy{Stage: "on"}}}
	got, err := shipIntegrationOf(pol, true, nil, 41, io.Discard)
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
