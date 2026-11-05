package core

import (
	"errors"
	"fmt"
	"testing"
)

func TestNewBlockerEnd_CarriesItsOutcomeAndItsDiagnosticThroughAWrap(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("resume: %w", NewBlockerEnd(Routable, Diagnostic{Message: "the pin has no route"}))
	var end blockerEnd
	if !errors.As(err, &end) {
		t.Fatalf("errors.As(%v) found no blocker end", err)
	}
	if end.outcome != Routable || end.Error() != "the pin has no route" {
		t.Errorf("end = %+v with text %q, want Routable and the diagnostic text", end, end.Error())
	}
}
