package consensusdispatch

import (
	"path/filepath"
	"testing"
)

// TestProfile_FullStructFromParse binds the named Profile type to ParseProfile's
// result and asserts full-struct equality against the expected parse. This pins
// the field-by-field mapping from the JSON consensus block (and the
// model_tier_default top-level key) onto the Profile struct, including the two
// fields that get a default (ModelTierDefault, RequireMinTier) when omitted —
// here both are supplied so the compare is exact across all five fields.
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
		ModelTierDefault: "sonnet", // from writeProfile's top-level model_tier_default
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
