//go:build acs

// Package cycle1255 materialises the cycle-1255 acceptance criteria for the two
// fleet-scoped tasks triage committed to this lane:
//
//   - retro-fleet-worktree-empty-fallback     (inbox retro-fleet-worktree-dispatch, w=0.9)
//   - test-amplification-covering-tests-scope (inbox test-amplification-context-scope, w=0.89)
//
// Predicate strategy — behavioural-via-subprocess (the cycle-563/987 precedent).
// Each predicate shells `go test -run '^(names)$' -v -count=1 <one named package>`
// over the DEFAULT build suite and requires a `--- PASS: <name>` line per test.
// That genuinely exercises the system under test: the RED contracts authored this
// cycle drive the real retro phase (through its bridge seam) and the real
// changedpkgs deriver, so a predicate greens only when production code makes them
// pass.
//
//   - Asserting on the PASS LINE, not the exit code, is essential: `go test -run`
//     with a pattern matching nothing exits 0 ("no tests to run"), so a still-
//     missing contract would false-GREEN.
//   - A source-grep predicate (FileContains over a .go file) is deliberately
//     avoided — it passes the moment the magic string appears, fix or no fix (the
//     cycle-85 degenerate-predicate ban).
//   - Every `go test` here names EXACTLY ONE package and is neither ./... nor one
//     of the known 40s+ suites (./internal/core, ./cmd/evolve) — the
//     flaky-predicate-shape rules.
//
// 002 is the anti-regression half: it re-runs the UNTOUCHED bridge fleet-refusal
// tests, so "fix" the empty-worktree window by widening the guard itself (the
// blind-widen regression this task's own notes name) stays RED.
package cycle1255

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	retroPkg        = "github.com/mickeyyaya/evolve-loop/go/internal/phases/retro"
	bridgePkg       = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	changedpkgsPkg  = "github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	ampPhaseSpecRel = ".evolve/phases/test-amplification/phase.json"
)

// assertDefaultSuiteTestsPass shells `go test -run '^(names)$' -v -count=1 pkg`
// in the DEFAULT build suite (no -tags) and requires EVERY name to have printed a
// `--- PASS: <name>` line. -count=1 defeats the test cache so the predicate always
// exercises current source.
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
			t.Errorf("default-suite test %s did NOT pass in %s "+
				"(missing, failing, or hidden behind a build tag the default suite skips). exit=%d\n"+
				"combined go-test output:\n%s", name, pkg, code, out)
		}
	}
}

// TestC1255_001_RetroEmptyWorktreeFallsBackToScratch — AC1-AC4 of
// retro-fleet-worktree-empty-fallback. Drives the real retro phase through its
// bridge seam: with an empty req.Worktree under a fleet supervisor the dispatched
// BridgeRequest must carry a real scratch directory under retro's own workspace —
// never the shared main tree, never the process cwd — while a provisioned
// worktree passes through untouched and a workspace-less request fabricates
// nothing.
func TestC1255_001_RetroEmptyWorktreeFallsBackToScratch(t *testing.T) {
	assertDefaultSuiteTestsPass(t, retroPkg,
		"TestRetro_EmptyWorktree_FallsBackToScratchUnderWorkspace",
		"TestRetro_EmptyWorktree_NeverMainTreeOrProcessCwd",
		"TestRetro_RealWorktree_PassedThroughUnchanged",
		"TestRetro_EmptyWorktreeAndWorkspace_NoFabricatedPath",
	)
}

// TestC1255_002_FleetWorktreeGuardNotWidened — the anti-regression AC. The retro
// fallback must be supplied by the PHASE (retro hands the bridge a real cwd); the
// bridge's fleet fail-closed refusal must be bit-for-bit intact. These are the
// pre-existing, untouched guard tests: relaxing errWorktreeRequired to "fix" the
// window turns this predicate RED.
func TestC1255_002_FleetWorktreeGuardNotWidened(t *testing.T) {
	assertDefaultSuiteTestsPass(t, bridgePkg,
		"TestFleetModeRefusesEmptyWorktree",
		"TestRecipeFleetModeRefusesEmptyWorktree",
		"TestApplyScratchCwd_NoOpWhenWorktreeSet",
		"TestApplyScratchCwd_NoOpWhenNoWorkspace",
	)
}

// TestC1255_003_CoveringTestsDeriverBehaviour — AC5-AC8 of
// test-amplification-covering-tests-scope. Exercises the real deriver over real
// on-disk trees: changed packages → their _test.go files only, deduped and
// sorted, both pattern forms accepted, and fail-open (nil, no panic) on every
// unusable input including the module-wide pattern.
func TestC1255_003_CoveringTestsDeriverBehaviour(t *testing.T) {
	assertDefaultSuiteTestsPass(t, changedpkgsPkg,
		"TestCoveringTests_DerivesTestFilesForChangedPackagesOnly",
		"TestCoveringTests_DedupesAcrossOverlappingPatterns",
		"TestCoveringTests_AcceptsNonRecursivePatternForm",
		"TestCoveringTests_FailsOpenOnUnusableInput",
	)
}

// TestC1255_004_CoveringTestsReachableFromProduction — AC9, the WIRING proof. The
// deriver must be called from a real non-test file in the go/ module (resolved
// from the parsed import graph, not a grep). A seam whose only caller is a test
// injects nothing into the phase and saves zero tokens.
func TestC1255_004_CoveringTestsReachableFromProduction(t *testing.T) {
	assertDefaultSuiteTestsPass(t, changedpkgsPkg,
		"TestCoveringTests_ReachableFromProduction",
	)
}

// TestC1255_005_AmplificationPhaseDeclaresCoveringTestsInput — AC10, the
// consumption half: the derived corpus reaches the agent only if the phase spec
// declares it as an input. Parses the real phase.json (structured config parse,
// not a source grep) and requires an inputs.files entry naming a covering-tests
// artifact under the cycle run dir, with the {cycle} placeholder intact so it
// resolves per cycle rather than being pinned to one run.
//
// acs-predicate: config-check
func TestC1255_005_AmplificationPhaseDeclaresCoveringTestsInput(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, filepath.FromSlash(ampPhaseSpecRel))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", ampPhaseSpecRel, err)
	}
	var spec struct {
		Inputs struct {
			Files []string `json:"files"`
		} `json:"inputs"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("%s is not valid JSON: %v", ampPhaseSpecRel, err)
	}

	var hit string
	for _, f := range spec.Inputs.Files {
		if strings.Contains(f, "covering-tests") {
			hit = f
			break
		}
	}
	if hit == "" {
		t.Fatalf("%s inputs.files = %v — none names a covering-tests artifact, so the derived corpus never reaches the agent and the blind whole-repo search continues", ampPhaseSpecRel, spec.Inputs.Files)
	}
	if !strings.Contains(hit, "{cycle}") {
		t.Errorf("covering-tests input %q has no {cycle} placeholder — it would pin every cycle to one run's artifact", hit)
	}
	// The pre-existing inputs must survive: dropping them would starve the agent
	// of the contract it amplifies against.
	for _, want := range []string{"tdd-contract.md", "build-report.md"} {
		found := false
		for _, f := range spec.Inputs.Files {
			if strings.Contains(f, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s inputs.files = %v — the pre-existing %s input was dropped", ampPhaseSpecRel, spec.Inputs.Files, want)
		}
	}
}
