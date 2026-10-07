package main

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCycleRunFlags(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		ok      bool
		want    cycleRunFlags
		message string
	}{
		{"every flag", []string{"--project-root", "/p", "--goal-hash", "abcd1234", "--goal", "ship it", "--evolve-dir", "/p/.e", "--simulate", "--bypass-policy"}, true,
			cycleRunFlags{projectRoot: "/p", goalHash: "abcd1234", goalText: "ship it", evolveDir: "/p/.e", simulate: true, bypassPolicy: true}, ""},
		{"defaults", []string{"--goal-hash", "abcd1234"}, true, cycleRunFlags{projectRoot: ".", goalHash: "abcd1234"}, ""},
		{"missing goal hash", []string{"--project-root", "/p"}, false, cycleRunFlags{}, "--goal-hash is required"},
		{"unknown flag", []string{"--goal-hash", "abcd1234", "--nope"}, false, cycleRunFlags{}, "flag provided but not defined: -nope"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer

			got, ok := parseCycleRunFlags(tc.args, &stderr)

			if ok != tc.ok || (tc.ok && got != tc.want) || !strings.Contains(stderr.String(), tc.message) {
				t.Errorf("parseCycleRunFlags(%q) = (%+v, %v), stderr %q; want (%+v, %v) naming %q", tc.args, got, ok, stderr.String(), tc.want, tc.ok, tc.message)
			}
		})
	}
}

func TestCycleRunFlags_RequestCarriesTheGoalTheBypassAndTheEvolveEnv(t *testing.T) {
	f := cycleRunFlags{goalHash: "abcd1234", goalText: "ship it", bypassPolicy: true}

	req := f.request("/abs/root", []string{"EVOLVE_X=1", "HOME=/h"})

	if req.ProjectRoot != "/abs/root" || req.GoalHash != "abcd1234" || req.Context["goal"] != "ship it" || !req.BypassPolicy {
		t.Errorf("request = %+v, want the absolutized root, the goal hash, the goal text in Context and the bypass", req)
	}
	if len(req.Env) != 1 || req.Env["EVOLVE_X"] != "1" {
		t.Errorf("request Env = %v, want only the EVOLVE_ variable", req.Env)
	}
}

func TestRunCycleRun_SimulateNeverWiresTheProductionOrchestrator(t *testing.T) {
	old := wireOrchestratorDepsFn
	t.Cleanup(func() { wireOrchestratorDepsFn = old })
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
		t.Fatal("--simulate wired the production orchestrator, which calls out to real CLIs")
		return orchDeps{}
	}
	var stdout, stderr bytes.Buffer

	runCycleRun([]string{"--project-root", t.TempDir(), "--goal-hash", "abcd1234", "--simulate"}, &stdout, &stderr)
}

func TestCycleHealthRoot_PrefersTheEnvOverTheWorkspace(t *testing.T) {
	env := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "proj", ".evolve", "runs", "cycle-7")

	t.Setenv("EVOLVE_PROJECT_ROOT", env)
	if got := cycleHealthRoot(workspace, io.Discard); got != env {
		t.Errorf("with EVOLVE_PROJECT_ROOT set, cycleHealthRoot = %q, want %q", got, env)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	if got := cycleHealthRoot(workspace, io.Discard); got != filepath.Dir(filepath.Dir(filepath.Dir(workspace))) {
		t.Errorf("without it, cycleHealthRoot = %q, want the workspace's project root", got)
	}
}
