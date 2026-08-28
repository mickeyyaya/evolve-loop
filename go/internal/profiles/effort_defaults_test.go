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
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
		"retrospective":      codexDeepTopEffort,
		"premise-challenge":  codexDeepTopEffort,
		"intent":             codexDeepTopEffort,
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

// codexDeepTopEffort is the single source for the 2026-08-28 directive's rung.
// The matrix pin below and the class guard both read it, so the next directive
// change is one edit rather than two that can drift apart.
const codexDeepTopEffort = "max"

// CLASS GUARD for the 2026-08-28 directive. The original change swept the 21
// codex deep/top profiles that existed at that moment — a point-in-time edit.
// failure-adjudicator.json landed hours later on a different branch (#508) and
// missed the sweep entirely, arriving on main at xhigh while every sibling ran
// max. Nothing caught it: the realizability guard passed (xhigh is still a
// mapped rung) and the matrix pin only names specific profiles.
//
// So the rule is pinned as a RULE rather than as a list. A new codex deep/top
// profile now has to state its effort deliberately instead of inheriting a
// stale default by accident.
//
// WHAT THIS DOES NOT COVER (adversarial review, stated so it is not over-read):
// the selector reads the DECLARED `model_tier_default`, not the tier a phase
// actually dispatches at. `subagent.applyModelTierOverride` floor-escalates a
// profile via `model_tier_overrides[situation]`, and one such situation is live
// today — `cycle_1_or_low_goal` fires whenever Cycle <= 1, so scout.json
// (codex, balanced, effort low) really does dispatch at DEEP tier on cycle 1
// while its effort stays low. Whether an ESCALATED tier should also raise
// effort is a cost decision for the operator, not something a guard should
// decide silently, so it is deliberately left alone and recorded here instead.
// builder/tester/evaluator carry the same shape but their situations are not
// yet plumbed.
//
// RUN WITH -count=1 WHEN ONLY A PROFILE JSON CHANGED. Go's test cache does not
// track reads that escape the module root via "..", so a bare `go test` serves
// a stale PASS after a profile-only edit — reproduced: cached "ok" while the
// regression was live on disk. CI and `make test` already pass -count=1; the
// exposure is local verification.
func TestCodexDeepTierProfilesAllRunAtMax(t *testing.T) {
	dir := effortProfilesDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		// FATAL, deliberately diverging from the sibling guard in
		// internal/bridge, which SKIPs when the profiles dir is missing. That
		// leniency suits a realizability check that can be legitimately
		// unrunnable; it does not suit this one. `.evolve/profiles` is tracked
		// and always present in a real checkout, so an unreadable dir here
		// means the guard is not looking at anything — and a guard that goes
		// quiet when its input vanishes is the failure mode this file exists
		// to prevent.
		t.Fatalf("read profiles dir: %v", err)
	}
	checked := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		var p struct {
			CLI              string `json:"cli"`
			ModelTierDefault string `json:"model_tier_default"`
			EffortLevel      string `json:"effort_level"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			// Do NOT skip. A corrupted profile silently dropping out of
			// `checked` would undercut the vacuity assertion below: the guard
			// would report "selector matched profiles" while quietly not
			// examining this one.
			t.Errorf("profile %s: unreadable JSON (%v) — it cannot be checked, so it cannot be trusted", e.Name(), err)
			continue
		}
		if p.CLI != "codex-tmux" || (p.ModelTierDefault != "deep" && p.ModelTierDefault != "top") {
			continue
		}
		checked++
		if p.EffortLevel != codexDeepTopEffort {
			t.Errorf("profile %s: codex %s-tier effort_level = %q, want \"max\" (2026-08-28 operator directive). A codex deep/top phase left at a lower rung runs quieter than its siblings with nothing reporting it.",
				e.Name(), p.ModelTierDefault, p.EffortLevel)
		}
	}
	// Guard the guard: if the selector ever stops matching (a cli rename, a
	// field rename), this must fail loudly rather than pass over zero profiles.
	if checked == 0 {
		t.Fatal("matched NO codex deep/top profiles — the selector is broken and this guard is vacuous")
	}
}
