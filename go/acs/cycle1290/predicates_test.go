//go:build acs

package cycle1290

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/faillearn"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goTestRun(t *testing.T, root, pkg string, names ...string) {
	t.Helper()
	anchored := make([]string, 0, len(names))
	for _, n := range names {
		anchored = append(anchored, "^"+n+"$")
	}
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "test", "-count=1", "-v", "-run", strings.Join(anchored, "|"), pkg)
	combined := stdout + stderr
	for _, n := range names {
		if !strings.Contains(combined, "--- PASS: "+n) {
			t.Errorf("%s: %s did not run-and-pass — it is missing from this tree or failing, so the behaviour it pins is unprotected", pkg, n)
		}
	}
	if err != nil || code != 0 {
		t.Errorf("go test %s exited %d (err=%v)\n%s", pkg, code, err, combined)
	}
}

func failureEvent() faillearn.FailureEvent {
	return faillearn.FailureEvent{
		Cycle:          1290,
		FailedPhase:    "audit",
		Scope:          faillearn.ScopePhase,
		Classification: "cycle-mid-execution-fail",
		Verdict:        "FAIL",
		Summary:        "audit rejected the deliverable",
		Defects:        []string{"floor artifacts publish at 0600", "inbox failure suppresses the retrospective"},
		EvidencePaths:  []string{"/tmp/ws/audit-report.md"},
		Now:            time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC),
	}
}

func remediationItems() []faillearn.InboxItem {
	return []faillearn.InboxItem{
		{ID: "retro-1290-publish-mode", Title: "Publish floor artifacts at 0644", Weight: 0.95, Kind: "bug", Priority: "H", Files: []string{"go/internal/faillearn/writer.go"}, InjectedBy: "retrofile"},
		{ID: "retro-1290-unqueued-marker", Title: "Preserve the diagnosis on inbox failure", Weight: 0.9, Kind: "bug", Priority: "H", Files: []string{"go/internal/faillearn/writer.go"}, InjectedBy: "retrofile"},
	}
}

func TestC1290_002_InboxFailurePreservesTheDiagnosisWithoutBreakingTheOrdering(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()

	blocked := filepath.Join(t.TempDir(), "inbox")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("prepare blocked inbox path: %v", err)
	}

	err := faillearn.WriteArtifacts(failureEvent(), runDir, lessonsDir, faillearn.WithInbox(blocked, remediationItems()))
	if err == nil {
		t.Fatal("WriteArtifacts must still return the inbox-write error — preserving the diagnosis is an addition to failing loudly, not a replacement for it")
	}
	if _, statErr := os.Stat(filepath.Join(runDir, "retrospective-report.md")); statErr == nil {
		t.Error("retrospective-report.md was written while the remediation reached no queue — the 1255 state the abort ordering exists to make unreachable")
	}

	raw, readErr := os.ReadFile(filepath.Join(runDir, "retrospective-unqueued.md"))
	if readErr != nil {
		t.Fatalf("a disk-level inbox failure must leave the diagnosis on disk as retrospective-unqueued.md: %v", readErr)
	}
	body := string(raw)
	if !strings.Contains(body, "UNQUEUED") {
		t.Errorf("retrospective-unqueued.md must carry an explicit UNQUEUED marker — an unmarked degraded retrospective reads as a complete one:\n%s", body)
	}
	if !strings.Contains(body, failureEvent().Summary) {
		t.Errorf("retrospective-unqueued.md must contain the failure diagnosis, not only a marker:\n%s", body)
	}
	for _, it := range remediationItems() {
		if !strings.Contains(body, it.ID) {
			t.Errorf("retrospective-unqueued.md does not name unqueued remediation item %q — those items are the work that was lost:\n%s", it.ID, body)
		}
	}
}

func TestC1290_003_TheRegressionPinsAreTreeResidentAndExecuting(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goTestRun(t, root, "./internal/faillearn",
		"TestWriteArtifacts_PublishedArtifactsHaveMode0644",
		"TestWriteArtifacts_ModeParityAlsoHoldsWithoutTheInboxOption",
		"TestWriteArtifacts_ExistingArtifactModeIsNotRewritten",
		"TestWriteArtifacts_InboxFailureWritesUnqueuedRetro",
		"TestWriteArtifacts_InboxFailureDegradedRetroIsIdempotent",
		"TestWriteArtifacts_SuccessMintsNoUnqueuedMarker",
		"TestWriteArtifacts_InboxFailureWithNoRunDirStillErrors",
		"TestWriteArtifacts_ItemLevelRejectionAlsoPreservesDiagnosis")
}

func TestC1290_004_TransactionalInvariantsSurviveUnmodified(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goTestRun(t, root, "./internal/faillearn",
		"TestWriteArtifacts_InboxItemsLandBesideRetrospective",
		"TestWriteArtifacts_InboxFailureLeavesNoRetrospective",
		"TestWriteArtifacts_WithoutInboxOptionIsUnchanged",
		"TestWriteArtifacts_EmptyInboxItemsMintsNoFiles")

	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"git", "-C", root, "diff", "--name-only", "HEAD", "--", "go/internal/faillearn/inbox_transactional_test.go")
	if err != nil || code != 0 {
		t.Fatalf("git diff exited %d (err=%v)\n%s%s", code, err, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("inbox_transactional_test.go was modified by this cycle (%s) — the 1255 invariant is load-bearing; a fix that needs to edit it is changing the contract rather than closing the residual", strings.TrimSpace(stdout))
	}
}

func TestC1290_005_ContinuationDocsRecordTheResidualClosure(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range []string{
		"docs/operations/batch-integrity-review-2026-08-04.md",
		"docs/architecture/continuation-defect-ledger.md",
	} {
		path := filepath.Join(root, rel)
		if !acsassert.FileExists(t, path) {
			continue
		}
		if !acsassert.FileContainsAny(path, "UNQUEUED", "retrospective-unqueued.md") {
			t.Errorf("%s does not record the unqueued-diagnosis closure — the 1287 landing named this residual rather than closing it, and an unrecorded closure is how the next hop loses it again", rel)
		}
	}
}

func TestC1290_006_TreeBuilds(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "build", "./...")
	if err != nil || code != 0 {
		t.Errorf("go build ./... exited %d (err=%v)\n%s%s", code, err, stdout, stderr)
	}
}
