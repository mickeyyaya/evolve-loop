# Comment history: `acs/cycle13`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle13/predicates_test.go:3` — above `package cycle13`

```text
// Package cycle13 materializes the cycle-13 acceptance criteria for the
// committed top_n task:
//
//	consolidate-checkpoint-cluster — remove all 3 CHECKPOINT_* registry rows
//	(EVOLVE_CHECKPOINT_DISABLE, EVOLVE_CHECKPOINT_REASON, EVOLVE_CHECKPOINT_REQUEST),
//	lower FlagCeiling 158→155, remove the inert t.Setenv from the test, and
//	remove CHECKPOINT_* rows from control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	consolidate-checkpoint-cluster:
//	  AC1+NEG1  All 3 CHECKPOINT_* flags absent from Lookup             → C13_001 (behavioral)
//	  AC2       Registry row count == 155                                → C13_002 (behavioral, count)
//	  AC3       FlagCeiling const == 155                                 → C13_003 (config-check, waiver)
//	  AC4       No os.Getenv reads for CHECKPOINT_* in production files  → C13_004 (config-check, waiver — PRE-EXISTING GREEN)
//	  AC5       control-flags.md has no CHECKPOINT_* rows               → C13_005 (config-check, waiver)
//
// ACs with manual+checklist disposition (enforced by CI):
//
//	AC6   (full test suite green): `go test ./...` exit 0
//	EDGE1 (TestRunLoop_DeprecatedCostEnvVarsInert passes after t.Setenv removal):
//	      CI: go test ./cmd/evolve/...
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C13_001 — Lookup returns ok=false for all 3 flags; cannot be
//	           satisfied by adding magic strings — the registry row must be absent.
//	Edge/OOD:  CHECKPOINT_REASON and CHECKPOINT_REQUEST (C13_001) — these flags
//	           never had production readers; they're the pure speculative-dead case.
//	           CHECKPOINT_DISABLE (C13_001) — had an inert t.Setenv in a test.
//	Lexical:   Lookup / len() / FileContains / FileNotContains — four distinct verbs.
//	Semantic:  registry-absence, row-count, ceiling-constant, doc-absence — four
//	           distinct behavioral dimensions.
//
// 1:1 enforcement: predicate=5, manual+checklist=2, unverifiable-remove=0,
// pre-existing-GREEN=1 (C13_004) → total AC=7 ✓
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (consolidate-checkpoint-cluster). Deferred tasks (BYPASS_*, RESUME_*, etc.)
// get zero predicates this cycle.
```
