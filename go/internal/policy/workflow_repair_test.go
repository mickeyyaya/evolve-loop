package policy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFixabilityConfig_Enforced(t *testing.T) {
	cTrue := FixabilityConfig{Stage: FixabilityEnforce}
	if !cTrue.Enforced() {
		t.Fatal("expected enforced for FixabilityEnforce")
	}
	cFalse := FixabilityConfig{Stage: FixabilityShadow}
	if cFalse.Enforced() {
		t.Fatal("did not expect enforced for FixabilityShadow")
	}
	cOff := FixabilityConfig{Stage: FixabilityOff}
	if cOff.Enforced() {
		t.Fatal("did not expect enforced for FixabilityOff")
	}
}

func TestPolicy_RepairConfig(t *testing.T) {
	var pNil Policy
	cfgNil := pNil.repairConfig()
	if cfgNil.Fixability.Stage != FixabilityShadow {
		t.Fatalf("got %q, want %q", cfgNil.Fixability.Stage, FixabilityShadow)
	}

	pNoRepair := Policy{Workflow: &WorkflowPolicy{}}
	cfgNoRepair := pNoRepair.repairConfig()
	if cfgNoRepair.Fixability.Stage != FixabilityShadow {
		t.Fatalf("got %q, want %q", cfgNoRepair.Fixability.Stage, FixabilityShadow)
	}

	pNoFixability := Policy{Workflow: &WorkflowPolicy{Repair: &RepairPolicy{}}}
	cfgNoFixability := pNoFixability.repairConfig()
	if cfgNoFixability.Fixability.Stage != FixabilityShadow {
		t.Fatalf("got %q, want %q", cfgNoFixability.Fixability.Stage, FixabilityShadow)
	}

	pCustom := Policy{Workflow: &WorkflowPolicy{Repair: &RepairPolicy{Fixability: &FixabilityPolicy{Stage: FixabilityEnforce}}}}
	cfgCustom := pCustom.repairConfig()
	if cfgCustom.Fixability.Stage != FixabilityEnforce {
		t.Fatalf("got %q, want %q", cfgCustom.Fixability.Stage, FixabilityEnforce)
	}
}

func TestWorkflowPolicy_ValidateRepair(t *testing.T) {
	var wNil *WorkflowPolicy
	if err := wNil.validateRepair(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wNoRepair := &WorkflowPolicy{}
	if err := wNoRepair.validateRepair(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wValid := &WorkflowPolicy{Repair: &RepairPolicy{Fixability: &FixabilityPolicy{Stage: FixabilityShadow}}}
	if err := wValid.validateRepair(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wInvalid := &WorkflowPolicy{Repair: &RepairPolicy{Fixability: &FixabilityPolicy{Stage: "invalid"}}}
	if err := wInvalid.validateRepair(); err == nil {
		t.Fatal("expected error for invalid stage")
	}
}

func TestRepairPolicy_Validate(t *testing.T) {
	var rNil *RepairPolicy
	if err := rNil.validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rNoFixability := &RepairPolicy{}
	if err := rNoFixability.validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rEmpty := &RepairPolicy{Fixability: &FixabilityPolicy{Stage: ""}}
	if err := rEmpty.validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, stage := range []string{FixabilityOff, FixabilityShadow, FixabilityEnforce} {
		r := &RepairPolicy{Fixability: &FixabilityPolicy{Stage: stage}}
		if err := r.validate(); err != nil {
			t.Fatalf("unexpected error for %q: %v", stage, err)
		}
	}

	rBad := &RepairPolicy{Fixability: &FixabilityPolicy{Stage: "unknown"}}
	if err := rBad.validate(); err == nil {
		t.Fatal("expected error for unknown stage")
	}
}

func TestParse_WorkflowRepairInvalidStageReturnsError(t *testing.T) {
	raw := []byte(`{"workflow": {"repair": {"fixability": {"stage": "invalid"}}}}`)
	_, err := Parse(raw)
	if err == nil {
		t.Fatal("expected error from Parse for invalid repair fixability stage")
	}
}

func TestWorkflowRepair_ExportTypesAndConstants(t *testing.T) {
	var _ RepairPolicy = RepairPolicy{Fixability: &FixabilityPolicy{Stage: FixabilityOff}}
	var _ FixabilityPolicy = FixabilityPolicy{Stage: FixabilityShadow}
	var _ RepairConfig = RepairConfig{Fixability: FixabilityConfig{Stage: FixabilityEnforce}}
	var fc FixabilityConfig
	if fc.Enforced() {
		t.Fatal("zero FixabilityConfig should not be enforced")
	}
}

func TestFixabilityFor_ReadsTheStageAndHoldsShadowOnAnUnreadablePolicy(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	var warn strings.Builder
	if got := FixabilityFor(root, "triage", &warn); got.Stage != FixabilityShadow || warn.Len() != 0 {
		t.Fatalf("no policy file: %+v, warn %q; want shadow and no warning", got, warn.String())
	}
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"workflow":{"repair":{"fixability":{"stage":"enforce"}}}}`)
	if got := FixabilityFor(root, "triage", &warn); !got.Enforced() {
		t.Fatalf("enforce policy: %+v, want enforced", got)
	}
	write(`{"workflow":{"repair":{"fixability":{"stage":"on"}}}}`)
	got := FixabilityFor(root, "census", &warn)
	if got.Stage != FixabilityShadow || !strings.Contains(warn.String(), "[census] WARN policy: workflow.repair.fixability") {
		t.Fatalf("a refused policy: %+v, warn %q; want shadow and a WARN that names the caller", got, warn.String())
	}
}
