//go:build acs

// Package cycle1507 materialises the cycle-1507 acceptance criteria for the two
// fleet-scoped top_n tasks pinned to this lane
// (`park-consume-releases-continuation-binding`):
//
//   - transactional-registry-retire-on-park-consume — an item leaving the
//     pending pool (park/quarantine, ship-consume) MUST release its
//     continuation-registry binding in the SAME operation, with the binding
//     VALUE preserved into the item file (`released_continuations[]`) so the
//     salvage pointer survives the release.
//   - planner-and-adoption-live-scope-guard — the scope-keyed registry read
//     (`inboxmover.ResolveContinuationForScope`, the ONE seam both the wave
//     planner's lane-scope minting and the post-triage adoption path go
//     through: injected at cmd/evolve/cmd_cycle.go:711 into
//     core.WithContinuationResolver) MUST refuse a binding whose scope id has
//     no live pending item — logged, released — instead of re-dispatching a
//     parked/consumed scope forever (live burns: cycles 1487, 1497).
//
// Predicate strategy (the cycle-85 degenerate-predicate ban): every predicate
// here drives a REAL production function against an on-disk fixture and asserts
// on its return value / stderr / the resulting on-disk bytes. `inboxmover` and
// `continuation` are imported and called directly — same module, so these are
// the production symbols, not a re-implementation. The only source-text
// assertion in this file is explicitly auxiliary (predicate 003) and carries no
// verdict on its own.
//
// "Live pending item" is defined by the batch loader's own reach, so the guard
// and the dispatcher can never disagree: an id is LIVE iff a `.json` in the
// inbox ROOT (inboxbatch.LoadDir's non-recursive scan) or in
// `inbox/processing/cycle-*/` (a lane currently holding it) carries that id.
// consumed/, quarantine/, processed/, rejected/ and retry/ are NOT live —
// LoadDir skips subdirs, which is exactly why a parked item stops being picked.
//
// Reliability (flaky-predicate-shape rules): no `/...` sweep, no multi-package
// `go test`, no wall-clock deadline, no literal PID; every subprocess gets an
// explicit cmd.Dir, never process cwd. The two `go test` invocations name ONE
// package each and neither is a known 40s+ suite.
package cycle1507

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// scopeID is the parked/consumed scope under test — the shape of the live burn.
const scopeID = "context-fill-telemetry-and-cap"

// liveSibling is an UNRELATED scope whose item is still pending. Its binding
// must survive every release and every guard pass: the anti-overreach control
// that fails a blanket registry prune.
const liveSibling = "some-other-live-todo"

// consumeRegressionTest is the named in-package regression test the ship-side
// half of task 1 must add — the wiring proof for `consumeCommittedItems`, which
// is unexported and therefore not directly callable from this package.
const consumeRegressionTest = "TestConsumeCommittedItems_ReleasesRegistryBinding"

// binding is the preserved-work value a release must not lose.
func binding(cycle int) continuation.Continuation {
	return continuation.Continuation{
		Worktree:     "/tmp/evolve/worktrees/cycle-" + fmt.Sprint(cycle),
		Branch:       "cycle-" + fmt.Sprint(cycle),
		SnapshotSHA:  "9813bc621fe4aa0d55e1c0d3f0e1a2b3c4d5e6f7",
		BaseSHA:      "d3c69cd2aa11bb22cc33dd44ee55ff6600112233",
		FindingsPath: ".evolve/runs/cycle-" + fmt.Sprint(cycle) + "/audit-fail-reason.json",
		Cycle:        cycle,
	}
}

// newProject builds an empty project root with an inbox tree.
func newProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatalf("fixture inbox dir: %v", err)
	}
	return root
}

// writeItem drops an inbox item carrying id into dir and returns its path.
func writeItem(t *testing.T, dir, id string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("fixture dir %s: %v", dir, err)
	}
	path := filepath.Join(dir, "2026-08-16T15-10-00Z-"+id+".json")
	doc := map[string]any{
		"id":       id,
		"kind":     "bug",
		"title":    "fixture item " + id,
		"priority": "high",
	}
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("fixture marshal: %v", err)
	}
	if err := os.WriteFile(path, append(body, '\n'), 0o644); err != nil {
		t.Fatalf("fixture write %s: %v", path, err)
	}
	return path
}

// bind writes a registry binding and fails loudly if the fixture itself broke.
func bind(t *testing.T, root, id string, c continuation.Continuation) {
	t.Helper()
	if err := continuation.WriteRegistryEntry(root, id, c); err != nil {
		t.Fatalf("fixture bind %s: %v", id, err)
	}
	if _, ok, err := continuation.ReadRegistryEntry(root, id); err != nil || !ok {
		t.Fatalf("fixture bind %s did not take (ok=%v err=%v)", id, ok, err)
	}
}

// bound reports whether scope id still holds a registry binding.
func bound(t *testing.T, root, id string) bool {
	t.Helper()
	_, ok, err := continuation.ReadRegistryEntry(root, id)
	if err != nil {
		t.Fatalf("read registry %s: %v", id, err)
	}
	return ok
}

// TestC1507_001_ParkReleasesRegistryBinding drives the real park path
// (inboxmover.Promote → quarantine) for an item that holds a registry binding
// and asserts the binding is gone afterwards — the transactional retire.
// The unrelated live sibling's binding must survive (anti-overreach).
func TestC1507_001_ParkReleasesRegistryBinding(t *testing.T) {
	root := newProject(t)
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeItem(t, inbox, scopeID)
	writeItem(t, inbox, liveSibling)
	bind(t, root, scopeID, binding(1484))
	bind(t, root, liveSibling, binding(1490))

	var errBuf bytes.Buffer
	res, err := inboxmover.Promote(
		inboxmover.Options{ProjectRoot: root, Stderr: &errBuf},
		scopeID, "quarantine", inboxmover.PromoteOpts{Cycle: "1507"})
	if err != nil {
		t.Fatalf("Promote(quarantine) errored: %v (stderr: %s)", err, errBuf.String())
	}
	if res.NoOp || res.DestPath == "" {
		t.Fatalf("Promote(quarantine) was a no-op: %+v (stderr: %s)", res, errBuf.String())
	}

	if bound(t, root, scopeID) {
		t.Errorf("RED: parking %q left its continuation-registry binding intact — the parked scope will be re-dispatched as an adopted continuation (cycle-1487 burn)", scopeID)
	}
	if !bound(t, root, liveSibling) {
		t.Errorf("RED: parking %q also released the UNRELATED live scope %q — release must be scoped to the retired item", scopeID, liveSibling)
	}
}

// TestC1507_002_ParkPreservesBindingPointerInItemFile asserts the released
// binding VALUE survives into the moved item file: a release that drops the
// snapshot SHA orphans the preserved work it was pointing at.
func TestC1507_002_ParkPreservesBindingPointerInItemFile(t *testing.T) {
	root := newProject(t)
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeItem(t, inbox, scopeID)
	c := binding(1484)
	bind(t, root, scopeID, c)

	var errBuf bytes.Buffer
	res, err := inboxmover.Promote(
		inboxmover.Options{ProjectRoot: root, Stderr: &errBuf},
		scopeID, "quarantine", inboxmover.PromoteOpts{Cycle: "1507"})
	if err != nil || res.DestPath == "" {
		t.Fatalf("Promote(quarantine) failed: res=%+v err=%v (stderr: %s)", res, err, errBuf.String())
	}

	raw, err := os.ReadFile(res.DestPath)
	if err != nil {
		t.Fatalf("read parked item %s: %v", res.DestPath, err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parked item %s is not JSON: %v", res.DestPath, err)
	}
	entry, ok := doc["released_continuations"]
	if !ok {
		t.Fatalf("RED: parked item %s has no released_continuations[] — the binding pointer (snapshot %s) is lost, not preserved", res.DestPath, c.SnapshotSHA)
	}
	var released []json.RawMessage
	if err := json.Unmarshal(entry, &released); err != nil {
		t.Fatalf("RED: released_continuations is not a JSON array in %s: %v", res.DestPath, err)
	}
	if len(released) == 0 {
		t.Fatalf("RED: released_continuations[] is empty in %s — nothing preserved", res.DestPath)
	}
	joined := ""
	for _, r := range released {
		joined += string(r)
	}
	for _, want := range []string{c.SnapshotSHA, c.Branch, c.BaseSHA} {
		if !strings.Contains(joined, want) {
			t.Errorf("RED: released_continuations[] in %s does not preserve %q — pointer loss on release", res.DestPath, want)
		}
	}
}

// TestC1507_003_ConsumeReleasesRegistryBinding is the ship-side half of the
// transactional retire. consumeCommittedItems is unexported, so the wiring
// proof is its NAMED in-package regression test: the predicate asserts the test
// exists and PASSES (asserting on the `--- PASS: <exact name>` line, not the
// exit code — `go test -run` with a pattern that matches nothing exits 0, the
// vacuous-green trap). The source check at the end is AUXILIARY only.
func TestC1507_003_ConsumeReleasesRegistryBinding(t *testing.T) {
	repo := acsassert.RepoRoot(t)
	goDir := filepath.Join(repo, "go")

	cmd := exec.Command("go", "test", "-count=1", "-v",
		"-run", "^"+consumeRegressionTest+"$", "./internal/phases/ship")
	cmd.Dir = goDir
	out, err := cmd.CombinedOutput()
	if !strings.Contains(string(out), "--- PASS: "+consumeRegressionTest) {
		t.Errorf("RED: %s did not PASS in ./internal/phases/ship (err=%v). The ship-time consumption path must release the consumed item's registry binding and preserve the pointer into the consumed item file; this named test is its wiring proof.\n%s",
			consumeRegressionTest, err, string(out))
	}

	// AUXILIARY (carries no verdict on its own): the release must be reachable
	// from consume.go itself, not only from the test.
	consumeSrc := filepath.Join(goDir, "internal", "phases", "ship", "consume.go")
	if !acsassert.FileContainsAny(consumeSrc, "DeleteRegistryEntry", "ReleaseContinuation", "continuation.") {
		t.Logf("AUX: %s references no continuation release symbol", consumeSrc)
	}
}

// TestC1507_004_AdoptionRefusesGhostScopeAndReleases is the negative test and
// the load-bearing anti-no-op signal for task 2: a registry binding whose scope
// id has NO live pending item anywhere must NOT be handed back for dispatch —
// it is logged and released. This is the exact cycle-1487/1497 shape (item
// parked/consumed out of the pool, binding immortal, lane minted anyway).
func TestC1507_004_AdoptionRefusesGhostScopeAndReleases(t *testing.T) {
	root := newProject(t)
	inbox := filepath.Join(root, ".evolve", "inbox")
	// The item is PARKED — present on disk, but in a dir LoadDir never walks.
	writeItem(t, filepath.Join(inbox, "quarantine"), scopeID)
	bind(t, root, scopeID, binding(1484))

	var errBuf bytes.Buffer
	got := inboxmover.ResolveContinuationForScope(
		inboxmover.Options{ProjectRoot: root, Stderr: &errBuf}, 1507, []string{scopeID})

	if got != nil {
		t.Errorf("RED: scope %q has no live pending item (parked into quarantine/) yet the registry binding was resolved for dispatch (snapshot %s) — this re-dispatches a parked scope forever", scopeID, got.SnapshotSHA)
	}
	if bound(t, root, scopeID) {
		t.Errorf("RED: the dead binding for %q was not released on the miss — it re-arms on every future wave", scopeID)
	}
	if !strings.Contains(errBuf.String(), scopeID) {
		t.Errorf("RED: the refusal was not logged (stderr does not name %q): %q", scopeID, errBuf.String())
	}

	// Edge/OOD, same seam: a blank scope id and an unknown scope are clean
	// misses that release nothing and must not panic.
	var edgeBuf bytes.Buffer
	if c := inboxmover.ResolveContinuationForScope(
		inboxmover.Options{ProjectRoot: root, Stderr: &edgeBuf}, 1507,
		[]string{"", "never-bound-scope"}); c != nil {
		t.Errorf("blank/unknown scope ids resolved a continuation: %+v", *c)
	}
}

// TestC1507_005_AdoptionAcceptsLivePendingItem is the anti-overreach control:
// a binding whose scope id IS a live pending inbox item must still be adopted
// and must NOT be released. A guard that fails this has traded the re-dispatch
// defect for a salvage-loss defect.
func TestC1507_005_AdoptionAcceptsLivePendingItem(t *testing.T) {
	root := newProject(t)
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeItem(t, inbox, liveSibling) // live: in the root LoadDir scans
	c := binding(1490)
	bind(t, root, liveSibling, c)

	var errBuf bytes.Buffer
	got := inboxmover.ResolveContinuationForScope(
		inboxmover.Options{ProjectRoot: root, Stderr: &errBuf}, 1507, []string{liveSibling})
	if got == nil {
		t.Fatalf("live pending scope %q lost its continuation — preserved work at %s orphaned (stderr: %s)", liveSibling, c.SnapshotSHA, errBuf.String())
	}
	if got.SnapshotSHA != c.SnapshotSHA {
		t.Errorf("adopted the wrong binding for %q: got %s want %s", liveSibling, got.SnapshotSHA, c.SnapshotSHA)
	}
	if !bound(t, root, liveSibling) {
		t.Errorf("the guard released a LIVE scope's binding (%q) — salvage pointer destroyed", liveSibling)
	}
}

// TestC1507_006_AdoptionAcceptsClaimedProcessingItem is the in-flight edge: an
// item claimed into processing/cycle-N/ has left the inbox ROOT but is still
// live (a lane holds it). Treating "not in the root" as dead would release the
// binding of a cycle that is mid-flight.
func TestC1507_006_AdoptionAcceptsClaimedProcessingItem(t *testing.T) {
	root := newProject(t)
	claimDir := filepath.Join(root, ".evolve", "inbox", "processing", "cycle-1506")
	writeItem(t, claimDir, scopeID) // claimed, NOT stamped with a continuation
	c := binding(1484)
	bind(t, root, scopeID, c)

	var errBuf bytes.Buffer
	got := inboxmover.ResolveContinuationForScope(
		inboxmover.Options{ProjectRoot: root, Stderr: &errBuf}, 1507, []string{scopeID})
	if got == nil {
		t.Fatalf("claimed (in-flight) scope %q lost its binding — preserved work at %s orphaned (stderr: %s)", scopeID, c.SnapshotSHA, errBuf.String())
	}
	if !bound(t, root, scopeID) {
		t.Errorf("the guard released an IN-FLIGHT scope's binding (%q) — a claimed item is live", scopeID)
	}
}

// TestC1507_007_ClaimStampedContinuationStillWins pins G1's untouched
// semantics: a continuation stamped on THIS cycle's processing claim resolves
// first, with no registry involvement. The regression control for the guard.
func TestC1507_007_ClaimStampedContinuationStillWins(t *testing.T) {
	root := newProject(t)
	claimDir := filepath.Join(root, ".evolve", "inbox", "processing", "cycle-1507")
	if err := os.MkdirAll(claimDir, 0o755); err != nil {
		t.Fatalf("fixture claim dir: %v", err)
	}
	c := binding(1500)
	doc := map[string]any{"id": scopeID, "title": "stamped claim", "continuation": c}
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("fixture marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claimDir, "claim.json"), append(body, '\n'), 0o644); err != nil {
		t.Fatalf("fixture write claim: %v", err)
	}

	var errBuf bytes.Buffer
	got := inboxmover.ResolveContinuationForScope(
		inboxmover.Options{ProjectRoot: root, Stderr: &errBuf}, 1507, nil)
	if got == nil || got.SnapshotSHA != c.SnapshotSHA {
		t.Fatalf("stamped claim continuation no longer resolves: got=%+v want snapshot %s (stderr: %s)", got, c.SnapshotSHA, errBuf.String())
	}
}

// TestC1507_008_TouchedSuitesRaceGreen is acceptance criterion 4's mechanical
// half: the two packages this cycle rewrites stay green under -race. One named
// package per invocation (no sweep, no multi-package call), explicit cmd.Dir.
func TestC1507_008_TouchedSuitesRaceGreen(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	for _, pkg := range []string{"./internal/inboxmover", "./internal/continuation"} {
		cmd := exec.Command("go", "test", "-race", "-count=1", pkg)
		cmd.Dir = goDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("RED: `go test -race %s` failed: %v\n%s", pkg, err, string(out))
		}
	}
}
