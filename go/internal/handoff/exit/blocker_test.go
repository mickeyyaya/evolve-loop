package exit_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/exit"
)

const (
	selfcheckPath = "go/internal/core/phase_bindings_selfcheck.go"
	breakerPath   = "go/internal/core/blocker_breaker.go"
	coverAsk      = "TestC1950_001_CoreLeaseBreakerCarryforwardBatchReachesFullCoverage"
	guardAsk      = "TestC1950_002_BatchAddsNoProductionEdits"
)

const selfcheckClaim = "BLOCKER protected-path go/internal/core/phase_bindings_selfcheck.go:138 the json.MarshalIndent error branch needs a seam in a file this phase may not edit"

const selfcheckFailure = "./acs/cycle1946 (-tags acs): unit tests FAIL\n" +
	"--- FAIL: TestC1946_001_CoreStatementCoverageIsFull (197.14s)\n" +
	"    predicates_test.go:52: 1 function(s) of ./internal/core below 100.0%:\n" +
	"        github.com/mickeyyaya/evolve-loop/go/internal/core/phase_bindings_selfcheck.go:138: writeBuildSelfCheckArtifact 81.8%\n" +
	"FAIL\nFAIL\tgithub.com/mickeyyaya/evolve-loop/go/acs/cycle1946\t197.400s\nFAIL\n"

const conflictClaim = "BLOCKER predicate-conflict " + coverAsk + " " + guardAsk + " go/internal/core/blocker_breaker.go:76-77 001 demands the dead branch covered and 002 forbids the production edit that removes it"

const conflictFailure = "./acs/cycle1950 (-tags acs): unit tests FAIL\n" +
	"--- FAIL: " + coverAsk + " (1.19s)\n" +
	"    predicates_test.go:74: blocker_breaker.go:AppendResolvedFingerprint is at 95.2% under the ^TestCoreCoverageBatch_ batch tests; want 100.0% (error branches included)\n" +
	"FAIL\nFAIL\tgithub.com/mickeyyaya/evolve-loop/go/acs/cycle1950\t1.471s\nFAIL\n"

const cycle1950Predicates = `//go:build acs

package cycle1950

func TestC1950_001_CoreLeaseBreakerCarryforwardBatchReachesFullCoverage(t *testing.T) {
	run("go", "test", "./internal/core")
}

func TestC1950_002_BatchAddsNoProductionEdits(t *testing.T) {
	out := run("git", "diff", "--name-only", "origin/main...HEAD", "--", "go/internal/core")
	_ = out
}
`

func laneWorktree(t *testing.T) string {
	t.Helper()
	wt := t.TempDir()
	for rel, body := range map[string]string{
		selfcheckPath:                         "package core\n",
		breakerPath:                           "package core\n",
		"go/acs/cycle1950/predicates_test.go": cycle1950Predicates,
	} {
		p := filepath.Join(wt, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return wt
}

func seamProver(t *testing.T, protected func(string) bool, failures ...string) exit.Prover {
	t.Helper()
	wt := laneWorktree(t)
	predicates := map[string]string{"predicates_test.go": cycle1950Predicates}
	return func(defects []string) exit.Proof {
		proof := core.NewBlockerProver().Prove(core.BlockerCase{
			Phase:               "build",
			Worktree:            wt,
			PredicatesAtRequest: predicates,
			PredicatesNow:       predicates,
			Protected:           protected,
			Failures:            failures,
			Defects:             defects,
			Explainable:         func(f string) bool { return strings.Contains(f, "unit tests FAIL") },
		})
		out := exit.Proof{Blocked: proof.Blocked()}
		for _, c := range proof.Proven {
			out.Proven = append(out.Proven, exit.Claim{Defect: c.Defect, Path: c.Path, Failing: c.Failing, Guard: c.Guard})
		}
		return out
	}
}

func blockedResult(ask, claim string) exit.Result {
	return exit.Result{Ask: ask, Status: handoff.StatusBlocked, Resolved: true, Claim: claim}
}

func conflictRequest() handoff.Request {
	return handoff.Request{Pair: "tdd-build", Asks: []handoff.Ask{{ID: coverAsk, Required: true}, {ID: guardAsk, Required: true}}}
}

func selfcheckRequest() handoff.Request {
	return handoff.Request{Pair: "tdd-build", Asks: []handoff.Ask{{ID: "a1", Required: true}, {ID: "a2", Required: true}}}
}

func TestDecide_TheCycle1950ConflictBlocksWithBothAskIds(t *testing.T) {
	policy := exit.Policy{MaxRedispatch: 2, Prover: seamProver(t, guards.TouchesProtectedSurface, conflictFailure)}
	cases := map[string][]exit.Result{
		"the guard ask is met":                          {blockedResult(coverAsk, conflictClaim), met(guardAsk)},
		"the guard ask is unmet":                        {blockedResult(coverAsk, conflictClaim), unmet(guardAsk)},
		"the claim sits on the guard ask":               {unmet(coverAsk), blockedResult(guardAsk, conflictClaim)},
		"the results come in the reverse request order": {met(guardAsk), blockedResult(coverAsk, conflictClaim)},
	}
	for name, results := range cases {
		t.Run(name, func(t *testing.T) {
			got := exit.Decide(conflictRequest(), results, nil, policy)
			if got.Outcome != exit.Blocked {
				t.Fatalf("Outcome = %q, want %q (decision %+v)", got.Outcome, exit.Blocked, got)
			}
			if want := []string{coverAsk, guardAsk}; !reflect.DeepEqual(got.Blocked, want) {
				t.Errorf("Blocked = %v, want %v", got.Blocked, want)
			}
			if want := []string{breakerPath}; !reflect.DeepEqual(got.Paths, want) {
				t.Errorf("Paths = %v, want %v", got.Paths, want)
			}
		})
	}
}

func TestDecide_AnAskThatNeedsAProtectedPathBlocksWithThePath(t *testing.T) {
	policy := exit.Policy{MaxRedispatch: 2, Prover: seamProver(t, guards.TouchesProtectedSurface, selfcheckFailure)}
	got := exit.Decide(selfcheckRequest(), []exit.Result{blockedResult("a1", selfcheckClaim), met("a2")}, nil, policy)
	if got.Outcome != exit.Blocked {
		t.Fatalf("Outcome = %q, want %q (decision %+v)", got.Outcome, exit.Blocked, got)
	}
	if want := []string{"a1"}; !reflect.DeepEqual(got.Blocked, want) {
		t.Errorf("Blocked = %v, want %v", got.Blocked, want)
	}
	if want := []string{selfcheckPath}; !reflect.DeepEqual(got.Paths, want) {
		t.Errorf("Paths = %v, want %v", got.Paths, want)
	}
}

func TestDecide_ABlockedClaimWithoutProofIsUnmet(t *testing.T) {
	results := []exit.Result{blockedResult("a1", selfcheckClaim), met("a2")}
	cases := map[string]exit.Policy{
		"no prover is wired":                         {MaxRedispatch: 2},
		"the seam finds the path unprotected":        {MaxRedispatch: 2, Prover: seamProver(t, func(string) bool { return false }, selfcheckFailure)},
		"the seam finds no floor failure to explain": {MaxRedispatch: 2, Prover: seamProver(t, guards.TouchesProtectedSurface)},
		"the prover proves a claim but not blocked": {MaxRedispatch: 2, Prover: func([]string) exit.Proof {
			return exit.Proof{Proven: []exit.Claim{{Path: selfcheckPath, Defect: selfcheckClaim}}}
		}},
		"the prover returns the empty proof":        {MaxRedispatch: 2, Prover: func([]string) exit.Proof { return exit.Proof{} }},
		"the prover is blocked but proves no claim": {MaxRedispatch: 2, Prover: func([]string) exit.Proof { return exit.Proof{Blocked: true} }},
	}
	for name, policy := range cases {
		t.Run(name, func(t *testing.T) {
			got := exit.Decide(selfcheckRequest(), results, nil, policy)
			if got.Outcome != exit.Retry {
				t.Fatalf("Outcome = %q, want %q (decision %+v)", got.Outcome, exit.Retry, got)
			}
			if want := []string{"a1"}; !reflect.DeepEqual(got.Unmet, want) {
				t.Errorf("Unmet = %v, want %v", got.Unmet, want)
			}
			if len(got.Blocked) != 0 || len(got.Paths) != 0 {
				t.Errorf("an unproven claim must carry no blocked ids or paths: %+v", got)
			}
		})
	}
}

func TestDecide_AClaimDoesNotExcuseAnotherUnmetRequiredAsk(t *testing.T) {
	req := handoff.Request{Asks: []handoff.Ask{{ID: "a1", Required: true}, {ID: "a2", Required: true}}}
	prover := func([]string) exit.Proof {
		return exit.Proof{Blocked: true, Proven: []exit.Claim{{Path: selfcheckPath, Defect: selfcheckClaim}}}
	}
	got := exit.Decide(req, []exit.Result{blockedResult("a1", selfcheckClaim), unmet("a2")}, nil, exit.Policy{MaxRedispatch: 2, Prover: prover})
	if got.Outcome != exit.Retry || !reflect.DeepEqual(got.Unmet, []string{"a1", "a2"}) {
		t.Fatalf("decision = %+v, want retry with both asks unmet: the lane can still work on a2", got)
	}
}

func TestDecide_ABlockedExitWinsOverTheCapAndOverNoGain(t *testing.T) {
	policy := exit.Policy{MaxRedispatch: 1, Prover: seamProver(t, guards.TouchesProtectedSurface, selfcheckFailure)}
	results := []exit.Result{blockedResult("a1", selfcheckClaim), met("a2")}
	for name, history := range map[string][]exit.Round{
		"the redispatch cap is spent": {round("a2")},
		"the round gained nothing":    {round("a2"), round("a2")},
		"the first round with no cap": nil,
	} {
		t.Run(name, func(t *testing.T) {
			if got := exit.Decide(selfcheckRequest(), results, history, policy); got.Outcome != exit.Blocked {
				t.Fatalf("Outcome = %q, want %q", got.Outcome, exit.Blocked)
			}
		})
	}
}

func TestDecide_PassWinsOverAStaleBlockedClaim(t *testing.T) {
	prover := func([]string) exit.Proof {
		t.Fatal("the prover ran although every required ask was met")
		return exit.Proof{}
	}
	results := []exit.Result{met("a1"), met("a2"), blockedResult("o1", selfcheckClaim)}
	req := handoff.Request{Asks: []handoff.Ask{{ID: "a1", Required: true}, {ID: "a2", Required: true}, {ID: "o1"}}}
	if got := exit.Decide(req, results, nil, exit.Policy{MaxRedispatch: 2, Prover: prover}); got.Outcome != exit.Pass {
		t.Fatalf("Outcome = %q, want %q", got.Outcome, exit.Pass)
	}
}

func TestDecide_TheProverReceivesOnlyTheClaimsOfBlockedResults(t *testing.T) {
	var seen [][]string
	prover := func(defects []string) exit.Proof {
		seen = append(seen, defects)
		return exit.Proof{}
	}
	results := []exit.Result{
		blockedResult("a1", selfcheckClaim),
		{Ask: "a2", Status: handoff.StatusUnmet, Resolved: true, Claim: "BLOCKER protected-path x.go an unmet ask carries no claim"},
		blockedResult("a3", ""),
		blockedResult("a4", conflictClaim),
	}
	req := handoff.Request{Asks: []handoff.Ask{{ID: "a1", Required: true}, {ID: "a2", Required: true}, {ID: "a3", Required: true}, {ID: "a4", Required: true}}}
	exit.Decide(req, results, nil, exit.Policy{MaxRedispatch: 2, Prover: prover})
	if want := [][]string{{selfcheckClaim, conflictClaim}}; !reflect.DeepEqual(seen, want) {
		t.Fatalf("prover calls = %q, want %q", seen, want)
	}
}

func TestDecide_TheProverIsNotCalledWithoutAClaim(t *testing.T) {
	prover := func([]string) exit.Proof {
		t.Fatal("the prover ran without a blocked claim")
		return exit.Proof{}
	}
	exit.Decide(selfcheckRequest(), []exit.Result{unmet("a1"), blockedResult("a2", "")}, nil, exit.Policy{MaxRedispatch: 2, Prover: prover})
}

func TestDecide_BlockedIdsAreTheRequestAsksTheProofNames(t *testing.T) {
	prove := func(claims ...exit.Claim) exit.Prover {
		return func([]string) exit.Proof { return exit.Proof{Blocked: true, Proven: claims} }
	}
	req := handoff.Request{Asks: []handoff.Ask{{ID: "a1", Required: true}, {ID: "a2", Required: true}, {ID: "a3", Required: true}}}
	results := []exit.Result{blockedResult("a1", "c1"), blockedResult("a3", "c3"), met("a2")}
	tests := []struct {
		name   string
		claims []exit.Claim
		want   exit.Outcome
		ids    []string
		paths  []string
	}{
		{"each claim blocks the result that carries it, in request order", []exit.Claim{{Defect: "c3", Path: "z.go"}, {Defect: "c1", Path: "a.go"}}, exit.Blocked, []string{"a1", "a3"}, []string{"a.go", "z.go"}},
		{"two claims on one path report the path once", []exit.Claim{{Defect: "c1", Path: "a.go"}, {Defect: "c3", Path: "a.go"}}, exit.Blocked, []string{"a1", "a3"}, []string{"a.go"}},
		{"a conflict names its failing and guard asks", []exit.Claim{{Defect: "c1", Path: "a.go", Failing: "a1", Guard: "a2"}, {Defect: "c3", Path: "a.go"}}, exit.Blocked, []string{"a1", "a2", "a3"}, []string{"a.go"}},
		{"an ask outside the request is not reported", []exit.Claim{{Defect: "c1", Path: "a.go", Failing: "a1", Guard: "ghost"}, {Defect: "c3", Path: "a.go"}}, exit.Blocked, []string{"a1", "a3"}, []string{"a.go"}},
		{"a proven claim that names no ask of the request blocks nothing", []exit.Claim{{Defect: "other", Path: "a.go", Failing: "ghost", Guard: "phantom"}}, exit.Retry, nil, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := exit.Decide(req, results, nil, exit.Policy{MaxRedispatch: 2, Prover: prove(tc.claims...)})
			if got.Outcome != tc.want {
				t.Fatalf("Outcome = %q, want %q (decision %+v)", got.Outcome, tc.want, got)
			}
			if !reflect.DeepEqual(got.Blocked, tc.ids) || !reflect.DeepEqual(got.Paths, tc.paths) {
				t.Errorf("Blocked = %v, Paths = %v; want %v, %v", got.Blocked, got.Paths, tc.ids, tc.paths)
			}
		})
	}
}

func TestDecide_BlockedDoesNotMutateItsInputs(t *testing.T) {
	results := []exit.Result{blockedResult(guardAsk, conflictClaim), unmet(coverAsk)}
	want := append([]exit.Result(nil), results...)
	policy := exit.Policy{MaxRedispatch: 2, Prover: seamProver(t, guards.TouchesProtectedSurface, conflictFailure)}
	first := exit.Decide(conflictRequest(), results, nil, policy)
	if !reflect.DeepEqual(results, want) {
		t.Errorf("results mutated: %v", results)
	}
	for i := 0; i < 10; i++ {
		if got := exit.Decide(conflictRequest(), results, nil, policy); !reflect.DeepEqual(got, first) {
			t.Fatalf("run %d = %+v, want %+v", i, got, first)
		}
	}
}

func TestBlockedWord_MatchesTheHandoffExit(t *testing.T) {
	if string(exit.Blocked) != string(handoff.ExitBlocked) {
		t.Errorf("Blocked = %q, want %q", exit.Blocked, handoff.ExitBlocked)
	}
}
