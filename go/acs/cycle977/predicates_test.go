//go:build acs

// Package cycle977 materializes the cycle-977 acceptance criteria for this
// fleet lane's single scoped id `wire-or-remove-model-tier-overrides-consumer`.
//
// Background. cycle-974 greened its C974_003 disjunction via the REMOVED branch
// (it deleted three below-floor inert keys). But the remaining, semantically
// meaningful overrides — e.g. scout.json `cycle_1_or_low_goal=deep` — are STILL
// inert: `ResolveModelTier` (go/internal/subagent/modeltier.go) never consults
// `model_tier_overrides`. The "no inert API" goal constraint is unfinished.
// Triage committed exactly one top_n task (T1 — WIRE, not remove: removal would
// break the archived cycle974/cycle339 predicate compile that field-accesses
// `.ModelTierOverrides`). Predicates are authored for T1 only.
//
// Builder contract (from scout T1): in `ResolveModelTier`, after computing the
// base tier, read the profile's `model_tier_overrides`, select the active
// situation from the real request signal (`cycle_1_or_low_goal` active when
// req.Cycle <= 1), clamp the override value to the profile's
// `model_tier_envelope` via the EXISTING `policy.TierRank` (no new rank table),
// and apply it as a FLOOR (max(base, clampedOverride)). Vocabulary stays
// abstract (fast/balanced/deep/top). Empty/nil override map ⇒ base unchanged.
//
// PREDICATE STYLE (cycle-85 rule): go/internal/subagent is importable from
// go/acs, so the load-bearing predicates C977_001/002/003 EXERCISE the live
// `subagent.ResolveModelTier` (the SUT) — they inject profile bodies (and the
// real scout.json) and assert on the RETURNED tier. A magic-string source edit
// cannot satisfy them; the resolver must actually consume the override. No
// predicate's sole assertion is a source grep.
//
// Adversarial diversity (skills/adversarial-testing §6):
//
//	NEGATIVE → C977_002 "clamp above max" (override=top, envelope max=deep) must
//	           return deep, NOT top: a naive apply-without-clamp is rejected, and
//	           the current ignore-override state (returns balanced) red-fails it.
//	           C977_003 asserts Cycle=5 returns the base tier (override inactive)
//	           so a hardcoded "always deep" no-op cannot pass.
//	EDGE     → C977_002 tables the nil/absent-map edge (base unchanged) and the
//	           floor-no-lower edge (override below default does not demote).
//	SEMANTIC → C977_001 (consumer raises within envelope), C977_003 (composed
//	           real-config producer-keyed path), C977_004 (cycle-974 regression),
//	           and C977_005 (rank-table reuse + vet clean) are DISTINCT outcomes.
//
// RED before T1: the resolver ignores overrides, so C977_001 (want deep, got
// balanced), C977_002's raise/clamp rows (got balanced), and C977_003's Cycle=1
// row (got balanced) all fail; C977_005 fails because modeltier.go does not yet
// reference policy.TierRank. C977_004 is regression coverage — green now (via
// cycle-974's REMOVED branch) and MUST stay green after wiring.
//
// AC map (1:1 with test-report.md AC-Materialization table):
//
//	AC1 production consumer reads+applies the override → C977_001 (predicate)
//	AC3 clamp-to-max negative + nil/floor edges         → C977_002 (predicate)
//	AC4 composed-path proof on real scout.json          → C977_003 (predicate)
//	AC2 cycle-974 predicate regression stays green       → C977_004 (predicate)
//	AC5 reuse policy.TierRank (no dup table) + vet clean  → C977_005 (predicate)
package cycle977

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/subagent"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// stubProfile returns a ReadProfile seam that yields the same JSON body for any
// path — the same injection style as go/internal/subagent/modeltier_test.go.
func stubProfile(body string) func(string) (string, error) {
	return func(string) (string, error) { return body, nil }
}

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// TestC977_001_OverrideConsumedRaisesTierWithinEnvelope proves a PRODUCTION
// consumer exists by exercising the exported resolver: a scout profile with
// default=balanced, envelope [balanced,deep] and override
// `cycle_1_or_low_goal=deep` must resolve to `deep` at Cycle==1 (the situation's
// real producer signal), because deep sits inside the envelope. The resolver
// returning the abstract `deep` string is only possible if the override map is
// actually read and applied — a field-access proof that a source edit cannot
// fake.
//
// RED before T1: the resolver ignores overrides and returns the base
// `balanced`.
func TestC977_001_OverrideConsumedRaisesTierWithinEnvelope(t *testing.T) {
	const profile = `{
		"role": "scout",
		"model_tier_default": "balanced",
		"model_tier_envelope": {"min": "balanced", "default": "balanced", "max": "deep"},
		"model_tier_overrides": {"cycle_1_or_low_goal": "deep"}
	}`

	tier, err := subagent.ResolveModelTier(
		subagent.ResolveModelTierRequest{ProfilePath: "/p", Cycle: 1},
		subagent.ResolveModelTierOptions{ReadProfile: stubProfile(profile)},
	)
	if err != nil {
		t.Fatalf("ResolveModelTier: unexpected error: %v", err)
	}
	if tier != "deep" {
		t.Errorf("RED: override cycle_1_or_low_goal=deep (within envelope) not"+
			" consumed at Cycle=1 — got %q, want \"deep\". ResolveModelTier must"+
			" read profile.model_tier_overrides and apply the active situation.",
			tier)
	}
}

// TestC977_002_OverrideClampedAndEdges tables the clamp/floor/nil behavior of
// the override consumer. The load-bearing NEGATIVE row is "clamp above max":
// an override of `top` under an envelope whose max is `deep` must resolve to
// `deep` — distinguishing the correct clamp both from the current ignore-state
// (which yields `balanced`) and from a naive no-clamp apply (which would yield
// `top`). The remaining rows pin the edges: an absent map leaves the base
// untouched, an override below the default does not demote (floor semantics),
// and an inactive situation (Cycle beyond the low-goal window) does not apply.
//
// RED before T1: the "clamp above max" row wants `deep` but the un-wired
// resolver returns `balanced`, so the whole test red-fails. The edge rows are
// green now and after (guards against an over-eager implementation).
func TestC977_002_OverrideClampedAndEdges(t *testing.T) {
	cases := []struct {
		name    string
		profile string
		cycle   int
		want    string
	}{
		{
			name: "clamp above max (top -> envelope max deep)",
			profile: `{"role":"scout","model_tier_default":"balanced",
				"model_tier_envelope":{"min":"balanced","max":"deep"},
				"model_tier_overrides":{"cycle_1_or_low_goal":"top"}}`,
			cycle: 1,
			want:  "deep",
		},
		{
			name: "nil/absent override map leaves base unchanged",
			profile: `{"role":"scout","model_tier_default":"balanced",
				"model_tier_envelope":{"min":"balanced","max":"deep"}}`,
			cycle: 1,
			want:  "balanced",
		},
		{
			name: "floor: override below default does not demote",
			profile: `{"role":"scout","model_tier_default":"deep",
				"model_tier_envelope":{"min":"balanced","max":"deep"},
				"model_tier_overrides":{"cycle_1_or_low_goal":"balanced"}}`,
			cycle: 1,
			want:  "deep",
		},
		{
			name: "situation inactive (high cycle) does not apply override",
			profile: `{"role":"scout","model_tier_default":"balanced",
				"model_tier_envelope":{"min":"balanced","max":"deep"},
				"model_tier_overrides":{"cycle_1_or_low_goal":"deep"}}`,
			cycle: 5,
			want:  "balanced",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tier, err := subagent.ResolveModelTier(
				subagent.ResolveModelTierRequest{ProfilePath: "/p", Cycle: tc.cycle},
				subagent.ResolveModelTierOptions{ReadProfile: stubProfile(tc.profile)},
			)
			if err != nil {
				t.Fatalf("ResolveModelTier: unexpected error: %v", err)
			}
			if tier != tc.want {
				t.Errorf("RED: got %q, want %q — the override consumer must clamp to"+
					" the envelope max, apply as a floor, and only fire for the active"+
					" situation.", tier, tc.want)
			}
		})
	}
}

// TestC977_003_ComposedPathRealScoutProfile is the composed-path wiring proof
// against the REAL .evolve/profiles/scout.json (default=balanced, envelope max
// deep, override cycle_1_or_low_goal=deep). It uses the production ReadProfile
// seam (os.ReadFile). At Cycle==1 the resolver must observe a `balanced -> deep`
// bump; at Cycle==5 the situation is inactive so the base `balanced` stands.
// The Cycle==5 assertion is the anti-no-op: a hardcoded "always deep" cannot
// satisfy both rows.
//
// RED before T1: the Cycle==1 row wants `deep` but the un-wired resolver
// returns `balanced`.
func TestC977_003_ComposedPathRealScoutProfile(t *testing.T) {
	scoutPath := filepath.Join(acsassert.RepoRoot(t), ".evolve", "profiles", "scout.json")

	active, err := subagent.ResolveModelTier(
		subagent.ResolveModelTierRequest{ProfilePath: scoutPath, Cycle: 1},
		subagent.ResolveModelTierOptions{},
	)
	if err != nil {
		t.Fatalf("ResolveModelTier(scout.json, Cycle=1): %v", err)
	}
	if active != "deep" {
		t.Errorf("RED: real scout.json + Cycle=1 must resolve to \"deep\""+
			" (balanced->deep via override cycle_1_or_low_goal within envelope) —"+
			" got %q. This is the composed-path wiring proof.", active)
	}

	inactive, err := subagent.ResolveModelTier(
		subagent.ResolveModelTierRequest{ProfilePath: scoutPath, Cycle: 5},
		subagent.ResolveModelTierOptions{},
	)
	if err != nil {
		t.Fatalf("ResolveModelTier(scout.json, Cycle=5): %v", err)
	}
	if inactive != "balanced" {
		t.Errorf("anti-no-op: real scout.json + Cycle=5 must stay at the base"+
			" \"balanced\" (cycle_1_or_low_goal inactive) — got %q. The override"+
			" must be keyed off the real producer signal, not hardcoded.", inactive)
	}
}

// TestC977_004_Cycle974RegressionStillGreen is regression coverage for AC2: the
// cycle-974 predicate that first flagged the inert map
// (TestC974_003_OverridesConsumedOrRemoved) must still pass after this cycle's
// WIRE. It runs the cycle-974 acs predicate via a `go test -tags acs`
// subprocess. Green now (cycle-974's REMOVED branch) and green after (the WIRED
// branch) — a regression guard, not a state flip.
func TestC977_004_Cycle974RegressionStillGreen(t *testing.T) {
	dir := goDir(t)
	out, serr, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", dir,
		"-tags", "acs",
		"-count=1", "-v",
		"-run", "TestC974_003_OverridesConsumedOrRemoved",
		"./acs/cycle974/...")
	if err != nil {
		t.Fatalf("RED: cycle-974 regression predicate failed (exit=%d) — wiring"+
			" the consumer must NOT break the cycle-974 no-inert-API guard:\n%s\n%s",
			code, tailLines(out, 30), tailLines(serr, 10))
	}
	passRe := regexp.MustCompile(`(?m)^--- PASS: TestC974_003_OverridesConsumedOrRemoved`)
	if !passRe.MatchString(out) {
		t.Errorf("RED: TestC974_003_OverridesConsumedOrRemoved not observed as a"+
			" PASS (exit=%d):\n%s", code, tailLines(out, 30))
	}
}

// TestC977_005_ReusesPolicyTierRankAndVetsClean enforces AC5: the override
// consumer must reuse the EXISTING policy.TierRank for its envelope clamp (no
// duplicate rank table — standing rule never_duplicate_centralize), and the
// subagent package must build/vet clean. The `go vet` subprocess is the
// load-bearing, non-degenerate assertion; the reuse check is a structural guard
// that a private rank map was not introduced.
//
// RED before T1: modeltier.go does not yet reference policy.TierRank.
func TestC977_005_ReusesPolicyTierRankAndVetsClean(t *testing.T) {
	dir := goDir(t)
	// Build+vet the consumer package: catches an override consumer that fails to
	// compile or that `go vet` flags.
	if _, serr, code, err := acsassert.SubprocessOutput(
		"go", "vet", "-C", dir, "./internal/subagent/..."); err != nil {
		t.Fatalf("RED: `go vet ./internal/subagent/...` failed (exit=%d):\n%s",
			code, tailLines(serr, 30))
	}

	modeltier := filepath.Join(dir, "internal", "subagent", "modeltier.go")
	if !acsassert.FileContainsAny(modeltier, "policy.TierRank", "TierRank(") {
		t.Errorf("RED: modeltier.go does not reference policy.TierRank — the" +
			" envelope clamp must REUSE the existing rank table" +
			" (never_duplicate_centralize), not introduce a new one.")
	}
}

// tailLines returns the last n lines of s (subprocess output trimming).
func tailLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
