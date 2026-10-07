package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func overrideProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, doc := range map[string]map[string]any{
		"auditor": {"name": "auditor", "cli": "claude-tmux", "allowed_clis": []string{"claude"}, "model_tier_default": "deep"},
		"scout":   {"name": "scout", "cli": "codex-tmux", "model_tier_default": "balanced"},
		"router":  {"name": "router", "cli": "agy-claude-tmux", "allowed_clis": []string{"agy-claude", "claude"}, "model_tier_default": "deep"},
	} {
		raw, _ := json.Marshal(doc)
		if err := os.WriteFile(filepath.Join(dir, name+".json"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func overrideRouter(t *testing.T, root string, bypass bool) *cliroute.Router {
	t.Helper()
	block := policy.CLIRouting{CLIs: []string{"agy", "agy-claude", "claude"}, Default: []string{"agy", "agy-claude", "claude"}}
	r, _, err := cliroute.Build(cliroute.Setup{
		Policy: policy.Policy{CLIRouting: &block}, Profiles: profiles.NewFromDir(routingProfilesDir(root)),
		Host: cliroute.Host{LookPath: everyBinaryPresent}, Bypass: bypass,
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return r
}

func refusedAgents(t *testing.T, refusals []string) []string {
	t.Helper()
	var agents []string
	for _, line := range refusals {
		agent, _, _ := strings.Cut(line, ":")
		agents = append(agents, agent)
	}
	return agents
}

func TestOverrideRefusals_AnOverrideOutsideTheAllowedSetIsRefusedAtStart(t *testing.T) {
	root := overrideProject(t)
	env := map[string]string{"EVOLVE_AUDITOR_CLI": "codex-tmux", "EVOLVE_SCOUT_CLI": "agy-tmux", "EVOLVE_ROUTER_CLI": "claude-tmux"}

	refusals, err := overrideRefusals(overrideRouter(t, root, false), root, env)

	if err != nil || !reflect.DeepEqual(refusedAgents(t, refusals), []string{"auditor"}) {
		t.Fatalf("refusals = %q (err %v), want the auditor alone", refusals, err)
	}
}

func TestOverrideRefusals_UnderBypassTheFloorAndTheProfileCeilingStillBind(t *testing.T) {
	root := overrideProject(t)
	env := map[string]string{"EVOLVE_AUDITOR_CLI": "codex-tmux", "EVOLVE_ROUTER_CLI": "agy-tmux", "EVOLVE_SCOUT_CLI": "codex-tmux"}

	refusals, err := overrideRefusals(overrideRouter(t, root, true), root, env)

	if err != nil || !reflect.DeepEqual(refusedAgents(t, refusals), []string{"auditor", "router"}) {
		t.Fatalf("refusals = %q (err %v), want the floor auditor and the router's profile ceiling, never the unconstrained scout", refusals, err)
	}
}

func TestOverrideRefusals_TheGlobalCLIOverrideIsCheckedForEveryAgent(t *testing.T) {
	root := overrideProject(t)

	refusals, err := overrideRefusals(overrideRouter(t, root, false), root, map[string]string{"EVOLVE_CLI": "agy-tmux"})

	if err != nil || !reflect.DeepEqual(refusedAgents(t, refusals), []string{"auditor", "router"}) {
		t.Fatalf("refusals = %q (err %v), want every agent whose allowed set excludes agy", refusals, err)
	}
}

func TestOverrideRefusals_NoOverrideNoRefusal(t *testing.T) {
	root := overrideProject(t)

	refusals, err := overrideRefusals(overrideRouter(t, root, false), root, map[string]string{"EVOLVE_PROJECT_ROOT": root})

	if err != nil || len(refusals) != 0 {
		t.Fatalf("refusals = %q (err %v), want none", refusals, err)
	}
}

func TestWireOrchestratorDeps_ARefusedOverrideIsARoutingErrorBeforeAnyDispatch(t *testing.T) {
	root, evolveDir := wiringRoot(t, `{"cli_routing":{"clis":["agy","claude"],"default":["agy","claude"]}}`)
	if err := os.WriteFile(filepath.Join(evolveDir, "profiles", "auditor.json"), []byte(`{"name":"auditor","cli":"claude-tmux","allowed_clis":["claude"],"model_tier_default":"deep"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	d := wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{env: map[string]string{"EVOLVE_AUDITOR_CLI": "codex-tmux"}})

	if d.RoutingErr == nil || !strings.Contains(d.RoutingErr.Error(), "EVOLVE_AUDITOR_CLI") {
		t.Fatalf("RoutingErr = %v, want the refused override named", d.RoutingErr)
	}
}

func TestRunLoop_HandsTheWiringItsBypassAndItsCLIOverrides(t *testing.T) {
	var got routingRun
	old := wireOrchestratorDepsFn
	t.Cleanup(func() { wireOrchestratorDepsFn = old })
	wireOrchestratorDepsFn = func(_, _ string, _ io.Writer, run routingRun) orchDeps {
		got = run
		return orchDeps{RoutingErr: os.ErrClosed}
	}
	root, evolveDir := wiringRoot(t, "")
	var stdout, stderr bytes.Buffer

	runLoop([]string{"--project-root", root, "--evolve-dir", evolveDir, "--goal-text", "x", "--cycles", "1", "--force-fresh", "--skip-preflight", "--bypass-policy", "--cli", "auditor=claude-tmux"}, nil, &stdout, &stderr)

	if !got.bypass || got.env["EVOLVE_AUDITOR_CLI"] != "claude-tmux" {
		t.Fatalf("run = %+v, want the loop's --bypass-policy and its --cli override", got)
	}
}

func TestRunCycleRun_HandsTheWiringItsBypass(t *testing.T) {
	var got routingRun
	old := wireOrchestratorDepsFn
	t.Cleanup(func() { wireOrchestratorDepsFn = old })
	wireOrchestratorDepsFn = func(_, _ string, _ io.Writer, run routingRun) orchDeps {
		got = run
		return orchDeps{RoutingErr: os.ErrClosed}
	}
	var stdout, stderr bytes.Buffer

	runCycleRun([]string{"--project-root", t.TempDir(), "--goal-hash", "abcd1234", "--bypass-policy"}, &stdout, &stderr)

	if !got.bypass {
		t.Fatalf("run = %+v, want the cycle's --bypass-policy", got)
	}
}

func TestWireCLIRouter_ABypassRunRoutesEveryLaunchWithoutTheTable(t *testing.T) {
	root := overrideProject(t)
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"cli_routing":{"clis":["agy","agy-claude","claude"],"default":["agy","agy-claude","claude"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		bypass     bool
		wantLegacy bool
	}{{true, true}, {false, false}} {
		r, err := wireCLIRouter(routerSite{root: root}, routingRun{bypass: tc.bypass}, io.Discard)
		if err != nil {
			t.Fatalf("bypass=%v: %v", tc.bypass, err)
		}
		d, err := r.Resolve(cliroute.Request{Agent: "scout", ProjectRoot: root, DefaultModel: unsetDispatchTier})
		if err != nil || d.Legacy() != tc.wantLegacy {
			t.Errorf("bypass=%v: rule %q legacy=%v, want legacy=%v", tc.bypass, d.Rule, d.Legacy(), tc.wantLegacy)
		}
	}
}

func TestOverrideRefusals_TheFloorBindsAFloorAgentWhoseProfileNamesNoCeiling(t *testing.T) {
	root := overrideProject(t)
	if err := os.WriteFile(filepath.Join(routingProfilesDir(root), "tdd-engineer.json"), []byte(`{"name":"tdd-engineer","cli":"claude-tmux","model_tier_default":"balanced"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	refusals, err := overrideRefusals(overrideRouter(t, root, true), root, map[string]string{"EVOLVE_TDD_ENGINEER_CLI": "codex-tmux"})

	if err != nil || !reflect.DeepEqual(refusedAgents(t, refusals), []string{"tdd-engineer"}) {
		t.Fatalf("refusals = %q (err %v): the Claude-family floor is code, not profile hygiene", refusals, err)
	}
}

func TestOverrideRefusals_TheAgentKeyOutranksTheGlobalOverrideAsTheResolverReadsThem(t *testing.T) {
	root := overrideProject(t)
	env := map[string]string{"EVOLVE_AUDITOR_CLI": "agy-tmux", "EVOLVE_CLI": "claude-tmux"}

	refusals, err := overrideRefusals(overrideRouter(t, root, true), root, env)

	if err != nil || !reflect.DeepEqual(refusedAgents(t, refusals), []string{"auditor"}) {
		t.Fatalf("refusals = %q (err %v), want the auditor (its own key names agy)", refusals, err)
	}
}

func TestOverrideRefusals_NamesTheRemedyForTheKeyThatWon(t *testing.T) {
	root := overrideProject(t)
	for env, remedy := range map[string]string{
		"EVOLVE_AUDITOR_CLI": "unset EVOLVE_AUDITOR_CLI",
		"EVOLVE_CLI":         "set EVOLVE_AUDITOR_CLI",
	} {
		refusals, err := overrideRefusals(overrideRouter(t, root, false), root, map[string]string{env: "codex-tmux"})
		if err != nil || len(refusals) == 0 || !strings.Contains(refusals[0], remedy) {
			t.Errorf("%s: refusals = %q (err %v), want the remedy %q", env, refusals, err, remedy)
		}
	}
}
