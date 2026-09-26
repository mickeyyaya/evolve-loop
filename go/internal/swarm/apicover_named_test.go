package swarm

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func TestGitMerger_SatisfiedByExecGitMerger(t *testing.T) {
	var merger GitMerger = ExecGitMerger{IntegrationWorktree: filepath.Join(t.TempDir(), "no-such-worktree")}
	err := merger.Merge(context.Background(), "cycle-1-integration", "cycle-1-w0")
	if err == nil {
		t.Fatal("Merge into a missing worktree must fail (not silently succeed)")
	}
	if !errors.Is(err, ErrMergeConflict) {
		t.Errorf("GitMerger failure must wrap ErrMergeConflict, got %v", err)
	}
}

func TestAcceptanceChecker_GatesMerge(t *testing.T) {
	var accept AcceptanceChecker = func(_ context.Context, workerID, _ string) error {
		if workerID == "w1" {
			return errors.New("go test failed")
		}
		return nil
	}
	rep := RunMergeTrain(context.Background(), "integ",
		[]string{"w0", "w1"}, branchMap("w0", "w1"),
		MergeTrainDeps{Merger: &scriptMerger{}, Accept: accept})
	if rep.AllMerged {
		t.Fatal("AcceptanceChecker failure must stop the train")
	}
	if len(rep.Outcomes) != 2 || rep.Outcomes[1].WorkerID != "w1" || rep.Outcomes[1].Merged {
		t.Errorf("w0 should merge then w1 fail the acceptance gate: %+v", rep.Outcomes)
	}
}

func TestConflictResolver_ResolvesAndRetries(t *testing.T) {
	m := &scriptMerger{failBranch: map[string]bool{"cycle-1-w1": true}}
	var resolver ConflictResolver = func(_ context.Context, workerID, _ string) error {
		if workerID == "w1" {
			m.failBranch["cycle-1-w1"] = false
		}
		return nil
	}
	rep := RunMergeTrain(context.Background(), "integ",
		[]string{"w0", "w1"}, branchMap("w0", "w1"),
		MergeTrainDeps{Merger: m, Resolver: resolver, MaxRetries: 1})
	if !rep.AllMerged {
		t.Fatalf("ConflictResolver should let the retry merge: %+v", rep.Outcomes)
	}
	if !rep.Outcomes[1].Resolved {
		t.Errorf("w1 must be marked Resolved after a successful resolver retry: %+v", rep.Outcomes[1])
	}
}

func TestMergeReport_ProducedByRunMergeTrain(t *testing.T) {
	var rep MergeReport = RunMergeTrain(context.Background(), "cycle-1-integration",
		[]string{"w0", "w1"}, branchMap("w0", "w1"), MergeTrainDeps{Merger: &scriptMerger{}})
	if !rep.AllMerged {
		t.Errorf("MergeReport.AllMerged must be true for a clean train: %+v", rep)
	}
	if len(rep.Outcomes) != 2 {
		t.Errorf("MergeReport must record one outcome per worker, got %d", len(rep.Outcomes))
	}
}

func TestSessionKiller_SatisfiedByExecSessionKiller(t *testing.T) {
	var gotPGID int
	var gotTmux string
	var killer SessionKiller = ExecSessionKiller{
		KillGroup: func(pgid int) error { gotPGID = pgid; return nil },
		KillTmux:  func(_ context.Context, s string) error { gotTmux = s; return nil },
	}
	if err := killer.Kill(context.Background(), SessionHandle{WorkerID: "w0", PGID: 4242, TmuxSession: "sess-w0"}); err != nil {
		t.Fatal(err)
	}
	if gotPGID != 4242 || gotTmux != "sess-w0" {
		t.Errorf("SessionKiller.Kill must drive both steps, got pgid=%d tmux=%q", gotPGID, gotTmux)
	}
}

func TestProcessGroupKiller_InvokedByExecSessionKiller(t *testing.T) {
	var got int
	var pgKiller ProcessGroupKiller = func(pgid int) error { got = pgid; return nil }
	k := ExecSessionKiller{KillGroup: pgKiller}
	if err := k.Kill(context.Background(), SessionHandle{PGID: 7777}); err != nil {
		t.Fatal(err)
	}
	if got != 7777 {
		t.Errorf("ProcessGroupKiller must receive the handle pgid, got %d", got)
	}
}

func TestTmuxKiller_SatisfiedByExecTmuxKill(t *testing.T) {
	var killer TmuxKiller = ExecTmuxKill
	if err := killer(context.Background(), ""); err == nil {
		t.Error("TmuxKiller (ExecTmuxKill) must refuse an empty session name")
	}
	if err := killer(context.Background(), "evolve-bridge-test"); err != nil {
		t.Errorf("TmuxKiller must accept a real session name best-effort, got %v", err)
	}
}

func TestReapReport_ProducedByReap(t *testing.T) {
	reg := NewSessionRegistry(filepath.Join(t.TempDir(), "s.json"), 1, "build", 1)
	_ = reg.Register(handle("w0"))
	_ = reg.Register(handle("w1"))
	var rep ReapReport = Reap(context.Background(), reg, &fakeKiller{failOn: map[string]bool{"w0": true}})
	if len(rep.Killed) != 2 {
		t.Errorf("ReapReport.Killed must list both reaped workers, got %v", rep.Killed)
	}
	if len(rep.Errors) != 1 {
		t.Errorf("ReapReport.Errors must record the w0 kill failure, got %v", rep.Errors)
	}
}

func TestReapRunReport_ProducedByReapRunSessions(t *testing.T) {
	dir := t.TempDir()
	path := writeRegistry(t, dir, "evolve-bridge-rZZZZ9999-c1-build-pid1-1")
	var rep ReapRunReport = ReapRunSessions(context.Background(), path, func(context.Context, string) error { return nil })
	if rep.Killed != 1 {
		t.Errorf("ReapRunReport.Killed must count the evolve-bridge session, got %d", rep.Killed)
	}
	if rep.Errors != 0 {
		t.Errorf("ReapRunReport.Errors must be 0 for a clean reap, got %d", rep.Errors)
	}
}

func TestWorkerProvisioner_SatisfiedByGitProvisioner(t *testing.T) {
	var prov WorkerProvisioner = NewGitWorkerProvisioner(nil, "")
	if err := prov.Cleanup(context.Background(), t.TempDir(), ""); err != nil {
		t.Errorf("WorkerProvisioner.Cleanup of an empty worktree must be a nil no-op, got %v", err)
	}
}

func TestConflict_ProducedByValidate(t *testing.T) {
	plan := writerPlan(
		fileWorker("w0", "go/internal/foo/a.go"),
		fileWorker("w1", "go/internal/foo/a.go"),
	)
	res := Validate(plan)
	if len(res.Conflicts) != 1 {
		t.Fatalf("overlapping writer plan must yield one Conflict, got %+v", res.Conflicts)
	}
	var c Conflict = res.Conflicts[0]
	if c.File != "go/internal/foo/a.go" {
		t.Errorf("Conflict.File must name the contended path, got %q", c.File)
	}
	if len(c.Workers) != 2 {
		t.Errorf("Conflict.Workers must name both claimants, got %v", c.Workers)
	}
}

func TestValidationResult_ProducedByValidate(t *testing.T) {
	plan := writerPlan(
		fileWorker("w0", "go/internal/foo/a.go"),
		fileWorker("w1", "go/internal/bar/b.go"),
	)
	var res ValidationResult = Validate(plan)
	if !res.OK || res.Collapse {
		t.Fatalf("disjoint writer plan must be OK without collapse: %+v", res)
	}
	if len(res.MergeOrder) != 2 {
		t.Errorf("ValidationResult.MergeOrder must serialize both workers, got %v", res.MergeOrder)
	}
}

func TestSessionStatus_LifecycleThroughRegistry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	reg := NewSessionRegistry(path, 1, "build", 1)
	_ = reg.Register(handle("w0"))

	var live SessionStatus = reg.Snapshot()[0].Status
	if live != StatusLive {
		t.Errorf("a registered session must be StatusLive, got %q", live)
	}

	if err := reg.MarkReaped("w0"); err != nil {
		t.Fatal(err)
	}
	_, _, _, sessions, _ := LoadManifest(path)
	var reaped SessionStatus = sessions[0].Status
	if reaped != StatusReaped {
		t.Errorf("a reaped session must persist as StatusReaped, got %q", reaped)
	}
}

func TestSessionRegistry_TypeNamedAndExercised(t *testing.T) {
	var reg *SessionRegistry = NewSessionRegistry(filepath.Join(t.TempDir(), "s.json"), 1, "build", 1)
	_ = reg.Register(handle("w0"))
	_ = reg.Register(handle("w1"))
	if got := reg.Live(); len(got) != 2 {
		t.Errorf("SessionRegistry.Live must return both registered sessions, got %d", len(got))
	}
}

func TestSwarmResult_TotalTokens(t *testing.T) {
	s := SwarmResult{Workers: []WorkerResult{
		{Tokens: cyclestate.TokenUsage{Input: 10, Output: 3, CacheRead: 7, CacheWrite: 2}},
		{Tokens: cyclestate.TokenUsage{Input: 5, Output: 4, CacheRead: 1, CacheWrite: 0}},
	}}
	got := s.TotalTokens()
	want := cyclestate.TokenUsage{Input: 15, Output: 7, CacheRead: 8, CacheWrite: 2}
	if got != want {
		t.Errorf("SwarmResult.TotalTokens = %+v, want %+v (field-wise sum across workers)", got, want)
	}
	if zero := (SwarmResult{}).TotalTokens(); zero != (cyclestate.TokenUsage{}) {
		t.Errorf("SwarmResult.TotalTokens on zero workers = %+v, want zero usage", zero)
	}
}
