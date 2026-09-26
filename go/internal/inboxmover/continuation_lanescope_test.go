package inboxmover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

func seedRegistry(t *testing.T, root, scopeID, sha string, cycle int) continuation.Continuation {
	t.Helper()
	c := continuation.Continuation{
		Worktree:    filepath.Join(root, "wt"),
		Branch:      "evolve/cycle-" + strconv.Itoa(cycle),
		SnapshotSHA: sha,
		BaseSHA:     "base1111111111111111111111111111111111111",
		Cycle:       cycle,
	}
	if err := continuation.WriteRegistryEntry(root, scopeID, c); err != nil {
		t.Fatalf("seed registry %s: %v", scopeID, err)
	}
	return c
}

func seedClaim(t *testing.T, root string, cycle int, taskID, stampSHA string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "inbox", "processing", "cycle-"+strconv.Itoa(cycle))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	item := map[string]any{"id": taskID}
	if stampSHA != "" {
		item["continuation"] = continuation.Continuation{
			Branch: "evolve/cycle-claim", SnapshotSHA: stampSHA, Cycle: cycle,
		}
	}
	body, _ := json.Marshal(item)
	if err := os.WriteFile(filepath.Join(dir, taskID+".json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveContinuationForScope_FallsBackToLaneScopeRegistry(t *testing.T) {
	root := t.TempDir()
	want := seedRegistry(t, root, "chain-boundary-loop", "5555555555555555555555555555555555555555", 1078)

	got := ResolveContinuationForScope(Options{ProjectRoot: root}, 1102, []string{"chain-boundary-loop"})
	if got == nil {
		t.Fatal("a lane-scope-only scope with a registered binding must resolve (cycle-1078 orphan class)")
	}
	if got.SnapshotSHA != want.SnapshotSHA || got.Cycle != want.Cycle {
		t.Errorf("resolved %+v, want snapshot %q cycle %d", got, want.SnapshotSHA, want.Cycle)
	}
	if c := ResolveContinuation(Options{ProjectRoot: root}, 1102); c != nil {
		t.Errorf("claim-only resolution must still be nil here, got %+v", c)
	}
}

// The claim uses the lane's own id: a claim outside the lane scope is a peer's and is skipped.
func TestResolveContinuationForScope_ClaimWinsOverRegistry(t *testing.T) {
	root := t.TempDir()
	seedRegistry(t, root, "scope-a", "6666666666666666666666666666666666666666", 1078)
	seedClaim(t, root, 1102, "scope-a", "7777777777777777777777777777777777777777")

	got := ResolveContinuationForScope(Options{ProjectRoot: root}, 1102, []string{"scope-a"})
	if got == nil {
		t.Fatal("claim stamp must resolve")
	}
	if got.SnapshotSHA != "7777777777777777777777777777777777777777" {
		t.Errorf("claim-stamped continuation must win over the registry fallback, got %+v", got)
	}
}

func TestResolveContinuationForScope_UnstampedClaimStillFallsBack(t *testing.T) {
	root := t.TempDir()
	seedRegistry(t, root, "scope-a", "8888888888888888888888888888888888888888", 1078)
	seedClaim(t, root, 1102, "task-a", "")

	got := ResolveContinuationForScope(Options{ProjectRoot: root}, 1102, []string{"scope-a"})
	if got == nil || got.SnapshotSHA != "8888888888888888888888888888888888888888" {
		t.Errorf("an unstamped claim must not suppress the lane-scope fallback, got %+v", got)
	}
}

func TestResolveContinuationForScope_NoBindingIsNil(t *testing.T) {
	root := t.TempDir()
	seedRegistry(t, root, "scope-a", "9999999999999999999999999999999999999999", 1078)

	cases := []struct {
		name   string
		scopes []string
	}{
		{"unknown scope", []string{"scope-zzz"}},
		{"empty list", []string{}},
		{"nil list", nil},
		{"blank id", []string{""}},
		{"blank and unknown", []string{"", "scope-zzz"}},
	}
	for _, tc := range cases {
		if got := ResolveContinuationForScope(Options{ProjectRoot: root}, 1102, tc.scopes); got != nil {
			t.Errorf("%s: must resolve nil, got %+v (cross-lane adoption risk)", tc.name, got)
		}
	}
}

func TestResolveContinuationForScope_ScopeOrderIsDeterministic(t *testing.T) {
	root := t.TempDir()
	seedRegistry(t, root, "scope-b", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 1078)
	seedRegistry(t, root, "scope-c", "cccccccccccccccccccccccccccccccccccccccc", 1079)

	got := ResolveContinuationForScope(Options{ProjectRoot: root}, 1102, []string{"scope-a", "scope-b", "scope-c"})
	if got == nil || got.SnapshotSHA != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Fatalf("first bound scope in declared order must win, got %+v", got)
	}
	got = ResolveContinuationForScope(Options{ProjectRoot: root}, 1102, []string{"scope-c", "scope-b"})
	if got == nil || got.SnapshotSHA != "cccccccccccccccccccccccccccccccccccccccc" {
		t.Errorf("resolution must follow the declared scope order, got %+v", got)
	}
}

func TestResolveContinuationForScope_EmptySnapshotIsNotABinding(t *testing.T) {
	root := t.TempDir()
	if err := continuation.WriteRegistryEntry(root, "scope-a", continuation.Continuation{Cycle: 1078}); err != nil {
		t.Fatal(err)
	}
	if got := ResolveContinuationForScope(Options{ProjectRoot: root}, 1102, []string{"scope-a"}); got != nil {
		t.Errorf("an entry with no snapshot_sha is not a binding, got %+v", got)
	}
}

func TestResolveContinuation_ClaimOnlyPathUnchanged(t *testing.T) {
	root := t.TempDir()
	seedRegistry(t, root, "scope-a", "dddddddddddddddddddddddddddddddddddddddd", 1078)

	if got := ResolveContinuation(Options{ProjectRoot: root}, 1102); got != nil {
		t.Errorf("ResolveContinuation must remain claim-only, got %+v", got)
	}
	seedClaim(t, root, 1102, "task-a", "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee")
	got := ResolveContinuation(Options{ProjectRoot: root}, 1102)
	if got == nil || got.SnapshotSHA != "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee" {
		t.Errorf("claim-stamped resolution regressed, got %+v", got)
	}
}

func TestResolveContinuationForScope_CorruptRegistryIsNilNotPanic(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(continuation.RegistryPath(root), []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ResolveContinuationForScope(Options{ProjectRoot: root}, 1102, []string{"scope-a"}); got != nil {
		t.Errorf("corrupt registry must resolve nil (fresh start), got %+v", got)
	}
}
