package audit

import (
	"errors"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsverdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

var overridingGateRE = regexp.MustCompile(`forced FAIL \[([^\]]+)\]`)

var checkSeamGates = []struct {
	seam string
	wire func(*hooks)
}{
	{"gofmtCheck", func(h *hooks) { h.gofmtCheck = offenders("internal/x/x.go") }},
	{"solutionCheck", func(h *hooks) { h.solutionCheck = offenders("missing section: Decision") }},
	{"skillsDriftCheck", func(h *hooks) { h.skillsDriftCheck = offenders("commands/audit.md") }},
	{"goVetCheck", func(h *hooks) { h.goVetCheck = offenders("internal/foo: import cycle") }},
	{"acsDurableCheck", func(h *hooks) { h.acsDurableCheck = offenders("flag-ceiling") }},
	{"integrationTierCheck", func(h *hooks) { h.integrationTierCheck = offenders("TestFleetSoak") }},
	{"apicoverEnforceCheck", func(h *hooks) { h.apicoverEnforceCheck = offenders("internal/bar:12") }},
	{"apicoverNewPkgGraduationCheck", func(h *hooks) { h.apicoverNewPkgGraduationCheck = offenders("internal/baz") }},
}

func classifyPASSNarrative(t *testing.T, h hooks, ws string) []core.Diagnostic {
	t.Helper()
	verdict, diags, _ := h.Classify(narrativeReport("PASS"), core.PhaseRequest{Workspace: ws}, core.BridgeResponse{})
	if verdict != core.VerdictFAIL {
		t.Fatalf("verdict=%q, want the gate to force FAIL; diags=%+v", verdict, diags)
	}
	return diags
}

func requireGateClassified(t *testing.T, diags []core.Diagnostic) {
	t.Helper()
	conflict := requireConflict(t, diags, "PASS")
	match := overridingGateRE.FindStringSubmatch(conflict)
	if match == nil {
		t.Fatalf("conflict record names no overriding gate: %s", conflict)
	}
	var reasons []string
	for _, d := range diags {
		if d.Severity == "error" && d.Message != conflict {
			reasons = append(reasons, d.Message)
		}
	}
	if len(reasons) != 1 {
		t.Fatalf("want exactly one gate diagnosis beside the conflict record, got %q", reasons)
	}
	if got := core.AuditGateOf(reasons[0]); got != match[1] {
		t.Errorf("core classifies the %q gate's diagnosis as gate %q:\n%s", match[1], got, reasons[0])
	}
}

func TestAuditGateRemedies_ClassifyEveryGateProducer(t *testing.T) {
	seamType := reflect.TypeOf((func(core.PhaseRequest) ([]string, error))(nil))
	var seams, walked []string
	for _, f := range reflect.VisibleFields(reflect.TypeOf(hooks{})) {
		if f.Type == seamType {
			seams = append(seams, f.Name)
		}
	}
	for _, g := range checkSeamGates {
		walked = append(walked, g.seam)
	}
	slices.Sort(seams)
	slices.Sort(walked)
	if !slices.Equal(seams, walked) {
		t.Fatalf("hooks check seams %v, walked %v: a gate added to the audit needs a row in core's gate table and a walk here", seams, walked)
	}

	for _, g := range checkSeamGates {
		t.Run(g.seam, func(t *testing.T) {
			var h hooks
			g.wire(&h)
			ws := t.TempDir()
			yes := true
			writeACSVerdictShip(t, ws, 0, &yes)
			requireGateClassified(t, classifyPASSNarrative(t, h, ws))
		})
	}
	t.Run("EGPS red_count>0", func(t *testing.T) {
		ws := t.TempDir()
		writeACSVerdictReds(t, ws, "cycle1828/TestC1828_001_Red")
		requireGateClassified(t, classifyPASSNarrative(t, hooks{}, ws))
	})
}

func TestAuditGateRemedies_HostAndBookkeepingFailuresAreNoGateDiagnosis(t *testing.T) {
	no := false
	cases := []struct {
		name     string
		h        hooks
		artifact string
		prep     func(t *testing.T, ws string)
	}{
		{"EGPS ship_eligible=false", hooks{}, narrativeReport("PASS"), func(t *testing.T, ws string) { writeACSVerdictShip(t, ws, 0, &no) }},
		{"acs-verdict.json unreadable", hooks{}, narrativeReport("PASS"), func(*testing.T, string) {}},
		{"host predicate execution", hooks{genVerdict: func(core.PhaseRequest) error { return errors.New("go: command not found") }}, narrativeReport("PASS"), func(*testing.T, string) {}},
		{"EGPS reds the harness could not run", hooks{}, narrativeReport("PASS"), func(t *testing.T, ws string) {
			writeACSVerdictReds(t, ws, acsverdict.SyntheticRedPrefix+"cycle9001/TestC9001_001", acsverdict.SyntheticRedPrefix+"cycle9001/TestC9001_002")
		}},
		{"closure claim", hooks{}, narrativeReport("PASS") + "\nResolved: the cycle-1424 defect is closed.\n", func(t *testing.T, ws string) {
			yes := true
			writeACSVerdictShip(t, ws, 0, &yes)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			tc.prep(t, ws)
			verdict, diags, _ := tc.h.Classify(tc.artifact, core.PhaseRequest{Workspace: ws}, core.BridgeResponse{})
			if verdict != core.VerdictFAIL {
				t.Fatalf("verdict=%q, want FAIL; diags=%+v", verdict, diags)
			}
			for _, d := range diags {
				if gate := core.AuditGateOf(d.Message); d.Severity == "error" && gate != "" {
					t.Errorf("%s classified as gate %q; only a rebuild-repairable gate earns the derived class and the floor overrule:\n%s", tc.name, gate, d.Message)
				}
			}
		})
	}
}

func TestAuditGateRemedies_ProducersRenderTheTableRemedy(t *testing.T) {
	cases := []struct {
		gate string
		wire func(*hooks)
	}{
		{"gofmt", func(h *hooks) { h.gofmtCheck = offenders("internal/x/x.go") }},
		{"solution-contract", func(h *hooks) { h.solutionCheck = offenders("missing section: Decision") }},
		{"skills-drift", func(h *hooks) { h.skillsDriftCheck = offenders("commands/audit.md") }},
		{"apicover new-package graduation gate", func(h *hooks) { h.apicoverNewPkgGraduationCheck = offenders("internal/baz") }},
	}
	for _, tc := range cases {
		t.Run(tc.gate, func(t *testing.T) {
			var h hooks
			tc.wire(&h)
			ws := t.TempDir()
			yes := true
			writeACSVerdictShip(t, ws, 0, &yes)
			remedy := core.AuditGateRemedy(tc.gate)
			if remedy == "" {
				t.Fatalf("core's gate table has no remedy for %q", tc.gate)
			}

			diags := classifyPASSNarrative(t, h, ws)

			for _, d := range diags {
				if core.AuditGateOf(d.Message) == tc.gate {
					if !strings.Contains(d.Message, remedy) {
						t.Errorf("the %q diagnosis states a remedy other than core's table row %q:\n%s", tc.gate, remedy, d.Message)
					}
					return
				}
			}
			t.Fatalf("no %q diagnosis among %+v", tc.gate, diags)
		})
	}
}
