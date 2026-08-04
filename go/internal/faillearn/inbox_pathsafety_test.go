package faillearn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// inbox_pathsafety_test.go — RED contract for cycle-1282 D7
// (.evolve/runs/cycle-1279/audit-report.md, LOW): WithInbox concatenates
// `it.ID + ".json"` into a path with no sanitisation (inbox.go:81). The sole
// current caller is safe (remediationSlug emits [a-z0-9-] only), so this is not
// presently exploitable — it is a trap on a NEWLY EXPORTED API, and the next
// caller is the one that falls in.
//
// The rule: an id that is not a bare filename is rejected loudly, exactly as an
// empty id already is. Rejection (not sanitisation-and-write) is the right
// shape here — a silently rewritten id produces an item nobody can address by
// the id they filed it under, which is the erasure this package exists to stop.

// traversalIDs enumerates the shapes that must never reach the filesystem as a
// path: parent escapes, absolute paths, and nested separators.
func traversalIDs() []string {
	return []string{
		"../escaped",
		"../../escaped",
		"/tmp/absolute",
		"nested/child",
		"..",
	}
}

// TestWriteArtifacts_InboxRejectsPathEscapingID — D7 negative. Each shape must
// return an error naming the offending id, and must leave nothing outside the
// inbox directory.
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
				WithInbox(inbox, []InboxItem{{ID: id, Title: "t", Kind: "bug", Priority: "H", InjectedBy: "faillearn-failure-floor"}}),
			)
			if err == nil {
				t.Fatalf("id %q was accepted — an id that is not a bare filename must be rejected, like the empty-id case already is", id)
			}
			if !strings.Contains(err.Error(), id) {
				t.Errorf("the rejection must name the offending id; got %v", err)
			}

			// Nothing may have escaped: the only thing under root/inbox is
			// whatever the writer legitimately created, and root itself must
			// carry no stray *.json.
			strays, _ := filepath.Glob(filepath.Join(root, "*.json"))
			if len(strays) > 0 {
				t.Errorf("id %q wrote outside the inbox directory: %v", id, strays)
			}
		})
	}
}

// TestWriteArtifacts_InboxAcceptsOrdinaryID — the POSITIVE half: the shape
// remediationSlug actually produces must still be written, or the D7 guard
// would break F1(ii) while closing a latent trap.
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
		WithInbox(inbox, []InboxItem{{ID: id, Title: "t", Kind: "bug", Priority: "H", InjectedBy: "faillearn-failure-floor"}}),
	); err != nil {
		t.Fatalf("an ordinary slug id must still be written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(inbox, id+".json")); err != nil {
		t.Errorf("item %s did not land in the inbox: %v", id, err)
	}
}
