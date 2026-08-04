//go:build acs

// Package cycle1290 materialises the cycle-1290 acceptance criteria for the two
// fleet-scoped tasks pinned to this lane (inbox item `continuation-defect-ledger`,
// third hop of the 1285 → 1287 → 1290 continuation chain):
//
//   - faillearn-publish-mode-parity              → cycle-1287 audit defects[0] (F1,
//     MEDIUM): the failure floor publishes its own artifacts at 0600 while the rest
//     of the runtime publishes 0644, and nothing pins the mode.
//   - faillearn-inbox-failure-preserves-diagnosis → the residual the 1287 landing
//     note named rather than closed: a disk-level inbox failure yields ZERO
//     artifacts, so the diagnosis dies with the queue write.
//
// Predicate strategy. Predicates 001/002 drive the production entry point
// (faillearn.WriteArtifacts) directly from this package and assert on the emitted
// artifacts' MODE and CONTENT — so they are immune to the two cheapest gaming
// moves at once: deleting the in-package unit tests, and asserting `err == nil`
// without ever stat-ing anything. 003/004 then require those in-package tests to
// be tree-resident and executing (the cycle-1285 lesson: a red reproducer minted
// and abandoned in the same cycle protects nothing), and require the pre-existing
// transactional invariants to still pass UNMODIFIED — greening 002 by weakening
// inbox_transactional_test.go is the fix being wrong, not the contract being met.
// Subprocess predicates run ONE named package under an explicit -run expression
// with per-name PASS accounting, per the flaky-predicate-shape rules.
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

// goTestRun runs ONE named package under an explicit -run expression built from
// the exact test names given, and requires EVERY named test to have executed and
// PASSED.
//
// Per-name accounting, not exit code alone: `go test -run TestThatDoesNotExist
// ./pkg` exits 0 with a warning, and an alternation where only some names exist
// exits 0 with no warning at all — so an exit-code predicate greens on a tree that
// deleted the very tests it exists to protect. `go -C <dir>` anchors the
// invocation to the worktree under test rather than the process cwd, which differs
// between the main tree, a worktree, and each fleet lane.
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

// TestC1290_001_FloorArtifactsPublishAtTheAtomicwriteMode is T1's behavioural
// criterion, exercised against the production call rather than against a source
// grep for `Chmod`: every artifact WriteArtifacts publishes — retrospective,
// lesson, inbox item — must land at 0644, the mode internal/atomicwrite documents
// and enforces for every other published runtime artifact (atomicwrite.go:61-63).
// RED today: os.CreateTemp yields 0600 and os.Link preserves it, so all three land
// 0600 and are unreadable to the other fleet lanes and the operator that read them.
func TestC1290_001_FloorArtifactsPublishAtTheAtomicwriteMode(t *testing.T) {
	runDir, lessonsDir, inboxDir := t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "inbox")

	if err := faillearn.WriteArtifacts(failureEvent(), runDir, lessonsDir, faillearn.WithInbox(inboxDir, remediationItems())); err != nil {
		t.Fatalf("WriteArtifacts: %v", err)
	}

	paths := []string{filepath.Join(runDir, "retrospective-report.md")}
	for _, it := range remediationItems() {
		paths = append(paths, filepath.Join(inboxDir, it.ID+".json"))
	}
	ents, err := os.ReadDir(lessonsDir)
	if err != nil {
		t.Fatalf("read lessons dir: %v", err)
	}
	for _, e := range ents {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".yaml" {
			paths = append(paths, filepath.Join(lessonsDir, e.Name()))
		}
	}
	if len(paths) != 4 {
		t.Fatalf("expected 4 published artifacts (report + 2 inbox items + 1 lesson), got %d: %v", len(paths), paths)
	}
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			t.Errorf("stat %s: %v", p, err)
			continue
		}
		if got := info.Mode().Perm(); got != 0o644 {
			t.Errorf("%s published with mode %04o, want 0644 (atomicwrite contract)", filepath.Base(p), got)
		}
	}
}

// TestC1290_002_InboxFailurePreservesTheDiagnosisWithoutBreakingTheOrdering is
// T2's behavioural criterion. Four properties in one predicate because any three
// of them are satisfiable by a wrong fix: the error is still returned (no
// swallow), retrospective-report.md is still absent (the 1255 invariant, and the
// ordering in WriteArtifacts is NOT reversed), retrospective-unqueued.md carries
// the diagnosis, and it names every remediation item that reached no queue.
// RED today: the inbox failure returns before anything is written, so the run dir
// is empty and the failure analysis is lost along with the queue write.
func TestC1290_002_InboxFailurePreservesTheDiagnosisWithoutBreakingTheOrdering(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()

	// A regular file where the inbox DIRECTORY must go: MkdirAll and create both
	// fail ENOTDIR. Deterministic, no fault-injection seam, and not defeated by a
	// root CI runner the way a chmod-based injection would be.
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

// TestC1290_003_TheRegressionPinsAreTreeResidentAndExecuting requires this cycle's
// reproducers to survive as tests in the package they protect. The cycle-1285
// lesson this closes: a red reproducer minted and abandoned inside the same cycle
// leaves the defect free to return the moment the predicate package ages out of
// the ACS lane (cycle predicates run for one cycle; package tests run forever).
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

// TestC1290_004_TransactionalInvariantsSurviveUnmodified is the anti-gaming twin
// of 002. The cheapest way to green a "write something on the failure arm"
// criterion is to relax the invariant that says nothing may be written there, so
// the four pre-existing transactional locks are pinned independently and must stay
// green with their file UNMODIFIED (hypothesis H3: if the fix requires editing
// them, the design is wrong and belongs on the failure arm instead of the order).
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

// TestC1290_005_ContinuationDocsRecordTheResidualClosure is the operator DOCS
// directive for this lane: the two governed documents must carry the residual and
// its closure, so the next hop reconciles against a written record instead of
// re-deriving it. A content assertion is the criterion itself here (the artifact
// under test IS the document), not a stand-in for a behavioural check.
func TestC1290_005_ContinuationDocsRecordTheResidualClosure(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range []string{
		"docs/operations/batch-integrity-review-2026-08-04.md",
		"docs/architecture/continuation-defect-ledger.md",
	} {
		path := filepath.Join(root, rel)
		if !acsassert.FileExists(t, path) {
			continue // FileExists already reported the miss
		}
		if !acsassert.FileContainsAny(path, "UNQUEUED", "retrospective-unqueued.md") {
			t.Errorf("%s does not record the unqueued-diagnosis closure — the 1287 landing named this residual rather than closing it, and an unrecorded closure is how the next hop loses it again", rel)
		}
	}
}

// TestC1290_006_TreeBuilds compiles the whole module: both tasks edit
// go/internal/faillearn/writer.go, a package with off-lane consumers
// (core/failure_learning.go, core/reset.go, cmd/evolve/cmd_loop_outcome.go), so a
// signature change reaches packages this cycle never touches. `go build`, not a
// `go test ./...` sweep — the flaky-shape rules ban the sweep as a predicate.
func TestC1290_006_TreeBuilds(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", filepath.Join(root, "go"), "build", "./...")
	if err != nil || code != 0 {
		t.Errorf("go build ./... exited %d (err=%v)\n%s%s", code, err, stdout, stderr)
	}
}
