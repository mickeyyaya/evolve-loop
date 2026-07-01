package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Test Amplification (cycle 435, black-box adversarial pass on top of the
// TDD-authored phase_advisor_fallback_test.go). Designed from the contract
// (advisorLaunch must walk [identity.CLI]+profile.cli_fallback via
// llmroute.Dispatch, per test-report.md/build-report.md) and the existing
// sequencedBridge/writeRouterProfile fixtures, without reading
// phase_advisor.go's advisorLaunch implementation. Covers gaps the original
// 4-test suite left open: the full default trigger-code set, chains longer
// than 2, an explicit-empty (vs omitted) cli_fallback array, and malformed
// or missing router-profile input (the fail-open path build-report.md
// documents for loadDispatchProfile).

// TestAdvisorPlan_AllDefaultTriggerExitCodesFallBack (basic, table-driven):
// the cycle-435 goal names five standard triggers [80 81 85 124 127]; the
// TDD suite only proves 81 end-to-end at the advisor layer. This closes the
// other four so the advisor's own resilience -- the dispatch everything else
// depends on -- isn't accidentally narrower than the runner's.
func TestAdvisorPlan_AllDefaultTriggerExitCodesFallBack(t *testing.T) {
	for _, code := range []int{80, 81, 85, 124, 127} {
		code := code
		t.Run(fmt.Sprintf("exit=%d", code), func(t *testing.T) {
			root := writeRouterProfile(t, "agy-tmux", []string{"claude-tmux"}, []int{80, 81, 85, 124, 127})
			sb := &sequencedBridge{seq: []sequencedResp{
				{resp: BridgeResponse{ExitCode: code}, err: fmt.Errorf("agy-tmux: exit=%d", code)},
				{resp: BridgeResponse{ExitCode: 0, Stdout: `[{"phase":"scout","run":true,"justification":"x"}]`}},
			}}
			adv := NewPhaseAdvisor(sb, WithProposerCLI("agy-tmux"))
			in := baseRouteInput()
			in.ProjectRoot = root
			in.Workspace = t.TempDir()

			plan, err := adv.Plan(in)

			if err != nil {
				t.Fatalf("Plan: exit=%d expected fallback to produce a plan, got err=%v", code, err)
			}
			if plan == nil || len(plan.Entries) == 0 {
				t.Fatalf("Plan: exit=%d got empty plan %+v", code, plan)
			}
			want := []string{"agy-tmux", "claude-tmux"}
			if got := sb.calledCLIs(); !equalStrings(got, want) {
				t.Errorf("Plan: exit=%d bridge.Launch called with CLIs %v, want %v", code, got, want)
			}
		})
	}
}

// TestAdvisorPlan_ThreeCLIChainFallsBackAcrossTwoHops (edge: chain length
// != 2): the TDD suite only ever scripts a 1-fallback chain. A real
// cli_fallback list can have multiple entries -- the advisor must walk past
// TWO failing candidates before reaching the one that succeeds, not just
// "retry once".
func TestAdvisorPlan_ThreeCLIChainFallsBackAcrossTwoHops(t *testing.T) {
	root := writeRouterProfile(t, "agy-tmux", []string{"codex-tmux", "claude-tmux"}, []int{81})
	sb := &sequencedBridge{seq: []sequencedResp{
		{resp: BridgeResponse{ExitCode: 81}, err: errors.New("agy-tmux: exit=81")},
		{resp: BridgeResponse{ExitCode: 81}, err: errors.New("codex-tmux: exit=81")},
		{resp: BridgeResponse{ExitCode: 0, Stdout: `[{"phase":"scout","run":true,"justification":"x"}]`}},
	}}
	adv := NewPhaseAdvisor(sb, WithProposerCLI("agy-tmux"))
	in := baseRouteInput()
	in.ProjectRoot = root
	in.Workspace = t.TempDir()

	plan, err := adv.Plan(in)

	if err != nil {
		t.Fatalf("Plan: expected the third candidate to succeed, got err=%v", err)
	}
	if plan == nil || len(plan.Entries) == 0 {
		t.Fatalf("Plan: got empty plan %+v", plan)
	}
	want := []string{"agy-tmux", "codex-tmux", "claude-tmux"}
	if got := sb.calledCLIs(); !equalStrings(got, want) {
		t.Errorf("Plan: bridge.Launch called with CLIs %v, want %v (both intermediate hops tried in order)", got, want)
	}
}

// TestAdvisorPlan_ExplicitEmptyCLIFallbackArrayByteIdentical (edge: explicit
// empty JSON array, not an omitted field): "cli_fallback":[] is a distinct
// wire shape from a profile that never mentions cli_fallback at all (the
// case TestAdvisorPlan_NoFallbackProfileByteIdentical already covers). Both
// must behave identically -- exactly one dispatch, no phantom fallback.
func TestAdvisorPlan_ExplicitEmptyCLIFallbackArrayByteIdentical(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir profiles dir: %v", err)
	}
	body := `{"name":"router","cli":"claude-tmux","cli_fallback":[]}`
	if err := os.WriteFile(filepath.Join(dir, "router.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write router.json: %v", err)
	}
	sb := &sequencedBridge{seq: []sequencedResp{
		{resp: BridgeResponse{ExitCode: 0, Stdout: `[{"phase":"scout","run":true,"justification":"x"}]`}},
	}}
	adv := NewPhaseAdvisor(sb, WithProposerCLI("claude-tmux"))
	in := baseRouteInput()
	in.ProjectRoot = root
	in.Workspace = t.TempDir()

	plan, err := adv.Plan(in)

	if err != nil {
		t.Fatalf("Plan: unexpected error with an explicit-empty cli_fallback array: %v", err)
	}
	if plan == nil || len(plan.Entries) == 0 {
		t.Fatalf("Plan: got empty plan %+v", plan)
	}
	if len(sb.calls) != 1 {
		t.Errorf("Plan: bridge.Launch called %d time(s), want exactly 1 (explicit-empty array must behave like no fallback at all)", len(sb.calls))
	}
}

// TestAdvisorPlan_MalformedRouterProfileDegradesToSingleDispatch
// (adversarial: corrupted config input): build-report.md documents
// loadDispatchProfile as fail-open (nil) on any read/parse error, so a
// malformed router.json degrades to a single-candidate chain rather than a
// hard dispatch failure. This pins that documented contract at the black-box
// Plan() level: invalid JSON on disk must never panic and must never turn
// into an unconditional error before the primary CLI is even tried.
func TestAdvisorPlan_MalformedRouterProfileDegradesToSingleDispatch(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir profiles dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "router.json"), []byte(`{not valid json`), 0o644); err != nil {
		t.Fatalf("write malformed router.json: %v", err)
	}
	sb := &sequencedBridge{seq: []sequencedResp{
		{resp: BridgeResponse{ExitCode: 0, Stdout: `[{"phase":"scout","run":true,"justification":"x"}]`}},
	}}
	adv := NewPhaseAdvisor(sb, WithProposerCLI("claude-tmux"))
	in := baseRouteInput()
	in.ProjectRoot = root
	in.Workspace = t.TempDir()

	plan, err := adv.Plan(in)

	if err != nil {
		t.Fatalf("Plan: a malformed router.json must degrade to a single-candidate dispatch, not error out before trying the primary CLI: %v", err)
	}
	if plan == nil || len(plan.Entries) == 0 {
		t.Fatalf("Plan: got empty plan %+v", plan)
	}
	if len(sb.calls) != 1 {
		t.Errorf("Plan: bridge.Launch called %d time(s), want exactly 1 (malformed profile parse failure must not fabricate a fallback chain)", len(sb.calls))
	}
}

// TestAdvisorPlan_MissingRouterProfileDegradesToSingleDispatch (edge: no
// profile file on disk at all, not even an empty .evolve/profiles dir):
// mirrors the malformed-JSON case above for the "file doesn't exist" flavor
// of the same fail-open contract.
func TestAdvisorPlan_MissingRouterProfileDegradesToSingleDispatch(t *testing.T) {
	root := t.TempDir() // no .evolve/profiles/router.json written at all
	sb := &sequencedBridge{seq: []sequencedResp{
		{resp: BridgeResponse{ExitCode: 0, Stdout: `[{"phase":"scout","run":true,"justification":"x"}]`}},
	}}
	adv := NewPhaseAdvisor(sb, WithProposerCLI("claude-tmux"))
	in := baseRouteInput()
	in.ProjectRoot = root
	in.Workspace = t.TempDir()

	plan, err := adv.Plan(in)

	if err != nil {
		t.Fatalf("Plan: a missing router.json must degrade to a single-candidate dispatch: %v", err)
	}
	if plan == nil || len(plan.Entries) == 0 {
		t.Fatalf("Plan: got empty plan %+v", plan)
	}
	if len(sb.calls) != 1 {
		t.Errorf("Plan: bridge.Launch called %d time(s), want exactly 1 (no profile on disk must not fabricate a fallback chain)", len(sb.calls))
	}
}
