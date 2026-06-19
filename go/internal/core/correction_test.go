package core

import (
	"strings"
	"testing"
)

func TestComposeCorrection(t *testing.T) {
	t.Parallel()
	got := composeCorrection("audit deliverable failed contract: [missing_section] required section 'Verdict' not found")
	if !strings.Contains(got, "REJECTED") {
		t.Errorf("missing rejection framing: %q", got)
	}
	if !strings.Contains(got, "missing_section") {
		t.Errorf("must embed the violation reason: %q", got)
	}
	if !strings.Contains(got, "contracted path") {
		t.Errorf("must instruct writing at the contracted path: %q", got)
	}
}
