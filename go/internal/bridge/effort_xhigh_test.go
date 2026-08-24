package bridge

// effort_xhigh_test.go — 2026-08-24 operator directive: deep/top-tier phases
// run at xhigh reasoning effort. Two contracts: (1) xhigh REALIZES on the
// families with an effort dial (realizeScalar silently drops unmapped enum
// values, so a missing codex mapping would silently lose the dial — worse
// than high); (2) EVERY tracked profile's effort_level is realizable by its
// own family's manifest — the class guard, so a future profile value can
// never silently no-op on a values-mapped manifest.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func repoRootForEffort(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
}

func TestEffortXhigh_RealizesOnCodexAndClaude(t *testing.T) {
	for _, tc := range []struct {
		manifest string
		want     []string
	}{
		{"codex-tmux", []string{"-c", "model_reasoning_effort=xhigh"}},
		{"claude-tmux", []string{"--effort", "xhigh"}},
	} {
		m, err := LoadManifest(tc.manifest)
		if err != nil {
			t.Fatalf("LoadManifest(%s): %v", tc.manifest, err)
		}
		r := Realize(m, LaunchIntent{Effort: "xhigh"})
		if !containsSubsequence(r.LaunchFlags, tc.want) {
			t.Errorf("%s: xhigh effort did not realize — flags %v want subsequence %v", tc.manifest, r.LaunchFlags, tc.want)
		}
	}
}

func containsSubsequence(hay, needle []string) bool {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if slices.Equal(hay[i:i+len(needle)], needle) {
			return true
		}
	}
	return false
}

// Every tracked profile's effort_level must be realizable by the manifest of
// the profile's own cli family: mapped in a values table, pass-through (flag
// with no values), or the family has no effort dial at all (noop/absent).
// realizeScalar's silent-drop on unmapped enum values makes this the ONLY
// place the mismatch can be caught before a live dispatch quietly loses its
// reasoning-effort dial.
func TestTrackedProfileEffortLevelsAllRealizable(t *testing.T) {
	root := repoRootForEffort(t)
	profDir := filepath.Join(root, ".evolve", "profiles")
	entries, err := os.ReadDir(profDir)
	if err != nil {
		t.Skipf("profiles dir unavailable: %v", err)
	}
	checked := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(profDir, e.Name()))
		if err != nil {
			continue
		}
		var p struct {
			CLI         string `json:"cli"`
			EffortLevel string `json:"effort_level"`
		}
		if json.Unmarshal(raw, &p) != nil || p.EffortLevel == "" || p.CLI == "" {
			continue
		}
		m, err := LoadManifest(p.CLI)
		if err != nil {
			continue // family manifest absent here — other tests own that
		}
		spec, ok := m.Params["effort"]
		if !ok || spec.Channel == "noop" {
			continue // no dial: cleanly no-ops by design
		}
		checked++
		if len(spec.Values) > 0 {
			if _, mapped := spec.Values[p.EffortLevel]; !mapped {
				t.Errorf("profile %s: effort_level %q is not mapped in %s's effort values — realizeScalar would SILENTLY drop the dial", e.Name(), p.EffortLevel, p.CLI)
			}
		}
	}
	if checked == 0 {
		t.Skip("no effort-bearing profiles found")
	}
}
