package core

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func documentContractUnderTddRule(t *testing.T, rule config.CondRule) string {
	t.Helper()
	dir := t.TempDir()
	item := writeItem(t, dir, "netflix-margin", `{"id":"netflix-margin","title":"Netflix margin","deliverable_kind":"document","acceptance":["routing plan justifies tdd:false"]}`)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	o.cfg.DeliverableKinds = map[string]config.DeliverableKindSpec{config.DeliverableKindDocument: documentSpec()}
	o.cfg.Conditional = map[string]config.CondRule{"tdd": rule}
	cs := CycleState{CycleID: 1692, WorkspacePath: documentWorkspace(t, "document")}
	out := o.seedTaskContract(context.Background(), map[string]string{"fleet_scope_paths": "netflix-margin=" + item}, PhaseAudit, cs, dir)
	return out[CtxKeyTaskContract]
}

func TestSeedTaskContract_DocumentCycleStatesTddFromTheRegistryRule(t *testing.T) {
	block := documentContractUnderTddRule(t, config.DefaultTddRule())
	for _, want := range []string{
		"TDD: not required for this cycle",
		"conditional_mandatory.tdd = " + config.DefaultTddRuleExpr,
		"routing decides whether tdd runs, not this item's acceptance",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("document contract lacks %q:\n%s", want, block)
		}
	}
}

func TestSeedTaskContract_ARegistryThatPinsTddForDocumentsSaysSo(t *testing.T) {
	block := documentContractUnderTddRule(t, config.CondRule{Field: "cycle_size", Op: "!=", Value: "trivial"})
	if !strings.Contains(block, "TDD: required for this cycle (phase-registry.json conditional_mandatory.tdd = cycle_size!=trivial)") {
		t.Errorf("the tdd line must follow the registry's rule, not a fixed sentence:\n%s", block)
	}
}
