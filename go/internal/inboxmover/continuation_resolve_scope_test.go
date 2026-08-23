package inboxmover

// continuation_resolve_scope_test.go — a lane must not adopt another lane's
// continuation.
//
// Live incident (wave-20260822a-verify): cycle-1536's lane-scope.json declared
// todo_ids ["pipeline-defect-infra-systemic"], but its audit report reads
// "Task (continuation-bound): pipeline-replay-contract-boundary (ADR-0076
// continuation of cycle 1532)" — a peer lane's task. It then shipped adcbddb2
// carrying .evolve/evals/pipeline-replay-contract-boundary.md, a file belonging
// to cycle-1535's lane. cycle-1535 could no longer rebase onto main (its own
// eval file was already there under someone else's commit), hit a non-derived
// conflict, routed to the debugger, and lost a PASS cycle's landing entirely.
//
// Cause: ResolveContinuationForScope takes scopeIDs and never applies them to
// the inbox-claim path. ResolveContinuation walks processing/cycle-N/*.json in
// sorted order and returns the FIRST claim carrying a snapshot — skipping
// unstamped ones — so a stamped claim for a DIFFERENT scope wins over the
// lane's own scope entirely.
//
// The claim-first ordering is deliberate (G1) and stays. What changes is that a
// SCOPED lane only adopts claims belonging to its own scope. An unscoped cycle
// (sequential/solo, empty scopeIDs) keeps today's behavior byte-for-byte —
// there is no lane identity to violate there, and narrowing it would break G1.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

// writeClaim drops one claimed inbox item into processing/cycle-N/.
// stamped=false writes an item with no continuation, which the resolver skips.
func writeClaim(t *testing.T, root string, cycle int, id string, stamped bool, fromCycle int) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "inbox", "processing", "cycle-"+itoa(cycle))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	item := map[string]any{"id": id, "title": id}
	if stamped {
		item["continuation"] = continuation.Continuation{
			Cycle:       fromCycle,
			SnapshotSHA: "deadbeefcafe",
			Branch:      "cycle-" + itoa(fromCycle),
			Worktree:    "/tmp/wt-" + itoa(fromCycle),
		}
	}
	b, _ := json.MarshalIndent(item, "", "  ")
	// Filename deliberately id-prefixed: the resolver walks sorted names, so the
	// test controls which claim it meets first.
	if err := os.WriteFile(filepath.Join(dir, id+".json"), b, 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

// THE headline regression, in the live shape: the lane's OWN claim is unstamped
// and sorts first; a PEER's claim is stamped. Today the peer's wins.
func TestResolveContinuationForScope_DoesNotAdoptAPeerLanesClaim(t *testing.T) {
	root := t.TempDir()
	writeClaim(t, root, 1536, "pipeline-defect-infra-systemic", false, 0)      // this lane's own — no stamp
	writeClaim(t, root, 1536, "pipeline-replay-contract-boundary", true, 1532) // a PEER lane's — stamped

	got := ResolveContinuationForScope(Options{ProjectRoot: root}, 1536, []string{"pipeline-defect-infra-systemic"})

	if got != nil {
		t.Fatalf("lane scoped to %q must not adopt the peer scope's continuation (cycle %d) — that is how cycle-1536 shipped cycle-1535's eval file and destroyed its landing; got %+v",
			"pipeline-defect-infra-systemic", got.Cycle, got)
	}
}

// The lane's OWN stamped claim must still be adopted — the fix must not disarm
// legitimate continuation.
func TestResolveContinuationForScope_AdoptsItsOwnClaim(t *testing.T) {
	root := t.TempDir()
	writeClaim(t, root, 1536, "pipeline-defect-infra-systemic", true, 1500)
	writeClaim(t, root, 1536, "pipeline-replay-contract-boundary", true, 1532)

	got := ResolveContinuationForScope(Options{ProjectRoot: root}, 1536, []string{"pipeline-defect-infra-systemic"})

	if got == nil {
		t.Fatalf("a lane's OWN stamped claim must still be adopted")
	}
	if got.Cycle != 1500 {
		t.Fatalf("adopted the wrong claim: want the lane's own (cycle 1500), got cycle %d", got.Cycle)
	}
}

// NO-REGRESSION (G1): an UNSCOPED cycle — sequential/solo, no lane identity —
// keeps today's behavior exactly: the first stamped claim wins.
func TestResolveContinuationForScope_UnscopedCycleIsUnchanged(t *testing.T) {
	root := t.TempDir()
	writeClaim(t, root, 1536, "some-other-task", true, 1532)

	for _, scopes := range [][]string{nil, {}, {"   "}} {
		got := ResolveContinuationForScope(Options{ProjectRoot: root}, 1536, scopes)
		if got == nil || got.Cycle != 1532 {
			t.Fatalf("an unscoped cycle must keep legacy claim-first behavior (scopes=%v); got %+v", scopes, got)
		}
	}
}

// A lane declaring several ids adopts a claim matching ANY of them.
func TestResolveContinuationForScope_MatchesAnyDeclaredScopeID(t *testing.T) {
	root := t.TempDir()
	writeClaim(t, root, 1544, "zzz-last-alphabetically", true, 1400)

	got := ResolveContinuationForScope(Options{ProjectRoot: root},
		1544, []string{"lost-ship-closeout-failure", "zzz-last-alphabetically"})

	if got == nil || got.Cycle != 1400 {
		t.Fatalf("a claim matching any declared scope id must be adopted; got %+v", got)
	}
}

// The walk must CONTINUE past an out-of-scope claim, not abandon the lane.
//
// Ordering is load-bearing and easy to get accidentally right: the resolver
// walks sorted filenames, so a test whose own-scope claim happens to sort first
// never exercises the skip at all. Here the PEER's claim sorts FIRST ("aaa-"
// before "zzz-"), so the lane's own binding is only reachable if the skip is a
// `continue` rather than a `return nil`.
func TestResolveContinuationForScope_ContinuesPastAPeerClaimToItsOwn(t *testing.T) {
	root := t.TempDir()
	writeClaim(t, root, 1600, "aaa-peer-lane-task", true, 1500) // sorts FIRST, out of scope
	writeClaim(t, root, 1600, "zzz-this-lane-task", true, 1590) // sorts LAST, in scope

	got := ResolveContinuationForScope(Options{ProjectRoot: root}, 1600, []string{"zzz-this-lane-task"})

	if got == nil {
		t.Fatalf("an out-of-scope claim met first must be SKIPPED, not abandon the lane's own binding")
	}
	if got.Cycle != 1590 {
		t.Fatalf("resolved the wrong binding: want the lane's own (cycle 1590), got cycle %d", got.Cycle)
	}
}

// A claim carrying a continuation object with NO snapshot ref is not resumable
// work, so it is not a binding — on the CLAIM path, not only the registry path.
// Without this, dropping the SnapshotSHA check hands the lane an unusable
// binding and the cycle seeds from an empty ref.
func TestResolveContinuationForScope_ClaimWithEmptySnapshotIsNotABinding(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "inbox", "processing", "cycle-1601")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Continuation present but snapshot_sha empty — the shape writeClaim cannot
	// express, and exactly what a half-written stamp looks like on disk.
	body := []byte(`{"id":"this-lane-task","continuation":{"cycle":1590,"snapshot_sha":"","branch":"cycle-1590"}}`)
	if err := os.WriteFile(filepath.Join(dir, "this-lane-task.json"), body, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if got := ResolveContinuationForScope(Options{ProjectRoot: root}, 1601, []string{"this-lane-task"}); got != nil {
		t.Fatalf("a claim with no snapshot ref is not resumable work and must not bind; got %+v", got)
	}
}
