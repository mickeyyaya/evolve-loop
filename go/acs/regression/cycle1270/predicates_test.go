//go:build acs

// Package cycle1270 materialises the cycle-1270 acceptance criteria.
//
// Scope note (read this before judging the predicates below). This lane is an
// ADR-0076 continuation of the cycle-1268 salvage snapshot
// (continuation-manifest.json: base 85b3d368, snapshot 8e221b83), so the bulk of
// all three triage-committed items is ALREADY IN TREE (`git diff --stat
// main...HEAD`: 32 files, +2911) and the scout report's "unimplemented" claims
// are stale against this branch. The TDD phase verified the live tree instead of
// trusting the report and pinned only what is genuinely open — the residuals the
// fault-localization phase ranked as suspects 5-7 — plus the deterministic
// build-floor blocker that phase ranked #1-#3.
//
// The three triage-committed items (`triage-report.md` ## top_n):
//
//	infra-teardown-predicate-single-source  → 004, 005      (residual: scan-root hole)
//	retro-fleet-worktree-dispatch           → 006, 007      (residual: absent positive join)
//	test-amplification-context-scope        → 008, 009      (residual: silent degradation)
//
// The blocker (001-003) is NOT a triage item. It is the recorded cause of
// cycle-1268's death (`.evolve/runs/cycle-1268/audit-fail-reason.json`:
// `./cmd/evolve: unit tests FAIL`) and this cycle's fault-localization phase
// re-verified it will re-fire here, because this diff touches
// go/cmd/evolve/cmd_worktree.go and the floor therefore runs that package again:
//
//	gitexec.AddWorktreeWithRetry retries on ANY non-zero exit with a real
//	time.Sleep ladder (2s + 4s). 33 cmd/evolve tests transitively reach
//	core.gitWorktree.Create over a t.TempDir() that is not a git repository —
//	a PERMANENT rc=128 — so each pays the full 6s. Measured: 33 x 6s = 198s of
//	pure backoff in a package the floor runs with -timeout 120s. Deterministic,
//	not a flake. ADR-0082:83's claim that "test tiers install a no-op sleep …
//	so no suite pays the ladder" is false on the transitive-dispatch axis: the
//	seam (core/worktree.go:129 worktreeAddRetrySleep) is unexported and in a
//	different package than the 33 tests that reach it.
//
// Pinning the blocker here is deliberate. Triage cannot have committed it (it
// was diagnosed AFTER triage, by the phase whose job that is), no predicate can
// pass while it stands, and the anti-goals below are the ones that make a
// "green" cycle a lie. It binds no DEFERRED floor: 001-003 target
// internal/gitexec + internal/core, which are already Task 1's own package set.
// `tokenopt-session-resume-on-retry` is ## deferred and carries ZERO predicates
// (R9.3).
//
// Predicate strategy — behavioural-via-subprocess (the cycle-563/987/1255/1268
// precedent). Each predicate shells `go test -run '^(names)$' -v -count=1` over
// exactly ONE named package and requires a `--- PASS: <name>` line per test.
//
//   - Asserting on the PASS LINE, not the exit code, is essential: `go test -run`
//     with a pattern matching nothing exits 0 ("no tests to run"), so a still-
//     missing contract would false-GREEN. This is what makes these RED today.
//   - No source-grep is load-bearing anywhere in this file (the cycle-85 ban):
//     every assertion runs the system under test.
//   - Flaky-predicate-shape rules: every invocation names EXACTLY ONE package,
//     never ./..., and the ones naming ./internal/core and ./cmd/evolve (known
//     40s+ suites) are narrowed with -run, which the rule explicitly permits.
//     No wall-clock bounds, no literal PIDs, no bare `git`, no load generators.
//     003 in particular asserts on OUTPUT STATE (did the ladder announce?), not
//     on elapsed time — a duration bound would be exactly the banned shape, and
//     would also stretch arbitrarily under fleet contention.
package cycle1270

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	gitexecPkg = "github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	corePkg    = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	bridgePkg  = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	retroPkg   = "github.com/mickeyyaya/evolve-loop/go/internal/phases/retro"
	cmdPkg     = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
)

// runNamedTests shells `go test -run '^(names)$' -v -count=1 pkg` in the DEFAULT
// build suite (no -tags) and returns the combined output. -count=1 defeats the
// test cache so the predicate always exercises current source.
//
// The exec is spelled out HERE and again in assertNamedTestsPass rather than
// factored into a third helper both call. That is deliberate, not an oversight:
// the flaky-shape linter resolves a predicate's argv by following same-package
// helper calls exactly ONE level deep (flakylint_exec.go:88 —
// "Depth 1 exactly"), binding the call's arguments to the helper's parameters.
// A shared exec two hops down is unresolvable, so `-run` becomes invisible and
// every predicate naming ./internal/core draws a false "narrow the invocation
// with -run" finding at code that already narrows — the exact 48%-false-positive
// class the hop was added to kill. Six duplicated lines keep the lint TRUE.
func runNamedTests(t *testing.T, pkg string, names ...string) string {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-v", "-count=1", pkg)
	if code == -1 {
		// -1 means the subprocess never launched (toolchain/module resolution
		// failure) — a harness error, not a test verdict.
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	return stdout + stderr
}

// assertNamedTestsPass requires EVERY name to have printed a `--- PASS: <name>`
// line. A name that does not exist yet prints nothing, so the predicate is RED
// until the contract is actually authored — the property that makes this suite
// a RED contract rather than a rubber stamp.
//
// It repeats runNamedTests' exec instead of calling it — see that function's
// note on the linter's one-level helper hop.
func assertNamedTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-v", "-count=1", pkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("test %s did NOT pass in %s "+
				"(missing, failing, or hidden behind a build tag the default suite skips).\n"+
				"combined go-test output:\n%s", name, pkg, out)
		}
	}
}

// ---------------------------------------------------------------------------
// BLOCKER — the deterministic build-floor failure this cycle inherits.
// Fault-localization suspects #1 (0.95), #2 (0.85), #3 (0.70).
// ---------------------------------------------------------------------------

// TestC1270_001_PermanentWorktreeAddFailureCostsZeroBackoff — edit locations 1-2.
//
// The shared retry loop must consult a transience predicate BEFORE it sleeps, so
// a permanent condition (`fatal: not a git repository`, rc=128) returns after
// attempt 1 instead of paying the 2s+4s ladder. The three required tests are one
// per axis, and each one is load-bearing on its own:
//
//   - PermanentFailureSkipsBackoff — the fix. Zero sleeps, ONE invocation, and
//     the final rc + git's own stderr still returned intact. The loud-fail
//     contract (gitexec/worktree.go:46-50) is preserved: refuted PR #400 is the
//     record of what silencing the alarm costs, and this predicate must not be
//     satisfiable by suppressing the error.
//   - TransientStillRetriesToBound — the NEGATIVE guard. A "fix" that simply
//     stopped retrying would pass the first test and re-break PR #401's
//     collision absorber; rc=255 lock-shaped must still ride the full bound.
//   - NilRetryablePreservesRetryEverything — the zero value stays usable, so no
//     existing caller silently changes behaviour when the field is added.
func TestC1270_001_PermanentWorktreeAddFailureCostsZeroBackoff(t *testing.T) {
	assertNamedTestsPass(t, gitexecPkg,
		"TestAddWorktreeWithRetry_PermanentFailureSkipsBackoff",
		"TestAddWorktreeWithRetry_TransientStillRetriesToBound",
		"TestAddWorktreeWithRetry_NilRetryablePreservesRetryEverything",
		// PR #401's contract must stay green through the change — "existing
		// tests keep passing" is part of the criterion, not a courtesy.
		"TestAddWorktreeWithRetry_RetriesTransientFailure",
		"TestAddWorktreeWithRetry_BoundedThenSurfacesFinalFailure",
		"TestAddWorktreeWithRetry_CleanRunCostsOneAttemptAndNoSleep",
		"TestAddWorktreeWithRetry_ZeroValueConfigUsesDefaults",
		"TestAddWorktreeWithRetry_IssuesWorktreeAddArgv",
	)
}

// TestC1270_002_CoreSuppliesTransiencePredicateAndHonestAnnouncement — edit
// location 4, plus the diagnosability half of edit location 5.
//
// core is the caller that must actually SUPPLY the predicate — a classifier that
// exists in gitexec but is never passed leaves the 198s tax exactly where it is
// (the "seam whose only caller is a test" shape the house rules ban). And
// OnRetry currently prints `after transient rc=%d` for failures it has not
// established are transient, which is how a permanent rc=128 came to be logged
// as contention 33 times; the announcement must not claim transience it has not
// classified.
//
// BuildFloorSelfCheckFailures_KeepsTailDiagnostic is the reason cycle-1268 was
// undiagnosable rather than merely broken: the floor truncates with head[:400],
// but Go writes the panic/--- FAIL lines at the TAIL, so the recorded reason was
// 400 bytes of `[engine] WARN: Deps.TokenResolver is nil` and nothing else. The
// operator was handed noise where the stack trace was.
func TestC1270_002_CoreSuppliesTransiencePredicateAndHonestAnnouncement(t *testing.T) {
	assertNamedTestsPass(t, corePkg,
		"TestWorktreeAddRetry_NotAGitRepositoryCostsZeroBackoff",
		"TestWorktreeAddRetry_LockCollisionStillRetriesToBound",
		"TestWorktreeAddRetry_AnnouncementDoesNotClaimUnclassifiedTransience",
		"TestBuildFloorSelfCheckFailures_KeepsTailDiagnostic",
		// The provisioning fail-fast stays armed: a persistent failure must
		// still surface rc + stderr. Pinned here so 001's speed-up cannot be
		// bought by silencing the alarm.
		"TestGitWorktreeCreate_PersistentFailureStillFailsLoudly",
		"TestGitWorktreeCreate_RetriesTransientAddFailure",
	)
}

// TestC1270_003_CmdEvolveNoLongerPaysTheRetryLadder — the END-TO-END proof that
// the blocker is actually cleared, and the only predicate here that binds the
// production dispatch path rather than a unit seam.
//
// It drives one of the 33 affected cmd/evolve tests — which reaches the real
// clock through runLoop → core.Orchestrator.RunCycle → newCycleRun →
// core.gitWorktree.Create — and requires the ladder to have announced ZERO
// times. Today that test prints `[worktree] retry 1/2` and `retry 2/2` and takes
// 6.73s, ~6.00s of it sleep.
//
// Asserting on the ANNOUNCEMENT, not on elapsed time, is deliberate and is the
// difference between a durable predicate and a false-red generator: a wall-clock
// bound is a banned flaky shape (it stretches arbitrarily under fleet load), and
// it would also pass for the wrong reason on a fast host. The retry line is
// deterministic state — the condition is a permanent `not a git repository`, so
// after the fix the count is exactly zero, on any host, under any load.
//
// The test must still PASS. A fix that made cmd/evolve fast by breaking
// provisioning would clear the retry line and fail the PASS assertion.
func TestC1270_003_CmdEvolveNoLongerPaysTheRetryLadder(t *testing.T) {
	const name = "TestLoop_MaxCyclesExit_ClearsCompletedMarker"
	out := runNamedTests(t, cmdPkg, name)
	if !strings.Contains(out, "--- PASS: "+name) {
		t.Fatalf("%s did not pass in %s — the blocker fix must not break loop provisioning.\n"+
			"combined go-test output:\n%s", name, cmdPkg, out)
	}
	if strings.Contains(out, "[worktree] retry ") {
		t.Errorf("the worktree retry ladder still fires under %s.\n"+
			"The failure it retries is PERMANENT (`fatal: not a git repository` on a t.TempDir()), "+
			"so every announcement is 2s+4s of pure sleep bought for nothing — 33 tests x 6s = 198s "+
			"in a package the build floor runs with -timeout 120s. Classify the failure before "+
			"sleeping (gitexec/worktree.go:59-70); do NOT raise the floor's timeout to hide it.\n"+
			"combined go-test output:\n%s", name, out)
	}
}

// ---------------------------------------------------------------------------
// TASK 1 — infra-teardown-predicate-single-source (triage ## top_n).
// Residual only: the consolidation itself is landed and green (IsInfraTeardownError
// adopted at orchestrator.go:60 and cyclerun_dispatch.go:238). Fault-localization
// suspect #5 (0.45) is a guard-COVERAGE hole, not an active defect.
// ---------------------------------------------------------------------------

// TestC1270_004_UnionUniquenessScanCoversConsumerPackages — edit location 7.
//
// TestInfraTeardownUnion_SpelledExactlyOnce calls
// findInfraTeardownUnionSpellings(".") — the scan root is internal/core ONLY.
// The predicate's consumers live outside it (phases/runner/runner.go:889,
// bridge/engine.go:551-579), so a re-spelled union in either package passes the
// "spelled exactly once" guard untouched. No live duplicate exists today, which
// is exactly why this must be pinned now: the guard's whole purpose is the
// item's own "if a THIRD sentinel is ever added" concern, and a guard that
// cannot see two thirds of its own blast radius does not serve it.
//
// DetectsPlantedDuplicateOutsideCore is the anti-no-op assertion and the one
// that carries the criterion: it plants a synthetic union spelling in a package
// OUTSIDE internal/core and requires the scan to REPORT it. Widening the root
// list without widening what the scanner actually sees would pass a
// "roots include X" check and fail this one.
//
// Keep the AST-uniqueness shape — do not degrade it to a string-grep, and do NOT
// collapse the narrower sites while widening: failure_hook.go:79 and
// failure_learning.go:215 are timeout-only, retry_backoff.go:54 and
// retry_opts.go:90 transient-only, ON PURPOSE. 005 pins that they survive.
func TestC1270_004_UnionUniquenessScanCoversConsumerPackages(t *testing.T) {
	assertNamedTestsPass(t, corePkg,
		"TestInfraTeardownUnion_ScanCoversConsumerPackages",
		"TestInfraTeardownUnion_DetectsPlantedDuplicateOutsideCore",
		"TestInfraTeardownUnion_SpelledExactlyOnce",
	)
}

// TestC1270_005_NarrowerPredicateSitesSurviveTheWidening — the negative half of
// AC-1, and the anti-goal the fault-localization report calls out by name.
//
// The cheapest way to make a uniqueness scan green over three packages is to
// "unify" every timeout-only and transient-only site into IsInfraTeardownError.
// That would be a live behaviour regression wearing a consolidation's clothes:
// those sites mean genuinely narrower concepts. These four tests are the
// existing pins; they must stay green THROUGH the widening, so 004 cannot be
// bought by collapsing them.
func TestC1270_005_NarrowerPredicateSitesSurviveTheWidening(t *testing.T) {
	assertNamedTestsPass(t, corePkg,
		"TestTimeoutOnlySites_NotWidenedToUnion",
		"TestWritePhaseFailureDiag_TimeoutOnlyNotWidened",
		"TestIsTransientBridgeError_StaysTransientOnly",
		// Renamed 2026-08-24 (cycle-1551 fix): the gate's single-source
		// predicate widened to IsOptionalSkippableError (infra teardown OR
		// missing persona doc) and the equivalence proof was retargeted with
		// it — same pin, wider predicate, still exactly one spelling.
		"TestOptionalInfraSkip_GateAgreesWithIsOptionalSkippableError",
	)
}

// ---------------------------------------------------------------------------
// TASK 2 — retro-fleet-worktree-dispatch (triage ## top_n).
// Root cause fixed upstream (PR #401, a497ffe1); the item's own acceptance
// criterion is the REGRESSION TEST, which is what is still open.
// Fault-localization suspect #6 (0.40).
// ---------------------------------------------------------------------------

// TestC1270_006_MintedScratchCwdClearsTheFleetGuard — edit location 8, the
// missing JOIN.
//
// Both halves of the fleet-worktree contract are tested today and neither
// proves the contract holds: retro proves it MINTS a scratch cwd
// (retro_worktree_fallback_test.go), and the driver proves it REFUSES an empty
// one (driver_tmux_repl_workdir_test.go:44). Nothing proves a minted ScratchCwd
// directory actually CLEARS the guard at driver_tmux_repl.go:118. A future
// tightening of that guard (e.g. requiring a .git entry) would break every
// fleet-lane retro with both suites still green — the exact silent-regression
// shape this item exists to close.
//
// The positive case is required alongside the existing negative, in one
// predicate, because the pair IS the contract: accepts a real owned cwd, refuses
// an empty one. Do NOT relax the guard to satisfy this — the fix belongs on the
// phase side and already landed at retro.go:87; this asserts the guard accepts a
// legitimate cwd, never that it accepts less.
func TestC1270_006_MintedScratchCwdClearsTheFleetGuard(t *testing.T) {
	assertNamedTestsPass(t, bridgePkg,
		"TestFleetModeAcceptsScratchCwdWorktree",
		"TestFleetModeRefusesEmptyWorktree",
	)
}

// TestC1270_007_RetroFleetDispatchCarriesLaneWorktreeEndToEnd — the item's
// literal acceptance criterion ("a fleet-mode dispatch test proving retro's
// BridgeRequest carries the lane worktree").
//
// The new test must join retro's mint to the bridge's guard through the value
// that actually travels: the worktree retroWorktree resolves must be a real,
// existing, workspace-owned directory that the fleet guard's own predicate
// accepts — not merely a non-empty string. A fabricated path would satisfy
// "non-empty" and still strand the lane at isDir().
//
// The four existing pins ride along because they are the properties a
// convenience fix would quietly trade away: a provisioned worktree must pass
// through VERBATIM (a fallback that fired unconditionally would strand every
// normal retro in an empty scratch dir with no repo), the fallback must never
// resolve to the shared main tree or the dispatching process cwd (the leak the
// guard closes, and the shape refuted PR #400 tried to ship), and with no owned
// workspace it must return "" rather than fabricate a path.
func TestC1270_007_RetroFleetDispatchCarriesLaneWorktreeEndToEnd(t *testing.T) {
	assertNamedTestsPass(t, retroPkg,
		"TestRetroWorktree_FleetScratchCwdSatisfiesBridgeGuardPredicate",
		"TestRetro_EmptyWorktree_FallsBackToScratchUnderWorkspace",
		"TestRetro_EmptyWorktree_NeverMainTreeOrProcessCwd",
		"TestRetro_RealWorktree_PassedThroughUnchanged",
		"TestRetro_EmptyWorktreeAndWorkspace_NoFabricatedPath",
	)
}

// ---------------------------------------------------------------------------
// TASK 3 — test-amplification-context-scope (triage ## top_n).
// The derivation half (CoveringTests, DirectImporters, the artifact, the
// fail-open guard, truncation logging, path sanitisation) is landed and green.
// Fault-localization suspect #7 (0.35) is the open residual.
// ---------------------------------------------------------------------------

// TestC1270_008_AbsentCoveringCorpusIsNeverSilent — edit location 9.
//
// writeCoveringTests is called from inside `if completed == PhaseBuild`
// (phase_bindings.go:351-356). On any path where test-amplification runs without
// a fresh build completion in the SAME process — a resume past build, or a
// future insertion after a different phase — the artifact is absent and the
// phase silently reverts to the whole-repo Grep the corpus exists to remove.
//
// Silent is the defect, not slow. The corpus exists to make a before/after token
// measurement interpretable (5.4M cache-read tokens/run baseline); a run that
// quietly degrades to the unscoped behaviour produces a number nobody can read,
// and no operator signal says which run they are looking at. This mirrors the
// reasoning already applied one layer down, where a TRUNCATED corpus warns
// loudly for exactly the same reason.
//
// Either fix satisfies this — derive the corpus on the paths that precede
// test-amplification, or emit the operator WARN when the phase runs with no
// corpus on disk — but the fail-open contract is not negotiable: an underivable
// diff must still leave the phase working exactly as it does today.
func TestC1270_008_AbsentCoveringCorpusIsNeverSilent(t *testing.T) {
	assertNamedTestsPass(t, corePkg,
		"TestCoveringTests_AbsentCorpusIsAnnouncedNotSilent",
		"TestCoveringTests_AvailableToTestAmplificationWithoutFreshBuild",
	)
}

// TestC1270_009_CoveringCorpusContractSurvives — the regression floor under 008.
//
// Every one of these is a property a "just derive it on more paths" change can
// break, and each was paid for by an incident or an audit finding:
// fail-open when nothing is derivable, an exact truncation count that makes a
// trimmed corpus impossible to mistake for a complete one, and the sanitiser
// that neutralises attacker-influenced filenames before they are interpolated
// into a document agent.md declares AUTHORITATIVE (a backtick closes the code
// span; a newline injects top-level markdown into a code-writing agent's input).
//
// LeavesBenignPathsVerbatim is the OOD counterweight: a sanitiser that mangled
// ordinary Go test paths would pass the injection test and quietly corrupt every
// real corpus.
func TestC1270_009_CoveringCorpusContractSurvives(t *testing.T) {
	assertNamedTestsPass(t, corePkg,
		"TestWriteCoveringTests_NoOpWithoutWorktreeOrWorkspace",
		"TestRenderCoveringTests_ReportsOmittedCount",
		"TestWriteCoveringTests_WarnsLoudlyOnTruncation",
		"TestWriteCoveringTests_SilentWhenNothingTruncated",
		"TestRenderCoveringTests_NeutralizesInjectedMarkdown",
		"TestRenderCoveringTests_LeavesBenignPathsVerbatim",
	)
}
