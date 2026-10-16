package consensusdispatch

import (
	"path/filepath"
	"testing"
)

func TestProfile_FullStructFromParse(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "auditor.json")
	writeProfile(t, p, map[string]any{
		"enabled":          true,
		"cli_voters":       []string{"claude", "gemini", "codex"},
		"quorum":           2,
		"require_min_tier": "full",
	})
	got, err := ParseProfile(p)
	if err != nil {
		t.Fatalf("ParseProfile: %v", err)
	}
	want := Profile{
		ModelTierDefault: "sonnet",
		Enabled:          true,
		CLIVoters:        []string{"claude", "gemini", "codex"},
		Quorum:           2,
		RequireMinTier:   "full",
	}
	if got.ModelTierDefault != want.ModelTierDefault ||
		got.Enabled != want.Enabled ||
		got.Quorum != want.Quorum ||
		got.RequireMinTier != want.RequireMinTier ||
		!equalStrings(got.CLIVoters, want.CLIVoters) {
		t.Errorf("ParseProfile = %+v, want %+v", got, want)
	}
}
