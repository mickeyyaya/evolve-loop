//go:build acs

// Package cycle1292 materialises the cycle-1292 acceptance criteria for the
// single fleet-scoped lane pinned to this cycle (inbox item
// `continuation-defect-ledger`, fifth hop of the 1279 → 1282 → 1285/1287 → 1290
// → 1292 chain). It closes the two defects the immediate ancestor's disposition
// ledger carried forward OPEN (`.evolve/runs/cycle-1290/defect-dispositions.json`):
//
//   - 1290-D2 → the partial-write OVERCLAIM. writeInboxItems writes one file per
//     item and returns on the FIRST failure, so items before the failing one are
//     already on disk; preserveDiagnosis nonetheless lists every configured item
//     as "still UNQUEUED" in retrospective-unqueued.md.
//   - 1290-D1 → the UNBACKED DEFERRAL. Both governed documents assert 1287-F2 was
//     "queued as audit-eval-existence-path-convention", and no such item exists
//     in .evolve/inbox — an unbacked claim inside the very lane that exists to
//     catch unbacked claims.
//
// Predicate strategy. 001/002 drive the production entry point
// (faillearn.WriteArtifacts) from OUTSIDE the package and assert on the emitted
// artifact's content, so they survive both cheap gaming moves at once: deleting
// the in-package reproducer, and asserting `err != nil` without ever reading the
// artifact. 003 then requires the in-package reproducer to be tree-resident and
// run-and-pass together with the pre-existing 1287/1290 invariants — greening 001
// by weakening inbox_transactional_test.go or inbox_failure_degraded_test.go is
// the fix being wrong, not the contract being met. 004 drives the REAL inbox
// consumer (inboxbatch.LoadDir) rather than stat-ing a filename, because the
// defect is "the claim is unbacked", and a file the loader drops backs nothing.
// Subprocess predicates run ONE named package under an explicit -run expression
// with per-name PASS accounting, per the flaky-predicate-shape rules.
package cycle1292

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/faillearn"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// deferralItemSlug is the id the two governed documents promise is queued. The
// claim is only true when an inbox item the loader can actually parse carries it.
const deferralItemSlug = "audit-eval-existence-path-convention"

// goTestRun runs ONE named package under an explicit -run expression built from
// the exact test names given, and requires EVERY named test to have executed and
// PASSED.
//
// Per-name accounting, not exit code alone: `go test -run TestThatDoesNotExist
// ./pkg` exits 0 with a warning, and an alternation where only some names exist
// exits 0 with no warning at all — so an exit-code predicate greens on a tree
// that deleted the very tests it exists to protect. `go -C <dir>` anchors the
// invocation to the worktree under test rather than the process cwd, which
// differs between the main tree, a worktree, and each fleet lane.
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
		Cycle:          1292,
		FailedPhase:    "audit",
		Scope:          faillearn.ScopePhase,
		Classification: "cycle-mid-execution-fail",
		Verdict:        "FAIL",
		Summary:        "audit rejected the deliverable",
		Defects:        []string{"the degraded retrospective overclaims which remediation reached no queue"},
		EvidencePaths:  []string{"/tmp/ws/audit-report.md"},
		Now:            time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC),
	}
}

func ledgerItems() []faillearn.InboxItem {
	return []faillearn.InboxItem{
		{ID: "acs-1292-queued", Title: "reaches disk before the failure", Weight: 0.95, Kind: "bug", Priority: "H", InjectedBy: "retrofile"},
		{ID: "acs-1292-fails", Title: "the write that fails", Weight: 0.9, Kind: "bug", Priority: "H", InjectedBy: "retrofile"},
		{ID: "acs-1292-unattempted", Title: "never attempted", Weight: 0.85, Kind: "bug", Priority: "M", InjectedBy: "retrofile"},
	}
}

// unqueuedSection isolates the "still UNQUEUED" list from the degraded artifact.
// Section-scoped, never whole-file: the artifact embeds the rendered
// retrospective, whose defect text can legitimately mention an item id, so a
// whole-file Contains would green on the very overclaim under test.
func unqueuedSection(t *testing.T, body string) string {
	t.Helper()
	lines := strings.Split(body, "\n")
	start := -1
	for i, ln := range lines {
		if strings.HasPrefix(ln, "#") && strings.Contains(ln, "still UNQUEUED") {
			start = i + 1
			break
		}
	}
	if start < 0 {
		t.Fatalf("degraded artifact has no \"still UNQUEUED\" section:\n%s", body)
	}
	var out []string
	for _, ln := range lines[start:] {
		if strings.HasPrefix(ln, "#") {
			break
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

// TestC1292_001_PartialInboxWriteNamesOnlyUnqueuedItems is the primary predicate
// for 1290-D2. An id collision on item 2-of-3 leaves item 1 on disk; the degraded
// artifact must name items 2 and 3 and must NOT name item 1.
func TestC1292_001_PartialInboxWriteNamesOnlyUnqueuedItems(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()
	inboxDir := filepath.Join(t.TempDir(), "inbox")
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatalf("prepare inbox dir: %v", err)
	}
	items := ledgerItems()
	// A DIFFERENT item already filed under item 2's id — the cycle-1282 DEF-4
	// collision rule refuses to drop ours, which fails the write at index 1.
	collision, err := json.MarshalIndent(faillearn.InboxItem{ID: items[1].ID, Title: "filed by another lane", Weight: 0.5, Kind: "chore", Priority: "L", InjectedBy: "other-lane"}, "", "  ")
	if err != nil {
		t.Fatalf("encode colliding item: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inboxDir, items[1].ID+".json"), collision, 0o644); err != nil {
		t.Fatalf("write colliding item: %v", err)
	}

	writeErr := faillearn.WriteArtifacts(failureEvent(), runDir, lessonsDir, faillearn.WithInbox(inboxDir, items))
	if writeErr == nil {
		t.Fatal("an id collision must still abort WriteArtifacts — an accurate item list is an ADDITION to failing loudly, never a replacement")
	}
	if _, statErr := os.Stat(filepath.Join(runDir, "retrospective-report.md")); statErr == nil {
		t.Error("retrospective-report.md was written while remediation reached no queue — the 1255 abort ordering must stay unreversed")
	}
	if _, statErr := os.Stat(filepath.Join(inboxDir, items[0].ID+".json")); statErr != nil {
		t.Fatalf("fixture premise broken: %q was expected on disk before the failing item: %v", items[0].ID, statErr)
	}

	raw, readErr := os.ReadFile(filepath.Join(runDir, "retrospective-unqueued.md"))
	if readErr != nil {
		t.Fatalf("the diagnosis must still be preserved as retrospective-unqueued.md: %v", readErr)
	}
	section := unqueuedSection(t, string(raw))
	if strings.Contains(section, items[0].ID) {
		t.Errorf("retrospective-unqueued.md lists %q as still UNQUEUED although it is on disk in the inbox — cycle-1290 D2, the overclaim this cycle closes\n--- UNQUEUED section ---\n%s", items[0].ID, section)
	}
	for _, it := range items[1:] {
		if !strings.Contains(section, it.ID) {
			t.Errorf("retrospective-unqueued.md omits %q, which reached no queue — under-claiming loses the work the artifact exists to preserve\n--- UNQUEUED section ---\n%s", it.ID, section)
		}
	}
}

// TestC1292_002_TotalInboxFailureStillNamesEveryItem is the boundary predicate:
// when the inbox directory itself is unwritable NOTHING reached the queue, so the
// correct list is every item. Guards the wrong fix that assumes "everything
// before the error index succeeded" or simply drops the first entry.
func TestC1292_002_TotalInboxFailureStillNamesEveryItem(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()
	// A regular file where the inbox DIRECTORY must go: MkdirAll and create both
	// fail ENOTDIR. Deterministic, and not defeated by a root CI runner the way a
	// chmod-based injection would be.
	blocked := filepath.Join(t.TempDir(), "inbox")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("prepare blocked inbox path: %v", err)
	}
	items := ledgerItems()

	if err := faillearn.WriteArtifacts(failureEvent(), runDir, lessonsDir, faillearn.WithInbox(blocked, items)); err == nil {
		t.Fatal("an unwritable inbox directory must still return an error")
	}
	raw, readErr := os.ReadFile(filepath.Join(runDir, "retrospective-unqueued.md"))
	if readErr != nil {
		t.Fatalf("the diagnosis must be preserved on a total inbox failure: %v", readErr)
	}
	section := unqueuedSection(t, string(raw))
	for _, it := range items {
		if !strings.Contains(section, it.ID) {
			t.Errorf("retrospective-unqueued.md omits %q although NOTHING reached the queue on this arm\n--- UNQUEUED section ---\n%s", it.ID, section)
		}
	}
}

// TestC1292_003_ReproducerAndPriorInvariantsRunAndPass requires the in-package
// reproducer for 1290-D2 to be tree-resident and executing (the cycle-1285
// lesson: a red reproducer minted and abandoned in the same cycle protects
// nothing), AND the pre-existing 1287/1290 invariants to still pass. Greening
// 001 by weakening the transactional or degraded-arm locks fails here.
func TestC1292_003_ReproducerAndPriorInvariantsRunAndPass(t *testing.T) {
	goTestRun(t, acsassert.RepoRoot(t), "./internal/faillearn",
		// cycle-1292 reproducer (1290-D2)
		"TestWriteArtifacts_PartialWriteNamesOnlyUnqueuedItems",
		"TestWriteArtifacts_PartialWriteItemRejectionNamesOnlyUnqueuedItems",
		"TestWriteArtifacts_PartialWrite_TotalFailureNamesEveryItem",
		"TestWriteArtifacts_PartialWrite_FirstItemFailsNamesEveryItem",
		// pre-existing invariants that must survive the fix
		"TestWriteArtifacts_InboxFailureWritesUnqueuedRetro",
		"TestWriteArtifacts_InboxFailureDegradedRetroIsIdempotent",
		"TestWriteArtifacts_SuccessMintsNoUnqueuedMarker",
		"TestWriteArtifacts_ItemLevelRejectionAlsoPreservesDiagnosis",
	)
}

// TestC1292_004_DeferralClaimIsBackedByALoadableInboxItem is the predicate for
// 1290-D1. It drives the REAL consumer — inboxbatch.LoadDir, the loader the
// triage path uses — rather than stat-ing a filename: the defect is that a
// documented deferral is unbacked, and an item the loader drops or parses into an
// empty shell backs nothing (the cycle-1190 dropped-field shape).
func TestC1292_004_DeferralClaimIsBackedByALoadableInboxItem(t *testing.T) {
	dir := filepath.Join(acsassert.RepoRoot(t), ".evolve", "inbox")
	items, warnings, err := inboxbatch.LoadDir(dir)
	if err != nil {
		t.Fatalf("load %s: %v", dir, err)
	}
	for _, w := range warnings {
		if strings.Contains(w, deferralItemSlug) {
			t.Errorf("the %s inbox item is malformed and was skipped by the loader — a file the consumer drops backs no deferral claim: %s", deferralItemSlug, w)
		}
	}
	var found *inboxbatch.Item
	for i := range items {
		if strings.Contains(items[i].ID, deferralItemSlug) || strings.Contains(items[i].Path, deferralItemSlug) {
			found = &items[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("no inbox item under %s carries %q, yet both governed documents assert 1287-F2 was queued under that id — the unbacked deferral claim of cycle-1290 D1", dir, deferralItemSlug)
	}
	if strings.TrimSpace(found.Title) == "" {
		t.Errorf("inbox item %s has an empty title — an unreadable queue entry is not a backed deferral", found.ID)
	}
	if found.Weight <= 0 {
		t.Errorf("inbox item %s has weight %v — a zero-weight item is never selected, so the deferral would remain effectively unqueued", found.ID, found.Weight)
	}
	if strings.TrimSpace(found.Priority) == "" || strings.TrimSpace(found.Kind) == "" {
		t.Errorf("inbox item %s is missing kind (%q) and/or priority (%q) — the fields triage batches on", found.ID, found.Kind, found.Priority)
	}
	if strings.TrimSpace(found.InjectedBy) == "" {
		t.Errorf("inbox item %s has an empty injected_by — inboxbatch.ConsoleRouted reads an empty injected_by as OPERATOR-authored and honours a route override from it; an agent-filed item must never inherit that authority", found.ID)
	}
}

// TestC1292_005_GovernedDocsRecordTheLedgerContinuation requires both governed
// documents to carry the 1290-D1/1290-D2 continuation record, so the next hop
// reads the resolution from the documents rather than re-deriving it — the whole
// point of the ledger.
//
// acs-predicate: config-check — a documentation criterion has no runtime surface
// to exercise; the artifact's presence and shape IS the requirement.
func TestC1292_005_GovernedDocsRecordTheLedgerContinuation(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, doc := range []string{
		filepath.Join(root, "docs", "architecture", "continuation-defect-ledger.md"),
		filepath.Join(root, "docs", "operations", "batch-integrity-review-2026-08-04.md"),
	} {
		for _, needle := range []string{"1290-D1", "1290-D2", deferralItemSlug} {
			if !acsassert.FileContains(t, doc, needle) {
				t.Errorf("%s must record the cycle-1292 continuation of the defect ledger (missing %q) — a landing that closes a carried-forward defect without saying so in the governed document is the laundering this lane exists to stop", doc, needle)
			}
		}
	}
}
