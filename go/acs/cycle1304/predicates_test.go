//go:build acs

// Package cycle1304 holds the cycle-1304 ACS predicates.
//
// Scope: this lane is pinned to the todo-id `contract-block-cli-escalation`,
// whose code fix already landed on main in cycle-1300 (dossier 061345a4).
// The two committed tasks are queue/doc hygiene, so the system under test is
// the QUEUE STATE and the DOC ROW themselves — these artifacts are the
// deliverable, not a source-grep proxy for some other behavior. The inbox
// predicates parse the JSON payload (identity + provenance fields) rather
// than grepping for a magic string, and the README predicates pin the row's
// semantics plus a no-collateral-damage guard so "delete the row" cannot pass.
package cycle1304

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	// The stranded live-queue item this cycle must consume.
	inboxItemName = "2026-08-04T07-15-00Z-contract-block-cli-escalation.json"
	// Identity of that item's payload — the pair that distinguishes it from
	// the superseded 2026-07-30 instance already sitting in consumed/.
	wantItemID        = "contract-block-cli-escalation"
	wantItemCreatedAt = "2026-08-04T07:15:00Z"
	// The superseded instance, which must survive untouched.
	priorConsumedName = "2026-07-30-contract-block-cli-escalation.json"

	alignmentREADME = "docs/research/deliverable-alignment-2026-08/README.md"
	staleRowPhrase  = "fix still queued at 0.95"
	ladderRowAnchor = "Correction/retry ladder"
)

// inboxItem is the subset of the queue-item schema these predicates assert on.
type inboxItem struct {
	ID         string  `json:"id"`
	CreatedAt  string  `json:"created_at"`
	Priority   string  `json:"priority"`
	Weight     float64 `json:"weight"`
	Title      string  `json:"title"`
	ConsumedBy string  `json:"consumed_by"`
}

// readInboxItem decodes a queue item, failing the caller on unreadable or
// malformed JSON (a corrupt move is a failure, never a silent skip).
func readInboxItem(t *testing.T, path string) (inboxItem, bool) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		return inboxItem{}, false
	}
	var item inboxItem
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Errorf("inbox item %s is not valid JSON: %v", path, err)
		return inboxItem{}, false
	}
	return item, true
}

// findConsumedItem scans .evolve/inbox/consumed/ for the item matching the
// (id, created_at) identity pair. Filename-agnostic on purpose: the predicate
// asserts the item ARRIVED, not that the mover picked a particular name.
func findConsumedItem(t *testing.T, root string) (string, inboxItem, bool) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "inbox", "consumed")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Errorf("cannot read consumed dir %s: %v", dir, err)
		return "", inboxItem{}, false
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		item, ok := readInboxItem(t, path)
		if !ok {
			continue
		}
		if item.ID == wantItemID && item.CreatedAt == wantItemCreatedAt {
			return path, item, true
		}
	}
	return "", inboxItem{}, false
}

// ladderRow returns the correction/retry-ladder row of the alignment README.
func ladderRow(t *testing.T, root string) (string, bool) {
	t.Helper()
	path := filepath.Join(root, alignmentREADME)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("cannot read %s: %v", alignmentREADME, err)
		return "", false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, ladderRowAnchor) {
			return line, true
		}
	}
	return "", false
}

// TestC1304_001_InboxItemLeavesLiveQueue asserts the stranded item no longer
// sits in the live queue root. RED today: the file is present.
func TestC1304_001_InboxItemLeavesLiveQueue(t *testing.T) {
	root := acsassert.RepoRoot(t)
	live := filepath.Join(root, ".evolve", "inbox", inboxItemName)
	if _, err := os.Stat(live); err == nil {
		t.Errorf("inbox item still in the live queue at %s — cycle-1300 landed its fix (061345a4), so it must be consumed", live)
	} else if !os.IsNotExist(err) {
		t.Errorf("unexpected error stat-ing %s: %v", live, err)
	}
}

// TestC1304_002_InboxItemLandsInConsumed asserts the SAME item (matched by the
// id + created_at identity pair, not by filename) now lives under consumed/.
// Paired with 001 this makes a delete-instead-of-move indistinguishable from a
// failure. RED today: nothing in consumed/ carries created_at 2026-08-04.
func TestC1304_002_InboxItemLandsInConsumed(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path, item, ok := findConsumedItem(t, root)
	if !ok {
		t.Fatalf("no item with id=%q created_at=%q found under .evolve/inbox/consumed/ — the move did not happen (or the payload identity was rewritten)", wantItemID, wantItemCreatedAt)
	}
	if item.Priority != "P1" {
		t.Errorf("%s: priority=%q, want P1 — consumption must move the payload, not rewrite it", path, item.Priority)
	}
	if item.Weight != 0.96 {
		t.Errorf("%s: weight=%v, want 0.96 — consumption must move the payload, not rewrite it", path, item.Weight)
	}
	if !strings.Contains(item.Title, "escalation") {
		t.Errorf("%s: title=%q lost its subject — expected the original escalation title", path, item.Title)
	}
}

// TestC1304_003_ConsumedItemCitesLanding asserts the consumed item carries a
// consumed_by provenance field naming the cycle-1300 landing evidence, matching
// the convention the 2026-07-30 instance already established. Without this the
// move is an unexplained deletion. RED today: the item is not in consumed/ at all.
func TestC1304_003_ConsumedItemCitesLanding(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path, item, ok := findConsumedItem(t, root)
	if !ok {
		t.Fatalf("item id=%q created_at=%q absent from .evolve/inbox/consumed/", wantItemID, wantItemCreatedAt)
	}
	if strings.TrimSpace(item.ConsumedBy) == "" {
		t.Fatalf("%s: consumed_by is empty — every consumed item must record why it was closed", path)
	}
	// The landing evidence: the dossier commit and the code that implements the fix.
	if !strings.Contains(item.ConsumedBy, "061345a4") {
		t.Errorf("%s: consumed_by does not cite the cycle-1300 landing commit 061345a4; got %q", path, item.ConsumedBy)
	}
	if !strings.Contains(item.ConsumedBy, "contract_escalation") {
		t.Errorf("%s: consumed_by does not cite the implementing code (go/internal/core/contract_escalation.go); got %q", path, item.ConsumedBy)
	}
	// The LIVE EVIDENCE addendum the item itself demanded — the no-escalation-target
	// remedy. Closing the item without citing it would close it on a partial fix.
	if !strings.Contains(item.ConsumedBy, "salvage") {
		t.Errorf("%s: consumed_by does not address the LIVE EVIDENCE 'no escalation target' remedy (contract_salvage_retry); got %q", path, item.ConsumedBy)
	}
}

// TestC1304_004_PriorConsumedInstanceUntouched is the negative/collateral guard:
// the superseded 2026-07-30 instance must survive with its own provenance intact.
// A move that clobbers the destination filename would trip this.
func TestC1304_004_PriorConsumedInstanceUntouched(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, ".evolve", "inbox", "consumed", priorConsumedName)
	item, ok := readInboxItem(t, path)
	if !ok {
		t.Fatalf("the superseded consumed instance %s is missing — consuming the new item must not clobber it", priorConsumedName)
	}
	if item.CreatedAt != "2026-07-29T03:30:00Z" {
		t.Errorf("%s: created_at=%q, want 2026-07-29T03:30:00Z — the prior instance was overwritten", path, item.CreatedAt)
	}
	if !strings.Contains(item.ConsumedBy, "console-2026-07-30") {
		t.Errorf("%s: prior instance lost its consumed_by provenance; got %q", path, item.ConsumedBy)
	}
}

// TestC1304_005_AlignmentREADMEDropsStaleQueuedClaim asserts the stale
// "fix still queued at 0.95" claim is gone from the alignment README.
// RED today: line 63 carries it.
func TestC1304_005_AlignmentREADMEDropsStaleQueuedClaim(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, alignmentREADME)
	if !acsassert.FileNotContains(t, path, staleRowPhrase) {
		t.Errorf("%s still claims %q — the fix landed in cycle-1300 (061345a4)", alignmentREADME, staleRowPhrase)
	}
}

// TestC1304_006_LadderRowCitesLandedStatus asserts the correction/retry-ladder
// row still EXISTS and now states the landed status. Deleting the row to satisfy
// 005 fails here. RED today: the row exists but cites none of the landing markers.
func TestC1304_006_LadderRowCitesLandedStatus(t *testing.T) {
	root := acsassert.RepoRoot(t)
	row, ok := ladderRow(t, root)
	if !ok {
		t.Fatalf("%s: the %q row is gone — the fix is to UPDATE the row, not delete it", alignmentREADME, ladderRowAnchor)
	}
	if strings.Contains(row, staleRowPhrase) {
		t.Errorf("%s: ladder row still carries the stale claim: %s", alignmentREADME, row)
	}
	// Any one landing marker satisfies this; the wording is the builder's call.
	markers := []string{"cycle-1300", "061345a4", "landed", "LANDED", "live"}
	hit := false
	for _, m := range markers {
		if strings.Contains(row, m) {
			hit = true
			break
		}
	}
	if !hit {
		t.Errorf("%s: ladder row cites no landing evidence (want one of %v); got: %s", alignmentREADME, markers, row)
	}
}

// TestC1304_007_AlignmentREADMENoCollateralDamage is the scope guard: the edit
// touches one row, so the document's other portfolio rows and its post-tail
// baseline section must be unchanged. GREEN today by construction — it exists to
// stay green, catching a rewrite that "fixes" line 63 by gutting the file.
func TestC1304_007_AlignmentREADMENoCollateralDamage(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, alignmentREADME)
	survivors := []string{
		"Unified deliverable contract",
		"Contract gate + self-check + breaker",
		"Verdict sentinel + file-authoritative transport",
		"EGPS / ACS predicates",
		"Adversarial audit",
		"bad_verdict",
	}
	for _, s := range survivors {
		if !acsassert.FileContains(t, path, s) {
			t.Errorf("%s: lost content %q — the ladder-row fix must not rewrite the rest of the document", alignmentREADME, s)
		}
	}
}
