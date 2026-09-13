package gatesignal

// reporter_test.go — ADR-0101 S2b (design signal-center-design.md §15.5): the
// contract gate's producer. RED first: every decision the gate can reach is
// ONE event under module gate.contract with the origin Reviewer.Review, the
// kind gate.passed (the phase advanced) or gate.rejected (it did not), a
// registered GATE_CONTRACT_* code, and fields that name what was checked.

import (
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func recording() (*Reporter, *[]signalcenter.Event) {
	c := signalcenter.New()
	got := &[]signalcenter.Event{}
	c.Subscribe(func(e signalcenter.Event) { *got = append(*got, e) })
	return New(c), got
}

func one(t *testing.T, got []signalcenter.Event) signalcenter.Event {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("exactly one event per decision: %+v", got)
	}
	e := got[0]
	if e.Module != signalcenter.ModuleGateContract || e.Origin != Origin || e.Cycle != 1627 || e.RunID != "run-1" || e.Phase != "triage" {
		t.Fatalf("every event carries the gate's module, its origin and the check's identity: %+v", e)
	}
	return e
}

var check = Check{Cycle: 1627, RunID: "run-1", Phase: "triage"}

func TestReporter_Verified_NamesWhatWasChecked(t *testing.T) {
	r, got := recording()
	r.Verified(check, Verified{Artifact: "triage-report.md", Bytes: 4213, Owed: []string{"triage-decision.json"}, Effects: []string{"inbox-claim"}, Stage: "enforce"})
	e := one(t, *got)
	if e.Kind != signalcenter.KindGatePassed || e.Severity != signalcenter.SeverityInfo || e.Code != CodeVerified {
		t.Fatalf("a clean verification is gate.passed INFO GATE_CONTRACT_VERIFIED: %+v", e)
	}
	if !strings.Contains(e.Reason, "triage-report.md") || !strings.Contains(e.Reason, "4213 B") || !strings.Contains(e.Reason, "triage-decision.json") || !strings.Contains(e.Reason, "inbox-claim") {
		t.Fatalf("the reason names the artifact, its size, the owed files and the effects: %q", e.Reason)
	}
	if e.Fields["artifact"] != "triage-report.md" || e.Fields["bytes"] != "4213" || e.Fields["owed"] != "triage-decision.json" || e.Fields["effects"] != "inbox-claim" || e.Fields["stage"] != "enforce" {
		t.Fatalf("fields carry the checked set: %+v", e.Fields)
	}
}

func TestReporter_Verified_NoDeclaredArtifact(t *testing.T) {
	r, got := recording()
	r.Verified(check, Verified{Stage: "enforce"})
	e := one(t, *got)
	_, hasArtifact := e.Fields["artifact"]
	_, hasOwed := e.Fields["owed"]
	if !strings.Contains(e.Reason, "no declared artifact") || hasArtifact || hasOwed {
		t.Fatalf("a contract without a file says so and carries no empty-valued keys: %+v", e)
	}
}

func TestReporter_Rejected_IsWarnWithTheCorrectionDirective(t *testing.T) {
	r, got := recording()
	r.Rejected(check, "triage deliverable failed contract: [missing_secondary] triage-decision.json missing", []string{"missing_secondary"}, 1, 3)
	e := one(t, *got)
	if e.Kind != signalcenter.KindGateRejected || e.Severity != signalcenter.SeverityWarn || e.Code != CodeRejected {
		t.Fatalf("a block is gate.rejected WARN GATE_CONTRACT_REJECTED: %+v", e)
	}
	if !strings.Contains(e.Reason, "[missing_secondary] triage-decision.json missing") {
		t.Fatalf("the reason is the gate's own summary — the correction directive: %q", e.Reason)
	}
	if e.Fields["codes"] != "missing_secondary" || e.Fields["blocks"] != "1" || e.Fields["threshold"] != "3" {
		t.Fatalf("fields carry the violation codes and the breaker count: %+v", e.Fields)
	}
}

func TestReporter_WouldBlock_IsWarnUnderAShadowStage(t *testing.T) {
	r, got := recording()
	r.WouldBlock(check, "triage deliverable failed contract: [missing_artifact] x", []string{"missing_artifact", "empty_secondary"}, "shadow", true)
	e := one(t, *got)
	if e.Kind != signalcenter.KindGatePassed || e.Severity != signalcenter.SeverityWarn || e.Code != CodeWouldBlock {
		t.Fatalf("a would-block is gate.passed WARN GATE_CONTRACT_WOULD_BLOCK (the phase advanced, the deliverable is still missing): %+v", e)
	}
	if e.Fields["stage"] != "shadow" || e.Fields["codes"] != "missing_artifact,empty_secondary" || e.Fields["salvage"] != "would" {
		t.Fatalf("fields carry the stage, the codes and whether salvage would have acted: %+v", e.Fields)
	}
	r2, got2 := recording()
	r2.WouldBlock(check, "r", nil, "advisory", false)
	e2 := one(t, *got2)
	_, hasSalvage := e2.Fields["salvage"]
	_, hasCodes := e2.Fields["codes"]
	if hasSalvage || hasCodes {
		t.Fatalf("no salvage, no codes: no empty-valued keys: %+v", e2.Fields)
	}
}

func TestReporter_Salvaged_IsInfo(t *testing.T) {
	r, got := recording()
	r.Salvaged(check, "audit-report.md", "fenced_json")
	e := one(t, *got)
	if e.Kind != signalcenter.KindGatePassed || e.Severity != signalcenter.SeverityInfo || e.Code != CodeSalvaged {
		t.Fatalf("a salvage is gate.passed INFO GATE_CONTRACT_SALVAGED: %+v", e)
	}
	if e.Fields["artifact"] != "audit-report.md" || e.Fields["pattern"] != "fenced_json" || !strings.Contains(e.Reason, "fenced_json") {
		t.Fatalf("the salvage names the artifact and the pattern: %+v", e)
	}
}

func TestReporter_Demoted_IsWarn(t *testing.T) {
	r, got := recording()
	r.Demoted(check, "triage deliverable failed contract: [missing_artifact] x", 3)
	e := one(t, *got)
	if e.Kind != signalcenter.KindGatePassed || e.Severity != signalcenter.SeverityWarn || e.Code != CodeDemoted {
		t.Fatalf("a breaker demotion is gate.passed WARN GATE_CONTRACT_DEMOTED: %+v", e)
	}
	if e.Fields["blocks"] != "3" || !strings.Contains(e.Reason, "3 consecutive") || !strings.Contains(e.Reason, "[missing_artifact] x") {
		t.Fatalf("the demotion names the count and the last violation: %+v", e)
	}
}

func TestReporter_FailOpen_IsWarn(t *testing.T) {
	r, got := recording()
	r.FailOpen(check, errors.New("phase not in catalog"))
	e := one(t, *got)
	if e.Kind != signalcenter.KindGatePassed || e.Severity != signalcenter.SeverityWarn || e.Code != CodeFailOpen || !strings.Contains(e.Reason, "phase not in catalog") {
		t.Fatalf("an ambiguity is gate.passed WARN GATE_CONTRACT_FAIL_OPEN carrying the error: %+v", e)
	}
}

func TestReporter_NilCenterIsTheNullObject(t *testing.T) {
	r := New(nil)
	if r.Wired() {
		t.Fatal("no Center: unwired")
	}
	r.Verified(check, Verified{})
	r.Rejected(check, "r", nil, 1, 3)
	r.WouldBlock(check, "r", nil, "shadow", false)
	r.Salvaged(check, "a", "p")
	r.Demoted(check, "r", 3)
	r.FailOpen(check, errors.New("e"))
	wired, _ := recording()
	if !wired.Wired() {
		t.Fatal("a Center: wired")
	}
}

func TestCodes_AreRegisteredUnderTheGateModule(t *testing.T) {
	for _, code := range []signalcenter.Code{CodeVerified, CodeRejected, CodeWouldBlock, CodeSalvaged, CodeDemoted, CodeFailOpen} {
		if m, ok := signalcenter.IsRegistered(code); !ok || m != signalcenter.ModuleGateContract {
			t.Fatalf("%s must be registered under module gate.contract: %v %v", code, m, ok)
		}
		if !strings.HasPrefix(string(code), "GATE_CONTRACT_") {
			t.Fatalf("codes are MODULE_SNAKE_CASE: %s", code)
		}
	}
}

// Architecture review of S2b (LOW): emit filters into its own map; the
// caller's map is never mutated.
func TestReporter_EmitDoesNotMutateTheCallersFields(t *testing.T) {
	r, got := recording()
	r.Verified(check, Verified{Artifact: "a.md", Bytes: 1, Stage: "enforce"})
	e := one(t, *got)
	if _, has := e.Fields["owed"]; has {
		t.Fatalf("empty-valued keys are dropped from the emitted event: %+v", e.Fields)
	}
	mine := map[string]string{"k": "", "kept": "v"}
	r.emit(check, signalcenter.KindGatePassed, signalcenter.SeverityInfo, CodeVerified, "r", mine)
	if len(mine) != 2 || mine["k"] != "" {
		t.Fatalf("the caller's map is left intact: %+v", mine)
	}
	if last := (*got)[len(*got)-1]; len(last.Fields) != 1 || last.Fields["kept"] != "v" {
		t.Fatalf("the emitted copy carries only the non-empty keys: %+v", last.Fields)
	}
}
