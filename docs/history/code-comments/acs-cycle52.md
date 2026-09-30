# Comment history: `acs/cycle52`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle52/predicates_test.go:3` — above `package cycle52`

```text
// Package cycle52 materializes the cycle-52 acceptance criteria for two tasks:
//
//	skip-preflight-cli-52 — remove EVOLVE_SKIP_PREFLIGHT and EVOLVE_SKIP_PREFLIGHT_BOOT
//	  from the flag registry; replace both os.Getenv reads in cmd_loop_preflight.go
//	  with cfg.SkipPreflight / cfg.SkipPreflightBoot fields on loopConfig; add
//	  --skip-preflight / --skip-preflight-boot CLI flags in cmd_loop_args.go;
//	  migrate tests from t.Setenv/os.Setenv to explicit CLI args;
//	  lower FlagCeiling 50→48.
//
//	ship-auto-confirm-split-const-52 — remove EVOLVE_SHIP_AUTO_CONFIRM from registry;
//	  define split-const `const envShipAutoConfirm = "EVOLVE_" + "SHIP_AUTO_CONFIRM"`
//	  with SSOT IPC-protocol comment in verify.go; replace two literal occurrences;
//	  lower FlagCeiling 48→47.
//
// AC map (1:1 with triage top_n for both tasks):
//
//	=== Task A: skip-preflight-cli-52 ===
//	AC1  EVOLVE_SKIP_PREFLIGHT absent from registry           → C52A_001 (behavioral: Lookup)
//	AC2  EVOLVE_SKIP_PREFLIGHT_BOOT absent from registry      → C52A_002 (behavioral: Lookup)
//	AC3  No prod os.Getenv(SKIP_PREFLIGHT) in preflight.go   → C52A_003 (config-check, waiver)
//	AC4  No prod os.Getenv(SKIP_PREFLIGHT_BOOT) in preflight → C52A_004 (config-check, waiver)
//	AC5  loopConfig has SkipPreflight bool field              → C52A_005 (config-check, waiver, FileMatchesRegex)
//	AC6  loopConfig has SkipPreflightBoot bool field          → C52A_006 (config-check, waiver, FileMatchesRegex)
//	AC7  FlagCeiling == 48 (intermediate after Task A)        → C52A_007 (config-check, waiver)
//	AC8  No t.Setenv/os.Setenv for either flag in cmd/evolve  → C52A_008 (config-check, waiver)
//	     tests (manual+checklist: cmd/evolve suite green)     → manual+checklist (Auditor)
//	     flagreaders ACS guard green                          → manual+checklist (Auditor)
//	NEG  row count ≤ 48 after Task A (allows B to reach 47)  → C52A_NEG (behavioral: len)
//
//	=== Task B: ship-auto-confirm-split-const-52 ===
//	AC1  EVOLVE_SHIP_AUTO_CONFIRM absent from registry        → C52B_001 (behavioral: Lookup)
//	AC2  split-const envShipAutoConfirm in verify.go          → C52B_002 (config-check, waiver)
//	AC3  no bare literal in verify.go prod reader             → C52B_003 (config-check, waiver)
//	AC4  FlagCeiling == 47 (final)                            → C52B_004 (config-check, waiver)
//	NEG  exact row count == 47                                → C52B_NEG (behavioral: len)
//
// Manual+checklist ACs (addressed to Auditor):
//
//	Task A (cmd/evolve test suite green):
//	  (a) exit 0: cd go && go test ./cmd/evolve/... -count=1
//	  (b) no FAIL packages in output
//	  (c) runLoop calls that relied on TestMain global os.Setenv pass --skip-preflight explicitly
//	  (d) cmd_loop_preflight_test.go tests use cfg.SkipPreflight=true not env override
//
//	Task A (flagreaders ACS guard green):
//	  (a) exit 0: go test -tags acs ./acs/regression/flagreaders/... -count=1
//	  (b) EVOLVE_SKIP_PREFLIGHT absent from non-test, non-registry Go prod files:
//	      grep -rn '"EVOLVE_SKIP_PREFLIGHT"' go/ --include='*.go' | grep -v '_test.go' | grep -v 'registry_table.go' → 0 matches
//	  (c) EVOLVE_SKIP_PREFLIGHT_BOOT absent from same set → 0 matches
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C52A_001/002/C52B_001 — flags ABSENT from Lookup (any hit = still registered).
//	            C52A_NEG: row count ≤ 48 (upper bound; prevents Task B from failing A's check).
//	            C52B_NEG: exact count == 47 (catches over-removal <47 AND under-removal >47).
//	Edge/OOD:   C52B_NEG exact count rejects both directions.
//	            C52A_005/006 FileMatchesRegex with \s+ tolerates gofmt column-alignment (cycle-51 lesson).
//	Lexical:    Lookup / len / FileNotContains / FileMatchesRegex / FileContains — 5 distinct verbs.
//	Semantic:   registry-absence (3 flags), env-read-clean (1 file × 2 vars), struct-fields (2),
//	            test-setenv-clean (3 test files), ceiling-const (2 values: 48/47),
//	            split-const-present (1), bare-literal-absent (1), exact-row-count (1) — 9 dimensions.
//
// Floor binding (R9.3): predicates authored ONLY for tasks in the triage top_n.
// Deferred tasks (EVOLVE_WORKTREE_PATH, EVOLVE_STRICT_AUDIT, etc.) get zero predicates.
//
// Cycle-51 lesson applied: struct-field assertions use FileMatchesRegex with \s+ (not
// FileContains with exact single space). gofmt column-aligns struct fields with multiple
// spaces when a longer sibling field is present; FileContains("SkipPreflight bool") fails
// when gofmt produces "SkipPreflight     bool". FileMatchesRegex with `SkipPreflight\s+bool`
// tolerates any whitespace count.
//
// 1:1 enforcement:
//
//	Task A: predicate=7 (C52A_001–008, C52A_NEG), manual+checklist=2 (suite/flagreaders),
//	        pre-existing-GREEN=0, unverifiable-remove=0 → total AC=9 ✓
//	Task B: predicate=4 (C52B_001–004, C52B_NEG), manual+checklist=0,
//	        pre-existing-GREEN=0, unverifiable-remove=0 → total AC=5 ✓
```

### `go/acs/cycle52/predicates_test.go:183` — above `func TestC52A_005_LoopConfig_HasSkipPreflightField(t *testing.T) {`

```text
// === loopConfig struct fields (config-check waiver — FileMatchesRegex, cycle-51 lesson) ===
```

### `go/acs/cycle52/predicates_test.go:185` — above `func TestC52A_005_LoopConfig_HasSkipPreflightField(t *testing.T) {`

```text
// TestC52A_005_LoopConfig_HasSkipPreflightField verifies that loopConfig (defined in
// cmd_loop.go) has a SkipPreflight bool field. This is the DI seam replacing the
// os.Getenv read.
//
// IMPORTANT (cycle-51 lesson): uses FileMatchesRegex with `SkipPreflight\s+bool` —
// NOT FileContains with a single-space string. When SkipPreflightBoot is a sibling
// field with a longer name, gofmt column-aligns "SkipPreflight" with 5+ spaces before
// "bool". FileContains("SkipPreflight bool") would fail on gofmt-formatted output.
//
// acs-predicate: config-check
//
// RED: cmd_loop.go currently has ForceFresh bool as the last bool field;
// SkipPreflight bool is not present.
```

### `go/acs/cycle52/predicates_test.go:213` — above `func TestC52A_006_LoopConfig_HasSkipPreflightBootField(t *testing.T) {`

```text
// TestC52A_006_LoopConfig_HasSkipPreflightBootField verifies that loopConfig has a
// SkipPreflightBoot bool field — the DI seam replacing the EVOLVE_SKIP_PREFLIGHT_BOOT
// os.Getenv read at cmd_loop_preflight.go:27.
//
// IMPORTANT (cycle-51 lesson): uses FileMatchesRegex with `SkipPreflightBoot\s+bool`.
// SkipPreflightBoot is the longer sibling that CAUSES gofmt to pad SkipPreflight.
// The regex handles any column-alignment gofmt produces.
//
// acs-predicate: config-check
//
// RED: cmd_loop.go does not have a SkipPreflightBoot bool field.
```

### `go/acs/cycle52/predicates_test.go:329` — above `func TestC52B_002_VerifyGo_HasSplitConst(t *testing.T) {`

```text
// TestC52B_002_VerifyGo_HasSplitConst verifies that verify.go defines the split-const
// `const envShipAutoConfirm = "EVOLVE_" + "SHIP_AUTO_CONFIRM"` with the IPC-protocol
// comment. The split-const pattern (bucket-5) makes the flag invisible to the
// flagreaders guard while preserving the IPC handoff contract between
// releasepipeline/rollback (setters) and ship/verify.go (reader).
//
// Precedent: cycle-49 EVOLVE_LANE split-const (identical pattern).
//
// acs-predicate: config-check
//
// RED: verify.go currently has the bare literal "EVOLVE_SHIP_AUTO_CONFIRM" at :239.
// No split-const is defined.
```
