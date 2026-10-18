package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/runscope"
)

type namelessMinter struct{}

func (namelessMinter) Register(phaseconfig.PhaseConfig) (phasespec.PhaseSpec, PhaseRunner, error) {
	return phasespec.PhaseSpec{}, probeFailRunner{}, nil
}

func TestRegisterMintedPhases_InvalidSpecWarnsAtMergeAndRouting(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{registrar: namelessMinter{}, runners: map[Phase]PhaseRunner{}}
	published := 0
	o.catalogPublisher = func(phasespec.Catalog) { published++ }
	o.registerMintedPhases(&router.PhasePlan{MintPhases: []phaseconfig.PhaseConfig{{}}})
	if published != 1 {
		t.Fatalf("catalog must still publish, published=%d", published)
	}
}

func TestRunScopedMergeReview_MalformedComposedDiffIsEntangled(t *testing.T) {
	t.Parallel()
	res, err := RunScopedMergeReview(ScopedMergeInput{ComposedDiff: []byte("@@ not a header\n")}, nil)
	if err == nil || res.Disposition != ScopedMergeEntangled {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

type scoutWritingStrategy struct {
	router.StaticPreset
	workspace string
}

func (s scoutWritingStrategy) Decide(in router.RouteInput) router.RouterDecision {
	body := "<!-- challenge-token: x -->\ngoal_type: feature\ndeliverable_kind: code\ncycle_size_estimate: small\n\n## Selected Tasks\n- a\n"
	_ = os.WriteFile(filepath.Join(s.workspace, "scout-report.md"), []byte(body), 0o644)
	return s.StaticPreset.Decide(in)
}

func TestSelectNext_FreshDigestSatisfyingTheSpineAdvancesWithoutFailOpen(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	cr := spineWarnCycleRun(t, scoutWritingStrategy{workspace: ws}, NewStateMachine(), ws)
	cr.cs.CompletedPhases = []string{"scout"}

	next, act, err := cr.selectNext()
	if err != nil || act != loopNext || next != PhaseBuild {
		t.Fatalf("selectNext() = (%q, %v, %v), want advance to build", next, act, err)
	}
	if len(cr.result.SpineFailOpens) != 0 {
		t.Fatalf("a spine satisfied by the fresh digest must not fail open, got %d", len(cr.result.SpineFailOpens))
	}
}

func TestFinalizeCycle_BenignLateWriteRaceReconcilesToPass(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeVerdicts(t, ws, "PASS", "PASS")
	o := &Orchestrator{
		storage:          &fakeUpdaterStorage{},
		gitHEAD:          func() (string, error) { return "same-head", nil },
		failurePolicy:    policy.DefaultSystemFailurePolicy(),
		contractVerifier: okStubVerifier{ok: true},
	}
	result := &CycleResult{FinalVerdict: VerdictFAIL}
	if _, err := o.finalizeCycle(context.Background(), CycleState{CycleID: 1, WorkspacePath: ws}, 1, "same-head", "", result, &State{}, nil); err != nil {
		t.Fatalf("finalizeCycle: %v", err)
	}
	if result.FinalVerdict != VerdictPASS {
		t.Fatalf("final verdict = %q, want the reconciled PASS", result.FinalVerdict)
	}
}

func TestParseAdjudication_UnbalancedBraceIsNil(t *testing.T) {
	t.Parallel()
	if parseAdjudication("dangling { open", filepath.Join(t.TempDir(), "absent")) != nil {
		t.Fatal("unbalanced span must not parse")
	}
}

func TestOriginDefaultBranch_GitFailureFallsBackToMain(t *testing.T) {
	useScriptedGit(t, func(int, []string) scriptedReply { return scriptedReply{exit: 1} })
	if got := originDefaultBranch(context.Background(), gitexecAt("/p")); got != "main" {
		t.Fatalf("branch = %q", got)
	}
}

func TestGitWorktree_CreateReusedWorktreeFailures(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(t.TempDir(), "wts")
	wt := runscope.New(runscope.LaneFromRoot(root), "", 3).WorktreeDir(base)
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFileAt(t, filepath.Join(wt, "dirty"), "d")

	useScriptedGit(t, func(_ int, args []string) scriptedReply {
		switch args[0] {
		case "status":
			return scriptedReply{stdout: " M dirty\n"}
		case "reset":
			return scriptedReply{exit: 1}
		}
		return scriptedReply{}
	})
	if _, err := (gitWorktree{baseOverride: base}).Create(root, 3); err == nil || !strings.Contains(err.Error(), "worktree reuse") {
		t.Fatalf("err = %v", err)
	}

	useScriptedGit(t, func(_ int, args []string) scriptedReply {
		if args[0] == "rev-parse" {
			return scriptedReply{exit: 128}
		}
		return scriptedReply{exit: 1}
	})
	if _, err := (gitWorktree{baseOverride: base}).Create(root, 3); err == nil {
		t.Fatal("an invalid leftover worktree must be pruned and the fresh add must fail under the scripted git")
	}
	if _, err := os.Stat(wt); err == nil {
		t.Fatal("the invalid leftover worktree directory must be removed")
	}
}

func TestLexicalPath_RelativeAndAbsoluteSpellingsAgree(t *testing.T) {
	t.Parallel()
	if lexicalPath("rel/./x") != lexicalPath("rel/x") {
		t.Fatal("equivalent relative spellings must clean to one path")
	}
	if !sameDirectory("/a/./b", "/a/b") {
		t.Fatal("lexically equal absolute paths must compare equal")
	}
}

func TestCheckpointInterruptedPhase_CheckpointFailureWarns(t *testing.T) {
	old := ResumeBoundaryCheckpointer
	t.Cleanup(func() { ResumeBoundaryCheckpointer = old })
	ResumeBoundaryCheckpointer = func(CycleState, string, time.Time) error { return errors.New("checkpoint refused") }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cr := &cycleRun{ctx: ctx, o: &Orchestrator{now: time.Now}}
	cr.checkpointInterruptedPhase(context.Canceled)
}
