package audit

import (
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// audit_verdict_conflict_gates_test.go — RED contract for the cycle-1127
// continuation of `emit-verdict-conflict-diagnostic` (inbox item
// `verdict-coherence-auditor-vs-egps`, weight 0.92, 4th recurrence of the
// cycle-87 / cycle-352 / cycle-456 family).
//
// What is ALREADY done (cycle-1124 salvage, HEAD 33596bb0): the three EGPS
// override branches (acs-verdict.json unreadable, red_count>0,
// ship_eligible=false) record the disagreement — see
// audit_verdict_conflict_test.go, all GREEN at HEAD.
//
// What is STILL OPEN and is what this file pins: AC-1 names FIVE more gates
// that force `verdict = core.VerdictFAIL` in hooks.Classify —
//
//	audit.go  gofmt gate                       (h.gofmtCheck)
//	audit.go  skills-drift gate                (h.skillsDriftCheck)
//	audit.go  applyCIGate x5                   (goVet, acsDurable,
//	                                            integrationTier,
//	                                            apicoverEnforce,
//	                                            apicoverNewPkgGraduation)
//
// — and NONE of them records the auditor's narrative verdict before clobbering
// it. The operator-facing consequence is identical to the EGPS case the salvage
// already closed: a FAIL dossier whose SubstantiveError says only "gofmt: 3
// file(s) are not gofmt -s clean" cannot be told apart from one where the
// auditor itself independently found the cycle broken. Half a fix is a fix that
// still loses the signal on 5 of 8 gates.
//
// Contract pinned here (an extension of the salvaged contract, NOT a rewrite —
// every existing TestVerdictConflict_* case must stay green):
//
//  1. EVERY gate that forces FAIL over a found, non-FAIL narrative emits the
//     error-severity `verdict-conflict:` record naming that narrative verdict.
//  2. Exactly ONE record per Classify call, no matter how many gates fired —
//     the record is a statement about the call, not about each gate. This is
//     what makes a post-gate single-exit emission the natural implementation.
//  3. The fail-OPEN paths stay silent: a gate that could not RUN emits its
//     existing warning and does not force FAIL, so there is no conflict.
//  4. AC-4: the returned verdict is byte-identical to today's behaviour in
//     every case. The record is additive; it never softens a gate.
//
// Out of scope, deliberately unpinned: the policy.json workflow.strict_audit
// WARN→FAIL promotion. AC-1 does not name it, and it is a policy decision on a
// narrative the auditor already declined to pass, not a mechanical gate
// disagreeing with a clean read. Whether the implementation happens to cover it
// is left free; no test here asserts either way.

// offenders is a check seam that reports a gate hit.
func offenders(names ...string) func(core.PhaseRequest) ([]string, error) {
	return func(core.PhaseRequest) ([]string, error) { return names, nil }
}

// cannotRun is a check seam that fails OPEN (the gate could not execute).
func cannotRun(msg string) func(core.PhaseRequest) ([]string, error) {
	return func(core.PhaseRequest) ([]string, error) { return nil, errors.New(msg) }
}

// classifyGates runs the real Classify over a temp workspace holding a GREEN
// EGPS verdict (red_count=0, ship_eligible=true), so the EGPS branches — the
// only ones already implemented — cannot fire. Any conflict record observed
// here therefore comes from the non-EGPS gate under test, never from the
// salvaged code path.
func classifyGates(t *testing.T, h hooks, artifact string) (string, []core.Diagnostic) {
	t.Helper()
	ws := t.TempDir()
	yes := true
	writeACSVerdictShip(t, ws, 0, &yes)
	verdict, diags, _ := h.Classify(artifact, core.PhaseRequest{Workspace: ws}, core.BridgeResponse{})
	return verdict, diags
}

// nonEGPSGates enumerates every gate in Classify that forces FAIL and is NOT
// one of the three EGPS branches already covered by the salvage. Each entry
// wires exactly one seam so the resulting conflict record is attributable.
var nonEGPSGates = []struct {
	name  string
	wire  func(*hooks)
	inMsg string // a substring the gate's own error diagnostic carries
}{
	{"gofmt", func(h *hooks) { h.gofmtCheck = offenders("acs/cycle1127/predicates_test.go") }, "gofmt"},
	{"skills-drift", func(h *hooks) { h.skillsDriftCheck = offenders("skills/evolve-auditor/SKILL.md") }, "drift"},
	{"go-vet", func(h *hooks) { h.goVetCheck = offenders("internal/foo: import cycle") }, "vet"},
	{"acs-durable", func(h *hooks) { h.acsDurableCheck = offenders("flag-ceiling") }, "acs-durable"},
	{"integration-tier", func(h *hooks) { h.integrationTierCheck = offenders("TestFleetSoak") }, "integration"},
	{"apicover-enforce", func(h *hooks) { h.apicoverEnforceCheck = offenders("internal/bar:12") }, "apicover"},
	{"apicover-newpkg", func(h *hooks) { h.apicoverNewPkgGraduationCheck = offenders("internal/baz") }, "apicover"},
}

// TestVerdictConflict_EveryNonEGPSGateRecordsTheConflict — the crux of this
// cycle. For each non-EGPS gate: a narrative PASS that the gate overrides to
// FAIL must leave exactly one error-severity conflict record naming "PASS".
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

// TestVerdictConflict_NonEGPSGate_NarrativeWARN — the narrative verdict is
// carried verbatim on the non-EGPS gates too, not hardcoded to "PASS".
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

// TestVerdictConflict_MultipleGatesStillOneRecord — AC-2 / anti-duplication.
// A cycle that is simultaneously EGPS-red, gofmt-dirty, skills-drifted and
// vet-broken is ONE conflict (the auditor said PASS, the machine said no), not
// four. Four records would quadruple the dossier's SubstantiveError text and
// give the identical-fingerprint breaker four spellings of one event.
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

// --- Negative / anti-no-op axis --------------------------------------------

// TestVerdictConflict_NonEGPSGate_NoNoiseWhenNarrativeFAIL — the coherent case
// on every non-EGPS gate: auditor and gate agree, nothing to record.
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

// TestVerdictConflict_NonEGPSGate_NoNoiseWhenNarrativeUnparseable — no
// narrative was parsed, so there is no claim to disagree with. Fabricating one
// here would fire on every malformed report in the fleet.
func TestVerdictConflict_NonEGPSGate_NoNoiseWhenNarrativeUnparseable(t *testing.T) {
	h := hooks{gofmtCheck: offenders("main.go")}
	_, diags := classifyGates(t, h, "# Audit Report\n\nprose with no verdict declaration\n")
	if got := conflictDiags(diags); len(got) != 0 {
		t.Errorf("emitted %d conflict record(s) with no parseable narrative: %v", len(got), got)
	}
}

// TestVerdictConflict_GateCouldNotRun_NoConflict — AC-3, the fail-OPEN axis.
// Every non-EGPS gate fails open: an infra error emits a warning and leaves the
// verdict alone. No override happened, so no conflict exists — and the PASS
// must survive. An implementation that keys the record off "a gate diagnostic
// exists" instead of "the verdict was overridden" fails here.
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

// TestVerdictConflict_AllGatesGreen_NoConflict — the ordinary shipping cycle.
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

// --- AC-4: the returned verdict is unchanged --------------------------------

// TestVerdictConflict_VerdictUnchangedAcrossGateMatrix — the additive-only
// proof. For every (narrative x gate-state) combination the returned verdict is
// exactly what the gate semantics dictate today. A "fix" that suppresses a gate
// FAIL so the conflict stops appearing — the tempting wrong turn on a
// diagnosability task — fails here.
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
