# Comment history: `acs/cycle50`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle50/predicates_test.go:3` — above `package cycle50`

```text
// Package cycle50 materializes the cycle-50 acceptance criteria for two tasks:
//
//	codex-config-path-di-50 — remove EVOLVE_CODEX_CONFIG_PATH from registry:
//	  Single os.Getenv read in codex_pretrust.go:142 migrates to codexConfigPath
//	  string field on bridge.Config struct. ALL 5 test files (codex_pretrust_test.go,
//	  codex_pretrust_amplify_test.go, codex_pretrust_concurrent_test.go,
//	  codex_pretrust_launch_test.go, preflight_test.go) replace t.Setenv calls with
//	  cfg.codexConfigPath field assignment. Lower FlagCeiling 52 → 51.
//
//	release-strict-pass-cli-50 — remove EVOLVE_RELEASE_STRICT_PASS from registry:
//	  Two prod os.Getenv reads (cmd_release_preflight.go:55, releasepipeline/bridges.go:26)
//	  migrate to --strict-pass CLI flag + releasepipeline.Options.StrictPass bool field.
//	  Lower FlagCeiling 51 → 50.
//
// AC map (1:1 with triage top_n for both tasks):
//
//	=== Task A: codex-config-path-di-50 ===
//	AC1  EVOLVE_CODEX_CONFIG_PATH absent from registry         → C50A_001 (behavioral: Lookup)
//	AC2  No prod os.Getenv in codex_pretrust.go                → C50A_002 (config-check, waiver)
//	AC3  bridge.Config has codexConfigPath string field        → C50A_003 (config-check, waiver)
//	AC4  FlagCeiling == 51 (intermediate after Task A)         → C50A_004 (config-check, waiver)
//	AC5  Zero t.Setenv("EVOLVE_CODEX_CONFIG_PATH") in bridge/ → C50A_005 (config-check, waiver)
//	AC6  bridge test suite green                               → manual+checklist (Auditor)
//	AC7  flagreaders ACS guard green                           → manual+checklist (Auditor)
//	NEG  row count ≤ 51 after Task A (allows B to reach 50)   → C50A_NEG (behavioral: len)
//
//	=== Task B: release-strict-pass-cli-50 ===
//	AC1  EVOLVE_RELEASE_STRICT_PASS absent from registry       → C50B_001 (behavioral: Lookup)
//	AC2  No prod os.Getenv in cmd_release_preflight.go         → C50B_002 (config-check, waiver)
//	AC3  No prod os.Getenv in releasepipeline/bridges.go       → C50B_003 (config-check, waiver)
//	AC4  FlagCeiling == 50 (final)                             → C50B_004 (config-check, waiver)
//	AC5  --strict-pass CLI flag registered in preflight cmd    → C50B_005 (config-check, waiver)
//	AC6  releasepipeline.Options has StrictPass bool field     → C50B_006 (config-check, waiver)
//	AC7  releasepipeline + cmd/evolve test suites green        → manual+checklist (Auditor)
//	AC8  flagreaders ACS guard green                           → manual+checklist (Auditor)
//	NEG  exact row count == 50 (final state, both tasks)       → C50B_NEG (behavioral: len)
//
// Manual+checklist ACs (addressed to Auditor):
//
//	Task A AC6 (bridge test suite green):
//	  (a) exit 0: cd go && go test ./internal/bridge/... -count=1
//	  (b) no FAIL packages in output (includes root ./internal/bridge/, not just sub-packages)
//	  (c) all 5 test files compile without t.Setenv("EVOLVE_CODEX_CONFIG_PATH")
//	  (d) TestPretrustCodexProjects* tests pass using cfg.codexConfigPath (not env override)
//
//	Task A AC7 (flagreaders ACS guard):
//	  (a) exit 0: go test -tags acs ./acs/regression/flagreaders/... -count=1
//	  (b) EVOLVE_CODEX_CONFIG_PATH absent from non-test, non-registry Go prod files:
//	      grep -rn '"EVOLVE_CODEX_CONFIG_PATH"' go/ --include='*.go' | grep -v '_test.go' | grep -v 'registry_table.go' → 0 matches
//
//	Task B AC7 (releasepipeline + cmd/evolve tests green):
//	  (a) exit 0: cd go && go test ./internal/releasepipeline/... ./cmd/evolve/... -count=1
//	  (b) no FAIL packages; bridges_test.go compiles with updated 5-arg runPreflightLib signature
//	  (c) docs_contract_test.go compiles without "EVOLVE_RELEASE_STRICT_PASS" allowedUndocumented entry
//
//	Task B AC8 (flagreaders ACS guard):
//	  (a) exit 0: go test -tags acs ./acs/regression/flagreaders/... -count=1
//	  (b) EVOLVE_RELEASE_STRICT_PASS absent from non-test, non-registry Go prod files:
//	      grep -rn '"EVOLVE_RELEASE_STRICT_PASS"' go/ --include='*.go' | grep -v '_test.go' | grep -v 'registry_table.go' → 0 matches
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C50A_001/C50B_001 — flags ABSENT from Lookup (any hit = still registered).
//	            C50A_NEG: row count ≤ 51 (upper bound; prevents Task B from failing A's check).
//	            C50B_NEG: exact count == 50 (catches over-removal <50 AND under-removal >50).
//	Edge/OOD:   C50B_NEG exact count rejects both directions; C50A_NEG is one-sided upper bound.
//	Lexical:    Lookup / len / FileNotContains / FileContains / CountInGoFunc — 5 distinct verbs.
//	Semantic:   registry-absence (2 flags), env-read-clean (3 prod files), struct-field-add (engine.go),
//	            test-migration (5 test files), ceiling-const (2 values: 51/50), cli-flag-registered
//	            (cmd_release_preflight.go), options-struct-field (releasepipeline.go),
//	            exact-row-count (final invariant) — 8 dimensions.
//
// Floor binding (R9.3): predicates authored ONLY for tasks in the triage top_n.
// Deferred tasks (EVOLVE_WORKTREE_PATH, EVOLVE_REFLECTION_JOURNAL, etc.) get zero predicates.
//
// 1:1 enforcement:
//
//	Task A: predicate=6 (C50A_001–005, C50A_NEG), manual+checklist=2 (AC6/AC7),
//	        pre-existing-GREEN=0, unverifiable-remove=0 → total AC=8 ✓
//	Task B: predicate=7 (C50B_001–006, C50B_NEG), manual+checklist=2 (AC7/AC8),
//	        pre-existing-GREEN=0, unverifiable-remove=0 → total AC=9 ✓
```

### `go/acs/cycle50/predicates_test.go:100` — above `func TestC50A_001_CodexConfigPath_AbsentFromRegistry(t *testing.T) {`

```text
// TestC50A_001_CodexConfigPath_AbsentFromRegistry verifies that
// EVOLVE_CODEX_CONFIG_PATH is no longer registered after the DI migration.
// The single production reader at codex_pretrust.go:142 is replaced by
// cfg.codexConfigPath on the bridge.Config struct; the registry row must be deleted.
//
// Covers Task A AC1. BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT.
// Adding a source comment cannot satisfy this; the registry row must be absent.
//
// RED: EVOLVE_CODEX_CONFIG_PATH is currently registered at registry_table.go:18
// with Status=StatusInternal, Doc="Undocumented production reader (inventory 2026-06-11)".
```

### `go/acs/cycle50/predicates_test.go:176` — above `func TestC50A_005_BridgeTests_NoSetenvCodexConfigPath(t *testing.T) {`

```text
// TestC50A_005_BridgeTests_NoSetenvCodexConfigPath verifies that all 5 bridge test
// files that previously called t.Setenv("EVOLVE_CODEX_CONFIG_PATH", ...) have been
// migrated to set cfg.codexConfigPath instead. This ensures tests exercise the DI
// code path, not the now-removed env read.
//
// Files checked (confirmed package=bridge, same-package access to unexported field):
//   - codex_pretrust_test.go
//   - codex_pretrust_amplify_test.go
//   - codex_pretrust_concurrent_test.go
//   - codex_pretrust_launch_test.go
//   - preflight_test.go
//
// acs-predicate: config-check
//
// RED: all 5 files currently have t.Setenv("EVOLVE_CODEX_CONFIG_PATH", ...) calls.
// Cycle-41 failed by only updating codex_pretrust_test.go (1/5). This cycle
// updates all 5 atomically.
```

### `go/acs/cycle50/predicates_test.go:242` — above `func TestC50B_001_ReleaseStrictPass_AbsentFromRegistry(t *testing.T) {`

```text
// TestC50B_001_ReleaseStrictPass_AbsentFromRegistry verifies that
// EVOLVE_RELEASE_STRICT_PASS is no longer registered after the CLI flag migration.
// Both production readers (cmd_release_preflight.go:55 and releasepipeline/bridges.go:26)
// are replaced by the --strict-pass CLI flag wired through releasepipeline.Options.StrictPass.
//
// Covers Task B AC1. BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT.
//
// RED: EVOLVE_RELEASE_STRICT_PASS is currently registered at registry_table.go:49
// with Status=StatusInternal, Doc="Undocumented production reader (inventory 2026-06-11)".
```

### `go/acs/cycle50/predicates_test.go:264` — above `func TestC50B_002_ReleaseStrictPass_AbsentFromReleasePreflight(t *testing.T) {`

```text
// TestC50B_002_ReleaseStrictPass_AbsentFromReleasePreflight verifies that
// os.Getenv("EVOLVE_RELEASE_STRICT_PASS") has been removed from cmd_release_preflight.go.
// After the migration, the command parses --strict-pass from CLI args instead.
// Precedent: --force-fresh migration (cycle-49 Task A) used the same bucket-4 pattern.
//
// acs-predicate: config-check
//
// RED: cmd_release_preflight.go:55 currently has:
//
//	strictPass := os.Getenv("EVOLVE_RELEASE_STRICT_PASS") == "1"
```

### `go/acs/cycle50/predicates_test.go:321` — above `func TestC50B_005_StrictPassFlag_RegisteredInPreflight(t *testing.T) {`

```text
// TestC50B_005_StrictPassFlag_RegisteredInPreflight verifies that --strict-pass
// is registered as a CLI flag in cmd_release_preflight.go. The bucket-4 pattern
// (transient emergency hatch → explicit CLI flag) follows the --force-fresh
// precedent from cycle-49 and the --skip-tests flag from releasepipeline.
//
// acs-predicate: config-check
//
// RED: cmd_release_preflight.go currently has no --strict-pass flag; the env var
// os.Getenv("EVOLVE_RELEASE_STRICT_PASS") is used directly at line 55.
```
