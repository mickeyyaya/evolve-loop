//go:build acs

// Package cycle1532 materialises the cycle-1532 acceptance criteria for the two
// fleet-scoped tasks pinned to the `pipeline-defect-infra-systemic` lane:
//
//   - pipeline-timeout-lesson-e2e        → the cross-phase persistence/planning contract
//   - pipeline-replay-contract-boundary  → the captured-pane transient boundary
//
// What is NOT yet proven. #478 (artifact-timeout transient disclosure) and #479
// (judgment-FAIL lesson recording) are each green inside their OWN package, and
// each is silent about the other. The two meet at a boundary neither landing
// exercises: a diagnosed infrastructure timeout must stay non-retryable (exit 81
// is NOT ErrTransientBridgeFailure) while a judgment verdict must stay a
// PLANNER-VISIBLE lesson that creates no failure-adapter halt vector. Today the
// judgment-lesson tests stop at state.CarryoverTodos; nothing asserts the lesson
// survives persistence AND the advisor prompt's 20-entry cap to reach the next
// cycle's planning surface (writeRoutingContext, the real Scout-facing render).
//
// Predicate strategy — each predicate exercises the SYSTEM, never a source grep
// of production code (the cycle-85 degenerate-predicate ban). Because the seams
// under test (recordAndBranch, isTransientBridgeError, classifyTransientPane,
// writeRoutingContext) are all UNEXPORTED, they are unreachable from a leaf acs
// package; every predicate therefore drives them through the sanctioned
// behavioural-via-subprocess shape (the cycle-987/997 precedent): a `-run`-
// narrowed, single-named-package `go test -v` that must print `--- PASS: <name>`
// for each binding test Builder authors. Asserting on the PASS LINE — never on
// exit 0 — is load-bearing: `go test -run` on a pattern matching NO test exits 0
// with "no tests to run", so a still-missing binding test would false-GREEN.
//
// Invocations are `-run`-narrowed against ONE named package each, never a `/...`
// sweep, so a concurrent lane's contamination in an untouched package can never
// red this cycle (the flaky-predicate-shape / scope-lint contract).
package cycle1532

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// corePkg / bridgePkg are the two DEFAULT-suite packages that own this lane's
// binding tests. Both are named in triage's committed file set, so both are in
// this cycle's touched scope.
const (
	corePkg   = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

// assertDefaultSuiteTestsPass shells `go test -run '^(names)$' -count=1 -v pkg`
// in the DEFAULT build suite (no -tags) and requires EVERY name to print a
// `--- PASS: <name>` line. -count=1 defeats the test cache, so a stale cached
// result from an earlier phase can never stand in for a live run.
func assertDefaultSuiteTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-count=1", "-v", pkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("binding test %s did NOT pass in %s "+
				"(missing, failing, or hidden behind a build tag). exit=%d\ncombined go-test output:\n%s",
				name, pkg, code, out)
		}
	}
}

// TestC1532_001_JudgmentLessonReachesNextCyclePlannerPrompt — AC1/AC2/AC3 of
// pipeline-timeout-lesson-e2e. The #479 tests stop at the in-memory todo array.
// The pipeline-facing claim is stronger and untested: the lesson must survive
// PERSISTENCE and the advisor prompt's maxCarryoverTodosInPrompt cap to appear
// on the surface the next cycle's planner actually reads (writeRoutingContext),
// and a CONTROL phase's FAIL must reach that surface not at all.
//
//   - ...ReachesNextCyclePlannerPrompt — re-read the PERSISTED state (not the
//     in-memory one) and render the planner context; the objection text must be
//     there. A lesson that lives only in RAM dies on the abort branches this fix
//     exists for, and a lesson dropped by the renderer teaches nobody.
//   - ...SurvivesPlannerPromptCapUnderCrowdedCarryover — the EDGE case: with more
//     than maxCarryoverTodosInPrompt lower-priority todos resident, the P1 lesson
//     must still render. This is the live shape (the array carries dozens of
//     entries), and a naive insertion-order prefix silently drops the newest.
//   - ...ControlPhaseFAILReachesNoPlannerPrompt — the NEGATIVE: retro/debugger
//     FAIL must contribute nothing. Retro WRITES lessons; teaching from its own
//     FAIL recurses into its output. Without this, a predicate that merely
//     asserts "some todo rendered" passes on a teach-everything implementation.
func TestC1532_001_JudgmentLessonReachesNextCyclePlannerPrompt(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestJudgmentLessonEndToEnd_ReachesNextCyclePlannerPrompt",
		"TestJudgmentLessonEndToEnd_SurvivesPlannerPromptCapUnderCrowdedCarryover",
		"TestJudgmentLessonEndToEnd_ControlPhaseFAILReachesNoPlannerPrompt",
	)
}

// TestC1532_002_TimeoutDiagnosticNeverMutatesRetryOrFailureLearning — AC4, the
// crux of the cross-phase composition and ADR-0090 decisions 1/2. One cycle
// carries BOTH an artifact-timeout death and a non-authoritative judgment FAIL.
// The binding test must assert, in that composed state:
//
//   - bridgeExitCode(ErrArtifactTimeout) == 81 — the classification is unchanged;
//   - isTransientBridgeError(ErrArtifactTimeout) == false — the diagnostic data
//     added by #478 did NOT widen the retry sentinel (exit 81 stays
//     non-retryable, transient-bridge-retry AC-1);
//   - state.FailedAt is unchanged by the judgment lesson — teaching must not
//     import a halt vector into failureadapter.Decide (sameClassStreak /
//     tailInfraTransientStreak), even when a real infra failure is also present.
//
// The composition is the point: each half is already unit-green in isolation, so
// a test that checks them separately proves nothing this predicate wants. The
// cheapest fake this blocks is exactly that — two unit assertions that never
// share a cycle state.
func TestC1532_002_TimeoutDiagnosticNeverMutatesRetryOrFailureLearning(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestArtifactTimeoutEndToEnd_DiagnosticNeverMutatesRetryOrFailureLearning",
	)
}

// TestC1532_003_TransientRecognitionIsManifestScopedNotHardCoded — AC5/AC6 of
// pipeline-replay-contract-boundary. #478's family test proves every family
// DECLARES a transient_regex; it does not prove recognition is actually SOURCED
// from the launched family's manifest. The two named binding tests close the
// gaming vectors the scout named:
//
//   - ...LivePaneIsFamilyScopedNotHardCodedText — the cycle-1523 captured claude
//     pane must classify transient under claude-tmux and NOT under codex-tmux,
//     agy-tmux, or ollama-tmux. An implementation that hard-codes "529" or any
//     provider prose lights all four and fails here; only a manifest-resolved
//     pattern discriminates.
//   - ...EchoedProviderTextNeverClassifiesForAnyFamily — pane text the AGENT
//     merely quoted or echoed from its prompt must never classify, for every
//     family (the existing coverage is claude-only). This is both the F1
//     indirect-prompt-injection boundary and the anti-gaming one: a classifier
//     that scans the raw pane instead of the stripped pane passes today's tests
//     and fails this.
func TestC1532_003_TransientRecognitionIsManifestScopedNotHardCoded(t *testing.T) {
	assertDefaultSuiteTestsPass(t, bridgePkg,
		"TestArtifactTimeoutTransient_LivePaneIsFamilyScopedNotHardCodedText",
		"TestArtifactTimeoutTransient_EchoedProviderTextNeverClassifiesForAnyFamily",
	)
}

// TestC1532_004_LandedRegressionsNotWeakened is the ANTI-WEAKENING floor for both
// tasks. This cycle's whole deliverable is regression strength, so the cheapest
// way to green it is to relax the #478/#479 tests the new ones sit beside —
// widen an assertion, delete a negative case, retire a fixture. Naming the nine
// landed binding tests here makes that path RED: a deleted or renamed test
// prints no PASS line, and a weakened one that starts failing prints FAIL.
//
// Deliberately NOT a whole-package sweep of core/bridge: that samples untouched
// code and manufactures false reds under fleet load (the cycles 1115/1117 class).
// Each invocation is `-run`-narrowed to the named tests in ONE package.
func TestC1532_004_LandedRegressionsNotWeakened(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestRecordAndBranch_PremiseChallengeFAILTeaches",
		"TestRecordAndBranch_PremiseChallengeFAILDoesNotRecordFailedApproach",
		"TestJudgmentLesson_ControlPhasesDoNotTeach",
		"TestJudgmentLesson_PersistedToStorage",
		"TestJudgmentLesson_AuthoritativePhaseYieldsToFloorRecorder",
	)
	assertDefaultSuiteTestsPass(t, bridgePkg,
		"TestRunTmuxREPL_ArtifactTimeout_MarkerFlagsTransientOnLivePane",
		"TestRunTmuxREPL_ArtifactTimeout_SilentPaneIsNotTransient",
		"TestClassifyTransientPane_IgnoresEchoedPromptText",
		"TestClassifyTransientPane_UnknownDriverFailsOpen",
	)
}
