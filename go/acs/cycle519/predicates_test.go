//go:build acs

// Package cycle519 materialises the cycle-519 acceptance criteria.
//
// TRIAGE COMMITTED ONE ## top_n TASK this cycle (triage-decision.json):
//
//	loop-cannot-selfheal-dirty-main-tree — implement ONLY the boot pre-flight
//	slice: detect uncommitted tracked-source changes in the main tree at loop
//	boot (git status --porcelain, EXCLUDING .evolve/ and knowledge-base/) and
//	auto-quarantine via a TIMESTAMPED `git stash` (or fail-fast with a precise
//	remediation message if quarantine is unsafe).
//
// (The three deferred sub-scopes of this L-sized item — tree-diff-guard
// attribution, stale-marker self-heal, pre-wave worktree isolation — are in
// triage ## deferred, NOT top_n, so NO predicates are authored for them, per
// R9.3: predicates bind ONLY to triage-committed work. The scout-report's
// wave-seed / disjoint-topn fleet tasks are handled by a sibling lane and are
// likewise out of scope here.)
//
// ── DELTA vs. PRE-EXISTING GREEN (transparently reported) ──────────────────
// Detection, the .evolve/ + knowledge-base/ exclusion, and the non-destructive
// stash all shipped in cycles 507/514 (boot_preflight.go / cmd_loop_boot_recovery.go).
// The ONE behaviour the committed slice ADDS is the TIMESTAMP on the quarantine
// stash label: cmd_loop_boot_recovery.go currently stashes under the FIXED
// constant "boot-quarantine", collapsing every boot quarantine across every batch
// under one ambiguous, unrecoverable name. TestC519_001 is therefore RED at TDD
// time; the three regression predicates (002-004) PIN the already-shipped
// detect/exclude/non-destructive contract so the timestamp change cannot silently
// regress them (they are GREEN now — non-degenerate: each drives a real in-package
// test that CALLS the system and asserts on a stash/side-effect, never a grep).
//
// Predicate strategy (mirrors cycle507/514/518): BEHAVIOURAL predicates drive the
// system under test through its in-package tests via subprocess `go test`,
// asserting a non-degenerate pass (requireTestsRan closes the cycle-85 "no tests
// to run" trap) — never a source grep. Driven tests:
//
//	cmd/evolve/cmd_loop_boot_recovery_timestamp_test.go  (RED: timestamped label)
//	cmd/evolve/cmd_loop_boot_recovery_test.go            (GREEN: detect+quarantine wiring)
//	internal/core/boot_preflight_test.go                 (GREEN: exclusion + non-destructive)
package cycle519

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cmdEvolvePkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	corePkg      = "github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// runGoTest runs `go test` on pkg filtered by runFilter, returning combined
// output + exit code. Behavioural predicates invoke the system under test through
// its own in-package tests — no source-grep gaming.
func runGoTest(t *testing.T, runFilter, pkg string) (out string, code int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", runFilter, pkg)
	return stdout + "\n" + stderr, code
}

// requireTestsRan closes the degenerate-predicate trap: `go test -run X` with no
// matching test (renamed/unwritten) — or a package that fails to build — exits
// without running the required tests, which must NOT green the predicate.
func requireTestsRan(t *testing.T, out string, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — required tests are unwritten or renamed")
		return
	}
	if got := strings.Count(out, "=== RUN"); got < min {
		t.Errorf("only %d test(s) ran, need >= %d (package build failure or renamed tests)", got, min)
	}
}

// TestC519_001_QuarantineStashLabelTimestamped (AC-CC, RED headline): the boot
// pre-flight must auto-quarantine leaked tracked-source dirt via a TIMESTAMPED
// git stash, not the fixed constant "boot-quarantine". Drives the behavioural
// cmd/evolve test that runs bootRecoverFn on a dirty repo and inspects the ACTUAL
// stash git created. RED until the loop threads a timestamped label through the
// quarantine call.
func TestC519_001_QuarantineStashLabelTimestamped(t *testing.T) {
	out, code := runGoTest(t,
		"TestDefaultBootRecovery_QuarantineStashLabelIsTimestamped", cmdEvolvePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("boot quarantine stash label is NOT timestamped (still the fixed 'boot-quarantine' constant) — successive boot quarantines are indistinguishable/unrecoverable.\n%s", out)
	}
}

// TestC519_002_DetectsAndQuarantinesLeakedSource (AC-CA, regression GREEN): the
// boot pre-flight actually detects a leaked tracked-source file and quarantines
// it so the tree is clean for the first cycle's tree-diff guard — proving the
// detect+stash path is invoked from the orchestrator (not merely defined). Drives
// the shipped wiring test; must stay GREEN through the timestamp change.
func TestC519_002_DetectsAndQuarantinesLeakedSource(t *testing.T) {
	out, code := runGoTest(t,
		"TestDefaultBootRecovery_QuarantinesDirtyTree", cmdEvolvePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("boot pre-flight no longer detects+quarantines leaked tracked source (detection/wiring regressed).\n%s", out)
	}
}

// TestC519_003_ExcludesLoopManagedDirs (AC-CB, regression GREEN): detection must
// EXCLUDE the loop's own managed dirs (.evolve/, knowledge-base/) so normal
// in-flight cycle writes never trigger a false-positive quarantine of loop state.
// Drives the shipped classifyDirtyPaths exclusion test.
func TestC519_003_ExcludesLoopManagedDirs(t *testing.T) {
	out, code := runGoTest(t,
		"TestClassifyDirtyPaths_IgnoresLoopManaged", corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf(".evolve/ and knowledge-base/ must be EXCLUDED from the boot dirty-scan; exclusion regressed.\n%s", out)
	}
}

// TestC519_004_QuarantineNonDestructivePreservesContent (AC-CD, regression GREEN):
// the quarantine must be NON-DESTRUCTIVE (stash, not checkout) — popping restores
// the leaked content, and the tree is left clean afterward. Drives the shipped
// core stash test; guards that the timestamped-label change keeps quarantine
// recoverable.
func TestC519_004_QuarantineNonDestructivePreservesContent(t *testing.T) {
	out, code := runGoTest(t,
		"TestQuarantineDirtyTree_LeavesStatusCleanAndPreservesContent", corePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("boot quarantine must stay NON-DESTRUCTIVE (stash-preserves-content) and leave the tree clean; regressed.\n%s", out)
	}
}
