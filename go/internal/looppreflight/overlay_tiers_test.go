package looppreflight

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runWithPolicy(t *testing.T, body string) Result {
	t.Helper()
	opts := goodPipelineOptions(t)
	if body != "" {
		if err := os.WriteFile(filepath.Join(opts.EvolveDir, "policy.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return r
}

func TestRun_OverlayTierSelectors(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantLevel  CheckLevel
		wantDetail string
	}{
		{"no policy file", "", LevelPass, ""},
		{"canonical selectors", `{"overlays":{"rules":[{"tiers":["deep","top"],"skills":["fable"]}]}}`, LevelPass, ""},
		{"non-canonical selector", `{"overlays":{"rules":[{"tiers":["deep","nonsense"],"skills":["fable"]}]}}`, LevelWarn, "nonsense"},
		{"unreadable policy", `{"overlays":`, LevelWarn, "policy.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := runWithPolicy(t, tc.body)
			c := findCheck(t, r, "overlay-tier-selectors")
			if c.Level != tc.wantLevel {
				t.Fatalf("level = %s, want %s (%s: %s)", c.Level, tc.wantLevel, c.Message, c.Detail)
			}
			if !strings.Contains(c.Detail, tc.wantDetail) {
				t.Errorf("detail %q does not name %q", c.Detail, tc.wantDetail)
			}
			if r.Halted() {
				t.Errorf("overlay tier selector check must never halt; OverallLevel=%s", r.OverallLevel)
			}
		})
	}
}
