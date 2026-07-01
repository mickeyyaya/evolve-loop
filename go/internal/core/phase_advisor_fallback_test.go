package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TDD RED (cycle 435, task advisor-cli-fallback-chain / A1): PhaseAdvisor is
// the bridge-backed brain everything else depends on, but advisorLaunch
// (phase_advisor.go:248-288) does a SINGLE p.bridge.Launch and returns an
// error on any failure — it never reads the router profile's cli_fallback
// chain, so a primary CLI hiccup degrades the WHOLE cycle to the static
// spine. Live proof: .evolve/runs/cycle-435/router-launch-error.txt —
// agy-tmux hit exit 81 (quota wall), cli_fallback:["claude-tmux"] was NEVER
// tried. These tests pin the fix: advisorLaunch must walk
// [identity.CLI]+profile.cli_fallback via llmroute.Dispatch, exactly as the
// runner's WS-G1 loop already does for every ordinary phase.
//
// AC map (1:1, R9.3 floor-binding — advisor-cli-fallback-chain is a ## top_n
// task):
//
//	AC1 primary exit=81 → falls back → PRODUCES a plan          → TestAdvisorPlan_FallsBackOnTriggerExit
//	AC2 all-candidates-fail → error → degrade (backstop)        → TestAdvisorPlan_AllCandidatesFailDegradesToStatic
//	AC3 non-trigger exit → no reroute (real FAIL never rerouted)→ TestAdvisorPlan_NonTriggerExitNeverReroutes
//	AC4 single-CLI chain → byte-identical to today               → TestAdvisorPlan_NoFallbackProfileByteIdentical
//	AC5 every candidate launched through the bridge port         → TestAdvisorPlan_FallsBackOnTriggerExit (asserts both calls hit p.bridge.Launch, no direct-exec bypass)
//	AC6 apicover clean on llmroute + core                        → go/acs/cycle435/predicates_test.go (subprocess apicover -enforce; a package-boundary concern, not a core unit test)
//
// Adversarial diversity (SKILL §6): negative = AC3 (a real FAIL must not
// silently reroute — the cheapest no-op fake, "always retry", is exactly what
// this discriminates); edge = AC4 (empty cli_fallback must not change
// behavior); semantic = AC2 vs AC1 (exhausting the chain vs. recovering
// mid-chain are distinct terminal states, not the same code path restated).

// sequencedBridge is a core.Bridge fake that returns one scripted
// (BridgeResponse, error) pair per call, in order (clamped to the last entry
// once exhausted). It records every BridgeRequest it received so a test can
// assert BOTH which CLIs were tried, in what order, AND that every attempt
// flowed through the bridge port (no direct-exec bypass).
type sequencedBridge struct {
	seq   []sequencedResp
	calls []BridgeRequest
}

type sequencedResp struct {
	resp BridgeResponse
	err  error
}

func (s *sequencedBridge) Launch(_ context.Context, req BridgeRequest) (BridgeResponse, error) {
	s.calls = append(s.calls, req)
	i := len(s.calls) - 1
	if i >= len(s.seq) {
		i = len(s.seq) - 1
	}
	return s.seq[i].resp, s.seq[i].err
}

func (s *sequencedBridge) Probe(_ context.Context) (BridgeProbe, error) { return BridgeProbe{}, nil }

func (s *sequencedBridge) calledCLIs() []string {
	out := make([]string, len(s.calls))
	for i, c := range s.calls {
		out[i] = c.CLI
	}
	return out
}

// writeRouterProfile drops a .evolve/profiles/router.json with a cli_fallback
// chain under a fresh temp project root, and returns that root for
// RouteInput.ProjectRoot (mirrors advisorLaunch's own derivation:
// filepath.Join(in.ProjectRoot, ".evolve", "profiles", "router.json")).
func writeRouterProfile(t *testing.T, primaryCLI string, fallback []string, onExit []int) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir profiles dir: %v", err)
	}
	body := `{"name":"router","cli":"` + primaryCLI + `"`
	if len(fallback) > 0 {
		body += `,"cli_fallback":["` + strings.Join(fallback, `","`) + `"]`
	}
	if len(onExit) > 0 {
		onExitStrs := make([]string, len(onExit))
		for i, n := range onExit {
			onExitStrs[i] = strconv.Itoa(n)
		}
		body += `,"cli_fallback_on_exit":[` + strings.Join(onExitStrs, ",") + `]`
	}
	body += `}`
	if err := os.WriteFile(filepath.Join(dir, "router.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write router.json: %v", err)
	}
	return root
}

// TestAdvisorPlan_FallsBackOnTriggerExit (AC1 + AC5, positive): the live
// cycle-435 failure replayed — primary agy-tmux exits 81 (a trigger), the
// declared fallback claude-tmux succeeds. Plan() must return a real plan (not
// an error the caller degrades on), and BOTH candidates must have flowed
// through p.bridge.Launch (AC5: no direct-exec bypass).
func TestAdvisorPlan_FallsBackOnTriggerExit(t *testing.T) {
	root := writeRouterProfile(t, "agy-tmux", []string{"claude-tmux"}, []int{81})
	sb := &sequencedBridge{seq: []sequencedResp{
		{resp: BridgeResponse{ExitCode: 81}, err: errors.New("bridge: launch exit=81 (Individual quota reached)")},
		{resp: BridgeResponse{ExitCode: 0, Stdout: `[{"phase":"scout","run":true,"justification":"x"}]`}},
	}}
	adv := NewPhaseAdvisor(sb, WithProposerCLI("agy-tmux"))
	in := baseRouteInput()
	in.ProjectRoot = root
	in.Workspace = t.TempDir()

	plan, err := adv.Plan(in)

	if err != nil {
		t.Fatalf("Plan: expected fallback to produce a plan, got err=%v (cycle-435 regression: advisor degrades to static instead of trying cli_fallback)", err)
	}
	if plan == nil || len(plan.Entries) == 0 {
		t.Fatalf("Plan: got empty plan %+v, want a real phase plan from the fallback CLI", plan)
	}
	want := []string{"agy-tmux", "claude-tmux"}
	if got := sb.calledCLIs(); !equalStrings(got, want) {
		t.Errorf("Plan: bridge.Launch called with CLIs %v, want %v (both candidates through the bridge port, in order)", got, want)
	}
}

// TestAdvisorPlan_AllCandidatesFailDegradesToStatic (AC2): every candidate in
// the chain exhausts a trigger exit — Plan() must still return an error so
// the orchestrator's existing degrade-to-static backstop fires (this is the
// LAST-resort path the fix must preserve, not remove).
func TestAdvisorPlan_AllCandidatesFailDegradesToStatic(t *testing.T) {
	root := writeRouterProfile(t, "agy-tmux", []string{"claude-tmux"}, []int{81})
	sb := &sequencedBridge{seq: []sequencedResp{
		{resp: BridgeResponse{ExitCode: 81}, err: errors.New("agy-tmux: exit=81")},
		{resp: BridgeResponse{ExitCode: 81}, err: errors.New("claude-tmux: exit=81 too")},
	}}
	adv := NewPhaseAdvisor(sb, WithProposerCLI("agy-tmux"))
	in := baseRouteInput()
	in.ProjectRoot = root
	in.Workspace = t.TempDir()

	if _, err := adv.Plan(in); err == nil {
		t.Fatalf("Plan: expected an error when every chain candidate is exhausted (degrade-to-static backstop), got nil")
	}
	want := []string{"agy-tmux", "claude-tmux"}
	if got := sb.calledCLIs(); !equalStrings(got, want) {
		t.Errorf("Plan: bridge.Launch called with CLIs %v, want %v (chain fully exhausted)", got, want)
	}
}

// TestAdvisorPlan_NonTriggerExitNeverReroutes (AC3, negative — the strongest
// anti-no-op signal): a non-trigger exit is a REAL failure (e.g. the model
// declined, or a genuine content error), not a dispatch stall. The chain must
// NEVER advance to the fallback on it — a naive "always retry the chain"
// implementation would wrongly pass this AC1 but fail here.
func TestAdvisorPlan_NonTriggerExitNeverReroutes(t *testing.T) {
	root := writeRouterProfile(t, "agy-tmux", []string{"claude-tmux"}, []int{81})
	sb := &sequencedBridge{seq: []sequencedResp{
		{resp: BridgeResponse{ExitCode: 2}, err: errors.New("agy-tmux: real failure, exit=2 (not a trigger)")},
	}}
	adv := NewPhaseAdvisor(sb, WithProposerCLI("agy-tmux"))
	in := baseRouteInput()
	in.ProjectRoot = root
	in.Workspace = t.TempDir()

	if _, err := adv.Plan(in); err == nil {
		t.Fatalf("Plan: expected the non-trigger error to surface, got nil")
	}
	if len(sb.calls) != 1 {
		t.Fatalf("Plan: bridge.Launch called %d time(s) %v, want exactly 1 — a non-trigger exit must never reroute to the fallback CLI", len(sb.calls), sb.calledCLIs())
	}
}

// TestAdvisorPlan_NoFallbackProfileByteIdentical (AC4, edge case): a router
// profile with NO cli_fallback declared must dispatch exactly once, exactly
// like today's un-chained advisorLaunch — the fix is additive, not a
// behavior change for the common (no-fallback-configured) case.
func TestAdvisorPlan_NoFallbackProfileByteIdentical(t *testing.T) {
	root := writeRouterProfile(t, "claude-tmux", nil, nil)
	sb := &sequencedBridge{seq: []sequencedResp{
		{resp: BridgeResponse{ExitCode: 0, Stdout: `[{"phase":"scout","run":true,"justification":"x"}]`}},
	}}
	adv := NewPhaseAdvisor(sb, WithProposerCLI("claude-tmux"))
	in := baseRouteInput()
	in.ProjectRoot = root
	in.Workspace = t.TempDir()

	plan, err := adv.Plan(in)

	if err != nil {
		t.Fatalf("Plan: unexpected error with no fallback configured: %v", err)
	}
	if plan == nil || len(plan.Entries) == 0 {
		t.Fatalf("Plan: got empty plan %+v", plan)
	}
	if len(sb.calls) != 1 {
		t.Errorf("Plan: bridge.Launch called %d time(s), want exactly 1 (byte-identical single-CLI chain)", len(sb.calls))
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
