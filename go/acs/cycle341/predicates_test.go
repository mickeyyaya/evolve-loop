//go:build acs

package cycle341

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC341_001_AllProfilesSubstitutabilityAtParityTestPasses(t *testing.T) {
	dir := filepath.Join(acsassert.RepoRoot(t), "go")
	out, _, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", dir,
		"-count=1", "-v",
		"-run", "TestAllProfilesSubstitutabilityAtParity",
		"./internal/profiles/...")
	if err != nil {
		t.Fatalf("go test subprocess error: %v", err)
	}
	passRe := regexp.MustCompile(`(?m)^--- PASS: TestAllProfilesSubstitutabilityAtParity`)
	if !passRe.MatchString(out) {
		t.Errorf("RED: TestAllProfilesSubstitutabilityAtParity not found as PASS (exit=%d)\n"+
			"Builder must add the function to go/internal/profiles/profile_model_routing_amplification_test.go\nOut:\n%s",
			code, tailLines(out, 30))
	}
}

// acs-predicate: config-check — the deliverable IS the test contract file.
func TestC341_002_AllProfilesSubstitutabilityUsesRealManifests(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	testFile := filepath.Join(root, "go", "internal", "profiles", "profile_model_routing_amplification_test.go")
	for _, call := range []string{"loadSwappableManifests", "bridge.LoadManifest"} {
		count, err := acsassert.CountInGoFunc(testFile, "TestAllProfilesSubstitutabilityAtParity", call)
		if err != nil {
			t.Fatalf("RED: TestAllProfilesSubstitutabilityAtParity not found in profile_model_routing_amplification_test.go: %v", err)
		}
		if count > 0 {
			return
		}
	}
	t.Errorf("RED: TestAllProfilesSubstitutabilityAtParity body references neither loadSwappableManifests " +
		"nor bridge.LoadManifest — test must use real bridge manifests, not a synthetic Catalog{} fixture")
}

// acs-predicate: config-check — the deliverable IS the test contract file.
func TestC341_003_AllProfilesSubstitutabilityIteratesViaLoaderList(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	testFile := filepath.Join(root, "go", "internal", "profiles", "profile_model_routing_amplification_test.go")
	count, err := acsassert.CountInGoFunc(testFile, "TestAllProfilesSubstitutabilityAtParity", ".List()")
	if err != nil {
		t.Fatalf("RED: TestAllProfilesSubstitutabilityAtParity not found in profile_model_routing_amplification_test.go: %v", err)
	}
	if count == 0 {
		t.Errorf("RED: TestAllProfilesSubstitutabilityAtParity body does not call loader.List() — " +
			"must enumerate ALL profiles dynamically, not a hardcoded list of 7 spine phases")
	}
}

// acs-predicate: config-check — the criterion IS the function body contract.
func TestC341_004_AllProfilesSubstitutabilityUsesErrorfNotSkip(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	testFile := filepath.Join(root, "go", "internal", "profiles", "profile_model_routing_amplification_test.go")

	errorfCount, err := acsassert.CountInGoFunc(testFile, "TestAllProfilesSubstitutabilityAtParity", "t.Errorf")
	if err != nil {
		t.Fatalf("RED: TestAllProfilesSubstitutabilityAtParity not found in profile_model_routing_amplification_test.go: %v", err)
	}
	if errorfCount == 0 {
		t.Errorf("RED: TestAllProfilesSubstitutabilityAtParity contains no t.Errorf calls — " +
			"tier lookup failures must trigger t.Errorf, not t.Skip")
	}

	skipCount, skipErr := acsassert.CountInGoFunc(testFile, "TestAllProfilesSubstitutabilityAtParity", "t.Skip")
	if skipErr == nil && skipCount > 0 {
		t.Errorf("RED: TestAllProfilesSubstitutabilityAtParity uses t.Skip — " +
			"tier lookup misses must be hard failures (t.Errorf), not skips")
	}
}

func TestC341_005_FullProfilesSuitePassesWithNewTest(t *testing.T) {
	dir := filepath.Join(acsassert.RepoRoot(t), "go")
	out, _, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", dir,
		"-count=1", "-v",
		"./internal/profiles/...")
	if err != nil {
		t.Fatalf("go test subprocess error: %v", err)
	}
	if code != 0 {
		t.Fatalf("RED: go test ./internal/profiles/... exited %d — regression in existing tests\nOut:\n%s",
			code, tailLines(out, 40))
	}
	topLevelPassRe := regexp.MustCompile(`^--- PASS: [^/]+ \(`)
	topLevelPassCount := 0
	for _, line := range strings.Split(out, "\n") {
		if topLevelPassRe.MatchString(strings.TrimSpace(line)) {
			topLevelPassCount++
		}
	}
	if topLevelPassCount < 24 {
		t.Errorf("RED: go test ./internal/profiles/... produced only %d top-level PASS functions; "+
			"want ≥ 24 (baseline 23 + TestAllProfilesSubstitutabilityAtParity)\n"+
			"Builder must add TestAllProfilesSubstitutabilityAtParity to the amplification test file",
			topLevelPassCount)
	}
}

// acs-predicate: config-check — the criterion IS the documentation contract.
func TestC341_006_PolicyDocCitesAllProfilesSubstitutabilityTest(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	policyDoc := filepath.Join(root, "docs", "architecture", "model-routing-policy.md")
	if !acsassert.FileContains(t, policyDoc, "TestAllProfilesSubstitutabilityAtParity") {
		t.Errorf("RED: docs/architecture/model-routing-policy.md does not cite TestAllProfilesSubstitutabilityAtParity\n" +
			"Builder must update the 'Substitutability acceptance test' paragraph to reference the new all-profiles parity guard")
	}
}

// acs-predicate: config-check — the criterion IS the documentation contract.
func TestC341_007_PolicyDocDocumentsAllowedClisExceptions(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	policyDoc := filepath.Join(root, "docs", "architecture", "model-routing-policy.md")
	if !acsassert.FileContains(t, policyDoc, "allowed_clis") {
		t.Errorf("RED: docs/architecture/model-routing-policy.md does not document allowed_clis dispatch exceptions\n" +
			"Builder must add a note explaining that builder/tdd-engineer/tester have intentional\n" +
			"allowed_clis restrictions (cross-family floor, TDD ≠ builder invariant) — tier vocabulary\n" +
			"is driver-agnostic but dispatch eligibility is constrained by design")
	}
}

// acs-predicate: config-check — ensures T2's doc update doesn't reintroduce vendor names.
func TestC341_008_PolicyDocSubstitutabilitySectionHasNoVendorTierNames(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	policyDoc := filepath.Join(root, "docs", "architecture", "model-routing-policy.md")

	raw, err := os.ReadFile(policyDoc)
	if err != nil {
		t.Fatalf("cannot read model-routing-policy.md: %v", err)
	}

	text := string(raw)
	start := strings.Index(text, "### Substitutability acceptance test")
	if start == -1 {
		t.Fatalf("RED: 'Substitutability acceptance test' section not found in model-routing-policy.md\n" +
			"Builder must keep or add this section header when updating T2")
	}
	section := text[start:]
	if nextH := strings.Index(section[4:], "\n##"); nextH != -1 {
		section = section[:nextH+4]
	}

	vendorPatterns := []struct {
		name string
		re   *regexp.Regexp
	}{
		{"haiku as quoted tier value", regexp.MustCompile(`["` + "`" + `]haiku["` + "`" + `]`)},
		{"sonnet as quoted tier value", regexp.MustCompile(`["` + "`" + `]sonnet["` + "`" + `]`)},
		{"opus as quoted tier value", regexp.MustCompile(`["` + "`" + `]opus["` + "`" + `]`)},
	}
	for _, vp := range vendorPatterns {
		if vp.re.MatchString(section) {
			t.Errorf("RED: substitutability section of model-routing-policy.md contains %s — "+
				"tier vocabulary must use canonical tiers (fast/balanced/deep), not vendor model names",
				vp.name)
		}
	}
}

func tailLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
