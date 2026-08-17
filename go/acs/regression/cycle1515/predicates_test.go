//go:build acs

// Package cycle1515 materialises the cycle-1515 acceptance criteria for the
// fleet-scoped todo `park-consume-releases-continuation-binding`, whose triage
// split it into three top_n tasks:
//
//   - registry-release-on-park-consume — an item leaving the pending pool
//     (park/quarantine, ship-time consume) MUST release its scope-keyed
//     continuation-registry binding in the SAME operation, with the binding
//     VALUE preserved into the item file (`released_continuations[]`).
//   - planner-adoption-live-item-guard — the scope-keyed registry read
//     (`inboxmover.ResolveContinuationForScope`) MUST refuse a binding whose
//     scope id names no live pending item, and release the ghost.
//   - continuation-operator-cli — `evolve continuation list` /
//     `evolve continuation release <scope-id>` must exist so console never
//     hand-edits continuation-registry.json under its flock sidecar again.
//
// Standing state at RED time (verified live, not assumed from the filed item):
// the first two tasks ALREADY landed on this branch as cycle-1507's
// continuation work — `internal/inboxmover/continuation_retire.go`,
// `internal/phases/ship/consume.go:96-105`, and the guard at
// `internal/inboxmover/continuation_resolve.go:86`. Predicates 001-003 are
// therefore PRE-EXISTING GREEN and stand as regression pins: this cycle must
// not regress the wired lifecycle while adding the operator surface. Predicates
// 004-008 are the genuine RED — no `evolve continuation` subcommand exists
// (`ls go/cmd/evolve | grep -i continu` → no match).
//
// Predicate strategy (the cycle-85 degenerate-predicate ban): every predicate
// here drives a REAL production entry point — the exported inboxmover /
// continuation functions, or the `evolve` binary built from THIS worktree in
// TestMain — and asserts on its return value, exit code, stderr, or the
// resulting on-disk bytes. There is no load-bearing source-text assertion in
// this file.
//
// The CLI predicates run the binary with an explicit cmd.Dir set to the fixture
// project root and assert through the process boundary, so they are a
// REACHABILITY proof of the dispatcher wiring (registry.go's commands table),
// not a direct call into an unreachable handler.
//
// Reliability (flaky-predicate-shape rules): no `/...` sweep, no multi-package
// `go test`, no known 40s+ suite, no wall-clock deadline, no literal PID; every
// subprocess carries an explicit cmd.Dir and never inherits process cwd.
package cycle1515

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
)

// scopeID is the parked/consumed scope under test — the shape of the live burn
// (cycle-1487 re-dispatched this exact id from an immortal binding).
const scopeID = "context-fill-telemetry-and-cap"

// liveSibling is an UNRELATED scope whose item is still pending. Its binding
// must survive every release, every guard pass and every CLI release — the
// anti-overreach control that fails a blanket registry prune.
const liveSibling = "some-other-live-todo"

// evolveBin is the CLI built from THIS worktree's source in TestMain.
var evolveBin string

// buildErr is non-empty when the worktree build failed; the CLI predicates fail
// loudly with it rather than skipping (a predicate that cannot run is never a
// PASS).
var buildErr string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "acs-cycle1515-bin-")
	if err != nil {
		buildErr = fmt.Sprintf("mktemp for evolve build: %v", err)
		os.Exit(m.Run())
	}
	defer os.RemoveAll(dir)

	goMod, err := moduleRoot()
	if err != nil {
		buildErr = err.Error()
		os.Exit(m.Run())
	}
	bin := filepath.Join(dir, "evolve-under-test")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/evolve")
	cmd.Dir = goMod
	if out, berr := cmd.CombinedOutput(); berr != nil {
		buildErr = fmt.Sprintf("go build ./cmd/evolve (dir=%s): %v\n%s", goMod, berr, out)
	} else {
		evolveBin = bin
	}
	os.Exit(m.Run())
}

// moduleRoot returns <worktree>/go by walking up from the predicate package
// dir, so no predicate depends on process cwd.
func moduleRoot() (string, error) {
	wd, err := os.Getwd() // go test runs in the package dir: <root>/go/acs/regression/cycle1515
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, serr := os.Stat(filepath.Join(dir, "go.mod")); serr == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("no go.mod found walking up from %s", wd)
}

// requireBinary returns the built CLI or fails the predicate with the build
// error — never a silent skip.
func requireBinary(t *testing.T) string {
	t.Helper()
	if buildErr != "" {
		t.Fatalf("evolve CLI did not build from this worktree: %s", buildErr)
	}
	if evolveBin == "" {
		t.Fatalf("evolve CLI path is empty and no build error was recorded")
	}
	return evolveBin
}

// runCLI executes the built binary rooted AT the fixture project (explicit
// cmd.Dir — never process cwd) and returns stdout, stderr and the exit code.
func runCLI(t *testing.T, projectRoot string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(requireBinary(t), args...)
	cmd.Dir = projectRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running %v in %s: %v", args, projectRoot, err)
	}
	return stdout.String(), stderr.String(), code
}

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

// preservedPointers returns the concatenated released_continuations[] entries
// of the item file at path, failing when the key is absent or malformed.
func preservedPointers(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read item %s: %v", path, err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("item %s is not JSON: %v", path, err)
	}
	entry, ok := doc["released_continuations"]
	if !ok {
		t.Fatalf("RED: item %s has no released_continuations[] — the binding pointer is lost, not preserved\n%s", path, string(raw))
	}
	var released []json.RawMessage
	if err := json.Unmarshal(entry, &released); err != nil {
		t.Fatalf("RED: released_continuations is not a JSON array in %s: %v", path, err)
	}
	if len(released) == 0 {
		t.Fatalf("RED: released_continuations[] is empty in %s — nothing preserved", path)
	}
	joined := ""
	for _, r := range released {
		joined += string(r)
	}
	return joined
}

// ---------------------------------------------------------------------------
// Task: registry-release-on-park-consume  (regression pins — see package doc)
// ---------------------------------------------------------------------------

// TestC1515_001_ParkReleasesBindingAndPreservesPointer drives the real park
// path (inboxmover.Promote → quarantine) for an item holding a registry
// binding: the binding must be gone, the unrelated live sibling's binding must
// survive, and the released VALUE must land in the parked item file. A release
// that drops the snapshot SHA orphans the preserved work it pointed at.
func TestC1515_001_ParkReleasesBindingAndPreservesPointer(t *testing.T) {
	root := newProject(t)
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeItem(t, inbox, scopeID)
	writeItem(t, inbox, liveSibling)
	c := binding(1484)
	bind(t, root, scopeID, c)
	bind(t, root, liveSibling, binding(1490))

	var errBuf bytes.Buffer
	res, err := inboxmover.Promote(
		inboxmover.Options{ProjectRoot: root, Stderr: &errBuf},
		scopeID, "quarantine", inboxmover.PromoteOpts{Cycle: "1515"})
	if err != nil {
		t.Fatalf("Promote(quarantine) errored: %v (stderr: %s)", err, errBuf.String())
	}
	if res.NoOp || res.DestPath == "" {
		t.Fatalf("Promote(quarantine) was a no-op: %+v (stderr: %s)", res, errBuf.String())
	}

	if bound(t, root, scopeID) {
		t.Errorf("RED: parking %q left its continuation-registry binding intact — the parked scope is re-dispatched as an adopted continuation forever (cycle-1487 burn)", scopeID)
	}
	if !bound(t, root, liveSibling) {
		t.Errorf("RED: parking %q also released the UNRELATED live scope %q — release must be scoped to the retired item", scopeID, liveSibling)
	}
	joined := preservedPointers(t, res.DestPath)
	for _, want := range []string{c.SnapshotSHA, c.Branch, c.BaseSHA} {
		if !strings.Contains(joined, want) {
			t.Errorf("RED: released_continuations[] in %s does not preserve %q — pointer loss on release", res.DestPath, want)
		}
	}
}

// TestC1515_002_ParkOfUnboundItemInventsNoAnnotation is the anti-overreach edge
// case: an item that never held a binding must be parked with its JSON
// untouched. A retire path that unconditionally writes released_continuations[]
// would pass predicate 001 while corrupting every ordinary parked item.
func TestC1515_002_ParkOfUnboundItemInventsNoAnnotation(t *testing.T) {
	root := newProject(t)
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeItem(t, inbox, scopeID)

	var errBuf bytes.Buffer
	res, err := inboxmover.Promote(
		inboxmover.Options{ProjectRoot: root, Stderr: &errBuf},
		scopeID, "quarantine", inboxmover.PromoteOpts{Cycle: "1515"})
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
	if _, present := doc["released_continuations"]; present {
		t.Errorf("RED: an item with no registry binding gained released_continuations[] on park: %s", string(raw))
	}
}

// ---------------------------------------------------------------------------
// Task: planner-adoption-live-item-guard  (regression pin — see package doc)
// ---------------------------------------------------------------------------

// TestC1515_003_ScopeResolveRefusesRetiredBinding drives the ONE seam the wave
// planner's lane minting and the post-triage adoption path both go through and
// asserts the cycle-1487 shape is refused: item parked in quarantine/, binding
// still live ⇒ nil return, WARN on stderr, ghost binding released. The live
// sibling in the same call must still resolve — a guard that refuses
// everything would trade re-dispatch for salvage loss.
func TestC1515_003_ScopeResolveRefusesRetiredBinding(t *testing.T) {
	root := newProject(t)
	inbox := filepath.Join(root, ".evolve", "inbox")
	// The parked shape: the item lives in quarantine/, out of the batch
	// loader's reach, while the registry still binds its scope.
	writeItem(t, filepath.Join(inbox, "quarantine"), scopeID)
	bind(t, root, scopeID, binding(1484))

	var errBuf bytes.Buffer
	opts := inboxmover.Options{ProjectRoot: root, Stderr: &errBuf}
	got := inboxmover.ResolveContinuationForScope(opts, 1515, []string{scopeID})
	if got != nil {
		t.Errorf("RED: ResolveContinuationForScope adopted a binding for parked scope %q (snapshot %s) — the parked scope is re-dispatched with no live item behind it", scopeID, got.SnapshotSHA)
	}
	if !strings.Contains(errBuf.String(), scopeID) {
		t.Errorf("RED: refusing the dead binding for %q was SILENT — the operator gets a vanished lane with no explanation. stderr: %q", scopeID, errBuf.String())
	}
	if bound(t, root, scopeID) {
		t.Errorf("RED: the ghost binding for %q survived the refusal — it re-arms on the very next wave", scopeID)
	}

	// Anti-overreach control: a scope whose item IS live must still resolve.
	writeItem(t, inbox, liveSibling)
	live := binding(1490)
	bind(t, root, liveSibling, live)
	var errBuf2 bytes.Buffer
	optsLive := inboxmover.Options{ProjectRoot: root, Stderr: &errBuf2}
	resolved := inboxmover.ResolveContinuationForScope(optsLive, 1515, []string{liveSibling})
	if resolved == nil || resolved.SnapshotSHA != live.SnapshotSHA {
		t.Errorf("RED: the guard also refused LIVE scope %q (got %+v) — preserved work for a pending item must still resume", liveSibling, resolved)
	}
}

// ---------------------------------------------------------------------------
// Task: continuation-operator-cli  (the genuine RED for cycle 1515)
// ---------------------------------------------------------------------------

// TestC1515_004_ContinuationListShowsBindings drives `evolve continuation list`
// through the process boundary — which is simultaneously the reachability proof
// that the subcommand is registered in the dispatcher table, since an
// unregistered handler cannot be routed to. Every bound scope must be reported
// with the pointer an operator needs (scope id, snapshot SHA, ancestor cycle).
func TestC1515_004_ContinuationListShowsBindings(t *testing.T) {
	root := newProject(t)
	c := binding(1484)
	sib := binding(1490)
	bind(t, root, scopeID, c)
	bind(t, root, liveSibling, sib)

	stdout, stderr, code := runCLI(t, root, "continuation", "list")
	if code != 0 {
		t.Fatalf("RED: `evolve continuation list` exited %d — the operator surface does not exist, so console keeps hand-editing continuation-registry.json under its flock sidecar.\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	for _, want := range []string{scopeID, c.SnapshotSHA, fmt.Sprint(c.Cycle), liveSibling, sib.SnapshotSHA} {
		if !strings.Contains(stdout, want) {
			t.Errorf("RED: `evolve continuation list` output omits %q — an operator cannot see which binding to release.\nstdout: %s", want, stdout)
		}
	}
}

// TestC1515_005_ContinuationListOnEmptyRegistryIsCleanExit is the edge case: a
// project with no registry at all (the ordinary case for every healthy cycle)
// must be a clean exit-0 report, not an error and not a phantom scope.
func TestC1515_005_ContinuationListOnEmptyRegistryIsCleanExit(t *testing.T) {
	root := newProject(t)

	stdout, stderr, code := runCLI(t, root, "continuation", "list")
	if code != 0 {
		t.Fatalf("RED: `evolve continuation list` on an unbound project exited %d — an absent registry is the normal state, not a failure.\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if strings.Contains(stdout, scopeID) || strings.Contains(stdout, liveSibling) {
		t.Errorf("RED: `evolve continuation list` reported a scope on an EMPTY registry.\nstdout: %s", stdout)
	}
}

// TestC1515_006_ContinuationReleaseReleasesAndAnnotates drives `evolve
// continuation release <scope-id>`: the binding must go, the unrelated live
// sibling's binding must survive, and the released VALUE must be preserved into
// the scope's item file — the same preserve-then-release contract predicate 001
// pins for the park path, reached from the operator surface rather than
// duplicated inside it.
func TestC1515_006_ContinuationReleaseReleasesAndAnnotates(t *testing.T) {
	root := newProject(t)
	inbox := filepath.Join(root, ".evolve", "inbox")
	itemPath := writeItem(t, inbox, scopeID)
	writeItem(t, inbox, liveSibling)
	c := binding(1484)
	bind(t, root, scopeID, c)
	bind(t, root, liveSibling, binding(1490))

	stdout, stderr, code := runCLI(t, root, "continuation", "release", scopeID)
	if code != 0 {
		t.Fatalf("RED: `evolve continuation release %s` exited %d — no operator release path exists.\nstdout: %s\nstderr: %s", scopeID, code, stdout, stderr)
	}
	if bound(t, root, scopeID) {
		t.Errorf("RED: `evolve continuation release %s` reported success but the binding is still in the registry", scopeID)
	}
	if !bound(t, root, liveSibling) {
		t.Errorf("RED: releasing %q also dropped the UNRELATED binding for %q — release must name exactly one scope", scopeID, liveSibling)
	}
	joined := preservedPointers(t, itemPath)
	for _, want := range []string{c.SnapshotSHA, c.Branch, c.BaseSHA} {
		if !strings.Contains(joined, want) {
			t.Errorf("RED: released_continuations[] in %s does not preserve %q — the operator release loses the salvage pointer the manual flock edit preserved by hand", itemPath, want)
		}
	}
}

// TestC1515_007_ContinuationReleaseRejectsUnknownScope is the negative test —
// the strongest anti-no-op signal here. A command that exits 0 on every input
// would satisfy predicates 004 and 006 while telling an operator that a typo'd
// scope id was successfully released.
func TestC1515_007_ContinuationReleaseRejectsUnknownScope(t *testing.T) {
	root := newProject(t)
	bind(t, root, liveSibling, binding(1490))

	stdout, stderr, code := runCLI(t, root, "continuation", "release", "no-such-scope-id")
	if code == 0 {
		t.Errorf("RED: `evolve continuation release no-such-scope-id` exited 0 on a scope that holds NO binding — a typo reads as a successful release.\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	if !strings.Contains(stdout+stderr, "no-such-scope-id") {
		t.Errorf("RED: the unknown-scope failure never named the scope, so the operator cannot see what was rejected.\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	if !bound(t, root, liveSibling) {
		t.Errorf("RED: a failed release of an unknown scope collaterally dropped %q's binding", liveSibling)
	}
}

// TestC1515_008_ContinuationRejectsMalformedInvocations covers the remaining
// malformed-input edges through the real dispatcher: a bare `continuation`, an
// unknown subcommand, and `release` with no scope argument must each fail
// loudly rather than defaulting to some destructive interpretation.
//
// The registration precondition is load-bearing: without it this predicate is
// vacuously GREEN on a repo that has no `continuation` command at all (every
// invocation fails as an unknown command), which is exactly the no-op-passable
// shape the cycle-85 ban exists to catch.
func TestC1515_008_ContinuationRejectsMalformedInvocations(t *testing.T) {
	root := newProject(t)
	bind(t, root, scopeID, binding(1484))

	if stdout, stderr, code := runCLI(t, root, "continuation", "list"); code != 0 {
		t.Fatalf("RED: precondition — `evolve continuation list` exited %d, so the subcommand is not registered and the malformed-input edges below would pass vacuously.\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	cases := [][]string{
		{"continuation"},
		{"continuation", "frobnicate"},
		{"continuation", "release"},
	}
	for _, args := range cases {
		stdout, stderr, code := runCLI(t, root, args...)
		if code == 0 {
			t.Errorf("RED: `evolve %s` exited 0 — a malformed invocation must fail loudly.\nstdout: %s\nstderr: %s", strings.Join(args, " "), stdout, stderr)
		}
	}
	if !bound(t, root, scopeID) {
		t.Errorf("RED: a malformed invocation mutated the registry — %q's binding was dropped without an explicit scope argument", scopeID)
	}
}
