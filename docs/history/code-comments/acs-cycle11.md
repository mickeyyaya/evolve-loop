# Comment history: `acs/cycle11`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle11/predicates_test.go:3` — above `package cycle11`

```text
// Package cycle11 materializes the cycle-11 acceptance criteria for the
// committed top_n task:
//
//	consolidate-observer-inactivity-cluster — remove all 13 OBSERVER_*/INACTIVITY_*
//	registry rows; add ObserverPolicy struct to policy.go; update 3 production
//	read sites (cmd_phase_observer.go, cmd_phase_watchdog.go, cmd_cycle.go);
//	lower FlagCeiling 176→163.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	consolidate-observer-inactivity-cluster:
//	  AC1+NEG1+NEG2 All 13 OBSERVER_*/INACTIVITY_* flags absent from Lookup → C11_001 (behavioral)
//	  AC3           Registry row count == 163                                 → C11_002 (behavioral, count)
//	  AC2           FlagCeiling const == 163                                  → C11_003 (config-check, waiver)
//	  AC4(a)        cmd_phase_observer.go OBSERVER_* env reads gone           → C11_004 (config-check, waiver)
//	  AC4(b)        cmd_phase_watchdog.go INACTIVITY_* env reads gone         → C11_005 (config-check, waiver)
//	  AC4(c)        cmd_cycle.go OBSERVER_AUTOSPAWN os.Getenv gone            → C11_006 (config-check, waiver)
//	  AC9           ObserverPolicy struct present in internal/policy/policy.go → C11_007 (config-check, waiver)
//	  EDGE1         control-flags.md has no OBSERVER_*/INACTIVITY_* rows      → C11_008 (config-check, waiver)
//	  EDGE3         docs_contract_test.go INACTIVITY_*/OBSERVER_EOF_GRACE_S
//	                removed from allowedUndocumented                          → C11_009 (config-check, waiver)
//
// ACs with manual+checklist disposition (enforced by CI, no cycle predicate needed):
//
//	AC5  (flagregistry tests pass): TestAll_SortedByName + TestRegistry_FlagCeiling in CI
//	AC6  (full suite 0 FAIL): CI pipeline
//	AC7  (flagreaders guard passes): CI acs lane — go test -tags acs ./acs/regression/flagreaders/...
//	AC10 (registry sorted): TestAll_SortedByName in normal CI run
//
// ACs removed:
//
//	AC8  (ACS cycle11 predicates pass): self-referential — unverifiable-remove
//	EDGE2 (.apicover-enforce has cycle11): pre-existing GREEN (TDD adds ./acs/cycle11/ during RED phase)
//
// Adversarial diversity (SKILL §6):
//
//	Negative:    C11_001 — Lookup returns ok=false for all 13 flags; cannot satisfy
//	             by adding magic strings — the registry row must be absent.
//	Edge/OOD:    OBSERVER_ENABLED + OBSERVER_ENFORCE are in C11_001 — they were dead
//	             (0 production reads); their registry absence is the OOD case (pure
//	             delete, no code migration).
//	Lexical:     Lookup / len() / FileNotContains / FileContains — four distinct verbs.
//	Semantic:    registry count, flag-absence, env-reads-deleted, struct-added,
//	             docs-updated — five distinct behavioral checks.
//
// 1:1 enforcement: predicate=9, manual+checklist=4, unverifiable-remove=1,
// pre-existing-GREEN=1 → total AC=15 ✓
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (consolidate-observer-inactivity-cluster). Deferred tasks (BRIDGE_* cluster,
// CHECKPOINT_*, etc.) get zero predicates this cycle.
```
