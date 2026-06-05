package core

import (
	"strings"
	"testing"
)

func TestComposeCorrection(t *testing.T) {
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

func TestResolveContractCorrectionRetries(t *testing.T) {
	cases := []struct {
		in   map[string]string
		want int
	}{
		{nil, 2}, // default
		{map[string]string{"EVOLVE_CONTRACT_CORRECTION_RETRIES": "0"}, 0}, // disable allowed
		{map[string]string{"EVOLVE_CONTRACT_CORRECTION_RETRIES": "3"}, 3},
		{map[string]string{"EVOLVE_CONTRACT_CORRECTION_RETRIES": "99"}, 5}, // clamp to max
		{map[string]string{"EVOLVE_CONTRACT_CORRECTION_RETRIES": "-1"}, 2}, // invalid → default
		{map[string]string{"EVOLVE_CONTRACT_CORRECTION_RETRIES": "x"}, 2},  // unparseable → default
	}
	for _, c := range cases {
		if got := resolveContractCorrectionRetries(c.in); got != c.want {
			t.Errorf("env=%v → %d, want %d", c.in, got, c.want)
		}
	}
}
