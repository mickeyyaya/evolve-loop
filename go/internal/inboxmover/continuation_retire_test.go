package inboxmover

// continuation_retire_test.go — durable regression coverage for the two halves
// of park-consume-releases-continuation-binding. The cycle-1507 ACS predicates
// pin the same contract, but they are cycle-scoped and get archived; this is
// the coverage that travels with the package.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

// retireFixture seeds root with a live item + binding for each id.
func retireFixture(t *testing.T, ids ...string) string {
	t.Helper()
	root := t.TempDir()
	for i, id := range ids {
		seedRootItem(t, root, id)
		seedRegistryOnly(t, root, id, 1484+i)
	}
	return root
}

// seedRootItem drops a pending item carrying id at the inbox ROOT — the batch
// loader's own reach, and therefore live.
func seedRootItem(t *testing.T, root, id string) string {
	t.Helper()
	return seedItemIn(t, filepath.Join(root, ".evolve", "inbox"), id)
}

// seedItemIn writes an item carrying id into an arbitrary inbox subtree.
func seedItemIn(t *testing.T, dir, id string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "2026-08-16T00-00-00Z-"+id+".json")
	body, err := json.Marshal(map[string]any{"id": id, "title": "fixture " + id})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// seedRegistryOnly binds id WITHOUT creating an item.
func seedRegistryOnly(t *testing.T, root, id string, cycle int) continuation.Continuation {
	t.Helper()
	c := continuation.Continuation{
		Worktree:    filepath.Join(root, "wt"),
		Branch:      "evolve/cycle-" + id,
		SnapshotSHA: "9813bc621fe4aa0d55e1c0d3f0e1a2b3c4d5e6f7",
		BaseSHA:     "d3c69cd2aa11bb22cc33dd44ee55ff6600112233",
		Cycle:       cycle,
	}
	if err := continuation.WriteRegistryEntry(root, id, c); err != nil {
		t.Fatalf("seed binding %s: %v", id, err)
	}
	return c
}

func stillBound(t *testing.T, root, id string) bool {
	t.Helper()
	_, ok, err := continuation.ReadRegistryEntry(root, id)
	if err != nil {
		t.Fatalf("read registry %s: %v", id, err)
	}
	return ok
}

// TestPromote_ReleasesBindingAndPreservesPointer covers the write half: every
// Promote destination is out of the batch loader's reach, so the binding must
// be released in the same operation — with its VALUE preserved onto the retired
// item, and an unrelated live lane's binding untouched.
func TestPromote_ReleasesBindingAndPreservesPointer(t *testing.T) {
	const parked, sibling = "context-fill-telemetry-and-cap", "some-other-live-todo"
	for _, state := range []string{"quarantine", "processed", "rejected", "retry"} {
		t.Run(state, func(t *testing.T) {
			root := retireFixture(t, parked, sibling)
			want, ok, err := continuation.ReadRegistryEntry(root, parked)
			if err != nil || !ok {
				t.Fatalf("fixture binding missing: ok=%v err=%v", ok, err)
			}

			res, err := Promote(Options{ProjectRoot: root, Stderr: &bytes.Buffer{}}, parked, state, PromoteOpts{Cycle: "1507"})
			if err != nil || res.NoOp {
				t.Fatalf("Promote(%s) failed: res=%+v err=%v", state, res, err)
			}
			if stillBound(t, root, parked) {
				t.Errorf("promote → %s/ left %q bound — the retired scope re-arms on the next wave", state, parked)
			}
			if !stillBound(t, root, sibling) {
				t.Errorf("promote of %q released the unrelated live scope %q", parked, sibling)
			}

			raw, rerr := os.ReadFile(res.DestPath)
			if rerr != nil {
				t.Fatalf("read retired item: %v", rerr)
			}
			var doc struct {
				Released []continuation.Continuation `json:"released_continuations"`
			}
			if uerr := json.Unmarshal(raw, &doc); uerr != nil {
				t.Fatalf("retired item is not JSON: %v", uerr)
			}
			if len(doc.Released) != 1 || doc.Released[0].SnapshotSHA != want.SnapshotSHA ||
				doc.Released[0].BaseSHA != want.BaseSHA || doc.Released[0].Branch != want.Branch {
				t.Errorf("released_continuations[] did not preserve the binding %+v: %s", want, string(raw))
			}
		})
	}
}

// TestPromote_UnboundItemGainsNoReleasedKey — the ordinary case stays byte-clean.
func TestPromote_UnboundItemGainsNoReleasedKey(t *testing.T) {
	root := t.TempDir()
	seedRootItem(t, root, "plain-todo")

	res, err := Promote(Options{ProjectRoot: root}, "plain-todo", "processed", PromoteOpts{Cycle: "1507"})
	if err != nil || res.NoOp {
		t.Fatalf("Promote failed: res=%+v err=%v", res, err)
	}
	raw, err := os.ReadFile(res.DestPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if _, present := doc["released_continuations"]; present {
		t.Errorf("an unbound item gained released_continuations[]: %s", string(raw))
	}
}

// TestResolveContinuationForScope_GhostScopeRefusedAndReleased covers the read
// half: a binding whose scope id has no live pending item is refused, logged
// and released — the cycle-1487/1497 re-dispatch-forever shape.
func TestResolveContinuationForScope_GhostScopeRefusedAndReleased(t *testing.T) {
	root := t.TempDir()
	const ghost = "context-fill-telemetry-and-cap"
	seedItemIn(t, filepath.Join(root, ".evolve", "inbox", "quarantine"), ghost) // parked
	seedRegistryOnly(t, root, ghost, 1484)

	var errBuf bytes.Buffer
	if got := ResolveContinuationForScope(Options{ProjectRoot: root, Stderr: &errBuf}, 1507, []string{ghost}); got != nil {
		t.Errorf("a ghost scope resolved for dispatch: %+v", got)
	}
	if stillBound(t, root, ghost) {
		t.Errorf("the ghost binding for %q was not released — it re-arms every wave", ghost)
	}
	if !strings.Contains(errBuf.String(), ghost) {
		t.Errorf("the refusal was not logged: %q", errBuf.String())
	}
}

// TestResolveContinuationForScope_RetiredDirsAreNotLive pins the liveness
// definition to the batch loader's reach: an item sitting in any retirement dir
// is NOT live, so its binding is a ghost.
func TestResolveContinuationForScope_RetiredDirsAreNotLive(t *testing.T) {
	const id = "parked-scope"
	for _, dir := range []string{"quarantine", "consumed", "processed", "rejected", "retry"} {
		t.Run(dir, func(t *testing.T) {
			root := t.TempDir()
			// processed/ and rejected/ nest a cycle-N level (promoteDestPath) —
			// the evidence scan must reach it, not just the flat dirs.
			seedItemIn(t, filepath.Join(root, ".evolve", "inbox", dir, "cycle-1506"), id)
			seedRegistryOnly(t, root, id, 1484)

			if got := ResolveContinuationForScope(Options{ProjectRoot: root, Stderr: &bytes.Buffer{}}, 1507, []string{id}); got != nil {
				t.Errorf("an item in %s/ is retired, not live — its binding must not dispatch: %+v", dir, got)
			}
		})
	}
}

// TestResolveContinuationForScope_NoItemAnywhereStillAdopts is THE
// anti-overreach control that shaped the guard: the wave planner also mints
// lane scopes from carryoverTodos, which never have an inbox file at all
// (cycle-1078's orphan class — the reason the scope-keyed registry exists).
// Absence of an item is therefore NOT evidence of retirement; only a copy
// sitting in a pool-exit dir is. A guard that released here would trade the
// re-dispatch defect for a salvage-loss defect.
func TestResolveContinuationForScope_NoItemAnywhereStillAdopts(t *testing.T) {
	root := t.TempDir()
	const laneOnly = "carryover-only-scope"
	want := seedRegistryOnly(t, root, laneOnly, 1484)

	got := ResolveContinuationForScope(Options{ProjectRoot: root, Stderr: &bytes.Buffer{}}, 1507, []string{laneOnly})
	if got == nil || got.SnapshotSHA != want.SnapshotSHA {
		t.Fatalf("a lane-scope-only binding with no inbox item must still resolve (cycle-1078 orphan class): got %+v", got)
	}
	if !stillBound(t, root, laneOnly) {
		t.Errorf("the guard released a lane-scope-only binding — carryover lanes would lose their preserved work every wave")
	}
}

// TestResolveContinuationForScope_ClaimedItemIsLive is the anti-overreach
// control for the in-flight edge: a lane holding the item in processing/cycle-N
// must keep its binding.
func TestResolveContinuationForScope_ClaimedItemIsLive(t *testing.T) {
	root := t.TempDir()
	const id = "in-flight-scope"
	seedClaim(t, root, 1506, id, "")
	want := seedRegistryOnly(t, root, id, 1484)

	got := ResolveContinuationForScope(Options{ProjectRoot: root, Stderr: &bytes.Buffer{}}, 1507, []string{id})
	if got == nil || got.SnapshotSHA != want.SnapshotSHA {
		t.Fatalf("a claimed (in-flight) scope lost its binding: got %+v", got)
	}
	if !stillBound(t, root, id) {
		t.Errorf("the guard released an in-flight scope's binding — salvage pointer destroyed")
	}
}
