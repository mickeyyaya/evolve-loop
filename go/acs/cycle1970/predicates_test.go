//go:build acs

package cycle1970

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
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
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

func metResult(ask string) exit.Result {
	return exit.Result{Ask: ask, Status: handoff.StatusMet, Resolved: true}
}

func conflictRequest() handoff.Request {
	return handoff.Request{Pair: "tdd-build", Asks: []handoff.Ask{{ID: coverAsk, Required: true}, {ID: guardAsk, Required: true}}}
}

func selfcheckRequest() handoff.Request {
	return handoff.Request{Pair: "tdd-build", Asks: []handoff.Ask{{ID: "a1", Required: true}, {ID: "a2", Required: true}}}
}

func TestC1970_001_Cycle1950AsksReturnBlockedWithBothIds(t *testing.T) {
	policy := exit.Policy{MaxRedispatch: 2, Prover: seamProver(t, guards.TouchesProtectedSurface, conflictFailure)}
	results := []exit.Result{blockedResult(coverAsk, conflictClaim), metResult(guardAsk)}

	got := exit.Decide(conflictRequest(), results, nil, policy)

	if got.Outcome != exit.Blocked {
		t.Fatalf("RED: Outcome = %q, want %q (decision %+v)", got.Outcome, exit.Blocked, got)
	}
	if want := []string{coverAsk, guardAsk}; !reflect.DeepEqual(got.Blocked, want) {
		t.Errorf("RED: Blocked = %v, want both ids %v", got.Blocked, want)
	}
}

func TestC1970_002_AskNeedingAProtectedPathBlocksWithThePath(t *testing.T) {
	if !guards.TouchesProtectedSurface(selfcheckPath) {
		t.Fatalf("RED: %s must be on the protected-surface manifest for this predicate to mean anything", selfcheckPath)
	}
	policy := exit.Policy{MaxRedispatch: 2, Prover: seamProver(t, guards.TouchesProtectedSurface, selfcheckFailure)}

	got := exit.Decide(selfcheckRequest(), []exit.Result{blockedResult("a1", selfcheckClaim), metResult("a2")}, nil, policy)

	if got.Outcome != exit.Blocked {
		t.Fatalf("RED: Outcome = %q, want %q (decision %+v)", got.Outcome, exit.Blocked, got)
	}
	if want := []string{selfcheckPath}; !reflect.DeepEqual(got.Paths, want) {
		t.Errorf("RED: Paths = %v, want %v", got.Paths, want)
	}
	if want := []string{"a1"}; !reflect.DeepEqual(got.Blocked, want) {
		t.Errorf("RED: Blocked = %v, want %v", got.Blocked, want)
	}
}

func TestC1970_003_ABlockedClaimWithNoProofIsUnmetNotBlocked(t *testing.T) {
	results := []exit.Result{blockedResult("a1", selfcheckClaim), metResult("a2")}
	cases := map[string]exit.Policy{
		"no prover wired":                    {MaxRedispatch: 2},
		"seam finds the path unprotected":    {MaxRedispatch: 2, Prover: seamProver(t, func(string) bool { return false }, selfcheckFailure)},
		"seam has no floor failure to match": {MaxRedispatch: 2, Prover: seamProver(t, guards.TouchesProtectedSurface)},
	}
	for name, policy := range cases {
		got := exit.Decide(selfcheckRequest(), results, nil, policy)
		if got.Outcome == exit.Blocked {
			t.Errorf("RED: %s: an unproven claim blocked the loop: %+v", name, got)
		}
		if got.Outcome != exit.Retry || !reflect.DeepEqual(got.Unmet, []string{"a1"}) {
			t.Errorf("RED: %s: decision = %+v, want retry with a1 unmet", name, got)
		}
	}
}

func TestC1970_004_TheProofReusesTheBlockerExitSeamAndAddsNoMatcher(t *testing.T) {
	results := []exit.Result{blockedResult("a1", selfcheckClaim), metResult("a2")}
	protected := exit.Decide(selfcheckRequest(), results, nil,
		exit.Policy{MaxRedispatch: 2, Prover: seamProver(t, func(p string) bool { return p == selfcheckPath }, selfcheckFailure)})
	open := exit.Decide(selfcheckRequest(), results, nil,
		exit.Policy{MaxRedispatch: 2, Prover: seamProver(t, func(string) bool { return false }, selfcheckFailure)})
	if protected.Outcome != exit.Blocked || open.Outcome == exit.Blocked {
		t.Fatalf("RED: the verdict must follow the seam's protected-surface func alone: protected=%+v open=%+v", protected, open)
	}

	out, _, code, err := acsassert.SubprocessOutput("go", "list", "-deps", "github.com/mickeyyaya/evolve-loop/go/internal/handoff/exit")
	if err != nil || code != 0 {
		t.Fatalf("go list exit=%d: %v", code, err)
	}
	for _, dep := range strings.Fields(out) {
		if strings.HasSuffix(dep, "/go/internal/core") || strings.HasSuffix(dep, "/go/internal/guards") {
			t.Errorf("RED: exit imports %s: core will import exit for the handoff reviewer, so the seam must be injected, not imported", dep)
		}
	}
}
