# A stale citation aborted a repair round, and the abort stranded the lane's claim (cycle 1822, 2026-10-07)

## What happened

Wave 73 ran cycle 1822 as the lane for inbox item `fleet-runpool-silent-success`. Times are from the cycle's `signals.ndjson`.

| Time (UTC) | Event |
|---|---|
| 04:58:37 | tdd PASS. |
| 05:19:05 | build PASS. The Builder's explanation document, `docs/explain/builds/cycle-1822-01m4aashs7rpra1rvmn7n0xzyv.md`, lists `go/internal/fleet/quota_test.go` under Changed Areas. |
| 05:31:07 | audit FAIL. `ORCHESTRATOR_AUDIT_REPAIR_GRANTED`: a repair round via tdd, attempt 1 of the `code-audit-fail` budget (2). |
| 05:39:32 | The repair tdd returns PASS. Afterwards the diff no longer contains `go/internal/fleet/quota_test.go`; the preserved branch `cycle-cd3ae73e-1822` has no change to that file. The post-review refresh fails with `refresh Build explanation after tdd: revalidate builder explanation handoff before sealing: Explanation Documentation: cited path go/internal/fleet/quota_test.go is not in the Build diff`, and `ORCHESTRATOR_PHASE_ABORTED` follows. |
| 05:43:23 | Failure learning dispatches a retrospective, and the cycle seals `ORCHESTRATOR_CYCLE_FAILED`. `evolve cycle run` exits 1. Its log line (`boundary-loop.log` line 946612) is the abort text, and no `[inbox-mover] released` line follows. |
| next boundary | `sync-main` refuses to run, because a tracked inbox file (the item claimed into `processing/cycle-1822/`) is deleted from the inbox root. The console ran `evolve inbox-mover recover-orphans`, and the boundary went on. |

An earlier cycle had failed the same way, citing `go/internal/phaseio/digest_test.go`. The code the repair round produced was never audited, because the cycle ended on the explanation's wording.

## Root cause

Two defects sat on the same path.

1. **A content failure after a post-Build writer was treated as an infrastructure failure.** After any later source writer (tdd in an audit-repair round, test-amplification), `applyPostReviewGuards` (`go/internal/core/cyclerun_postreview.go`) and its resume twin (`resumeExecution.run`, `go/internal/core/resume_execution.go`) call `explanationdocs.RefreshResult`. Test files are non-material, so dropping `quota_test.go` changed neither the material path set nor the material hash. The refresh therefore reached `revalidateResult`, where `CheckBuild` returned a content verdict on the Builder's document. `RefreshResult` returned every failure as one undifferentiated error, and both call sites aborted on any error. The same block already routed a material-scope change back to Build for an owner-authored explanation (`requiresBuild`), and the retry vocabulary already had `retry@explanation`, but a stale citation never reached either. A stale citation is a format or process failure. Under the operating policy only logic blocks, and a process failure gets a code-owned recovery rung ([operating policy](../operations/operating-policy.md)).
2. **The abort skipped the inbox failure walk.** `applyPostReviewGuards` returns its abort raw. Every other phase-failure abort in the correction ladder returns `wrapCycleLevelError`. All three roots that run a cycle, `cycleRunErrorExit` (`go/cmd/evolve/cmd_cycle.go`, the fleet lane's path), `handleCycleError` (the sequential loop) and `runResumeBatch`, ran the inbox closeout only when the error was a `*core.ErrCycleLevelFailure`. A raw error is batch-fatal: it stops the batch, which is a correct flow decision. But it also skipped `cycleoutcome.ApplyFailure`, so the claim was never drained from `processing/cycle-1822/`. Cycle 1816 failed with a cycle-level error (`cycle level failure in phase audit`), so its claim was released (`released ← processing/cycle-1816/`). The roots had conflated two separate questions: whether the batch continues, and whether the dead cycle's claims go back to the queue.

## The fix

- **Classification lives at the producer** (`go/internal/explanationdocs`). `checkBuild` returns content verdicts and a fault on separate channels.
  - The fault channel covers a binding or activation error, an unsupported contract, a git derive or hash error, an unreadable Build report, a manifest write error, a git error while proving the document's immutability, and a document that exists but cannot be read as a regular file.
  - Verdicts are a stale or missing cited path, Changed Areas drift, a missing or malformed section, a missing document and an oversized document: all are the Builder's to re-author.
  - `revalidateResult` wraps verdicts in a `contentFailure` that matches `errors.Is(err, explanationdocs.ErrContent)` and nothing else.
  - `CheckBuild`'s public `[]string` renders a fault after the verdicts that accompany it, so its output and every error text are byte-identical to the base for every input but one. When git cannot prove the document's immutability, a git failure just after the refresh's own git reads of the same worktree succeeded, the remaining document checks are skipped.
- **One home for every route back to Build** (`Orchestrator.routeAfterExplanationRefresh`, `go/internal/core/build_explanation_handoff.go`). Both dispatch roots ask `explanationRefreshEligible` (one eligibility rule), keep their `RefreshResult` call, and only schedule or abort with what the helper returns.
  - A material-scope change (`requiresBuild`) re-enters Build, unbudgeted, as before.
  - A non-content error is returned unchanged and aborts raw, as before.
  - A content failure re-enters Build. When every legal successor of the writer already leads to Build (`StateMachine.everyPathReaches`, a walk over the configured legal graph, never a phase-name literal), the round is free and never aborts: tdd's only successors are build-planner and build, so Build was coming anyway and its own explanation floor re-validates the document. That is cycle 1822's shape.
  - Any other writer (for example test-amplification) spends one `AuditRepairAttempts` of the `code-audit-fail` budget, read straight from `computeRetryEnvelope` while it still offers `retry@build`. A spent budget aborts as `ErrCycleLevelFailure`, so it ends the cycle and the batch continues; the message appends the envelope's reason (`no explanation re-author round left: retry budget spent for code-audit-fail (2/2)`).
  - Every taken round emits one WARN `phase.outcome` `ORCHESTRATOR_EXPLANATION_REAUTHOR_ROUTED`: `fields.next`, `fields.charged`, and for a charged round `fields.attempt` and `fields.envelope`, the envelope's own reason.
  - The Build re-run meets the stale citation at its own explanation floor, and the correction ladder hands the Builder the verdict. No new counter, knob or flag was added.
- **Every abnormal exit walks the failure closeout** (`go/cmd/evolve`).
  - `cycleRunErrorExit` applies the failure outcome to every error except the lane deferral, which keeps its own unbumped release.
  - `handleCycleError` applies it on the batch-fatal branch too, and still stops the batch.
  - `runResumeBatch` closes out every error as FAIL.
  - Every root tests the quota wall before it classifies the error, so a resumable wall keeps its claims even when it arrives unwrapped.
  - An error raised before a cycle is allocated carries cycle 0 and walks nothing.
  - Whether an error is cycle-level still decides batch flow, and nothing else.

## Review round 1

The architecture and Go review returned FIX_THEN_MERGE. Its findings and their fixes:

- **The resume root's abort branch was untested** (mutant R14 survived). `TestRunCycleFromPhase_AResumedRefreshAbortStopsBeforeBuild` now covers it with two rows: a budgeted writer past its budget, and a writer that leaves the Build report unreadable.
- **The first rung charged tdd's round, and still aborted once the budget was spent.** Round 1 cost the audit its second repair, and the final round reproduced cycle 1822 batch-fatally. The free route for writers that always lead to Build, and the cycle-level abort, are the fix.
- **Ten changed lines had no killing test.** Tests were added for each: R2, R3, R5, R6, R7, R12, R13, R15, R19, R22.
- **The signal reused `explanationCorrectionEnvelope`'s reason**, which describes an audit that rejected only the explanation. It also reused the `gate.corrected` kind. Both changed.
- **A fault co-occurring with content verdicts dropped the verdicts from `CheckBuild`.** They are kept again.
- **The coverage index lacked this incident.** Two rows added; the summary was recounted.
- **Low findings:**
  - The quota wall is now checked first in `handleCycleError`.
  - An error before allocation (cycle 0) walks nothing.
  - An oversized explanation document is content.
  - The `AuditRepairActive` wording was corrected. In 1822's shape an audit had already spoken, so the flag was already true. A free tdd round keeps the repair's `round1` prompt archive; the first rung would have archived `round2` with no `round1`.

## Review round 2

The delta review judged every design choice sound and the production code correct, and asked for test pins plus one dashboard defect:
- **The material-scope route had no test**, on the base as well (mutant B1). Now `TestApplyPostReviewGuards_AMaterialScopeChangeAfterAWriterRoutesToBuildWithoutCharge` (tdd and a budgeted writer) and `TestRunCycleFromPhase_AResumedMaterialScopeChangeRoutesToBuild` pin it.
- **A fault beside a content verdict** must abort as a fault and keep every line: `TestRefreshResult_AFaultBesideAContentVerdictAbortsAndKeepsEveryLine` pins the exact message.
- **The cycle-level failure names the writer phase** (asserted in the budgeted-writer abort test).
- **The re-author event no longer sets `Event.Attempt`**, which means a dispatch attempt to its consumers. The spent attempt stays in `fields.attempt`.
- **The dashboard counted every `phase.outcome` as a phase run**, so the verdict-less routing dispositions (this signal and the existing audit-repair decision) showed a passed tdd as an incomplete extra round. `dashboard.scanStream` now counts a run only when `fields.verdict` is set (`TestScanStream_ARoutingDispositionIsNotAPhaseRun`, red first).
- **The routing command was renamed** `routeAfterExplanationRefresh`: it spends, signals and prints, so a query's name misled.

## Tests (red first)

The red evidence is in the console session's scratchpad: `explrefresh/red-*.txt` for the first pass, `explrefresh/fix1-red-*.txt` for review round 1 and `explrefresh/fix2-red-*.txt` for round 2 (the `Event.Attempt` assertion and the dashboard). Each red ran against the code it fixes. The first pass stubbed only the new vocabulary: `ErrContent` declared and never wrapped, the signal code declared and never registered.

- `explanationdocs`:
  - `TestRefreshResult_StaleCitationAfterPostBuildWriterIsAContentFailure`: red, and it reproduced the live text exactly.
  - `TestRefreshResult_AnOversizedDocumentIsExplanationContent`: red; the oversize was a fault.
  - `TestCheckBuild_AFaultBesideContentVerdictsKeepsEveryLine`: red with 1 line where base has 2; it passes on the base tree.
  - Preservation:
    - `TestRefreshResult_UnreadableOrUnboundFailureIsNotAContentFailure`: a corrupt host snapshot, a symlinked document, a missing Build report, a binding that names another worktree.
    - `TestRefreshResult_AContentFailureMatchesOnlyErrContent`
    - `TestRefreshResult_AMissingDocumentIsExplanationContent`
    - `TestRefreshResult_AFaultKeepsTheValidatorsFaultText`
    - `TestSealResult_AManifestWriteFaultIsNotExplanationContent`
    - `TestCheckBuild_AMaterialDiffDeclaredNotApplicableIsRejected`
- `core`:
  - `TestApplyPostReviewGuards_AStaleCitationAfterTDDRoutesToBuildWithoutSpendingTheBudget` (attempts 1 and 2): red. The first rung spent the budget, and at 2/2 it aborted with the cycle 1822 text.
  - `TestRunCycleFromPhase_AStaleCitationAfterAResumedTDDRoutesToBuildWithoutSpending`: red with `resume refresh Build explanation after tdd: …`. It also requires Build to be the next dispatch, before build-planner.
  - `TestApplyPostReviewGuards_AStaleCitationAfterABudgetedWriterSpendsOneRepairAttempt`, under the compiled and an injected policy: red on the kind and on the envelope text.
  - `TestApplyPostReviewGuards_ABudgetedWriterPastItsBudgetEndsTheCycleNotTheBatch`: red; the abort was not cycle-level.
  - `TestRunCycleFromPhase_AResumedRefreshAbortStopsBeforeBuild`: the budgeted row was red, not cycle-level; the fault row is preservation.
  - `TestStateMachine_EveryPathReaches`: red against a stub.
  - `TestApplyPostReviewGuards_BindingMismatchStillAbortsAsBatchFatal` (preservation; it also pins failure learning).
- `cmd/evolve`:
  - `TestCycleRunErrorExit_ABatchFatalAbortReleasesTheCyclesClaims` and `TestHandleCycleError_ABatchFatalAbortReleasesClaimsAndStillStopsTheBatch`: red, each with one file left in `processing/cycle-7`.
  - The `a batch-fatal abort` row of `TestRunResumeBatch_AResumedFailWalksTheFailureLifecycle` (integration tag): red, same.
  - `TestHandleCycleError_AnUnwrappedQuotaWallKeepsItsClaimsAndPauses`: red; the batch stopped and the claim was released.
  - `TestCycleErrorRoots_AnErrorBeforeAllocationWalksNoCycle`: red, `processing/cycle-0/ absent` on both roots.
  - `TestHandleCycleError_ABatchFatalAbortWalksTheLifecycleThroughTheRootLedger` and `TestRunResumeBatch_AResumedQuotaWallKeepsItsClaims` (preservation).

## Mutants

Review round 1 ran 52 `go test -overlay` mutants, each one idea:
- the reviewer's 23 (control C0 and R1–R22), re-anchored on the new code with the same ideas;
- the first pass's 17 (M0–M16), re-expressed;
- 12 new ones (N1–N12) for this round's logic: the graph rule (including a no-backtrack mutant that only a rejoining-branch row exposes), the cycle-level wrap, the quota order, cycle 0, oversize, M3's output, the signal kind, the charged flag and the envelope.

49 are killed. Three are set aside:
- **R8, unreachable:** a git error proving the document's immutability, right after the same worktree's git reads succeeded; there is no git seam to inject it.
- **R11, near-equivalent:** Verify checks the diff SHA before `validateRequired`, so a document that stops being a regular file or becomes unreadable fails earlier, and a missing or oversized one is content.
- **R16, near-equivalent:** a lane deferral comes before triage commits anything and classifies system-level, so walking first is an unbumped drain that differs only in the ledger reason text.

R9 (a missing document classed as a fault) survived the first run of this round. After M3, a fault rendered after the verdicts produces the same line a content verdict does, so only the classification differs, and `TestRefreshResult_AMissingDocumentIsExplanationContent` now kills it. M0, M3 and M4 were re-expressed after their first form left an import unused and failed to compile. The first pass ran 17 mutants and killed all 17 (one, the resume root dropping `cursor.schedule`, only after its twin was tightened).

Review round 2 re-ran all 52 on the round-2 code, together with the delta review's B1 (the material route dropped), E1 (the material route shadowed by the no-error case), E4 (the cycle-level failure naming Build), E5 (the event's `Attempt` restored), X1 (a fault dropping the co-occurring verdicts), X2 (a fault beside content taking the content route) and D5, and F1 (the dashboard counting a verdict-less `phase.outcome`). That makes 60, of which 56 are killed. There is no new survivor beyond R8, R11 and R16. D5, the free rule asking for Audit instead of Build, is equivalent: Build's only legal successor is Audit, so a writer that reaches Build on every path reaches Audit too, and a catalog writer with no legal edges reaches neither.

## Limitations

- A content failure's re-author round runs a full Build dispatch, not a document-only one. `CtxKeyExplanationReauthor` and its prompt are worded for an audit's doc-only FAIL ("the audit rejected only the explanation document"), so they are not reused here. The Build floor's correction ladder carries the exact verdict instead.
- `applyPostReviewGuards` (109 lines) and `resumeExecution.run` (273) are still over the 50-line cap, though both are smaller than at base (114 and 274). The protected guard `TestBuildExplanationLifecycleWiring` pins the `explanationdocs.RefreshResult` call inside those exact functions.
- A charged round before any audit has spoken (a writer such as test-amplification ahead of the first audit) leaves `AuditRepairActive` false. Its Build gets no repair brief, and its previous prompt is overwritten rather than archived, as for any non-repair re-dispatch.
