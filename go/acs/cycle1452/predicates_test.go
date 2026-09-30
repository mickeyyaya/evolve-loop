//go:build acs

package cycle1452

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string { return filepath.Join(acsassert.RepoRoot(t), "go") }

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

func TestC1452_003_MarkedItemIsConsumedByItsOwnLandingShip(t *testing.T) {
	shipFixtures(t, "TestPromoteInbox_ClosesInboxMarkerConsumes",
		"TestPromoteInbox_ClosesInboxMarkerConsumesUnnamedItemOnLandedShip",
		"TestPromoteInbox_ClosesInboxMarkerConsumesWithNoTriageDecision",
	)
}

func TestC1452_004_UnlandedOrUnmarkedLandingConsumesNothing(t *testing.T) {
	shipFixtures(t, "TestPromoteInbox_(ClosesInboxMarkerSkippedOnUnlandedShip|LandedShipWithoutMarkerConsumesOnlyTriageNamedItems|AbsentBuildReportIsNotAnError)",
		"TestPromoteInbox_ClosesInboxMarkerSkippedOnUnlandedShip",
		"TestPromoteInbox_LandedShipWithoutMarkerConsumesOnlyTriageNamedItems",
		"TestPromoteInbox_AbsentBuildReportIsNotAnError",
	)
}

// acs-predicate: config-check — a documentation-presence criterion has no
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
	ref := filepath.Join(root, "agents", "evolve-builder-reference.md")
	if !acsassert.FileContainsAny(ref, "partial", "best-effort", "best effort") {
		t.Errorf("agents/evolve-builder-reference.md documents the marker without the must-NOT-on-a-partial-landing caveat — the doc then licenses false-positive consumption")
	}
}
