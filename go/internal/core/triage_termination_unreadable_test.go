package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func writeNoticeOnlyItems(t *testing.T, inbox string) {
	t.Helper()
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	items := map[string]string{
		"long-title.json": `{"id":"long-title","title":"long\u0007title"}`,
		"dup-a.json":      `{"id":"dup"}`,
		"dup-b.json":      `{"id":"dup"}`,
	}
	for name, body := range items {
		if err := os.WriteFile(filepath.Join(inbox, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestHasClaimableInboxWork_ANoticeAboutAReadableItemIsNotUnreadWork(t *testing.T) {
	root := t.TempDir()
	writeNoticeOnlyItems(t, filepath.Join(root, ".evolve", "inbox"))
	if hasClaimableInboxWork(root, 0, withholdsEverything) {
		t.Fatal("a trimmed title or a duplicate id is a notice about an item the menu judged, not unread work")
	}
}

func TestHasClaimableInboxWork_AnUnreadableFileIsNeverProofOfNoWork(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeNoticeOnlyItems(t, inbox)
	if err := os.WriteFile(filepath.Join(inbox, "broken.json"), []byte(`{not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !hasClaimableInboxWork(root, 0, withholdsEverything) {
		t.Fatal("a file the check cannot read could be claimable work")
	}
}

func TestUnansweredClaimableWork_ANoticeIsNotUnreadWorkOnTheLanePath(t *testing.T) {
	root := t.TempDir()
	workspace := t.TempDir()
	writeLaneScope(t, workspace, "long-title")
	writeNoticeOnlyItems(t, filepath.Join(root, ".evolve", "inbox"))
	if unansweredClaimableWork(root, workspace, 7, withholdsEverything) {
		t.Fatal("a scoped item the menu withholds is not unanswered work because its title was trimmed")
	}
}

func TestRunCycle_EmptyTriageOverWithheldWorkIsPlannedNoWorkWhenAnItemWasTrimmed(t *testing.T) {
	t.Parallel()
	root := writeClaimableInbox(t, 1)
	writeNoticeOnlyItems(t, filepath.Join(root, ".evolve", "inbox"))
	runners := buildRunners(nil)
	runners[PhaseTriage] = &countingTriageRunner{inner: triageDecisionRunner{verdict: VerdictPASS, decision: `{"top_n":[],"deferred":[],"dropped":[]}`}}
	orchestrator := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}), WithLaneMenu(withholdsEverything))
	result, err := orchestrator.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "notice"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if result.TerminationReason != CycleTerminationTriageNoWork || !IsTriageNoWorkResult(result) {
		t.Errorf("a notice about a withheld item does not turn planned no-work into a claim failure: reason=%q verdict=%q", result.TerminationReason, result.FinalVerdict)
	}
}

func TestHasClaimableInboxWork_AnUnreadableClaimIsNeverProofOfNoWork(t *testing.T) {
	root := t.TempDir()
	claimed := inboxbatch.ProcessingCycleDir(filepath.Join(root, ".evolve", "inbox"), 0)
	if err := os.MkdirAll(claimed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claimed, "broken.json"), []byte(`{not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !hasClaimableInboxWork(root, 0, withholdsEverything) {
		t.Fatal("a claim the check cannot read could be work this cycle took")
	}
}
