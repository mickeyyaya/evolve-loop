package faillearn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func traversalIDs() []string {
	return []string{
		"../escaped",
		"../../escaped",
		"/tmp/absolute",
		"nested/child",
		"..",
	}
}

func TestWriteArtifacts_InboxRejectsPathEscapingID(t *testing.T) {
	for _, id := range traversalIDs() {
		t.Run(id, func(t *testing.T) {
			root := t.TempDir()
			inbox := filepath.Join(root, "inbox")
			runDir := filepath.Join(root, "run")
			lessons := filepath.Join(root, "lessons")
			for _, d := range []string{inbox, runDir, lessons} {
				if err := os.MkdirAll(d, 0o755); err != nil {
					t.Fatalf("mkdir %s: %v", d, err)
				}
			}

			err := WriteArtifacts(
				FailureEvent{Cycle: 1279, FailedPhase: "audit", Scope: ScopePhase, Classification: "deliverable-rejected", Verdict: "FAIL", Summary: "s"},
				runDir, lessons,
				WithInbox(inbox, []InboxItem{{ID: id, Title: "t", Kind: "bug", Priority: "H", PriorityClass: "correctness", InjectedBy: "faillearn-failure-floor"}}),
			)
			if err == nil {
				t.Fatalf("id %q was accepted — an id that is not a bare filename must be rejected, like the empty-id case already is", id)
			}
			if !strings.Contains(err.Error(), id) {
				t.Errorf("the rejection must name the offending id; got %v", err)
			}

			strays, _ := filepath.Glob(filepath.Join(root, "*.json"))
			if len(strays) > 0 {
				t.Errorf("id %q wrote outside the inbox directory: %v", id, strays)
			}
		})
	}
}

func TestWriteArtifacts_InboxAcceptsOrdinaryID(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, "inbox")
	runDir := filepath.Join(root, "run")
	lessons := filepath.Join(root, "lessons")
	for _, d := range []string{inbox, runDir, lessons} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	const id = "retro-1279-reconcile-truncate-writes-the-ledger"
	if err := WriteArtifacts(
		FailureEvent{Cycle: 1279, FailedPhase: "audit", Scope: ScopePhase, Classification: "deliverable-rejected", Verdict: "FAIL", Summary: "s"},
		runDir, lessons,
		WithInbox(inbox, []InboxItem{{ID: id, Title: "t", Kind: "bug", Priority: "H", PriorityClass: "correctness", InjectedBy: "faillearn-failure-floor"}}),
	); err != nil {
		t.Fatalf("an ordinary slug id must still be written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(inbox, id+".json")); err != nil {
		t.Errorf("item %s did not land in the inbox: %v", id, err)
	}
}
