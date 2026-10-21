# internal/auditcascade

> Inbox item: `auditor-tier-cascade` (shadow stage only). Caller: `go/internal/phases/audit/cascade_shadow.go`, from the audit's `finalize`. Policy: `audit_cascade` in `.evolve/policy.json` (`go/internal/policy/audit_cascade.go`).

## Purpose

`internal/auditcascade` decides if a cheap-tier audit pass escalates to the deep tier. The item proposes a cascade: a first audit pass at a cheap tier, and a deep pass only when a trigger fires. This package holds the trigger rule. It is pure: the caller gives the changed paths, the verdict and the rule.

## Design

- **Why it exists.** Each audit runs at the deep tier, whatever the size or the risk of the diff. The item cites FrugalGPT (arXiv 2305.05176) and Tokenomics (arXiv 2601.14470). Tokenomics measured code review at 59.4% of all tokens in an agentic pipeline. A cascade can only save tokens if its triggers are right, so the shadow stage measures the triggers first.
- **The triggers.** `Decide` escalates when one or more of these is true:
  - the verdict is in `escalate_verdicts` (for example `FAIL` or `WARN`);
  - a changed path starts with a prefix in `risk_paths` (for example `go/internal/core/`). Give each directory with its trailing `/`, or `go/internal/core` also matches `go/internal/corex/`;
  - the diff changes more files than `max_changed_files` (0 means no size bound);
  - the diff is unknown. An unknown diff fails closed.

  The decision lists each reason that fired, in that order.
- **No thresholds in Go.** The compiled default is stage `off`, with no paths and no bound. The policy block sets each value. A stage other than `shadow` reads as `off`, because the `enforce` stage is not built.
- **The shadow record.** At stage `shadow`, the audit writes `audit-cascade-shadow.json` in the cycle workspace after its verdict is final. The record holds the stage, the cycle, the final verdict, the count of changed files and the decision (`would_escalate`, `reasons`). The audit reads the diff of the worktree against `HEAD` (`changedpkgs.ChangedFilesChecked`), untracked files included. The audit still runs at the deep tier, so the record changes no verdict and no dispatch.
- **Faults.** A failed write of the record adds one warning diagnostic to the audit. A diff that the audit cannot read records `changed_files: 0` with the reason `the diff is unknown`, so a soak analysis reads the reasons, not the count.

```json
{"audit_cascade": {"stage": "shadow", "risk_paths": ["go/internal/core/", "go/internal/guards/", "go/internal/phases/ship/"], "max_changed_files": 40, "escalate_verdicts": ["FAIL", "WARN"]}}
```

- **The checked-in policy runs the shadow stage** (operator decision of 2026-10-21), with the block above. `TestLoad_TheCheckedInPolicyRecordsTheAuditCascadeInShadow` decodes the block strictly and pins the stage. After one or two waves, the operator reads the escalation rate from the records. Above about 60%, the item is parked, because then the cascade saves little.

## Limits

- The shadow record shows how often a cheap pass escalates. It cannot show the agreement of a cheap pass with the deep pass, because no cheap pass runs. That measure needs a second, cheap audit dispatch in shadow, which costs tokens. It is an open decision of the item.
- The item also names a confidence trigger. The audit report has no confidence field, so this package has no such trigger.
- The `enforce` stage (run the cheap pass first, and the deep pass only on a trigger) is not built. It also needs the cross-family property of the adversarial audit to hold at the cheap tier.

## Invariants

- A trivial clean diff does not escalate. A diff that touches a risk path always escalates. Pinned by `TestDecide_EscalatesOnARiskPathALargeDiffOrAFlaggedVerdictAndSkipsATrivialCleanDiff` and `TestRecordCascadeShadow_AHighRiskDiffWouldEscalateAndATrivialCleanDiffWouldNot`.
- At stage `off` the audit writes no record and reads no diff. Pinned by `TestRecordCascadeShadow_WritesNothingWhenTheCascadeIsOffOrThereIsNoWorkspace`.
