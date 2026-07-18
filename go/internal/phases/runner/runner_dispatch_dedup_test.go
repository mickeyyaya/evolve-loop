package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TDD RED (cycle 435, task runner-dispatch-dedup / A2, dependsOn
// advisor-cli-fallback-chain / A1): the runner's WS-G1 fallback walk
// (runner.go:485-537, the inline `for i, candidateCLI := range
// plan.Candidates` loop) and the llmroute.Dispatch extracted for the advisor
// are the SAME "advance the CLI chain on a trigger exit" algorithm.
// [[never_duplicate_centralize_via_design_patterns]] is the user's HIGHEST
// rule: once Dispatch exists (go/internal/llmroute/dispatch_test.go), the
// runner's hand-rolled copy must be deleted and replaced with a call to it —
// coexistence of both is the violation this task closes.
//
// AC map (1:1, R9.3 floor-binding — runner-dispatch-dedup is a ## top_n task):
//
//	AC1 runner fallback/clihealth tests green under -race (regression)  → pre-existing GREEN: runner_fallback_test.go, runner_clihealth_test.go, runner_driver_bench_test.go (re-run post-refactor by go/acs/cycle435/predicates_test.go)
//	AC2 inline loop removed                                 (negative)  → TestRunnerDispatch_NoInlineFallbackLoop (RED today)
//	AC3 delegates to llmroute.Dispatch, chain still works    (positive) → TestRunnerDispatch_CallsLlmrouteDispatch (RED today)
//	AC4 non-trigger exit still never reroutes (regression)              → pre-existing GREEN: TestRun_NoFallbackOnNonTriggerExit (runner_fallback_test.go)
//	AC5 exit-85 bench side effect preserved (regression)                → pre-existing GREEN: TestRun_Exit85RateLimitBenchesFamily (runner_clihealth_test.go)
//
// Adversarial diversity (SKILL §6): AC2 is the negative (a refactor that
// keeps BOTH the old loop and a new Dispatch call would pass every OTHER
// AC — this is the one that catches "added the call but never deleted the
// duplicate"). TestRunnerDispatch_CallsLlmrouteDispatch pairs its source
// check with an ACTUAL r.Run() fallback (behavioral), so a dead reference to
// "llmroute.Dispatch" in a comment can't game it — the predicate-quality
// rule bars a source-text-only assertion from being the SOLE evidence.

// readRunnerSource returns runner.go's contents (sibling of this test file —
// runtime.Caller(0) locates the package dir so the check works regardless of
// the caller's cwd, matching the seed_phase_e2e_test.go convention).
func readRunnerSource(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	src, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "runner.go"))
	if err != nil {
		t.Fatalf("read runner.go: %v", err)
	}
	return string(src)
}

// TestRunnerDispatch_NoInlineFallbackLoop (AC2, negative): the hand-rolled
// WS-G1 loop must be gone — a coexisting duplicate is the exact violation
// this task closes, not just an untidy leftover.
func TestRunnerDispatch_NoInlineFallbackLoop(t *testing.T) {
	src := readRunnerSource(t)
	if strings.Contains(src, "for i, candidateCLI := range plan.Candidates") {
		t.Errorf("runner.go still contains the inline WS-G1 fallback loop — delegate to llmroute.Dispatch instead ([[never_duplicate_centralize_via_design_patterns]])")
	}
}

// TestRunnerDispatch_CallsLlmrouteDispatch (AC3, positive): runner.go must
// call llmroute.Dispatch, AND the fallback chain must still actually work
// end-to-end through r.Run() after the extraction (the scriptedBridge/
// writeFallbackProfile fixtures are the same ones
// TestRun_FallbackOnBootTimeout_PrimaryFailsSecondarySucceeds already uses —
// reused, not duplicated).
func TestRunnerDispatch_CallsLlmrouteDispatch(t *testing.T) {
	src := readRunnerSource(t)
	// WS-876: the runner dispatches through llmroute.DispatchTiered — the tier×CLI
	// fallback walk lives in the llmroute PACKAGE, not hand-rolled in the runner.
	// (DispatchTiered re-uses the same trigger/break CLI-walk semantics as
	// Dispatch, but re-implements the inner loop rather than calling Dispatch
	// per-tier, because the all-85 tier step-down decision needs each attempt's
	// exit code — which Dispatch's return type does not expose.)
	if !strings.Contains(src, "llmroute.DispatchTiered(") {
		t.Errorf("runner.go does not call llmroute.DispatchTiered — the tier+CLI fallback walk must live in the shared llmroute package, not a hand-rolled copy in the runner")
	}

	hooks := &fakeHooks{
		phase: "auditor", agent: "evolve-auditor", model: "sonnet",
		prompt: "x", verdict: core.VerdictPASS, nextPhase: "ship",
	}
	sb := &scriptedBridge{
		responses: map[string]scriptedResp{
			"codex-tmux": {
				resp: core.BridgeResponse{ExitCode: 80, Stderr: "REPL boot timeout"},
				err:  errors.New("bridge: launch exit=80"),
			},
			"claude-tmux": {}, // empty = success
		},
	}
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	r := New(Options{
		Hooks:   hooks,
		Bridge:  sb,
		Prompts: fakePromptsFS("evolve-auditor", "x"),
	})

	if _, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root,
		Workspace:   t.TempDir(),
	}); err != nil {
		t.Fatalf("Run: expected the fallback to still succeed post-dedup, got err=%v", err)
	}
	want := []string{"codex-tmux", "claude-tmux"}
	if len(sb.calls) != len(want) || sb.calls[0] != want[0] || sb.calls[1] != want[1] {
		t.Errorf("Run: dispatched %v, want %v (chain behavior preserved through the Dispatch delegation)", sb.calls, want)
	}
}
