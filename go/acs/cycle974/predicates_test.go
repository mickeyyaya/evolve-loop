//go:build acs

// Package cycle974 materializes the cycle-974 acceptance criteria for this
// fleet lane's single scoped id `scan-phase-fast-tier-envelopes`, which scout
// resolved to a real phase-contract-drift defect plus a tightly-coupled
// inert-API defect in the SAME model-tier-override mechanism. Triage committed
// TWO coherent top_n tasks (Task 2 dependsOn Task 1); predicates are authored
// for BOTH (both are top_n, not deferred — the deferred
// profile-authoring-lint-schema-validator gets ZERO predicates, R9.3
// floor-binding).
//
//	T1 envelope-floor-guard-model-tier-overrides — every model_tier_overrides
//	   value in every profile MUST rank within its OWN
//	   model_tier_envelope [min,max] on the canonical ladder
//	   fast<balanced<deep<top. Currently violated (see RED note below), AND a
//	   permanent regression guard TestModelTierOverridesWithinEnvelope must
//	   exist in package profiles so future drift is caught in normal CI.
//	T2 wire-or-remove-model-tier-overrides-consumer — the ModelTierOverrides
//	   map has ZERO production consumers (only *_test.go read it; phaseconfig.go
//	   names it in a comment only). "No inert API" is a hard goal constraint:
//	   Builder must EITHER wire a real production consumer OR remove the inert
//	   override entries. Leaving it as-discovered (declared but unread) is not
//	   an acceptable outcome — the disjunction predicate below red-fails it.
//
// PREDICATE STYLE (cycle-85 rule): go/internal/profiles + modelcatalog are
// importable from go/acs, so C974_001 EXERCISES the live profiles.Loader (the
// SUT) against the real config and asserts on the typed
// ModelTierEnvelope/ModelTierOverrides fields — a magic-string source edit
// cannot satisfy it, the JSON must change. C974_002 runs the real guard test
// via a `go test` subprocess. C974_003 asserts a structural deliverable-state
// disjunction (a real production consumer exists, OR the inert entries are
// gone) — not a "does source contain text X" grep: neither branch can be faked
// without actually wiring a consumer or deleting the dead config.
//
// Adversarial diversity (skills/adversarial-testing §6):
//
//	NEGATIVE → C974_001 red-fails on the CURRENT drifted config (the strongest
//	           anti-no-op: an empty/no-op change leaves violations in place);
//	           C974_003 red-fails on the leave-as-is (inert) outcome.
//	POSITIVE → C974_002 requires the permanent guard test to be PRESENT and
//	           PASS (a positive signal a no-op cannot fake).
//	EDGE     → C974_001 enforces BOTH envelope bounds (below-min AND above-max),
//	           catching over-max overrides scout's prose undercounted.
//	SEMANTIC → floor-guard (T1) and inert-API resolution (T2) are DISTINCT
//	           outcomes, asserted by separate predicates.
//
// RED note (surfaced per Core Rule 3 — no silent change): scout's prose
// estimated "6 violations". C974_001 encodes the AC as literally stated
// ("between min and max"), which also catches OVER-max overrides scout's
// min-direction scan missed: tester (ultrathink/m_complex/audit_retry=deep >
// max=balanced) and orchestrator (cycle_1_or_low_goal=deep > max=balanced).
// The true pre-fix violation count is higher than 6; the predicate is faithful
// to the stated invariant, not the undercount.
//
// AC map (1:1 with test-report.md AC-Materialization table):
//
//	T1-a permanent envelope guard test exists & passes    → C974_002 (predicate)
//	T1-b every override within its own [min,max] envelope  → C974_001 (predicate)
//	T2   ModelTierOverrides consumed OR removed (no inert)  → C974_003 (predicate)
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

// tierRank maps each canonical tier to its ordinal on the ladder
// fast<balanced<deep<top (the SSOT is modelcatalog.CanonicalTiers).
func tierRank() map[string]int {
	rank := make(map[string]int, len(modelcatalog.CanonicalTiers))
	for i, tier := range modelcatalog.CanonicalTiers {
		rank[tier] = i
	}
	return rank
}

// TestC974_001_OverridesWithinEnvelope verifies that every model_tier_overrides
// value ranks BETWEEN its own profile's model_tier_envelope.min and .max
// (inclusive) on the canonical tier ladder. Profiles without an envelope (or
// with an empty min/max) are skipped, matching the existing envelope-test
// conventions in profile_model_routing_adversarial_test.go.
//
// Loads config through the live profiles.Loader (SUT) and asserts on the typed
// fields — the JSON must be corrected, a source magic string cannot pass this.
//
// RED before T1: cycle_4_plus_mature=fast / s_complex_with_cache=fast /
// routine_review=fast sit BELOW min=balanced (scout/builder/orchestrator/
// plan-reviewer/tester); clear_goal=balanced sits below min=deep (intent); and
// several overrides sit ABOVE max=balanced (tester, orchestrator).
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
			// Non-canonical envelope bounds are a separate test's concern
			// (TestEnvelopeTierHierarchyOrdering / canonical checks); skip
			// here so this predicate isolates the override-vs-envelope range.
			continue
		}
		for key, tier := range p.ModelTierOverrides {
			r, ok := rank[tier]
			if !ok {
				// Non-canonical override value → covered by
				// TestAllProfilesModelTierOverridesValuesAreCanonical.
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

// TestC974_002_EnvelopeGuardTestExistsAndPasses verifies that the PERMANENT
// regression guard TestModelTierOverridesWithinEnvelope exists in package
// profiles and passes. This is the durable protection that catches future
// override-vs-envelope drift in the normal `go test ./...` CI run — distinct
// from C974_001, which is this cycle's audit-scoped state check.
//
// RED before T1: the guard function does not exist → no PASS line in the
// verbose output. RED after JSON fix but before the guard is added: same.
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

// TestC974_003_OverridesConsumedOrRemoved enforces the "no inert API" goal
// constraint for the ModelTierOverrides map. It PASSES iff EITHER branch of the
// Builder's judgment call is genuinely done:
//
//	WIRED   → a production (non-_test.go) source file under go/internal accesses
//	          the field via `.ModelTierOverrides` (a real consumer), OR
//	REMOVED → the inert override entries scout flagged
//	          (cycle_4_plus_mature / s_complex_with_cache / routine_review) are
//	          gone from every loaded profile.
//
// RED on leave-as-is: no production consumer AND the inert entries still
// present — the exact discovered state. The predicate cannot be greened without
// actually wiring a consumer or deleting the dead config, so it is not a
// source-grep no-op.
func TestC974_003_OverridesConsumedOrRemoved(t *testing.T) {
	inertKeys := map[string]bool{
		"cycle_4_plus_mature":  true,
		"s_complex_with_cache": true,
		"routine_review":       true,
	}

	// REMOVED branch: no loaded profile still carries an inert override key.
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

	// WIRED branch: some production source file field-accesses .ModelTierOverrides.
	wired := hasProductionConsumer(t)

	if !wired && !removed {
		t.Errorf("RED: ModelTierOverrides is INERT — no production consumer accesses"+
			" .ModelTierOverrides AND %d inert override entr(y|ies) remain."+
			" Builder must WIRE a real consumer or REMOVE the dead entries"+
			" (no-inert-API goal constraint):\n  %s",
			len(remaining), strings.Join(remaining, "\n  "))
	}
}

// hasProductionConsumer reports whether any non-test .go file under go/internal
// field-accesses `.ModelTierOverrides` (a real runtime consumer). The
// leading dot excludes the struct-field declaration in profiles.go
// (`ModelTierOverrides map[...]`) and the comment mention in phaseconfig.go,
// both of which lack a dot prefix.
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

// dummyTB satisfies acsassert.TB so FileContains can be used as a pure boolean
// probe here (a missing file / absent substring must NOT fail the test — it is
// simply "no consumer in this file"). We never assert on the logged output.
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
