package swarm

// apicover_named_test.go — public-API coverage (ADR-0050 Phase 5).
//
// apicover flags an exported symbol UNCOVERED unless a _test.go in this package
// NAMES the identifier (and, for funcs/methods, also executes it >0%). The 14
// types below were exercised only INDIRECTLY by the existing suite (fakes
// implement the interfaces structurally; producers return the report structs)
// but their bare identifiers never appeared in test source, so apicover could
// not see them. Each test here NAMES the type via a meaningful use:
//   - interface seams: a compile-time `var _ Iface = concreteImpl` satisfaction
//     assertion, then a real method call through the interface variable;
//   - report/result structs: bind the value the REAL producer returns to a typed
//     variable and assert on its fields (no bare literals, no `_ = pkg.X`);
//   - the SessionStatus enum: asserted through the SessionRegistry it lives on.
//
// Rule 9: every assertion probes intent (a contract the type guarantees), not
// the mere existence of the symbol.

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

// ——— mergetrain.go interface seams ———

// TestGitMerger_SatisfiedByExecGitMerger pins the production GitMerger impl to
// the interface (compile-time) and proves the seam routes a Merge call through.
// ExecGitMerger.Merge against a non-existent worktree must fail wrapping
// ErrMergeConflict (its abort-on-failure contract) — that is the interface
// method actually doing work, not a no-op.
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

// TestAcceptanceChecker_GatesMerge binds a real AcceptanceChecker value and
// proves the merge-train honours its verdict: a checker that fails worker w1
// stops the train at w1 (acceptance gating is the "~80% fewer broken
// integrations" rule). Naming the AcceptanceChecker type on the var is what
// apicover keys on; the assertion proves the gate fires.
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

// TestConflictResolver_ResolvesAndRetries binds a real ConflictResolver value
// and proves the merge-train re-invokes it on a conflict, then retries the
// merge: the resolver "fixes" w1's branch so the second merge attempt lands and
// the outcome is marked Resolved.
func TestConflictResolver_ResolvesAndRetries(t *testing.T) {
	m := &scriptMerger{failBranch: map[string]bool{"cycle-1-w1": true}}
	var resolver ConflictResolver = func(_ context.Context, workerID, _ string) error {
		if workerID == "w1" {
			m.failBranch["cycle-1-w1"] = false // resolve so the retry merges
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

// TestMergeReport_ProducedByRunMergeTrain binds the MergeReport that the real
// producer (RunMergeTrain) returns to a typed variable and asserts its
// aggregate field: a clean two-worker train sets AllMerged and records one
// MergeOutcome per worker.
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

// ——— kill.go interface seams ———

// TestSessionKiller_SatisfiedByExecSessionKiller pins ExecSessionKiller to the
// SessionKiller interface and exercises the seam: a handle with a kill-able pgid
// and tmux session must drive both injected steps through the interface.
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

// TestProcessGroupKiller_InvokedByExecSessionKiller binds a real
// ProcessGroupKiller value and proves ExecSessionKiller calls it with the
// handle's pgid (the negative-pgid group-kill seam).
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

// TestTmuxKiller_SatisfiedByExecTmuxKill pins the production ExecTmuxKill to the
// TmuxKiller signature and exercises its SAFETY contract: an empty session name
// must be REFUSED (tmux resolves "" to the caller's own session — the killer-B
// suicide class), while a real name is accepted best-effort.
func TestTmuxKiller_SatisfiedByExecTmuxKill(t *testing.T) {
	var killer TmuxKiller = ExecTmuxKill
	if err := killer(context.Background(), ""); err == nil {
		t.Error("TmuxKiller (ExecTmuxKill) must refuse an empty session name")
	}
	// A real name is accepted (best-effort; tmux exit code ignored).
	if err := killer(context.Background(), "evolve-bridge-test"); err != nil {
		t.Errorf("TmuxKiller must accept a real session name best-effort, got %v", err)
	}
}

// TestReapReport_ProducedByReap binds the ReapReport that the real producer
// (Reap) returns and asserts its fields: reaping two live sessions where one
// killer errors records the kill in Killed and the failure in Errors.
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

// ——— reap_runsessions.go report struct ———

// TestReapRunReport_ProducedByReapRunSessions binds the ReapRunReport that the
// real producer (ReapRunSessions) returns and asserts its accounting: one
// evolve-bridge session is Killed, the empty + foreign names are Skipped.
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

// ——— provision.go interface seam ———

// TestWorkerProvisioner_SatisfiedByGitProvisioner pins the production
// provisioner (NewGitWorkerProvisioner) to the WorkerProvisioner interface and
// exercises a no-op method through it: Cleanup of an empty worktree path is the
// documented best-effort no-op (returns nil without shelling out to git).
func TestWorkerProvisioner_SatisfiedByGitProvisioner(t *testing.T) {
	var prov WorkerProvisioner = NewGitWorkerProvisioner(nil, "")
	if err := prov.Cleanup(context.Background(), t.TempDir(), ""); err != nil {
		t.Errorf("WorkerProvisioner.Cleanup of an empty worktree must be a nil no-op, got %v", err)
	}
}

// ——— types.go value types ———

// TestConflict_ProducedByValidate binds the Conflict value that the real
// producer (Validate) emits for an overlapping writer plan and asserts its
// fields: the contended file and both claiming worker IDs.
func TestConflict_ProducedByValidate(t *testing.T) {
	plan := writerPlan(
		fileWorker("w0", "go/internal/foo/a.go"),
		fileWorker("w1", "go/internal/foo/a.go"), // same file → one Conflict
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

// TestValidationResult_ProducedByValidate binds the ValidationResult that the
// real producer (Validate) returns for a disjoint writer plan and asserts its
// verdict fields: OK with no Collapse and a full merge order.
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

// ——— registry.go status enum ———

// TestSessionStatus_LifecycleThroughRegistry names the SessionStatus type and
// both its values (StatusLive / StatusReaped) and asserts the registry drives
// the lifecycle: a freshly Registered handle is StatusLive; after MarkReaped its
// persisted status is StatusReaped.
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

// TestSessionRegistry_TypeNamedAndExercised names the SessionRegistry type on a
// typed variable (apicover keys on the named identifier) and exercises a
// round-trip: Register then Live reflects exactly the live work-list.
func TestSessionRegistry_TypeNamedAndExercised(t *testing.T) {
	var reg *SessionRegistry = NewSessionRegistry(filepath.Join(t.TempDir(), "s.json"), 1, "build", 1)
	_ = reg.Register(handle("w0"))
	_ = reg.Register(handle("w1"))
	if got := reg.Live(); len(got) != 2 {
		t.Errorf("SessionRegistry.Live must return both registered sessions, got %d", len(got))
	}
}
