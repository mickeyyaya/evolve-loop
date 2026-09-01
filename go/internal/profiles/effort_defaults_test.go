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
	// 2026-09-01 operator directive: the CODEX-routed deep/top phases
	// (gpt-5.6-sol) run at HIGH — superseding 2026-08-28's max rung (which
	// had superseded 2026-08-24's xhigh). Max is codex's most quota-hungry
	// rung ("Max and Ultra consume usage limits faster"); the operator traded
	// one rung of reasoning for quota headroom. Note the deliberate inversion
	// this creates: the CLAUDE-routed deep/top graders stay at xhigh (above
	// codex's high) — Anthropic's Opus guidance (docs/research/
	// fable-simulation-2026/model-profiles.md) recommends xhigh for agentic
	// work, and the graders are the adversarial quality floor. The abstract
	// effort dial is realized per family; the split is by design, not drift.
	// The fast/balanced rows keep the cycle-566 cost matrix.
	want := map[string]string{
		"scout":              "low",
		"triage":             "low",
		"tdd-engineer":       "medium",
		"builder":            "medium",
		"auditor":            "xhigh",
		"adversarial-review": "xhigh",
		"retrospective":      codexDeepTopRung,
		"premise-challenge":  codexDeepTopRung,
		"intent":             codexDeepTopRung,
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

// codexDeepTopRung is the single source for the codex deep/top effort rung —
// the one edit a directive change requires (proven: 2026-08-28 max, 2026-09-01
// high). Read by the matrix pin's codex rows and the class guard.
const codexDeepTopRung = "high"

// maxEffortRung names the literal top rung for the CLI-agnostic converse
// guard: "max" is the most expensive rung on every family that exposes it,
// and it may only ever appear on a deep/top profile. Since 2026-09-01 nothing
// runs at max (codex deep/top moved to high), so the converse polices a
// currently-empty set — kept armed for any future adoption.
const maxEffortRung = "max"

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
func TestCodexDeepTierProfilesAllRunAtDirectedRung(t *testing.T) {
	// Through RealTreeProfiles, NOT a raw directory scan. The runtime mints
	// UNTRACKED profile stubs into .evolve/profiles; a scanner that binds
	// everything on disk reds on state that can never reach a CI checkout
	// (the 2026-08-09 zero-ship batch, fingerprint cd49274beab2).
	//
	// This is not hypothetical here: the first version of this guard DID scan
	// raw, and it failed in the live runtime plane on two minted stubs
	// (disposition-preflight, regression-predicate-precheck) that carry no
	// effort_level at all — it would have red the test gate on every cycle.
	loader, names := RealTreeProfiles(t)
	checked := 0
	for _, name := range names {
		p, err := loader.Get(name)
		if err != nil {
			// Reported, not skipped. Get does more than decode — it also
			// expands $include_policy — so a tracked profile with perfectly
			// valid routing fields can fail here for an unrelated reason and
			// silently drop out of `checked`, weakening the vacuity guard.
			// Loud beats quiet: the guard exists to notice things.
			t.Errorf("profile %s: Get failed (%v) — it cannot be checked, so it cannot be trusted", name, err)
			continue
		}
		if p.CLI != "codex-tmux" || (p.ModelTierDefault != "deep" && p.ModelTierDefault != "top") {
			continue
		}
		checked++
		if p.EffortLevel != codexDeepTopRung {
			t.Errorf("profile %s: codex %s-tier effort_level = %q, want %q (2026-09-01 operator directive). A codex deep/top phase off the directed rung runs differently than its siblings with nothing reporting it.",
				name, p.ModelTierDefault, p.EffortLevel, codexDeepTopRung)
		}
	}
	if checked == 0 {
		t.Fatal("matched NO tracked codex deep/top profiles — the selector is broken and this guard is vacuous")
	}
}

// The CONVERSE of the class guard above (placement law), per the 2026-08-29
// operator directive: "the max thinking level should only apply to deep/top
// model". Together the two make it a biconditional for codex — deep/top ⟺ max —
// and this half alone constrains EVERY family, so `max` can never drift onto a
// fast/balanced phase.
//
// Why pin a constraint nothing violates today: max is the most expensive rung
// on every CLI that has one (codex's own picker warns "Max and Ultra consume
// usage limits faster"). A fast/balanced phase is fast/balanced BECAUSE it was
// costed that way — scout and triage sit at low under the cycle-566 matrix.
// Silently promoting one to max would raise spend with nothing reporting it:
// the same shape as every other defect this file guards, a change nobody sees.
//
// CLI-AGNOSTIC on purpose. The directive is about thinking level vs model tier,
// not about codex — claude exposes max too, and the moment a claude profile
// adopts it the same rule must hold.
//
// NOTE on escalation: this reads the DECLARED tier, like its sibling. A phase
// floor-escalated at dispatch (scout.json → deep on cycle 1 via
// model_tier_overrides) keeps its declared effort, and that is COMPLIANT:
// scout runs low, and low is not max. The directive restricts where max may
// APPEAR; it does not require an escalated phase to adopt it.
func TestMaxEffortOnlyOnDeepOrTopProfiles(t *testing.T) {
	// Same funnel, same reason — see the sibling above.
	loader, names := RealTreeProfiles(t)
	checked := 0
	for _, name := range names {
		p, err := loader.Get(name)
		if err != nil {
			// Reported, not skipped. Get does more than decode — it also
			// expands $include_policy — so a tracked profile with perfectly
			// valid routing fields can fail here for an unrelated reason and
			// silently drop out of `checked`, weakening the vacuity guard.
			// Loud beats quiet: the guard exists to notice things.
			t.Errorf("profile %s: Get failed (%v) — it cannot be checked, so it cannot be trusted", name, err)
			continue
		}
		if p.EffortLevel != maxEffortRung {
			continue
		}
		checked++
		// An unset model_tier_default resolves to "balanced" (resolvellm), so
		// an omitted field is a violation, not an exemption.
		tier := p.ModelTierDefault
		if tier == "" {
			tier = "balanced"
		}
		if tier != "deep" && tier != "top" {
			t.Errorf("profile %s (cli %s): effort_level %q on a %q-tier phase — max is reserved for deep/top models (2026-08-29 directive). It is the most expensive rung; a fast/balanced phase was costed that way deliberately.",
				name, p.CLI, p.EffortLevel, tier)
		}
	}
	if checked == 0 {
		// Since 2026-09-01 this is the EXPECTED state: codex deep/top moved to
		// high, so no tracked profile carries max. The guard stays armed for
		// any future max adoption. Decode-health delegation, precisely: the
		// sibling class guard's per-profile comparison reads the same
		// EffortLevel field through the same loader and fails loudly (Errorf)
		// on a decode regression; its zero-match Fatal is a separate
		// CLI/tier-selector check, not an EffortLevel guard.
		t.Logf("no tracked profile at effort %q — expected since the 2026-09-01 directive; placement law armed for future adoption", maxEffortRung)
	}
}
