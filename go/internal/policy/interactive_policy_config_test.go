package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInteractivePolicy_DefaultAndOverride(t *testing.T) {
	if got := (Policy{}).WorkflowConfig().InteractivePolicy; got != "recommended_or_first" {
		t.Errorf("absent workflow block: WorkflowConfig().InteractivePolicy = %v, want recommended_or_first", got)
	}

	p := Policy{Workflow: &WorkflowPolicy{InteractivePolicy: "escalate", InteractivePolicies: map[string]string{"scout": "auto_yes"}}}
	cfg := p.WorkflowConfig()
	if got := cfg.InteractivePolicy; got != "escalate" {
		t.Errorf("WorkflowConfig().InteractivePolicy = %v, want escalate", got)
	}
	if got := cfg.InteractivePolicies["scout"]; got != "auto_yes" {
		t.Errorf("WorkflowConfig().InteractivePolicies[\"scout\"] = %v, want auto_yes", got)
	}
}

func TestInteractivePolicyFor_LoadsFromDisk(t *testing.T) {
	dir := t.TempDir()

	if got := InteractivePolicyFor(dir, "builder"); got != "recommended_or_first" {
		t.Errorf("absent policy.json: InteractivePolicyFor = %v, want recommended_or_first", got)
	}

	evolveDir := filepath.Join(dir, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	policyPath := filepath.Join(evolveDir, "policy.json")

	if err := os.WriteFile(policyPath, []byte(`{"workflow":{"interactive_policy":"escalate","interactive_policies":{"scout":"auto_yes"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := InteractivePolicyFor(dir, "builder"); got != "escalate" {
		t.Errorf("InteractivePolicyFor (builder) = %v, want escalate (fallback to global)", got)
	}
	if got := InteractivePolicyFor(dir, "scout"); got != "auto_yes" {
		t.Errorf("InteractivePolicyFor (scout) = %v, want auto_yes (per-agent override)", got)
	}
	if got := InteractivePolicyFor(dir, ""); got != "escalate" {
		t.Errorf("InteractivePolicyFor (empty agent) = %v, want escalate", got)
	}

	if err := os.WriteFile(policyPath, []byte(`{not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := InteractivePolicyFor(dir, "scout"); got != "recommended_or_first" {
		t.Errorf("InteractivePolicyFor (malformed policy) = %v, want recommended_or_first (fail-open)", got)
	}
}
