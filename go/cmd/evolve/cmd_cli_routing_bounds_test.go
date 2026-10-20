package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestCLIRoutingSet_WritesTheLadderBoundsOfAnAgent(t *testing.T) {
	root := routingWriteProject(t, effortTable)
	for _, args := range [][]string{
		{"set", "agents.router", "agy-claude,claude", "--model", "deep", "--max-tier", "top", "--max-effort", "xhigh"},
		{"set", "agents.auditor", "--max-tier", "top"},
	} {
		if rc, _, stderr := runRoutingVerb(t, inRoot(root, args...)...); rc != 0 {
			t.Fatalf("%v: rc=%d stderr=%s", args, rc, stderr)
		}
	}
	pol, _ := readRoutingPolicy(t, root)
	want := map[string]policy.AgentRule{
		"router":  {CLI: []string{"agy-claude", "claude"}, Model: "deep", MaxTier: "top", MaxEffort: "xhigh"},
		"auditor": {MaxTier: "top"},
	}
	if !reflect.DeepEqual(pol.CLIRouting.Agents, want) {
		t.Fatalf("agents = %+v, want %+v", pol.CLIRouting.Agents, want)
	}
}

func TestCLIRoutingSet_RefusesABadLadderBoundBeforeTheWrite(t *testing.T) {
	root := routingWriteProject(t, effortTable)
	_, before := readRoutingPolicy(t, root)
	wantRefusal := map[string]string{"huge": `unknown tier "huge"`, "turbo": `unknown effort "turbo"`, "deep": "apply only to agents.<agent>"}
	for _, args := range [][]string{
		{"set", "agents.router", "--max-tier", "huge"},
		{"set", "agents.router", "--max-effort", "turbo"},
		{"set", "tiers.deep", "--max-tier", "deep"},
	} {
		rc, _, stderr := runRoutingVerb(t, inRoot(root, args...)...)
		if _, after := readRoutingPolicy(t, root); rc == 0 || !bytes.Equal(before, after) {
			t.Errorf("%v: rc=%d stderr=%q, want a refusal and no write", args, rc, stderr)
		}
		if !strings.Contains(stderr, wantRefusal[args[len(args)-1]]) {
			t.Errorf("%v: stderr %q does not name the refusal %q", args, stderr, wantRefusal[args[len(args)-1]])
		}
	}
}

func TestUnsetEffort_KeepsAnEntryThatStillHoldsLadderBounds(t *testing.T) {
	t.Parallel()
	block := policy.CLIRouting{Agents: map[string]policy.AgentRule{"builder": {Effort: "high", MaxTier: "top"}}}

	got, err := unsetEffort(block, "agents", "builder")
	if err != nil {
		t.Fatal(err)
	}
	if rule, ok := got.Agents["builder"]; !ok || !reflect.DeepEqual(rule, policy.AgentRule{MaxTier: "top"}) {
		t.Fatalf("agents = %+v, want builder kept with max_tier top", got.Agents)
	}
}
