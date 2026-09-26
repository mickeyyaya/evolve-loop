package phasecontract

import "testing"

func TestArtifactFilename(t *testing.T) {
	cases := []struct {
		name  string
		phase string
		want  string
	}{
		{"registered scout", "scout", ArtifactName("scout")},
		{"registered build", "build", ArtifactName("build")},
		{"registered audit", "audit", ArtifactName("audit")},
		{"alias advisor resolves to router", "advisor", ArtifactName("router")},
		{"NoArtifact phase falls back", "ship", "ship-report.md"},
		{"unregistered phase falls back", "custom-lint", "custom-lint-report.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ArtifactFilename(tc.phase); got != tc.want {
				t.Fatalf("ArtifactFilename(%q) = %q, want %q", tc.phase, got, tc.want)
			}
		})
	}
}

func TestArtifactFilenameNeverEmpty(t *testing.T) {
	phases := []string{"", "scout", "build", "audit", "ship", "retro", "tdd", "triage"}
	for _, p := range phases {
		if got := ArtifactFilename(p); got == "" {
			t.Errorf("ArtifactFilename(%q) returned empty — callers would join the workspace dir itself", p)
		}
	}
}

func TestArtifactFilenameMatchesRegistryForEveryRegisteredPhase(t *testing.T) {
	for phase, c := range contracts {
		if c.NoArtifact || c.ArtifactName == "" {
			continue
		}
		if got := ArtifactFilename(phase); got != c.ArtifactName {
			t.Errorf("ArtifactFilename(%q) = %q, registry says %q", phase, got, c.ArtifactName)
		}
	}
}
