package inboxmover

// continuation_lanescope_test.go — ADR-0076 slice C, G2 (cycle-1104) resolve
// side. G1's ResolveContinuation reads ONLY this cycle's inbox processing
// claims, so a lane whose scope came from the wave planner (no claim file) can
// never resolve a binding — cycle-1078's preserved snapshot was orphaned for
// exactly that reason. ResolveContinuationForScope adds the second identity
// class: claims FIRST (G1 semantics untouched), then the scope-id-keyed
// registry for the lane's todo ids.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

// seedRegistry stamps a registry binding for scopeID under root.
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

// seedClaim writes a processing claim for cycle, optionally carrying a stamp.
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

// TestResolveContinuationForScope_FallsBackToLaneScopeRegistry — the headline
// AC: a cycle with NO processing claim at all still resolves the binding its
// lane-scope todo id carries. This is the cycle-1078 case, end to end.
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
	// Proof the fallback is real and not an accident of the claim path: the
	// claim-only resolver still finds nothing for the same cycle.
	if c := ResolveContinuation(Options{ProjectRoot: root}, 1102); c != nil {
		t.Errorf("claim-only resolution must still be nil here, got %+v", c)
	}
}

// TestResolveContinuationForScope_ClaimWinsOverRegistry — G1 is unaffected:
// when a claim carries a stamp, that stamp is returned even though the lane
// scope also has a registry binding. Ordering, not replacement.
func TestResolveContinuationForScope_ClaimWinsOverRegistry(t *testing.T) {
	root := t.TempDir()
	seedRegistry(t, root, "scope-a", "6666666666666666666666666666666666666666", 1078)
	seedClaim(t, root, 1102, "task-a", "7777777777777777777777777777777777777777")

	got := ResolveContinuationForScope(Options{ProjectRoot: root}, 1102, []string{"scope-a"})
	if got == nil {
		t.Fatal("claim stamp must resolve")
	}
	if got.SnapshotSHA != "7777777777777777777777777777777777777777" {
		t.Errorf("claim-stamped continuation must win over the registry fallback, got %+v", got)
	}
}

// TestResolveContinuationForScope_UnstampedClaimStillFallsBack — the subtle
// half of the ordering rule: a claim EXISTING is not the condition; a claim
// carrying a STAMP is. An unstamped claim (the ordinary case) must not block
// the lane-scope fallback.
func TestResolveContinuationForScope_UnstampedClaimStillFallsBack(t *testing.T) {
	root := t.TempDir()
	seedRegistry(t, root, "scope-a", "8888888888888888888888888888888888888888", 1078)
	seedClaim(t, root, 1102, "task-a", "")

	got := ResolveContinuationForScope(Options{ProjectRoot: root}, 1102, []string{"scope-a"})
	if got == nil || got.SnapshotSHA != "8888888888888888888888888888888888888888" {
		t.Errorf("an unstamped claim must not suppress the lane-scope fallback, got %+v", got)
	}
}

// TestResolveContinuationForScope_NoBindingIsNil — NEGATIVE. Unknown scope
// ids, an empty scope list, nil scopes, and blank ids all resolve to nil. A
// resolver that returned "some" binding for an unrelated scope would adopt
// another lane's work into this one.
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

// TestResolveContinuationForScope_ScopeOrderIsDeterministic — a multi-id lane
// resolves the FIRST id that carries a binding, in the order the lane scope
// declares. Deterministic resolution is what makes a re-run reproducible.
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

// TestResolveContinuationForScope_EmptySnapshotIsNotABinding — NEGATIVE: a
// registry entry with no snapshot ref is not resumable work; returning it
// would send the orchestrator into validateContinuation with an empty binding
// every cycle.
func TestResolveContinuationForScope_EmptySnapshotIsNotABinding(t *testing.T) {
	root := t.TempDir()
	if err := continuation.WriteRegistryEntry(root, "scope-a", continuation.Continuation{Cycle: 1078}); err != nil {
		t.Fatal(err)
	}
	if got := ResolveContinuationForScope(Options{ProjectRoot: root}, 1102, []string{"scope-a"}); got != nil {
		t.Errorf("an entry with no snapshot_sha is not a binding, got %+v", got)
	}
}

// TestResolveContinuation_ClaimOnlyPathUnchanged — REGRESSION on G1. The
// original claim-only entry point must keep ignoring the registry entirely, so
// callers that deliberately want claim semantics (and PR #363's behaviour) are
// byte-identical after this extension.
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

// TestResolveContinuationForScope_CorruptRegistryIsNilNotPanic — a corrupt
// registry must degrade to "no continuation" (fresh start) rather than crash
// the orchestrator mid-cycle; the loudness lives in the registry reader's
// error, which this resolver surfaces to the log, not to a panic.
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
