package stageevidence_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	probeSiteFile  = "internal/stageevidence/seal.go"
	probeSiteKind  = "Outcome literal fills Routed"
	probeSiteLine  = probeSiteFile + ": line 59: " + probeSiteKind
	probeTestName  = "TestSealOutcomeRoutedReachesConflictClass"
	otherSiteLine  = "internal/other/emit.go: line 7: assignment fills Outcome.Routed"
	otherSiteFile  = "internal/other/emit.go"
	otherSiteKind  = "assignment fills Outcome.Routed"
	otherTestName  = "TestOtherEmitReachesConflictClass"
	unregisteredIn = "unregistered"
	missingIn      = "missing"
)

func probeRegistry() map[routedSite]string {
	return map[routedSite]string{{File: probeSiteFile, Kind: probeSiteKind}: probeTestName}
}

func TestRoutedRegistry_UnregisteredEmitSiteFails(t *testing.T) {
	violations := routedRegistryViolations([]string{probeSiteLine, otherSiteLine}, probeRegistry(), map[string]bool{probeTestName: true})
	if len(violations) != 1 {
		t.Fatalf("violations = %v, want exactly the unregistered site", violations)
	}
	if !strings.Contains(violations[0], otherSiteFile) || !strings.Contains(violations[0], unregisteredIn) {
		t.Fatalf("violation %q must name %s as unregistered", violations[0], otherSiteFile)
	}
}

func TestRoutedRegistry_MissingScorerTestFails(t *testing.T) {
	registry := probeRegistry()
	registry[routedSite{File: otherSiteFile, Kind: otherSiteKind}] = otherTestName
	violations := routedRegistryViolations([]string{probeSiteLine, otherSiteLine}, registry, map[string]bool{probeTestName: true})
	if len(violations) != 1 {
		t.Fatalf("violations = %v, want exactly the registered name that does not exist", violations)
	}
	if !strings.Contains(violations[0], otherTestName) || !strings.Contains(violations[0], missingIn) {
		t.Fatalf("violation %q must name %s as missing", violations[0], otherTestName)
	}
}

func TestRoutedRegistry_RegisteredSiteWithExistingTestPasses(t *testing.T) {
	violations := routedRegistryViolations([]string{probeSiteLine}, probeRegistry(), map[string]bool{probeTestName: true})
	if len(violations) != 0 {
		t.Fatalf("a registered site with an existing scorer test must pass, got %v", violations)
	}
}

func TestRoutedRegistry_RegistrySiteAbsentFromTreeIsAccepted(t *testing.T) {
	registry := probeRegistry()
	registry[routedSite{File: otherSiteFile, Kind: otherSiteKind}] = otherTestName
	names := map[string]bool{probeTestName: true, otherTestName: true}
	if violations := routedRegistryViolations([]string{probeSiteLine}, registry, names); len(violations) != 0 {
		t.Fatalf("a registered site not yet in the tree must be accepted, got %v", violations)
	}
}

func TestRoutedRegistry_SameFileOtherKindIsUnregistered(t *testing.T) {
	finding := probeSiteFile + ": line 80: assignment fills held-proposal item"
	violations := routedRegistryViolations([]string{finding}, probeRegistry(), map[string]bool{probeTestName: true})
	if len(violations) != 1 || !strings.Contains(violations[0], unregisteredIn) {
		t.Fatalf("a different hit kind in a registered file must be unregistered, got %v", violations)
	}
}

func TestRoutedRegistry_ScorerTestNamesReadsTestFunctions(t *testing.T) {
	dir := t.TempDir()
	src := "package probe\n\nimport \"testing\"\n\nfunc TestAlpha(t *testing.T) {}\n\nfunc helperBeta() {}\n\nfunc BenchmarkGamma(b *testing.B) {}\n"
	if err := os.WriteFile(filepath.Join(dir, "alpha_test.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	names, err := scorerTestNames(dir)
	if err != nil {
		t.Fatalf("scorerTestNames: %v", err)
	}
	if !names["TestAlpha"] || names["helperBeta"] || names["BenchmarkGamma"] {
		t.Fatalf("names = %v, want only TestAlpha", names)
	}
}

func TestRoutedRegistry_RealRegistryNamesExistingScorerTests(t *testing.T) {
	names, err := scorerTestNames(".")
	if err != nil {
		t.Fatalf("scorerTestNames: %v", err)
	}
	if !names[probeTestName] {
		t.Fatalf("the seal scorer test %s is not a test function of the package", probeTestName)
	}
	registered := false
	for site, name := range routedEmitRegistry {
		if site.File == probeSiteFile {
			registered = true
			if !names[name] {
				t.Errorf("registry maps %s to %s, which is not a test function", site.File, name)
			}
		}
	}
	if !registered {
		t.Fatalf("routedEmitRegistry has no entry for %s", probeSiteFile)
	}
}
