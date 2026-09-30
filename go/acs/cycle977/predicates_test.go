//go:build acs

package cycle977

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/subagent"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func stubProfile(body string) func(string) (string, error) {
	return func(string) (string, error) { return body, nil }
}

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

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

func TestC977_005_ReusesPolicyTierRankAndVetsClean(t *testing.T) {
	dir := goDir(t)
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

func tailLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
