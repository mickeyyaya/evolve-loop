//go:build acs

// Package cycle340 materializes the cycle-340 acceptance criteria for the two
// committed top_n tasks (driver-agnostic model-routing campaign):
//
//	T1  substitutability-at-parity-acceptance-test — add TestSpineSubstitutabilityAtParity
//	    to go/internal/profiles/driver_agnostic_test.go; fixture covers codex/agy/ollama
//	    × fast/balanced/deep tiers; asserts non-empty Lookup() for each spine phase at
//	    its canonical tier; uses t.Errorf (not t.Skip) for lookup failures.
//
//	T2  fix-profiles-agents-md-vendor-tier-docs — update .evolve/profiles/AGENTS.md
//	    "Model selection" table row for model_tier_default to replace legacy vendor names
//	    (haiku/sonnet/opus) with canonical tiers (fast/balanced/deep); add prohibition note.
//
// Predicates are BEHAVIORAL where possible (cycle-85 lesson). C340_001 runs the
// real test subprocess. C340_002 and C340_003 are config-check waivers on the
// test contract file (the test file IS the deliverable). C340_004 and C340_005
// are config-check waivers on a documentation file.
//
// AC map (1:1 with triage-report.md top_n items):
//
//	T1.pass     TestSpineSubstitutabilityAtParity passes in profiles pkg     → C340_001
//	T1.drivers  fixture covers codex, agy, ollama drivers                    → C340_002
//	T1.errorf   function uses t.Errorf not t.Skip for lookup failures        → C340_003
//	T2.no-vendor no haiku/sonnet/opus in model_tier_default row              → C340_004
//	T2.canonical canonical tiers (fast/balanced/deep) appear in row          → C340_005
package cycle340

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC340_001_SpineSubstitutabilityAtParityTestPasses verifies that
// TestSpineSubstitutabilityAtParity exists and passes in go/internal/profiles/.
// Behavioral: runs the go test subprocess and asserts the PASS line appears.
// RED: function does not exist yet → no "--- PASS: TestSpineSubstitutabilityAtParity" line.
func TestC340_001_SpineSubstitutabilityAtParityTestPasses(t *testing.T) {
	dir := filepath.Join(acsassert.RepoRoot(t), "go")
	out, _, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", dir,
		"-count=1", "-v",
		"-run", "TestSpineSubstitutabilityAtParity",
		"./internal/profiles/...")
	if err != nil {
		t.Fatalf("go test subprocess error: %v", err)
	}
	passRe := regexp.MustCompile(`(?m)^--- PASS: TestSpineSubstitutabilityAtParity`)
	if !passRe.MatchString(out) {
		t.Errorf("RED: TestSpineSubstitutabilityAtParity not found as PASS (exit=%d)\n"+
			"Builder must add the function to go/internal/profiles/driver_agnostic_test.go\nOut:\n%s",
			code, tailLines(out, 30))
	}
}

// TestC340_002_SpineSubstitutabilityFixtureCoversAltDrivers verifies that the
// substitutability test fixture covers codex, agy, and ollama drivers inside
// the function body of TestSpineSubstitutabilityAtParity.
// acs-predicate: config-check — the deliverable IS the test contract file.
// RED: TestSpineSubstitutabilityAtParity does not exist → CountInGoFunc errors.
func TestC340_002_SpineSubstitutabilityFixtureCoversAltDrivers(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	testFile := filepath.Join(root, "go", "internal", "profiles", "driver_agnostic_test.go")
	for _, driver := range []string{"codex", "agy", "ollama"} {
		count, err := acsassert.CountInGoFunc(testFile, "TestSpineSubstitutabilityAtParity", driver)
		if err != nil {
			t.Fatalf("RED: TestSpineSubstitutabilityAtParity not found in driver_agnostic_test.go: %v", err)
		}
		if count == 0 {
			t.Errorf("RED: %q not referenced in TestSpineSubstitutabilityAtParity body — "+
				"fixture must cover codex/agy/ollama alt drivers", driver)
		}
	}
}

// TestC340_003_SpineSubstitutabilityUsesErrorfNotSkip verifies that
// TestSpineSubstitutabilityAtParity uses t.Errorf for lookup failures, not t.Skip.
// acs-predicate: config-check — the criterion IS the function body contract.
// RED: TestSpineSubstitutabilityAtParity does not yet exist → CountInGoFunc errors.
func TestC340_003_SpineSubstitutabilityUsesErrorfNotSkip(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	testFile := filepath.Join(root, "go", "internal", "profiles", "driver_agnostic_test.go")

	errorfCount, err := acsassert.CountInGoFunc(testFile, "TestSpineSubstitutabilityAtParity", "t.Errorf")
	if err != nil {
		t.Fatalf("RED: TestSpineSubstitutabilityAtParity not found in driver_agnostic_test.go: %v", err)
	}
	if errorfCount == 0 {
		t.Errorf("RED: TestSpineSubstitutabilityAtParity contains no t.Errorf calls — " +
			"lookup failures must trigger t.Errorf, not t.Skip")
	}

	skipCount, skipErr := acsassert.CountInGoFunc(testFile, "TestSpineSubstitutabilityAtParity", "t.Skip")
	if skipErr == nil && skipCount > 0 {
		t.Errorf("RED: TestSpineSubstitutabilityAtParity uses t.Skip — " +
			"missing lookup must be a hard failure (t.Errorf), not a skip")
	}
}

// TestC340_004_AgentsMdNoVendorNamesInModelTierDefaultRow verifies that the
// model_tier_default documentation row in .evolve/profiles/AGENTS.md no longer
// lists vendor model names (haiku/sonnet/opus).
// acs-predicate: config-check — the criterion IS a documentation row assertion.
// RED: AGENTS.md model_tier_default row currently says "haiku, sonnet, opus".
func TestC340_004_AgentsMdNoVendorNamesInModelTierDefaultRow(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	agentsMd := filepath.Join(root, ".evolve", "profiles", "AGENTS.md")

	raw, err := os.ReadFile(agentsMd)
	if err != nil {
		t.Fatalf("cannot read AGENTS.md: %v", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.Contains(line, "model_tier_default") {
			continue
		}
		lower := strings.ToLower(line)
		for _, vendor := range []string{"haiku", "sonnet", "opus"} {
			if strings.Contains(lower, vendor) {
				t.Errorf("RED: model_tier_default row in AGENTS.md contains vendor name %q\n"+
					"  row: %s\n"+
					"  Update to canonical tiers: fast/balanced/deep",
					vendor, strings.TrimSpace(line))
			}
		}
	}
}

// TestC340_005_AgentsMdCanonicalTiersInModelTierDefaultRow verifies that the
// model_tier_default row in AGENTS.md now documents canonical tiers (fast/balanced/deep).
// acs-predicate: config-check — the criterion IS a documentation row assertion.
// RED: current row uses only vendor names, not canonical tiers.
func TestC340_005_AgentsMdCanonicalTiersInModelTierDefaultRow(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	agentsMd := filepath.Join(root, ".evolve", "profiles", "AGENTS.md")

	raw, err := os.ReadFile(agentsMd)
	if err != nil {
		t.Fatalf("cannot read AGENTS.md: %v", err)
	}
	found := false
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.Contains(line, "model_tier_default") {
			continue
		}
		if strings.Contains(line, "fast") || strings.Contains(line, "balanced") || strings.Contains(line, "deep") {
			found = true
		}
	}
	if !found {
		t.Errorf("RED: model_tier_default row in AGENTS.md does not mention canonical tiers (fast/balanced/deep)\n" +
			"  Update to cite modelcatalog.CanonicalTiers vocabulary (fast/balanced/deep)")
	}
}

func tailLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
