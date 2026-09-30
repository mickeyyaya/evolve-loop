# Comment history: `acs/cycle48`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle48/predicates_test.go:3` — above `package cycle48`

```text
// Package cycle48 materializes the cycle-48 acceptance criteria for two tasks:
//
//	cache-prefix-v2-dead-field-48 — remove dead field EVOLVE_CACHE_PREFIX_V2:
//	  RunRequest.CachePrefixV2 is never read in Run(); env read at cmd_subagent.go:507
//	  has zero runtime effect. Pure no-op removal: build-time struct literal checks
//	  catch any missed test update before test execution.
//	  Lower FlagCeiling 56 → 55.
//
//	guards-log-di-48 — DI migration for EVOLVE_GUARDS_LOG:
//	  change appendGuardsLog(evolveDir, ...) to appendGuardsLog(logPath, ...)
//	  and compute logPath at the call site (cmd_guard.go:122).
//	  Lower FlagCeiling 55 → 54.
//
// AC map (1:1 with triage top_n for both tasks):
//
//	=== Task A: cache-prefix-v2-dead-field-48 ===
//	AC1  EVOLVE_CACHE_PREFIX_V2 absent from registry        → C48A_001 (behavioral: Lookup)
//	AC2  No prod env read for CACHE_PREFIX_V2               → C48A_002 (config-check, waiver)
//	AC3  CachePrefixV2 field absent from run.go             → C48A_003 (config-check, waiver)
//	AC4  FlagCeiling == 55                                  → C48A_004 (config-check, waiver)
//	AC5  cmd_subagent_env_test.go zero CACHE_PREFIX_V2 refs → C48A_005 (config-check, waiver)
//	AC6  go test ./internal/subagent/... PASS               → manual+checklist (Auditor)
//	AC7  go test ./cmd/evolve/... PASS                      → manual+checklist (Auditor)
//	AC8  go test ./internal/flagregistry/... PASS           → manual+checklist (Auditor)
//	AC9  flagreaders ACS guard PASS                         → manual+checklist (Auditor)
//	NEG  row count ≤ 55 after Task A flags removed          → C48A_NEG (behavioral: len)
//
//	=== Task B: guards-log-di-48 ===
//	AC1  EVOLVE_GUARDS_LOG absent from registry             → C48B_001 (behavioral: Lookup)
//	AC2  No prod os.Getenv read for GUARDS_LOG              → C48B_002 (config-check, waiver)
//	AC3  appendGuardsLog first param is logPath string      → C48B_003 (config-check, waiver)
//	AC4  Zero t.Setenv("EVOLVE_GUARDS_LOG") in test files   → C48B_004 (config-check, waiver)
//	AC5  FlagCeiling == 54                                  → C48B_005 (config-check, waiver)
//	AC6  docs_contract_test.go zero GUARDS_LOG refs         → C48B_006 (config-check, waiver)
//	AC7  go test ./cmd/evolve/... PASS                      → manual+checklist (Auditor)
//	AC8  go test ./internal/flagregistry/... PASS           → manual+checklist (Auditor)
//	AC9  flagreaders ACS guard PASS                         → manual+checklist (Auditor)
//	NEG  exact row count == 54 (final state after both)     → C48B_NEG (behavioral: len)
//
// Manual+checklist ACs (addressed to Auditor):
//
//	Task A AC6 (subagent tests pass):
//	  (a) exit 0: cd go && go test ./internal/subagent/...
//	  (b) no FAIL packages in output
//
//	Task A AC7 (cmd/evolve tests pass):
//	  (a) exit 0: cd go && go test ./cmd/evolve/...
//	  (b) no FAIL packages in output
//
//	Task A AC8 (flagregistry tests pass):
//	  (a) exit 0: cd go && go test ./internal/flagregistry/...
//	  (b) TestRegistry_FlagCeiling passes (FlagCeiling == 55 after Task A, == 54 after Task B)
//
//	Task A AC9 (flagreaders ACS guard):
//	  (a) go test -tags acs ./acs/regression/flagreaders/...
//	  (b) EVOLVE_CACHE_PREFIX_V2 does not appear as an orphan reader
//
//	Task B AC7 (cmd/evolve tests pass):
//	  (a) exit 0: cd go && go test ./cmd/evolve/...
//	  (b) no FAIL packages; TestAppendGuardsLog_* pass with DI path injection
//
//	Task B AC8 (flagregistry tests pass):
//	  (a) exit 0: cd go && go test ./internal/flagregistry/...
//	  (b) TestRegistry_FlagCeiling passes (FlagCeiling == 54)
//
//	Task B AC9 (flagreaders ACS guard):
//	  (a) go test -tags acs ./acs/regression/flagreaders/...
//	  (b) EVOLVE_GUARDS_LOG does not appear as an orphan reader
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C48A_001/C48B_001 — flags ABSENT from Lookup (any hit = still registered).
//	            C48A_NEG: row count ≤ 55 (upper bound, allows Task B to apply in same build).
//	            C48B_NEG: exact count == 54 (catches both over-removal <54 and under-removal >54).
//	Edge/OOD:   C48B_NEG exact count rejects both directions; C48A_NEG is one-sided upper bound.
//	Lexical:    Lookup / len / FileNotContains / FileContains / FileMatchesRegex — five distinct verbs.
//	Semantic:   registry-absence (2 flags), env-read-clean (2 files), struct-field-absent (run.go),
//	            DI-signature (cmd_guard.go), test-clean (2 test files), ceiling-const (2 values),
//	            docs-contract-clean (docs_contract_test.go), exact-row-count — 10 dimensions.
//
// Floor binding (R9.3): predicates authored ONLY for tasks in the triage top_n.
// Deferred tasks (EVOLVE_MODELCATALOG_AUTOREFRESH, EVOLVE_FORCE_FRESH, etc.) get zero predicates.
//
// 1:1 enforcement:
//
//	Task A: predicate=6 (C48A_001–005, C48A_NEG), manual+checklist=4 (AC6/AC7/AC8/AC9),
//	        unverifiable-remove=0 → total AC=10 ✓
//	Task B: predicate=7 (C48B_001–006, C48B_NEG), manual+checklist=3 (AC7/AC8/AC9),
//	        unverifiable-remove=0 → total AC=10 ✓
```
