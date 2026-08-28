package profiles

// effort_defaults_test.go — cycle-566 RED test for the per-phase EFFORT default
// matrix (inbox `per-phase-effort-routing`). Loads the REAL shipped profiles (not
// a fixture) and asserts each phase pins the committed effort level. Every value
// is config-sourced — read through the loader from .evolve/profiles/*.json — so
// the production defaults carry ZERO Go literals (acceptance: "all config").
//
// Evidence for the matrix (inbox summary): Opus 4.5 at medium effort matches
// Sonnet 4.5's best SWE-bench score at 76% fewer output tokens; max effort buys
// single-digit gains at ~4x cost. Cheap survey/classification phases run low;
// generative/judgement phases run medium.
//
// RED now: scout/triage currently pin "medium", auditor pins "high", and
// tdd-engineer/adversarial-review pin nothing. GREEN once the config is aligned.

import (
	"path/filepath"
	"runtime"
	"testing"
)

// effortProfilesDir resolves the live .evolve/profiles directory relative to this
// test file so the matrix is asserted against the profiles the loop actually
// ships, not a hand-built fixture (drift between the two would otherwise hide).
func effortProfilesDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".evolve", "profiles")
}

// TestEffortDefaults_Matrix — AC-B: the committed per-phase effort defaults.
// scout/triage=low (cheap survey + classification); tdd/audit/adversarial=medium
// (judgement); builder=medium (generation). Keyed by the on-disk profile file
// basename each phase resolves to.
func TestEffortDefaults_Matrix(t *testing.T) {
	loader := NewFromDir(effortProfilesDir(t))
	// 2026-08-28 operator directive supersedes the 2026-08-24 xhigh rung for
	// the CODEX-routed deep/top phases: they run at max, the rung above xhigh
	// that codex 0.147.0 exposes via /model -> "More reasoning..." -> Max
	// (verified live: `-c model_reasoning_effort=max` renders "gpt-5.6-sol max"
	// in the status bar, and holds through /plan with the paired plan-mode key).
	//
	// The two CLAUDE-routed deep/top graders deliberately STAY at xhigh:
	// Anthropic's own Opus guidance (docs/research/fable-simulation-2026/
	// model-profiles.md) recommends xhigh for coding/agentic work and warns max
	// "can be prone to overthinking". Different family, different ceiling — the
	// abstract effort dial is realized per family, so this split is by design,
	// not drift. The fast/balanced rows keep the cycle-566 cost matrix.
	want := map[string]string{
		"scout":              "low",
		"triage":             "low",
		"tdd-engineer":       "medium",
		"builder":            "medium",
		"auditor":            "xhigh",
		"adversarial-review": "xhigh",
		"retrospective":      "max",
		"premise-challenge":  "max",
		"intent":             "max",
	}
	for profile, effort := range want {
		p, err := loader.Get(profile)
		if err != nil {
			t.Fatalf("Get(%s): %v", profile, err)
		}
		if p.EffortLevel != effort {
			t.Errorf("profile %s: effort_level = %q, want %q (committed per-phase effort matrix)", profile, p.EffortLevel, effort)
		}
	}
}
