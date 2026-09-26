package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

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

func TestLaunch_OnStopReviewBranchStillValidatesViaEngine(t *testing.T) {
	a := New()
	a.SetOnStopReview(func(int, string, string, string) {})
	_, err := a.Launch(context.Background(), core.BridgeRequest{
		CLI: "not-a-real-cli", Profile: "/p", Model: "auto",
		Prompt: "body", Workspace: t.TempDir(), ArtifactPath: "/a.md", Agent: "scout",
	})
	if err == nil {
		t.Fatal("Launch with unsupported CLI must return an engine error")
	}
}

func TestLaunch_InjectsDeliverableContract(t *testing.T) {
	fe := &fakeEngine{}
	artifact := "/abs/.evolve/runs/cycle-213/build-report.md"
	_, err := withEngine(fe).Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: "/p", Prompt: "PERSONA-BODY",
		Workspace: t.TempDir(), ArtifactPath: artifact, Agent: "build",
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	got := fe.gotReq.Prompt
	if !strings.Contains(got, "## Deliverable Contract (build)") {
		t.Errorf("prompt missing contract block:\n%s", truncate(got, 400))
	}
	if !strings.Contains(got, artifact) {
		t.Errorf("prompt missing exact artifact path %q", artifact)
	}
	if strings.Index(got, artifact) < strings.Index(got, "PERSONA-BODY") {
		t.Errorf("artifact path must appear AFTER the body (footer/recency); prompt:\n%s", got)
	}
	if before := got[:strings.Index(got, "PERSONA-BODY")]; strings.Contains(before, artifact) {
		t.Errorf("artifact path leaked into the cacheable prefix:\n%s", before)
	}
}

func TestLaunch_NoContract_ForUnregisteredAgent(t *testing.T) {
	fe := &fakeEngine{}
	_, err := withEngine(fe).Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: "/p", Prompt: "BODY",
		Workspace: t.TempDir(), ArtifactPath: "/a.md", Agent: "not-a-phase",
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if strings.Contains(fe.gotReq.Prompt, "Deliverable Contract") {
		t.Errorf("unregistered agent should get no contract block")
	}
}

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

func TestProbe_DefaultEngineFactoryIsUsable(t *testing.T) {
	got, err := New().Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe through default engine factory: %v", err)
	}
	if got.Version == "" {
		t.Fatalf("Probe version is empty: %+v", got)
	}
	if len(got.CLIs) == 0 {
		t.Fatalf("Probe CLI map is empty: %+v", got)
	}
}

func TestNewDefault_ReturnsUsableAdapter(t *testing.T) {
	a := NewDefault("/any/project/root", nil)
	if a == nil || a.engineFactory == nil {
		t.Fatal("NewDefault must wire a non-nil engine factory")
	}
}

// runOnce launches against a fake engine and returns the prompt the engine received.
func runOnce(t *testing.T, agent, prompt string, env map[string]string) string {
	return runOnceWithPolicy(t, agent, prompt, env, "")
}

func runOnceWithPolicy(t *testing.T, agent, prompt string, env map[string]string, policy string) string {
	t.Helper()
	fe := &fakeEngine{}
	_, err := withEngine(fe).Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: "/p", Model: "auto",
		Prompt: prompt, Workspace: t.TempDir(), ArtifactPath: "/a.md", Agent: agent, Env: env,
		InteractivePolicy: policy,
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	return fe.gotReq.Prompt
}

func TestLaunch_DefaultPolicy_InjectsRecommendedOrFirstPrefix(t *testing.T) {
	body := runOnce(t, "scout", "scout prompt body", nil)
	if !strings.HasPrefix(body, "## Subagent Interactive Policy (recommended_or_first)") {
		t.Errorf("prompt missing recommended-or-first prefix; got first 80 chars: %q", truncate(body, 80))
	}
	if !strings.Contains(body, "scout prompt body") {
		t.Errorf("prompt missing original body after prefix")
	}
}

func TestLaunch_NoPolicyPrefix_WhenEscalateExplicit(t *testing.T) {
	body := runOnceWithPolicy(t, "builder", "builder body", nil, PolicyEscalate)
	if strings.Contains(body, "Subagent Interactive Policy") {
		t.Errorf("escalate policy must not inject a block; got first 120 chars: %q", truncate(body, 120))
	}
	// The contract block is still injected under escalate, so only the body's presence is asserted.
	if !strings.Contains(body, "builder body") {
		t.Errorf("original body missing under escalate; got %q", truncate(body, 120))
	}
}

func TestLaunch_AutoYesPolicy_InjectsAlternatePrefix(t *testing.T) {
	body := runOnceWithPolicy(t, "auditor", "auditor body", nil, PolicyAutoYes)
	if !strings.HasPrefix(body, "## Subagent Interactive Policy (auto_yes)") {
		t.Errorf("auto_yes policy must inject auto_yes block; got first 80 chars: %q", truncate(body, 80))
	}
	if !strings.Contains(body, "auditor body") {
		t.Errorf("prompt missing original body after prefix")
	}
}

func writePolicyJson(t *testing.T, dir, content string) {
	t.Helper()
	dotEvolve := filepath.Join(dir, ".evolve")
	if err := os.MkdirAll(dotEvolve, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dotEvolve, "policy.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestResolvePolicy_PolicyJsonHasPrecedence(t *testing.T) {
	dir := t.TempDir()
	writePolicyJson(t, dir, `{"workflow": {"interactive_policy": "auto_yes", "interactive_policies": {"scout": "escalate"}}}`)

	if got := resolvePolicy(dir, "scout", PolicyRecommendedOrFirst); got != PolicyEscalate {
		t.Errorf("policy.json per-agent should win: got=%q want=%q", got, PolicyEscalate)
	}
	if got := resolvePolicy(dir, "builder", PolicyEscalate); got != PolicyAutoYes {
		t.Errorf("policy.json global should win over profile: got=%q want=%q", got, PolicyAutoYes)
	}
}

func TestResolvePolicy_ProfilePolicyHasPrecedenceIfDefault(t *testing.T) {
	dir := t.TempDir()
	if got := resolvePolicy(dir, "scout", PolicyEscalate); got != PolicyEscalate {
		t.Errorf("profile should be used when policy.json is empty: got=%q want=%q", got, PolicyEscalate)
	}
}

func TestResolvePolicy_DefaultWhenUnset(t *testing.T) {
	dir := t.TempDir()
	if got := resolvePolicy(dir, "builder", ""); got != PolicyRecommendedOrFirst {
		t.Errorf("default policy got=%q want=%q", got, PolicyRecommendedOrFirst)
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

func TestLaunch_PolicyBlockStableAcrossRuns(t *testing.T) {
	body1 := runOnce(t, "scout", "BODYTOKEN1", nil)
	body2 := runOnce(t, "scout", "BODYTOKEN2", nil)
	prefix1 := body1[:strings.Index(body1, "BODYTOKEN1")]
	prefix2 := body2[:strings.Index(body2, "BODYTOKEN2")]
	if prefix1 != prefix2 {
		t.Errorf("cacheable prefix not stable across runs (cache invalidation risk)\n  run1: %q\n  run2: %q",
			truncate(prefix1, 100), truncate(prefix2, 100))
	}
}

func TestLaunch_BootTimeoutStoreWired(t *testing.T) {
	projectRoot := t.TempDir()
	a := NewDefault(projectRoot, nil)
	if !a.BootTimeoutStoreWired() {
		t.Error("expected BootTimeoutStoreWired() true for an Adapter built via NewDefault (production deps inject the boot-timeout strike writer)")
	}
	if New().BootTimeoutStoreWired() {
		t.Error("expected BootTimeoutStoreWired() false for a bare New() Adapter")
	}

	eng := a.engineFactory(nil)
	concrete, ok := eng.(*gobridge.Engine)
	if !ok {
		t.Fatalf("expected engineFactory to return a *bridge.Engine, got %T", eng)
	}
	if concrete == nil {
		t.Error("expected concrete engine to be non-nil")
	}
}
