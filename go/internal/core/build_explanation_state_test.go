package core

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
)

func TestNewExplanationLifecycleReviewer_LegacyBuildRequiresNoSnapshot(t *testing.T) {
	reviewer := NewExplanationLifecycleReviewer()
	result := reviewer.Review(context.Background(), ReviewInput{Phase: string(PhaseBuild)})
	if !result.Approve || result.Retry || result.Reason != "" {
		t.Fatalf("legacy Build review=%+v, want unconditional approval", result)
	}
}

func TestBuildExplanationHandoff_PreBuildFailureIsNotReportedAsMissing(t *testing.T) {
	root := t.TempDir()
	workspace := t.TempDir()
	if err := explanationdocs.Activate(explanationdocs.CycleBinding{
		ProjectRoot: root, Workspace: workspace, Cycle: 42, RunID: "run-42",
		ContractVersion: explanationdocs.CurrentContractVersion,
	}); err != nil {
		t.Fatal(err)
	}
	handoff := projectBuildExplanation(root, CycleState{
		CycleID: 42, RunID: "run-42", WorkspacePath: workspace,
		ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
		CompletedPhases:                 []string{string(PhaseScout), string(PhaseTDD)},
	})
	if handoff.State != BuildExplanationNotYetBuilt || handoff.View != nil || handoff.Error != "" {
		t.Fatalf("pre-Build handoff=%+v", handoff)
	}
}

func TestBuildExplanationHandoff_SealedHostContextCannotBeDowngradedByMutableState(t *testing.T) {
	root := t.TempDir()
	workspace := t.TempDir()
	binding := explanationdocs.CycleBinding{
		ProjectRoot: root, Worktree: t.TempDir(), Workspace: workspace,
		BaseSHA: strings.Repeat("a", 40), Cycle: 42, RunID: "run-42",
		ContractVersion: explanationdocs.CurrentContractVersion,
	}
	activation := binding
	activation.Worktree, activation.BaseSHA = "", ""
	if err := explanationdocs.Activate(activation); err != nil {
		t.Fatal(err)
	}
	if err := explanationdocs.SealBuild(binding); err != nil {
		t.Fatal(err)
	}
	handoff := projectBuildExplanation(root, CycleState{
		CycleID: 42, RunID: "run-42", WorkspacePath: workspace,
		ActiveWorktree: binding.Worktree, WorktreeBaseSHA: binding.BaseSHA,
		ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
		// Deliberately forge the Builder-writable lifecycle projection.
		CompletedPhases: []string{string(PhaseScout)},
	})
	if handoff.State != BuildExplanationInvalid || handoff.View != nil || handoff.Error == "" {
		t.Fatalf("sealed host context was downgraded to pre-Build: %+v", handoff)
	}
}

func TestBuildExplanationHandoff_LegacyIsExplicit(t *testing.T) {
	handoff := projectBuildExplanation(t.TempDir(), CycleState{})
	if handoff.State != BuildExplanationLegacy || handoff.View != nil {
		t.Fatalf("legacy handoff=%+v", handoff)
	}
}

func TestPostBuildExplanationRefreshEligibility_IsLimitedToLaterSourceWriters(t *testing.T) {
	cr := &cycleRun{
		o: NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil)),
		cs: CycleState{
			ExplanationDocumentationVersion: explanationdocs.CurrentContractVersion,
			CompletedPhases:                 []string{string(PhaseBuild)},
		},
	}
	if !cr.postBuildExplanationRefreshEligible(PhaseTDD) {
		t.Fatal("post-Build source writer was not eligible for snapshot refresh")
	}
	if cr.postBuildExplanationRefreshEligible(PhaseAudit) || cr.postBuildExplanationRefreshEligible(PhaseBuild) {
		t.Fatal("Audit or Build must not use post-Build refresh")
	}
}
