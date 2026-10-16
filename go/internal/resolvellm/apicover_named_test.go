package resolvellm

import (
	"path/filepath"
	"testing"
)

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
