# ADR-0113 — A correction re-dispatch completes on worktree evidence

- **Status:** Accepted (2026-10-01, console lane). Consumes inbox `correction-completion-needs-deliverable-rewrite` (F39 part 2; part 1, the explaining nudge, landed 2026-09-26).
- **Supersedes nothing.** Keeps the pre-dispatch baseline guard (the cycle-1550 fix) for every dispatch and adds a second, narrower evidence source beside it.
- **Related:** [ADR-0106 logic-first delivery](0106-logic-first-delivery.md) (a process failure gets a recovery rung, never a block; its [design doc](../logic-first-delivery-design.md) §12 listed this as an open question); [ADR-0027](0027-commit-as-evidence.md) (completion evidence other than a polled file); [ADR-0101 Signal Center](0101-signal-center.md); the bridge's [package notes](../packages/internal-bridge.md).

## Context

The bridge decides when a tmux-driven phase is done. Under the artifact contract a phase is done when its deliverable exists, is non-empty and has stopped changing. Since cycle 1550 the bridge also refuses a deliverable that is byte-identical (same size and mtime) to what was on disk before the prompt went out: that file is the previous attempt's report, and accepting it re-grades an old verdict as the new dispatch's result. The founding case of that guard was itself a correction re-dispatch whose agent changed nothing.

The guard has a blind spot. When the contract review rejects a deliverable for a defect that lives in another file, the correct fix leaves the deliverable untouched:

- **Cycle 1691** (wave 8, 2026-09-26). The build handoff floor rejected the build because one Changed Areas path lacked a what/why line in the explanation document. Correction 1 re-dispatched at 04:37:58. The builder fixed the explanation document at 04:39; `build-report.md` needed no change and `evolve phase verify build` called it well-formed. The bridge saw an unchanged deliverable, waited out the 1200 s build review interval, and sent the one-shot nudge. The agent re-checked, answered "Phase is complete" and did not rewrite. The phase idled until the operator touched `build-report.md` at 05:16 (content unchanged). Without that touch the stop review would have ended the dispatch with exit 81 and failed a correct cycle on a pipeline defect.
- **Cycle 1707** idled about 20 minutes the same way, and six more recoveries in cycles 1706–1720 depended on the agent accepting the nudge.

Part 1 of the fix made the nudge explain the stale deliverable and ask for a re-check. It still relies on the agent choosing to rewrite a file it believes is correct.

## Decision

Keep the baseline guard for every dispatch. Add a completion contract, `worktree-evidence`, that carries a well-formed deliverable forward when the agent has demonstrably acted, and ask for a record of the correction up front.

### Who qualifies (core decides)

`core.PhaseRequest.BridgeCompletion()` returns `core.CompletionWorktreeEvidence` only when all three hold:

1. the request carries a correction directive (the contract-correction ladder, the resume review ladder and the remediation fix all set one; a first dispatch never does);
2. the phase may write source (`WorktreeReadOnly` is false; the orchestrator's fence predicate sets it, so audit, adversarial-review and any unregistered phase are read-only and never qualify);
3. the dispatch has a worktree to take evidence from.

Otherwise it returns `""`, the artifact contract. The phase runner projects the value onto `BridgeRequest.Completion` in its one request builder, so every dispatch path gets the same answer.

The contract names are one typed vocabulary, `core.CompletionContract` (`CompletionArtifact`, `CompletionStdout`, `CompletionGit`, `CompletionWorktreeEvidence`), declared once beside `BridgeRequest.Completion`, which carries that type. The bridge aliases the constants and converts the `--completion` flag's string once, where it builds its `Config`. Because this predicate alone decides who may complete on a carried deliverable, its file (`core/bridge_completion.go`) is on the guards' protected-surface manifest: a lane that could widen it could carry its own stale deliverable past the baseline guard.

### What counts as evidence (the bridge observes)

The `worktree-evidence` contract is the artifact contract (the per-tick stability window is unchanged) plus one extra door at the idle checkpoint:

- **A snapshot at dispatch.** Before the prompt is delivered, the bridge takes a worktree snapshot and keeps it in the dispatch baseline (`dispatchBaseline`) beside the artifact candidates' entries. It runs `git --no-optional-locks -C <worktree> status --porcelain=v1 -z --untracked-files=all --no-renames` and records the size and mtime of every listed path. `--no-optional-locks` keeps the host from taking the index lock under an agent that may run git itself. `--untracked-files=all` lists each untracked file: by default git lists only an untracked directory, whose size and mtime do not change when a file inside it is edited.
- **Host paths never count.** Paths under the worktree's `.evolve/` (host state), paths inside the phase workspace when it is nested in the worktree (challenge token, telemetry, the interaction ledger, the live channel), and the deliverable's own candidate locations are dropped from both snapshots.
- **The idle checkpoint decides.** When a review checkpoint pauses and the pane is not busy (`StopEvent.Busy`, which also covers a render-wedged pane), the bridge takes a second snapshot. It completes when the located deliverable still matches its pre-dispatch baseline, every contract secondary exists, and at least one non-host path differs between the two snapshots (added, removed, reverted or re-edited). A deliverable rewritten since dispatch is left to the stability window, so this door never shortcuts a half-written file.
- **It fails closed.** If either snapshot fails (git cannot read the worktree), there is no evidence and the dispatch completes only on a rewritten deliverable, as before. A failed dispatch snapshot is logged as a WARN.
- **It reports what it saw.** The completion note names the carried deliverable and the changed paths, and the bridge emits `BRIDGE_COMPLETED_ON_WORKTREE_EVIDENCE` (WARN, fields `deliverable`, `changed_paths`, `paths`, `cli`) so the operator sees every carried completion in the Signal Center.

### Who judges the deliverable (core verifies)

The bridge does not judge well-formedness and imports no phase verifier. A carried completion is an ordinary exit 0: the runner's verdict engine verifies and classifies the deliverable, and the correction ladder re-runs the contract review, exactly as for a rewritten one. A carried deliverable that is still wrong is rejected again and costs a correction round, like a wrong rewrite.

### The fast path (the directive)

`composeCorrection(round, reason, remediation)` now ends every correction directive with: append a `## Correction N` section to the deliverable naming what was fixed and where, also when the fix is in another file, so the review that follows and every later reader can see what each correction changed; a deliverable that is not Markdown is written again in full. The request states what the agent must write and why. It does not describe how the host decides completion: that rule belongs to the bridge, and a prompt that restates it drifts from it. An agent that follows it rewrites the deliverable within seconds and completes through the ordinary window. The worktree-evidence door is the backstop for an agent that does not.

## Alternatives considered

- **Drop the baseline guard for corrections.** Rejected: the guard's founding case was a correction whose agent changed nothing, and that agent would complete again.
- **Have the bridge run `phase verify` before completing.** Rejected: it would make the bridge import the deliverable verifiers and duplicate core's review; the layering is evidence in the bridge, judgement in core.
- **Complete on any tick where the worktree changed.** Rejected: a mid-turn agent has changed the worktree too; only the idle checkpoint knows the agent stopped. Completing mid-turn would hand core a tree that is still being edited.
- **Snapshot by walking the filesystem.** Rejected: it cannot tell ignored build output from source without git, and a walk of a large ignored directory at every dispatch is expensive. `git status` lists exactly the paths an agent's fix could consist of.
- **Nudge first, carry forward only after.** Rejected: it doubles the backstop's latency to two review intervals (40 minutes for a build) for no extra safety; the nudge still fires when there is no evidence.
- **A new launch flag instead of a completion contract.** Rejected: completion contracts are already the bridge's Strategy seam (`newCompletionDetector`), the `--completion` flag already carries the name, and the launch flag surface stays unchanged.

## Consequences

- A correction whose fix lives outside the deliverable completes without operator action: at once when the agent appends its `## Correction N` section, at the first idle review checkpoint (the phase's review interval, 1200 s for a build) when it does not.
- A correction where the agent changes nothing still ends in the nudge and then exit 81. A first dispatch, and every read-only phase, still refuse a pre-dispatch leftover.
- The contract applies to the tmux drivers, which honour completion contracts. Headless drivers ignore it, as they ignore every completion contract.
- Each qualifying dispatch runs `git status` once at dispatch and once per idle checkpoint; non-qualifying dispatches run nothing new.
- Every correction directive gains one paragraph; the byte-identical pins of the directive were re-approved in the same change.
- `core/bridge_completion.go` joins the protected-surface manifest, so an autonomous lane can no longer edit who qualifies; changes to it land through the console.

## Tests

Bridge, through `Engine.LaunchArgs` with the claude-tmux driver, the real pre-dispatch capture and a real git worktree: `TestWorktreeEvidence_AFixOutsideAnUntouchedDeliverableCompletesWithoutOperatorAction`, `…_TheBridgeReportsTheCarriedDeliverableAndTheAgentsPaths`, `…_AnAgentThatChangesNothingNeverCompletesOnTheStaleDeliverable`, `…_AFirstDispatchStillRefusesALeftoverDespiteWorktreeEdits`, `…_HostWritesNeverCountAsAgentAction`, `…_ABusyAgentIsNotCompletedMidTurn`, `…_AnAgentThatActsAfterTheNudgeCompletesAtTheNextIdle`, `…_ARewrittenDeliverableStillCompletesByTheArtifactWindow`, `…_NoDispatchSnapshotMeansNoEvidenceEvenOnceGitRecovers`. Detector and snapshot: `TestWorktreeSnapshot_ChangedSinceSeesEveryAgentEditAndNothingElse`, `TestWorktreeEvidenceDetector_ARewriteStillSettlingAtIdleIsLeftToTheStabilityWindow`, `…_WaitsForTheContractsSecondaryDeliverables`, `…_AnIdleSnapshotGitCannotTakeIsNoEvidence`, `…_AWriteAtTheDeliverablesOwnFallbackIsNotAgentEvidence`, `…_AnEditInsideAnUntrackedDirectoryIsAgentEvidence`. Layering: `TestBridge_ReportsEvidenceButNeverImportsAPhaseVerifier`. Vocabulary: `TestCompletionContractVocabulary_SpelledOnce`, `TestCompletionContractVocabulary_EveryRequestNamesItsContractByTheTypedConstant`. Core: `TestPhaseRequest_BridgeCompletion_*` (three), `TestComposeCorrection_AsksForAnAppendedSectionNumberedByTheRound`, `TestCorrectionRecord_StatesTheAgentsDutyNeverTheHostsCompletionRule`, `TestBridgeRequest_EachCompletionContractTravelsAsTheNameTheBridgeParses`, `TestCorrectionLadder_EachReDispatchAsksForItsOwnNumberedSection`. Runner: `TestRun_DispatchesTheCompletionContractTheRequestNames`. Guards: `TestProtectedSurfaceManifest_CoversTheCorrectionCompletionPredicate`.
