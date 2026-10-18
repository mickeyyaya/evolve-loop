package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const selfcheckPath = "go/internal/core/phase_bindings_selfcheck.go"

const breakerPath = "go/internal/core/blocker_breaker.go"

const cycle1946StrictFailure = "changed package(s) listed in go/.cover-strict are below their floor; test each uncovered block below, unless its note says a test cannot reach it:\n" +
	"    cover strict: internal/core: 99.97% (3412 of 3413 statements) < 100.0% FAIL\n" +
	"      uncovered go/internal/core/phase_bindings_selfcheck.go:145-148 (2 statement(s))"

const cycle1946PredicateFailure = "./acs/cycle1946 (-tags acs): unit tests FAIL\n" +
	"--- FAIL: TestC1946_001_CoreStatementCoverageIsFull (197.14s)\n" +
	"    predicates_test.go:52: 1 function(s) of ./internal/core below 100.0%:\n" +
	"        github.com/mickeyyaya/evolve-loop/go/internal/core/phase_bindings_selfcheck.go:138: writeBuildSelfCheckArtifact 81.8%\n" +
	"FAIL\nFAIL\tgithub.com/mickeyyaya/evolve-loop/go/acs/cycle1946\t197.400s\nFAIL\n"

const cycle1946Claim = "BLOCKER protected-path go/internal/core/phase_bindings_selfcheck.go:138 the json.MarshalIndent error branch needs a seam in a file this phase may not edit"

const cycle1950Failure = "./acs/cycle1950 (-tags acs): unit tests FAIL\n" +
	"--- FAIL: TestC1950_001_CoreLeaseBreakerCarryforwardBatchReachesFullCoverage (1.19s)\n" +
	"    predicates_test.go:74: blocker_breaker.go:AppendResolvedFingerprint is at 95.2% under the ^TestCoreCoverageBatch_ batch tests; want 100.0% (error branches included)\n" +
	"FAIL\nFAIL\tgithub.com/mickeyyaya/evolve-loop/go/acs/cycle1950\t1.471s\nFAIL\n"

const cycle1950Claim = "BLOCKER predicate-conflict TestC1950_001_CoreLeaseBreakerCarryforwardBatchReachesFullCoverage TestC1950_002_BatchAddsNoProductionEdits go/internal/core/blocker_breaker.go:76-77 001 demands the dead branch covered and 002 forbids the production edit that removes it"

const cycle1950Predicates = `//go:build acs

package cycle1950

func TestC1950_001_CoreLeaseBreakerCarryforwardBatchReachesFullCoverage(t *testing.T) {
	run("go", "test", "./internal/core")
}

func TestC1950_002_BatchAddsNoProductionEdits(t *testing.T) {
	out := run("git", "diff", "--name-only", "origin/main...HEAD", "--", "go/internal/core")
	_ = out
}

func TestC1950_003_Unrelated(t *testing.T) {
	run("go", "test", "./internal/other")
}
`

func protectsSelfcheck(p string) bool { return p == selfcheckPath }

func blockerWorktree(t *testing.T) string {
	t.Helper()
	wt := t.TempDir()
	for _, rel := range []string{selfcheckPath, breakerPath, "go/internal/core/x_test.go"} {
		writeBlockerFile(t, wt, rel, "package core\n")
	}
	writeBlockerFile(t, wt, "go/acs/cycle1950/predicates_test.go", cycle1950Predicates)
	return wt
}

func writeBlockerFile(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func blockerCase(wt string, failures, defects []string) BlockerCase {
	return BlockerCase{
		Phase:               string(PhaseBuild),
		Worktree:            wt,
		PredicatesAtRequest: map[string]string{"predicates_test.go": cycle1950Predicates},
		PredicatesNow:       map[string]string{"predicates_test.go": cycle1950Predicates},
		Protected:           protectsSelfcheck,
		Failures:            failures,
		Defects:             defects,
		Explainable:         buildFloorExplainable,
	}
}

func TestBlockerProver_Cycle1946AProtectedSeamIsAProvenBlocker(t *testing.T) {
	t.Parallel()
	c := blockerCase(blockerWorktree(t), []string{cycle1946StrictFailure, cycle1946PredicateFailure}, []string{"other defect text", cycle1946Claim})

	proof := NewBlockerProver().Prove(c)

	if !proof.Blocked() || len(proof.Proven) != 1 || proof.Proven[0].Path != selfcheckPath {
		t.Fatalf("proof = %+v; the 1946 seam is a real blocker", proof)
	}
}

func TestBlockerProver_Cycle1950TwoConflictingPredicatesAreAProvenBlocker(t *testing.T) {
	t.Parallel()
	c := blockerCase(blockerWorktree(t), []string{cycle1950Failure}, []string{cycle1950Claim})

	proof := NewBlockerProver().Prove(c)

	if !proof.Blocked() || proof.Proven[0].Kind != BlockerKindPredicateConflict {
		t.Fatalf("proof = %+v; the 1950 conflict is a real blocker", proof)
	}
}

func TestBlockerProver_AClaimThatIsNotRealStillFails(t *testing.T) {
	t.Parallel()
	conflict := func(failing, guard, path string) string {
		return "BLOCKER predicate-conflict " + failing + " " + guard + " " + path + " why"
	}
	const t1, t2 = "TestC1950_001_CoreLeaseBreakerCarryforwardBatchReachesFullCoverage", "TestC1950_002_BatchAddsNoProductionEdits"
	cases := map[string]struct {
		failures []string
		claim    string
		edit     func(*BlockerCase)
		refusal  string
	}{
		"an unprotected path": {
			[]string{cycle1950Failure}, "BLOCKER protected-path " + breakerPath + ":76 why", nil, "not on the protected-surface manifest",
		},
		"a protected path the lane changed": {
			[]string{cycle1946PredicateFailure}, cycle1946Claim, func(c *BlockerCase) { c.Changed = []string{selfcheckPath} }, "the lane changed it",
		},
		"no manifest wired": {
			[]string{cycle1946PredicateFailure}, cycle1946Claim, func(c *BlockerCase) { c.Protected = nil }, "no protected-surface manifest",
		},
		"a path that does not exist": {
			[]string{cycle1946PredicateFailure}, "BLOCKER protected-path go/internal/core/gone.go why", func(c *BlockerCase) { c.Protected = func(string) bool { return true } }, "does not exist",
		},
		"a test file": {
			[]string{cycle1946PredicateFailure}, "BLOCKER protected-path go/internal/core/x_test.go why", func(c *BlockerCase) { c.Protected = func(string) bool { return true } }, "not a non-test Go source file",
		},
		"a path outside the tree": {
			[]string{cycle1946PredicateFailure}, "BLOCKER protected-path ../etc/x.go why", nil, "not a non-test Go source file",
		},
		"an unknown kind": {
			[]string{cycle1946PredicateFailure}, "BLOCKER too-hard " + selfcheckPath + " why", nil, "unknown blocker kind",
		},
		"a bare BLOCKER word": {
			[]string{cycle1946PredicateFailure}, "BLOCKER", nil, "malformed",
		},
		"a conflict on a missing path": {
			[]string{cycle1950Failure}, conflict(t1, t2, "go/internal/core/gone.go"), nil, "does not exist",
		},
		"a claim with no path": {
			[]string{cycle1946PredicateFailure}, "BLOCKER protected-path", nil, "malformed",
		},
		"a conflict with too few fields": {
			[]string{cycle1950Failure}, "BLOCKER predicate-conflict " + t1 + " " + t2, nil, "malformed",
		},
		"a conflict that names one predicate twice": {
			[]string{cycle1950Failure}, conflict(t1, t1, breakerPath), nil, "names one predicate twice",
		},
		"a conflict with a predicate of another cycle": {
			[]string{cycle1950Failure}, conflict(t1, "TestC1949_002_Other", breakerPath), nil, "not a predicate of this cycle",
		},
		"a conflict whose failing predicate passes": {
			[]string{cycle1946PredicateFailure}, conflict(t1, t2, breakerPath), nil, "does not fail at the floor",
		},
		"a conflict whose guard fails too": {
			[]string{cycle1950Failure + "--- FAIL: " + t2 + " (0.01s)\n"}, conflict(t1, t2, breakerPath), nil, "fails at the floor too",
		},
		"a conflict whose guard fails in a subtest": {
			[]string{cycle1950Failure + "    --- FAIL: " + t2 + "/sub (0.01s)\n"}, conflict(t1, t2, breakerPath), nil, "fails at the floor too",
		},
		"a conflict whose guard does not guard the path": {
			[]string{cycle1950Failure}, conflict(t1, "TestC1950_003_Unrelated", breakerPath), nil, "does not guard",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := blockerCase(blockerWorktree(t), tc.failures, []string{tc.claim})
			if tc.edit != nil {
				tc.edit(&c)
			}
			proof := NewBlockerProver().Prove(c)
			if proof.Blocked() {
				t.Fatalf("a claim that is not real escaped the failure: %+v", proof)
			}
			if !strings.Contains(strings.Join(proof.Refusals, "\n"), tc.refusal) {
				t.Errorf("refusals = %q; want one that says %q", proof.Refusals, tc.refusal)
			}
		})
	}
}

func TestBlockerProver_AProvenClaimDoesNotExcuseAFailureItDoesNotExplain(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"a second uncovered file":                 cycle1946StrictFailure + "\n      uncovered go/internal/core/blocker_breaker.go:70-72 (1 statement(s))",
		"a cut diagnostic":                        "./acs/cycle1946 (-tags acs): unit tests FAIL\n…" + selfcheckPath,
		"a plain unit-test failure":               "./internal/core: unit tests FAIL\n" + selfcheckPath,
		"a failure that names no file":            "./acs/cycle1946 (-tags acs): unit tests FAIL\n--- FAIL: TestC1946_002 (0.1s)\n",
		"a second failing predicate with no file": "./acs/cycle1946 (-tags acs): unit tests FAIL\n--- FAIL: TestC1946_002 (0.1s)\n    got 3, want 4\n--- FAIL: TestC1946_001 (0.1s)\n    " + selfcheckPath + ":138 at 81.8%\n",
		"a second strict package with no file":    cycle1946StrictFailure + "\n    cover strict: internal/other: 90.0% < 100.0% FAIL",
		"a header that names another file":        "./acs/cycle1946 (-tags acs): unit tests FAIL\ngo/internal/core/plain.go\n--- FAIL: TestC1946_001 (0.1s)\n    " + selfcheckPath + "\n",
	}
	for name, extra := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := blockerCase(blockerWorktree(t), []string{cycle1946PredicateFailure, extra}, []string{cycle1946Claim})
			proof := NewBlockerProver().Prove(c)
			if proof.Blocked() {
				t.Fatalf("a failure the claim does not explain was excused: %+v", proof)
			}
			if !strings.Contains(strings.Join(proof.Refusals, "\n"), "no proven blocker explains") {
				t.Errorf("refusals = %q", proof.Refusals)
			}
		})
	}
}

func TestBlockerProver_NoClaimMeansNoProofAndNoRefusal(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]BlockerCase{
		"no defect":        blockerCase(blockerWorktree(t), []string{cycle1946PredicateFailure}, []string{"plain defect"}),
		"no floor failure": blockerCase(blockerWorktree(t), nil, []string{cycle1946Claim}),
		"no explainability": func() BlockerCase {
			c := blockerCase(t.TempDir(), []string{cycle1946PredicateFailure}, []string{cycle1946Claim})
			c.Explainable = nil
			return c
		}(),
	} {
		proof := NewBlockerProver().Prove(c)
		if proof.Blocked() || len(proof.Proven) != 0 {
			t.Errorf("%s: proof = %+v", name, proof)
		}
	}
}

func TestBuildFloorExplainable_OnlyCoverageAndPredicateFailures(t *testing.T) {
	t.Parallel()
	for failure, want := range map[string]bool{
		cycle1946StrictFailure:                                   true,
		cycle1946PredicateFailure:                                true,
		"./internal/core: unit tests FAIL\nx":                    false,
		"protected control-plane path changed: " + selfcheckPath: false,
	} {
		if got := buildFloorExplainable(failure); got != want {
			t.Errorf("buildFloorExplainable(%q) = %v, want %v", failure, got, want)
		}
	}
}

func TestBlockerRouteDiagnostic_NamesTheItemTheProofAndTheDiagnosis(t *testing.T) {
	t.Parallel()
	proof := NewBlockerProver().Prove(blockerCase(blockerWorktree(t), []string{cycle1950Failure}, []string{cycle1950Claim}))

	d := BlockerRouteDiagnostic(string(PhaseBuild), "item-1", proof)

	if d.Code != DiagCodeBlockerRoute || d.Subject != "item-1" || d.Severity != "error" {
		t.Errorf("diagnostic = %+v", d)
	}
	for _, want := range []string{"blocked: needs console", "build", "predicate-conflict", breakerPath, "TestC1950_002_BatchAddsNoProductionEdits", "forbids the production edit"} {
		if !strings.Contains(d.Message, want) {
			t.Errorf("message %q lacks %q", d.Message, want)
		}
	}
}

func TestBlockerProver_TheEvidenceMustBeCompleteAndUntouched(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		edit    func(*BlockerCase)
		refusal string
	}{
		"the lane edited a predicate": {func(c *BlockerCase) {
			c.PredicatesNow = map[string]string{"predicates_test.go": cycle1950Predicates + "\nfunc TestC1950_004(t *testing.T) {}\n"}
		}, "changed the predicates"},
		"the lane added a predicate file": {func(c *BlockerCase) {
			c.PredicatesNow = map[string]string{"predicates_test.go": cycle1950Predicates, "guard_test.go": "package cycle1950\n"}
		}, "changed the predicates"},
		"the diff could not be read":   {func(c *BlockerCase) { c.EvidenceFault = "git diff failed" }, "not complete"},
		"no explainable failure kinds": {func(c *BlockerCase) { c.Explainable = nil }, "no failure kinds"},
		"the lane removed test lines in the package": {func(c *BlockerCase) {
			c.ShrunkTests = []string{"go/internal/core/blocker_breaker_test.go"}
		}, "removed test lines"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := blockerCase(blockerWorktree(t), []string{cycle1950Failure}, []string{cycle1950Claim})
			tc.edit(&c)
			proof := NewBlockerProver().Prove(c)
			if proof.Blocked() || !strings.Contains(strings.Join(proof.Refusals, "\n"), tc.refusal) {
				t.Errorf("proof = %+v; want a refusal that says %q", proof, tc.refusal)
			}
		})
	}
}

func TestBlockerProver_ASymlinkIsNotTheProtectedFile(t *testing.T) {
	t.Parallel()
	wt := blockerWorktree(t)
	if err := os.Remove(filepath.Join(wt, filepath.FromSlash(selfcheckPath))); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(wt, filepath.FromSlash(breakerPath)), filepath.Join(wt, filepath.FromSlash(selfcheckPath))); err != nil {
		t.Fatal(err)
	}
	if proof := NewBlockerProver().Prove(blockerCase(wt, []string{cycle1946PredicateFailure}, []string{cycle1946Claim})); proof.Blocked() {
		t.Errorf("proof = %+v", proof)
	}
}

func TestCyclePredicateSources_ReadsEachGoFileOfTheCycle(t *testing.T) {
	t.Parallel()
	wt := blockerWorktree(t)
	writeBlockerFile(t, wt, "go/acs/cycle1950/doc.md", "not go")
	if err := os.MkdirAll(filepath.Join(wt, "go", "acs", "cycle1950", "dir.go"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := cyclePredicateSources(wt, 1950)

	if got["predicates_test.go"] != cycle1950Predicates || !strings.HasPrefix(got["dir.go"], "unreadable: ") || len(got) != 2 {
		t.Errorf("sources = %v", got)
	}
	if empty := cyclePredicateSources("", 1950); len(empty) != 0 {
		t.Errorf("no worktree, no sources: %v", empty)
	}
}

func TestPhaseVerdictDefects_OnlyAFailureBlockGivesDefects(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if got := phaseVerdictDefects(ws, PhaseBuild); got != nil {
		t.Errorf("no report: %v", got)
	}
	writeBlockerFile(t, ws, "build-report.md", "## Changes\n<!-- evolve-verdict: {\"phase\":\"build\",\"verdict\":\"PASS\"} -->\n")
	if got := phaseVerdictDefects(ws, PhaseBuild); got != nil {
		t.Errorf("a PASS verdict has no defects: %v", got)
	}
}

func TestBlockerProver_AFailureWithNoTestItemsIsJudgedWhole(t *testing.T) {
	t.Parallel()
	head := "./acs/cycle1946 (-tags acs): unit tests FAIL\n# build output\n"
	for failure, want := range map[string]bool{
		head + selfcheckPath + ":3: undefined: x\n": true,
		head + "exit status 2\n":                    false,
	} {
		c := blockerCase(blockerWorktree(t), []string{failure}, []string{cycle1946Claim})
		if got := NewBlockerProver().Prove(c).Blocked(); got != want {
			t.Errorf("Blocked() = %v for %q, want %v", got, failure, want)
		}
	}
}
