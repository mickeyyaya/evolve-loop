# Comment history: `acs/cycle43`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle43/predicates_test.go:3` — above `package cycle43`

```text
// Package cycle43 materializes the cycle-43 acceptance criteria for one task:
//
//	bridge-timing-psmas-config-43 — migrate 5 env-read flags to typed
//	BridgePolicy / bridge.Deps / WorkflowPolicy fields:
//	  EVOLVE_SCROLLBACK_LINES     → BridgePolicy.ScrollbackLines  / bridge.Deps.ScrollbackLines
//	  EVOLVE_BOOT_TIMEOUT_S       → BridgePolicy.BootTimeoutS     / bridge.Deps.BootTimeoutS
//	  EVOLVE_ARTIFACT_TIMEOUT_S   → BridgePolicy.ArtifactTimeoutS / bridge.Deps.ArtifactTimeoutS
//	  EVOLVE_ARTIFACT_MAX_EXTENDS → BridgePolicy.ArtifactMaxExtends / bridge.Deps.ArtifactMaxExtends
//	  EVOLVE_PSMAS_SKIP           → WorkflowPolicy.PSMASEnabled   / WorkflowConfig.PSMASEnabled
//	Remove 5 rows from registry_table.go; lower FlagCeiling 73→68.
//	Migrate ALL 8 bridge test files (cycle-42 failure lesson: incomplete migration).
//	Remove EVOLVE_BOOT_TIMEOUT_S CI override from .github/workflows/go.yml.
//
// AC map (1:1 with triage top_n):
//
//	AC1  5 flags absent from Lookup                  → C43_001 (behavioral)
//	AC2  No prod env reads for 5 flags               → C43_002 (config-check, waiver)
//	AC3  BridgePolicy has 4 int timing fields        → C43_003 (behavioral, compile-fail RED)
//	AC4  WorkflowPolicy has PSMASEnabled *bool       → C43_004 (behavioral, compile-fail RED)
//	AC5  bridge.Deps has 4 typed int fields          → C43_005 (behavioral, compile-fail RED)
//	AC6  bridge tests pass                           → manual+checklist (see below)
//	AC7  Full suite passes                           → manual+checklist (see below)
//	AC8  FlagCeiling == 68                           → C43_008 (config-check, waiver)
//	AC9  flagreaders guard passes                    → manual+checklist (see below)
//	AC10 control-flags.md updated                   → C43_010 (config-check, waiver)
//	AC11 CI workflow clean                           → C43_011 (config-check, waiver)
//	AC12 All bridge test files migrated              → C43_012 (config-check, waiver)
//	NEG  Registry count is exactly 68               → C43_NEG_RowCount (behavioral)
//	NEG  PSMAS_SKIP absent from cyclerun*.go        → C43_NEG_PSMASAbsent (config-check, waiver)
//
// ACs with manual+checklist disposition:
//
//	AC6 (bridge tests pass):
//	  Checklist for Auditor:
//	  (a) exit 0 from `cd go && go test -count=1 ./internal/bridge/...`
//	  (b) zero FAIL lines in the output
//	  (c) run with -v and verify tests for each of the 8 migrated test files all pass
//
//	AC7 (full suite passes):
//	  Checklist for Auditor:
//	  (a) exit 0 from `cd go && go test -count=1 ./...`
//	  (b) no FAIL packages in output
//	  (c) `go build ./...` exits 0
//
//	AC9 (flagreaders guard passes):
//	  Checklist for Auditor:
//	  (a) exit 0 from `go test -tags acs ./acs/regression/flagreaders/...`
//	  (b) none of the 5 flag names appear in non-test, non-registry Go files:
//	      `grep -rn '"EVOLVE_SCROLLBACK_LINES"\|"EVOLVE_BOOT_TIMEOUT_S"\|"EVOLVE_ARTIFACT_TIMEOUT_S"\|"EVOLVE_ARTIFACT_MAX_EXTENDS"\|"EVOLVE_PSMAS_SKIP"'
//	       go/ --include='*.go' | grep -v '_test.go' | grep -v 'registry_table.go'` → 0 matches
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C43_001 — 5 flags must be ABSENT from Lookup (any hit = flag still registered).
//	            C43_NEG_RowCount — registry must be EXACTLY 68; over- or under-removal fails.
//	            C43_NEG_PSMASAbsent — "EVOLVE_PSMAS_SKIP" must be ABSENT from cyclerun*.go.
//	Edge/OOD:   C43_NEG_RowCount checks exact 68: over-removal (<68) and under-removal (>68) both fail.
//	Lexical:    Lookup / len / FileNotContains / FileContains / struct-field-access / PSMASEnabled
//	            resolver / BridgePolicy / WorkflowPolicy / bridge.Deps composite literals — distinct verbs.
//	Semantic:   registry-absence (5 flags), exact-row-count (anti-both-directions), no-env-reads
//	            (multi-file anti-gaming), struct-field-existence (3 new API surfaces), no-doc-entries,
//	            ceiling-const, ci-yml-absent, test-file-migration, psmas-prod-absent — 10 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for bridge-timing-psmas-config-43
// (sole top_n task). Deferred tasks (CODEX_CONFIG_PATH, EVOLVE_STRICT_AUDIT,
// StatusInternal cluster) get zero predicates.
//
// 1:1 enforcement:
//
//	predicate=10 (C43_001,002,003,004,005,008,010,011,012,NEG_RowCount,NEG_PSMASAbsent — 11 funcs)
//	manual+checklist=3 (AC6,AC7,AC9)
//	unverifiable-remove=0
//	total AC count=12 + 2 NEG = 14 disposition rows; every AC has exactly one row.
```

### `go/acs/cycle43/predicates_test.go:88` — above `var removedFlags = []string{`

```text
// removedFlags is the canonical list of 5 env flags that cycle-43 removes
// from the registry and from all production env readers.
```

### `go/acs/cycle43/predicates_test.go:119` — above `func TestC43_002_NoProdEnvReadsForRemovedFlags(t *testing.T) {`

```text
// TestC43_002_NoProdEnvReadsForRemovedFlags verifies that the 5 flag name string
// literals have been deleted from all production source files (non-test, non-registry).
//
// Covers AC2 (anti-gaming, cycle-8 split-const lesson): removing registry rows without
// deleting the env reads is the split-const hiding pattern. The prod readers are:
//   - driver_tmux_repl.go: EVOLVE_SCROLLBACK_LINES, EVOLVE_BOOT_TIMEOUT_S, EVOLVE_ARTIFACT_TIMEOUT_S
//   - recipe_adapter.go: EVOLVE_BOOT_TIMEOUT_S
//   - engine.go: EVOLVE_ARTIFACT_MAX_EXTENDS
//   - cmd_phase_observer.go: EVOLVE_ARTIFACT_MAX_EXTENDS
//   - core/cyclerun.go, core/cyclerun_record.go, core/cyclerun_select.go: EVOLVE_PSMAS_SKIP
//
// acs-predicate: config-check
//
// RED: all 5 flag string literals currently appear in the above prod files.
```

### `go/acs/cycle43/predicates_test.go:389` — above `func TestC43_012_AllBridgeTestFilesMigrated(t *testing.T) {`

```text
// TestC43_012_AllBridgeTestFilesMigrated verifies that none of the 8 bridge test
// files listed in scout-report Finding 3 still reference the 5 env flag names
// via Deps.Env map or LookupEnv map arguments.
//
// Covers AC12 (critical — this is the cycle-42 failure point: 3 test files were left
// unmigrated, leaving bridge tests RED after the production env reads were removed).
//
// acs-predicate: config-check
//
// RED: 8 bridge test files currently use EVOLVE_ARTIFACT_TIMEOUT_S / EVOLVE_BOOT_TIMEOUT_S
// / EVOLVE_SCROLLBACK_LINES / EVOLVE_ARTIFACT_MAX_EXTENDS via Deps.Env map injection.
// After migration, each test must use typed Deps fields directly (Deps{ArtifactTimeoutS: N}).
```

### `go/acs/cycle43/predicates_test.go:474` — above `func TestC43_NEG_PSMASAbsentFromCyclerunProdFiles(t *testing.T) {`

```text
// TestC43_NEG_PSMASAbsentFromCyclerunProdFiles verifies that the
// "EVOLVE_PSMAS_SKIP" string literal has been deleted from all 3 production
// cyclerun*.go files in core.
//
// Covers NEG_PSMASAbsent. Anti-gaming (cycle-8 split-const lesson): Builder cannot
// remove the registry row while leaving envchain.BoolValue(cr.envSnap["EVOLVE_PSMAS_SKIP"], false)
// call sites. All 3 prod files must replace envchain reads with pol.WorkflowConfig().PSMASEnabled.
//
// acs-predicate: config-check
//
// RED: cyclerun.go:384, cyclerun_record.go:89, cyclerun_select.go:87 all contain
// envchain.BoolValue(cr.envSnap["EVOLVE_PSMAS_SKIP"], false) — 3 occurrences.
```
