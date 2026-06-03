package config

import "testing"

// commit_evidence_test.go — EVOLVE_COMMIT_EVIDENCE (ADR-0027) flag parsing.
// Default off (byte-identical legacy path-poll); shadow/enforce recognized;
// "advisory" and typos default to off with a warning (a typo must never
// silently enable phase commits).

func TestCommitEvidence_Default(t *testing.T) {
	cfg, _ := Load("", nil)
	if cfg.CommitEvidence != StageOff {
		t.Fatalf("default CommitEvidence = %v, want StageOff", cfg.CommitEvidence)
	}
}

func TestCommitEvidence_EnvParse(t *testing.T) {
	cases := []struct {
		in       string
		want     Stage
		wantWarn bool
	}{
		{"off", StageOff, false},
		{"0", StageOff, false},
		{"shadow", StageShadow, false},
		{"enforce", StageEnforce, false},
		{"advisory", StageOff, true}, // no advisory state for this axis
		{"garbage", StageOff, true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			cfg, ws := Load("", map[string]string{"EVOLVE_COMMIT_EVIDENCE": c.in})
			if cfg.CommitEvidence != c.want {
				t.Errorf("CommitEvidence(%q) = %v, want %v", c.in, cfg.CommitEvidence, c.want)
			}
			gotWarn := false
			for _, w := range ws {
				if w.Code == "unknown-value" && contains(w.Message, "EVOLVE_COMMIT_EVIDENCE") {
					gotWarn = true
				}
			}
			if gotWarn != c.wantWarn {
				t.Errorf("warn(%q) = %v, want %v (warnings=%+v)", c.in, gotWarn, c.wantWarn, ws)
			}
		})
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
