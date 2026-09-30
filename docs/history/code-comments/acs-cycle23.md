# Comment history: `acs/cycle23`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle23/predicates_test.go:3` — above `package cycle23`

```text
// Package cycle23 materializes the cycle-23 acceptance criteria for:
//
//	dead-flag-sweep-23 — remove 5 confirmed-dead EVOLVE_* registry rows
//	(EVOLVE_TASK_MODE, EVOLVE_REQUIRE_TEAM_CONTEXT, EVOLVE_CODEX_REQUIRE_FULL,
//	EVOLVE_RUN_TIMEOUT, EVOLVE_QUOTA_DANGER_PCT),
//	lower FlagCeiling 145→140, clean agent/skill refs,
//	remove ParseQuotaDangerPct function, regenerate docs/architecture/control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	dead-flag-sweep-23:
//	  AC1  All 5 dead flags absent from Lookup            → C23_001 (behavioral)
//	  AC2  Registry row count == 140                      → C23_002 (behavioral, count)
//	  AC3  FlagCeiling const == 140                       → C23_003 (config-check, waiver)
//	  AC4  No os.Getenv reads for 5 flags in prod Go      → C23_004 (config-check, waiver — PRE-EXISTING GREEN)
//	  AC5  control-flags.md has no dead-flag rows         → C23_005 (config-check, waiver)
//	  AC8  ParseQuotaDangerPct removed from helpers.go    → C23_008 (config-check, waiver)
//	  NEG1 WORKTREE_PATH still in registry                → C23_006 (behavioral — PRE-EXISTING GREEN)
//	  NEG2 No removed flags in agents/ or skills/         → C23_007 (config-check, waiver)
//
// ACs with manual+checklist disposition:
//
//	AC6 (flagreaders guard green): `go test -tags acs ./acs/regression/flagreaders/...`
//	AC7 (C50_009 still green):     `go test -tags acs ./acs/regression/cycle50/...`
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C23_001 — Lookup returns ok=false for all 5 flags; a magic-string
//	           patch of source cannot satisfy this — the registry row must be absent.
//	Edge/OOD:  EVOLVE_QUOTA_DANGER_PCT has StatusInternal (differs from the other 4
//	           StatusActive flags) — both status classes must be removed cleanly.
//	Lexical:   Lookup / len(All) / FileContains / FileNotContains / SubprocessOutput —
//	           five distinct verbs across the eight predicates.
//	Semantic:  registry-absence, row-count, ceiling-const, env-read-absence,
//	           doc-absence, worktree-path-present, agents/skills-ref-absence, func-removal.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (dead-flag-sweep-23). Deferred tasks (WORKTREE_PATH, QUOTA_RESET_AT/HOURS,
// per-phase-cli-model cluster, BYPASS/DISPATCH clusters) get zero predicates.
//
// 1:1 enforcement: predicate=8, manual+checklist=2, unverifiable-remove=0 → total AC=10 ✓
```

### `go/acs/cycle23/predicates_test.go:54` — above `var deadFlags = []string{`

```text
// deadFlags is the canonical list of 5 confirmed-dead EVOLVE_* flags that
// cycle-23 removes. All have 0 Go production readers per scout-report §Key Findings.
```

### `go/acs/cycle23/predicates_test.go:73` — above `func TestC23_001_DeadFlagsAbsentFromRegistry(t *testing.T) {`

```text
// TestC23_001_DeadFlagsAbsentFromRegistry verifies that all 5 dead flags are
// no longer registered after Builder removes their rows from registry_table.go.
//
// Covers AC1 (all 5 rows absent). Includes:
//   - EVOLVE_TASK_MODE: StatusActive, 0 production readers (budget_tiers removed PR #96)
//   - EVOLVE_REQUIRE_TEAM_CONTEXT: StatusActive, 0 production readers (phase-gate-precondition.sh deleted)
//   - EVOLVE_CODEX_REQUIRE_FULL: StatusActive, 0 production readers (bridge uses --require-full struct field)
//   - EVOLVE_RUN_TIMEOUT: StatusActive, 0 production readers (budget system removed PR #96)
//   - EVOLVE_QUOTA_DANGER_PCT: StatusInternal (edge: different status class), ParseQuotaDangerPct has no callers
//
// BEHAVIORAL: calls flagregistry.Lookup() for each flag — the production SSOT.
// A source edit alone cannot satisfy this; the registry row must be absent for
// Lookup to return ok=false.
//
// RED: all 5 flags are currently registered; each Lookup returns (flag, true).
```

### `go/acs/cycle23/predicates_test.go:155` — above `func TestC23_006_WorktreePathStillInRegistry(t *testing.T) {`

```text
// TestC23_006_WorktreePathStillInRegistry verifies that EVOLVE_WORKTREE_PATH
// remains in the registry after the 5-row removal — it is a live IPC handoff
// (agents/evolve-tester.md) pinned by C50_009.
//
// Covers NEG1. Cycles 17 and 18 both failed when builder over-reached and removed
// WORKTREE_PATH, breaking C50_009. This predicate makes that over-reach
// immediately detectable.
//
// BEHAVIORAL: calls flagregistry.Lookup("EVOLVE_WORKTREE_PATH") — the production SSOT.
//
// PRE-EXISTING GREEN: WORKTREE_PATH is currently registered.
```
