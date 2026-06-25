package policy

import "testing"

// IntegrityMode resolves the integrity sub-policy (ADR-0065) with safe
// defaults: absent/partial/unknown ⇒ pipeline + shadow + provenance-required.
// The default is byte-neutral with today's behavior (pipeline mode = the
// existing single-pin check; shadow stage = log-only).
func TestIntegrityMode(t *testing.T) {
	bptr := func(b bool) *bool { return &b }
	cases := []struct {
		name      string
		in        *IntegrityPolicy
		wantMode  string
		wantStage string
		wantProv  bool
	}{
		{"absent_defaults", nil, "pipeline", "shadow", true},
		{"phase_enforce", &IntegrityPolicy{Mode: "phase", Stage: "enforce"}, "phase", "enforce", true},
		{"phase_shadow_explicit", &IntegrityPolicy{Mode: "phase", Stage: "shadow"}, "phase", "shadow", true},
		{"bogus_mode_falls_back", &IntegrityPolicy{Mode: "bogus"}, "pipeline", "shadow", true},
		{"bogus_stage_falls_back", &IntegrityPolicy{Mode: "phase", Stage: "nope"}, "phase", "shadow", true},
		{"provenance_disabled", &IntegrityPolicy{ProvenanceRequired: bptr(false)}, "pipeline", "shadow", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := Policy{Integrity: tc.in}
			mode, stage, prov := p.IntegrityMode()
			if mode != tc.wantMode || stage != tc.wantStage || prov != tc.wantProv {
				t.Errorf("IntegrityMode()=(%q,%q,%v), want (%q,%q,%v)",
					mode, stage, prov, tc.wantMode, tc.wantStage, tc.wantProv)
			}
		})
	}
}
