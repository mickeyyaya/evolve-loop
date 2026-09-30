# Comment history: `acs/cycle34`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle34/predicates_test.go:3` — above `package cycle34`

```text
// Package cycle34 materializes the cycle-34 acceptance criteria for:
//
//	gates-config-cluster-34 — migrate 4 gate-control env flags from
//	applyEnv reads in config.go to policy.GatesPolicy (Configuration Object);
//	delete env reads; wire at cmd_cycle.go composition root.
//	  - EVOLVE_CONTRACT_GATE  → policy.GatesPolicy.ContractGate
//	  - EVOLVE_EVAL_GATE      → policy.GatesPolicy.EvalGate
//	  - EVOLVE_TRIAGE_CAP_GATE → policy.GatesPolicy.TriageCapGate
//	  - EVOLVE_REVIEW_GATE    → policy.GatesPolicy.ReviewGate
//	Lower FlagCeiling 93→89; regenerate docs/architecture/control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	gates-config-cluster-34:
//	  AC1   4 flags absent from Lookup               → C34_001 (behavioral)
//	  AC2   Registry row count == 89                 → C34_002 (behavioral, count)
//	  AC3   FlagCeiling const == 89                  → C34_003 (config-check, waiver)
//	  AC4   No env reads in config.go                → C34_004 (config-check, waiver)
//	  AC5   GatesConfig() defaults: enforce/enforce/enforce/off → C34_005 (behavioral)
//	  AC6   EVOLVE_WORKTREE_PATH still registered    → C34_006 (behavioral, PRE-EXISTING GREEN)
//	  AC7   flagreaders regression guard green        → manual+checklist (see below)
//	  NEG1  config.Load(emptyEnv) → ContractGate==StageEnforce → C34_NEG1 (behavioral, PRE-EXISTING GREEN)
//
// ACs with manual+checklist disposition:
//
//	AC7 (flagreaders guard green): `go test -tags acs ./acs/regression/flagreaders/...`
//	    Checklist for Auditor:
//	    (a) no compile errors with -tags acs on the cycle34 package;
//	    (b) exit 0 from `cd go && go test -tags acs ./acs/regression/flagreaders/...`;
//	    (c) none of the 4 flag name strings appear in any non-test, non-registry Go file:
//	        grep -rn '"EVOLVE_CONTRACT_GATE"\|"EVOLVE_EVAL_GATE"\|"EVOLVE_TRIAGE_CAP_GATE"\|"EVOLVE_REVIEW_GATE"'
//	         go/ --include='*.go' | grep -v '_test.go' | grep -v 'registry_table.go'
//	         | grep -v 'acs/cycle34' → 0 matches;
//	    (d) all 4 flags had active env reads in config.go:applyEnv before the sweep;
//	        verify that their env reads (and only their env reads) are removed.
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C34_001 — 4 flags must be ABSENT from Lookup (if Builder misses
//	           any one, Lookup returns ok=true and the test fails immediately).
//	           C34_004 — env-read literals must be ABSENT from config.go
//	           (if Builder removes registry rows without deleting call sites, the
//	           literal strings remain and these tests fail — the cycle-8 split-const
//	           anti-gaming check).
//	Edge/OOD:  C34_002 checks exact count 89; both over-removal (< 89) and
//	           under-removal (> 89) fail.
//	Lexical:   Lookup / len / FileContains / FileNotContains / config.Load /
//	           direct struct-field access — distinct assertion verbs across the suite.
//	Semantic:  registry-absence, row-count, ceiling-const, no-env-reads,
//	           config-defaults (GatesConfig), worktree-path-present,
//	           load-default-behavior — 7 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (gates-config-cluster-34). Deferred tasks (WorkflowDefaults cluster,
// StatusInternal classification pass) get zero predicates.
//
// 1:1 enforcement:
//
//	predicate=7, manual+checklist=1, unverifiable-remove=0 → total AC=8 ✓
```

### `go/acs/cycle34/predicates_test.go:74` — above `var removedFlags = []string{`

```text
// removedFlags is the canonical list of 4 flags that cycle-34 removes:
//   - EVOLVE_CONTRACT_GATE:    migrated to policy.GatesPolicy.ContractGate
//   - EVOLVE_EVAL_GATE:        migrated to policy.GatesPolicy.EvalGate
//   - EVOLVE_TRIAGE_CAP_GATE:  migrated to policy.GatesPolicy.TriageCapGate
//   - EVOLVE_REVIEW_GATE:      migrated to policy.GatesPolicy.ReviewGate
```

### `go/acs/cycle34/predicates_test.go:106` — above `func TestC34_004_NoGateEnvReadsInConfigGo(t *testing.T) {`

```text
// TestC34_004_NoGateEnvReadsInConfigGo verifies that all 4 gate-flag env-read
// string literals have been deleted from config.go:applyEnv.
//
// Covers AC4. Anti-gaming (cycle-8 split-const lesson): Builder cannot remove
// the registry rows while leaving env["EVOLVE_CONTRACT_GATE"] (and siblings) in
// config.go:applyEnv. This predicate catches that gap for all 4 flags.
//
// acs-predicate: config-check
//
// RED: config.go currently reads all 4 flags at lines ~517–542 in applyEnv.
// All 4 flag name strings must be absent after migration.
```

### `go/acs/cycle34/predicates_test.go:178` — above `func TestC34_006_WorktreePathStillRegistered(t *testing.T) {`

```text
// TestC34_006_WorktreePathStillRegistered is the non-repeat guard: verifies that
// EVOLVE_WORKTREE_PATH was NOT accidentally removed as part of the cluster sweep.
// Cycles 17, 18, and 19 all failed when a Builder removed WORKTREE_PATH —
// this predicate closes that regression surface.
//
// Covers AC6 (FORBIDDEN-REPEAT guard). BEHAVIORAL: calls flagregistry.Lookup —
// the test fails if Builder removes the row (Lookup returns ok=false).
//
// PRE-EXISTING GREEN: WORKTREE_PATH is currently in the registry (StatusInternal);
// this test is GREEN before Builder makes any changes. It stays GREEN only if
// Builder does NOT touch the WORKTREE_PATH row.
```
