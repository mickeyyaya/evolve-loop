package cyclestate

import (
	"reflect"
	"testing"
)

func TestErrorMessages_ProjectsOnlyErrorSeverityInOrder(t *testing.T) {
	t.Parallel()
	got := ErrorMessages([]Diagnostic{
		{Severity: SeverityWarning, Message: "metrics file absent"},
		{Severity: SeverityError, Message: "first"},
		{Severity: "info", Message: "ignored"},
		{Severity: SeverityError, Message: "second"},
	})
	if want := []string{"first", "second"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ErrorMessages = %v, want %v", got, want)
	}
	if got := ErrorMessages(nil); got != nil {
		t.Errorf("nil diagnostics must project to nil, got %v", got)
	}
	if got := ErrorMessages([]Diagnostic{{Severity: SeverityWarning, Message: "only a warning"}}); got != nil {
		t.Errorf("warnings are not reasons, got %v", got)
	}
	if SeverityError != "error" || SeverityWarning != "warning" {
		t.Errorf("the severity vocabulary is the wire value producers emit: %q %q", SeverityError, SeverityWarning)
	}
}
