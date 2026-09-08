//go:build integration

package checkpoint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

type interruptedResumeRunner struct {
	phase  string
	cancel context.CancelFunc
	calls  int
}

func (r *interruptedResumeRunner) Name() string { return r.phase }
func (r *interruptedResumeRunner) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	r.calls++
	if r.cancel != nil {
		r.cancel()
		return core.PhaseResponse{}, context.Canceled
	}
	return core.PhaseResponse{Phase: r.phase, Verdict: core.VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func TestResumeProgress_SecondInterruptionRetainsCurrentPhase(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	ws := core.RunWorkspacePath(root, 7)
	if err := os.MkdirAll(ws, 0755); err != nil {
		t.Fatal(err)
	}
	store := storage.New(evolveDir)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cs := core.CycleState{CycleID: 7, Phase: "build", WorkspacePath: ws, RunID: "original-run", CompletedPhases: []string{"scout", "triage", "tdd"}}
	if err := store.WriteState(ctx, core.State{LastCycleNumber: 6, LastAllocatedCycleNumber: 7}); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteCycleState(ctx, cs); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(evolveDir, core.CycleStateFile)
	if err := ApplyToStateFile(path, Compose(cs, ReasonQuotaLikely, 0, "", time.Now())); err != nil {
		t.Fatal(err)
	}
	build := &interruptedResumeRunner{phase: "build"}
	audit := &interruptedResumeRunner{phase: "audit", cancel: cancel}
	ship := &interruptedResumeRunner{phase: "ship"}
	o := core.NewOrchestrator(store, &fixtures.FakeLedger{}, map[core.Phase]core.PhaseRunner{core.PhaseBuild: build, core.PhaseAudit: audit, core.PhaseShip: ship})
	rp, err := core.LoadResumeState(ctx, root, evolveDir, core.ResumeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = o.RunCycleFromPhase(ctx, core.CycleRequest{ProjectRoot: root, GoalHash: "original-goal"}, rp); !errors.Is(err, context.Canceled) {
		t.Fatalf("want interruption: %v", err)
	}
	next, err := core.LoadResumeState(context.Background(), root, evolveDir, core.ResumeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if next.Phase != "audit" || next.CycleID != 7 {
		t.Fatalf("second pause resumes stale progress: %+v", next)
	}
	if build.calls != 1 || ship.calls != 0 {
		t.Fatalf("unexpected dispatches build=%d ship=%d", build.calls, ship.calls)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var blob map[string]any
	if err := json.Unmarshal(raw, &blob); err != nil {
		t.Fatal(err)
	}
	if blob["run_id"] != "original-run" {
		t.Fatalf("checkpoint changed run identity: %v", blob["run_id"])
	}
	audit.cancel = nil
	if _, err := o.RunCycleFromPhase(context.Background(), core.CycleRequest{ProjectRoot: root, GoalHash: "original-goal"}, next); err != nil {
		t.Fatalf("second resume: %v", err)
	}
	if build.calls != 1 || audit.calls != 2 || ship.calls != 1 {
		t.Fatalf("second resume replayed completed work: build=%d audit=%d ship=%d", build.calls, audit.calls, ship.calls)
	}
	if _, err := core.LoadResumeState(context.Background(), root, evolveDir, core.ResumeOptions{}); !errors.Is(err, core.ErrNoCheckpoint) {
		t.Fatalf("completed checkpoint can be resurrected: %v", err)
	}

}
