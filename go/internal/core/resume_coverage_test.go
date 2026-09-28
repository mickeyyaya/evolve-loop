//go:build integration

package core

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCycleFromPhase_NilResumePoint(t *testing.T) {
	t.Parallel()
	o := mustBuildOrchestrator(t)
	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{}, nil)
	if err == nil {
		t.Errorf("expected nil-resumePoint error")
	}
}

func TestRunCycleFromPhase_InvalidPhase(t *testing.T) {
	t.Parallel()
	o := mustBuildOrchestrator(t)
	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{},
		&ResumePoint{Phase: "bogus-phase"})
	if err == nil {
		t.Errorf("expected invalid-phase error")
	}
}

func TestRunCycleFromPhase_PhaseEndInvalid(t *testing.T) {
	t.Parallel()
	o := mustBuildOrchestrator(t)
	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{},
		&ResumePoint{Phase: string(PhaseEnd)})
	if err == nil {
		t.Errorf("expected end-phase rejection")
	}
}

func TestRunCycleFromPhase_HappyPath(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{
		state:      State{LastCycleNumber: 5},
		cycleState: CycleState{CycleID: 5, WorkspacePath: "/tmp/ws"},
	}
	ldgr := &fakeLedger{}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, ldgr, runners)
	res, err := o.RunCycleFromPhase(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(),
	}, &ResumePoint{Phase: string(PhaseBuild), CycleID: 5})
	if err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}
	if res.Cycle != 5 {
		t.Errorf("Cycle=%d want 5", res.Cycle)
	}
	if len(res.PhasesRun) == 0 {
		t.Errorf("no phases ran")
	}
	if res.FinalVerdict != VerdictPASS {
		t.Errorf("verdict=%q", res.FinalVerdict)
	}
}

func TestRunCycleFromPhase_RejectsCheckpointIdentityMismatch(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state CycleState
		point ResumePoint
	}{
		{
			name:  "cycle",
			state: CycleState{CycleID: 8, WorkspacePath: "/tmp/ws"},
			point: ResumePoint{Phase: string(PhaseAudit), CycleID: 7},
		},
		{
			name:  "worktree",
			state: CycleState{CycleID: 7, WorkspacePath: "/tmp/ws", ActiveWorktree: "/tmp/other"},
			point: ResumePoint{Phase: string(PhaseAudit), CycleID: 7, WorktreePath: "/tmp/sealed"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storage := &fakeStorage{cycleState: tc.state}
			o := NewOrchestrator(storage, &fakeLedger{}, buildRunners(nil))
			_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: t.TempDir()}, &tc.point)
			if err == nil || !strings.Contains(err.Error(), "resume identity mismatch") {
				t.Fatalf("mismatched %s identity error=%v", tc.name, err)
			}
		})
	}
}

func TestRunCycleFromPhase_InsertedPhaseInRunnersAccepted(t *testing.T) {
	t.Parallel()
	const inserted = Phase("mutation-gate") // registered at runtime, not spine-valid
	if inserted.IsValid() {
		t.Fatalf("test premise broken: %q must NOT be a spine-valid phase", inserted)
	}
	st := &fakeStorage{
		state:      State{LastCycleNumber: 5},
		cycleState: CycleState{CycleID: 5, WorkspacePath: "/tmp/ws"},
	}
	runners := buildRunners(nil)
	insertedRunner := &fakeRunner{name: string(inserted)}
	runners[inserted] = insertedRunner
	o := NewOrchestrator(st, &fakeLedger{}, runners)

	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(),
	}, &ResumePoint{Phase: string(inserted), CycleID: 5})

	if insertedRunner.calls == 0 {
		t.Fatalf("RED: inserted phase %q in o.runners was rejected by the resume guard "+
			"(runner never dispatched); RunCycleFromPhase err=%v", inserted, err)
	}
	if err != nil && strings.Contains(err.Error(), "invalid resume phase") {
		t.Errorf("RED: guard returned invalid-resume-phase for an in-runners phase: %v", err)
	}
}

func TestRunCycleFromPhase_PhaseStartRejected(t *testing.T) {
	t.Parallel()
	o := mustBuildOrchestrator(t)
	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{},
		&ResumePoint{Phase: string(PhaseStart)})
	if err == nil {
		t.Errorf("RED/REGRESSION: PhaseStart must be rejected as a resume target")
	}
}

func TestRunCycleFromPhase_MissingRunner(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{
		state:      State{LastCycleNumber: 5},
		cycleState: CycleState{CycleID: 5, WorkspacePath: "/tmp/ws"},
	}
	// Only register PhaseRetro; resume requests PhaseBuild → missing.
	runners := map[Phase]PhaseRunner{
		PhaseRetro: &fakeRunner{name: string(PhaseRetro)},
	}
	o := NewOrchestrator(st, &fakeLedger{}, runners)
	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(),
	}, &ResumePoint{Phase: string(PhaseBuild), CycleID: 5})
	if err == nil {
		t.Errorf("expected missing-runner error")
	}
}

func TestRunCycleFromPhase_LedgerError(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{
		state:      State{LastCycleNumber: 5},
		cycleState: CycleState{CycleID: 5, WorkspacePath: "/tmp/ws"},
	}
	ldgr := &fakeLedger{failOnAppend: true}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, ldgr, runners)
	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(),
	}, &ResumePoint{Phase: string(PhaseBuild), CycleID: 5})
	if err == nil {
		t.Errorf("expected ledger error")
	}
}

func TestDefaultCurrentHead_RealRepo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@e.com"},
		{"config", "user.name", "t"},
		{"commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	head, err := defaultCurrentHead(dir)
	if err != nil {
		t.Fatalf("defaultCurrentHead: %v", err)
	}
	if len(head) < 40 {
		t.Errorf("head too short: %q", head)
	}
}

func TestDefaultCurrentHead_NotARepo(t *testing.T) {
	t.Parallel()
	if _, err := defaultCurrentHead(t.TempDir()); err == nil {
		t.Errorf("expected error for non-git dir")
	}
}

func TestDefaultPathExists(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	if !defaultPathExists(tmp) {
		t.Errorf("expected true for existing dir")
	}
	if defaultPathExists(filepath.Join(tmp, "no-such-file")) {
		t.Errorf("expected false for missing path")
	}
}

func TestIntFromAny_AllTypes(t *testing.T) {
	t.Parallel()
	if intFromAny(float64(42)) != 42 {
		t.Error("float64")
	}
	if intFromAny(int(7)) != 7 {
		t.Error("int")
	}
	if intFromAny("hello") != 0 {
		t.Error("string default")
	}
	if intFromAny(nil) != 0 {
		t.Error("nil default")
	}
}

func TestFloatFromAny_AllTypes(t *testing.T) {
	t.Parallel()
	if floatFromAny(float64(1.5)) != 1.5 {
		t.Error("float64")
	}
	if floatFromAny(int(3)) != 3.0 {
		t.Error("int")
	}
	if floatFromAny("bad") != 0 {
		t.Error("string default")
	}
	if floatFromAny(nil) != 0 {
		t.Error("nil default")
	}
}

func TestStrFromAny_Wrong(t *testing.T) {
	t.Parallel()
	if got := strFromAny(42); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
	if got := strFromAny("hello"); got != "hello" {
		t.Errorf("expected hello, got %q", got)
	}
}

func TestDecideAfterRetro_AllBranches(t *testing.T) {
	t.Parallel()
	o := mustBuildOrchestrator(t)
	branch, _, _, _ := o.decideAfterRetro(CycleState{}, VerdictPASS, nil)
	_ = branch
	branch, _, _, _ = o.decideAfterRetro(CycleState{}, VerdictFAIL, nil)
	_ = branch
	branch, _, _, _ = o.decideAfterRetro(CycleState{}, VerdictWARN, nil)
	_ = branch
}

func TestLoadResumeState_InvalidJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	evolveDir := filepath.Join(dir, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	bad := filepath.Join(evolveDir, "cycle-state.json")
	if err := os.WriteFile(bad, []byte("not-json{"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := LoadResumeState(context.Background(), dir, evolveDir, ResumeOptions{
		CurrentHead: func(_ string) (string, error) { return "abc", nil },
		PathExists:  func(_ string) bool { return true },
	})
	if err == nil {
		t.Errorf("expected JSON parse error")
	}
}

// mustBuildOrchestrator constructs a default orchestrator with all-PASS
// runners for tests that don't need precise control.
func mustBuildOrchestrator(t *testing.T) *Orchestrator {
	t.Helper()
	st := &fakeStorage{
		state:      State{LastCycleNumber: 0},
		cycleState: CycleState{},
	}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))
	return o
}

var _ = errors.New
