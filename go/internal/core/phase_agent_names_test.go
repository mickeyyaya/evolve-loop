package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPhaseAgentName_ValuesResolveToCheckedInProfiles(t *testing.T) {
	profilesDir := findProfilesDir(t)
	for phase, agent := range phaseAgentName {
		path := filepath.Join(profilesDir, agent+".json")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("phaseAgentName[%q] = %q, but %s does not exist — table drifted from the phase package's AgentPromptName() or the profile was renamed", phase, agent, path)
		}
	}
}

// findProfilesDir walks up from the test's working directory (go/internal/core)
// to the repo root and returns <root>/.evolve/profiles. Fails the test if it is
// not found, so the drift guard never silently no-ops.
func findProfilesDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		cand := filepath.Join(dir, ".evolve", "profiles")
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			return cand
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf(".evolve/profiles not found walking up from working dir")
		}
		dir = parent
	}
}
