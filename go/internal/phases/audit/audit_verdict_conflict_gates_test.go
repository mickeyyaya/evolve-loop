package audit

import (
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func offenders(names ...string) func(core.PhaseRequest) ([]string, error) {
	return func(core.PhaseRequest) ([]string, error) { return names, nil }
}

func cannotRun(msg string) func(core.PhaseRequest) ([]string, error) {
	return func(core.PhaseRequest) ([]string, error) { return nil, errors.New(msg) }
}

func classifyGates(t *testing.T, h hooks, artifact string) (string, []core.Diagnostic) {
	t.Helper()
	ws := t.TempDir()
	yes := true
	writeACSVerdictShip(t, ws, 0, &yes)
	verdict, diags, _ := h.Classify(artifact, core.PhaseRequest{Workspace: ws}, core.BridgeResponse{})
	return verdict, diags
}

var nonEGPSGates = []struct {
	name  string
	wire  func(*hooks)
	inMsg string
}{
	{"gofmt", func(h *hooks) { h.gofmtCheck = offenders("acs/cycle1127/predicates_test.go") }, "gofmt"},
	{"skills-drift", func(h *hooks) { h.skillsDriftCheck = offenders("skills/evolve-auditor/SKILL.md") }, "drift"},
	{"go-vet", func(h *hooks) { h.goVetCheck = offenders("internal/foo: import cycle") }, "vet"},
	{"acs-durable", func(h *hooks) { h.acsDurableCheck = offenders("flag-ceiling") }, "acs-durable"},
	{"integration-tier", func(h *hooks) { h.integrationTierCheck = offenders("TestFleetSoak") }, "integration"},
	{"apicover-enforce", func(h *hooks) { h.apicoverEnforceCheck = offenders("internal/bar:12") }, "apicover"},
	{"apicover-newpkg", func(h *hooks) { h.apicoverNewPkgGraduationCheck = offenders("internal/baz") }, "apicover"},
}

func TestVerdictConflict_EveryNonEGPSGateRecordsTheConflict(t *testing.T) {
	for _, g := range nonEGPSGates {
		t.Run(g.name, func(t *testing.T) {
			var h hooks
			g.wire(&h)
			verdict, diags := classifyGates(t, h, narrativeReport("PASS"))
			if verdict != core.VerdictFAIL {
				t.Fatalf("verdict=%q, want FAIL — the %s gate must still outrank the narrative", verdict, g.name)
			}
			if !hasDiagContaining(diags, g.inMsg) {
				t.Fatalf("test wiring bug: the %s gate diagnostic (%q) never fired; diags=%+v", g.name, g.inMsg, diags)
			}
			msg := requireConflict(t, diags, "PASS")
			if !strings.Contains(msg, "verdict-conflict:") {
				t.Errorf("conflict record is not prefixed `verdict-conflict:`: %s", msg)
			}
		})
	}
}

func TestVerdictConflict_NonEGPSGate_NarrativeWARN(t *testing.T) {
	h := hooks{gofmtCheck: offenders("main.go")}
	_, passDiags := classifyGates(t, h, narrativeReport("PASS"))
	_, warnDiags := classifyGates(t, h, narrativeReport("WARN"))
	pass := requireConflict(t, passDiags, "PASS")
	warn := requireConflict(t, warnDiags, "WARN")
	if pass == warn {
		t.Errorf("narrative PASS and WARN produced identical conflict records on the gofmt gate: %s", pass)
	}
}

func TestVerdictConflict_MultipleGatesStillOneRecord(t *testing.T) {
	h := hooks{
		gofmtCheck:       offenders("main.go"),
		skillsDriftCheck: offenders("skills/x/SKILL.md"),
		goVetCheck:       offenders("internal/foo: import cycle"),
	}
	ws := t.TempDir()
	writeACSVerdictReds(t, ws, "cycle1127/TestC1127_001_Red")
	verdict, diags, _ := h.Classify(narrativeReport("PASS"), core.PhaseRequest{Workspace: ws}, core.BridgeResponse{})
	if verdict != core.VerdictFAIL {
		t.Fatalf("verdict=%q, want FAIL", verdict)
	}
	if got := conflictDiags(diags); len(got) != 1 {
		t.Errorf("want exactly 1 conflict record for a Classify call with 4 firing gates, got %d: %v", len(got), got)
	}
}

func TestVerdictConflict_NonEGPSGate_NoNoiseWhenNarrativeFAIL(t *testing.T) {
	for _, g := range nonEGPSGates {
		t.Run(g.name, func(t *testing.T) {
			var h hooks
			g.wire(&h)
			_, diags := classifyGates(t, h, narrativeReport("FAIL"))
			if got := conflictDiags(diags); len(got) != 0 {
				t.Errorf("emitted %d conflict record(s) on the COHERENT narrative-FAIL case: %v", len(got), got)
			}
		})
	}
}

func TestVerdictConflict_NonEGPSGate_NoNoiseWhenNarrativeUnparseable(t *testing.T) {
	h := hooks{gofmtCheck: offenders("main.go")}
	_, diags := classifyGates(t, h, "# Audit Report\n\nprose with no verdict declaration\n")
	if got := conflictDiags(diags); len(got) != 0 {
		t.Errorf("emitted %d conflict record(s) with no parseable narrative: %v", len(got), got)
	}
}

func TestVerdictConflict_GateCouldNotRun_NoConflict(t *testing.T) {
	h := hooks{
		gofmtCheck:                    cannotRun("gofmt: executable file not found"),
		skillsDriftCheck:              cannotRun("registry load failed"),
		goVetCheck:                    cannotRun("go: not in PATH"),
		acsDurableCheck:               cannotRun("build failed"),
		integrationTierCheck:          cannotRun("timeout"),
		apicoverEnforceCheck:          cannotRun("apicover missing"),
		apicoverNewPkgGraduationCheck: cannotRun("git unavailable"),
	}
	verdict, diags := classifyGates(t, h, narrativeReport("PASS"))
	if verdict != core.VerdictPASS {
		t.Fatalf("verdict=%q, want PASS — every gate failed OPEN, none may force FAIL", verdict)
	}
	if got := conflictDiags(diags); len(got) != 0 {
		t.Errorf("emitted %d conflict record(s) when no gate overrode the verdict: %v", len(got), got)
	}
}

func TestVerdictConflict_AllGatesGreen_NoConflict(t *testing.T) {
	clean := func(core.PhaseRequest) ([]string, error) { return nil, nil }
	h := hooks{
		gofmtCheck:                    clean,
		skillsDriftCheck:              clean,
		goVetCheck:                    clean,
		acsDurableCheck:               clean,
		integrationTierCheck:          clean,
		apicoverEnforceCheck:          clean,
		apicoverNewPkgGraduationCheck: clean,
	}
	verdict, diags := classifyGates(t, h, narrativeReport("PASS"))
	if verdict != core.VerdictPASS {
		t.Fatalf("verdict=%q, want PASS", verdict)
	}
	if got := conflictDiags(diags); len(got) != 0 {
		t.Errorf("emitted %d conflict record(s) with every gate green: %v", len(got), got)
	}
}

func TestVerdictConflict_VerdictUnchangedAcrossGateMatrix(t *testing.T) {
	clean := func(core.PhaseRequest) ([]string, error) { return nil, nil }
	cases := []struct {
		name      string
		narrative string
		h         hooks
		want      string
	}{
		{"pass/all-green", "PASS", hooks{gofmtCheck: clean, goVetCheck: clean}, core.VerdictPASS},
		{"warn/all-green", "WARN", hooks{gofmtCheck: clean, goVetCheck: clean}, core.VerdictWARN},
		{"fail/all-green", "FAIL", hooks{gofmtCheck: clean, goVetCheck: clean}, core.VerdictFAIL},
		{"pass/gofmt-dirty", "PASS", hooks{gofmtCheck: offenders("main.go")}, core.VerdictFAIL},
		{"warn/gofmt-dirty", "WARN", hooks{gofmtCheck: offenders("main.go")}, core.VerdictFAIL},
		{"pass/vet-dirty", "PASS", hooks{goVetCheck: offenders("import cycle")}, core.VerdictFAIL},
		{"pass/gofmt-cannot-run", "PASS", hooks{gofmtCheck: cannotRun("boom")}, core.VerdictPASS},
		{"warn/vet-cannot-run", "WARN", hooks{goVetCheck: cannotRun("boom")}, core.VerdictWARN},
		{"pass/no-gates-wired", "PASS", hooks{}, core.VerdictPASS},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := classifyGates(t, tc.h, narrativeReport(tc.narrative))
			if got != tc.want {
				t.Errorf("verdict=%q, want %q — the conflict record must be ADDITIVE, never a "+
					"change to what gates ship", got, tc.want)
			}
		})
	}
}
