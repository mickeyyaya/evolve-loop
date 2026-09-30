# Comment history: `acs/cycle22`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle22/predicates_test.go:3` — above `package cycle22`

```text
// Package cycle22 materializes the cycle-22 acceptance criteria for:
//
//	dead-flag-sweep-22 — remove 9 confirmed-dead EVOLVE_* registry rows
//	(EVOLVE_BUILDER_REVIEW_SKILLS, EVOLVE_BUILDER_REVIEW_THRESHOLD,
//	EVOLVE_BUILDER_SELF_REVIEW, EVOLVE_BUILDER_WORKTREE,
//	EVOLVE_PASS_CONFIDENCE_THRESHOLD, EVOLVE_RESEARCH_CACHE_ENABLED,
//	EVOLVE_USE_LEGACY_BASH, EVOLVE_TRIAGE_AUTO_SKIP_TRIVIAL,
//	EVOLVE_TRIAGE_TOP_N),
//	lower FlagCeiling 154→145, regenerate docs/architecture/control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	dead-flag-sweep-22:
//	  AC1       All 9 dead flags absent from Lookup            → C22_001 (behavioral)
//	  AC2       Registry row count == 145                      → C22_002 (behavioral, count)
//	  AC3       FlagCeiling const == 145                       → C22_003 (config-check, waiver)
//	  AC4       No os.Getenv reads for 9 flags in prod Go      → C22_004 (config-check, waiver — PRE-EXISTING GREEN)
//	  AC5       control-flags.md has no dead-flag rows         → C22_005 (config-check, waiver)
//	  AC8       WORKTREE_PATH still in registry                → C22_006 (behavioral — PRE-EXISTING GREEN)
//	  NEG1      runtime-reference.md preserves RESEARCH_CACHE_ENABLED → C22_007 (config-check, waiver — PRE-EXISTING GREEN)
//
// ACs with manual+checklist disposition:
//
//	AC6 (C50_009 still green):   `go test -tags acs ./acs/regression/cycle50/...`
//	AC7 (flagreaders guard):     `go test -tags acs ./acs/regression/flagreaders/...`
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C22_001 — Lookup returns ok=false for all 9 flags; cannot be
//	           satisfied by adding magic strings — the registry row must be absent.
//	Edge/OOD:  EVOLVE_TRIAGE_TOP_N + EVOLVE_TRIAGE_AUTO_SKIP_TRIVIAL have StatusActive
//	           (not StatusInternal) — they appear live but have 0 production readers.
//	Lexical:   Lookup / len() / FileContains / FileNotContains / SubprocessOutput — five distinct verbs.
//	Semantic:  registry-absence, row-count, ceiling-const, env-read-absence,
//	           doc-absence, worktree-path-still-present, runtime-ref-preserved.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (dead-flag-sweep-22). All deferred tasks get zero predicates this cycle.
//
// 1:1 enforcement: predicate=7, manual+checklist=2, unverifiable-remove=0 → total AC=9 ✓
```

### `go/acs/cycle22/predicates_test.go:53` — above `var deadFlags = []string{`

```text
// deadFlags is the canonical list of 9 dead EVOLVE_* flags that cycle-22 removes.
// All have 0 Go production readers and 0 shell readers per scout-report §Key Findings.
```

### `go/acs/cycle22/predicates_test.go:97` — above `func TestC22_004_NoProductionReaderForDeadFlags(t *testing.T) {`

```text
// TestC22_004_NoProductionReaderForDeadFlags verifies that no production Go file
// reads any of the 9 dead flags via os.Getenv.
//
// Scout confirmed 0 production readers before the cycle — this predicate documents
// the architectural contract and prevents re-introduction by cycle-22 or future work.
//
// // acs-predicate: config-check — the os.Getenv ABSENCE is the structural contract.
//
// PRE-EXISTING GREEN: grep confirms 0 production os.Getenv reads before this cycle.
```

### `go/acs/cycle22/predicates_test.go:153` — above `func TestC22_006_WorktreePathStillInRegistry(t *testing.T) {`

```text
// TestC22_006_WorktreePathStillInRegistry verifies that EVOLVE_WORKTREE_PATH
// remains in the registry after the 9-row removal — it is a live IPC handoff
// (agents/evolve-tester.md:96,113) pinned by C50_009.
//
// Covers AC8. Cycles 17 and 18 both failed when builder over-reached and removed
// WORKTREE_PATH, breaking C50_009. This predicate makes that over-reach
// immediately detectable.
//
// BEHAVIORAL: calls flagregistry.Lookup("EVOLVE_WORKTREE_PATH") — the production SSOT.
//
// PRE-EXISTING GREEN: WORKTREE_PATH is currently registered (line 163 of registry_table.go).
```
