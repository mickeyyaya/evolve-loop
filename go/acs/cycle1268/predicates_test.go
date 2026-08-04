//go:build acs

// Package cycle1268 materialises the cycle-1268 acceptance criteria for the two
// tasks triage committed to this lane:
//
//   - worktree-provisioning-retry-consolidate (inbox retro-fleet-worktree-dispatch, w=0.9)
//   - test-amplification-context-scope        (inbox test-amplification-context-scope, w=0.89)
//
// Scope note (read before judging these predicates). This lane is an ADR-0076
// continuation of cycle-1267 (snapshot 79d130d4), whose audit FAILed on an
// unrelated integration-tier bridge flake — so task 2 arrived here already
// substantially landed. The TDD phase verified the live tree rather than
// trusting the report, and pinned only what is genuinely open:
//
//	Task 1 — fully open. PR #401's bounded retry exists at ONE of four
//	         `git worktree add` call sites; CreateFrom, swarm's addWorktree and
//	         the operator CLI still issue the bare unretried add. 001-004.
//	Task 2 — the derivation half (CoveringTests, DirectImporters, the artifact,
//	         the fail-open guard, the truncation log) is PRE-EXISTING GREEN and
//	         is pinned as a regression guard, not as new work. The open half is
//	         the D3/F1 MEDIUM its own auditor raised: renderCoveringTests
//	         interpolates attacker-influenced filenames unescaped into a
//	         document agent.md declares authoritative. 005-006.
//
// Predicate strategy — behavioural-via-subprocess (the cycle-563/987/1255/1267
// precedent). Each predicate shells `go test -run '^(names)$' -v -count=1` over
// exactly ONE named package and requires a `--- PASS: <name>` line per test.
//
//   - Asserting on the PASS LINE, not the exit code, is essential: `go test -run`
//     with a pattern matching nothing exits 0 ("no tests to run"), so a still-
//     missing contract would false-GREEN.
//   - No source-grep predicate is used as a load-bearing assertion — it would
//     pass the moment the magic string appeared, fix or no fix (cycle-85 ban).
//   - Flaky-predicate-shape rules: every invocation names EXACTLY ONE package,
//     never ./..., and the two naming ./internal/core and ./cmd/evolve (known
//     40s+ suites) are narrowed with -run, which the rule explicitly permits.
//     No wall-clock bounds, no literal PIDs, no bare `git`, no load generators.
//
// Consolidation is pinned BEHAVIOURALLY rather than by grepping for a helper
// name: 002/003/004 each assert the site honours gitexec.DefaultWorktreeAddAttempts,
// so three private copies of the constant cannot satisfy the suite. A structural
// pin on a package-qualified call shape is also what burned cycle-644.
package cycle1268

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	gitexecPkg = "github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	corePkg    = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	swarmPkg   = "github.com/mickeyyaya/evolve-loop/go/internal/swarm"
	cmdPkg     = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
)

// assertDefaultSuiteTestsPass shells `go test -run '^(names)$' -v -count=1 pkg`
// in the DEFAULT build suite (no -tags) and requires EVERY name to have printed
// a `--- PASS: <name>` line. -count=1 defeats the test cache so the predicate
// always exercises current source.
func assertDefaultSuiteTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-v", "-count=1", pkg)
	if code == -1 {
		// -1 means the subprocess never launched (toolchain/module resolution
		// failure) — a harness error, not a test verdict.
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("test %s did NOT pass in %s "+
				"(missing, failing, or hidden behind a build tag the default suite skips). exit=%d\n"+
				"combined go-test output:\n%s", name, pkg, code, out)
		}
	}
}

// TestC1268_001_SharedWorktreeAddRetryHelperExists — AC1-AC4 of task 1.
//
// The extraction target itself: one bounded retry-with-backoff loop, in the
// only package core, swarm and cmd/evolve all already depend on. It must absorb
// a transient rc=255, stay bounded at DefaultWorktreeAddAttempts, surface git's
// own diagnosis on persistent failure (the CB.2 alarm chain stays armed — the
// refuted PR #400 is the record of what silencing it costs), and charge a clean
// provision exactly one attempt and zero backoff.
func TestC1268_001_SharedWorktreeAddRetryHelperExists(t *testing.T) {
	assertDefaultSuiteTestsPass(t, gitexecPkg,
		"TestAddWorktreeWithRetry_RetriesTransientFailure",
		"TestAddWorktreeWithRetry_BoundedThenSurfacesFinalFailure",
		"TestAddWorktreeWithRetry_CleanRunCostsOneAttemptAndNoSleep",
		"TestAddWorktreeWithRetry_ZeroValueConfigUsesDefaults",
		"TestAddWorktreeWithRetry_IssuesWorktreeAddArgv",
	)
}

// TestC1268_002_CreateFromRetriesTransientCollision — AC5-AC7 of task 1,
// adoption site #1 (ADR-0076 continuation seeding, worktree.go:208).
//
// The two PR #401 tests are re-run alongside the new ones: "existing tests
// staying green" is an explicit acceptance criterion, so a consolidation that
// regressed Create while fixing CreateFrom must not be able to green this.
func TestC1268_002_CreateFromRetriesTransientCollision(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestGitWorktreeCreateFrom_RetriesTransientAddFailure",
		"TestGitWorktreeCreateFrom_PersistentFailureStillFailsLoudly",
		"TestGitWorktreeCreateFrom_CleanRunCostsOneAttemptAndNoSleep",
		"TestGitWorktreeCreate_RetriesTransientAddFailure",
		"TestGitWorktreeCreate_PersistentFailureStillFailsLoudly",
	)
}

// TestC1268_003_SwarmWorkerProvisioningRetries — AC8-AC11 of task 1, adoption
// site #2: the highest-contention seam in the tree (N workers, one shared .git).
// BOTH production entry points are required — wiring one path only is the same
// defect, just narrower (#373).
func TestC1268_003_SwarmWorkerProvisioningRetries(t *testing.T) {
	assertDefaultSuiteTestsPass(t, swarmPkg,
		"TestSwarmCreateWorker_RetriesTransientAddFailure",
		"TestSwarmCreateIntegration_RetriesTransientAddFailure",
		"TestSwarmCreateWorker_PersistentFailureStillFailsLoudly",
		"TestSwarmCreateWorker_CleanRunCostsOneAttemptAndNoSleep",
	)
}

// TestC1268_004_OperatorCLIWorktreeCreateRetries — AC12-AC14 of task 1,
// adoption site #3. This site also bypasses gitexec entirely today (a raw
// exec.Command whose error path never sees git's exit code), so the predicate
// additionally requires rc/stderr parity with the other three sites.
func TestC1268_004_OperatorCLIWorktreeCreateRetries(t *testing.T) {
	assertDefaultSuiteTestsPass(t, cmdPkg,
		"TestRunWorktreeCreate_RetriesTransientAddFailure",
		"TestRunWorktreeCreate_PersistentFailureStillFailsLoudly",
		"TestRunWorktreeCreate_CleanRunCostsOneAttemptAndNoSleep",
	)
}

// TestC1268_005_CoveringTestsCorpusResistsPromptInjection — the open half of
// task 2 (audit finding D3/F1 MEDIUM against the continuation snapshot).
//
// covering-tests.md is declared authoritative to a code-writing agent, and its
// contents are filenames from the worktree diff — attacker-influenced by
// construction. Each path must render as exactly one list item whose code span
// the path itself cannot close, and no filename may introduce a top-level
// markdown directive. The companion benign-path test blocks the obvious
// over-correction (an escape that mangles ordinary Go test paths would break
// the corpus for every real cycle).
func TestC1268_005_CoveringTestsCorpusResistsPromptInjection(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestRenderCoveringTests_NeutralizesInjectedMarkdown",
		"TestRenderCoveringTests_LeavesBenignPathsVerbatim",
	)
}

// TestC1268_006_CoveringTestsDerivationStaysGreen — the PRE-EXISTING GREEN half
// of task 2, pinned as a regression guard rather than as new work.
//
// The derivation (changed packages → their _test.go files, widened by direct
// reverse-importers) and its fail-open guard landed in the continuation
// snapshot. The injection fix in 005 edits the same rendering path, so this
// predicate exists to make a hardening regression in the corpus derivation
// impossible to ship silently. Declaring it green-on-arrival is the honest
// disposition; omitting it would leave the diff's blast radius unpinned.
func TestC1268_006_CoveringTestsDerivationStaysGreen(t *testing.T) {
	assertDefaultSuiteTestsPass(t, "github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs",
		"TestCoveringTests_DerivesTestFilesForChangedPackagesOnly",
		"TestCoveringTests_FailsOpenOnUnusableInput",
		"TestCoveringTests_ReachableFromProduction",
		"TestDirectImporters_WidensToReverseImportersIncludingTestOnly",
		"TestDirectImporters_FailsOpenOnUnusableInput",
		"TestDirectImporters_ReachableFromProduction",
	)
}

// TestC1268_007_CoveringTestsCapStaysLoudAndCorrect — the "no silent caps" half
// of task 2, also PRE-EXISTING GREEN and pinned as a regression guard.
//
// This one is directly in the injection fix's blast radius: escaping changes the
// byte length of every rendered line, so a hardening that ignored the cap
// arithmetic would either drop paths without saying so or warn on clean cycles
// (which trains an operator to ignore the real warning). The omitted count must
// stay single-sourced in renderCoveringTests and still reach the operator's log.
func TestC1268_007_CoveringTestsCapStaysLoudAndCorrect(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestRenderCoveringTests_TruncatesWithVisibleNote",
		"TestRenderCoveringTests_EmitsEveryPathWhenUnderCap",
		"TestRenderCoveringTests_ReportsOmittedCount",
		"TestWriteCoveringTests_WarnsLoudlyOnTruncation",
		"TestWriteCoveringTests_SilentWhenNothingTruncated",
	)
}
