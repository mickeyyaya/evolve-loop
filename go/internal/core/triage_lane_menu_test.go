package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func withholdsEverything(string, []inboxbatch.Item) []inboxbatch.Item { return nil }

func TestHasClaimableInboxWork_AsksTheLaneMenu(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "guarded.json"), []byte(`{"id":"guarded","kind":"bug","files":["go/internal/guards/role.go"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !hasClaimableInboxWork(root, 0, routingOnlyLaneMenu) {
		t.Fatal("without a lane menu only console routes and kinds are withheld, so the guarded item reads as claimable")
	}
	var menu LaneMenuFn = withholdsEverything
	if hasClaimableInboxWork(root, 0, menu) {
		t.Fatal("the lane menu decides what is claimable")
	}
}

func TestRunCycle_EmptyTriageOverWorkTheLaneMenuWithholdsIsPlannedNoWork(t *testing.T) {
	t.Parallel()
	root := writeClaimableInbox(t, 1)
	runners := buildRunners(nil)
	runners[PhaseTriage] = &countingTriageRunner{inner: triageDecisionRunner{verdict: VerdictPASS, decision: `{"top_n":[],"deferred":[],"dropped":[]}`}}
	orchestrator := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}), WithLaneMenu(withholdsEverything))
	if !orchestrator.LaneMenuWired() {
		t.Fatal("WithLaneMenu wires the lane menu")
	}
	result, err := orchestrator.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "lane-menu"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if result.TerminationReason != CycleTerminationTriageNoWork || !IsTriageNoWorkResult(result) {
		t.Errorf("work the lane menu withholds is not a claim failure: reason=%q verdict=%q", result.TerminationReason, result.FinalVerdict)
	}
}

func TestHasClaimableInboxWork_AClaimIsWorkTheCycleTookWhateverTheMenuSays(t *testing.T) {
	root := t.TempDir()
	claimed := inboxbatch.ProcessingCycleDir(filepath.Join(root, ".evolve", "inbox"), 7)
	if err := os.MkdirAll(claimed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claimed, "waiting.json"), []byte(`{"id":"waiting","deps":["console-owned"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !hasClaimableInboxWork(root, 7, withholdsEverything) {
		t.Fatal("an item this cycle claimed and left uncommitted is a claim failure, so the failure drain releases it")
	}
}

func TestUnansweredClaimableWork_TheLanePathAsksTheMenuForPendingItemsButNeverForClaims(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	workspace := t.TempDir()
	writeLaneScope(t, workspace, "pending-waiting", "claimed")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "pending-waiting.json"), []byte(`{"id":"pending-waiting","deps":["console-owned"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if unansweredClaimableWork(root, workspace, 7, withholdsEverything) {
		t.Error("a scoped pending item the lane menu withholds is not unanswered claimable work")
	}
	claimedDir := inboxbatch.ProcessingCycleDir(inbox, 7)
	if err := os.MkdirAll(claimedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claimedDir, "claimed.json"), []byte(`{"id":"claimed"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !unansweredClaimableWork(root, workspace, 7, withholdsEverything) {
		t.Error("a scoped item this cycle claimed and left unanswered is a claim failure whatever the lane menu says")
	}
}
