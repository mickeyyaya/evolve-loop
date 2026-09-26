package inboxmover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

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
	// The filename is the id, so a test controls which claim the sorted walk meets first.
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

func TestResolveContinuationForScope_MatchesAnyDeclaredScopeID(t *testing.T) {
	root := t.TempDir()
	writeClaim(t, root, 1544, "zzz-last-alphabetically", true, 1400)

	got := ResolveContinuationForScope(Options{ProjectRoot: root},
		1544, []string{"lost-ship-closeout-failure", "zzz-last-alphabetically"})

	if got == nil || got.Cycle != 1400 {
		t.Fatalf("a claim matching any declared scope id must be adopted; got %+v", got)
	}
}

// The peer's claim sorts first, so the lane's own binding is reachable only if the skip continues the walk.
func TestResolveContinuationForScope_ContinuesPastAPeerClaimToItsOwn(t *testing.T) {
	root := t.TempDir()
	writeClaim(t, root, 1600, "aaa-peer-lane-task", true, 1500)
	writeClaim(t, root, 1600, "zzz-this-lane-task", true, 1590)

	got := ResolveContinuationForScope(Options{ProjectRoot: root}, 1600, []string{"zzz-this-lane-task"})

	if got == nil {
		t.Fatalf("an out-of-scope claim met first must be SKIPPED, not abandon the lane's own binding")
	}
	if got.Cycle != 1590 {
		t.Fatalf("resolved the wrong binding: want the lane's own (cycle 1590), got cycle %d", got.Cycle)
	}
}

func TestResolveContinuationForScope_ClaimWithEmptySnapshotIsNotABinding(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "inbox", "processing", "cycle-1601")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// A stamp with an empty snapshot_sha (a half-written stamp), which writeClaim cannot express.
	body := []byte(`{"id":"this-lane-task","continuation":{"cycle":1590,"snapshot_sha":"","branch":"cycle-1590"}}`)
	if err := os.WriteFile(filepath.Join(dir, "this-lane-task.json"), body, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if got := ResolveContinuationForScope(Options{ProjectRoot: root}, 1601, []string{"this-lane-task"}); got != nil {
		t.Fatalf("a claim with no snapshot ref is not resumable work and must not bind; got %+v", got)
	}
}
