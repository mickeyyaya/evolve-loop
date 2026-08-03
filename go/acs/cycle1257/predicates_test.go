//go:build acs

// Package cycle1257 materialises the cycle-1257 acceptance criteria for the two
// tasks this fleet lane committed in triage `## top_n`:
//
//	egps-tia-manifest-and-selection          (M — predicates 001-003)
//	egps-tia-stage-config-and-boundary-sweep (S — predicates 003-005)
//
// Nothing was deferred or dropped this cycle, so every top_n task carries
// predicates and no predicate gates non-committed work (the R9.3 floor-binding
// rule).
//
// # The task
//
// The EGPS Go lane runs the FULL `./acs/regression/<sub>` corpus every cycle,
// unconditionally: `goLanePatterns` (acssuite.go:356) enumerates every dir under
// `go/acs/regression/` with no reference to what the cycle actually changed.
// `changedPackagesForCycle` (acssuite.go:716) already computes the changed set,
// but it is used ONLY to export CHANGED_PACKAGES into the predicate env — never
// to select which regression scopes run. This cycle closes that gap in the
// deterministic Meta-PTS shape (no ML): intersect the changed set — widened by
// reverse dependencies — against a per-regression-scope target mapping, and gate
// the whole thing behind a policy-injected `stage: off|shadow|enforce`, with
// full-corpus override triggers so selection can never permanently hide a
// regression class.
//
// The reverse-dependency half is load-bearing, not decoration: cycle-1250
// changed `internal/router` and no changed-scope gate ever selected
// `internal/routingtest`, which imports router and owns the keystone parity
// invariant — main stayed red for 5 commits. `changedpkgs.ImporterClosure`
// (importerclosure.go:42) already landed for exactly this and is the SSOT to
// reuse; a naive exact-string intersect would reproduce the miss.
//
// # Predicate strategy
//
// Every predicate below EXECUTES the system under test through its unit suite,
// filtered by `-run` to named tests in ONE named package — never a source-grep
// of production code (the cycle-85 degenerate-predicate ban) and never a `/...`
// test sweep or a 40s+ suite (the flaky-predicate-shape ban; measured baselines
// are internal/policy 0.74s, internal/acssuite 1.20s).
//
// Asserting each individual `--- PASS: <name>` line — not merely the package
// exit code — is what makes a renamed, skipped, or never-authored test a RED. A
// `-run` expression that matches nothing exits 0, so the exit code alone would
// green a package with zero of the required tests.
//
// The stage/override predicates (004, 005) drive the EXPORTED production entry
// `acssuite.Run` through its `Options.GoExec` seam, so what they assert is the
// pattern set the PRODUCTION path actually hands to `go test`. A test that called
// a selection helper directly would pass on dead code and prove nothing — the
// wiring proof is a reachability test.
//
//   - 001 is the crux: the cycle-1250 reverse-dependency reproducer PLUS its
//     anti-no-op negative (a select-everything filter must FAIL).
//   - 002 pins the scope→target mapping and that the always-on scopes
//     (this-cycle, redteam) can never be filtered away.
//   - 003 is the fail-open floor: an underivable changed set and an unknown
//     stage string both degrade to the FULL corpus, never to a narrower one.
//   - 004 pins the three stage semantics end-to-end through Run.
//   - 005 pins the four full-corpus override triggers, including the inbox
//     item's explicit wiring proof (an out-of-selection regression scope still
//     EXECUTES at the boundary sweep).
//   - 006 is the ADR-0069 repo-wide apicover guard, scoped to the two enrolled
//     packages this cycle mutates, plus the blast-radius build/vet check.
package cycle1257

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	// acssuitePkg and policyPkg are the two named packages under test. Full
	// module paths so the predicate is independent of the acs package's cwd.
	acssuitePkg = "github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	policyPkg   = "github.com/mickeyyaya/evolve-loop/go/internal/policy"
	// modulePkg is used for the build-only blast-radius check (never `go test`).
	modulePkg = "github.com/mickeyyaya/evolve-loop/go/..."
)

// runGoTest runs ONE named package's tests filtered to runExpr and asserts each
// wantPass test reported PASS. See the package doc: the per-test `--- PASS:`
// assertion is the anti-gaming half — a nonexistent test yields exit 0.
func runGoTest(t *testing.T, pkg, runExpr string, wantPass []string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", runExpr, "-v", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %q %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			runExpr, pkg, code, err, stdout, stderr)
	}
	for _, name := range wantPass {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("test %s did not report PASS (renamed, skipped, or not authored)\nstdout:\n%s",
				name, stdout)
		}
	}
}

// -----------------------------------------------------------------------------
// AC1 + AC2 — the cycle-1250 reverse-dependency reproducer, and its negative.
// -----------------------------------------------------------------------------

// TestC1257_001_ReverseDependencySelectionReproducer is the crux predicate.
//
// AC1: with selection active, a cycle whose only changed package is a
// router-shaped leaf MUST still select the regression scope that covers the
// package IMPORTING it — the cycle-1250 miss. Forward-only intersection
// (exact-string package equality, the `topn_gate_slug_identity` disease) does not
// satisfy this; the changed set has to be widened through
// changedpkgs.ImporterClosure before the intersect.
//
// AC2 is the anti-no-op negative and rides in the same predicate deliberately:
// a regression scope covering a package that neither is nor imports anything in
// the changed set MUST NOT be selected. Without it, "select every scope" — i.e.
// today's unconditional behaviour — satisfies AC1 and the feature is worthless.
func TestC1257_001_ReverseDependencySelectionReproducer(t *testing.T) {
	runGoTest(t, acssuitePkg,
		"^TestGoLaneSelection_(ReverseDependencyReproducer|ExcludesUnrelatedScopes)$",
		[]string{
			"TestGoLaneSelection_ReverseDependencyReproducer",
			"TestGoLaneSelection_ExcludesUnrelatedScopes",
		})
}

// -----------------------------------------------------------------------------
// AC3 + AC4 — the scope→target mapping, and the un-filterable always-on scopes.
// -----------------------------------------------------------------------------

// TestC1257_002_TargetMappingAndAlwaysOnScopes executes the two structural
// halves selection rests on.
//
// AC3: each `./acs/regression/<sub>` scope resolves to the set of SOURCE package
// patterns it exercises (derived from predicate metadata or a declared manifest —
// Builder's choice, the contract is the resolved mapping, not its mechanism).
//
// AC4: the always-on scopes — the current cycle's own `./acs/cycle<N>` and
// `./acs/redteam` — are NEVER removed by selection at any stage. Red-team
// predicates are the anti-gaming floor; a selection filter that can drop them
// would let a cycle that touches nothing they map to ship unaudited.
func TestC1257_002_TargetMappingAndAlwaysOnScopes(t *testing.T) {
	runGoTest(t, acssuitePkg,
		"^TestGoLaneSelection_AlwaysOnScopesNeverFiltered$|^TestRegressionTargets_DerivedFromPredicateSources$",
		[]string{
			"TestRegressionTargets_DerivedFromPredicateSources",
			"TestGoLaneSelection_AlwaysOnScopesNeverFiltered",
		})
}

// -----------------------------------------------------------------------------
// AC5 + AC6 — the fail-open floor: every uncertainty widens, never narrows.
// -----------------------------------------------------------------------------

// TestC1257_003_FailsOpenToFullCorpus pins the direction of every degradation.
//
// AC5: when the changed-package set is empty or underivable (no handoff, a git
// failure, a concurrent-fleet index-lock race), selection MUST fall back to the
// FULL corpus. "We could not tell what changed" is the one case where running
// everything is mandatory; an empty changed set intersecting to an empty
// selection would silently green the gate with zero regression coverage — a
// fail-CLOSED bug dressed as an optimisation.
//
// AC6: `ACSConfig.Stage` is config-injected from the policy.json `acs` block —
// never a Go literal, never an env flag (standing rules
// `phase_settings_from_config_not_code`, `no_feature_flags_use_design_patterns`).
// An absent block, an absent key, and an UNKNOWN string all resolve to `off`, so
// a policy typo cannot silently start hiding scopes.
func TestC1257_003_FailsOpenToFullCorpus(t *testing.T) {
	runGoTest(t, acssuitePkg,
		"^TestGoLaneSelection_FailsOpenOnUnderivableChangedSet$",
		[]string{"TestGoLaneSelection_FailsOpenOnUnderivableChangedSet"})
	runGoTest(t, policyPkg,
		"^TestACSConfig_StageDefaultsOffAndParsesFromPolicy$",
		[]string{"TestACSConfig_StageDefaultsOffAndParsesFromPolicy"})
}

// -----------------------------------------------------------------------------
// AC7 + AC8 + AC9 — the three stage semantics, through the production entry.
// -----------------------------------------------------------------------------

// TestC1257_004_StageSemanticsThroughRun drives `acssuite.Run` — the exported
// production entry — once per stage, and asserts on the patterns Run actually
// handed its GoExec seam.
//
// AC7 (`off`, the compiled default): every scope runs. Byte-for-byte today's
// behaviour, so landing this feature changes nothing until an operator opts in.
//
// AC8 (`shadow`): every scope STILL runs — shadow never skips — and the
// would-skip count is surfaced on the returned Verdict, so selection fidelity is
// measurable against the live corpus before `enforce` is ever trusted. A shadow
// stage that skipped anything, or that skipped nothing but reported nothing,
// fails this AC.
//
// AC9 (`enforce`): non-selected regression scopes are not invoked.
func TestC1257_004_StageSemanticsThroughRun(t *testing.T) {
	runGoTest(t, acssuitePkg,
		"^TestRun_Stage(OffRunsFullCorpus|ShadowRunsFullCorpusAndReportsWouldSkip|EnforceSkipsUnselectedScopes)$",
		[]string{
			"TestRun_StageOffRunsFullCorpus",
			"TestRun_StageShadowRunsFullCorpusAndReportsWouldSkip",
			"TestRun_StageEnforceSkipsUnselectedScopes",
		})
}

// -----------------------------------------------------------------------------
// AC10-AC13 — the override triggers, and the inbox item's wiring proof.
// -----------------------------------------------------------------------------

// TestC1257_005_FullCorpusOverrideTriggers pins the safety valves that make this
// a superset-preserving optimisation rather than a coverage cut. All four drive
// exported `Run`.
//
// AC10 (any-red): a RED anywhere in the SELECTED set forces the full corpus in
// the SAME invocation. A red means the change's blast radius was mis-estimated,
// so the estimate stops being trusted immediately — not next cycle.
//
// AC11 (batch boundary): an explicit full-sweep request forces the full corpus
// regardless of stage. This is the operator/boundary escape hatch.
//
// AC12 (weekly floor): when the recorded last-full-sweep is older than the
// configured maximum age, the full corpus is forced and the record refreshed. A
// long run of narrowly-scoped cycles can otherwise leave a regression class
// unexecuted indefinitely; the floor bounds that window in wall-clock terms.
//
// AC13 is the wiring proof the inbox item demands BY NAME, and it is the one
// assertion that cannot be replaced by a claim in a report: a regression scope
// that selection excluded this cycle is observed EXECUTING at the boundary
// sweep. Not "is in the pattern list" — invoked, with a result on the wire.
func TestC1257_005_FullCorpusOverrideTriggers(t *testing.T) {
	runGoTest(t, acssuitePkg,
		"^TestRun_(AnyRedInSelectedScopeForcesFullCorpus|ExplicitFullSweepOverridesSelection|WeeklyFloorForcesFullCorpus|UnselectedRegressionPredicateExecutesAtBoundarySweep)$",
		[]string{
			"TestRun_AnyRedInSelectedScopeForcesFullCorpus",
			"TestRun_ExplicitFullSweepOverridesSelection",
			"TestRun_WeeklyFloorForcesFullCorpus",
			"TestRun_UnselectedRegressionPredicateExecutesAtBoundarySweep",
		})
}

// -----------------------------------------------------------------------------
// AC14 — house rule: new exports are NAMED and EXERCISED; blast radius clean.
// -----------------------------------------------------------------------------

// TestC1257_006_ApicoverEnforcedAndModuleClean is a GUARD (ratchet) predicate: it
// is green on today's tree by design (measured baseline: acssuite 10/10 exported
// covered, policy 127/127, 0 false-green, ~2.1s) and goes red only if this
// cycle's work breaks it.
//
// AC14: both `internal/acssuite` and `internal/policy` are enrolled in
// go/.apicover-enforce (lines 122 and 62), so ADR-0069's repo-wide gate requires
// every export this cycle adds — the stage field, the selection entry, any new
// Options/Verdict surface — to be NAMED in a real assertion that EXECUTES it.
// Enrolled-but-unnamed and named-but-uncovered both fail the tree AFTER the
// cycle ships; running the actual gate scoped to these two packages catches it
// inside the cycle instead. Scoped by an explicit APICOVER_PKGS override so this
// never becomes a whole-repo sweep.
//
// The build/vet half is the blast-radius check: a new field on a widely-embedded
// config struct breaks callers the changed package's own tests never touch.
func TestC1257_006_ApicoverEnforcedAndModuleClean(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")

	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"make", "-C", goDir, "apicover-enforce",
		"APICOVER_PKGS=./internal/acssuite ./internal/policy")
	if code != 0 || err != nil {
		t.Errorf("apicover-enforce failed for the two enrolled packages this cycle mutates "+
			"(exit %d, err=%v) — an added export is uncovered or false-green\nstdout:\n%s\nstderr:\n%s",
			code, err, stdout, stderr)
	}

	stdout, stderr, code, err = acsassert.SubprocessOutput("go", "build", modulePkg)
	if code != 0 || err != nil {
		t.Fatalf("go build %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			modulePkg, code, err, stdout, stderr)
	}

	for _, pkg := range []string{acssuitePkg, policyPkg} {
		stdout, stderr, code, err = acsassert.SubprocessOutput("go", "vet", pkg)
		if code != 0 || err != nil {
			t.Errorf("go vet %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
				pkg, code, err, stdout, stderr)
		}
	}
}
