//go:build acs

package cycle340

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

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

// acs-predicate: config-check — the deliverable IS the test contract file.
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

// acs-predicate: config-check — the criterion IS the function body contract.
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

// acs-predicate: config-check — the criterion IS a documentation row assertion.
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

// acs-predicate: config-check — the criterion IS a documentation row assertion.
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
