# Comment history: `acs/cycle1590`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1590/predicates_test.go:3` — above `package cycle1590`

```text
// Package cycle1590 materializes the cycle-1590 acceptance criteria for this
// fleet lane's two committed tasks:
//
//   - verdict-surface-clean-exit-binding (inbox id
//     pipeline-defect-pipeline-blocker-cycle1582, P0): the ADR-0072 clean-exit
//     coherence floor must bind the RUNTIME-minted substantive-error and
//     full-deliverable-validity evidence into coherence.CheckVerdictCoherence
//     before a negative verdict is recorded or the loop is halted — a green
//     audit + green ACS + fully-valid deliverable must self-heal ONLY the
//     clean-exit-late-write race, and a malformed PASS-sentinel report or a
//     missing/malformed ACS verdict must never launder into a reconcile.
//   - worktree-provisioning-retry-boundary (inbox id
//     worktree-provisioning-retry): every production `git worktree add`
//     entry point (core.Create, core.CreateFrom, swarm provisioning, the
//     `evolve worktree create` operator CLI) must go through the shared
//     gitexec.AddWorktreeWithRetry bounded backoff, preserving the FIRST
//     failure's diagnostics when a later attempt also fails and staying
//     fail-fast (no backoff) on a permanent condition.
//
// Both areas are scout-flagged as *already implemented* (verdict binding at
// internal/core/system_failure.go's detectVerdictIncoherence; retry adoption
// at all four call sites named in the inbox record) — this file is the
// PERMANENT regression lock the scout report calls for: prove the behavior
// live via the real subprocess-driven regression suites (not a source grep),
// so a future edit that regresses either contract fails THIS cycle's audit
// instead of silently reproducing the cycle-1582 halt or the pre-#401
// un-retried-root incident.
//
// Predicate style (cycle-85 rule): every predicate EXERCISES the system under
// test. C1590_001-003 call coherence.CheckVerdictCoherence directly and
// assert on the returned struct (pure, in-process). C1590_004-010 are
// subprocess predicates that require an explicit "--- PASS: <name>" for each
// named regression test — a renamed/skipped/never-authored test yields no
// PASS line and fails the predicate; exit 0 alone is never sufficient. No
// source-grep predicate exists in this file.
//
// Adversarial diversity (skills/adversarial-testing §6):
//
//	NEGATIVE  → C1590_002 (deliverable INVALID → forged still HALTS — the
//	            strongest anti-no-op signal: a no-op that always reconciles
//	            fails here) and C1590_009 (permanent rc=128 must NOT retry).
//	EDGE/OOD  → C1590_003 (coherent-without-the-deliverable-check cases must
//	            never manufacture a reconcile) and C1590_008 (first-failure
//	            preserved when a later attempt ALSO fails).
//	SEMANTIC  → reconcile / halt / retry-recover / fail-fast-no-backoff are
//	            four distinct outcomes, each asserted separately.
//
// AC map (1:1 with the disposition table in test-report.md):
//
//	Task 1 AC1 (green+valid deliverable self-heals, no halt)        → C1590_001, C1590_006
//	Task 1 AC2 (malformed report / missing ACS never launders)       → C1590_002, C1590_003
//	Task 1 AC3 (SubstantiveError from audit AND ship suppresses halt) → C1590_005
//	Task 1 -race / build / vet regression                            → C1590_010, C1590_011, C1590_012
//	Task 2 AC1 (all 4 entry points use the shared retry helper)       → C1590_004, C1590_006, C1590_007
//	Task 2 AC2 (retryable first failure stays visible)                → C1590_008
//	Task 2 AC3 (permanent failure fails fast, no backoff)             → C1590_009
```

### `go/acs/cycle1590/predicates_test.go:193` — above `func TestC1590_005_CallSiteBindsSubstantiveErrorAndFullVerify(t *testing.T) {`

```text
// TestC1590_005_CallSiteBindsSubstantiveErrorAndFullVerify requires the
// core-package regression proving detectVerdictIncoherence (a) treats a
// ship-phase explained FAIL exactly like an audit-phase one (not just
// AuditFailReasons) and (b) derives DeliverableValid from the FULL
// deliverable.Verify chain (challenge-token + required sections +
// ADR-0039), not the cheap sentinel parse — so a PASS-sentinel-tagged but
// malformed audit-report still halts and a fully-valid one reconciles.
```
