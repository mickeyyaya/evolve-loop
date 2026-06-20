//go:build acs

// Package cycle46 materializes the cycle-46 acceptance criteria for task:
//
//	retro-stdout-config-46 — remove 2 flags from the operator registry:
//	  EVOLVE_RETRO_MODEL   → Bucket 1 (Config Object): retro.Config{Model string}
//	                         replaces req.Env["EVOLVE_RETRO_MODEL"] in Run().
//	  EVOLVE_STDOUT_FILTER → Bucket 1 (Config Object / DI): runner.Options{DisableStdoutFilter bool}
//	                         replaces envchain.Resolve("EVOLVE_STDOUT_FILTER", req.Env, "", "on").
//	Lower FlagCeiling 61 → 59.
//
// AC map (1:1 with triage top_n):
//
//	AC1  EVOLVE_RETRO_MODEL absent from registry         → C46_001 (behavioral: Lookup)
//	AC2  EVOLVE_STDOUT_FILTER absent from registry       → C46_002 (behavioral: Lookup)
//	AC3  No prod env read for EVOLVE_RETRO_MODEL         → C46_003 (config-check, waiver)
//	AC4  No prod envchain read for EVOLVE_STDOUT_FILTER  → C46_004 (config-check, waiver)
//	AC5  FlagCeiling == 59                               → C46_005 (config-check, waiver)
//	AC6  retro.Config.Model string field exists          → C46_006 (behavioral: reflect)
//	AC7  runner.Options.DisableStdoutFilter bool exists  → C46_007 (behavioral: reflect)
//	AC8  retro tests pass                                → manual+checklist (Auditor)
//	AC9  runner tests pass                               → manual+checklist (Auditor)
//	AC10 flagregistry tests pass                         → manual+checklist (Auditor)
//	AC11 flagreaders ACS guard passes                    → manual+checklist (Auditor; standing regression)
//	AC12 acs/cycle46 predicates pass                     → this file (self-referential, no extra func)
//	AC13-NEG No EVOLVE_STDOUT_FILTER env key in test    → C46_009 (config-check, waiver)
//
// Manual+checklist ACs (addressed to Auditor):
//
//	AC8 (retro tests pass):
//	  (a) exit 0: cd go && go test ./internal/phases/retro/...
//	  (b) no FAIL packages in output
//
//	AC9 (runner tests pass):
//	  (a) exit 0: cd go && go test ./internal/phases/runner/...
//	  (b) no FAIL packages in output
//
//	AC10 (flagregistry tests pass):
//	  (a) exit 0: cd go && go test ./internal/flagregistry/...
//	  (b) TestRegistry_FlagCeiling passes (FlagCeiling == 59)
//
//	AC11 (flagreaders ACS guard):
//	  (a) evolve acs suite (or: go test -tags acs ./acs/regression/flagreaders/...)
//	  (b) Neither EVOLVE_RETRO_MODEL nor EVOLVE_STDOUT_FILTER appears as an orphan reader
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C46_001/002 — both flags ABSENT from Lookup (any hit = still registered).
//	            C46_NEG_ExactRowCountIs59 — registry EXACTLY 59; over- or under-removal fails.
//	Edge/OOD:   ExactRowCountIs59 catches both <59 (over-removal) and >59 (under-removal).
//	Lexical:    Lookup / len / FileNotContains / FileContains / reflect / CountInGoFunc — distinct verbs.
//	Semantic:   registry-absence (2 flags), exact-row-count, prod-source-clean (2 files),
//	            struct-field-existence (2 types), docs-contract, test-env-key, ceiling-const,
//	            control-flags-doc, run-func-scoped AST — 10 distinct dimensions.
//
// Floor binding (R9.3): predicates authored ONLY for retro-stdout-config-46
// (in triage top_n). Deferred tasks get zero predicates.
//
// 1:1 enforcement:
//
//	predicate count: 12 funcs (C46_001-009, C46_010, C46_012, C46_NEG)
//	manual+checklist: 4 (AC8, AC9, AC10, AC11 — checklists addressed to Auditor above)
//	unverifiable-remove: 0
//	Total AC count: 16; every AC has exactly one disposition row.
package cycle46

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/retro"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// removedFlags lists the 2 env flags removed in cycle 46.
var removedFlags = []string{
	"EVOLVE_RETRO_MODEL",
	"EVOLVE_STDOUT_FILTER",
}

// === Registry absence (behavioral: Lookup) ===

// TestC46_001_RetroModelAbsentFromRegistry verifies that EVOLVE_RETRO_MODEL is no
// longer registered in the flag registry after the Config Object migration.
//
// Covers AC1. BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT.
// Adding a source comment cannot satisfy this; the registry row must be absent.
//
// RED: EVOLVE_RETRO_MODEL is currently registered at registry_table.go:57.
func TestC46_001_RetroModelAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_RETRO_MODEL"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (retro-stdout-config-46: Config Object).\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_RETRO_MODEL", f.Status, f.Cluster)
	}
}

// TestC46_002_StdoutFilterAbsentFromRegistry verifies that EVOLVE_STDOUT_FILTER is
// no longer registered in the flag registry after the DI field migration.
//
// Covers AC2. BEHAVIORAL: calls flagregistry.Lookup().
//
// RED: EVOLVE_STDOUT_FILTER is currently registered at registry_table.go:64.
func TestC46_002_StdoutFilterAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_STDOUT_FILTER"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (retro-stdout-config-46: DI field).\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_STDOUT_FILTER", f.Status, f.Cluster)
	}
}

// === Prod-source clean (config-check waivers) ===

// TestC46_003_RetroModelAbsentFromProdSource verifies that the env map read
// req.Env["EVOLVE_RETRO_MODEL"] has been deleted from retro.go and replaced
// with p.model (sourced from Config.Model in New()).
//
// acs-predicate: config-check
//
// RED: retro.go:83 currently has: model := req.Env["EVOLVE_RETRO_MODEL"].
func TestC46_003_RetroModelAbsentFromProdSource(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "phases", "retro", "retro.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_RETRO_MODEL"`) {
		t.Errorf("RED: retro.go still contains the env read \"EVOLVE_RETRO_MODEL\".\n"+
			"Builder must delete: model := req.Env[\"EVOLVE_RETRO_MODEL\"] (line 83)\n"+
			"and replace it with model := p.model (sourced from Config.Model in New()).\n"+
			"File: %s", f)
	}
}

// TestC46_004_StdoutFilterAbsentFromProdSource verifies that the envchain.Resolve
// call for "EVOLVE_STDOUT_FILTER" has been deleted from runner.go and replaced
// with !b.disableStdoutFilter (sourced from opts.DisableStdoutFilter in New()).
//
// acs-predicate: config-check
//
// RED: runner.go:634 currently has: envchain.Resolve("EVOLVE_STDOUT_FILTER", req.Env, "", "on").
func TestC46_004_StdoutFilterAbsentFromProdSource(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "phases", "runner", "runner.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_STDOUT_FILTER"`) {
		t.Errorf("RED: runner.go still contains the envchain.Resolve(\"EVOLVE_STDOUT_FILTER\") call (line 634).\n"+
			"Builder must replace it with !b.disableStdoutFilter (sourced from opts.DisableStdoutFilter in New()).\n"+
			"File: %s", f)
	}
}

// === Struct field existence (behavioral: reflect) ===

// TestC46_006_RetroConfigHasModelField verifies that retro.Config has a Model string
// field after the Config Object migration. This is the DI seam that replaces
// the env map read.
//
// Covers AC6. BEHAVIORAL: uses reflect.TypeOf to check the field exists at runtime.
// A source-file edit that adds a Model field to a DIFFERENT struct cannot satisfy this.
//
// RED: retro.Config currently only has Bridge, Prompts, NowFn — no Model field.
func TestC46_006_RetroConfigHasModelField(t *testing.T) {
	rt := reflect.TypeOf(retro.Config{})
	f, ok := rt.FieldByName("Model")
	if !ok {
		t.Fatalf("RED: retro.Config has no 'Model' field.\n"+
			"Builder must add: Model string to the Config struct in retro/retro.go\n"+
			"(type: %s currently has fields: %s)", rt.Name(), fieldNames(rt))
	}
	if f.Type.Kind() != reflect.String {
		t.Errorf("RED: retro.Config.Model has kind %v, want string", f.Type.Kind())
	}
}

// TestC46_007_RunnerOptionsHasDisableStdoutFilter verifies that runner.Options has a
// DisableStdoutFilter bool field after the DI field migration. This is the seam
// that replaces the envchain.Resolve("EVOLVE_STDOUT_FILTER", ...) call.
//
// Covers AC7. BEHAVIORAL: uses reflect.TypeOf to check the field exists at runtime.
//
// RED: runner.Options currently does not have a DisableStdoutFilter field.
func TestC46_007_RunnerOptionsHasDisableStdoutFilter(t *testing.T) {
	rt := reflect.TypeOf(runner.Options{})
	f, ok := rt.FieldByName("DisableStdoutFilter")
	if !ok {
		t.Fatalf("RED: runner.Options has no 'DisableStdoutFilter' field.\n"+
			"Builder must add: DisableStdoutFilter bool to Options struct in runner/runner.go\n"+
			"(type: %s does not currently have this field)", rt.Name())
	}
	if f.Type.Kind() != reflect.Bool {
		t.Errorf("RED: runner.Options.DisableStdoutFilter has kind %v, want bool", f.Type.Kind())
	}
}

// fieldNames returns a comma-joined list of exported field names for a struct type.
// Used in diagnostic messages only.
func fieldNames(t reflect.Type) string {
	var names []string
	for i := 0; i < t.NumField(); i++ {
		names = append(names, t.Field(i).Name)
	}
	if len(names) == 0 {
		return "(none)"
	}
	result := names[0]
	for _, n := range names[1:] {
		result += ", " + n
	}
	return result
}

// === Docs contract clean (config-check waiver) ===

// TestC46_008_RetroModelRemovedFromDocsContract verifies that EVOLVE_RETRO_MODEL has
// been removed from the allowedUndocumented map in docs_contract_test.go.
//
// acs-predicate: config-check
//
// RED: docs_contract_test.go:87 currently has "EVOLVE_RETRO_MODEL": true.
func TestC46_008_RetroModelRemovedFromDocsContract(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "docs_contract_test.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_RETRO_MODEL"`) {
		t.Errorf("RED: docs_contract_test.go still has \"EVOLVE_RETRO_MODEL\" in allowedUndocumented.\n"+
			"Builder must remove the entry \"EVOLVE_RETRO_MODEL\": true (line 87).\n"+
			"After removing from registry, the allowedUndocumented entry is stale.\n"+
			"File: %s", f)
	}
}

// === Negative: no env-key pattern in test (AC13-NEG) ===

// TestC46_009_StdoutFilterTestNoEnvKey verifies that stdout_filter_test.go no longer
// uses the EVOLVE_STDOUT_FILTER env key to drive the test. After the DI migration,
// DisableStdoutFilter: true in Options replaces the env map entry.
//
// acs-predicate: config-check
//
// RED: stdout_filter_test.go:100 currently has: Env: map[string]string{"EVOLVE_STDOUT_FILTER": "off"}.
func TestC46_009_StdoutFilterTestNoEnvKey(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "phases", "runner", "stdout_filter_test.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_STDOUT_FILTER"`) {
		t.Errorf("RED: stdout_filter_test.go still references \"EVOLVE_STDOUT_FILTER\" as an env key.\n"+
			"Builder must rewrite TestRun_StdoutFilter_OffEnvSkipsFilter (line 100) to use\n"+
			"Options{DisableStdoutFilter: true} instead of Env: map[string]string{\"EVOLVE_STDOUT_FILTER\": \"off\"}.\n"+
			"File: %s", f)
	}
}

// === FlagCeiling (config-check waiver) ===

// TestC46_005_FlagCeilingIs59 verifies that the FlagCeiling ratchet constant has been
// updated to 59 after removing both flags (61 − 2 = 59).
//
// acs-predicate: config-check
//
// RED: registry_ceiling_test.go currently has FlagCeiling = 61.
func TestC46_005_FlagCeilingIs59(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	ceilingFile := filepath.Join(root, "go", "internal", "flagregistry", "registry_ceiling_test.go")
	if !acsassert.FileContains(t, ceilingFile, "FlagCeiling = 59") {
		t.Errorf("RED: registry_ceiling_test.go does not contain 'FlagCeiling = 59'.\n"+
			"Builder must lower the FlagCeiling constant to 59 in the same diff as\n"+
			"removing both registry rows (61 − 2 = 59).\n"+
			"File: %s", ceilingFile)
	}
}

// === Control-flags doc clean (config-check waiver) ===

// TestC46_010_ControlFlagsDocClean verifies that the regenerated
// docs/architecture/control-flags.md no longer contains entries for either removed flag.
//
// acs-predicate: config-check
//
// RED: control-flags.md currently has rows for EVOLVE_RETRO_MODEL and EVOLVE_STDOUT_FILTER.
// After migration, Builder must run `evolve flags generate` and commit the updated doc.
func TestC46_010_ControlFlagsDocClean(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlagsDoc := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range removedFlags {
		if !acsassert.FileNotContains(t, controlFlagsDoc, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must regenerate docs/architecture/control-flags.md after removing\n"+
				"both flag rows (run `evolve flags generate` in the same diff).\n"+
				"File path: %s", name, controlFlagsDoc)
		}
	}
}

// === Run-func scoped AST check (behavioral) ===

// TestC46_012_RetroModelRunFuncEnvReadGone verifies — using Go AST-level scope —
// that the EVOLVE_RETRO_MODEL string literal is absent from the Run() function body
// in retro.go. This is stronger than a file-wide grep: a later cycle adding a
// comment elsewhere in the file cannot flip this predicate.
//
// BEHAVIORAL: CountInGoFunc parses the AST to scope the count to Run().
//
// RED: retro.go:83 (inside Run) currently has: model := req.Env["EVOLVE_RETRO_MODEL"].
func TestC46_012_RetroModelRunFuncEnvReadGone(t *testing.T) {
	root := acsassert.RepoRoot(t)
	retroFile := filepath.Join(root, "go", "internal", "phases", "retro", "retro.go")
	count, err := acsassert.CountInGoFunc(retroFile, "Run", `"EVOLVE_RETRO_MODEL"`)
	if err != nil {
		t.Fatalf("RED: CountInGoFunc on retro.go Run() failed: %v\n"+
			"(function renamed or file unreadable — Builder must not rename Run)", err)
	}
	if count > 0 {
		t.Errorf("RED: retro.go Run() still references \"EVOLVE_RETRO_MODEL\" (%d occurrence(s)).\n"+
			"Builder must replace the env map read with p.model (sourced from Config.Model).\n"+
			"File: %s", count, retroFile)
	}
}

// === Exact row count (behavioral: negative / edge) ===

// TestC46_NEG_ExactRowCountIs59 verifies that after removing both flags the total
// registry count is exactly 59.
//
// BEHAVIORAL: calls len(flagregistry.All) — the production count.
// Over-removal (<59) AND under-removal (>59) both fail.
//
// RED: registry currently has 61 rows (FlagCeiling=61).
func TestC46_NEG_ExactRowCountIs59(t *testing.T) {
	got := len(flagregistry.All)
	if got != 59 {
		t.Errorf("RED: len(flagregistry.All) = %d, want 59 (61 − 2 removed flags).\n"+
			"Builder must remove exactly 2 rows from registry_table.go:\n"+
			"  EVOLVE_RETRO_MODEL, EVOLVE_STDOUT_FILTER\n"+
			"Over-removal (<59) and under-removal (>59) both fail. Current count: %d",
			got, got)
	}
}
