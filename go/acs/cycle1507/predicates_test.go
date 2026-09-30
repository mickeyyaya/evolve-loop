//go:build acs

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

const scopeID = "context-fill-telemetry-and-cap"

const liveSibling = "some-other-live-todo"

const consumeRegressionTest = "TestConsumeCommittedItems_ReleasesRegistryBinding"

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

func newProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatalf("fixture inbox dir: %v", err)
	}
	return root
}

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

func bind(t *testing.T, root, id string, c continuation.Continuation) {
	t.Helper()
	if err := continuation.WriteRegistryEntry(root, id, c); err != nil {
		t.Fatalf("fixture bind %s: %v", id, err)
	}
	if _, ok, err := continuation.ReadRegistryEntry(root, id); err != nil || !ok {
		t.Fatalf("fixture bind %s did not take (ok=%v err=%v)", id, ok, err)
	}
}

func bound(t *testing.T, root, id string) bool {
	t.Helper()
	_, ok, err := continuation.ReadRegistryEntry(root, id)
	if err != nil {
		t.Fatalf("read registry %s: %v", id, err)
	}
	return ok
}

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

	consumeSrc := filepath.Join(goDir, "internal", "phases", "ship", "consume.go")
	if !acsassert.FileContainsAny(consumeSrc, "DeleteRegistryEntry", "ReleaseContinuation", "continuation.") {
		t.Logf("AUX: %s references no continuation release symbol", consumeSrc)
	}
}

func TestC1507_004_AdoptionRefusesGhostScopeAndReleases(t *testing.T) {
	root := newProject(t)
	inbox := filepath.Join(root, ".evolve", "inbox")
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

	var edgeBuf bytes.Buffer
	if c := inboxmover.ResolveContinuationForScope(
		inboxmover.Options{ProjectRoot: root, Stderr: &edgeBuf}, 1507,
		[]string{"", "never-bound-scope"}); c != nil {
		t.Errorf("blank/unknown scope ids resolved a continuation: %+v", *c)
	}
}

func TestC1507_005_AdoptionAcceptsLivePendingItem(t *testing.T) {
	root := newProject(t)
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeItem(t, inbox, liveSibling)
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

func TestC1507_006_AdoptionAcceptsClaimedProcessingItem(t *testing.T) {
	root := newProject(t)
	claimDir := filepath.Join(root, ".evolve", "inbox", "processing", "cycle-1506")
	writeItem(t, claimDir, scopeID)
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
