//go:build acs

// Package cycle339 materializes the cycle-339 acceptance criteria for the two
// committed top_n tasks (driver-agnostic model-routing campaign):
//
//	T1  migrate-domain-profiles-to-canonical-tiers — all 79 domain/optional
//	    profiles must use canonical capability tiers (fast/balanced/deep) instead
//	    of vendor model names (sonnet/opus/haiku), including overrides and
//	    envelope fields. 4 envelope-driven exceptions apply (memo/evaluator→fast,
//	    plan-reviewer/retrospective→deep).
//	T2  widen-driver-agnostic-acceptance-test — TestAllProfilesAreDriverAgnostic
//	    dynamically enumerates all profiles via os.ReadDir and passes.
//
// Predicates are BEHAVIORAL (cycle-85 lesson): they invoke the system under
// test — the live profiles.Loader for T1, and a go test subprocess for T2.
// No load-bearing assertion is "does source file contain text X".
//
// AC map (1:1 with scout-report.md "Acceptance Criteria Summary"):
//
//	T1.defaults   no vendor names in model_tier_default (79 profiles)   → C339_001
//	T1.overrides  no vendor names in overrides/envelope (14 overrides)  → C339_002
//	T1.exceptions envelope-driven 4 profiles get correct special tiers  → C339_003
//	T2.test       TestAllProfilesAreDriverAgnostic exists and passes     → C339_004
package cycle339

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolveloop/go/internal/profiles"
	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// vendorTiers is the set of legacy vendor model names that must not appear in
// any profile tier field after migration.
var vendorTiers = map[string]bool{"sonnet": true, "opus": true, "haiku": true}

func canonicalSet() map[string]bool {
	m := make(map[string]bool, len(modelcatalog.CanonicalTiers))
	for _, t := range modelcatalog.CanonicalTiers {
		m[t] = true
	}
	return m
}

func profilesDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), ".evolve", "profiles")
}

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// TestC339_001_NoVendorNamesInProfileDefaultTiers verifies that every profile in
// .evolve/profiles/ uses a canonical capability tier (fast/balanced/deep) for
// model_tier_default — never a vendor model name (sonnet/opus/haiku).
//
// Uses profiles.Loader (the live system under test) to enumerate and load all
// profiles, then asserts on the typed ModelTierDefault field. A magic string
// addition to source cannot satisfy this — the profile JSON files must change.
//
// RED before T1: 79 profiles have "sonnet" or "opus" as model_tier_default.
func TestC339_001_NoVendorNamesInProfileDefaultTiers(t *testing.T) {
	canonical := canonicalSet()
	loader := profiles.NewFromDir(profilesDir(t))

	names, err := loader.List()
	if err != nil {
		t.Fatalf("profiles.Loader.List: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("RED: profiles directory is empty — expected ≥89 profiles")
	}

	var bad []string
	for _, name := range names {
		p, lerr := loader.Get(name)
		if lerr != nil {
			t.Errorf("load %q: %v", name, lerr)
			continue
		}
		if !canonical[p.ModelTierDefault] {
			bad = append(bad, name+": model_tier_default="+p.ModelTierDefault)
		}
	}
	if len(bad) > 0 {
		t.Errorf("RED: %d profiles still use vendor model names in model_tier_default"+
			" (migrate to fast/balanced/deep):\n  %s",
			len(bad), strings.Join(bad, "\n  "))
	}
}

// TestC339_002_NoVendorNamesInProfileOverridesAndEnvelopes verifies that
// model_tier_overrides and model_tier_envelope fields also use canonical tiers.
// Covers the 14 overrides across evaluator/inspirer/intent/orchestrator/
// plan-reviewer/retrospective/tester and any envelope fields with vendor names.
//
// RED before T1: 14 override values carry vendor names (haiku/sonnet/opus).
func TestC339_002_NoVendorNamesInProfileOverridesAndEnvelopes(t *testing.T) {
	loader := profiles.NewFromDir(profilesDir(t))

	names, err := loader.List()
	if err != nil {
		t.Fatalf("profiles.Loader.List: %v", err)
	}

	var bad []string
	for _, name := range names {
		p, lerr := loader.Get(name)
		if lerr != nil {
			t.Errorf("load %q: %v", name, lerr)
			continue
		}
		for key, tier := range p.ModelTierOverrides {
			if vendorTiers[tier] {
				bad = append(bad, name+": model_tier_overrides["+key+"]="+tier)
			}
		}
		if env := p.ModelTierEnvelope; env != nil {
			for _, entry := range []struct{ label, tier string }{
				{"min", env.Min},
				{"default", env.Default},
				{"max", env.Max},
			} {
				if entry.tier != "" && vendorTiers[entry.tier] {
					bad = append(bad, name+": model_tier_envelope."+entry.label+"="+entry.tier)
				}
			}
		}
	}
	if len(bad) > 0 {
		t.Errorf("RED: %d override/envelope fields still use vendor names"+
			" (must be canonical tiers fast/balanced/deep):\n  %s",
			len(bad), strings.Join(bad, "\n  "))
	}
}

// TestC339_003_EnvelopeDrivenExceptionsApplied verifies the 4 profiles that must
// NOT be naively mapped to "balanced" (the default for sonnet→balanced):
//
//	memo:          sonnet→fast   (envelope default=fast)
//	evaluator:     sonnet→fast   (envelope default=fast)
//	plan-reviewer: sonnet→deep   (envelope default=deep)
//	retrospective: sonnet→deep   (envelope default=deep)
//
// This is the adversarial anti-no-op check: a bulk "sonnet→balanced" migration
// passes C339_001 but fails here because the envelope-aware mapping is wrong.
//
// RED before T1: all 4 still have model_tier_default="sonnet".
func TestC339_003_EnvelopeDrivenExceptionsApplied(t *testing.T) {
	loader := profiles.NewFromDir(profilesDir(t))

	cases := []struct {
		profile  string
		wantTier string
		reason   string
	}{
		{"memo", "fast", "envelope default=fast; naive sonnet→balanced would be wrong"},
		{"evaluator", "fast", "envelope default=fast; naive sonnet→balanced would be wrong"},
		{"plan-reviewer", "deep", "envelope default=deep; naive sonnet→balanced would be wrong"},
		{"retrospective", "deep", "envelope default=deep; naive sonnet→balanced would be wrong"},
	}

	for _, tc := range cases {
		p, err := loader.Get(tc.profile)
		if err != nil {
			t.Errorf("load profile %q: %v", tc.profile, err)
			continue
		}
		if p.ModelTierDefault != tc.wantTier {
			t.Errorf("RED: %s: model_tier_default=%q, want %q (%s)",
				tc.profile, p.ModelTierDefault, tc.wantTier, tc.reason)
		}
	}
}

// TestC339_004_AllProfilesDriverAgnosticTestPasses verifies that
// TestAllProfilesAreDriverAgnostic exists in go/internal/profiles/ and passes.
// The test must dynamically enumerate all profiles and assert canonical tiers.
//
// RED before T2: the test function does not exist yet → no PASS line in output.
// RED after T1 but before T2: if Builder migrates profiles but forgets the test.
func TestC339_004_AllProfilesDriverAgnosticTestPasses(t *testing.T) {
	dir := goDir(t)
	out, _, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", dir,
		"-count=1", "-v",
		"-run", "TestAllProfilesAreDriverAgnostic",
		"./internal/profiles/...")
	if err != nil {
		t.Fatalf("go test subprocess error: %v", err)
	}
	passRe := regexp.MustCompile(`(?m)^--- PASS: TestAllProfilesAreDriverAgnostic`)
	if !passRe.MatchString(out) {
		t.Errorf("RED: TestAllProfilesAreDriverAgnostic not found as a PASS in test output"+
			" (exit=%d) — Builder must add the dynamic all-profiles test.\nOut (tail):\n%s",
			code, tailLines(out, 30))
	}
}

func tailLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
