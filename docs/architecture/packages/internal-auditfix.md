# internal/auditfix

> Decision: [ADR-0158](../adr/0158-a-repair-round-runs-only-for-a-finding-the-lane-can-fix-and-the-auditor-applies-a-small-prescribed-fix.md) decisions 7 to 13. Design: [retry-ladder.md](../retry-ladder.md) §5. Plan: [cost-aware-repair-2026-10](../../plans/cost-aware-repair-2026-10.md) item 6.

## Purpose

`internal/auditfix` runs the `audit_fix` rung of the retry ladder. When `retryladder.ChooseAuditFix` chooses the rung, the auditor applies the prescribed patch in its loaded session. A fresh seat reviews only that patch, and the deterministic gates run again. The original audit verdict stays the verdict for the rest of the diff.

The operator asked on 2026-10-27: "make sure audit will involve fix and trigger another review for its changes; balance cost and output". In cycle 2035, the repair round of a test helper fix cost 4.6 LLM minutes of tdd and 47.0 LLM minutes of build ([case study](../../incidents/2026-10-27-cycle-2035-repair-loop.md)). A patch and one review cost less.

## Design

`Handler` implements `core.RungHandler`. `New(cfg, ports)` takes the stage, the held-proposal switch and four ports:

| Port | What it does | Boundary |
|---|---|---|
| `Auditor` | `Seat` names the loaded audit session. `ApplyFix` sends the prescriptions to that session and returns the reason for each changed path. | the bridge session of the audit |
| `Reviewer` | `ReviewSeat` picks a fresh seat that is not the auditor seat. `Review` judges the patch and the prescriptions only. | a new bridge dispatch |
| `Gates` | `Run` runs the handoff floor, cover-changed and ACS on the worktree. | subprocesses |
| `Worktree` | `Snapshot`, `Diff` from the snapshot and `Restore` to the snapshot. | git |

`Handle` takes these steps in order:

1. It returns `not_chosen` for another rung or a declined choice, and `off` at stage `off`. It writes no record.
2. If the record of the cycle says `escalate`, the escalation failed. It returns `Stop` with the reason `audit_fix_exhausted` and calls no port.
3. At `shadow`, it writes the record with the cited files and the prescriptions, and returns `shadow`. It calls no port and changes no file.
4. At `enforce`, it asks for a review seat. A seat with no session, or with the session of the auditor, gives `no_review_seat`. The repair round of today then runs.
5. It takes a snapshot, sends the prescriptions to the auditor and reads the diff from the snapshot.
6. It refuses an empty patch, an uncited file, or a fence path outside a held proposal. It also refuses a patch above the `audit_fix` bounds, from `retryladder.AuditFixOverBounds` on the measured diff.
7. The fresh seat reviews the patch. Then the gates run.
8. A refusal, a failed review or a red gate restores the snapshot and returns `Escalate`. The caller then runs one raised build with the L6 digest.
9. A pass returns `Handled` with `passed`. A patch on a fence path at `held_proposals` `enforce` returns `held_proposal`, and the ship check of ADR-0157 holds the landing.

Each outcome except `off` and `not_chosen` writes `audit-fix.json` in the run directory. The record has the stage, the outcome, the refusal, the failed reasons and the two seats. It also has each finding id with its prescription, the cited files, and the patch with the reason of each change. `ReadRecord` reads it.

## Invariants

- **The fixer seat never reviews.** `TestHandle_TheFixerSeatIsNeverTheReviewerSeat` gives the session of the auditor, and then no session, as the review seat. Each case declines before the auditor runs.
- **A patch stays inside the cited files and the bounds.** `TestHandle_RefusesAPatchOutsideTheCitedFilesOrTheBoundsBeforeTheReview` covers an empty patch, an uncited file, too many lines and new production logic. No review runs, and the tree goes back to the snapshot.
- **A fence path is held, not landed.** `TestHandle_APatchOnAFencePathJoinsTheHeldProposalOnlyAtEnforce`.
- **One escalation, then a stop.** `TestHandle_AFailedReviewOrARedGateEscalatesOnceThenStops`.
- **Shadow changes nothing.** `TestHandle_ShadowRecordsWhatItWouldDoAndChangesNoFile`.
- **Cycle 2035 declines.** `TestReplay_Cycle2035AuditFindingsDecline` replays the four audit failure blocks of cycle 2035 (`testdata/cycle-2035-audit-failures.json`). Each block has more findings than prescriptions, so each block gets the rule `unprescribed` and no fix runs.

## Limits

- No production code registers the handler or builds the ports yet. The call sites are on the fence (`phases/audit`, `internal/core`), so the inbox item `wire-cost-aware-repair-call-sites` wires them.
- The handler pairs the defect and the prescription by their index in the failure block. An audit that writes them in another order gets a wrong pair in the record. The auditor still gets all prescriptions.
- The handler does not sort the findings into lane and hand-off findings. The fixability classifier (`repair-fixability-handoff`, PR #1188) does that before `ChooseAuditFix`.
- The routing picks the family of the review seat. The handler checks only that the session differs from the auditor session.
