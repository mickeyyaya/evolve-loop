package advisor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

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

func exitErr(cli string, code int) scriptedResp {
	return scriptedResp{resp: LaunchResponse{ExitCode: code}, err: fmt.Errorf("%s: exit=%d", cli, code)}
}

func okPlan() scriptedResp { return scriptedResp{resp: LaunchResponse{Stdout: planJSON()}} }

func assertOneEvent(t *testing.T, got []signalcenter.Event, code signalcenter.Code, fields map[string]string) signalcenter.Event {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("exactly one event, got %+v", got)
	}
	e := got[0]
	if e.Code != code || e.Module != signalcenter.ModuleAdvisor || e.Kind != signalcenter.KindAdvisorWarning || e.Severity != signalcenter.SeverityWarn {
		t.Fatalf("event shape: %+v", e)
	}
	for k, v := range fields {
		if e.Fields[k] != v {
			t.Errorf("fields[%s] = %q, want %q (%+v)", k, e.Fields[k], v, e.Fields)
		}
	}
	return e
}

func TestNew_NilLauncherFailsEveryLaunchWithNilBridge(t *testing.T) {
	for _, c := range []struct {
		name, want, decision string
		launch               func(*Advisor, router.RouteInput) error
	}{
		{"propose", "routing proposer: nil bridge", "proposal", func(a *Advisor, in router.RouteInput) error { _, err := a.Propose(in); return err }},
		{"plan", "phase advisor: nil bridge", "plan", func(a *Advisor, in router.RouteInput) error { _, err := a.Plan(in); return err }},
		{"replan", "phase advisor: nil bridge", "replan", func(a *Advisor, in router.RouteInput) error { _, err := a.RePlan(in); return err }},
	} {
		a, got := observed(t, nil, defaultIdentity())
		if err := c.launch(a, baseRouteInput()); err == nil || err.Error() != c.want {
			t.Errorf("%s: %v, want %q", c.name, err, c.want)
		}
		e := assertOneEvent(t, *got, CodeLaunchFailed, map[string]string{"step": "preflight", "decision": c.decision})
		if e.Reason != c.want || e.Cycle != 7 || e.Phase != "build" {
			t.Errorf("%s: the event carries the error text and the cycle/phase stamp: %+v", c.name, e)
		}
	}
}

func TestLaunch_PreflightOrderIsBridgeWorkspaceDepth(t *testing.T) {
	noWs := baseRouteInput()
	noWs.Workspace = ""
	depthTrue := WithDepthCheck(func(map[string]string) bool { return true })
	cases := []struct {
		name string
		l    Launcher
		in   router.RouteInput
		want string
	}{
		{"nil launcher beats the empty workspace and the depth guard", nil, noWs, "phase advisor: nil bridge"},
		{"empty workspace beats the depth guard", refusingLauncher{t}, noWs, "phase advisor: empty workspace"},
		{"the depth guard beats the launch", refusingLauncher{t}, baseRouteInput(), "phase advisor: recursion guard: depth check failed"},
	}
	for _, c := range cases {
		a, got := observed(t, c.l, defaultIdentity(), depthTrue)
		if _, err := a.Plan(c.in); err == nil || err.Error() != c.want {
			t.Errorf("%s: %v, want %q", c.name, err, c.want)
		}
		assertOneEvent(t, *got, CodeLaunchFailed, map[string]string{"step": "preflight", "decision": "plan", "contract": "router"})
	}
	fl := &fakeLauncher{stdout: planJSON()}
	if _, err := New(fl, defaultIdentity(), nil, WithDepthCheck(func(map[string]string) bool { return false })).Plan(tempInput(t)); err != nil || fl.calls != 1 {
		t.Fatalf("a false depth guard must not block the advisor: %v (%d calls)", err, fl.calls)
	}
}

func TestLaunch_ThreadsWorktreeArtifactContractCompletionAgentCycleEnv(t *testing.T) {
	for _, c := range []struct {
		name, stdout, contract, artifact, completion string
		launch                                       func(*Advisor, router.RouteInput) error
	}{
		{"plan", planJSON(), "router", "routing-plan.json", "artifact", func(a *Advisor, in router.RouteInput) error { _, err := a.Plan(in); return err }},
		{"replan", `[{"phase":"audit","run":true}]`, "router-replan", "routing-replan.json", "artifact", func(a *Advisor, in router.RouteInput) error { _, err := a.RePlan(in); return err }},
		{"proposal", `{"next_phase":"audit"}`, "router-proposal", "routing-proposal.json", "artifact", func(a *Advisor, in router.RouteInput) error { _, err := a.Propose(in); return err }},
	} {
		for _, active := range []string{"", "/wt/cycle-7"} {
			fl := &fakeLauncher{stdout: c.stdout}
			in := tempInput(t)
			in.ActiveWorktree = active
			if err := c.launch(New(fl, defaultIdentity(), plainWriter), in); err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			r := fl.gotReq
			wantWT := active
			if wantWT == "" {
				wantWT = in.Workspace
			}
			if r.Worktree != wantWT || r.Workspace != in.Workspace || r.ProjectRoot != "/proj" {
				t.Errorf("%s: roots project=%q ws=%q wt=%q, want wt=%q", c.name, r.ProjectRoot, r.Workspace, r.Worktree, wantWT)
			}
			if r.ArtifactPath != filepath.Join(in.Workspace, c.artifact) || r.Contract != c.contract || r.Completion != c.completion || r.Agent != "router" {
				t.Errorf("%s: artifact=%q contract=%q completion=%q agent=%q", c.name, r.ArtifactPath, r.Contract, r.Completion, r.Agent)
			}
			if r.Cycle != 7 || r.Env == nil || r.Env["EVOLVE_CLI"] != "claude-tmux" || r.CLI != "claude-tmux" || r.Model != "opus" {
				t.Errorf("%s: cycle=%d env=%v cli=%q model=%q", c.name, r.Cycle, r.Env, r.CLI, r.Model)
			}
			if !strings.HasSuffix(r.Profile, "/.evolve/profiles/router.json") {
				t.Errorf("%s: profile=%q, want .../.evolve/profiles/router.json", c.name, r.Profile)
			}
		}
	}
}

func TestLaunch_WalksItsRouteAndReportsExhaustion(t *testing.T) {
	route := func(candidates ...string) Option {
		return WithRoute(llmroute.Plan{Candidates: candidates, Triggers: []int{80, 81, 85, 124, 127}})
	}
	for _, code := range []int{80, 81, 85, 124, 127} {
		fl := &fakeLauncher{seq: []scriptedResp{exitErr("agy-tmux", code), okPlan()}}
		plan, err := New(fl, Identity{CLI: "agy-tmux", Model: "opus", AgentLabel: "router"}, plainWriter, route("agy-tmux", "claude-tmux")).Plan(tempInput(t))
		if err != nil || plan == nil || len(plan.Entries) == 0 {
			t.Fatalf("exit=%d: the fallback must produce a plan: %v %+v", code, err, plan)
		}
		if got := fl.calledCLIs(); strings.Join(got, ",") != "agy-tmux,claude-tmux" {
			t.Errorf("exit=%d: CLIs tried %v", code, got)
		}
	}
	t.Run("three-hop chain", func(t *testing.T) {
		fl := &fakeLauncher{seq: []scriptedResp{exitErr("agy-tmux", 81), exitErr("codex-tmux", 81), okPlan()}}
		if _, err := New(fl, Identity{CLI: "agy-tmux"}, nil, route("agy-tmux", "codex-tmux", "claude-tmux")).Plan(tempInput(t)); err != nil {
			t.Fatal(err)
		}
		if got := fl.calledCLIs(); strings.Join(got, ",") != "agy-tmux,codex-tmux,claude-tmux" {
			t.Errorf("both intermediate hops tried in order: %v", got)
		}
	})
	t.Run("exhausted chain is one dispatch signal", func(t *testing.T) {
		root := t.TempDir()
		fl := &fakeLauncher{seq: []scriptedResp{exitErr("agy-tmux", 81), exitErr("claude-tmux", 81)}}
		in := tempInput(t)
		in.ProjectRoot = root
		a, got := observed(t, fl, Identity{CLI: "agy-tmux", Model: "opus", AgentLabel: "router"}, route("agy-tmux", "claude-tmux"))
		_, err := a.Plan(in)
		if err == nil || err.Error() != "phase advisor: bridge launch: claude-tmux: exit=81" {
			t.Fatalf("exhaustion surfaces the terminal attempt's error: %v", err)
		}
		e := assertOneEvent(t, *got, CodeLaunchFailed, map[string]string{
			"step": "dispatch", "cli": "agy-tmux", "chain": "agy-tmux,claude-tmux", "exit_code": "81",
			"profile": filepath.Join(root, ".evolve", "profiles", "router.json"), "decision": "plan", "contract": "router",
		})
		if e.Reason != err.Error() || e.Origin != "Advisor.Plan" {
			t.Errorf("reason/origin: %+v", e)
		}
	})
	t.Run("a non-trigger exit never reroutes", func(t *testing.T) {
		fl := &fakeLauncher{seq: []scriptedResp{exitErr("agy-tmux", 2)}}
		if _, err := New(fl, Identity{CLI: "agy-tmux"}, nil, route("agy-tmux", "claude-tmux")).Plan(tempInput(t)); err == nil || fl.calls != 1 {
			t.Fatalf("a real failure surfaces after exactly one launch: %v (%d)", err, fl.calls)
		}
	})
	t.Run("a one-CLI route dispatches once with no signal", func(t *testing.T) {
		fl := &fakeLauncher{stdout: planJSON()}
		a, got := observed(t, fl, defaultIdentity(), route("claude-tmux"))
		if _, err := a.Plan(tempInput(t)); err != nil || fl.calls != 1 || len(*got) != 0 {
			t.Fatalf("exactly one dispatch, no signal: %v (%d calls, %+v)", err, fl.calls, *got)
		}
	})
}

func TestLaunch_ResolvesSkillOverlaysPerAttemptFromTheZeroPolicy(t *testing.T) {
	fl := &fakeLauncher{stdout: planJSON()}
	if _, err := New(fl, Identity{CLI: "claude-tmux", Model: "deep", AgentLabel: "router"}, nil).Plan(tempInput(t)); err != nil {
		t.Fatal(err)
	}
	if len(fl.gotReq.Skills) != 1 || fl.gotReq.Skills[0] != "fable" {
		t.Errorf("a deep-tier advisor dispatch resolves the compiled overlay: %v", fl.gotReq.Skills)
	}
	fl = &fakeLauncher{stdout: planJSON()}
	if _, err := New(fl, defaultIdentity(), nil).Plan(tempInput(t)); err != nil {
		t.Fatal(err)
	}
	if len(fl.gotReq.Skills) != 1 || fl.gotReq.Skills[0] != "fable" {
		t.Errorf("the raw opus default is the deep tier, so the compiled deep rule resolves fable: %v", fl.gotReq.Skills)
	}
	root := writeRouterProfile(t, "agy-tmux", []string{"claude-tmux"}, []int{81})
	fl = &fakeLauncher{seq: []scriptedResp{exitErr("agy-tmux", 81), okPlan()}}
	in := tempInput(t)
	in.ProjectRoot = root
	var seen []string
	resolver := WithOverlayResolver(func(cli string) []string { seen = append(seen, cli); return []string{"custom-" + cli} })
	if _, err := New(fl, Identity{CLI: "agy-tmux"}, nil, resolver, WithRoute(llmroute.Plan{Candidates: []string{"agy-tmux", "claude-tmux"}, Triggers: []int{81}})).Plan(in); err != nil {
		t.Fatal(err)
	}
	if strings.Join(seen, ",") != "agy-tmux,claude-tmux" || fl.reqs[0].Skills[0] != "custom-agy-tmux" || fl.reqs[1].Skills[0] != "custom-claude-tmux" {
		t.Errorf("the resolver sees every attempted cli: %v / %v", seen, fl.reqs)
	}
}

func TestLaunch_ProfilePathProjectsEvolveDirOf(t *testing.T) {
	fl := &fakeLauncher{stdout: planJSON()}
	in := tempInput(t)
	in.ProjectRoot = t.TempDir()
	if _, err := New(fl, defaultIdentity(), nil).Plan(in); err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(paths.EvolveDirOf(in.ProjectRoot), "profiles", "router.json"); fl.gotReq.Profile != want {
		t.Errorf("profile = %q, want %q", fl.gotReq.Profile, want)
	}
	fl = &fakeLauncher{stdout: planJSON()}
	if _, err := New(fl, Identity{CLI: "claude-tmux", Profile: "/explicit.json"}, nil).Plan(in); err != nil {
		t.Fatal(err)
	}
	if fl.gotReq.Profile != "/explicit.json" {
		t.Errorf("the identity's explicit profile wins: %q", fl.gotReq.Profile)
	}
}

func TestPropose_FailuresAreVisibleForTheFirstTime(t *testing.T) {
	a, got := observed(t, &fakeLauncher{err: errors.New("boom")}, defaultIdentity())
	if _, err := a.Propose(tempInput(t)); err == nil || err.Error() != "routing proposer: bridge launch: boom" {
		t.Fatalf("Propose: %v", err)
	}
	e := assertOneEvent(t, *got, CodeLaunchFailed, map[string]string{"step": "dispatch", "decision": "proposal", "contract": "router-proposal", "chain": "claude-tmux", "exit_code": "0"})
	if e.Origin != "Advisor.Propose" {
		t.Errorf("origin: %+v", e)
	}
	fl := &fakeLauncher{stdout: `{"next_phase":"ship","justification":"skip audit"}`}
	strat := router.LLMProposal{Proposer: New(fl, defaultIdentity(), nil)}
	in := tempInput(t)
	in.Cfg = routingCfg()
	in.Completed = []string{"scout", "build"}
	dec := strat.Decide(in)
	if dec.NextPhase != "audit" || fl.calls != 1 {
		t.Errorf("the kernel forces audit before ship (%s) after one launch (%d)", dec.NextPhase, fl.calls)
	}
	clamped := false
	for _, c := range dec.Clamps {
		clamped = clamped || (c.Rule == "llm-proposal-clamped" && c.Proposed == "ship" && c.Forced == "audit")
	}
	if !clamped {
		t.Errorf("expected llm-proposal-clamped(ship->audit), clamps=%+v", dec.Clamps)
	}
}

func TestRePlan_UsesTheReplanDecisionEndToEnd(t *testing.T) {
	ws := t.TempDir()
	fl := &fakeLauncher{stdout: `[{"phase":"scout","run":true,"justification":"x"},{"phase":"build","run":true,"justification":"y"}]`, durationMS: 9}
	in := baseRouteInput()
	in.Workspace = ws
	in.Signals = router.RoutingSignals{Scout: router.ScoutSignals{Present: true, ItemCount: 5, CycleSizeEstimate: "large"}}
	a := New(fl, Identity{CLI: "claude-tmux", Model: "opus", Persona: "PERSONA", AgentLabel: "router"}, plainWriter)
	got, err := a.RePlan(in)
	if err != nil || len(got.Entries) != 2 {
		t.Fatalf("RePlan: %v %+v", err, got)
	}
	if fl.gotReq.ArtifactPath != filepath.Join(ws, "routing-replan.json") || fl.gotReq.Contract != "router-replan" {
		t.Errorf("artifact=%q contract=%q", fl.gotReq.ArtifactPath, fl.gotReq.Contract)
	}
	if !strings.Contains(fl.gotReq.Prompt, "item_count=5") {
		t.Errorf("the populated signals reach the prompt:\n%s", fl.gotReq.Prompt)
	}
	if span := readArtifact(t, filepath.Join(ws, "advisor-span-replan.json")); !strings.Contains(span, `"replan_depth":1`) {
		t.Errorf("replan_depth 1 on the re-plan span: %s", span)
	}
	if _, err := a.Plan(in); err != nil {
		t.Fatal(err)
	}
	if span := readArtifact(t, filepath.Join(ws, "advisor-span-plan.json")); !strings.Contains(span, `"replan_depth":0`) {
		t.Errorf("replan_depth 0 on the initial span: %s", span)
	}
}

func TestLaunch_DispatchWiringFlowsToTheLauncher(t *testing.T) {
	for _, c := range []struct{ cli, model string }{{"codex-tmux", "gpt-5.5"}, {"agy", "gemini-3.5-flash"}, {"claude-tmux", "opus"}} {
		fl := &fakeLauncher{stdout: planJSON()}
		if _, err := New(fl, Identity{CLI: c.cli, Model: c.model, Persona: "PERSONA_MARKER_42", AgentLabel: "router"}, nil).Plan(tempInput(t)); err != nil {
			t.Fatalf("%s: %v", c.cli, err)
		}
		r := fl.gotReq
		if r.CLI != c.cli || r.Model != c.model || r.Completion != "artifact" || !strings.HasSuffix(r.ArtifactPath, "routing-plan.json") || !strings.Contains(r.Prompt, "PERSONA_MARKER_42") || r.Agent != "router" {
			t.Errorf("%s/%s: %+v", c.cli, c.model, r)
		}
	}
}
