package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/pkg/version"
)

type cycleStateReadFailStorage struct{ *fakeStorage }

func (cycleStateReadFailStorage) ReadCycleState(context.Context) (CycleState, error) {
	return CycleState{}, errors.New("cycle-state unreadable")
}

func TestApplyFailureDecisionFloor_JustificationStandsInForMissingEvidence(t *testing.T) {
	t.Parallel()

	o := floorOrchestrator(router.StaticPreset{})
	cases := []struct {
		name     string
		decision string
		want     string
	}{
		{"justification only", `{"category":"infra-systemic","level":"system","justification":"every CLI family exhausted","action":"halt-and-diagnose"}`, "orchestrator-classified infra-systemic: every CLI family exhausted"},
		{"evidence wins over justification", `{"category":"infra-systemic","level":"system","evidence":"E","justification":"J","action":"halt-and-diagnose"}`, "orchestrator-classified infra-systemic: E"},
		{"neither leaves an empty tail", `{"category":"infra-systemic","level":"system","action":"halt-and-diagnose"}`, "orchestrator-classified infra-systemic: "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeAuditWithFailure(t, dir, "FAIL", "code-audit-fail", "a lost write in shared state")
			writeDecision(t, dir, tc.decision)

			sig := o.applyFailureDecisionFloor(CycleState{CycleID: 7, WorkspacePath: dir}, VerdictFAIL)

			if sig == nil || !sig.Halt || sig.Category != policy.CategoryInfraSystemic || sig.Level != policy.LevelSystem {
				t.Fatalf("sig = %+v, want a halting infra-systemic system signal", sig)
			}
			if sig.Evidence != tc.want {
				t.Errorf("Evidence = %q, want %q", sig.Evidence, tc.want)
			}
		})
	}
}

func TestDefaultPostBuildRepinProvenance_VerifiesAncestryAgainstHEAD(t *testing.T) {
	t.Parallel()

	r := gittest.Fixture(t)
	for _, body := range []string{"one", "two"} {
		if err := os.WriteFile(filepath.Join(r.Dir, "f.txt"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		r.Git("add", "f.txt")
		r.Git("commit", "-q", "-m", body)
	}
	parent := r.Git("rev-parse", "HEAD~1")
	r.Git("checkout", "-q", "-b", "side", parent)
	if err := os.WriteFile(filepath.Join(r.Dir, "g.txt"), []byte("side"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.Git("add", "g.txt")
	r.Git("commit", "-q", "-m", "side")
	sideTip := r.Git("rev-parse", "HEAD")
	r.Git("checkout", "-q", "main")

	commit, verified := defaultPostBuildRepinProvenance(r.Dir)

	if commit != version.Commit() {
		t.Errorf("commit = %q, want the binary's embedded commit %q", commit, version.Commit())
	}
	cases := []struct {
		name string
		sha  string
		want bool
	}{
		{"empty commit is unverifiable", "", false},
		{"ancestor of HEAD", parent, true},
		{"HEAD itself", "HEAD", true},
		{"commit off another branch", sideTip, false},
		{"unknown commit", "0123456789abcdef0123456789abcdef01234567", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := verified(tc.sha); got != tc.want {
				t.Errorf("verified(%q) = %v, want %v", tc.sha, got, tc.want)
			}
		})
	}
}

func TestLoadResumeBootstrap_ReadCycleStateFailureIsWrapped(t *testing.T) {
	t.Parallel()

	o := NewOrchestrator(cycleStateReadFailStorage{&fakeStorage{}}, &fakeLedger{}, buildRunners(nil))

	_, err := o.loadResumeBootstrap(context.Background(), CycleRequest{ProjectRoot: t.TempDir()}, &ResumePoint{CycleID: 3}, PhaseAudit)

	if err == nil || err.Error() != "read cycle-state: cycle-state unreadable" {
		t.Fatalf("err = %v, want read cycle-state: cycle-state unreadable", err)
	}
}

func TestLoadResumeBootstrap_FleetCheckpointForAnotherWorkspaceIsRejected(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	declared := filepath.Join(root, ".evolve", "runs", "cycle-5")
	other := filepath.Join(root, ".evolve", "runs", "cycle-6")
	storage := &fakeStorage{cycleState: CycleState{CycleID: 5, WorkspacePath: declared}}
	o := NewOrchestrator(storage, &fakeLedger{}, buildRunners(nil))

	_, err := o.loadResumeBootstrap(context.Background(), CycleRequest{ProjectRoot: root},
		&ResumePoint{CycleID: 5, StatePath: filepath.Join(other, CycleStateFile)}, PhaseAudit)

	if err == nil || !strings.Contains(err.Error(), "fleet checkpoint path does not match declared workspace") {
		t.Fatalf("err = %v, want the fleet identity mismatch", err)
	}
}

func TestLoadResumeBootstrap_CheckpointCycleFillsAnUnnumberedHost(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	storage := &fakeStorage{cycleState: CycleState{WorkspacePath: filepath.Join(root, "ws")}}
	o := NewOrchestrator(storage, &fakeLedger{}, buildRunners(nil))

	boot, err := o.loadResumeBootstrap(context.Background(), CycleRequest{ProjectRoot: root}, &ResumePoint{CycleID: 11}, PhaseAudit)

	if err != nil {
		t.Fatalf("loadResumeBootstrap: %v", err)
	}
	if boot.cycle != 11 || boot.cycleState.CycleID != 11 || boot.startPhase != PhaseAudit {
		t.Errorf("boot cycle=%d cs=%d start=%s, want cycle 11 from the checkpoint and audit kept", boot.cycle, boot.cycleState.CycleID, boot.startPhase)
	}
}

func TestLoadResumeBootstrap_CycleStateDisagreeingWithTheCheckpointIsRejected(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	storage := &fakeStorage{
		state:      State{LastCycleNumber: 6},
		cycleState: CycleState{CycleID: 5, WorkspacePath: filepath.Join(root, "ws")},
	}
	o := NewOrchestrator(storage, &fakeLedger{}, buildRunners(nil))

	_, err := o.loadResumeBootstrap(context.Background(), CycleRequest{ProjectRoot: root}, &ResumePoint{CycleID: 6}, PhaseAudit)

	if err == nil || err.Error() != "resume identity mismatch: cycle-state cycle 5 does not match checkpoint cycle 6" {
		t.Fatalf("err = %v, want the cycle mismatch", err)
	}
}

func TestLoadResumeBootstrap_CorruptActivationMarkerFailsTheRecovery(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	marker := filepath.Join(root, ".evolve", "build-explanation-contracts", "cycle-8.json")
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("{corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	storage := &fakeStorage{cycleState: CycleState{
		CycleID: 8, RunID: "r8", WorkspacePath: filepath.Join(root, "ws"), ActiveWorktree: root,
		WorktreeBaseSHA: "abc123", ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
	}}
	o := NewOrchestrator(storage, &fakeLedger{}, buildRunners(nil))

	_, err := o.loadResumeBootstrap(context.Background(), CycleRequest{ProjectRoot: root}, &ResumePoint{CycleID: 8}, PhaseAudit)

	if err == nil || !strings.HasPrefix(err.Error(), "recover rebased Build checkpoint: ") || !strings.Contains(err.Error(), "activation marker") {
		t.Fatalf("err = %v, want the recovery failure naming the activation marker", err)
	}
	if storage.writeCSCalls != 0 {
		t.Errorf("cycle-state written %d time(s), want none on a failed recovery", storage.writeCSCalls)
	}
}

func TestLoadResumeBootstrap_RecoveredBaseThatCannotBePersistedIsAnError(t *testing.T) {
	t.Parallel()

	root, workspace, newBase := rebasedSplitFixture(t)
	storage := &fakeStorage{failOnWriteCS: true, state: State{LastCycleNumber: 42}, cycleState: CycleState{
		CycleID: 42, RunID: "run-42", WorkspacePath: workspace, ActiveWorktree: root,
		WorktreeBaseSHA: newBase, ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
	}}
	o := NewOrchestrator(storage, &fakeLedger{}, buildRunners(nil))

	_, err := o.loadResumeBootstrap(context.Background(), CycleRequest{ProjectRoot: root},
		&ResumePoint{CycleID: 42, WorktreePath: root}, PhaseAudit)

	if err == nil || !strings.HasPrefix(err.Error(), "persist recovered rebased Build checkpoint: ") {
		t.Fatalf("err = %v, want the persist failure", err)
	}
	if storage.writeCSCalls != 1 {
		t.Errorf("WriteCycleState calls = %d, want exactly the one recovery write", storage.writeCSCalls)
	}
}

func TestLoadResumeBootstrap_RecoveredBaseForcesTheBuildPhase(t *testing.T) {
	t.Parallel()

	root, workspace, newBase := rebasedSplitFixture(t)
	storage := &fakeStorage{state: State{LastCycleNumber: 42}, cycleState: CycleState{
		CycleID: 42, RunID: "run-42", WorkspacePath: workspace, ActiveWorktree: root,
		WorktreeBaseSHA: newBase, ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
	}}
	o := NewOrchestrator(storage, &fakeLedger{}, buildRunners(nil))

	boot, err := o.loadResumeBootstrap(context.Background(), CycleRequest{ProjectRoot: root},
		&ResumePoint{CycleID: 42, WorktreePath: root}, PhaseAudit)

	if err != nil {
		t.Fatalf("loadResumeBootstrap: %v", err)
	}
	if boot.startPhase != PhaseBuild || storage.cycleState.WorktreeBaseSHA != newBase {
		t.Errorf("start=%s persisted base=%q, want build and %q", boot.startPhase, storage.cycleState.WorktreeBaseSHA, newBase)
	}
}
