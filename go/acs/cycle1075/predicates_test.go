//go:build acs

// Package cycle1075 materialises the cycle-1075 acceptance criteria for the two
// fleet-scoped `loop-batch-chaining` tasks pinned to this lane:
//
//   - chain-boundary-loop  → the outer batch-chaining loop + its stop conditions
//   - chain-policy-flag    → `--until-inbox-empty` CLI opt-in + policy `chain` block
//
// Predicate strategy — every predicate EXERCISES the system under test (the
// cycle-85 degenerate-predicate ban forbids source-greps as the load-bearing
// assertion):
//
//   - 001 runs the REAL `evolve loop` binary via `go run` in a throwaway project
//     and reads the emitted --dry-run config JSON: with `--until-inbox-empty` the
//     resolved config must report chain mode ON; WITHOUT it (the negative axis)
//     the same invocation must report it OFF. A no-op that ignores the flag fails.
//   - 002 CALLS the policy loader (`policy.Load` → `Policy.ChainConfig()`) against
//     temp policy.json fixtures: an explicit `chain` block must be honoured, an
//     absent block must fall back to a positive compiled default cap, and a
//     zero/negative `max_batches` (the edge axis) must never yield a 0 cap that
//     would disable chaining outright.
//   - 003 runs the fake-runner chain tests in ./cmd/evolve as a subprocess and
//     requires all four boundary stop conditions (inbox-empty clean exit, quota
//     wall → checkpoint+defer instead of relaunch, max_batches cap, `.evolve/
//     loop-stop` operator brake) to be covered by PASSING named tests.
//   - 004 runs the fleet-width-preserved-across-batches regression test — a
//     naive per-batch re-init silently regresses lane width, so it gets its own
//     predicate per the standing width commitment.
//
// 003/004 shell out to `go test` rather than importing the code because
// go/cmd/evolve is `package main` and cannot be imported. The predicates assert
// on the runner's PASS lines for specifically-named behaviours, so an empty or
// filtered-out run ("no tests to run") is a failure, not a silent pass.
package cycle1075

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goDir is the module dir every subprocess command runs from.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// runInGoDir executes a command with go/ as the working directory. acsassert's
// SubprocessOutput has no cwd knob, so the dir switch is expressed as `go -C`.
func runGo(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	full := append([]string{"-C", goDir(t)}, args...)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", full...)
	if err != nil && stdout == "" && stderr == "" {
		t.Fatalf("could not execute `go %s`: %v", strings.Join(full, " "), err)
	}
	return stdout, stderr, code
}

// dryRunChainMode invokes the real `evolve loop --dry-run` through `go run` in a
// throwaway project root and reports the resolved config's chain-mode flag.
// Returns (chainMode, rawConfigJSON).
func dryRunChainMode(t *testing.T, extraArgs ...string) (bool, string) {
	t.Helper()
	proj := t.TempDir()
	if err := os.MkdirAll(filepath.Join(proj, ".evolve"), 0o755); err != nil {
		t.Fatalf("seed temp project: %v", err)
	}
	args := []string{"run", "./cmd/evolve", "loop",
		"--dry-run",
		"--project-root", proj,
		"--goal-text", "acs-1075-chain-probe",
	}
	args = append(args, extraArgs...)
	stdout, stderr, code := runGo(t, args...)
	if code != 0 {
		t.Fatalf("`evolve loop --dry-run %s` exited %d (must be accepted and exit 0)\nstdout:\n%s\nstderr:\n%s",
			strings.Join(extraArgs, " "), code, stdout, stderr)
	}
	var doc struct {
		DryRun bool            `json:"dry_run"`
		Config json.RawMessage `json:"config"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("dry-run stdout is not the expected config JSON: %v\nstdout:\n%s", err, stdout)
	}
	var cfg map[string]any
	if err := json.Unmarshal(doc.Config, &cfg); err != nil {
		t.Fatalf("dry-run `config` is not a JSON object: %v", err)
	}
	// chain_mode is the loopConfig field the flag must set. Absent ⇒ off (which
	// is correct for the negative case and a failure for the positive case).
	v, ok := cfg["chain_mode"]
	if !ok {
		return false, string(doc.Config)
	}
	b, isBool := v.(bool)
	if !isBool {
		t.Fatalf("dry-run config field chain_mode is %T, want bool\nconfig:\n%s", v, string(doc.Config))
	}
	return b, string(doc.Config)
}

// TestC1075_001_UntilInboxEmptyFlagDrivesChainMode — AC3/AC1 (chain-policy-flag).
// The `--until-inbox-empty` CLI opt-in (the explicit param mandated over an env
// flag) must be ACCEPTED by `evolve loop` and must turn chain mode on in the
// resolved config. Negative axis: the identical invocation WITHOUT the flag must
// leave chain mode off — chaining is opt-in, never the silent default.
func TestC1075_001_UntilInboxEmptyFlagDrivesChainMode(t *testing.T) {
	on, cfgOn := dryRunChainMode(t, "--until-inbox-empty")
	if !on {
		t.Errorf("`evolve loop --until-inbox-empty --dry-run` resolved chain mode OFF; the flag must set loopConfig chain_mode=true\nconfig:\n%s", cfgOn)
	}

	off, cfgOff := dryRunChainMode(t)
	if off {
		t.Errorf("`evolve loop --dry-run` (no --until-inbox-empty) resolved chain mode ON; chaining must stay opt-in\nconfig:\n%s", cfgOff)
	}
}

// writePolicy writes a policy.json fixture into a temp dir and returns its path.
func writePolicy(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write policy fixture: %v", err)
	}
	return path
}

// TestC1075_002_ChainPolicyBlockOverrideAndDefault — AC3 (chain-policy-flag),
// policy half. Calls the real loader and resolver: an explicit `chain` block must
// be honoured verbatim; an ABSENT block must fall back to a positive compiled
// default cap (mirroring WorkflowConfig's defaults precedent, policy.go:802+);
// and a zero / negative `max_batches` (edge axis) must resolve to that positive
// default rather than a 0 cap that would silently disable chaining.
func TestC1075_002_ChainPolicyBlockOverrideAndDefault(t *testing.T) {
	// Absent block → compiled defaults.
	base, err := policy.Load(writePolicy(t, `{"floor":[]}`))
	if err != nil {
		t.Fatalf("policy.Load on a chain-less policy failed: %v", err)
	}
	def := base.ChainConfig()
	if def.Enabled {
		t.Errorf("absent chain block resolved Enabled=true; chaining must default OFF (CLI opt-in)")
	}
	if def.MaxBatches <= 0 {
		t.Errorf("absent chain block resolved MaxBatches=%d; the compiled default cap must be positive", def.MaxBatches)
	}

	// Explicit block → honoured verbatim.
	p, err := policy.Load(writePolicy(t, `{"chain":{"enabled":true,"max_batches":7}}`))
	if err != nil {
		t.Fatalf("policy.Load on a chain policy failed: %v", err)
	}
	got := p.ChainConfig()
	if !got.Enabled {
		t.Errorf("chain.enabled=true in policy.json resolved Enabled=false")
	}
	if got.MaxBatches != 7 {
		t.Errorf("chain.max_batches=7 resolved MaxBatches=%d, want 7", got.MaxBatches)
	}

	// Edge axis: a bogus non-positive cap must not disable chaining.
	for _, body := range []string{
		`{"chain":{"enabled":true,"max_batches":0}}`,
		`{"chain":{"enabled":true,"max_batches":-3}}`,
	} {
		bp, err := policy.Load(writePolicy(t, body))
		if err != nil {
			t.Fatalf("policy.Load on %s failed: %v", body, err)
		}
		if c := bp.ChainConfig(); c.MaxBatches <= 0 {
			t.Errorf("policy %s resolved MaxBatches=%d; a non-positive cap must fall back to the positive compiled default", body, c.MaxBatches)
		}
	}
}

// passedTests returns the set of test names that reported `--- PASS:` in a
// verbose `go test` run (top-level names only; subtest paths are kept whole).
func passedTests(out string) map[string]bool {
	names := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "--- PASS:") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, "--- PASS:"))
		if i := strings.Index(rest, " "); i > 0 {
			rest = rest[:i]
		}
		if rest != "" {
			names[rest] = true
		}
	}
	return names
}

// anyNameContains reports whether some passing test name contains every one of
// the given lowercase fragments (an OR over the alternative fragment sets).
func anyNameMatches(names map[string]bool, alternatives [][]string) bool {
	for n := range names {
		low := strings.ToLower(n)
		for _, frags := range alternatives {
			all := true
			for _, f := range frags {
				if !strings.Contains(low, f) {
					all = false
					break
				}
			}
			if all {
				return true
			}
		}
	}
	return false
}

// runChainTests runs the ./cmd/evolve tests matching pattern verbosely and
// returns the set of passing test names. A non-zero exit, or a run that matched
// nothing, is a hard failure — an empty run must never read as satisfied.
func runChainTests(t *testing.T, pattern string) map[string]bool {
	t.Helper()
	stdout, stderr, code := runGo(t, "test", "-count=1", "-v", "-run", pattern, "./cmd/evolve")
	combined := stdout + "\n" + stderr
	if code != 0 {
		t.Fatalf("`go test -run %s ./cmd/evolve` exited %d (chain behaviour tests must PASS)\n%s", pattern, code, tail(combined))
	}
	names := passedTests(combined)
	if len(names) == 0 {
		t.Fatalf("`go test -run %s ./cmd/evolve` matched no passing tests — the chain behaviour tests are missing\n%s", pattern, tail(combined))
	}
	return names
}

// tail trims long runner output to the last 4000 bytes for readable failures.
func tail(s string) string {
	if len(s) <= 4000 {
		return s
	}
	return "…\n" + s[len(s)-4000:]
}

// TestC1075_003_ChainBoundaryStopConditionsCovered — AC1 + AC2 + AC3
// (chain-boundary-loop). The outer chaining loop must be exercised by PASSING
// fake-runner tests covering all four boundary decisions:
//
//	inbox drained      → next batch starts with no external invocation, then clean exit
//	quota wall         → checkpoint + defer (NOT an immediate relaunch into an 85)
//	max_batches cap    → the chain stops at the configured cap
//	.evolve/loop-stop  → the operator brake halts the chain at the boundary
//
// Each condition is required by name, so implementing one and skipping the rest
// leaves this predicate RED.
func TestC1075_003_ChainBoundaryStopConditionsCovered(t *testing.T) {
	names := runChainTests(t, "TestRunLoopChain")

	conditions := []struct {
		label string
		alts  [][]string
	}{
		{"inbox drained → chained batch + clean exit", [][]string{{"inbox"}, {"drain"}}},
		{"quota wall → checkpoint/defer, no relaunch", [][]string{{"quota"}, {"exhaust"}}},
		{"max_batches cap", [][]string{{"maxbatch"}, {"batchcap"}, {"cap"}}},
		{"`.evolve/loop-stop` operator brake", [][]string{{"stopfile"}, {"loopstop"}, {"brake"}}},
	}
	for _, c := range conditions {
		if !anyNameMatches(names, c.alts) {
			t.Errorf("no PASSING TestRunLoopChain* test covers the %s stop condition (saw: %s)", c.label, sortedKeys(names))
		}
	}
	if len(names) < 4 {
		t.Errorf("only %d TestRunLoopChain* tests passed; all four boundary stop conditions need distinct coverage (saw: %s)", len(names), sortedKeys(names))
	}
}

// TestC1075_004_FleetWidthPreservedAcrossBatches — AC4 (chain-boundary-loop).
// Fleet width is a hard operator commitment; a chain loop that re-initialises
// per batch can silently drop lanes. A dedicated regression test asserting lane
// count is stable from batch N to batch N+1 must exist and PASS.
func TestC1075_004_FleetWidthPreservedAcrossBatches(t *testing.T) {
	names := runChainTests(t, "TestRunLoopChain.*(Fleet|Width)|Fleet.*Chain")
	if !anyNameMatches(names, [][]string{{"width"}, {"lane"}}) {
		t.Errorf("no PASSING test asserts fleet width/lane count is preserved across chained batches (saw: %s)", sortedKeys(names))
	}
}

// sortedKeys renders a name set deterministically for failure messages.
func sortedKeys(m map[string]bool) string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	if len(out) == 0 {
		return "<none>"
	}
	// insertion sort — tiny sets, no import needed beyond strings
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return strings.Join(out, ", ")
}
