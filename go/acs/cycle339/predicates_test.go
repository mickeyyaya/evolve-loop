//go:build acs

package cycle339

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

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
