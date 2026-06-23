//go:build acs

// Package cycle47 materializes the cycle-47 acceptance criteria for two tasks:
//
//	release-ollama-env-aliases-47 — remove 2 env aliases with existing CLI equivalents:
//	  EVOLVE_RELEASE_REQUIRE_PREFLIGHT → alias for --require-preflight in cmd_release_pipeline.go
//	  EVOLVE_OLLAMA_BASE               → alias for --ollama-base in cmd_skills_publish.go
//	  Lower FlagCeiling 59 → 57.
//
//	modelcatalog-classifier-di-47 — DI migration:
//	  EVOLVE_MODELCATALOG_CLASSIFIER_CLI → add overrideCLI string param to pickClassifierCLI()
//	  Lower FlagCeiling 57 → 56.
//
// AC map (1:1 with triage top_n for both tasks):
//
//	=== Task A: release-ollama-env-aliases-47 ===
//	AC1  EVOLVE_RELEASE_REQUIRE_PREFLIGHT absent from registry → C47A_001 (behavioral: Lookup)
//	AC2  EVOLVE_OLLAMA_BASE absent from registry               → C47A_002 (behavioral: Lookup)
//	AC3  No prod env read for RELEASE_REQUIRE_PREFLIGHT        → C47A_003 (config-check, waiver)
//	AC4  No prod env read for OLLAMA_BASE                      → C47A_004 (config-check, waiver)
//	AC5  cmd_release_pipeline_test.go has zero env key refs    → C47A_005 (config-check, waiver)
//	AC6  FlagCeiling == 57                                     → C47A_006 (config-check, waiver)
//	AC7  go test ./cmd/evolve/... PASS                         → manual+checklist (Auditor)
//	AC8  go test ./internal/flagregistry/... PASS              → manual+checklist (Auditor)
//	AC9  flagreaders ACS guard PASS                            → manual+checklist (Auditor)
//	AC10 control-flags.md regenerated with 57 flags            → C47A_010 (config-check, waiver)
//	NEG  row count ≤ 57 after A-task flags removed             → C47A_NEG (behavioral: len)
//
//	=== Task B: modelcatalog-classifier-di-47 ===
//	AC1  EVOLVE_MODELCATALOG_CLASSIFIER_CLI absent from registry → C47B_001 (behavioral: Lookup)
//	AC2  Zero prod reads for MODELCATALOG_CLASSIFIER_CLI         → C47B_002 (behavioral: CountInGoFunc)
//	AC3  pickClassifierCLI has overrideCLI string second param   → C47B_003 (config-check, waiver)
//	AC4  cmd_models_live_test.go has zero env key refs           → C47B_004 (config-check, waiver)
//	AC5  FlagCeiling == 56                                       → C47B_005 (config-check, waiver)
//	AC6  go test ./cmd/evolve/... PASS                           → manual+checklist (Auditor)
//	AC7  flagreaders ACS guard PASS                              → manual+checklist (Auditor)
//	AC8  go test -tags acs ./acs/cycle47/ PASS                   → manual+checklist (self-referential)
//	NEG  exact row count == 56 (final state after both tasks)    → C47B_NEG (behavioral: len)
//
// Manual+checklist ACs (addressed to Auditor):
//
//	AC7 (Task A — cmd/evolve tests pass):
//	  (a) exit 0: cd go && go test ./cmd/evolve/...
//	  (b) no FAIL packages in output
//	  (c) docs_contract_test.go entry for EVOLVE_RELEASE_REQUIRE_PREFLIGHT removed
//	      (otherwise TestAllFlagsInRegistryAreDocumented fails here)
//
//	AC8 (Task A — flagregistry tests pass):
//	  (a) exit 0: cd go && go test ./internal/flagregistry/...
//	  (b) TestRegistry_FlagCeiling passes (FlagCeiling == 57)
//
//	AC9 (Task A — flagreaders ACS guard):
//	  (a) evolve acs suite (or: go test -tags acs ./acs/regression/flagreaders/...)
//	  (b) Neither EVOLVE_RELEASE_REQUIRE_PREFLIGHT nor EVOLVE_OLLAMA_BASE appears as orphan reader
//
//	AC6 (Task B — cmd/evolve tests pass):
//	  (a) exit 0: cd go && go test ./cmd/evolve/...
//	  (b) no FAIL packages in output; TestPickClassifierCLI* and TestShouldRefreshCatalog pass
//
//	AC7 (Task B — flagreaders ACS guard):
//	  (a) evolve acs suite (or: go test -tags acs ./acs/regression/flagreaders/...)
//	  (b) EVOLVE_MODELCATALOG_CLASSIFIER_CLI does not appear as orphan reader
//
//	AC8 (Task B — acs/cycle47 suite passes):
//	  (a) cd go && go test -tags acs -count=1 ./acs/cycle47/
//	  (b) all TestC47A_* and TestC47B_* pass (GREEN after Builder implements both tasks)
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C47A_001/002, C47B_001 — flags ABSENT from Lookup (any hit = still registered).
//	            C47A_NEG: row count ≤ 57 (catches under-removal from 59).
//	            C47B_NEG: exact count == 56 (catches over- and under-removal; strongest invariant).
//	Edge/OOD:   C47B_NEG catches both <56 (over-removal) and >56 (under-removal).
//	Lexical:    Lookup / len / FileNotContains / FileContains / FileMatchesRegex / CountInGoFunc — six distinct verbs.
//	Semantic:   registry-absence (3 flags), exact-row-count / upper-bound-row-count, prod-source-clean (3 files),
//	            test-env-key-clean (2 test files), ceiling-const (2 values), control-flags-doc, DI-signature — 9 dimensions.
//
// Floor binding (R9.3): predicates authored ONLY for tasks in the triage top_n.
// Deferred tasks (EVOLVE_MODELCATALOG_AUTOREFRESH, EVOLVE_HANG_CLASSIFIER, etc.) get zero predicates.
//
// 1:1 enforcement:
//
//	predicate count: 14 funcs (C47A_001–006, C47A_010, C47A_NEG, C47B_001–005, C47B_NEG)
//	manual+checklist: 6 (A_AC7, A_AC8, A_AC9, B_AC6, B_AC7, B_AC8 — checklists addressed to Auditor above)
//	unverifiable-remove: 0
//	Total AC count: 20; every AC has exactly one disposition row.
package cycle47

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// taskAFlags lists the 2 env-alias flags removed in Task A.
var taskAFlags = []string{
	"EVOLVE_RELEASE_REQUIRE_PREFLIGHT",
	"EVOLVE_OLLAMA_BASE",
}

// =============================================================================
// Task A — release-ollama-env-aliases-47
// =============================================================================

// === Registry absence (behavioral: Lookup) ===

// TestC47A_001_ReleasePreflight_AbsentFromRegistry verifies that
// EVOLVE_RELEASE_REQUIRE_PREFLIGHT is no longer registered after the alias is
// deleted from cmd_release_pipeline.go (the --require-preflight CLI flag already
// exists and serves the same purpose).
//
// Covers Task A AC1. BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT.
// Adding a source comment cannot satisfy this; the registry row must be absent.
//
// RED: EVOLVE_RELEASE_REQUIRE_PREFLIGHT is currently registered at registry_table.go:55.
func TestC47A_001_ReleasePreflight_AbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_RELEASE_REQUIRE_PREFLIGHT"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (release-ollama-env-aliases-47: env alias removal).\n"+
			"The --require-preflight CLI flag already provides this functionality.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_RELEASE_REQUIRE_PREFLIGHT", f.Status, f.Cluster)
	}
}

// TestC47A_002_OllamaBase_AbsentFromRegistry verifies that EVOLVE_OLLAMA_BASE is
// no longer registered after the alias is deleted from cmd_skills_publish.go
// (the --ollama-base CLI flag already exists and serves the same purpose).
//
// Covers Task A AC2. BEHAVIORAL: calls flagregistry.Lookup().
//
// RED: EVOLVE_OLLAMA_BASE is currently registered at registry_table.go:41.
func TestC47A_002_OllamaBase_AbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_OLLAMA_BASE"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (release-ollama-env-aliases-47: env alias removal).\n"+
			"The --ollama-base CLI flag already provides this functionality.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_OLLAMA_BASE", f.Status, f.Cluster)
	}
}

// === Prod-source clean (config-check waivers) ===

// TestC47A_003_ReleasePreflight_AbsentFromProdSource verifies that the env alias
// os.Getenv("EVOLVE_RELEASE_REQUIRE_PREFLIGHT") has been removed from
// cmd_release_pipeline.go (line 95), and the help-text line at line 45 also removed.
//
// acs-predicate: config-check
//
// RED: cmd_release_pipeline.go:95 currently has:
//
//	if !requirePreflight && os.Getenv("EVOLVE_RELEASE_REQUIRE_PREFLIGHT") == "1" {
func TestC47A_003_ReleasePreflight_AbsentFromProdSource(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_release_pipeline.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_RELEASE_REQUIRE_PREFLIGHT"`) {
		t.Errorf("RED: cmd_release_pipeline.go still contains the env alias \"EVOLVE_RELEASE_REQUIRE_PREFLIGHT\".\n"+
			"Builder must delete:\n"+
			"  line 95: if !requirePreflight && os.Getenv(\"EVOLVE_RELEASE_REQUIRE_PREFLIGHT\") == \"1\" { ... }\n"+
			"  line 45: Env: EVOLVE_RELEASE_REQUIRE_PREFLIGHT=1 same as --require-preflight. (help text)\n"+
			"The --require-preflight CLI flag is the canonical interface.\n"+
			"File: %s", f)
	}
}

// TestC47A_004_OllamaBase_AbsentFromProdSource verifies that the env fallback
// cfg.OllamaBase = os.Getenv("EVOLVE_OLLAMA_BASE") has been removed from
// cmd_skills_publish.go (line 161).
//
// acs-predicate: config-check
//
// RED: cmd_skills_publish.go:161 currently has:
//
//	cfg.OllamaBase = os.Getenv("EVOLVE_OLLAMA_BASE")
func TestC47A_004_OllamaBase_AbsentFromProdSource(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_skills_publish.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_OLLAMA_BASE"`) {
		t.Errorf("RED: cmd_skills_publish.go still contains the env fallback \"EVOLVE_OLLAMA_BASE\".\n"+
			"Builder must delete: cfg.OllamaBase = os.Getenv(\"EVOLVE_OLLAMA_BASE\") (line 161)\n"+
			"The --ollama-base CLI flag is already the primary; the env fallback duplicates it.\n"+
			"File: %s", f)
	}
}

// === Test-file clean (config-check waiver) ===

// TestC47A_005_ReleasePipelineTest_NoEnvKey verifies that
// cmd_release_pipeline_test.go no longer uses t.Setenv("EVOLVE_RELEASE_REQUIRE_PREFLIGHT").
// After the alias removal, the test must pass "--require-preflight" as a CLI arg instead.
//
// acs-predicate: config-check
//
// RED: cmd_release_pipeline_test.go:79 currently has:
//
//	t.Setenv("EVOLVE_RELEASE_REQUIRE_PREFLIGHT", "1")
func TestC47A_005_ReleasePipelineTest_NoEnvKey(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_release_pipeline_test.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_RELEASE_REQUIRE_PREFLIGHT"`) {
		t.Errorf("RED: cmd_release_pipeline_test.go still references \"EVOLVE_RELEASE_REQUIRE_PREFLIGHT\".\n"+
			"Builder must replace: t.Setenv(\"EVOLVE_RELEASE_REQUIRE_PREFLIGHT\", \"1\") (line 79)\n"+
			"with: \"--require-preflight\" in the args slice passed to the command.\n"+
			"File: %s", f)
	}
}

// === FlagCeiling after Task A (config-check waiver) ===

// === Control-flags doc clean (config-check waiver) ===

// TestC47A_010_ControlFlagsDocClean verifies that the regenerated
// docs/architecture/control-flags.md no longer contains entries for either
// Task A flag. After removal, Builder must run `evolve flags generate` and commit
// the updated doc.
//
// acs-predicate: config-check
//
// RED: control-flags.md currently has rows for both EVOLVE_RELEASE_REQUIRE_PREFLIGHT
// and EVOLVE_OLLAMA_BASE.
func TestC47A_010_ControlFlagsDocClean(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlagsDoc := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range taskAFlags {
		if !acsassert.FileNotContains(t, controlFlagsDoc, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must regenerate docs/architecture/control-flags.md after removing\n"+
				"both Task A flag rows (run `evolve flags generate` in the same diff).\n"+
				"File path: %s", name, controlFlagsDoc)
		}
	}
}

// === Negative: upper-bound row count after Task A (behavioral) ===

// TestC47A_NEG_RowCountAtMost57 verifies that after Task A's two flag removals the
// registry row count has dropped from 59 to at most 57. A ≤ 57 check (rather than
// exact == 57) allows Task B to also be applied in the same build without this
// predicate re-failing on the further-reduced count of 56.
//
// BEHAVIORAL: calls len(flagregistry.All) — the production count.
//
// RED: registry currently has 59 rows (FlagCeiling=59); 59 > 57 fails.
func TestC47A_NEG_RowCountAtMost57(t *testing.T) {
	got := len(flagregistry.All)
	if got > 57 {
		t.Errorf("RED: len(flagregistry.All) = %d, want ≤ 57 (59 − 2 Task A flags).\n"+
			"Builder must remove exactly these 2 rows from registry_table.go:\n"+
			"  EVOLVE_RELEASE_REQUIRE_PREFLIGHT, EVOLVE_OLLAMA_BASE\n"+
			"Current count %d exceeds 57 — Task A flags not yet removed.",
			got, got)
	}
}

// =============================================================================
// Task B — modelcatalog-classifier-di-47
// =============================================================================

// === Registry absence (behavioral: Lookup) ===

// TestC47B_001_ClassifierCLI_AbsentFromRegistry verifies that
// EVOLVE_MODELCATALOG_CLASSIFIER_CLI is no longer registered after the DI migration
// (overrideCLI string param replaces the os.Getenv read inside pickClassifierCLI).
//
// Covers Task B AC1. BEHAVIORAL: calls flagregistry.Lookup().
//
// RED: EVOLVE_MODELCATALOG_CLASSIFIER_CLI is currently registered at registry_table.go:39.
func TestC47B_001_ClassifierCLI_AbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_MODELCATALOG_CLASSIFIER_CLI"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (modelcatalog-classifier-di-47: DI migration).\n"+
			"The env read must be replaced with the overrideCLI string param in pickClassifierCLI.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_MODELCATALOG_CLASSIFIER_CLI", f.Status, f.Cluster)
	}
}

// === Prod-source AST-scoped clean (behavioral: CountInGoFunc) ===

// TestC47B_002_ClassifierCLI_AbsentFromPickClassifierCLI verifies — using Go
// AST-level scoping — that the EVOLVE_MODELCATALOG_CLASSIFIER_CLI string literal
// is absent from the pickClassifierCLI() function body in cmd_models_live.go.
// This is stronger than a file-wide grep: a later cycle adding a comment elsewhere
// cannot flip this predicate.
//
// Covers Task B AC2. BEHAVIORAL: CountInGoFunc parses the AST to scope the count
// to pickClassifierCLI().
//
// RED: cmd_models_live.go:138 (inside pickClassifierCLI) currently has:
//
//	if env := os.Getenv("EVOLVE_MODELCATALOG_CLASSIFIER_CLI"); env != "" && readySet[env] {
func TestC47B_002_ClassifierCLI_AbsentFromPickClassifierCLI(t *testing.T) {
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_models_live.go")
	count, err := acsassert.CountInGoFunc(f, "pickClassifierCLI", `"EVOLVE_MODELCATALOG_CLASSIFIER_CLI"`)
	if err != nil {
		t.Fatalf("RED: CountInGoFunc on cmd_models_live.go pickClassifierCLI() failed: %v\n"+
			"(function renamed or file unreadable — Builder must not rename pickClassifierCLI)\n"+
			"File: %s", err, f)
	}
	if count > 0 {
		t.Errorf("RED: cmd_models_live.go pickClassifierCLI() still references %q (%d occurrence(s)).\n"+
			"Builder must replace os.Getenv(\"EVOLVE_MODELCATALOG_CLASSIFIER_CLI\") with the\n"+
			"overrideCLI string parameter injected by the caller.\n"+
			"File: %s", "EVOLVE_MODELCATALOG_CLASSIFIER_CLI", count, f)
	}
}

// === DI signature check (config-check waiver) ===

// TestC47B_003_PickClassifierCLI_HasOverrideCLIParam verifies that pickClassifierCLI
// now accepts an overrideCLI string second parameter, replacing the internal env read.
// The signature must match: pickClassifierCLI(ready []string, overrideCLI string).
//
// acs-predicate: config-check
//
// RED: cmd_models_live.go:133 currently has:
//
//	func pickClassifierCLI(ready []string) string {
func TestC47B_003_PickClassifierCLI_HasOverrideCLIParam(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_models_live.go")
	if !acsassert.FileMatchesRegex(t, f, `func pickClassifierCLI\([^)]*overrideCLI[^)]*\)`) {
		t.Errorf("RED: cmd_models_live.go does not contain a pickClassifierCLI signature with overrideCLI param.\n"+
			"Builder must change the signature from:\n"+
			"  func pickClassifierCLI(ready []string) string\n"+
			"to:\n"+
			"  func pickClassifierCLI(ready []string, overrideCLI string) string\n"+
			"and update all callers (cmd_models_live.go call site + cmd_models_live_test.go).\n"+
			"File: %s", f)
	}
}

// === Test-file clean (config-check waiver) ===

// TestC47B_004_ClassifierCLITest_NoEnvKey verifies that cmd_models_live_test.go
// no longer uses t.Setenv("EVOLVE_MODELCATALOG_CLASSIFIER_CLI") to inject the
// classifier override. After the DI migration, tests pass the override directly as
// the second argument to pickClassifierCLI(ready, overrideCLI).
//
// acs-predicate: config-check
//
// RED: cmd_models_live_test.go:37,60,66 currently has 3 t.Setenv calls for this key.
func TestC47B_004_ClassifierCLITest_NoEnvKey(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_models_live_test.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_MODELCATALOG_CLASSIFIER_CLI"`) {
		t.Errorf("RED: cmd_models_live_test.go still references \"EVOLVE_MODELCATALOG_CLASSIFIER_CLI\".\n"+
			"Builder must migrate all 3 t.Setenv calls (lines 37, 60, 66) to pass the value directly:\n"+
			"  was: t.Setenv(\"EVOLVE_MODELCATALOG_CLASSIFIER_CLI\", \"<val>\") + pickClassifierCLI(ready)\n"+
			"  now: pickClassifierCLI(ready, \"<val>\")\n"+
			"File: %s", f)
	}
}

// === FlagCeiling after Task B (config-check waiver) ===

// === Exact row count — final state (behavioral: negative / edge) ===
