package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const parityStateWithEveryLifecycle = `{"failedApproaches":[` +
	`{"cycle":11,"classification":"infrastructure-systemic","summary":"live infra","recordedAt":"2026-10-01T00:00:00Z","expiresAt":"2099-01-01T00:00:00Z"},` +
	`{"cycle":12,"classification":"code-build-fail","summary":"live build","recordedAt":"2026-10-02T00:00:00Z","expiresAt":"2099-01-01T00:00:00Z"},` +
	`{"cycle":13,"classification":"ship-gate-config","summary":"live gate","recordedAt":"2026-10-03T00:00:00Z","expiresAt":"2099-01-01T00:00:00Z"},` +
	`{"cycle":14,"classification":"code-audit-fail","summary":"expired audit","recordedAt":"2020-01-01T00:00:00Z","expiresAt":"2020-02-01T00:00:00Z"},` +
	`{"cycle":15,"classification":"infrastructure-transient","summary":"expired legacy","recordedAt":"2020-01-01T00:00:00Z"},` +
	`{"cycle":16,"classification":"code-audit-fail","summary":"untimed legacy"}],` +
	`"carryoverTodos":[` +
	`{"id":"todo-expired","cycles_unpicked":2,"expiresAt":"2020-02-01T00:00:00Z"},` +
	`{"id":"todo-live","cycles_unpicked":1,"expiresAt":"2099-01-01T00:00:00Z"},` +
	`{"id":"todo-untimed","cycles_unpicked":0}]}`

type parityState struct {
	FailedApproaches []map[string]any `json:"failedApproaches"`
	CarryoverTodos   []map[string]any `json:"carryoverTodos"`
}

func parityProject(t *testing.T) (root, evolveDir string) {
	t.Helper()
	root = t.TempDir()
	evolveDir = filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "state.json"), []byte(parityStateWithEveryLifecycle), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, evolveDir
}

func readParityState(t *testing.T, evolveDir string) parityState {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(evolveDir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var st parityState
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("state.json: %v (%q)", err, raw)
	}
	return st
}

func parityFailedCycles(st parityState) []float64 {
	cycles := make([]float64, 0, len(st.FailedApproaches))
	for _, e := range st.FailedApproaches {
		c, _ := e["cycle"].(float64)
		cycles = append(cycles, c)
	}
	return cycles
}

func parityCarryover(st parityState) map[string]map[string]any {
	byID := make(map[string]map[string]any, len(st.CarryoverTodos))
	for _, e := range st.CarryoverTodos {
		id, _ := e["id"].(string)
		byID[id] = e
	}
	return byID
}

func parityResolvedBy(t *testing.T, evolveDir, fingerprint string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(evolveDir, "resolved-fingerprints.json"))
	if err != nil {
		t.Fatalf("resolved-fingerprints.json: %v", err)
	}
	var records []core.ResolvedFingerprint
	if err := json.Unmarshal(raw, &records); err != nil {
		t.Fatalf("resolved-fingerprints.json: %v (%q)", err, raw)
	}
	var by []string
	for _, r := range records {
		if r.Fingerprint == fingerprint {
			by = append(by, r.ResolvedBy)
		}
	}
	return by
}

func TestFailuresResetMatchesTheLoopReset(t *testing.T) {
	const fingerprint = "fp-loop-parity"
	_, loopEvolveDir := parityProject(t)
	verbRoot, verbEvolveDir := parityProject(t)

	var loopStderr bytes.Buffer
	maintainBatchState(loopConfig{EvolveDir: loopEvolveDir, Reset: true, Fingerprint: fingerprint}, false, &loopStderr)
	var stdout, stderr bytes.Buffer
	code := runFailures([]string{"reset", "--fingerprint", fingerprint, "--project-root", verbRoot}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("failures reset rc=%d want 0; stderr=%q", code, stderr.String())
	}
	loopState, verbState := readParityState(t, loopEvolveDir), readParityState(t, verbEvolveDir)
	if got, want := parityFailedCycles(verbState), parityFailedCycles(loopState); !reflect.DeepEqual(got, want) {
		t.Errorf("failures reset kept cycles %v, loop --reset kept %v: the verb must perform the loop's prune", got, want)
	}
	if got := parityFailedCycles(loopState); !reflect.DeepEqual(got, []float64{12, 14, 16}) {
		t.Errorf("loop --reset kept cycles %v, want [12 14 16] (infrastructure + ship-gate-config pruned); stderr=%q", got, loopStderr.String())
	}
	for _, side := range []struct{ name, evolveDir string }{{"loop --reset", loopEvolveDir}, {"failures reset", verbEvolveDir}} {
		if by := parityResolvedBy(t, side.evolveDir, fingerprint); !reflect.DeepEqual(by, []string{"operator-reset"}) {
			t.Errorf("%s recorded %q resolved_by %v, want exactly [operator-reset]", side.name, fingerprint, by)
		}
	}
}

func TestFailuresPruneMatchesTheLaunchPruneWithoutCarryoverBookkeeping(t *testing.T) {
	_, loopEvolveDir := parityProject(t)
	verbRoot, verbEvolveDir := parityProject(t)
	seeded := parityCarryover(readParityState(t, verbEvolveDir))

	var loopStderr bytes.Buffer
	maintainBatchState(loopConfig{EvolveDir: loopEvolveDir}, true, &loopStderr)
	var stdout, stderr bytes.Buffer
	code := runFailures([]string{"prune", "--project-root", verbRoot}, nil, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("failures prune rc=%d want 0; stderr=%q", code, stderr.String())
	}
	loopState, verbState := readParityState(t, loopEvolveDir), readParityState(t, verbEvolveDir)
	if got, want := parityFailedCycles(verbState), parityFailedCycles(loopState); !reflect.DeepEqual(got, want) {
		t.Errorf("failures prune kept failedApproaches cycles %v, the launch-time prune kept %v", got, want)
	}
	if got := parityFailedCycles(verbState); !reflect.DeepEqual(got, []float64{11, 12, 13, 16}) {
		t.Errorf("failures prune kept failedApproaches cycles %v, want [11 12 13 16] (expired and legacy-expired removed)", got)
	}
	loopTodos, verbTodos := parityCarryover(loopState), parityCarryover(verbState)
	if len(verbTodos) != len(loopTodos) {
		t.Errorf("failures prune kept carryoverTodos %v, the launch-time prune kept %v", verbTodos, loopTodos)
	}
	for id := range loopTodos {
		if _, ok := verbTodos[id]; !ok {
			t.Errorf("carryoverTodos %q survived the launch-time prune but not failures prune", id)
		}
	}
	if _, ok := verbTodos["todo-expired"]; ok {
		t.Errorf("failures prune kept the expired carryoverTodos entry todo-expired: %v", verbTodos)
	}
	for _, id := range []string{"todo-live", "todo-untimed"} {
		if !reflect.DeepEqual(verbTodos[id], seeded[id]) {
			t.Errorf("failures prune changed carryoverTodos %q from %v to %v: cycles_unpicked and expiresAt are per-batch bookkeeping", id, seeded[id], verbTodos[id])
		}
		if seeded[id]["cycles_unpicked"] == loopTodos[id]["cycles_unpicked"] {
			t.Errorf("the launch-time prune left cycles_unpicked on %q at %v: the loop must keep its per-batch increment", id, loopTodos[id]["cycles_unpicked"])
		}
	}
}
