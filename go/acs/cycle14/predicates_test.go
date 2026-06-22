//go:build acs

// Package cycle14 materializes the cycle-14 acceptance criteria for two tasks:
//
//   - ipcenv-leaf: Create IPC-const leaf package (internal/ipcenv) with 3 exported
//     consts: FleetKey, FleetScopeKey, WorktreeRootKey — each marked SSOT IPC-protocol-allowed.
//     Zero imports from this codebase (pure const leaf). Enrolled in .apicover-enforce.
//
//   - ipcenv-wire: Route all readers through ipcenv + remove 3 registry rows.
//     No bare "EVOLVE_FLEET", "EVOLVE_FLEET_SCOPE", or "EVOLVE_WORKTREE_ROOT" string
//     literals in production Go files (outside ipcenv/). fleet.go local consts removed.
//     flagregistry rows for the 3 flags deleted. Build clean.
//
// AC map (1:1 with scout-report.md ACs, all in top_n):
//
//	ipcenv-leaf:
//	  AC1  3 exported consts exist with correct values + SSOT comment → C14_001
//	  AC2  Package compiles (go build ./internal/ipcenv/...)           → C14_002
//	  AC3  Enrolled in .apicover-enforce                               → C14_003
//	  AC4  No imports from this codebase (leaf purity)                 → C14_004
//
//	ipcenv-wire:
//	  AC5  No bare EVOLVE_FLEET literal in prod files                  → C14_005
//	  AC5- SSOT in ipcenv.go DOES contain literal (grep-exclusion check) → C14_005neg
//	  AC6  No bare EVOLVE_FLEET_SCOPE literal in prod files            → C14_006
//	  AC7  No bare EVOLVE_WORKTREE_ROOT literal in prod files          → C14_007
//	  AC8  3 registry rows deleted                                     → C14_008
//	  AC8- An unrelated flag still in registry (surgical-delete check) → C14_008neg
//	  AC9  Build is clean after wiring (go build ./...)                → C14_009
//	  AC10 fleet.go local consts removed                               → C14_010
//
// Floor binding (R9.3): predicates only for tasks in triage top_n.
package cycle14

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goBashCmd runs a bash command string from the go/ directory and returns
// combined stdout+stderr and exit code.
func goBashCmd(t *testing.T, cmd string) (combined string, code int) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	bashCmd := "cd '" + goDir + "' && " + cmd
	out, errOut, c, _ := acsassert.SubprocessOutput("bash", "-c", bashCmd)
	return strings.TrimSpace(out + "\n" + errOut), c
}

// ── ipcenv-leaf ─────────────────────────────────────────────────────────────

// TestC14_001_IPCEnvLeafConstValues verifies that go/internal/ipcenv/ipcenv.go
// defines the three exported consts with the correct string values and the
// required "SSOT IPC-protocol-allowed" annotation.
//
// BEHAVIORAL: reads the actual file to verify const declarations with exact
// values — a stub with the key names but wrong string values still fails.
//
// RED: file doesn't exist yet → FileContains fails on read error.
// GREEN: Builder creates ipcenv.go with the three const declarations.
//
// acs-predicate: config-check — const values ARE the IPC protocol constants
// (their string value is the observable behavior).
func TestC14_001_IPCEnvLeafConstValues(t *testing.T) {
	root := acsassert.RepoRoot(t)
	abs := filepath.Join(root, "go", "internal", "ipcenv", "ipcenv.go")

	if !acsassert.FileContains(t, abs, `FleetKey = "EVOLVE_FLEET"`) {
		t.Errorf("RED: ipcenv.go missing FleetKey const with correct value.\n" +
			"Builder must create go/internal/ipcenv/ipcenv.go with:\n" +
			"  const FleetKey = \"EVOLVE_FLEET\" // SSOT IPC-protocol-allowed\n" +
			"File: go/internal/ipcenv/ipcenv.go")
	}
	if !acsassert.FileContains(t, abs, `FleetScopeKey = "EVOLVE_FLEET_SCOPE"`) {
		t.Errorf("RED: ipcenv.go missing FleetScopeKey const with correct value.\n" +
			"Builder must add:\n" +
			"  const FleetScopeKey = \"EVOLVE_FLEET_SCOPE\" // SSOT IPC-protocol-allowed\n" +
			"File: go/internal/ipcenv/ipcenv.go")
	}
	if !acsassert.FileContains(t, abs, `WorktreeRootKey = "EVOLVE_WORKTREE_ROOT"`) {
		t.Errorf("RED: ipcenv.go missing WorktreeRootKey const with correct value.\n" +
			"Builder must add:\n" +
			"  const WorktreeRootKey = \"EVOLVE_WORKTREE_ROOT\" // SSOT IPC-protocol-allowed\n" +
			"File: go/internal/ipcenv/ipcenv.go")
	}
	if !acsassert.FileContains(t, abs, "SSOT IPC-protocol-allowed") {
		t.Errorf("RED: ipcenv.go missing SSOT IPC-protocol-allowed annotation.\n" +
			"Builder must annotate each const with: // SSOT IPC-protocol-allowed\n" +
			"File: go/internal/ipcenv/ipcenv.go")
	}
}

// TestC14_002_IPCEnvLeafCompiles verifies that go/internal/ipcenv compiles
// cleanly (go build ./internal/ipcenv/...).
//
// BEHAVIORAL: invokes the Go compiler. A text stub without valid Go syntax
// or a missing package both cause a non-zero exit.
//
// RED: package doesn't exist → go build exits non-zero ("no such directory").
// GREEN: Builder creates a valid Go package → go build exits 0.
func TestC14_002_IPCEnvLeafCompiles(t *testing.T) {
	combined, code := goBashCmd(t, "go build ./internal/ipcenv/...")
	if code != 0 {
		t.Errorf("RED: go build ./internal/ipcenv/... failed (exit %d).\n"+
			"Builder must create go/internal/ipcenv/ipcenv.go with valid Go syntax.\n"+
			"Output:\n%s", code, combined)
	}
}

// TestC14_003_IPCEnvLeafEnrolledInApicover verifies that go/.apicover-enforce
// contains the entry ./internal/ipcenv, enrolling the new package in the
// apicover completeness gate.
//
// acs-predicate: config-check — enrollment is a config file entry.
//
// RED: ./internal/ipcenv absent from .apicover-enforce.
// GREEN: Builder adds the line.
func TestC14_003_IPCEnvLeafEnrolledInApicover(t *testing.T) {
	root := acsassert.RepoRoot(t)
	abs := filepath.Join(root, "go", ".apicover-enforce")
	if !acsassert.FileContains(t, abs, "./internal/ipcenv") {
		t.Errorf("RED: go/.apicover-enforce does not contain ./internal/ipcenv.\n" +
			"Builder must add the line ./internal/ipcenv to go/.apicover-enforce\n" +
			"so the acs docgo completeness gate picks up the new package.\n" +
			"File: go/.apicover-enforce")
	}
}

// TestC14_004_IPCEnvLeafHasNoInternalImports verifies that internal/ipcenv
// imports nothing from this codebase (pure const leaf — zero intra-repo imports).
//
// BEHAVIORAL: parses the package's import list via `go list -json` and
// checks that no import starts with our module prefix.
//
// RED: package doesn't exist → go list fails (non-zero exit → Fatalf).
// GREEN: ipcenv.go has no intra-repo imports.
func TestC14_004_IPCEnvLeafHasNoInternalImports(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	out, errOut, code, _ := acsassert.SubprocessOutput("bash", "-c",
		"cd '"+goDir+"' && go list -json ./internal/ipcenv")
	combined := strings.TrimSpace(out + "\n" + errOut)
	if code != 0 {
		t.Fatalf("RED: go list -json ./internal/ipcenv failed — package may not exist yet.\n"+
			"Builder must create go/internal/ipcenv/ipcenv.go.\nOutput:\n%s", combined)
	}
	var info struct {
		Imports []string `json:"Imports"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &info); err != nil {
		t.Fatalf("RED: failed to parse go list -json output: %v\nRaw stdout:\n%s", err, out)
	}
	const modulePrefix = "github.com/mickeyyaya/evolve-loop/go"
	for _, imp := range info.Imports {
		if strings.HasPrefix(imp, modulePrefix) {
			t.Errorf("RED: ipcenv imports intra-repo package %q.\n"+
				"ipcenv must be a pure const leaf — remove all imports from this codebase.\n"+
				"File: go/internal/ipcenv/ipcenv.go", imp)
		}
	}
}

// ── ipcenv-wire ─────────────────────────────────────────────────────────────

// TestC14_005_NoBareLiteralFleetInProdFiles verifies that no production Go
// file (outside ipcenv/) contains the bare string literal "EVOLVE_FLEET".
//
// BEHAVIORAL: greps the go/ tree excluding _test.go files and ipcenv/ (the SSOT).
// Currently RED because multiple prod files use bare literals.
//
// RED: grep emits matches (literals present in bridge/core/ship/fleet/registry files).
// GREEN: all readers route through ipcenv.FleetKey; grep emits nothing.
func TestC14_005_NoBareLiteralFleetInProdFiles(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goPath := filepath.Join(root, "go")
	bashCmd := "grep -r '\"EVOLVE_FLEET\"' '" + goPath + "' --include='*.go'" +
		" | grep -v '_test\\.go' | grep -v '/ipcenv/' || true"
	out, _, _, _ := acsassert.SubprocessOutput("bash", "-c", bashCmd)
	out = strings.TrimSpace(out)
	if out != "" {
		t.Errorf("RED: bare \"EVOLVE_FLEET\" literal still present in production files.\n"+
			"Builder must replace each occurrence with ipcenv.FleetKey.\n"+
			"Remaining occurrences:\n%s", out)
	}
}

// TestC14_005neg_IPCEnvSSotContainsFleetLiteral is the adversarial negative:
// verifies that the ipcenv SSOT file DOES contain the "EVOLVE_FLEET" literal,
// confirming the exclusion in TestC14_005 is correct and the SSOT exists.
//
// RED: ipcenv.go doesn't exist yet → FileContains fails on read.
// GREEN: ipcenv.go defines FleetKey = "EVOLVE_FLEET".
func TestC14_005neg_IPCEnvSSotContainsFleetLiteral(t *testing.T) {
	root := acsassert.RepoRoot(t)
	abs := filepath.Join(root, "go", "internal", "ipcenv", "ipcenv.go")
	if !acsassert.FileContains(t, abs, `"EVOLVE_FLEET"`) {
		t.Errorf("RED: ipcenv.go does not contain the \"EVOLVE_FLEET\" literal — SSOT definition missing.\n" +
			"Builder must create go/internal/ipcenv/ipcenv.go with:\n" +
			"  const FleetKey = \"EVOLVE_FLEET\" // SSOT IPC-protocol-allowed")
	}
}

// TestC14_006_NoBareLiteralFleetScopeInProdFiles verifies that no production
// Go file (outside ipcenv/) contains the bare string literal "EVOLVE_FLEET_SCOPE".
//
// BEHAVIORAL: grep over go/ excluding test files and ipcenv/.
//
// RED: grep finds bare literals (currently in core/cyclerun.go and fleet/fleet.go).
// GREEN: all readers route through ipcenv.FleetScopeKey.
func TestC14_006_NoBareLiteralFleetScopeInProdFiles(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goPath := filepath.Join(root, "go")
	bashCmd := "grep -r '\"EVOLVE_FLEET_SCOPE\"' '" + goPath + "' --include='*.go'" +
		" | grep -v '_test\\.go' | grep -v '/ipcenv/' || true"
	out, _, _, _ := acsassert.SubprocessOutput("bash", "-c", bashCmd)
	out = strings.TrimSpace(out)
	if out != "" {
		t.Errorf("RED: bare \"EVOLVE_FLEET_SCOPE\" literal still present in production files.\n"+
			"Builder must replace each occurrence with ipcenv.FleetScopeKey.\n"+
			"Remaining occurrences:\n%s", out)
	}
}

// TestC14_007_NoBareLiteralWorktreeRootInProdFiles verifies that no production
// Go file (outside ipcenv/) contains the bare string literal "EVOLVE_WORKTREE_ROOT".
//
// BEHAVIORAL: grep over go/ excluding test files and ipcenv/.
//
// RED: grep finds bare literals (currently in cmd_subagent.go, acssuite.go, ship_recovery.go).
// GREEN: all readers route through ipcenv.WorktreeRootKey.
func TestC14_007_NoBareLiteralWorktreeRootInProdFiles(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goPath := filepath.Join(root, "go")
	bashCmd := "grep -r '\"EVOLVE_WORKTREE_ROOT\"' '" + goPath + "' --include='*.go'" +
		" | grep -v '_test\\.go' | grep -v '/ipcenv/' || true"
	out, _, _, _ := acsassert.SubprocessOutput("bash", "-c", bashCmd)
	out = strings.TrimSpace(out)
	if out != "" {
		t.Errorf("RED: bare \"EVOLVE_WORKTREE_ROOT\" literal still present in production files.\n"+
			"Builder must replace each occurrence with ipcenv.WorktreeRootKey.\n"+
			"Remaining occurrences:\n%s", out)
	}
}

// TestC14_008_ThreeRegistryRowsDeleted verifies that flagregistry no longer
// contains rows for EVOLVE_FLEET, EVOLVE_FLEET_SCOPE, or EVOLVE_WORKTREE_ROOT.
//
// BEHAVIORAL: calls flagregistry.Lookup() — exercises the actual registry,
// not a text search. A StatusDeprecated row would still be found and fail.
// (These flags must exit the registry entirely, not be demoted.)
//
// RED: Lookup returns true for any of the 3 flags (all three are StatusActive now).
// GREEN: Builder deletes the 3 rows → Lookup returns false for all three.
func TestC14_008_ThreeRegistryRowsDeleted(t *testing.T) {
	for _, name := range []string{"EVOLVE_FLEET", "EVOLVE_FLEET_SCOPE", "EVOLVE_WORKTREE_ROOT"} {
		if _, found := flagregistry.Lookup(name); found {
			t.Errorf("RED: flagregistry still contains row for %q.\n"+
				"Builder must delete this row from go/internal/flagregistry/registry_table.go.\n"+
				"These flags are reclassified as IPC protocol constants (internal/ipcenv);\n"+
				"they are not operator dials and require no registry row.", name)
		}
	}
}

// TestC14_008neg_UnrelatedFlagStillInRegistry is the adversarial negative:
// verifies that an unrelated flag survives the deletions, confirming the
// flag-row removal was surgical (not "truncate the whole table").
//
// BEHAVIORAL: Lookup for EVOLVE_SANDBOX which must remain StatusActive.
// PRE-EXISTING GREEN: Lookup("EVOLVE_SANDBOX") already returns true.
func TestC14_008neg_UnrelatedFlagStillInRegistry(t *testing.T) {
	if _, found := flagregistry.Lookup("EVOLVE_SANDBOX"); !found {
		t.Errorf("FAIL: EVOLVE_SANDBOX unexpectedly absent from flagregistry.\n" +
			"The registry deletions must be surgical: only EVOLVE_FLEET, EVOLVE_FLEET_SCOPE,\n" +
			"and EVOLVE_WORKTREE_ROOT should be removed.\n" +
			"File: go/internal/flagregistry/registry_table.go")
	}
}

// TestC14_009_BuildIsClean verifies that the go/ module compiles cleanly
// (go build ./...) after all ipcenv wiring changes. This is the build-level
// proxy for "all tests pass" — the toolchain-green ship gate enforces actual
// test passage separately.
//
// BEHAVIORAL: invokes the Go compiler on the entire module.
//
// RED: ipcenv package missing or wiring introduces compile errors.
// GREEN: all packages compile after leaf creation + const substitution.
func TestC14_009_BuildIsClean(t *testing.T) {
	combined, code := goBashCmd(t, "go build ./...")
	if code != 0 {
		t.Errorf("RED: go build ./... failed (exit %d).\n"+
			"Builder must ensure all packages compile after ipcenv wiring.\n"+
			"Common causes: missing import of ipcenv, undefined symbol, deleted const still referenced.\n"+
			"Output:\n%s", code, combined)
	}
}

// TestC14_010_FleetGoLocalConstsRemoved verifies that fleet/fleet.go no longer
// defines the unexported local consts fleetEnvKey and fleetScopeEnvKey.
//
// BEHAVIORAL (mixed): the const ABSENCE check ensures the removal happened;
// TestC14_009's build check ensures fleet still compiles (if consts were removed
// without adding the ipcenv import, go build ./... would fail).
//
// RED: fleet.go still contains the const declarations.
// GREEN: Builder removes them and routes through ipcenv.FleetKey/FleetScopeKey.
func TestC14_010_FleetGoLocalConstsRemoved(t *testing.T) {
	root := acsassert.RepoRoot(t)
	fleetGo := filepath.Join(root, "go", "internal", "fleet", "fleet.go")

	if !acsassert.FileNotContains(t, fleetGo, "const fleetEnvKey") {
		t.Errorf("RED: fleet.go still defines unexported const fleetEnvKey.\n" +
			"Builder must remove 'const fleetEnvKey = \"EVOLVE_FLEET\"'\n" +
			"and replace usages with ipcenv.FleetKey.\n" +
			"File: go/internal/fleet/fleet.go")
	}
	if !acsassert.FileNotContains(t, fleetGo, "const fleetScopeEnvKey") {
		t.Errorf("RED: fleet.go still defines unexported const fleetScopeEnvKey.\n" +
			"Builder must remove 'const fleetScopeEnvKey = \"EVOLVE_FLEET_SCOPE\"'\n" +
			"and replace usages with ipcenv.FleetScopeKey.\n" +
			"File: go/internal/fleet/fleet.go")
	}
}
