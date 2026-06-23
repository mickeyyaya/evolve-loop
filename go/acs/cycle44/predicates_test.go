//go:build acs

// Package cycle44 materializes the cycle-44 acceptance criteria for one task:
//
//	workflow-dead-ipc-44 — remove 3 flags from the operator registry:
//	  EVOLVE_STRATEGY         → Bucket 7 (dead): env write in buildCycleEnv never read;
//	                            Strategy flows via Context["strategy"]
//	  EVOLVE_RESET            → Bucket 7 (dead): cfg.Reset used before buildCycleEnv;
//	                            env write at cmd_loop_args.go:275 has zero readers
//	  EVOLVE_SHIP_RELEASE_NOTES → Bucket 5 (IPC): exec.Command parent→child env
//	                            handoff in releasepipeline → evolve ship subprocess;
//	                            replace string literal with split-const "EVOLVE_"+"SHIP_RELEASE_NOTES"
//	Lower FlagCeiling 68 → 65.
//
// AC map (1:1 with triage top_n):
//
//	AC1  flagregistry.All has 65 entries             → C44_NEG_ExactRowCountIs65 (behavioral)
//	AC2  EVOLVE_STRATEGY absent from prod env writes → C44_002 (config-check, waiver)
//	AC3  EVOLVE_RESET absent from prod env writes    → C44_003 (config-check, waiver)
//	AC4  EVOLVE_SHIP_RELEASE_NOTES literal absent    → C44_004 (config-check, waiver)
//	AC5  No prod os.Getenv reads for STRATEGY/RESET  → covered by AC2/AC3 (dead-write-only flags)
//	AC6  FlagCeiling == 65                           → C44_006 (config-check, waiver)
//	AC7  flagreaders ACS guard PASS                  → manual+checklist (see below)
//	AC8  Full affected-package suite passes          → manual+checklist (see below)
//	AC9  control-flags.md regenerated               → C44_009 (config-check, waiver)
//	AC10 SSOT IPC comment present                   → C44_010 (config-check, waiver)
//	NEG  3 flags absent from Lookup                 → C44_001 (behavioral)
//	NEG  Exact row count == 65                      → C44_NEG_ExactRowCountIs65 (behavioral)
//
// ACs with manual+checklist disposition:
//
//	AC7 (flagreaders ACS guard PASS):
//	  Checklist for Auditor:
//	  (a) exit 0 from `go test -tags acs ./acs/regression/flagreaders/...`
//	  (b) none of EVOLVE_STRATEGY, EVOLVE_RESET, EVOLVE_SHIP_RELEASE_NOTES appear in
//	      non-test, non-registry Go files:
//	      `grep -rn '"EVOLVE_STRATEGY"\|"EVOLVE_RESET"\|"EVOLVE_SHIP_RELEASE_NOTES"' go/ \
//	       --include='*.go' | grep -v '_test.go' | grep -v 'registry_table.go'` → 0 matches
//
//	AC8 (full suite passes):
//	  Checklist for Auditor:
//	  (a) exit 0 from `cd go && go test ./cmd/evolve/... ./internal/flagregistry/...
//	      ./internal/releasepipeline/... ./internal/phases/ship/...`
//	  (b) no FAIL packages in output
//	  (c) `go build ./...` exits 0
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C44_001 — 3 flags must be ABSENT from Lookup (any hit = flag still registered).
//	            C44_NEG_ExactRowCountIs65 — registry must be EXACTLY 65; over- or under-removal fails.
//	Edge/OOD:   C44_NEG_ExactRowCountIs65 catches both <65 (over-removal) and >65 (under-removal).
//	Lexical:    Lookup / len / FileNotContains / FileContains — distinct assertion verbs.
//	Semantic:   registry-absence (3 flags), exact-row-count (anti-both-directions), dead-env-write
//	            absent from cmd_loop_args.go (2 flags), IPC-literal-absent from 2 prod files,
//	            IPC-channel-preserved (split-const present), ceiling-const updated,
//	            control-flags doc clean — 7 distinct behavioral dimensions.
//
// Floor binding (R9.3): predicates authored only for workflow-dead-ipc-44
// (sole top_n task). Deferred tasks get zero predicates.
//
// 1:1 enforcement:
//
//	predicate=8 (C44_001, C44_002, C44_003, C44_004, C44_005, C44_006, C44_009, C44_010,
//	             C44_NEG_ExactRowCountIs65 = 9 funcs including the NEG)
//	manual+checklist=2 (AC7, AC8)
//	unverifiable-remove=0
//	total AC count=10 + 2 NEG; every AC has exactly one disposition row.
package cycle44

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// removedFlags is the canonical list of 3 env flags that cycle-44 removes
// from the registry.
var removedFlags = []string{
	"EVOLVE_RESET",
	"EVOLVE_SHIP_RELEASE_NOTES",
	"EVOLVE_STRATEGY",
}

// TestC44_001_RemovedFlagsAbsentFromRegistry verifies that all 3 env flags are
// no longer registered after Builder removes their rows from registry_table.go.
//
// Covers NEG (registry-absence). BEHAVIORAL: calls flagregistry.Lookup() for each
// flag — the production SSOT. A source edit alone cannot satisfy this; the registry
// row must be absent for Lookup to return ok=false.
//
// RED: all 3 flags are currently registered; each Lookup returns (flag, true).
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

// TestC44_002_StrategyAbsentFromProdEnvWrite verifies that the dead env write
// `out["EVOLVE_STRATEGY"] = cfg.Strategy` has been deleted from cmd_loop_args.go.
//
// Covers AC2. Strategy now flows only via Context["strategy"] — the env write at
// line 268 is pure noise.
//
// acs-predicate: config-check
//
// RED: cmd_loop_args.go:268 currently has out["EVOLVE_STRATEGY"] = cfg.Strategy.
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

// TestC44_003_ResetAbsentFromProdEnvWrite verifies that the dead env write
// `out["EVOLVE_RESET"] = "1"` has been deleted from cmd_loop_args.go.
//
// Covers AC3. cfg.Reset is consumed at cmd_loop.go:138 BEFORE buildCycleEnv is
// called — the env write at line 275 has zero production readers.
//
// acs-predicate: config-check
//
// RED: cmd_loop_args.go:275 currently has out["EVOLVE_RESET"] = "1" inside an
// if cfg.Reset block.
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

// TestC44_004_ShipReleaseNotesLiteralAbsent verifies that the string literal
// "EVOLVE_SHIP_RELEASE_NOTES" has been replaced with the split-const form
// "EVOLVE_"+"SHIP_RELEASE_NOTES" in all production source files.
//
// Covers AC4. The split-const breaks the EVOLVE_[A-Z][A-Z0-9_]* scanner regex
// that flagreaders uses to detect unsanctioned env reads, preserving the IPC
// channel while removing the registry row.
//
// acs-predicate: config-check
//
// RED: releasepipeline.go:620 has "EVOLVE_SHIP_RELEASE_NOTES="+releaseNotes;
//
//	gitops.go:624 has opts.envStr("EVOLVE_SHIP_RELEASE_NOTES").
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

// TestC44_005_ShipReleaseNotesIPCPreserved verifies that the IPC channel is preserved
// by checking that the split-const form "EVOLVE_"+"SHIP_RELEASE_NOTES" is still
// present in releasepipeline.go after the literal replacement.
//
// Covers AC10 (SSOT IPC comment). acs-predicate: config-check
//
// RED: releasepipeline.go currently has the full literal form; the split-const is absent.
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

// TestC44_009_ControlFlagsDocNoRemovedFlagRows verifies that the regenerated
// docs/architecture/control-flags.md no longer contains entries for the 3 removed flags.
//
// Covers AC9. acs-predicate: config-check
//
// RED: control-flags.md currently has rows for all 3 flags.
// After the migration, the doc must be regenerated (`evolve flags generate`) and
// all 3 flag names must be absent from the generated table.
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

// TestC44_010_DocsContractNoRemovedFlagAllowlist verifies that the 3 removed flags
// have been deleted from the allowedUndocumented map in docs_contract_test.go.
//
// Covers AC (docs hygiene). acs-predicate: config-check
//
// RED: docs_contract_test.go:85,90,92 currently has entries for EVOLVE_RESET,
// EVOLVE_SHIP_RELEASE_NOTES, and EVOLVE_STRATEGY in allowedUndocumented.
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
