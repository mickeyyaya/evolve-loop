//go:build acs

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

const scopeID = "context-fill-telemetry-and-cap"

const liveSibling = "some-other-live-todo"

var evolveBin string

var buildErr string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "acs-cycle1515-bin-")
	if err != nil {
		buildErr = fmt.Sprintf("mktemp for evolve build: %v", err)
		m.Run()
		return
	}
	defer os.RemoveAll(dir)

	goMod, err := moduleRoot()
	if err != nil {
		buildErr = err.Error()
		m.Run()
		return
	}
	bin := filepath.Join(dir, "evolve-under-test")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/evolve")
	cmd.Dir = goMod
	if out, berr := cmd.CombinedOutput(); berr != nil {
		buildErr = fmt.Sprintf("go build ./cmd/evolve (dir=%s): %v\n%s", goMod, berr, out)
	} else {
		evolveBin = bin
	}
	m.Run()
}

func moduleRoot() (string, error) {
	wd, err := os.Getwd()
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

func TestC1515_003_ScopeResolveRefusesRetiredBinding(t *testing.T) {
	root := newProject(t)
	inbox := filepath.Join(root, ".evolve", "inbox")
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

func TestC1515_006_ContinuationReleaseReleasesAndAnnotates(t *testing.T) {
	root := newProject(t)
	inbox := filepath.Join(root, ".evolve", "inbox")
	itemPath := writeItem(t, inbox, scopeID)
	writeItem(t, inbox, liveSibling)
	c := binding(1484)
	bind(t, root, scopeID, c)
	bind(t, root, liveSibling, binding(1490))

	stdout, stderr, code := runCLI(t, root, "continuation", "release", "-operator", scopeID)
	if code != 0 {
		t.Fatalf("RED: `evolve continuation release -operator %s` exited %d — authority was granted, so the release path itself is broken.\nstdout: %s\nstderr: %s", scopeID, code, stdout, stderr)
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
