package main

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

var acceptedCheckedInWarnings = map[string]bool{
	"cross_family_with.auditor+builder":      true,
	"cross_family_with.builder+tdd-engineer": true,
}

func checkedInRouter(t *testing.T) (*cliroute.Router, []cliroute.Finding, string) {
	t.Helper()
	_, root := checkedInPolicy(t)
	r, findings, err := buildCLIRouter(root, routingCatalog(root), cliroute.Host{LookPath: everyBinaryPresent})
	if err != nil {
		t.Fatalf("the checked-in table does not compile: %v", err)
	}
	return r, findings, root
}

func TestTheCheckedInTableCompiles(t *testing.T) {
	if pol, _ := checkedInPolicy(t); pol.CLIRouting == nil {
		t.Fatal("the checked-in policy declares no cli_routing table")
	}
	_, findings, _ := checkedInRouter(t)
	for _, f := range findings {
		if f.Severity != cliroute.SeverityWarn || !acceptedCheckedInWarnings[f.Key] {
			t.Errorf("unaccepted finding %s %s: %s", f.Severity, f.Key, f.Message)
		}
	}
}

func TestTheCheckedInTableRoutesNoLaunchToCodex(t *testing.T) {
	r, _, root := checkedInRouter(t)
	agents, err := profiles.NewFromDir(routingProfilesDir(root)).List()
	if err != nil {
		t.Fatal(err)
	}
	catalog := routingCatalog(root)
	var requests []cliroute.Request
	for _, agent := range agents {
		for _, tier := range []string{"", "fast", "balanced", "deep", "top"} {
			requests = append(requests, cliroute.Request{Agent: agent, Phase: agentPhase(agent, catalog), ProjectRoot: root, DefaultModel: unsetDispatchTier, Overlay: llmroute.Overlay{Tier: tier}})
		}
	}
	requests = append(requests,
		cliroute.Request{Agent: routerAgent, Launch: cliroute.LaunchAdvisor, ProjectRoot: root, DefaultModel: routerDefaultModel},
		cliroute.Request{Agent: cliroute.ClassifierAgent, Launch: cliroute.LaunchClassifier, ProjectRoot: root, DefaultModel: unsetDispatchTier})
	for _, req := range requests {
		d, err := r.Resolve(req)
		if err != nil {
			t.Errorf("%s at %q: %v", req.Agent, req.Overlay.Tier, err)
			continue
		}
		for _, cli := range d.Plan.Candidates {
			if llmroute.Family(cli) == "codex" {
				t.Errorf("%s at %q routes to %s (chain %v): codex has no subscription", req.Agent, req.Overlay.Tier, cli, d.Plan.Candidates)
			}
		}
	}
}
