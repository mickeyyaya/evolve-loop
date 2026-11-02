# internal/auditfix

> Decision: [ADR-0158](../adr/0158-a-repair-round-runs-only-for-a-finding-the-lane-can-fix-and-the-auditor-applies-a-small-prescribed-fix.md) decisions 7 to 13. Design: [retry-ladder.md](../retry-ladder.md) §5. Plan: [cost-aware-repair-2026-10](../../plans/cost-aware-repair-2026-10.md) item 6.

## Purpose

`internal/auditfix` runs the `audit_fix` rung of the retry ladder. When `retryladder.ChooseAuditFix` chooses the rung, the auditor applies the prescribed patch in its loaded session. A fresh seat reviews only that patch, and the deterministic gates run again. Then the audit runs again on the patched tree. The ship gate binds the verdict of the last audit to the tree that it audited (`phases/ship/audit.go`, `verifyAuditBinding`).

The operator asked on 2026-10-27: "make sure audit will involve fix and trigger another review for its changes; balance cost and output". In cycle 2035, the repair round of a test helper fix cost 4.6 LLM minutes of tdd and 47.0 LLM minutes of build ([case study](../../incidents/2026-10-27-cycle-2035-repair-loop.md)). A patch and one review cost less.

## Design

`Handler` implements `core.RungHandler`. `New(cfg, ports)` takes the stage, the held-proposal switch and four ports:

| Port | What it does | Boundary |
|---|---|---|
| `Auditor` | `Seat` names the seat that writes the patch, or returns an error. `ApplyFix` sends the prescriptions to that seat and returns the reason for each changed path. | a writer seat of the bridge |
| `Reviewer` | `ReviewSeat` picks a fresh seat that is not the auditor seat. `Review` judges the patch and the prescriptions only. | a new bridge dispatch |
| `Gates` | `Run` takes a `GateRequest` (cycle, run directory, worktree) and runs the handoff floor, cover-changed and ACS on the worktree. | subprocesses |
| `Worktree` | `Snapshot`, `Diff` from the snapshot and `Restore` to the snapshot. | git |

`Handle` takes these steps in order:

1. It returns `not_chosen` for another rung, and `off` at stage `off`. It writes no record.
2. If the record of the cycle says `escalate`, the escalation failed. It returns `Stop` with the reason `audit_fix_exhausted` and calls no port. This step comes before the choice. The later audit FAIL has no new choice, because the cycle used its one `audit_fix`.
3. It returns `not_chosen` for a declined choice, and writes no record.
4. At `shadow`, it writes the record with the cited files and the prescriptions, and returns `shadow`. It calls no port and changes no file.
5. At `enforce`, it asks for the fixer seat and a review seat. A seat with no session, or with the session of the auditor, gives `no_review_seat`. The repair round of today then runs.
6. It takes a snapshot, sends the prescriptions to the auditor and reads the diff from the snapshot.
7. It refuses an empty patch, an uncited file, or a fence path outside a held proposal. It also refuses a patch above the `audit_fix` bounds, from `retryladder.AuditFixOverBounds` on the measured diff.
8. The fresh seat reviews the patch. Then the gates run.
9. A refusal, a failed review or a red gate restores the snapshot and returns `Escalate`. The caller then runs the repair round of today. At `workflow.retry_ladder.stage` `enforce`, its first dispatch is the raised build with the L6 digest.
10. A pass returns `Handled` with `passed`, and the caller runs the audit again. A patch on a fence path at `held_proposals` `enforce` returns `held_proposal`, and the ship check of ADR-0157 holds the landing.

Each outcome except `off` and `not_chosen` writes `audit-fix.json` in the run directory. The record has the stage, the outcome, the refusal, the failed reasons and the two seats. It also has each finding id with its prescription, the cited files, and the patch with the reason of each change. `ReadRecord` reads it.

When the failure block has typed `Targets`, the cited files are those targets only. Otherwise the record reads the cited files from the text of the defects and the prescriptions. Pinned by `TestHandle_TypedTargetsAreTheOnlyCitedFiles`.

## Invariants

- **The fixer seat never reviews.** `TestHandle_TheFixerSeatIsNeverTheReviewerSeat` gives the session of the auditor, and then no session, as the review seat. Each case declines before the auditor runs.
- **A patch stays inside the cited files and the bounds.** `TestHandle_RefusesAPatchOutsideTheCitedFilesOrTheBoundsBeforeTheReview` covers an empty patch, an uncited file, too many lines and new production logic. No review runs, and the tree goes back to the snapshot.
- **A fence path is held, not landed.** `TestHandle_APatchOnAFencePathJoinsTheHeldProposalOnlyAtEnforce`.
- **One escalation, then a stop.** `TestHandle_AFailedReviewOrARedGateEscalatesOnceThenStops`.
- **Shadow changes nothing.** `TestHandle_ShadowRecordsWhatItWouldDoAndChangesNoFile`.
- **Cycle 2035 declines.** `TestReplay_Cycle2035AuditFindingsDecline` replays the four audit failure blocks of cycle 2035 (`testdata/cycle-2035-audit-failures.json`). Each block has more findings than prescriptions, so each block gets the rule `unprescribed` and no fix runs.

## Limits

- The composition root registers the handler (`cmd/evolve/audit_fix_wiring.go`, `auditFixRung`) with the production ports of [internal/auditfixports](internal-auditfixports.md). `cycleRun.auditFixRound` in `internal/core` calls it after each audit FAIL of a cycle that chose the rung ([internal-core](internal-core.md)).
- The fixer is not the loaded audit session. Each phase dispatch is an ephemeral bridge session, and no phase profile names a session, so the audit session ends with its dispatch. The fixer port starts a fresh `console-builder` seat on the cycle worktree, with the audit report and the prescriptions as its input. The seat that writes the patch is thus a seat of the routing of `console-builder`, not of the auditor.
- The handler pairs the defect and the prescription by their index in the failure block. An audit that writes them in another order gets a wrong pair in the record. The auditor still gets all prescriptions.
- The handler does not sort the findings into lane and hand-off findings. The fixability classifier (`repair-fixability-handoff`, PR #1188) does that before `ChooseAuditFix`.
- The routing picks the family of the review seat. The handler checks only that the session differs from the auditor session.
