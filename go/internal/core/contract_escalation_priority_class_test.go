package core

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
)

func TestContractGateDemotion_IntentIsACorrectnessClassedConsoleAutofile(t *testing.T) {
	in := ContractGateDemotion{Phase: PhaseBuild, CLI: "codex-tmux", Cycle: 42, Blocks: 3, Weight: 0.75, Reason: "the gate demoted itself"}.Intent()
	if in.PriorityClass != "correctness" {
		t.Errorf("a gate that stopped judging deliverables is a correctness defect; priority_class = %q", in.PriorityClass)
	}
	if in.Action != dispositionrouter.ActionAutofile || in.Route != dispositionrouter.RouteConsole {
		t.Errorf("the demotion is staged as a console autofile: %+v", in)
	}
	if in.ItemID != "contract-gate-demoted-build" || in.Cycle != 42 || in.Recurrence != 3 || in.Weight != 0.75 || in.Reason != "the gate demoted itself" {
		t.Errorf("the intent carries the demotion's facts: %+v", in)
	}
	if !strings.Contains(in.Pattern, "phase build") || !strings.Contains(in.Pattern, "cli=codex-tmux") {
		t.Errorf("the pattern names the phase and the CLI: %q", in.Pattern)
	}
}
