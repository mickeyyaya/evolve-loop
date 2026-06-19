package dossier

import "testing"

// TestBuild verifies Build returns a populated Dossier with Cycle+Goal from
// BuildOpts and at least one phase.
func TestBuild(t *testing.T) {
	d, err := Build(1, BuildOpts{WorkspacePath: t.TempDir(), Goal: "reduce flags"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if d.Cycle != 1 {
		t.Errorf("Cycle: got %d, want 1", d.Cycle)
	}
	if d.Goal != "reduce flags" {
		t.Errorf("Goal: got %q, want %q", d.Goal, "reduce flags")
	}
	if len(d.Phases) == 0 {
		t.Error("Build: expected >=1 PhaseRecord")
	}
	if d.FinalVerdict == "" {
		t.Error("Build: FinalVerdict must not be empty")
	}
}

// TestBuild_Errors verifies Build returns an error for cycle <= 0 (edge/OOD
// cases — the strongest anti-no-op signal for the validation path).
func TestBuild_Errors(t *testing.T) {
	cases := []struct {
		name  string
		cycle int
	}{
		{"negative cycle", -1},
		{"zero cycle", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Build(tc.cycle, BuildOpts{Goal: "x"})
			if err == nil {
				t.Errorf("Build(%d, ...): want error for invalid cycle, got nil", tc.cycle)
			}
		})
	}
}
