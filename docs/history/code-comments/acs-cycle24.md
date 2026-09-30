# Comment history: `acs/cycle24`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle24/predicates_test.go:3` — above `package cycle24`

```text
// Package cycle24 materializes the cycle-24 acceptance criteria for:
//
//	per-phase-cli-model-profiles — remove 5 per-phase agent config flags
//	(EVOLVE_AUDITOR_CLI, EVOLVE_TDD_ENGINEER_CLI, EVOLVE_TDD_ENGINEER_MODEL,
//	EVOLVE_BUILD_PERMISSION_MODE, EVOLVE_TDD_ENGINEER_PERMISSION_MODE)
//	by deleting os.Getenv tier-2 reads and consolidating to Profile SSOT.
//	Lower FlagCeiling 140→135, regenerate docs/architecture/control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	per-phase-cli-model-profiles:
//	  AC1  5 flags absent from Lookup             → C24_001 (behavioral)
//	  AC2  Registry row count == 135              → C24_002 (behavioral, count)
//	  AC3  FlagCeiling const == 135               → C24_003 (config-check, waiver)
//	  AC4  No prod readers for removed flags      → C24_004 (config-check, waiver)
//	  AC5  control-flags.md has no removed rows   → C24_005 (config-check, waiver)
//	  AC6  WORKTREE_PATH still in registry        → C24_006 (behavioral — PRE-EXISTING GREEN)
//	  AC8  llmroute skips os.Getenv for CLI+MODEL → C24_008 (behavioral)
//	  NEG1 profile.CLI honored (preserved)        → C24_NEG1 (behavioral — PRE-EXISTING GREEN)
//	  NEG2 runtime-reference.md no removed rows   → C24_NEG2 (config-check, waiver — PRE-EXISTING GREEN)
//
// ACs with manual+checklist disposition:
//
//	AC7 (flagreaders guard green): `go test -tags acs ./acs/regression/flagreaders/...`
//	    Checklist for Auditor: (a) no compile errors with -tags acs; (b) exit 0 from the
//	    flagreaders suite; (c) no stale EVOLVE_* references to the 5 removed flags in
//	    the flagreaders scan results.
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C24_008 — sets EVOLVE_AUDITOR_CLI and EVOLVE_TDD_ENGINEER_MODEL in the OS env
//	           (via t.Setenv), calls llmroute.Resolve with empty reqEnv, and asserts the
//	           sentinel OS values are NOT picked up. This is the strongest anti-no-op signal:
//	           the only way to satisfy it is to remove the os.Getenv tier-2 from resolvePrimary
//	           and resolveModel — a magic-string source edit cannot satisfy it.
//	Edge/OOD:  C24_001 tests ALL 5 flags (mix of StatusInternal status; all must be absent).
//	           C24_004 covers both runner.go (PERMISSION_MODE) and observer (phaseCLI) call sites.
//	Lexical:   Lookup / len / FileContains / FileNotContains / CountInGoFunc /
//	           t.Setenv / llmroute.Resolve — seven distinct verbs.
//	Semantic:  registry-absence, row-count, ceiling-const, structural-reader-absence,
//	           doc-absence, worktree-path-preserved, llmroute-env-bypass,
//	           profile-honored, runtime-reference-absence — 9 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (per-phase-cli-model-profiles). Deferred tasks (INTERACTIVE_POLICY flags,
// ROUTER_CLI/MODEL, Workflow Defaults cluster) get zero predicates.
//
// 1:1 enforcement: predicate=9, manual+checklist=1, unverifiable-remove=0 → total AC=10 ✓
```

### `go/acs/cycle24/predicates_test.go:63` — above `var removedFlags = []string{`

```text
// removedFlags is the canonical list of 5 per-phase agent config flags that
// cycle-24 removes by migrating from os.Getenv tier-2 to Profile SSOT tier-3.
// Covers two reader surfaces: llmroute (CLI+MODEL) and runner.go (PERMISSION_MODE).
```

### `go/acs/cycle24/predicates_test.go:166` — above `func TestC24_006_WorktreePathStillInRegistry(t *testing.T) {`

```text
// TestC24_006_WorktreePathStillInRegistry verifies that EVOLVE_WORKTREE_PATH
// remains in the registry after the 5-row removal — it is a live IPC handoff
// (agents/evolve-tester.md) pinned by C50_009.
//
// Covers AC6 (WORKTREE_PATH must not be touched). Cycles 17 and 18 both failed
// when builder over-reached and removed WORKTREE_PATH, breaking C50_009.
//
// BEHAVIORAL: calls flagregistry.Lookup("EVOLVE_WORKTREE_PATH") — the production SSOT.
//
// PRE-EXISTING GREEN: WORKTREE_PATH is currently registered.
```
