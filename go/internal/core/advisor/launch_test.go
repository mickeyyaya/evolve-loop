package advisor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
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

func TestLaunch_PreflightOrderIsBridgeWorkspaceDepthProfile(t *testing.T) {
	noWs := baseRouteInput()
	noWs.Workspace = ""
	depthTrue := WithDepthCheck(func(map[string]string) bool { return true })
	loads := 0
	counting := WithProfileLoader(func(string) (*profiles.Profile, error) { loads++; return nil, errors.New("must not load") })
	cases := []struct {
		name string
		l    Launcher
		in   router.RouteInput
		want string
	}{
		{"nil launcher beats the empty workspace and the depth guard", nil, noWs, "phase advisor: nil bridge"},
		{"empty workspace beats the depth guard", refusingLauncher{t}, noWs, "phase advisor: empty workspace"},
		{"the depth guard beats the profile read and the launch", refusingLauncher{t}, baseRouteInput(), "phase advisor: recursion guard: depth check failed"},
	}
	for _, c := range cases {
		a, got := observed(t, c.l, defaultIdentity(), depthTrue, counting)
		if _, err := a.Plan(c.in); err == nil || err.Error() != c.want {
			t.Errorf("%s: %v, want %q", c.name, err, c.want)
		}
		assertOneEvent(t, *got, CodeLaunchFailed, map[string]string{"step": "preflight", "decision": "plan", "contract": "router"})
	}
	if loads != 0 {
		t.Errorf("the profile loader ran %d time(s) on refused launches", loads)
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

func TestLaunch_WalksTheProfileFallbackChainAndReportsExhaustion(t *testing.T) {
	for _, code := range []int{80, 81, 85, 124, 127} {
		root := writeRouterProfile(t, "agy-tmux", []string{"claude-tmux"}, []int{80, 81, 85, 124, 127})
		fl := &fakeLauncher{seq: []scriptedResp{exitErr("agy-tmux", code), okPlan()}}
		in := tempInput(t)
		in.ProjectRoot = root
		plan, err := New(fl, Identity{CLI: "agy-tmux", Model: "opus", AgentLabel: "router"}, plainWriter).Plan(in)
		if err != nil || plan == nil || len(plan.Entries) == 0 {
			t.Fatalf("exit=%d: the fallback must produce a plan: %v %+v", code, err, plan)
		}
		if got := fl.calledCLIs(); strings.Join(got, ",") != "agy-tmux,claude-tmux" {
			t.Errorf("exit=%d: CLIs tried %v", code, got)
		}
	}
	t.Run("three-hop chain", func(t *testing.T) {
		root := writeRouterProfile(t, "agy-tmux", []string{"codex-tmux", "claude-tmux"}, []int{81})
		fl := &fakeLauncher{seq: []scriptedResp{exitErr("agy-tmux", 81), exitErr("codex-tmux", 81), okPlan()}}
		in := tempInput(t)
		in.ProjectRoot = root
		if _, err := New(fl, Identity{CLI: "agy-tmux"}, nil).Plan(in); err != nil {
			t.Fatal(err)
		}
		if got := fl.calledCLIs(); strings.Join(got, ",") != "agy-tmux,codex-tmux,claude-tmux" {
			t.Errorf("both intermediate hops tried in order: %v", got)
		}
	})
	t.Run("exhausted chain is one dispatch signal", func(t *testing.T) {
		root := writeRouterProfile(t, "agy-tmux", []string{"claude-tmux"}, []int{81})
		fl := &fakeLauncher{seq: []scriptedResp{exitErr("agy-tmux", 81), exitErr("claude-tmux", 81)}}
		in := tempInput(t)
		in.ProjectRoot = root
		a, got := observed(t, fl, Identity{CLI: "agy-tmux", Model: "opus", AgentLabel: "router"})
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
		root := writeRouterProfile(t, "agy-tmux", []string{"claude-tmux"}, []int{81})
		fl := &fakeLauncher{seq: []scriptedResp{exitErr("agy-tmux", 2)}}
		in := tempInput(t)
		in.ProjectRoot = root
		if _, err := New(fl, Identity{CLI: "agy-tmux"}, nil).Plan(in); err == nil || fl.calls != 1 {
			t.Fatalf("a real failure surfaces after exactly one launch: %v (%d)", err, fl.calls)
		}
	})
	for name, profile := range map[string]string{
		"no cli_fallback":             `{"name":"router","cli":"claude-tmux"}`,
		"explicit-empty cli_fallback": `{"name":"router","cli":"claude-tmux","cli_fallback":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, ".evolve", "profiles"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".evolve", "profiles", "router.json"), []byte(profile), 0o644); err != nil {
				t.Fatal(err)
			}
			fl := &fakeLauncher{stdout: planJSON()}
			in := tempInput(t)
			in.ProjectRoot = root
			a, got := observed(t, fl, defaultIdentity())
			if _, err := a.Plan(in); err != nil || fl.calls != 1 || len(*got) != 0 {
				t.Fatalf("exactly one dispatch, no signal: %v (%d calls, %+v)", err, fl.calls, *got)
			}
		})
	}
}

func TestLaunch_ProfileLoadFaultWarnsOnceAndAbsenceIsSilent(t *testing.T) {
	seed := func(t *testing.T, seedFn func(path string)) (string, string) {
		t.Helper()
		root := t.TempDir()
		path := filepath.Join(root, ".evolve", "profiles", "router.json")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		seedFn(path)
		return root, path
	}
	for name, seedFn := range map[string]func(path string){
		"a directory at router.json": func(path string) {
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
		},
		"malformed router.json": func(path string) {
			if err := os.WriteFile(path, []byte(`{not valid json`), 0o644); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			root, path := seed(t, seedFn)
			fl := &fakeLauncher{stdout: planJSON()}
			in := tempInput(t)
			in.ProjectRoot = root
			a, got := observed(t, fl, defaultIdentity())
			if _, err := a.Plan(in); err != nil || fl.calls != 1 {
				t.Fatalf("the dispatch still runs once on the primary: %v (%d)", err, fl.calls)
			}
			assertOneEvent(t, *got, CodeProfileLoadFailed, map[string]string{"step": "dispatch", "path": path, "decision": "plan"})
		})
	}
	t.Run("missing router.json", func(t *testing.T) {
		fl := &fakeLauncher{stdout: planJSON()}
		in := tempInput(t)
		in.ProjectRoot = t.TempDir()
		a, got := observed(t, fl, defaultIdentity())
		if _, err := a.Plan(in); err != nil || fl.calls != 1 || len(*got) != 0 {
			t.Fatalf("absence is the silent single dispatch: %v (%d calls, %+v)", err, fl.calls, *got)
		}
	})
	t.Run("no project root and no explicit profile reads nothing", func(t *testing.T) {
		loads := 0
		fl := &fakeLauncher{stdout: planJSON()}
		in := tempInput(t)
		in.ProjectRoot = ""
		a, got := observed(t, fl, defaultIdentity(), WithProfileLoader(func(string) (*profiles.Profile, error) { loads++; return nil, errors.New("x") }))
		if _, err := a.Plan(in); err != nil || loads != 0 || len(*got) != 0 || fl.gotReq.Profile != "" {
			t.Fatalf("an empty profile path short-circuits the read: %v loads=%d events=%+v profile=%q", err, loads, *got, fl.gotReq.Profile)
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
	if len(fl.gotReq.Skills) != 0 {
		t.Errorf("the raw opus default matches no tier selector: %v", fl.gotReq.Skills)
	}
	root := writeRouterProfile(t, "agy-tmux", []string{"claude-tmux"}, []int{81})
	fl = &fakeLauncher{seq: []scriptedResp{exitErr("agy-tmux", 81), okPlan()}}
	in := tempInput(t)
	in.ProjectRoot = root
	var seen []string
	resolver := WithOverlayResolver(func(cli string) []string { seen = append(seen, cli); return []string{"custom-" + cli} })
	if _, err := New(fl, Identity{CLI: "agy-tmux"}, nil, resolver).Plan(in); err != nil {
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
