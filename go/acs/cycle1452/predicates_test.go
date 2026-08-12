//go:build acs

// Package cycle1452 materialises the cycle-1452 acceptance criteria for the one
// fleet-scoped todo-id pinned to this lane: `consumption-rides-landing-ship`
// (weight 0.92, pipeline-repair).
//
// The defect: consuming an inbox item is a separate act from the ship that
// closes it, so forgetting is always possible. Live instance 2026-08-12 —
// schema-aligned-salvage-layer landed in #453, its item was never consumed, and
// wave cycle-1448 re-picked already-shipped work as live scope.
//
// The fix under test: a builder-authored, line-anchored `Closes-Inbox: <id>`
// marker in build-report.md, unioned into the committed set inside promoteInbox
// under the EXACT existing cycle-598 landing gate.
//
// Predicate strategy — every predicate exercises the system (the cycle-85
// degenerate-predicate ban):
//
//   - 001–002 CALL the real parser over real report bodies: 001 pins the
//     positive contract, 002 is the anti-false-positive half (prose that merely
//     mentions the marker must consume nothing). A substring-anywhere
//     implementation greens 001 and reds 002.
//   - 003–004 shell the ship package's own promoteInbox fixtures — ONE named
//     package narrowed with `-run`, per the flaky-predicate-shape rules — and
//     assert the named tests were actually SELECTED (Go's `-run` is a substring
//     match, so an unselected test is a silent pass). 003 is the consume half,
//     004 the must-NOT-consume half (unlanded ship, unmarked item).
//   - 005 is the documentation criterion: a text-presence check by nature, so it
//     carries an explicit waiver AND asserts git-TRACKING, not just disk
//     presence (cycle-93: a gitignored file is dropped at ship).
package cycle1452

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goDir returns the module root inside the cycle worktree.
func goDir(t *testing.T) string { return filepath.Join(acsassert.RepoRoot(t), "go") }

// TestC1452_001_ClosesInboxMarkerParsesAnchoredIDs — the parser's positive
// contract, called directly: line-anchored markers (bullet/indent/case tolerant)
// yield their comma-separated ids, deduped, first-seen order preserved.
func TestC1452_001_ClosesInboxMarkerParsesAnchoredIDs(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		{"plain marker line", "# Build Report\n\nCloses-Inbox: consumption-rides-landing-ship\n",
			[]string{"consumption-rides-landing-ship"}},
		{"comma separated", "Closes-Inbox: alpha-item, beta.item\n", []string{"alpha-item", "beta.item"}},
		{"bullet, backticks, mixed case", "- closes-inbox: `bullet-item`\nCLOSES-INBOX: shouty-item\n",
			[]string{"bullet-item", "shouty-item"}},
		{"dedup keeps first-seen order", "Closes-Inbox: b-item, a-item\nCloses-Inbox: a-item\n",
			[]string{"b-item", "a-item"}},
	}
	for _, c := range cases {
		got := inboxmover.ClosesInboxIDs([]byte(c.body))
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: ClosesInboxIDs(%q) = %#v, want %#v — a landing cannot close an item it cannot name",
				c.name, c.body, got, c.want)
		}
	}
}

// TestC1452_002_ClosesInboxMarkerRejectsProseAndNearMisses — the
// anti-false-positive half, and the strongest anti-no-op signal here. Consuming
// an item the landing did NOT close is silent data loss (the next wave never
// sees the work again), strictly worse than today's wasted bookkeeping cycle.
// `connects_to` is a hint, not a predicate, so closure may never be inferred.
func TestC1452_002_ClosesInboxMarkerRejectsProseAndNearMisses(t *testing.T) {
	cases := []struct{ name, body string }{
		{"mid-sentence prose", "This landing Closes-Inbox: not-really-closed, probably.\n"},
		{"documentation of the convention", "Emit \"Closes-Inbox: <id>\" only on a full landing.\n"},
		{"marker with no ids", "Closes-Inbox:\nCloses-Inbox:   \nCloses-Inbox: ,,\n"},
		{"prose spillover is not an id", "Closes-Inbox: the salvage layer item\n"},
		{"near-miss marker names", "Closes-Inbox-Maybe: nope\nClosesInbox: nope\nCloses: nope\n"},
		{"empty body", ""},
		{"ordinary report", "# Build Report\n\nAll tests green. No inbox item closed.\n"},
	}
	for _, c := range cases {
		if got := inboxmover.ClosesInboxIDs([]byte(c.body)); len(got) != 0 {
			t.Errorf("%s: ClosesInboxIDs(%q) = %#v, want none — false-positive consumption erases work that never shipped",
				c.name, c.body, got)
		}
	}
}

// shipFixtures asserts the given -run pattern selects AND passes every named
// promoteInbox fixture in the ship package. Selection is asserted explicitly:
// Go's -run is a substring match, so a renamed or absent test would otherwise
// pass vacuously (the cycle-1446 L3 class).
func shipFixtures(t *testing.T, pattern string, names ...string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", goDir(t), "test", "-count=1", "-run", pattern, "-v", "./internal/phases/ship")
	if err != nil || code != 0 {
		t.Errorf("`-run %s ./internal/phases/ship` is not green (exit=%d err=%v)\n%s\n%s",
			pattern, code, err, stdout, stderr)
	}
	for _, name := range names {
		if !strings.Contains(stdout, "=== RUN   "+name) {
			t.Errorf("`-run %s` never selected %s — the fixture that proves this criterion did not run\nstdout:\n%s",
				pattern, name, stdout)
		}
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("%s did not pass under `-run %s`\nstdout:\n%s", name, pattern, stdout)
		}
	}
}

// TestC1452_003_MarkedItemIsConsumedByItsOwnLandingShip — the wiring proof named
// in the inbox item's own `fix` field ("a fixture PASS landing an item's
// predicates must consume it transactionally"). Drives the REAL promoteInbox
// production path through the ship package's fixtures, including the
// decision-less lane shape that produced the #453 live instance.
func TestC1452_003_MarkedItemIsConsumedByItsOwnLandingShip(t *testing.T) {
	shipFixtures(t, "TestPromoteInbox_ClosesInboxMarkerConsumes",
		"TestPromoteInbox_ClosesInboxMarkerConsumesUnnamedItemOnLandedShip",
		"TestPromoteInbox_ClosesInboxMarkerConsumesWithNoTriageDecision",
	)
}

// TestC1452_004_UnlandedOrUnmarkedLandingConsumesNothing — the other half of the
// same `fix` sentence ("a partial landing must NOT"). Two ways to over-consume:
// a second, weaker gate on the marker path (cycle-598 reopened), or inferring
// closure from the diff. Both are fixture-pinned here, plus the degrade case
// where build-report.md is absent entirely.
func TestC1452_004_UnlandedOrUnmarkedLandingConsumesNothing(t *testing.T) {
	shipFixtures(t, "TestPromoteInbox_(ClosesInboxMarkerSkippedOnUnlandedShip|LandedShipWithoutMarkerConsumesOnlyTriageNamedItems|AbsentBuildReportIsNotAnError)",
		"TestPromoteInbox_ClosesInboxMarkerSkippedOnUnlandedShip",
		"TestPromoteInbox_LandedShipWithoutMarkerConsumesOnlyTriageNamedItems",
		"TestPromoteInbox_AbsentBuildReportIsNotAnError",
	)
}

// TestC1452_005_MarkerConventionIsDocumentedAndTracked — Task 4. A convention no
// Builder is told about is not a mechanism, so the persona reference and the
// canonical protocol doc must both carry the marker AND the must-NOT-on-a-partial
// -landing caveat. Tracking is asserted by subprocess because a gitignored doc is
// silently dropped at ship (cycle-93).
//
// acs-predicate: config-check — a documentation-presence criterion has no
// runtime behaviour to invoke; the executable half is the git-tracking probe.
func TestC1452_005_MarkerConventionIsDocumentedAndTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docs := []string{
		filepath.Join("agents", "evolve-builder-reference.md"),
		filepath.Join("docs", "architecture", "inbox-injection-protocol.md"),
	}
	for _, rel := range docs {
		abs := filepath.Join(root, rel)
		if !acsassert.FileExists(t, abs) {
			t.Errorf("%s missing — the marker convention has no home", rel)
			continue
		}
		if !acsassert.FileContains(t, abs, "Closes-Inbox:") {
			t.Errorf("%s does not document the `Closes-Inbox:` marker — a convention the Builder never reads is not a mechanism", rel)
		}
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
			t.Errorf("%s is untracked — it may be gitignored and dropped at ship (cycle-93)", rel)
		}
	}
	// The partial-landing caveat is the load-bearing sentence: emitting the
	// marker on a best-effort landing is exactly the false-positive consumption
	// predicate 002 forbids at the parser level.
	ref := filepath.Join(root, "agents", "evolve-builder-reference.md")
	if !acsassert.FileContainsAny(ref, "partial", "best-effort", "best effort") {
		t.Errorf("agents/evolve-builder-reference.md documents the marker without the must-NOT-on-a-partial-landing caveat — the doc then licenses false-positive consumption")
	}
}
