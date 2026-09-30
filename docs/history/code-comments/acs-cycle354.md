# Comment history: `acs/cycle354`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle354/amplified_test.go:3` — above `package cycle354`

```text
// Package cycle354 — adversarial test amplification for cycle-354.
//
// This file extends C354_001–C354_007 (predicates_test.go) with edge-case
// and boundary tests derived from the specification only (anti-bias: no
// implementation code was read when designing these tests).
//
// Gaps addressed:
//
//	C354_001–003 covered CoreInfra, Platform/CLI Hybrid, Workflow Defaults
//	clusters but skipped the Worktree/Workspace cluster entirely.
//
//	C354_004 requires ≥5 of 10 flags to show DEAD — a partial fix that
//	updates exactly 5 flags would still pass. Tests here require all 10.
//
//	C354_001–002 check for absence of ACTIVE but not DEPRECATED for most
//	flags; a fix that set flags to DEPRECATED instead of DEAD would pass
//	those tests. Tests here check absence of DEPRECATED for all 10.
//
//	The deferred Task 2 (fix-dynamic-routing-registry-default) must not
//	have been accidentally implemented; its sentinel annotation must remain.
```

### `go/acs/cycle354/amplified_test.go:48` — above `func TestC354_Amp_004_NoDeprecatedForRemainingTargetFlags(t *testing.T) {`

```text
// TestC354_Amp_004_NoDeprecatedForRemainingTargetFlags verifies that none of the 5
// remaining cycle-354 target flags carry a DEPRECATED status. Guards against a
// regression where a future edit sets a formerly-dead flag to DEPRECATED instead
// of leaving it absent (after cycle-359 removed the 5 Platform/CLI Hybrid flags).
```

### `go/acs/cycle354/amplified_test.go:66` — above `func TestC354_Amp_005_DeferredTaskStillDeferred(t *testing.T) {`

```text
// TestC354_Amp_005_DeferredTaskStillDeferred guards that the deferred
// fix-dynamic-routing-registry-default task was NOT accidentally implemented in
// this cycle. The sentinel is the "default-off" substring in the Cluster field of
// the EVOLVE_DYNAMIC_ROUTING registry entry (scout-report.md F2). If Task 2 were
// implemented, "default-off" would be replaced, breaking the generated index unless
// evolve flags generate was also re-run — which is out of scope for cycle-354.
```

### `go/acs/cycle354/predicates_test.go:3` — above `package cycle354`

```text
// Package cycle354 materializes the cycle-354 acceptance criteria for the
// two committed top_n tasks:
//
//   - fix-cluster-table-dead-flag-status — update 10 hand-maintained cluster
//     table rows in control-flags.md from ACTIVE/DEPRECATED → DEAD (registry=SSOT).
//   - cycle-audit-cycle-scoped-ci-gap — verify that the gofmt CI-parity gate and
//     SKILL.md-drift gate are wired in the audit phase (both fixes already shipped
//     in commits 23582c91 / 7feec764; predicates are pre-existing GREEN regression locks).
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	fix-cluster-table-dead-flag-status:
//	  AC1 (neg)  EVOLVE_RESOLVE_ROOTS_LOADED no longer ACTIVE in cluster table → C354_001
//	  AC2 (neg)  EVOLVE_FAILURE_CLASSIFICATIONS_LOADED no longer ACTIVE         → C354_001
//	  AC3 (neg)  GEMINI_CLAUDE_PATH / GEMINI_REQUIRE_FULL no longer ACTIVE      → C354_002
//	  AC4 (neg)  EVOLVE_STRICT_FAILURES no longer DEPRECATED                    → C354_003
//	  AC1 (pos)  At least 5 flags show DEAD in hand-maintained section           → C354_004
//	  AC2        evolve flags check exits 0 (pre-existing GREEN)                 → C354_005
//
//	cycle-audit-cycle-scoped-ci-gap (both pre-existing GREEN):
//	  CA1  gofmt CI-parity gate wired in audit.NewDefault                       → C354_006
//	  CA2  SKILL.md-drift gate wired in audit.NewDefault                        → C354_007
//
// Floor binding (R9.3): only committed top_n items get predicates.
// fix-dynamic-routing-registry-default is DEFERRED; no predicate for it.
```

### `go/acs/cycle354/predicates_test.go:151` — above `func TestC354_006_AuditGofmtGateIsWired(t *testing.T) {`

```text
// TestC354_006_AuditGofmtGateIsWired verifies that the gofmt CI-parity gate is
// wired into audit.NewDefault (the fix for cycles 339-341 shipping CI-red when
// generated go/acs/cycle<N>/predicates_test.go had gofmt diffs).
//
// BEHAVIORAL: runs the audit package test `TestNewDefault_WiresGofmtCheck` which
// creates a real gofmt-dirty .go file in a temp worktree and asserts that
// audit.NewDefault's Run returns VerdictFAIL. Source-only changes cannot satisfy
// this — the seam must be wired in production config.
//
// NOTE: pre-existing GREEN (fix committed in 23582c91 on 2026-06-15).
```

### `go/acs/cycle354/predicates_test.go:179` — above `func TestC354_007_AuditSkillsDriftGateIsWired(t *testing.T) {`

```text
// TestC354_007_AuditSkillsDriftGateIsWired verifies that the SKILL.md-drift gate
// is wired into audit.NewDefault (the fix for cycles 339-341 shipping CI-red when
// .evolve/profiles/*.json edits caused SKILL.md drift).
//
// BEHAVIORAL: runs `TestNewDefault_WiresSkillsDriftCheck` which creates a drifted
// SKILL.md fixture and asserts VerdictFAIL. A grep-over-source alone cannot make
// it pass — the seam must fire in the actual Run path.
//
// NOTE: pre-existing GREEN (fix committed in 7feec764 on 2026-06-15).
```
