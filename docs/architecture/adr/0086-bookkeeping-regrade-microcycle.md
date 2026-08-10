# ADR-0086 — Bookkeeping-only audit FAILs re-dispatch audit in-cycle (retro→audit micro-cycle)

- **Status:** Accepted (2026-08-10)
- **Driving evidence:** three-perspective batch investigation 2026-08-10 (task-scope / gate-economics / routing-learning agents) over cycles 1390–1429; [2026-08-10 persona-strip lobotomy](../../incidents/2026-08-10-persona-strip-lobotomy.md) is the sibling fix for the same FAIL family.
- **Amends:** the retro branch of the transition graph (ADR-0058/0060 config projections updated in lockstep); sits BELOW the ADR-0072 floor.

## Problem

An audit FAIL whose *only* explanations are bookkeeping contracts —
continuation-disposition preflight (`defect ledger: …`), closure-claim
citations (`closure claim without a citation: …`) — while the auditor's own
narrative graded the work PASS/WARN, still burned the full failure path:
retro → cycle FAIL → continuation lane re-drive. Measured in 1390–1429: six
such cycles; continuation re-drives passed **0/11** overall at ~2M tokens
each; each re-drive re-litigated a grown ledger. The cycle died to author one
JSON artifact.

## Decision

At the retro decision chokepoint, a bounded deterministic branch grants ONE
same-cycle audit re-dispatch when ALL of:

1. `CycleState.AuditFailReasons` contains a verdict-conflict record with
   `narrative=PASS|WARN` (`core.BookkeepingConflictAuditReason`);
2. every other reason is bookkeeping-class (`core.BookkeepingMetaAuditReason`);
3. `CycleState.BookkeepingRegradeAttempted` is false.

The branch returns `recoveryTarget(retro, "bookkeeping-regrade", audit)` —
catalog `Recovery.Targets` may remap the target — with the RetroDecision
contract reason `bookkeeping-regrade: …`. The caller consumes the
once-per-cycle slot in orchestrator memory. `retro→audit` is added to the
legality graph (literal + `phase-registry.json` `legal_successors`, kept in
lockstep by `TestLegalGraph_ConfigMatchesLiteral`).

Decision stack, top outranks bottom:

| Layer | Outcome |
|---|---|
| ADR-0072 floor (verdict-incoherence / infra-systemic) | HALT |
| **bookkeeping regrade (this ADR)** | retro→audit, once |
| routing strategy / failure adapter | tdd \| ship \| end |

The regrade is router-non-overridable (mirrors the floor): a strategy
proposing tdd/end cannot eat the bounded re-audit.

## Trust boundary

Eligibility reads orchestrator memory (`AuditFailReasons`, set at the
`recordFloorVerdictFailure` chokepoint), never a workspace file; the bound
lives in `CycleState` (persisted, resume-safe). An agent can neither trigger
a regrade (worst case: one extra gate-graded audit dispatch) nor unbound it
(deleting workspace markers changes nothing). Producer↔classifier prefix
drift is pinned by `phases/audit/bookkeeping_reason_singlesource_test.go`,
which feeds REAL minted diagnostics through the core matchers (ADR-0084 I2).

## Alternatives rejected

- **Remediation sub-dispatch inside the audit phase** — phases are
  single-dispatch by design; re-entry belongs to the orchestrator's decision
  layer where the SM, floor, and forensics already live.
- **Retry via the existing retro→tdd edge** — re-runs tdd+build for work that
  is already built and narrative-approved; the artifact gap is audit-owned.
- **Skip retro before the regrade** — audit FAIL→retro is the SM's invariant
  edge and retro's failure-learning record is wanted either way; the regrade
  costs one extra retro only in the re-fail case.

## Consequences

- The six 1390–1429-class cycles would have re-audited in-cycle; with the
  restored auditor persona (#434) and the disposition preseed (queued), the
  re-audit authors the artifact and proceeds audit→ship normally.
- A re-audit that fails again (even bookkeeping-only) falls through to the
  normal adapter/router path — no loop; the failure fingerprint keeps its
  stable `narrative=<verdict>` normalization.
- New SM edge is covered by the legality oracle, the config-parity test, and
  `bookkeeping_regrade_test.go` (eligibility matrix, once-bound, router
  non-override, floor supremacy).
