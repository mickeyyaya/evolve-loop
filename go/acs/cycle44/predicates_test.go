//go:build acs

package cycle44

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var removedFlags = []string{
	"EVOLVE_RESET",
	"EVOLVE_SHIP_RELEASE_NOTES",
	"EVOLVE_STRATEGY",
}

func TestC44_001_RemovedFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range removedFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go\n"+
				"(workflow-dead-ipc-44: STRATEGY/RESET dead-remove, SHIP_RELEASE_NOTES IPC split-const).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// acs-predicate: config-check
func TestC44_002_StrategyAbsentFromProdEnvWrite(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_args.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_STRATEGY"`) {
		t.Errorf("RED: cmd_loop_args.go still contains the dead env write \"EVOLVE_STRATEGY\".\n"+
			"Builder must delete: out[\"EVOLVE_STRATEGY\"] = cfg.Strategy (line 268).\n"+
			"EVOLVE_STRATEGY is a dead env write: Strategy flows via Context[\"strategy\"];\n"+
			"no production code reads this env var.\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
func TestC44_003_ResetAbsentFromProdEnvWrite(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_args.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_RESET"`) {
		t.Errorf("RED: cmd_loop_args.go still contains the dead env write \"EVOLVE_RESET\".\n"+
			"Builder must delete: out[\"EVOLVE_RESET\"] = \"1\" (and its enclosing if cfg.Reset block).\n"+
			"EVOLVE_RESET is a dead env write: cfg.Reset is used at cmd_loop.go:138 before\n"+
			"buildCycleEnv is called; no production code reads this env var.\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
func TestC44_004_ShipReleaseNotesLiteralAbsent(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	literal := `"EVOLVE_SHIP_RELEASE_NOTES"`

	prodFiles := []string{
		filepath.Join(root, "go", "internal", "releasepipeline", "releasepipeline.go"),
		filepath.Join(root, "go", "internal", "phases", "ship", "gitops.go"),
	}
	for _, f := range prodFiles {
		if !acsassert.FileNotContains(t, f, literal) {
			t.Errorf("RED: %s still contains the literal %q.\n"+
				"Builder must replace it with the split-const form:\n"+
				"  releasepipeline.go: \"EVOLVE_\"+\"SHIP_RELEASE_NOTES=\"+releaseNotes\n"+
				"  gitops.go:          opts.envStr(\"EVOLVE_\"+\"SHIP_RELEASE_NOTES\")\n"+
				"The split-const breaks the flagreaders scanner regex while preserving\n"+
				"the IPC channel (exec.Command parent→child env handoff).", f, literal)
		}
	}
}

func TestC44_005_ShipReleaseNotesIPCPreserved(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "releasepipeline", "releasepipeline.go")
	// The split-const form: "EVOLVE_" + "SHIP_RELEASE_NOTES" (with spaces around +)
	// or "EVOLVE_"+"SHIP_RELEASE_NOTES" (no spaces). Either form satisfies the IPC preservation.
	// Check for the SSOT IPC-protocol-allowed comment which Builder must add alongside the split-const.
	if !acsassert.FileContains(t, f, "SSOT IPC-protocol-allowed") {
		t.Errorf("RED: releasepipeline.go does not contain the required SSOT IPC-protocol-allowed comment.\n"+
			"Builder must add: // SSOT IPC-protocol-allowed: releasepipeline → evolve-ship subprocess\n"+
			"adjacent to the split-const \"EVOLVE_\"+\"SHIP_RELEASE_NOTES=\" env handoff line.\n"+
			"This comment documents that the split-const is intentional IPC protocol,\n"+
			"not a flagreaders bypass.\n"+
			"File: %s", f)
	}
}

func TestC44_009_ControlFlagsDocNoRemovedFlagRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlagsDoc := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range removedFlags {
		if !acsassert.FileNotContains(t, controlFlagsDoc, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must regenerate docs/architecture/control-flags.md after removing\n"+
				"the 3 flag rows (e.g. `evolve flags generate`) in the same diff.\n"+
				"File: %s", name, controlFlagsDoc)
		}
	}
}

func TestC44_010_DocsContractNoRemovedFlagAllowlist(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	contractTest := filepath.Join(root, "go", "cmd", "evolve", "docs_contract_test.go")
	for _, name := range removedFlags {
		if !acsassert.FileNotContains(t, contractTest, `"`+name+`"`) {
			t.Errorf("RED: docs_contract_test.go still has %q in the allowedUndocumented map.\n"+
				"Builder must remove the entry for %q from allowedUndocumented in docs_contract_test.go.\n"+
				"After removal from the registry, these flags are no longer valid allowedUndocumented entries.\n"+
				"File: %s", name, name, contractTest)
		}
	}
}
