# Comment history: `acs/cycle45`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle45/predicates_test.go:3` — above `package cycle45`

```text
// Package cycle45 materializes the cycle-45 acceptance criteria for two tasks:
//
//	Task 1: gobin-planworkspace-di-45 — remove 2 flags from the operator registry:
//	  EVOLVE_GO_BIN_TEST    → Bucket 3 (DI): goBinFn func() string param injected into
//	                         defaultSimulationRunner; removes os.Getenv("EVOLVE_GO_BIN_TEST")
//	  EVOLVE_PLAN_WORKSPACE → Bucket 6 (CLI flag): --workspace flag on plan-and-execute;
//	                         removes os.Getenv("EVOLVE_PLAN_WORKSPACE")
//	Lower FlagCeiling 65 → 63.
//
//	Task 2: codex-version-compact-di-45 — remove 2 more flags:
//	  EVOLVE_CODEX_VERSION_PATH → Bucket 3 (DI via pkg-level var): codexVersionPathFn;
//	                             removes os.Getenv("EVOLVE_CODEX_VERSION_PATH")
//	  EVOLVE_COMPACT_PROMPTS    → Bucket 1 (Config Object): Options.CompactPrompts bool;
//	                             removes envchain.Bool("EVOLVE_COMPACT_PROMPTS", req.Env, false)
//	Lower FlagCeiling 63 → 61.
//
// AC map (1:1 with triage top_n):
//
//	Task 1 (gobin-planworkspace-di-45):
//	 AC1  EVOLVE_GO_BIN_TEST absent from registry         → C45_001 (behavioral: Lookup)
//	 AC2  EVOLVE_PLAN_WORKSPACE absent from registry      → C45_002 (behavioral: Lookup)
//	 AC3  No prod Getenv for EVOLVE_GO_BIN_TEST           → C45_003 (config-check, waiver)
//	 AC4  No prod Getenv for EVOLVE_PLAN_WORKSPACE        → C45_004 (config-check, waiver)
//	 AC5  PLAN_WORKSPACE removed from allowedUndocumented → C45_005 (config-check, waiver)
//	 AC6  go test ./internal/releasepreflight/... passes  → manual+checklist (Auditor)
//
//	Task 2 (codex-version-compact-di-45):
//	 AC1  EVOLVE_CODEX_VERSION_PATH absent from registry  → C45B_001 (behavioral: Lookup)
//	 AC2  EVOLVE_COMPACT_PROMPTS absent from registry     → C45B_002 (behavioral: Lookup)
//	 AC3  No prod Getenv for EVOLVE_CODEX_VERSION_PATH    → C45B_003 (config-check, waiver)
//	 AC4  No envchain.Bool for EVOLVE_COMPACT_PROMPTS     → C45B_004 (config-check, waiver)
//	 AC5  FlagCeiling == 61                               → C45_006 (config-check, waiver)
//	 AC6  go test ./internal/bridge/... ./internal/phases/runner/... → manual+checklist
//	 AC7  control-flags.md clean (4 flags absent)         → C45_007 (config-check, waiver)
//	 NEG  Exact registry count == 61                      → C45_NEG (behavioral: len)
//
// ACs with manual+checklist disposition:
//
//	AC6-Task1 (releasepreflight suite passes):
//	  Checklist for Auditor:
//	  (a) exit 0 from `cd go && go test ./internal/releasepreflight/... ./cmd/evolve/... ./internal/flagregistry/...`
//	  (b) no FAIL packages in output
//	  (c) `go build ./...` exits 0 from go/
//
//	AC6-Task2 (bridge + runner suite passes):
//	  Checklist for Auditor:
//	  (a) exit 0 from `cd go && go test ./internal/bridge/... ./internal/phases/runner/... ./internal/flagregistry/...`
//	  (b) no FAIL packages in output
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C45_001/002/C45B_001/002 — all 4 flags ABSENT from Lookup (any hit = flag still registered).
//	            C45_NEG_ExactRowCountIs61 — registry EXACTLY 61; over- or under-removal fails.
//	Edge/OOD:   C45_NEG_ExactRowCountIs61 catches both <61 (over-removal) and >61 (under-removal).
//	Lexical:    Lookup / len / FileNotContains / FileContains — distinct assertion verbs.
//	Semantic:   registry-absence (4 flags), exact-row-count, prod-source-clean (4 files),
//	            docs-contract-hygiene, ceiling-const, control-flags-doc — 6 distinct dimensions.
//
// Floor binding (R9.3): predicates authored ONLY for gobin-planworkspace-di-45 and
// codex-version-compact-di-45 (both in triage top_n). Deferred tasks get zero predicates.
//
// 1:1 enforcement:
//
//	predicate count: 11 funcs (C45_001-007, C45B_001-004, C45_NEG)
//	manual+checklist: 2 (AC6-Task1, AC6-Task2 — checklist addressed to Auditor above)
//	unverifiable-remove: 0
//	Total AC count: 13; every AC has exactly one disposition row.
```

### `go/acs/cycle45/predicates_test.go:80` — above `var allRemovedFlags = []string{`

```text
// allRemovedFlags is the canonical list of 4 env flags removed in cycle 45.
```
