package routingtest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// FakeStorage is an in-memory core.Storage for the orchestrator surface.
type FakeStorage struct {
	state core.State
	cs    core.CycleState
}

func (f *FakeStorage) ReadState(context.Context) (core.State, error)      { return f.state, nil }
func (f *FakeStorage) WriteState(_ context.Context, s core.State) error   { f.state = s; return nil }
func (f *FakeStorage) ReadCycleState(context.Context) (core.CycleState, error) { return f.cs, nil }
func (f *FakeStorage) WriteCycleState(_ context.Context, cs core.CycleState) error {
	f.cs = cs
	return nil
}
func (f *FakeStorage) AcquireLock(context.Context) (func() error, error) {
	return func() error { return nil }, nil
}

// FakeLedger records appended entries.
type FakeLedger struct{ entries []core.LedgerEntry }

func (f *FakeLedger) Append(_ context.Context, e core.LedgerEntry) error {
	f.entries = append(f.entries, e)
	return nil
}
func (f *FakeLedger) Verify(context.Context) error { return nil }
func (f *FakeLedger) Iter(context.Context) (core.LedgerIterator, error) {
	return nil, errors.New("routingtest: ledger Iter unused")
}

// FakeRunner is a no-LLM phase runner returning a canned verdict.
type FakeRunner struct {
	name    string
	verdict string
	calls   int
}

func (f *FakeRunner) Name() string { return f.name }
func (f *FakeRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	f.calls++
	v := f.verdict
	if v == "" {
		v = core.VerdictPASS
	}
	return core.PhaseResponse{Phase: f.name, Verdict: v, ArtifactsDir: req.Workspace}, nil
}

// orchestratorPhases is the runnable set the orchestrator may sequence.
var orchestratorPhases = []core.Phase{
	core.PhaseIntent, core.PhaseScout, core.PhaseTriage, core.PhaseTDD,
	core.PhaseBuildPlanner, core.PhaseBuild, core.PhaseAudit, core.PhaseShip, core.PhaseRetro,
}

func buildRunners(verdicts map[string]string) map[core.Phase]core.PhaseRunner {
	out := make(map[core.Phase]core.PhaseRunner, len(orchestratorPhases))
	for _, p := range orchestratorPhases {
		out[p] = &FakeRunner{name: string(p), verdict: verdicts[string(p)]}
	}
	return out
}

// failedRecords converts FailedRecordSpec into core.FailedRecord with a
// non-expired RecordedAt so the retro failure-adapter sees them as active.
func failedRecords(specs []FailedRecordSpec) []core.FailedRecord {
	out := make([]core.FailedRecord, 0, len(specs))
	for i, s := range specs {
		out = append(out, core.FailedRecord{
			Cycle:          i + 1,
			Verdict:        s.Verdict,
			Classification: s.Classification,
			RecordedAt:     nonExpiredRecordedAt,
		})
	}
	return out
}

// seedWorkspace writes handoff files into the cycle workspace and returns it.
func seedWorkspace(t *testing.T, projectRoot string, cycle int, files map[string]string) string {
	t.Helper()
	ws := filepath.Join(projectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}
	return ws
}

// readRoutingDecisions parses every routing-decision-*.json in the workspace.
func readRoutingDecisions(t *testing.T, workspace string) []router.RouterDecision {
	t.Helper()
	paths, _ := filepath.Glob(filepath.Join(workspace, "routing-decision-*.json"))
	out := make([]router.RouterDecision, 0, len(paths))
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		var d router.RouterDecision
		if err := json.Unmarshal(raw, &d); err != nil {
			t.Fatalf("unmarshal %s: %v", p, err)
		}
		out = append(out, d)
	}
	return out
}
