package bridge

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

// claude-tmux declares effort as pass-through (flag, no values table), so
// realizeScalar appends any string; its rows prove the mechanism is wired,
// not that "max" is a valid claude effort level.
func TestEffortTopRungs_RealizeOnCodexAndClaude(t *testing.T) {
	for _, tc := range []struct {
		manifest string
		effort   string
		want     []string
	}{
		{"codex-tmux", "xhigh", []string{"-c", "model_reasoning_effort=xhigh"}},
		{"claude-tmux", "xhigh", []string{"--effort", "xhigh"}},
		{"codex-tmux", "max", []string{"-c", "model_reasoning_effort=max"}},
		{"claude-tmux", "max", []string{"--effort", "max"}},
	} {
		m, err := LoadManifest(tc.manifest)
		if err != nil {
			t.Fatalf("LoadManifest(%s): %v", tc.manifest, err)
		}
		r := Realize(m, LaunchIntent{Effort: tc.effort})
		if !containsSubsequence(r.LaunchFlags, tc.want) {
			t.Errorf("%s: %s effort did not realize — flags %v want subsequence %v", tc.manifest, tc.effort, r.LaunchFlags, tc.want)
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

// realizeScalar silently drops an unmapped enum value, so this is the only
// place a profile's effort_level/effort_overrides mismatch with its manifest
// is caught before a live dispatch quietly loses its reasoning-effort dial.
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
			CLI             string            `json:"cli"`
			EffortLevel     string            `json:"effort_level"`
			EffortOverrides map[string]string `json:"effort_overrides"`
		}
		if json.Unmarshal(raw, &p) != nil || p.CLI == "" {
			continue
		}
		// See ADR-0096.
		// An effort_overrides value is realized by the same realizeScalar
		// and silently dropped the same way.
		rungs := map[string]string{}
		if p.EffortLevel != "" {
			rungs["effort_level"] = p.EffortLevel
		}
		for tier, e := range p.EffortOverrides {
			if e != "" {
				rungs["effort_overrides."+tier] = e
			}
		}
		if len(rungs) == 0 {
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
			for field, rung := range rungs {
				if _, mapped := spec.Values[rung]; !mapped {
					t.Errorf("profile %s: %s %q is not mapped in %s's effort values — realizeScalar would SILENTLY drop the dial", e.Name(), field, rung, p.CLI)
				}
			}
		}
	}
	if checked == 0 {
		t.Skip("no effort-bearing profiles found")
	}
}
