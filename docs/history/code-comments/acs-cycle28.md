# Comment history: `acs/cycle28`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle28/predicates_test.go:3` — above `package cycle28`

```text
// Package cycle28 materializes the cycle-28 acceptance criteria for:
//
//	dispatch-cluster-28 — remove 6 EVOLVE_DISPATCH_* and EVOLVE_TRACKER_TTL_DAYS
//	flags by migrating them to:
//	  - EVOLVE_DISPATCH_DEPTH: IPC split-const (bucket 5) — comment + registry row only
//	  - EVOLVE_DISPATCH_LOG_TTL_DAYS, EVOLVE_TRACKER_TTL_DAYS, EVOLVE_DISPATCH_PLAN_LOG:
//	    CLI flags (bucket 4) in cmd_prune_ephemeral.go / cmd_subagent.go
//	  - EVOLVE_DISPATCH_POLICY, EVOLVE_DISPATCH_REPEAT_THRESHOLD: config-as-code
//	    (bucket 1) via policy.DispatchConfig in policy.go + cmd_loop_control.go
//	Lower FlagCeiling 126→120; regenerate docs/architecture/control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	dispatch-cluster-28:
//	  AC1  6 flags absent from Lookup              → C28_001 (behavioral)
//	  AC2  Registry row count == 120               → C28_002 (behavioral, count)
//	  AC3  FlagCeiling const == 120                → C28_003 (config-check, waiver)
//	  AC4  No env reads for 5 config/CLI flags     → C28_004 (config-check, waiver)
//	  AC5  policy.DispatchConfig struct + method   → C28_005 (behavioral + reflect)
//	  AC6  DISPATCH_DEPTH split-const comment      → C28_006 (config-check, waiver)
//	  AC7  flagreaders guard green                 → manual+checklist (see below)
//	  AC8  control-flags.md has no removed rows    → C28_008 (config-check, waiver)
//	  NEG1 CLI flag defaults preserved (30 and 7) → C28_NEG1 (config-check, waiver)
//	  NEG2 No residual env literals in cmd_loop_control → C28_NEG2 (config-check, waiver)
//
// ACs with manual+checklist disposition:
//
//	AC7 (flagreaders guard green): `go test -tags acs ./acs/regression/flagreaders/...`
//	    Checklist for Auditor:
//	    (a) no compile errors with -tags acs on the cycle28 package;
//	    (b) exit 0 from `go test -tags acs ./acs/regression/flagreaders/...`;
//	    (c) no literal string "EVOLVE_DISPATCH_POLICY", "EVOLVE_DISPATCH_REPEAT_THRESHOLD",
//	        "EVOLVE_DISPATCH_PLAN_LOG", "EVOLVE_DISPATCH_LOG_TTL_DAYS", or
//	        "EVOLVE_TRACKER_TTL_DAYS" in any non-test, non-registry Go file
//	        (grep -rn 'EVOLVE_DISPATCH_POLICY\|EVOLVE_DISPATCH_REPEAT' go/ --include='*.go'
//	        | grep -v '_test.go' | grep -v 'registry_table.go' → 0 matches);
//	    (d) EVOLVE_DISPATCH_DEPTH remains in subagent/recursion.go as the IPC
//	        split-const (grep -n 'EVOLVE_DISPATCH_DEPTH' go/internal/subagent/recursion.go
//	        → at least 1 hit; the const is retained for IPC handoff).
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C28_001 — 6 flags must be ABSENT from Lookup (if Builder misses one,
//	           Lookup returns ok=true and the test fails immediately).
//	           C28_NEG2 — EVOLVE_DISPATCH_POLICY/REPEAT literals must be ABSENT from
//	           cmd_loop_control.go (if Builder only removes the registry row without
//	           deleting the env read, the literal stays and this fails).
//	Edge/OOD:  C28_002 checks exact count 120; both over-removal (< 120) and
//	           under-removal (> 120) fail.
//	Lexical:   Lookup / len / FileContains / FileNotContains / reflect — distinct
//	           assertion verbs across the suite.
//	Semantic:  registry-absence, row-count, ceiling-const, no-env-reads,
//	           dispatch-config-struct, split-const-comment, doc-absence,
//	           cli-flag-defaults, residual-literal-absent — 9 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (dispatch-cluster-28). Deferred tasks (BYPASS_* cluster) get zero predicates.
//
// 1:1 enforcement: predicate=9, manual+checklist=1, unverifiable-remove=0 → total AC=10 ✓
```

### `go/acs/cycle28/predicates_test.go:74` — above `var dispatchFlags = []string{`

```text
// dispatchFlags is the canonical list of 6 flags that cycle-28 removes:
//   - EVOLVE_DISPATCH_DEPTH: IPC split-const (registry row removed; const in recursion.go stays)
//   - EVOLVE_DISPATCH_LOG_TTL_DAYS: migrated to --dispatch-log-ttl-days CLI flag
//   - EVOLVE_DISPATCH_PLAN_LOG: migrated to --dispatch-plan-log CLI flag
//   - EVOLVE_DISPATCH_POLICY: migrated to policy.DispatchConfig.Policy
//   - EVOLVE_DISPATCH_REPEAT_THRESHOLD: migrated to policy.DispatchConfig.RepeatThreshold
//   - EVOLVE_TRACKER_TTL_DAYS: migrated to --tracker-ttl-days CLI flag
```

### `go/acs/cycle28/predicates_test.go:304` — above `func TestC28_NEG2_NoResidualEnvLiteralsInLoopControl(t *testing.T) {`

```text
// TestC28_NEG2_NoResidualEnvLiteralsInLoopControl is the anti-gaming predicate
// that verifies cmd_loop_control.go no longer contains any literal env-key strings
// for the two config-as-code flags it previously read via os.Getenv.
//
// Anti-gaming rationale (cycle-8 lesson): a Builder could theoretically remove the
// registry row without deleting the os.Getenv call, leaving the literal in source.
// AC4 (TestC28_004) catches the quoted Getenv-call form; NEG2 adds a second layer
// by asserting the bare flag name string itself is absent — even in commented-out
// code. Together AC4 + NEG2 close both the live-env-read and residual-literal
// gaming surfaces for cmd_loop_control.go.
//
// acs-predicate: config-check
//
// RED: cmd_loop_control.go:38 contains "EVOLVE_DISPATCH_POLICY" and
// cmd_loop_control.go:58 contains "EVOLVE_DISPATCH_REPEAT_THRESHOLD".
```
