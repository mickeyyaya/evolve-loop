package bridge

// effort_toprungs_test.go — the top of the reasoning ladder, pinned per family.
//
// Directive history: 2026-08-24 put deep/top phases at xhigh; 2026-08-28
// moved the CODEX-routed ones to max; 2026-09-01 moved them to high (quota
// headroom). This file pins REALIZABILITY of the upper rungs regardless of
// which one the current directive selects — the max/xhigh rows below stay
// because the manifest must keep every mapped rung realizable (a directive
// can flip back with one profile edit); WHICH rung profiles actually pin
// lives in profiles/effort_defaults_test.go, not here.
//
// Two contracts: (1) each rung REALIZES on the families with an effort dial —
// realizeScalar silently drops unmapped enum values, so a missing codex
// mapping loses the dial with no error (observed exactly that way: the max row
// failed with flags [--yolo], the dial simply absent); (2) EVERY tracked
// profile's effort_level is realizable by its own family's manifest — the
// class guard, so a future profile value can never silently no-op.
//
// Ladder verified live against codex 0.147.0 (/model -> "More reasoning..."):
// low, medium, high, xhigh, max, ultra. claude exposes low..max via --effort
// (its picker's "Ultracode" is an orchestration mode, NOT an --effort token —
// `--effort ultracode` silently falls back to xhigh).

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

// ASYMMETRY, stated so these rows are not over-read: codex-tmux maps effort
// through a values TABLE, so an unmapped rung silently drops — that is the bug
// class here, and the codex rows are what catch it. claude-tmux declares effort
// as pass-through (flag, no values table), so realizeScalar appends ANY string;
// its rows would pass for "banana" just as readily as for "max". They therefore
// prove the pass-through MECHANISM is wired (they would fail if the channel
// flipped to noop, the flag were renamed, or a restrictive values table were
// added without the rung) — they prove NOTHING about max being a valid claude
// effort, and no claude-routed profile ships max today. Claimed narrowly on
// purpose: a guard cited for more than it checks is worse than no guard.
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
			CLI             string            `json:"cli"`
			EffortLevel     string            `json:"effort_level"`
			EffortOverrides map[string]string `json:"effort_overrides"`
		}
		if json.Unmarshal(raw, &p) != nil || p.CLI == "" {
			continue
		}
		// Every rung the profile can dispatch at: effort_level plus each
		// per-tier effort_overrides value (ADR-0096) — the override is realized
		// by the same realizeScalar and silently dropped the same way.
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
