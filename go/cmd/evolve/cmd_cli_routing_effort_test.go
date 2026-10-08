package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const effortTable = `{"cli_routing": {"clis": ["agy", "agy-claude", "claude"], "default": ["agy", "agy-claude", "claude"], "tiers": {"deep": ["agy-claude", "claude"]}}}`

func TestCLIRoutingSet_EffortWithoutAChainKeepsTheChain(t *testing.T) {
	root := routingWriteProject(t, effortTable)
	for _, args := range [][]string{
		{"set", "tiers.deep", "--effort", "high"},
		{"set", "tiers.fast", "--effort", "low"},
		{"set", "agents.router", "--effort", "low"},
		{"set", "agents.auditor", "claude", "--model", "deep", "--effort", "xhigh"},
	} {
		if rc, _, stderr := runRoutingVerb(t, inRoot(root, args...)...); rc != 0 {
			t.Fatalf("%v: rc=%d stderr=%s", args, rc, stderr)
		}
	}
	pol, _ := readRoutingPolicy(t, root)
	wantTiers := map[string]policy.TierRule{
		"deep": {CLIs: []string{"agy-claude", "claude"}, Effort: "high"},
		"fast": {Effort: "low"},
	}
	if !reflect.DeepEqual(pol.CLIRouting.Tiers, wantTiers) {
		t.Errorf("tiers = %+v, want %+v", pol.CLIRouting.Tiers, wantTiers)
	}
	wantAgents := map[string]policy.AgentRule{
		"router":  {Effort: "low"},
		"auditor": {CLI: []string{"claude"}, Model: "deep", Effort: "xhigh"},
	}
	if !reflect.DeepEqual(pol.CLIRouting.Agents, wantAgents) {
		t.Errorf("agents = %+v, want %+v", pol.CLIRouting.Agents, wantAgents)
	}
}

func TestCLIRoutingUnset_TheEffortKeyClearsOnlyTheEffort(t *testing.T) {
	root := routingWriteProject(t, `{"cli_routing": {"clis": ["agy", "agy-claude", "claude"], "default": ["agy", "agy-claude", "claude"], "tiers": {"deep": {"clis": ["agy-claude", "claude"], "effort": "high"}, "fast": {"effort": "low"}}}}`)
	for _, args := range [][]string{{"unset", "tiers.deep.effort"}, {"unset", "tiers.fast.effort"}} {
		if rc, _, stderr := runRoutingVerb(t, inRoot(root, args...)...); rc != 0 {
			t.Fatalf("%v: rc=%d stderr=%s", args, rc, stderr)
		}
	}
	pol, raw := readRoutingPolicy(t, root)
	want := map[string]policy.TierRule{"deep": {CLIs: []string{"agy-claude", "claude"}}}
	if !reflect.DeepEqual(pol.CLIRouting.Tiers, want) {
		t.Errorf("tiers = %+v, want %+v", pol.CLIRouting.Tiers, want)
	}
	if !strings.Contains(string(raw), `"deep": [`) {
		t.Errorf("a chain-only tier must write back in the list form:\n%s", raw)
	}
}

func TestCLIRoutingSet_RefusesABadEffortBeforeTheWrite(t *testing.T) {
	root := routingWriteProject(t, effortTable)
	_, before := readRoutingPolicy(t, root)
	for _, args := range [][]string{
		{"set", "tiers.deep", "--effort", "hihg"},
		{"set", "tiers.galactic", "--effort", "low"},
		{"set", "agents.nobody", "--effort", "low"},
		{"set", "default", "claude", "--effort", "low"},
	} {
		rc, _, stderr := runRoutingVerb(t, inRoot(root, args...)...)
		if _, after := readRoutingPolicy(t, root); rc == 0 || !bytes.Equal(before, after) {
			t.Errorf("%v: rc=%d stderr=%q, want a refusal and no write", args, rc, stderr)
		}
	}
}

func TestCLIRoutingExplain_PrintsTheEffortAndItsSource(t *testing.T) {
	root := routingWriteProject(t, `{"cli_routing": {"clis": ["agy", "agy-claude", "claude"], "default": ["agy", "agy-claude", "claude"], "tiers": {"deep": {"clis": ["agy-claude", "claude"], "effort": "high"}}, "agents": {"auditor": {"effort": "xhigh"}}}}`)
	rc, stdout, stderr := runRoutingVerb(t, inRoot(root, "explain", "router", "--static")...)
	if rc != 0 || !strings.Contains(stdout, "effort high (cli_routing.tiers.deep)") {
		t.Fatalf("explain router rc=%d stdout=%s stderr=%s, want the deep effort from the routing table", rc, stdout, stderr)
	}
	rc, stdout, _ = runRoutingVerb(t, inRoot(root, "show", "--static")...)
	if rc != 0 || !strings.Contains(stdout, "effort xhigh (cli_routing.agents.auditor)") || !strings.Contains(stdout, "effort high (cli_routing.tiers.deep)") {
		t.Fatalf("show rc=%d stdout=%s, want each effort with its source", rc, stdout)
	}
}

func TestCLIRoutingMigrate_MovesProfileEffortIntoTheTableAndIsIdempotent(t *testing.T) {
	root := routingWriteProject(t, effortTable)
	dir := routingProfilesDir(root)
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("router", "{\n  \"name\": \"router\",\n  \"cli\": \"agy-claude-tmux\",\n  \"allowed_clis\": [\"agy-claude\", \"claude\"],\n  \"model_tier_default\": \"deep\",\n  \"effort_level\": \"medium\"\n}\n")
	write("auditor", "{\n  \"name\": \"auditor\",\n  \"effort_level\": \"low\",\n  \"cli\": \"claude-tmux\",\n  \"allowed_clis\": [\"claude\"],\n  \"model_tier_default\": \"deep\"\n}\n")

	rc, stdout, stderr := runRoutingVerb(t, inRoot(root, "migrate", "--dry-run")...)
	if rc != 0 || !strings.Contains(stdout, "auditor.json") {
		t.Fatalf("dry-run rc=%d stdout=%s stderr=%s, want the profile rewrites listed", rc, stdout, stderr)
	}
	if raw, _ := os.ReadFile(filepath.Join(dir, "auditor.json")); !strings.Contains(string(raw), "effort_level") {
		t.Fatal("--dry-run rewrote a profile")
	}

	for run := 1; run <= 2; run++ {
		if rc, _, stderr := runRoutingVerb(t, inRoot(root, "migrate")...); rc != 0 {
			t.Fatalf("migrate run %d: rc=%d stderr=%s", run, rc, stderr)
		}
	}
	pol, _ := readRoutingPolicy(t, root)
	want := map[string]policy.AgentRule{"auditor": {Effort: "low"}}
	if !reflect.DeepEqual(pol.CLIRouting.Agents, want) {
		t.Errorf("agents = %+v, want only the auditor's non-default low; router's medium equals its deep default", pol.CLIRouting.Agents)
	}
	for _, name := range []string{"router", "auditor"} {
		raw, err := os.ReadFile(filepath.Join(dir, name+".json"))
		if err != nil || strings.Contains(string(raw), "effort") || !strings.Contains(string(raw), `"name": "`+name+`"`) {
			t.Errorf("%s.json = %s (%v), want the effort key gone and the rest kept", name, raw, err)
		}
	}
}

func TestCLIRoutingMigrate_RefusesAnOverrideWithNoTableForm(t *testing.T) {
	root := routingWriteProject(t, effortTable)
	path := filepath.Join(routingProfilesDir(root), "auditor.json")
	body := `{"name":"auditor","cli":"claude-tmux","allowed_clis":["claude"],"model_tier_default":"deep","effort_overrides":{"top":"max"}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, _, stderr := runRoutingVerb(t, inRoot(root, "migrate")...)
	if raw, _ := os.ReadFile(path); rc == 0 || !strings.Contains(stderr, "effort_overrides.top") || string(raw) != body {
		t.Fatalf("rc=%d stderr=%q, want a refusal naming effort_overrides.top and the profile unchanged", rc, stderr)
	}
}
