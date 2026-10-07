package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func routingWriteProject(t *testing.T, policyJSON string) string {
	t.Helper()
	root := t.TempDir()
	dir := routingProfilesDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"router":  `{"name":"router","cli":"agy-claude-tmux","allowed_clis":["agy-claude","claude"],"model_tier_default":"deep"}`,
		"auditor": `{"name":"auditor","cli":"claude-tmux","allowed_clis":["claude"],"model_tier_default":"deep"}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if policyJSON != "" {
		if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(policyJSON), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	return root
}

const initVerb = "init"

func inRoot(root string, args ...string) []string {
	return append(args, "--project-root", root)
}

func readRoutingPolicy(t *testing.T, root string) (policy.Policy, []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	pol, err := policy.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return pol, raw
}

func TestCLIRoutingInit_WritesACompilingBlockAndMovesTheLegacyTail(t *testing.T) {
	root := routingWriteProject(t, `{"gc": {"mode": "enforce"}, "workflow": {"universal_fallback_exclude": []}}`)

	rc, stdout, stderr := runRoutingVerb(t, inRoot(root, initVerb, "--clis", "agy,agy-claude,claude")...)

	if rc != 0 {
		t.Fatalf("rc=%d stderr=%s", rc, stderr)
	}
	pol, raw := readRoutingPolicy(t, root)
	if pol.CLIRouting == nil || !reflect.DeepEqual(pol.CLIRouting.CLIs, []string{"agy", "agy-claude", "claude"}) || !reflect.DeepEqual(pol.CLIRouting.Default, pol.CLIRouting.CLIs) {
		t.Fatalf("block = %+v, want clis and default in the given order", pol.CLIRouting)
	}
	if pol.Workflow != nil || !strings.Contains(string(raw), `"mode": "enforce"`) {
		t.Fatalf("the legacy tail key leaves with its now-empty workflow block, and gc stays: %s", raw)
	}
	if !strings.Contains(stdout, "ship with: evolve ship --class manual (at a wave boundary)") {
		t.Errorf("stdout = %q, want the ship hint", stdout)
	}
}

func TestCLIRoutingInit_RefusesASecondInit(t *testing.T) {
	root := routingWriteProject(t, `{"cli_routing": {"clis": ["claude"], "default": ["claude"]}}`)
	_, before := readRoutingPolicy(t, root)

	rc, _, stderr := runRoutingVerb(t, inRoot(root, initVerb, "--clis", "agy,claude")...)

	if _, after := readRoutingPolicy(t, root); rc != exitRoutingFinding || !bytes.Equal(before, after) || !strings.Contains(stderr, "cli-routing set") {
		t.Fatalf("rc=%d stderr=%q: an existing block is edited with set, never re-initialized", rc, stderr)
	}
}

func TestCLIRoutingSet_EditsOneEntryOfEachShape(t *testing.T) {
	root := routingWriteProject(t, `{"cli_routing": {"clis": ["agy", "agy-claude", "claude"], "default": ["agy", "claude"]}}`)
	for _, args := range [][]string{
		{"set", "default", "agy,agy-claude,claude"},
		{"set", "tiers.deep", "agy-claude,claude"},
		{"set", "work.build", "agy,claude"},
		{"set", "agents.router", "agy-claude,claude", "--model", "deep"},
		{"set", "after_chain", "stop"},
	} {
		if rc, _, stderr := runRoutingVerb(t, inRoot(root, args...)...); rc != 0 {
			t.Fatalf("%v: rc=%d stderr=%s", args, rc, stderr)
		}
	}
	got := *mustReadBlock(t, root)
	want := policy.CLIRouting{
		CLIs: []string{"agy", "agy-claude", "claude"}, Default: []string{"agy", "agy-claude", "claude"},
		Tiers: map[string][]string{"deep": {"agy-claude", "claude"}}, Work: map[string][]string{"build": {"agy", "claude"}},
		Agents:     map[string]policy.AgentRule{"router": {CLI: []string{"agy-claude", "claude"}, Model: "deep"}},
		AfterChain: "stop",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("block = %+v\nwant    %+v", got, want)
	}
}

func mustReadBlock(t *testing.T, root string) *policy.CLIRouting {
	t.Helper()
	pol, _ := readRoutingPolicy(t, root)
	if pol.CLIRouting == nil {
		t.Fatal("no cli_routing block")
	}
	return pol.CLIRouting
}

func TestCLIRoutingSet_AWriteThatWouldNotCompileIsRefusedAndTheFileIsUnchanged(t *testing.T) {
	root := routingWriteProject(t, `{"cli_routing": {"clis": ["agy", "claude"], "default": ["agy", "claude"]}}`)
	_, before := readRoutingPolicy(t, root)

	rc, _, stderr := runRoutingVerb(t, inRoot(root, "set", "clis", "agy")...)

	if _, after := readRoutingPolicy(t, root); rc != exitRoutingFinding || !bytes.Equal(before, after) || !strings.Contains(stderr, "cli_routing.clis") {
		t.Fatalf("rc=%d stderr=%q: a table without claude must never be written", rc, stderr)
	}
}

func TestCLIRoutingUnset_RemovesOneEntry(t *testing.T) {
	root := routingWriteProject(t, `{"cli_routing": {"clis": ["agy", "claude"], "default": ["agy", "claude"], "tiers": {"deep": ["claude"]}}}`)

	rc, _, stderr := runRoutingVerb(t, inRoot(root, "unset", "tiers.deep")...)

	if block := mustReadBlock(t, root); rc != 0 || len(block.Tiers) != 0 || len(block.Default) != 2 {
		t.Fatalf("rc=%d stderr=%q block=%+v", rc, stderr, block)
	}
}

func TestCLIRoutingMigrate_MovesThePinsTheRouterKeysAndTheTailIntoTheBlock(t *testing.T) {
	root := routingWriteProject(t, `{"cli_routing": {"clis": ["agy", "claude"], "default": ["agy", "claude"]},
		"pins": {"auditor": {"cli": "claude", "model": "deep"}},
		"router": {"cli": "claude-tmux", "model": "deep", "replan_depth": 2},
		"workflow": {"universal_fallback": false, "max_cycles_cap": 5}}`)

	if rc, _, stderr := runRoutingVerb(t, inRoot(root, "migrate", "--dry-run")...); rc != 0 || mustReadBlock(t, root).Agents != nil {
		t.Fatalf("rc=%d stderr=%q: a dry run writes nothing", rc, stderr)
	}
	rc, _, stderr := runRoutingVerb(t, inRoot(root, "migrate")...)

	pol, _ := readRoutingPolicy(t, root)
	block := pol.CLIRouting
	if rc != 0 || block.AfterChain != "stop" || !reflect.DeepEqual(block.Agents["auditor"], policy.AgentRule{CLI: []string{"claude"}, Model: "deep"}) ||
		!reflect.DeepEqual(block.Agents["router"], policy.AgentRule{CLI: []string{"claude-tmux"}, Model: "deep"}) {
		t.Fatalf("rc=%d stderr=%q block=%+v", rc, stderr, block)
	}
	if len(pol.Pins) != 0 || pol.Router.CLI != "" || pol.Router.Model != "" || pol.Router.ReplanDepth != 2 || pol.Workflow.UniversalFallback != nil || pol.Workflow.MaxCyclesCap != 5 {
		t.Fatalf("the legacy keys leave and their neighbours stay: pins=%v router=%+v workflow=%+v", pol.Pins, pol.Router, pol.Workflow)
	}
}

func TestCLIRoutingMigrate_RefusesWhatTheTableCannotSay(t *testing.T) {
	for name, tc := range map[string]struct{ body, reason string }{
		"a model-only pin":           {`{"cli_routing": {"clis": ["claude"], "default": ["claude"]}, "pins": {"auditor": {"model": "deep"}}}`, "sets only a model"},
		"per-decision router tiers":  {`{"cli_routing": {"clis": ["claude"], "default": ["claude"]}, "router": {"cli": "claude-tmux", "plan_model": "deep", "propose_model": "fast"}}`, "different tiers"},
		"a lone plan_model":          {`{"cli_routing": {"clis": ["claude"], "default": ["claude"]}, "router": {"cli": "claude-tmux", "plan_model": "deep"}}`, "different tiers"},
		"a router tier with no cli":  {`{"cli_routing": {"clis": ["claude"], "default": ["claude"]}, "router": {"model": "deep"}}`, "without router.cli"},
		"a non-empty tail exclusion": {`{"cli_routing": {"clis": ["claude"], "default": ["claude"]}, "workflow": {"universal_fallback_exclude": ["agy"]}}`, "has no table form"},
		"no block to migrate into":   {`{"pins": {"auditor": {"cli": "claude"}}}`, "init --clis"},
	} {
		t.Run(name, func(t *testing.T) {
			root := routingWriteProject(t, tc.body)
			_, before := readRoutingPolicy(t, root)
			rc, _, stderr := runRoutingVerb(t, inRoot(root, "migrate")...)
			if _, after := readRoutingPolicy(t, root); rc != exitRoutingFinding || !bytes.Equal(before, after) || !strings.Contains(stderr, tc.reason) {
				t.Fatalf("rc=%d stderr=%q: refused for %q with the file unchanged", rc, stderr, tc.reason)
			}
		})
	}
}

func TestCLIRoutingWrite_IsRefusedInsideAPhaseAndWhileACycleRuns(t *testing.T) {
	root := routingWriteProject(t, `{"cli_routing": {"clis": ["agy", "claude"], "default": ["agy", "claude"]}}`)
	_, before := readRoutingPolicy(t, root)
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "1")
	if rc, _, stderr := runRoutingVerb(t, inRoot(root, "set", "default", "claude")...); rc != exitRoutingFinding || !strings.Contains(stderr, "inside a phase") {
		t.Fatalf("rc=%d stderr=%q: a phase agent cannot rewrite its own routing", rc, stderr)
	}
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-7")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if rc, _, stderr := runRoutingVerb(t, inRoot(root, "set", "default", "claude")...); rc != exitRoutingFinding || !strings.Contains(stderr, "cycle-7") {
		t.Fatalf("rc=%d stderr=%q: the table never changes under a running cycle", rc, stderr)
	}
	if _, after := readRoutingPolicy(t, root); !bytes.Equal(before, after) {
		t.Fatal("a refused write leaves the file unchanged")
	}
}

func TestCLIRoutingSet_RefusesAMisshapenEdit(t *testing.T) {
	root := routingWriteProject(t, `{"cli_routing": {"clis": ["agy", "claude"], "default": ["agy", "claude"]}}`)
	_, before := readRoutingPolicy(t, root)
	for _, args := range [][]string{
		{"set", "default", "claude", "--model", "deep"},
		{"set", "after_chain", "stop,other_clis"},
		{"set", "bogus.key", "claude"},
		{"unset", "bogus"},
	} {
		if rc, _, stderr := runRoutingVerb(t, inRoot(root, args...)...); rc != exitRoutingFinding || stderr == "" {
			t.Errorf("%v: rc=%d stderr=%q, want a refusal", args, rc, stderr)
		}
	}
	if _, after := readRoutingPolicy(t, root); !bytes.Equal(before, after) {
		t.Fatal("a refused edit leaves the file unchanged")
	}
}

func TestRoutingProjectRoot_AnUnreadableWorkingDirectoryIsAnErrorNotAnEmptyRoot(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	broken := func() (string, error) { return "", os.ErrPermission }

	root, err := routingProjectRoot("", broken)

	if err == nil || root != "" || !strings.Contains(err.Error(), "--project-root") {
		t.Fatalf("root=%q err=%v, want an error that names --project-root", root, err)
	}
	if root, err := routingProjectRoot("/given", broken); err != nil || root != "/given" {
		t.Fatalf("an explicit root never reads the working directory: %q %v", root, err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", "/from-env")
	if root, err := routingProjectRoot("", broken); err != nil || root != "/from-env" {
		t.Fatalf("EVOLVE_PROJECT_ROOT outranks the working directory: %q %v", root, err)
	}
}
