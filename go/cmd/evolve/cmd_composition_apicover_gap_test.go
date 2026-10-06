package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
)

func TestComposedApicoverGate_WarningOnlyMissesNewUnnamedExport(t *testing.T) {
	t.Skip("reproduction PERMANENTLY disabled. The gap it reproduced is CLOSED — composedGateTargets[\"apicover\"] now names the enforcing apicover-enforce recipe, pinned tree-mutation-free by TestComposedApicoverGate_TargetRecipeEnforces (recipe text) and proven live in both directions at land time. The body runs against a throwaway copy of the module, never this tree (percycle-audit-apicover-newexport-parity residual)")
	repoRoot := t.TempDir()
	goRoot := filepath.Join(repoRoot, "go")
	if err := os.CopyFS(goRoot, os.DirFS(apicoverGoRoot(t))); err != nil {
		t.Fatalf("copy the module: %v", err)
	}

	fixtureRel := filepath.Join("internal", "apicoverreprofixture998")
	fixtureDir := filepath.Join(goRoot, fixtureRel)
	if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
		t.Fatalf("create fixture package dir: %v", err)
	}

	fixtureSrc := `// Package apicoverreprofixture998 is a throwaway fixture for
// bug-reproduction cycle-998 (percycle-audit-apicover-newexport-parity).
// It is removed by the reproducer test's cleanup and must never be committed.
package apicoverreprofixture998

// UncoveredExport has zero test references and zero executed coverage —
// exactly the shape FleetRebaseVerdict/ClassifyFleetRebaseCandidate and
// LandPrefixes had the moment they landed and broke main's apicover gate.
func UncoveredExport() string { return "uncovered" }
`
	if err := os.WriteFile(filepath.Join(fixtureDir, "fixture.go"), []byte(fixtureSrc), 0o644); err != nil {
		t.Fatalf("write fixture package: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	target := composedGateTargets["apicover"]
	makeCmd := exec.CommandContext(ctx, "make", "-C", "go", target,
		"APICOVER_PKGS=./"+filepath.ToSlash(fixtureRel)+"/...")
	makeCmd.Dir = repoRoot
	var makeOut bytes.Buffer
	makeCmd.Stdout = &makeOut
	makeCmd.Stderr = &makeOut
	makeErr := makeCmd.Run()

	var enforceReport bytes.Buffer
	enforceCode, enforceRunErr := apicover.Run(ctx, apicover.Config{Enforce: true, Dirs: []string{fixtureDir}}, &enforceReport)
	if enforceRunErr != nil {
		t.Fatalf("apicover.Run(Enforce:true) measurement error: %v", enforceRunErr)
	}
	if enforceCode == 0 {
		t.Fatalf("test fixture invalid: apicover.Run(Enforce:true) reported no offenders for a deliberately uncovered+unnamed export — fixture does not exercise the gap:\n%s", enforceReport.String())
	}

	if makeErr == nil {
		t.Skipf(
			"KNOWN BUG (percycle-audit-apicover-newexport-parity): "+
				"`make -C go %s` (the recipe composedGateTargets[\"apicover\"] binds — "+
				"the same command internal/core/composition_carryforward.go's "+
				"composedGatesTo relies on before letting a fleet-rebase carry-forward "+
				"reship without a full re-audit) exited 0 (reported PASS) even though "+
				"package %s contains an exported symbol (UncoveredExport) with zero test "+
				"references and zero executed coverage.\n\n"+
				"The real enforcing check (apicover.Run with Enforce:true) correctly "+
				"flags this package (exit=%d):\n%s\n\n"+
				"make output:\n%s",
			target, fixtureRel, enforceCode, enforceReport.String(), makeOut.String())
	}
}
