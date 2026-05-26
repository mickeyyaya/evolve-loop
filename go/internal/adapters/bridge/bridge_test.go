// Package bridge tests the Go-only adapter: it must validate the request,
// inject the resolved interactive policy into the prompt body, and
// delegate to the in-process Engine. The Engine's own launch/probe
// behavior is covered by internal/bridge; here a fake core.Bridge stands
// in so the assertions are about the adapter's two jobs (policy + delegation).
package bridge

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// fakeEngine is the injected core.Bridge. It records the request it
// received (so tests can assert the policy-injected prompt) and returns
// scripted results.
type fakeEngine struct {
	gotReq core.BridgeRequest
	resp   core.BridgeResponse
	err    error
	probe  core.BridgeProbe
}

func (f *fakeEngine) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	f.gotReq = req
	return f.resp, f.err
}

func (f *fakeEngine) Probe(context.Context) (core.BridgeProbe, error) {
	return f.probe, nil
}

// withEngine builds an Adapter wired to the given fake engine.
func withEngine(fe *fakeEngine) *Adapter {
	a := New()
	a.engineFactory = func(map[string]string) core.Bridge { return fe }
	return a
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// TestLaunch_RequiredFieldValidation — the adapter rejects requests
// missing any of the four required fields before touching the engine.
func TestLaunch_RequiredFieldValidation(t *testing.T) {
	full := core.BridgeRequest{CLI: "claude-tmux", Profile: "/p", Workspace: "/ws", ArtifactPath: "/a.md"}
	cases := map[string]func(core.BridgeRequest) core.BridgeRequest{
		"missing CLI":          func(r core.BridgeRequest) core.BridgeRequest { r.CLI = ""; return r },
		"missing Profile":      func(r core.BridgeRequest) core.BridgeRequest { r.Profile = ""; return r },
		"missing Workspace":    func(r core.BridgeRequest) core.BridgeRequest { r.Workspace = ""; return r },
		"missing ArtifactPath": func(r core.BridgeRequest) core.BridgeRequest { r.ArtifactPath = ""; return r },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			fe := &fakeEngine{}
			_, err := withEngine(fe).Launch(context.Background(), mutate(full))
			if err == nil {
				t.Fatalf("%s: want validation error, got nil", name)
			}
			if !strings.HasPrefix(err.Error(), "bridge: ") {
				t.Errorf("error should be a bridge: error; got %q", err.Error())
			}
			if fe.gotReq.CLI != "" {
				t.Errorf("engine must not be called when validation fails")
			}
		})
	}
}

// TestLaunch_DelegatesToEngine — a valid request reaches the engine and
// its response is returned verbatim.
func TestLaunch_DelegatesToEngine(t *testing.T) {
	fe := &fakeEngine{resp: core.BridgeResponse{ExitCode: 0, Stdout: "ARTIFACT"}}
	resp, err := withEngine(fe).Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: "/p", Model: "sonnet",
		Prompt: "body", Workspace: t.TempDir(), ArtifactPath: "/a.md", Agent: "scout",
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if resp.Stdout != "ARTIFACT" {
		t.Errorf("response not passed through; got %q", resp.Stdout)
	}
	if fe.gotReq.CLI != "claude-tmux" || fe.gotReq.Model != "sonnet" {
		t.Errorf("request not forwarded intact: %+v", fe.gotReq)
	}
}

// TestProbe_DelegatesToEngine — Probe forwards the engine's probe.
func TestProbe_DelegatesToEngine(t *testing.T) {
	fe := &fakeEngine{probe: core.BridgeProbe{Version: "darwin", CLIs: map[string]string{"claude-tmux": "full"}}}
	got, err := withEngine(fe).Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if got.Version != "darwin" || got.CLIs["claude-tmux"] != "full" {
		t.Errorf("probe not delegated: %+v", got)
	}
}

// TestNewDefault_ReturnsUsableAdapter — the production constructor wires a
// real engine factory (projectRoot reserved/unused).
func TestNewDefault_ReturnsUsableAdapter(t *testing.T) {
	a := NewDefault("/any/project/root")
	if a == nil || a.engineFactory == nil {
		t.Fatal("NewDefault must wire a non-nil engine factory")
	}
}

// --- interactive-policy injection (v12.1 Capability 3) ---

// runOnce launches the adapter against a fake engine and returns the
// prompt body the engine actually received (after policy injection).
func runOnce(t *testing.T, agent, prompt string, env map[string]string) string {
	t.Helper()
	fe := &fakeEngine{}
	_, err := withEngine(fe).Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: "/p", Model: "auto",
		Prompt: prompt, Workspace: t.TempDir(), ArtifactPath: "/a.md", Agent: agent, Env: env,
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	return fe.gotReq.Prompt
}

func TestLaunch_DefaultPolicy_InjectsRecommendedOrFirstPrefix(t *testing.T) {
	t.Setenv("EVOLVE_INTERACTIVE_POLICY", "")
	t.Setenv("EVOLVE_SCOUT_INTERACTIVE_POLICY", "")
	body := runOnce(t, "scout", "scout prompt body", nil)
	if !strings.HasPrefix(body, "## Subagent Interactive Policy (recommended_or_first)") {
		t.Errorf("prompt missing recommended-or-first prefix; got first 80 chars: %q", truncate(body, 80))
	}
	if !strings.Contains(body, "scout prompt body") {
		t.Errorf("prompt missing original body after prefix")
	}
}

func TestLaunch_NoPolicyPrefix_WhenEscalateExplicit(t *testing.T) {
	t.Setenv("EVOLVE_INTERACTIVE_POLICY", PolicyEscalate)
	body := runOnce(t, "builder", "builder body", nil)
	if strings.Contains(body, "Subagent Interactive Policy") {
		t.Errorf("escalate policy must not inject a block; got first 120 chars: %q", truncate(body, 120))
	}
	if body != "builder body" {
		t.Errorf("body=%q, want %q (no prefix)", body, "builder body")
	}
}

func TestLaunch_AutoYesPolicy_InjectsAlternatePrefix(t *testing.T) {
	t.Setenv("EVOLVE_INTERACTIVE_POLICY", PolicyAutoYes)
	body := runOnce(t, "auditor", "auditor body", nil)
	if !strings.HasPrefix(body, "## Subagent Interactive Policy (auto_yes)") {
		t.Errorf("auto_yes policy must inject auto_yes block; got first 80 chars: %q", truncate(body, 80))
	}
	if !strings.Contains(body, "auditor body") {
		t.Errorf("prompt missing original body after prefix")
	}
}

func TestResolvePolicy_PrecedenceOrder(t *testing.T) {
	t.Setenv("EVOLVE_INTERACTIVE_POLICY", PolicyAutoYes)
	t.Setenv("EVOLVE_SCOUT_INTERACTIVE_POLICY", PolicyEscalate)

	if got := resolvePolicy("scout", nil); got != PolicyEscalate {
		t.Errorf("per-agent env should win: got=%q want=%q", got, PolicyEscalate)
	}
	if got := resolvePolicy("scout", map[string]string{"EVOLVE_SCOUT_INTERACTIVE_POLICY": PolicyRecommendedOrFirst}); got != PolicyRecommendedOrFirst {
		t.Errorf("reqEnv per-agent should win: got=%q want=%q", got, PolicyRecommendedOrFirst)
	}
	if got := resolvePolicy("builder", map[string]string{"EVOLVE_INTERACTIVE_POLICY": PolicyRecommendedOrFirst}); got != PolicyRecommendedOrFirst {
		t.Errorf("reqEnv global should win: got=%q want=%q", got, PolicyRecommendedOrFirst)
	}
	if got := resolvePolicy("builder", nil); got != PolicyAutoYes {
		t.Errorf("global env should be used: got=%q want=%q", got, PolicyAutoYes)
	}
}

func TestResolvePolicy_DefaultWhenAllUnset(t *testing.T) {
	t.Setenv("EVOLVE_INTERACTIVE_POLICY", "")
	t.Setenv("EVOLVE_BUILDER_INTERACTIVE_POLICY", "")
	if got := resolvePolicy("builder", nil); got != PolicyRecommendedOrFirst {
		t.Errorf("default policy got=%q want=%q", got, PolicyRecommendedOrFirst)
	}
}

func TestResolvePolicy_EmptyAgent_FallsThroughToGlobal(t *testing.T) {
	t.Setenv("EVOLVE_INTERACTIVE_POLICY", PolicyAutoYes)
	if got := resolvePolicy("", nil); got != PolicyAutoYes {
		t.Errorf("empty agent should fall through to global: got=%q want=%q", got, PolicyAutoYes)
	}
}

func TestPerAgentPolicyEnv_HyphenToUnderscore(t *testing.T) {
	cases := map[string]string{
		"scout":         "EVOLVE_SCOUT_INTERACTIVE_POLICY",
		"builder":       "EVOLVE_BUILDER_INTERACTIVE_POLICY",
		"tdd-engineer":  "EVOLVE_TDD_ENGINEER_INTERACTIVE_POLICY",
		"plan-reviewer": "EVOLVE_PLAN_REVIEWER_INTERACTIVE_POLICY",
	}
	for agent, want := range cases {
		if got := perAgentPolicyEnv(agent); got != want {
			t.Errorf("perAgentPolicyEnv(%q)=%q, want %q", agent, got, want)
		}
	}
}

func TestInjectPolicyPrefix_UnknownValueDefaultsToRecommendedOrFirst(t *testing.T) {
	got := injectPolicyPrefix("body", "no-such-policy")
	if !strings.HasPrefix(got, "## Subagent Interactive Policy (recommended_or_first)") {
		t.Errorf("unknown policy should default to recommended_or_first; got first 80 chars: %q", truncate(got, 80))
	}
}

func TestInjectPolicyPrefix_EscalateReturnsBodyUnchanged(t *testing.T) {
	if got := injectPolicyPrefix("body", PolicyEscalate); got != "body" {
		t.Errorf("escalate should pass through unchanged; got=%q", got)
	}
}

func TestLaunch_PerAgentEnvOverrides_GlobalDefault(t *testing.T) {
	t.Setenv("EVOLVE_INTERACTIVE_POLICY", "")
	t.Setenv("EVOLVE_SCOUT_INTERACTIVE_POLICY", PolicyEscalate)

	if scoutBody := runOnce(t, "scout", "scout body", nil); scoutBody != "scout body" {
		t.Errorf("scout per-agent escalate not honored; got %q", truncate(scoutBody, 120))
	}
	if builderBody := runOnce(t, "builder", "builder body", nil); !strings.HasPrefix(builderBody, "## Subagent Interactive Policy (recommended_or_first)") {
		t.Errorf("builder should still get default block; got first 80 chars: %q", truncate(builderBody, 80))
	}
}

func TestLaunch_ReqEnvOverridesProcessEnv(t *testing.T) {
	t.Setenv("EVOLVE_INTERACTIVE_POLICY", PolicyAutoYes)
	body := runOnce(t, "builder", "builder body", map[string]string{"EVOLVE_INTERACTIVE_POLICY": PolicyEscalate})
	if body != "builder body" {
		t.Errorf("reqEnv should override process env; got %q", truncate(body, 120))
	}
}

func TestLaunch_PolicyBlockStableAcrossRuns(t *testing.T) {
	t.Setenv("EVOLVE_INTERACTIVE_POLICY", "")
	body1 := runOnce(t, "scout", "x", nil)
	body2 := runOnce(t, "scout", "y", nil)
	if prefix1, prefix2 := strings.TrimSuffix(body1, "x"), strings.TrimSuffix(body2, "y"); prefix1 != prefix2 {
		t.Errorf("policy prefix not stable across runs (cache invalidation risk)\n  run1: %q\n  run2: %q",
			truncate(prefix1, 100), truncate(prefix2, 100))
	}
}
