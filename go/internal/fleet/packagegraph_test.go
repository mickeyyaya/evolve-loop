package fleet

import "testing"

const repoRoot = "../.."

const (
	unrelatedLeafFile    = "internal/flagregistry/registry.go"
	unrelatedLeafPackage = "github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
)

func TestTransitivePackageSet_ResolvesTransitiveImport(t *testing.T) {
	set, err := TransitivePackageSet([]string{"internal/fleet/partition.go"}, repoRoot)
	if err != nil {
		t.Fatalf("TransitivePackageSet: %v", err)
	}
	const wantPkg = "github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	if !set[wantPkg] {
		t.Errorf("fleet transitively imports ipcenv (see partition.go), but %q missing from set=%v", wantPkg, set)
	}
}

func TestTransitivePackageSet_NoCrossContaminationFromUnrelatedPackage(t *testing.T) {
	set, err := TransitivePackageSet([]string{"internal/fleet/partition.go"}, repoRoot)
	if err != nil {
		t.Fatalf("TransitivePackageSet: %v", err)
	}
	if set[unrelatedLeafPackage] {
		t.Errorf("fleet does not import %s (verified via go list -deps); set must not contain it", unrelatedLeafPackage)
	}
}

func TestTransitivePackageSet_EmptyFiles_ReturnsEmptySet(t *testing.T) {
	set, err := TransitivePackageSet(nil, repoRoot)
	if err != nil {
		t.Fatalf("TransitivePackageSet(nil): %v", err)
	}
	if len(set) != 0 {
		t.Errorf("no files -> no packages, got set=%v", set)
	}
}

func TestTransitivePackageSet_UnknownFile_ReturnsError(t *testing.T) {
	if _, err := TransitivePackageSet([]string{"internal/does/not/exist/nope.go"}, repoRoot); err == nil {
		t.Errorf("a file outside any real package must error, not silently resolve")
	}
}

func TestIsGlobalZone_MatchesGoModAndGoSum(t *testing.T) {
	for _, f := range []string{"go.mod", "./go.mod", "go.sum"} {
		if !IsGlobalZone(f) {
			t.Errorf("IsGlobalZone(%q) = false, want true (global-zone file)", f)
		}
	}
}

func TestIsGlobalZone_OrdinarySourceFile_NotGlobalZone(t *testing.T) {
	if IsGlobalZone("internal/fleet/partition.go") {
		t.Errorf("an ordinary source file must not be classified as global-zone")
	}
}

func TestGlobalZoneFiles_NonEmpty(t *testing.T) {
	if len(GlobalZoneFiles()) == 0 {
		t.Errorf("GlobalZoneFiles() must list at least go.mod/go.sum")
	}
}

func TestUnrelatedLeafFixture_SharesNoPackageWithFleet(t *testing.T) {
	leaf, err := TransitivePackageSet([]string{unrelatedLeafFile}, repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	fleet, err := TransitivePackageSet([]string{"internal/fleet/partition.go"}, repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	for pkg := range leaf {
		if fleet[pkg] {
			t.Errorf("fixture %s shares %s with fleet, so the partition tests that use it as an unrelated package cannot split it from fleet; point unrelatedLeafFile at a package with no import shared with fleet", unrelatedLeafFile, pkg)
		}
	}
}
