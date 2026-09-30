//go:build acs

package cycle659

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func repoFile(t *testing.T, rel string) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), rel)
}

func TestC659_001_StatemapPackageTestsPassUnderRace(t *testing.T) {
	dir := goDir(t)
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-race", "-count=1",
		"./internal/adapters/statemap/...",
	)
	if code != 0 || err != nil {
		t.Fatalf("RED: `go test -race ./internal/adapters/statemap/...` failed (exit=%d): %v\n"+
			"Builder must create the leaf package internal/adapters/statemap with\n"+
			"  func ReadStateMap(path string) (map[string]any, error)\n"+
			"  func UpdateStateMap(path string, mutate func(map[string]any)) error\n"+
			"holding flock.PathLock(path) across the whole read-modify-write.\nOutput:\n%s",
			code, err, out+"\n"+errOut)
	}
}

func TestC659_002_ResetDuplicateRMWRemoved(t *testing.T) {
	dir := goDir(t)
	if _, errOut, code, err := acsassert.SubprocessOutput(
		"go", "build", "-C", dir, "./internal/adapters/statemap/...",
	); code != 0 || err != nil {
		t.Fatalf("RED: consolidated package does not build (exit=%d): %v\n%s", code, err, errOut)
	}

	resetGo := repoFile(t, "go/internal/core/reset.go")
	for _, dup := range []string{"func readJSONMapFile(", "func writeJSONMapFileAtomic("} {
		if acsassert.FileContainsAny(resetGo, dup) {
			t.Errorf("RED: reset.go still defines the duplicate RMW helper %q — it must be deleted "+
				"and SealCycle routed through statemap.UpdateStateMap(", dup)
		}
	}
	if !acsassert.FileContains(t, resetGo, "statemap.") {
		t.Errorf("RED: reset.go does not reference statemap.* — SealCycle must route its state.json " +
			"read-modify-write through statemap.UpdateStateMap(, not an inline copy")
	}
}

func TestC659_003_AllWritersRouteThroughStatemap(t *testing.T) {
	dir := goDir(t)
	if _, errOut, code, err := acsassert.SubprocessOutput(
		"go", "build", "-C", dir,
		"./internal/core/...", "./internal/phaseintegrity/...", "./internal/phases/ship/...",
	); code != 0 || err != nil {
		t.Fatalf("RED: caller packages do not build with the statemap dependency (exit=%d): %v\n%s",
			code, err, errOut)
	}

	callers := map[string]string{
		"go/internal/core/reset.go":            "SealCycle",
		"go/internal/phaseintegrity/repin.go":  "RepinShipSHA",
		"go/internal/phases/ship/statefile.go": "ship",
	}
	for rel, who := range callers {
		f := repoFile(t, rel)
		if !acsassert.FileContains(t, f, "statemap.") {
			t.Errorf("RED: %s (%s path) does not route through statemap.* — the single RMW source", rel, who)
		}
		if rel == "go/internal/core/reset.go" && acsassert.FileContainsAny(f, "storage.UpdateStateMap(") {
			t.Errorf("RED: reset.go references storage.UpdateStateMap( — forbidden core→storage import " +
				"cycle (cycle-644). The pin target MUST be statemap.UpdateStateMap(")
		}
	}
}

func TestC659_004_TouchedPackagesVetClean(t *testing.T) {
	dir := goDir(t)
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "vet", "-C", dir,
		"./internal/adapters/statemap/...", "./internal/core/...",
		"./internal/phaseintegrity/...", "./internal/phases/ship/...",
	)
	if code != 0 || err != nil {
		t.Fatalf("RED: `go vet` on the consolidated packages failed (exit=%d): %v\n%s",
			code, err, out+"\n"+errOut)
	}
}

// acs-predicate: config-check
func TestC659_005_StatemapGraduatedIntoApicoverEnforce(t *testing.T) {
	enforce := repoFile(t, "go/.apicover-enforce")
	if !acsassert.FileContainsAny(enforce,
		"./internal/adapters/statemap",
		"internal/adapters/statemap",
	) {
		t.Errorf("RED: go/.apicover-enforce does not list internal/adapters/statemap — the new leaf " +
			"package must be graduated into the enforced set (new-package obligation, 3rd-recurrence class)")
	}
	if !acsassert.FileContains(t, enforce, "./internal/adapters/flock") {
		t.Errorf("apicover-enforce sanity: expected the file to still list ./internal/adapters/flock")
	}
}
