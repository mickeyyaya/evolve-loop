package phasecontract

import "testing"

// TestArtifactFilename pins the SSOT-with-convention-fallback contract that the
// five hand-rolled `phase + "-report.md"` call sites were each re-deriving:
// registered phases resolve from the registry, everything else falls back to
// the filename convention (never the empty string, which would silently make a
// caller join the workspace dir itself).
func TestArtifactFilename(t *testing.T) {
	cases := []struct {
		name  string
		phase string
		want  string
	}{
		// Registered: the registry answer, not the convention. If a rename ever
		// makes these two coincide the test still holds — it asserts the VALUE
		// the registry publishes.
		{"registered scout", "scout", ArtifactName("scout")},
		{"registered build", "build", ArtifactName("build")},
		{"registered audit", "audit", ArtifactName("audit")},
		// Alias resolution rides through For, so ArtifactFilename inherits it.
		{"alias advisor resolves to router", "advisor", ArtifactName("router")},
		// NoArtifact (ship): ArtifactName returns "" to signal "no file
		// deliverable"; ArtifactFilename must convert that into the convention
		// rather than propagate the empty string.
		{"NoArtifact phase falls back", "ship", "ship-report.md"},
		// Unregistered user/inserted phase: convention.
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

// TestArtifactFilenameNeverEmpty is the property the call sites actually depend
// on: filepath.Join(workspace, ArtifactFilename(p)) must never collapse to the
// workspace directory itself for ANY phase the registry knows, nor for the
// empty phase name a mis-plumbed caller could pass.
func TestArtifactFilenameNeverEmpty(t *testing.T) {
	phases := []string{"", "scout", "build", "audit", "ship", "retro", "tdd", "triage"}
	for _, p := range phases {
		if got := ArtifactFilename(p); got == "" {
			t.Errorf("ArtifactFilename(%q) returned empty — callers would join the workspace dir itself", p)
		}
	}
}

// TestArtifactFilenameMatchesRegistryForEveryRegisteredPhase closes the drift
// this task exists to prevent: for every phase in the registry that HAS an
// artifact, the helper must return exactly the registry's name — no call site
// may end up with the convention where a registered name exists.
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
