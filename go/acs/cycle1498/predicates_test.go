//go:build acs

// Package cycle1498 materialises the cycle-1498 acceptance criteria for the one
// fleet-scoped task pinned to this lane, `retire-consumed-fleet-alias`: retire
// the consumed fleet alias `pipeline-defect-pipeline-blocker` through the
// EXISTING reviewed/locked `evolve carryover apply-decisions` path rather than a
// prompt-side suppression rule or a hand edit of state.json.
//
// Predicate strategy (the cycle-85 degenerate-predicate ban): predicates 001-003
// drive the REAL production caller — the `evolve` CLI entry point
// (`cmd/evolve` dispatch table → runCarryover → runCarryoverApplyDecisions →
// applyCarryoverDecisions) — against an on-disk state.json fixture, and assert on
// exit code plus the resulting on-disk bytes. No source grep carries any
// assertion. The binary under test is BUILT FROM THIS WORKTREE'S SOURCE in
// TestMain, never the pre-existing `go/evolve` artifact, so a regression the
// builder introduces in cmd_carryover.go turns these predicates RED instead of
// passing against a stale binary.
//
// Predicate 004 is the task's declared verifiableBy: the named regression test
// must exist in `go/cmd/evolve/cmd_carryover_test.go` and PASS. It asserts on the
// `--- PASS: <exact name>` line rather than the exit code, because `go test -run`
// with a pattern that matches nothing exits 0 — the vacuous-green trap.
//
// Reliability (flaky-predicate-shape rules): no `/...` sweep, no whole-package
// `go test` (004 is narrowed by an anchored `-run`), no wall-clock deadline, no
// literal PID; every subprocess is given absolute paths or an explicit cmd.Dir,
// never process cwd.
package cycle1498

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// consumedFleetAlias is the exact id the lane is authorized to retire.
const consumedFleetAlias = "pipeline-defect-pipeline-blocker"

// liveSibling is an unrelated, still-live carryover item that MUST survive every
// retirement apply — the anti-overreach control that fails a blanket prune or a
// substring filter.
const liveSibling = "todo-live-unrelated-sibling"

// namedRegressionTest is the task's declared verifiableBy target.
const namedRegressionTest = "TestCarryoverApplyDecisions_DropsConsumedFleetAlias"

// evolveBin is the CLI built from THIS worktree's source in TestMain.
var evolveBin string

// buildErr is non-empty when the worktree build failed; every predicate fails
// loudly with it rather than silently skipping (a predicate that cannot run is
// never a PASS).
var buildErr string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "acs-cycle1498-bin-")
	if err != nil {
		buildErr = fmt.Sprintf("mktemp for evolve build: %v", err)
		os.Exit(m.Run())
	}
	defer os.RemoveAll(dir)

	// Resolve the worktree's go/ module root without depending on process cwd.
	goMod, err := moduleRoot()
	if err != nil {
		buildErr = err.Error()
		os.Exit(m.Run())
	}
	bin := filepath.Join(dir, "evolve-under-test")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/evolve")
	cmd.Dir = goMod
	if out, err := cmd.CombinedOutput(); err != nil {
		buildErr = fmt.Sprintf("go build ./cmd/evolve (dir=%s): %v\n%s", goMod, err, out)
	} else {
		evolveBin = bin
	}
	os.Exit(m.Run())
}

// moduleRoot returns <worktree>/go using the same repo-root resolution the rest
// of the ACS suite uses. RepoRoot needs a *testing.T, so TestMain walks up from
// the predicate file's own directory instead.
func moduleRoot() (string, error) {
	wd, err := os.Getwd() // go test runs the binary in the package dir: <root>/go/acs/cycle1498
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
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
		t.Fatalf("cannot exercise the CLI under test: %s", buildErr)
	}
	if evolveBin == "" {
		t.Fatalf("evolve binary was not built (empty path, no recorded error)")
	}
	return evolveBin
}

// writeAliasFixture writes a state.json holding the consumed alias plus an
// unrelated live sibling, and returns its path. The sibling's action text
// MENTIONS the alias on purpose: a retirement that matches on text instead of id
// would delete it and fail the anti-overreach assertion.
func writeAliasFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	state := map[string]any{
		"stateRevision": float64(1),
		"carryoverTodos": []any{
			map[string]any{"id": consumedFleetAlias, "action": "consumed fleet alias; refuted premise", "priority": "low"},
			map[string]any{
				"id":       liveSibling,
				"action":   "still live: reconcile the " + consumedFleetAlias + " premise against the inbox",
				"priority": "high",
			},
		},
		"someOtherKey": "preserved",
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("marshal fixture state: %v", err)
	}
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write fixture state: %v", err)
	}
	return path
}

// writeDecisions writes a reviewed decisions doc and returns its path.
// aliasReason is placed verbatim so a caller can exercise the empty-reason guard.
func writeDecisions(t *testing.T, aliasReason string) string {
	t.Helper()
	doc := map[string]any{
		"source_count": 2,
		"decisions": []any{
			map[string]any{"id": consumedFleetAlias, "decision": "drop", "reason": aliasReason},
			map[string]any{"id": liveSibling, "decision": "keep", "reason": "still live operator work"},
		},
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal decisions: %v", err)
	}
	path := filepath.Join(t.TempDir(), "decisions.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write decisions: %v", err)
	}
	return path
}

// carryoverIDs decodes state.json and returns the resident carryover ids. It
// fatals when the file is not valid JSON, which is itself an assertion (the
// idempotency criterion requires the file stay parseable).
func carryoverIDs(t *testing.T, statePath string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state %s: %v", statePath, err)
	}
	var doc struct {
		CarryoverTodos []struct {
			ID string `json:"id"`
		} `json:"carryoverTodos"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("state.json is not valid JSON after the apply: %v\n%s", err, raw)
	}
	ids := make(map[string]bool, len(doc.CarryoverTodos))
	for _, e := range doc.CarryoverTodos {
		ids[e.ID] = true
	}
	return ids
}

// applyRetirement runs the real CLI entry point against the fixture.
func applyRetirement(t *testing.T, statePath, decisionsPath string) (stdout, stderr string, code int) {
	t.Helper()
	bin := requireBinary(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(bin,
		"carryover", "apply-decisions", "--apply", "--state", statePath, "--decisions", decisionsPath)
	if err != nil && code == 0 {
		t.Fatalf("running %s carryover apply-decisions: %v (stderr=%s)", bin, err, stderr)
	}
	return stdout, stderr, code
}

// TestC1498_001_AliasDropRemovesOnlyTheNamedAlias — AC1. A reviewed `drop`
// decision applied through the real CLI removes the consumed fleet alias and
// leaves the unrelated live sibling (and unmodelled state keys) intact.
func TestC1498_001_AliasDropRemovesOnlyTheNamedAlias(t *testing.T) {
	statePath := writeAliasFixture(t)
	decisionsPath := writeDecisions(t, "consumed fleet alias; residual shipped")

	stdout, stderr, code := applyRetirement(t, statePath, decisionsPath)
	if code != 0 {
		t.Fatalf("apply exited %d, want 0 (stdout=%s stderr=%s)", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "2→1") && !strings.Contains(stdout, "2->1") {
		t.Errorf("apply did not report a 2→1 convergence; stdout=%q", stdout)
	}

	ids := carryoverIDs(t, statePath)
	if ids[consumedFleetAlias] {
		t.Errorf("consumed alias %q survived a reviewed drop decision", consumedFleetAlias)
	}
	if !ids[liveSibling] {
		t.Errorf("unrelated live item %q was removed — retirement over-reached beyond the named id", liveSibling)
	}
	if len(ids) != 1 {
		t.Errorf("resident carryover ids = %d (%v), want exactly 1 (the live sibling)", len(ids), ids)
	}

	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if !strings.Contains(string(raw), `"someOtherKey"`) {
		t.Errorf("unrelated state key someOtherKey was dropped by the apply")
	}
}

// TestC1498_002_EmptyReasonDropIsRejectedBeforeAnyWrite — AC2, NEGATIVE. An
// unjustified (whitespace-only reason) drop of the alias must be refused with a
// non-zero exit AND leave state.json byte-identical: validation runs before the
// lock is taken. This is the anti-hand-edit guard; without it an unreviewed
// retirement could silently prune live carryover context.
func TestC1498_002_EmptyReasonDropIsRejectedBeforeAnyWrite(t *testing.T) {
	statePath := writeAliasFixture(t)
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	decisionsPath := writeDecisions(t, "   ") // whitespace-only == unjustified

	stdout, stderr, code := applyRetirement(t, statePath, decisionsPath)
	if code == 0 {
		t.Fatalf("apply exited 0 for an empty-reason drop; want non-zero (stdout=%s)", stdout)
	}
	if !strings.Contains(stderr, "empty reason") {
		t.Errorf("rejection did not name the empty-reason cause; stderr=%q", stderr)
	}

	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state after rejection: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("state.json was mutated by a REJECTED decisions file (want byte-identical)")
	}
	ids := carryoverIDs(t, statePath)
	if !ids[consumedFleetAlias] || !ids[liveSibling] {
		t.Errorf("carryover entries changed on a rejected apply: %v", ids)
	}
}

// TestC1498_003_RepeatedAliasDropIsIdempotent — AC3. Re-applying the same
// reviewed decision is harmless: exit 0, the file stays valid JSON, the alias
// stays gone, and the live sibling is never collaterally removed by the repeat.
func TestC1498_003_RepeatedAliasDropIsIdempotent(t *testing.T) {
	statePath := writeAliasFixture(t)
	decisionsPath := writeDecisions(t, "consumed fleet alias; residual shipped")

	if _, stderr, code := applyRetirement(t, statePath, decisionsPath); code != 0 {
		t.Fatalf("first apply exited %d, want 0 (stderr=%s)", code, stderr)
	}
	first := carryoverIDs(t, statePath)

	stdout, stderr, code := applyRetirement(t, statePath, decisionsPath)
	if code != 0 {
		t.Fatalf("second (idempotent) apply exited %d, want 0 (stderr=%s)", code, stderr)
	}
	second := carryoverIDs(t, statePath) // fatals if the repeat left torn/invalid JSON

	if len(first) != len(second) {
		t.Errorf("repeat apply changed the carryover count %d→%d (not idempotent)", len(first), len(second))
	}
	if second[consumedFleetAlias] {
		t.Errorf("consumed alias reappeared after the repeat apply")
	}
	if !second[liveSibling] {
		t.Errorf("live sibling %q was removed by the REPEAT apply", liveSibling)
	}
	if !strings.Contains(stdout, "dropped 0") {
		t.Errorf("repeat apply reported a non-zero drop count; stdout=%q", stdout)
	}
}

// TestC1498_004_NamedRegressionTestExistsAndPasses — AC4, the task's declared
// verifiableBy. The regression pin must live with the command it protects
// (go/cmd/evolve/cmd_carryover_test.go) so it runs in normal CI, not only in this
// cycle's ACS lane.
//
// The assertion is on the `--- PASS: <name>` line, NOT the exit code: `go test
// -run` with a pattern matching nothing exits 0, so an exit-code check would go
// vacuously green while the test is still absent.
func TestC1498_004_NamedRegressionTestExistsAndPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	testFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_carryover_test.go")
	if !acsassert.FileExists(t, testFile) {
		t.Fatalf("RED: %s missing", testFile)
	}
	// Tracking check (cycle-93): an untracked test file is dropped at ship.
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch",
		"go/cmd/evolve/cmd_carryover_test.go"); code != 0 {
		t.Errorf("RED: go/cmd/evolve/cmd_carryover_test.go is untracked — it would be dropped at ship")
	}

	cmd := exec.Command("go", "test", "./cmd/evolve", "-run", "^"+namedRegressionTest+"$", "-v", "-count=1")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	text := string(out)

	if !strings.Contains(text, "--- PASS: "+namedRegressionTest) {
		t.Fatalf("RED: %s did not run and PASS in ./cmd/evolve (go test err=%v)\n%s", namedRegressionTest, err, text)
	}
	if strings.Contains(text, "no tests to run") {
		t.Errorf("RED: `go test -run ^%s$` matched no test (vacuous green)", namedRegressionTest)
	}
}
