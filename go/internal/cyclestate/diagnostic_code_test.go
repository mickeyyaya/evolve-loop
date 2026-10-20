package cyclestate

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDiagnostic_CodeIsOptionalOnTheWire(t *testing.T) {
	plain, _ := json.Marshal(Diagnostic{Severity: SeverityError, Message: "m"})
	if string(plain) != `{"severity":"error","message":"m"}` {
		t.Errorf("an uncoded diagnostic keeps its two-field wire shape: %s", plain)
	}
	coded, _ := json.Marshal(Diagnostic{Severity: SeverityError, Message: "m", Code: DiagCodeTriageProtectedSurface})
	if string(coded) != `{"severity":"error","message":"m","code":"TRIAGE_PROTECTED_SURFACE"}` {
		t.Errorf("coded wire shape: %s", coded)
	}
	var back Diagnostic
	if err := json.Unmarshal(coded, &back); err != nil || back.Code != DiagCodeTriageProtectedSurface {
		t.Errorf("round trip: %+v %v", back, err)
	}
}

func TestDiagCodes_TriageRefusalVocabulary(t *testing.T) {
	want := map[string]string{
		DiagCodeTriageProtectedSurface:  "TRIAGE_PROTECTED_SURFACE",
		DiagCodeTriageTopNEmpty:         "TRIAGE_TOPN_EMPTY",
		DiagCodeTriageCommitmentInvalid: "TRIAGE_COMMITMENT_INVALID",
		DiagCodeTriageScopeUnanswered:   "TRIAGE_SCOPE_UNANSWERED",
		DiagCodeTriageLanePinMissing:    "TRIAGE_LANE_PIN_MISSING",
		DiagCodeProtectedPredicateRoute: "PROTECTED_PREDICATE_ROUTE",
		DiagCodeBlockerRoute:            "BLOCKER_ROUTE",
	}
	for got, spelled := range want {
		if got != spelled {
			t.Errorf("code %q must be spelled %q", got, spelled)
		}
	}
}

func TestErrorCodes_ProjectsErrorSeverityCodesOnce(t *testing.T) {
	diags := []Diagnostic{
		{Severity: SeverityError, Message: "a", Code: DiagCodeTriageProtectedSurface},
		{Severity: SeverityWarning, Message: "b", Code: DiagCodeTriageTopNEmpty},
		{Severity: SeverityError, Message: "c"},
		{Severity: SeverityError, Message: "d", Code: DiagCodeTriageProtectedSurface},
		{Severity: SeverityError, Message: "e", Code: DiagCodeTriageCommitmentInvalid},
	}
	if got := ErrorCodes(diags); !reflect.DeepEqual(got, []string{DiagCodeTriageProtectedSurface, DiagCodeTriageCommitmentInvalid}) {
		t.Errorf("ErrorCodes = %v", got)
	}
	if got := ErrorCodes(nil); got != nil {
		t.Errorf("nil in, nil out: %v", got)
	}
}

func TestRefusalDisposition_TableBesideTheVocabulary(t *testing.T) {
	cases := []struct {
		code                 string
		task, route, outrank bool
	}{
		{DiagCodeTriageProtectedSurface, true, true, false},
		{DiagCodeProtectedPredicateRoute, true, true, false},
		{DiagCodeBlockerRoute, true, true, true},
		{DiagCodeTriageTopNEmpty, true, false, false},
		{DiagCodeTriageCommitmentInvalid, false, false, false},
		{DiagCodeTriageScopeUnanswered, true, false, false},
		{DiagCodeTriageLanePinMissing, false, false, false},
		{"", false, false, false},
		{"SOMETHING_NEW", false, false, false},
	}
	var _ Disposition = RefusalDisposition(DiagCodeTriageProtectedSurface)
	for _, c := range cases {
		d := RefusalDisposition(c.code)
		if d.TaskLevel != c.task || d.RouteConsole != c.route || d.OutranksSentinel != c.outrank {
			t.Errorf("RefusalDisposition(%q) = %+v, want task=%v route=%v outrank=%v", c.code, d, c.task, c.route, c.outrank)
		}
	}
}

func TestCycleTerminationTriageClaimFailed_IsTheSealedSpelling(t *testing.T) {
	if CycleTerminationTriageClaimFailed != "triage-empty-commitment-claimable-work" {
		t.Errorf("the sealed termination reason is %q", CycleTerminationTriageClaimFailed)
	}
}

func TestCycleTerminationBlockedNeedsConsole_IsTheSealedSpelling(t *testing.T) {
	if CycleTerminationBlockedNeedsConsole != "blocked-needs-console" {
		t.Errorf("the sealed termination reason is %q", CycleTerminationBlockedNeedsConsole)
	}
}
