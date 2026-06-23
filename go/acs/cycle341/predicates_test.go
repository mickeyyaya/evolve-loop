//go:build acs

// Package cycle341 materializes the cycle-341 acceptance criteria for the two
// committed top_n tasks (driver-agnostic model-routing campaign, continuation):
//
//	T1  all-profiles-substitutability-parity-test — add TestAllProfilesSubstitutabilityAtParity
//	    to go/internal/profiles/profile_model_routing_amplification_test.go; uses real bridge
//	    manifests via loadSwappableManifests (not a synthetic Catalog{}); iterates all profiles
//	    via loader.List(); checks default tier AND overrides AND envelope; uses t.Errorf not t.Skip.
//
//	T2  policy-doc-substitutability-reference — update docs/architecture/model-routing-policy.md
//	    "Substitutability acceptance test" paragraph to cite TestAllProfilesSubstitutabilityAtParity
//	    and document allowed_clis dispatch exceptions for builder/tdd-engineer/tester.
//
// Predicates follow the cycle-85 lesson: BEHAVIORAL where possible; config-check waivers only
// where the deliverable IS the test/doc contract file.
//
// AC map (1:1 with triage-report.md top_n items):
//
//	T1.pass         TestAllProfilesSubstitutabilityAtParity passes           → C341_001 (BEHAVIORAL)
//	T1.real-manifests test uses loadSwappableManifests not synthetic Catalog → C341_002 (config-check)
//	T1.loader-list  test iterates via loader.List() not hardcoded list       → C341_003 (config-check)
//	T1.errorf       test uses t.Errorf not t.Skip for misses                 → C341_004 (config-check)
//	T1.regression   full go test ./internal/profiles/... passes (≥24 tests) → C341_005 (BEHAVIORAL)
//	T2.citation     policy doc cites TestAllProfilesSubstitutabilityAtParity → C341_006 (config-check)
//	T2.allowed-clis policy doc documents allowed_clis dispatch exceptions    → C341_007 (config-check)
//	T2.no-vendor    policy doc substitutability section has no bare vendor   → C341_008 (config-check, pre-existing GREEN)
//	                model names (haiku/sonnet/opus) as tier values
package cycle341

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// TestC341_001_AllProfilesSubstitutabilityAtParityTestPasses verifies that
// TestAllProfilesSubstitutabilityAtParity exists and passes in go/internal/profiles/.
// Behavioral: runs the go test subprocess and asserts the PASS line appears.
// RED: function does not exist yet → no "--- PASS: TestAllProfilesSubstitutabilityAtParity" line.
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

// TestC341_002_AllProfilesSubstitutabilityUsesRealManifests verifies that
// TestAllProfilesSubstitutabilityAtParity uses loadSwappableManifests (real bridge
// manifests) rather than a synthetic modelcatalog.Catalog{} fixture.
// acs-predicate: config-check — the deliverable IS the test contract file.
// RED: TestAllProfilesSubstitutabilityAtParity does not exist → CountInGoFunc errors.
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
			return // at least one real-manifest call found; criterion satisfied
		}
	}
	t.Errorf("RED: TestAllProfilesSubstitutabilityAtParity body references neither loadSwappableManifests " +
		"nor bridge.LoadManifest — test must use real bridge manifests, not a synthetic Catalog{} fixture")
}

// TestC341_003_AllProfilesSubstitutabilityIteratesViaLoaderList verifies that
// TestAllProfilesSubstitutabilityAtParity calls loader.List() to enumerate profiles
// rather than using a hardcoded slice of 7 spine phase names.
// acs-predicate: config-check — the deliverable IS the test contract file.
// RED: TestAllProfilesSubstitutabilityAtParity does not exist → CountInGoFunc errors.
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

// TestC341_004_AllProfilesSubstitutabilityUsesErrorfNotSkip verifies that
// TestAllProfilesSubstitutabilityAtParity uses t.Errorf for lookup failures (not t.Skip),
// so missing tiers are hard failures, not silent skips.
// acs-predicate: config-check — the criterion IS the function body contract.
// RED: TestAllProfilesSubstitutabilityAtParity does not exist → CountInGoFunc errors.
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

// TestC341_005_FullProfilesSuitePassesWithNewTest verifies that the full
// go test ./internal/profiles/... run includes TestAllProfilesSubstitutabilityAtParity
// in the PASS output (≥ 24 top-level PASS functions, up from the pre-T1 baseline of 23).
// Behavioral: runs the full profiles test suite subprocess.
// RED: function does not exist → only 23 top-level PASS lines, not ≥ 24.
// Note: subtests (TestFoo/SubtestName) are excluded from the count; only top-level
// test functions (--- PASS: TestFoo (Xs)) are counted, so the baseline is exactly 23.
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
	// Count top-level PASS lines (no subtest "/" in the name).
	// Matches "--- PASS: TestFoo (0.00s)" but NOT "--- PASS: TestFoo/SubtestName (0.00s)".
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

// TestC341_006_PolicyDocCitesAllProfilesSubstitutabilityTest verifies that
// docs/architecture/model-routing-policy.md cites TestAllProfilesSubstitutabilityAtParity
// as the definitive all-profiles parity guard.
// acs-predicate: config-check — the criterion IS the documentation contract.
// RED: model-routing-policy.md does not yet cite TestAllProfilesSubstitutabilityAtParity.
func TestC341_006_PolicyDocCitesAllProfilesSubstitutabilityTest(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	policyDoc := filepath.Join(root, "docs", "architecture", "model-routing-policy.md")
	if !acsassert.FileContains(t, policyDoc, "TestAllProfilesSubstitutabilityAtParity") {
		t.Errorf("RED: docs/architecture/model-routing-policy.md does not cite TestAllProfilesSubstitutabilityAtParity\n" +
			"Builder must update the 'Substitutability acceptance test' paragraph to reference the new all-profiles parity guard")
	}
}

// TestC341_007_PolicyDocDocumentsAllowedClisExceptions verifies that
// docs/architecture/model-routing-policy.md documents the intentional allowed_clis
// dispatch restrictions on builder/tdd-engineer/tester profiles.
// acs-predicate: config-check — the criterion IS the documentation contract.
// RED: model-routing-policy.md does not yet mention allowed_clis dispatch exceptions.
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

// TestC341_008_PolicyDocSubstitutabilitySectionHasNoVendorTierNames verifies that
// the "Substitutability acceptance test" section of model-routing-policy.md does not
// introduce bare vendor model names (haiku/sonnet/opus/gpt-4/gemini-pro) as tier values.
// acs-predicate: config-check — ensures T2's doc update doesn't reintroduce vendor names.
// NOTE: This predicate is PRE-EXISTING GREEN — the current section already has no vendor
// tier values. It is included as a regression guard to catch accidental vendor name
// reintroduction in the T2 doc update.
func TestC341_008_PolicyDocSubstitutabilitySectionHasNoVendorTierNames(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	policyDoc := filepath.Join(root, "docs", "architecture", "model-routing-policy.md")

	raw, err := os.ReadFile(policyDoc)
	if err != nil {
		t.Fatalf("cannot read model-routing-policy.md: %v", err)
	}

	// Extract the "Substitutability acceptance test" section (through the next h3/h2).
	text := string(raw)
	start := strings.Index(text, "### Substitutability acceptance test")
	if start == -1 {
		t.Fatalf("RED: 'Substitutability acceptance test' section not found in model-routing-policy.md\n" +
			"Builder must keep or add this section header when updating T2")
	}
	// Extract section until the next heading or end of file.
	section := text[start:]
	if nextH := strings.Index(section[4:], "\n##"); nextH != -1 {
		section = section[:nextH+4]
	}

	// Vendor names that must not appear as tier VALUE assignments in this section.
	// Note: "haiku-class", "sonnet-class", "opus-class" are capability descriptors (acceptable);
	// "model_tier_default: haiku" or `tier = "sonnet"` are the bad patterns.
	// We check for patterns like `"haiku"`, `"sonnet"`, `"opus"` as quoted tier values.
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
