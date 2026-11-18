package phasecmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

const ghostTriggerSpec = `{"name":"late-check","kind":"llm","optional":true,"archetype":"evaluate","after":"build",
"routing":{"insert_when":[{"field":"ghost.signal","op":"==","value":"x"}],"rubric_hint":["r"]}}`

func TestPhasesValidate_RefusesAnUnproducedTriggerField(t *testing.T) {
	root := newPhasesProject(t)
	writeUserPhaseFile(t, root, "late-check", ghostTriggerSpec)
	var out, errb bytes.Buffer
	code := RunPhases([]string{"validate"}, strings.NewReader(""), &out, &errb)
	if code != 2 {
		t.Fatalf("exit %d, want 2; out=%s", code, out.String())
	}
	if !strings.Contains(out.String(), "late-check") || !strings.Contains(out.String(), "ghost.signal") {
		t.Fatalf("output must name the phase and the field: %s", out.String())
	}
}

func TestUnproducedTriggerRefusals_NoRegistryRefusesNothing(t *testing.T) {
	got := unproducedTriggerRefusals(t.TempDir(), []phasespec.PhaseSpec{{Name: "x"}}, nil)
	if len(got) != 0 {
		t.Fatalf("a project without a registry has no routing to check, got %q", got)
	}
}

func TestPhasesValidate_ChecksOnlyTheNamedPhase(t *testing.T) {
	root := newPhasesProject(t)
	writeUserPhaseFile(t, root, "late-check", ghostTriggerSpec)
	writeUserPhaseFile(t, root, "plain-check", `{"name":"plain-check","kind":"llm","optional":true,"archetype":"evaluate","after":"build"}`)
	var out, errb bytes.Buffer
	if code := RunPhases([]string{"validate", "plain-check"}, strings.NewReader(""), &out, &errb); code != 0 {
		t.Fatalf("exit %d, want 0; out=%s", code, out.String())
	}
}
