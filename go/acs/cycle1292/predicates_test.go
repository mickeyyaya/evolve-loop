//go:build acs

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

const deferralItemSlug = "audit-eval-existence-path-convention"

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

func TestC1292_001_PartialInboxWriteNamesOnlyUnqueuedItems(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()
	inboxDir := filepath.Join(t.TempDir(), "inbox")
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatalf("prepare inbox dir: %v", err)
	}
	items := ledgerItems()
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

func TestC1292_002_TotalInboxFailureStillNamesEveryItem(t *testing.T) {
	runDir, lessonsDir := t.TempDir(), t.TempDir()
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

func TestC1292_003_ReproducerAndPriorInvariantsRunAndPass(t *testing.T) {
	goTestRun(t, acsassert.RepoRoot(t), "./internal/faillearn",
		"TestWriteArtifacts_PartialWriteNamesOnlyUnqueuedItems",
		"TestWriteArtifacts_PartialWriteItemRejectionNamesOnlyUnqueuedItems",
		"TestWriteArtifacts_PartialWrite_TotalFailureNamesEveryItem",
		"TestWriteArtifacts_PartialWrite_FirstItemFailsNamesEveryItem",
		"TestWriteArtifacts_InboxFailureWritesUnqueuedRetro",
		"TestWriteArtifacts_InboxFailureDegradedRetroIsIdempotent",
		"TestWriteArtifacts_SuccessMintsNoUnqueuedMarker",
		"TestWriteArtifacts_ItemLevelRejectionAlsoPreservesDiagnosis",
	)
}

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

// acs-predicate: config-check — a documentation criterion has no runtime surface
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
