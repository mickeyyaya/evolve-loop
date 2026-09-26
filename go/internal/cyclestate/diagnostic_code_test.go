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
		{Severity: SeverityWarning, Message: "b", Code: DiagCodeTriageTopNEmpty}, // a trail, not a reason
		{Severity: SeverityError, Message: "c"},                                  // uncoded
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
		code        string
		task, route bool
	}{
		{DiagCodeTriageProtectedSurface, true, true},
		{DiagCodeTriageTopNEmpty, true, false},
		{DiagCodeTriageCommitmentInvalid, false, false},
		{"", false, false},
		{"SOMETHING_NEW", false, false},
	}
	var _ Disposition = RefusalDisposition(DiagCodeTriageProtectedSurface) // names Disposition for apicover
	for _, c := range cases {
		d := RefusalDisposition(c.code)
		if d.TaskLevel != c.task || d.RouteConsole != c.route {
			t.Errorf("RefusalDisposition(%q) = %+v, want task=%v route=%v", c.code, d, c.task, c.route)
		}
	}
}
