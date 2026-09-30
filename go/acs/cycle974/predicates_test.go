//go:build acs

package cycle974

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func profilesDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), ".evolve", "profiles")
}

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func tierRank() map[string]int {
	rank := make(map[string]int, len(modelcatalog.CanonicalTiers))
	for i, tier := range modelcatalog.CanonicalTiers {
		rank[tier] = i
	}
	return rank
}

func TestC974_001_OverridesWithinEnvelope(t *testing.T) {
	rank := tierRank()
	loader := profiles.NewFromDir(profilesDir(t))

	names, err := loader.List()
	if err != nil {
		t.Fatalf("profiles.Loader.List: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("RED: profiles directory is empty — expected ≥80 profiles")
	}

	var bad []string
	for _, name := range names {
		p, lerr := loader.Get(name)
		if lerr != nil {
			t.Errorf("load %q: %v", name, lerr)
			continue
		}
		env := p.ModelTierEnvelope
		if env == nil || env.Min == "" || env.Max == "" {
			continue
		}
		minR, okMin := rank[env.Min]
		maxR, okMax := rank[env.Max]
		if !okMin || !okMax {
			continue
		}
		for key, tier := range p.ModelTierOverrides {
			r, ok := rank[tier]
			if !ok {
				continue
			}
			if r < minR || r > maxR {
				bad = append(bad, name+": model_tier_overrides["+key+"]="+tier+
					" outside envelope [min="+env.Min+", max="+env.Max+"]")
			}
		}
	}
	if len(bad) > 0 {
		t.Errorf("RED: %d model_tier_overrides value(s) fall outside their own"+
			" profile's envelope [min,max] (raise the override to the floor, or"+
			" widen the envelope with a justifying comment):\n  %s",
			len(bad), strings.Join(bad, "\n  "))
	}
}

func TestC974_002_EnvelopeGuardTestExistsAndPasses(t *testing.T) {
	dir := goDir(t)
	out, _, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", dir,
		"-count=1", "-v",
		"-run", "TestModelTierOverridesWithinEnvelope",
		"./internal/profiles/...")
	if err != nil {
		t.Fatalf("RED: go test subprocess for the envelope guard exited non-zero"+
			" (exit=%d) — the guard test exists but does not pass:\n%s",
			code, tailLines(out, 30))
	}
	passRe := regexp.MustCompile(`(?m)^--- PASS: TestModelTierOverridesWithinEnvelope`)
	if !passRe.MatchString(out) {
		t.Errorf("RED: TestModelTierOverridesWithinEnvelope not found as a PASS in"+
			" test output (exit=%d) — Builder must add the permanent envelope"+
			" regression guard to package profiles.\nOut (tail):\n%s",
			code, tailLines(out, 30))
	}
}

func TestC974_003_OverridesConsumedOrRemoved(t *testing.T) {
	inertKeys := map[string]bool{
		"cycle_4_plus_mature":  true,
		"s_complex_with_cache": true,
		"routine_review":       true,
	}

	loader := profiles.NewFromDir(profilesDir(t))
	names, err := loader.List()
	if err != nil {
		t.Fatalf("profiles.Loader.List: %v", err)
	}
	var remaining []string
	for _, name := range names {
		p, lerr := loader.Get(name)
		if lerr != nil {
			t.Errorf("load %q: %v", name, lerr)
			continue
		}
		for key := range p.ModelTierOverrides {
			if inertKeys[key] {
				remaining = append(remaining, name+": model_tier_overrides["+key+"]")
			}
		}
	}
	removed := len(remaining) == 0

	wired := hasProductionConsumer(t)

	if !wired && !removed {
		t.Errorf("RED: ModelTierOverrides is INERT — no production consumer accesses"+
			" .ModelTierOverrides AND %d inert override entr(y|ies) remain."+
			" Builder must WIRE a real consumer or REMOVE the dead entries"+
			" (no-inert-API goal constraint):\n  %s",
			len(remaining), strings.Join(remaining, "\n  "))
	}
}

func hasProductionConsumer(t *testing.T) bool {
	t.Helper()
	root := filepath.Join(acsassert.RepoRoot(t), "go", "internal")
	found := false
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if acsassert.FileContains(dummyTB{}, path, ".ModelTierOverrides") {
			found = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk go/internal: %v", err)
	}
	return found
}

type dummyTB struct{}

func (dummyTB) Helper()               {}
func (dummyTB) Errorf(string, ...any) {}

func tailLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
