package resolvellm

import (
	"path/filepath"
	"testing"
)

// TestResult_ResolveProducesProfileResult names the resolvellm.Result type
// (Resolve returns it but the bare type is never named in a test) and pins the
// whole Result a profile read produces: CLI + tier from the profile, Source
// pinned to "profile" since Step 9. Result is all-string (comparable), so the
// whole value is asserted with a want-struct equality.
func TestResult_ResolveProducesProfileResult(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, ".evolve", "profiles", "scout.json"), map[string]any{
		"cli":                "claude",
		"model_tier_default": "deep",
	})
	got, err := Resolve("scout", Options{ProjectRoot: dir})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := Result{CLI: "claude", ModelTier: "deep", Source: "profile"}
	if got != want {
		t.Errorf("Resolve = %+v, want %+v", got, want)
	}
}
