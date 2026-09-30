# Comment history: `acs/cycle31`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle31/predicates_test.go:3` — above `package cycle31`

```text
// Package cycle31 materializes the cycle-31 acceptance criteria for:
//
//	bypass-config-cluster-31 — migrate 6 EVOLVE_BYPASS_* flags from os.Getenv/
//	envBypass() reads to CLI flags (flag.BoolVar) threaded as struct params;
//	also dead-sweep EVOLVE_SKIP_WORKTREE (zero Go reader post v12). 7 flags total:
//	  - EVOLVE_BYPASS_COMMIT_GATE    → --bypass-commit-gate on evolve ship
//	  - EVOLVE_BYPASS_PHASE_GATE     → --bypass on evolve guard phase
//	  - EVOLVE_BYPASS_POSTEDIT_VALIDATE → --bypass on evolve postedit-validate
//	  - EVOLVE_BYPASS_PREFIX_GATE    → --bypass on evolve commit-prefix-gate
//	  - EVOLVE_BYPASS_ROLE_GATE      → --bypass on evolve guard role
//	  - EVOLVE_BYPASS_SHIP_GATE      → --bypass on evolve guard ship
//	  - EVOLVE_SKIP_WORKTREE         → dead sweep (no Go reader post v12)
//	Lower FlagCeiling 109→102; regenerate docs/architecture/control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	bypass-config-cluster-31:
//	  AC1  7 flags absent from Lookup                → C31_001 (behavioral)
//	  AC2  Registry row count == 102                 → C31_002 (behavioral, count)
//	  AC3  FlagCeiling const == 102                  → C31_003 (config-check, waiver)
//	  AC4  No envBypass/os.Getenv reads for BYPASS   → C31_004 (config-check, waiver)
//	  AC5  Guard constructors accept bypass bool      → C31_005 (behavioral, reflect)
//	  AC6  EVOLVE_WORKTREE_PATH still registered     → C31_006 (behavioral, PRE-EXISTING GREEN)
//	  AC7  flagreaders regression guard green         → manual+checklist (see below)
//	  AC8  control-flags.md has no removed rows      → C31_008 (config-check, waiver)
//	  NEG1 envBypass helper deleted from helpers.go  → C31_NEG1 (config-check, waiver)
//
// ACs with manual+checklist disposition:
//
//	AC7 (flagreaders guard green): `go test -tags acs ./acs/regression/flagreaders/...`
//	    Checklist for Auditor:
//	    (a) no compile errors with -tags acs on the cycle31 package;
//	    (b) exit 0 from `go test -tags acs ./acs/regression/flagreaders/...`;
//	    (c) no literal string "EVOLVE_BYPASS_COMMIT_GATE", "EVOLVE_BYPASS_PHASE_GATE",
//	        "EVOLVE_BYPASS_POSTEDIT_VALIDATE", "EVOLVE_BYPASS_PREFIX_GATE",
//	        "EVOLVE_BYPASS_ROLE_GATE", or "EVOLVE_BYPASS_SHIP_GATE" in any
//	        non-test, non-registry Go file via os.Getenv or envBypass()
//	        (grep -rn 'envBypass\|os\.Getenv.*BYPASS'
//	         go/ --include='*.go'
//	        | grep -v '_test.go' | grep -v 'registry_table.go'
//	        | grep -v 'acs/cycle31' → 0 matches);
//	    (d) EVOLVE_SKIP_WORKTREE is also absent from all production Go
//	        (it had a shell reader in run-cycle.sh removed in v12; zero Go readers).
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C31_001 — 7 flags must be ABSENT from Lookup (if Builder misses
//	           one, Lookup returns ok=true and the test fails immediately).
//	           C31_004 — envBypass and os.Getenv("EVOLVE_BYPASS_*") calls must be
//	           ABSENT from the specific production Go files (if Builder only removes
//	           the registry row without updating the call sites, this fails).
//	           C31_NEG1 — the envBypass() helper function must be ABSENT from
//	           guards/helpers.go (if Builder migrates callers but leaves the dead
//	           helper, this fails).
//	Edge/OOD:  C31_002 checks exact count 102; both over-removal (< 102) and
//	           under-removal (> 102) fail.
//	Lexical:   Lookup / len / FileContains / FileNotContains / reflect —
//	           distinct assertion verbs across the suite.
//	Semantic:  registry-absence, row-count, ceiling-const, no-env-reads,
//	           guard-DI-signature, worktree-path-present, doc-absence,
//	           helper-fn-deleted — 8 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (bypass-config-cluster-31). Deferred tasks (Dynamic Phase Routing, etc.) get
// zero predicates.
//
// 1:1 enforcement: predicate=8, manual+checklist=1, unverifiable-remove=0 → total AC=9 ✓
```

### `go/acs/cycle31/predicates_test.go:82` — above `var bypassFlags = []string{`

```text
// bypassFlags is the canonical list of 7 flags that cycle-31 removes:
//   - EVOLVE_BYPASS_COMMIT_GATE:       migrated to --bypass-commit-gate on evolve ship
//   - EVOLVE_BYPASS_PHASE_GATE:        migrated to --bypass on evolve guard phase
//   - EVOLVE_BYPASS_POSTEDIT_VALIDATE: migrated to --bypass on evolve postedit-validate
//   - EVOLVE_BYPASS_PREFIX_GATE:       migrated to --bypass on evolve commit-prefix-gate
//   - EVOLVE_BYPASS_ROLE_GATE:         migrated to --bypass on evolve guard role
//   - EVOLVE_BYPASS_SHIP_GATE:         migrated to --bypass on evolve guard ship
//   - EVOLVE_SKIP_WORKTREE:            dead sweep (shell reader removed in v12)
```

### `go/acs/cycle31/predicates_test.go:266` — above `func TestC31_006_WorktreePathStillRegistered(t *testing.T) {`

```text
// TestC31_006_WorktreePathStillRegistered is the non-repeat guard: verifies that
// EVOLVE_WORKTREE_PATH was NOT accidentally removed as part of the bypass cluster
// sweep. Cycles 17, 18, and 19 all failed when a Builder removed WORKTREE_PATH —
// this predicate closes that regression surface.
//
// Covers AC6 (FORBIDDEN-REPEAT guard). BEHAVIORAL: calls flagregistry.Lookup —
// the test fails if Builder removes the row (Lookup returns ok=false).
//
// PRE-EXISTING GREEN: WORKTREE_PATH is currently in the registry (StatusInternal);
// this test is GREEN before Builder makes any changes. It stays GREEN only if
// Builder does NOT touch the WORKTREE_PATH row.
```

### `go/acs/cycle31/predicates_test.go:311` — above `func TestC31_NEG1_EnvBypassHelperDeleted(t *testing.T) {`

```text
// TestC31_NEG1_EnvBypassHelperDeleted is the anti-gaming predicate that verifies
// the envBypass() helper function has been completely deleted from guards/helpers.go.
//
// Anti-gaming rationale (cycle-8/cycle-85 lesson): a Builder could migrate all
// three guard callers away from envBypass() while leaving the dead helper function
// in place. C31_004 confirms the BYPASS env-var strings are gone from individual
// guard files; NEG1 adds a second layer by asserting the shared envBypass() helper
// itself is gone — closing the gaming surface where the function stays as dead code.
//
// acs-predicate: config-check
//
// RED: guards/helpers.go:29 defines "func envBypass(name string) bool"
// that reads os.Getenv(name) == "1". After migration, this function must not exist.
```
