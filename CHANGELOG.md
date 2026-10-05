# Changelog

All notable changes to this project will be documented in this file.

## Changed — `go/acs` carries no comments: the predicate packages are stripped by the tool, and the history they carried is archived (2026-10-01)

- Step 3c of the comment plan ([comment-reduction-2026-09.md](docs/plans/comment-reduction-2026-09.md), Phase 3): the acceptance predicate packages under `go/acs` held 44,439 comment lines, about half the comment lines left in the module. `commentaudit strip acs` removed 43,636 comment lines from 543 files (the strip's count and the rank's comment-line count measure differently, so the two do not subtract exactly). The 1,129 left are machine-read: the `//go:build acs` tags and the `acs-predicate:` markers.
- `commentaudit strip` refuses to write any file whose code would change, and `commentaudit verify -base origin/main acs` confirms the landing is comment-only: `comment-only: 543 changed Go file(s) verified`.
- The history the comments carried (cycle numbers, incidents, the reasons a predicate exists) is kept: `commentaudit history` recorded 1,283 history-bearing comment groups in 485 package pages under [docs/history/code-comments/](docs/history/code-comments/README.md), and rewrote the archive's index.
- Checks: `go test -count=1 -tags acs ./acs/regression/...` passes (the CI suite, including `legacynames` over the new archive pages). Every `go/acs` package compiles as before, except three per-cycle packages (`cycle298`, `cycle523`, `cycle1694`) that already fail to compile on main from API drift, unchanged by this landing.
## Fixed — the loop's own inbox stamps no longer block a sync: `evolve sync-main` lands them (2026-10-01)

- A cycle that does not ship still writes to the inbox item it worked on: a route to the console, a failure-count bump. Those writes stayed uncommitted in the plane, so `evolve sync-main` refused the next boundary and the loop's wave sync could not fast-forward over a later change to the same item. Each such wave needed a hand-made data PR (#752) and a plain-git discard.
- One rule, `internal/inboxstamps`, lands them ([ADR-0112](docs/architecture/adr/0112-loop-inbox-stamps-land-at-the-sync.md)):
  - a **stamp** is a change to a tracked inbox item that touches only keys the mover writes (`route` and the lifecycle-owned fields, `inboxmover.IsMoverWritten`); an operator's hand edit, a deleted or added item, or any other tracked file is other dirt and still refuses, untouched;
  - judged from the merge base with origin, a stamp is **kept** and committed in the plane when origin left the item's bytes alone, **dropped** by name when origin retired it, **replayed** onto origin's version when origin edited or only reformatted it (a field both changed keeps origin's value), and reported **superseded** when origin changed every field it sets, whether origin landed it by hand or set its own values;
  - `evolve sync-main` commits the kept and replayed stamps around its merge, and the next lane ship publishes them; the loop's wave-boundary sync does the same without committing, because a commit would make the plane diverge, which halts the loop;
  - a failure after stamps were restored puts them back, so no failure path loses the loop's write.
- The ADR records why stamps stay in the tracked item rather than a gitignored sidecar: an item's route is one fact, already committed and reviewed there by the operator's `route-lane` and `route-console`.
- Review (architecture and Go, first round Block) found that judging origin from HEAD instead of the merge base would drop a later stamp on a plane still ahead with an earlier stamp commit, the zero-ship case where failure bumps matter most; that treating any inbox modification as a stamp would wipe an operator's edit; and that an item added since the merge base was read as retired. A second round found that the mover's JSON escaping made a third of the inbox read as hand-edited, that a sync whose stamp was already on origin exited 1, and that judging "kept" by field values while the merge is textual could leave the plane diverged. All are fixed and pinned. A third round (Warning) found three correct behaviours no test pinned, restoring every stamp when one cannot be written, a removal origin overrode, and restoring a replay-bound stamp when the landing commit fails, and a label that called origin's overriding value "already on origin"; each now has a test that fails when its fix is reverted, the label reads "superseded", and a stamp is reported dropped or superseded only after the merge succeeds.
- Tests, red first: `TestClassify_*`, `TestPlanAgainst_*`, `TestPlan_*`, `TestChangedOnRemote_*` on real repositories; `TestSyncMain_TheLoopsInboxStampsNeverBlockTheSync`, `TestSyncMain_AnOperatorEditToAnInboxItemStillRefuses`, `TestSyncMain_RefusesToMergeWhenTheStampsCannotLand`, `TestSyncMain_ACompletedSyncWhoseStampIsAlreadyOnOriginSucceeds`, `TestSyncMain_AConflictingMergeRestoresTheStampsItPreparedAndReportsNoneDropped`, `TestSyncMainAtWaveBoundary_DiscardsInboxStampsOriginSupersededAndFastForwards`, `TestSyncMainAtWaveBoundary_WarnsAndKeepsTheStampsWhenTheyCannotBePrepared` and `TestSyncMainAtWaveBoundary_ABlockedFastForwardRestoresThePreparedStamps` drive both callers; `TestIsMoverWritten_*` and `TestUpdateItemJSON_RewritesAnItemThroughTheFacade` pin the new facades. Mutation sweeps across the three review rounds: every mutant of a changed line that compiles is killed by a named test (the third round's five: `TestPlan_RestoreAppliesEveryStampAndNamesEachItCouldNot`, `TestPlan_AReplayRemovesAKeyTheStampRemovedUnlessOriginChangedIt`, `TestSyncMain_RefusesToMergeWhenTheStampsCannotLand`, `TestSyncMain_AConflictingMergeRestoresTheStampsItPreparedAndReportsNoneDropped`, `TestPlanAgainst_AStampWhoseEveryFieldOriginChangedIsSuperseded`).
- Docs: ADR-0112; the runtime-reference procedure "The console route at a boundary" and the `evolve sync-main` step; a new design page, [internal-inboxstamps.md](docs/architecture/packages/internal-inboxstamps.md).
- Consumes inbox `console-route-stamps-block-wave-sync`; its m7 part (the landing probe's result in `routed_reason`) is filed in this change as `route-reason-carries-the-landing-probe`.
## Fixed — a document lane is routed as a document: the bugfix phases declare they do not admit it, the advisor plans from the lane's item, the re-plan counts scout's tasks, and tdd is decided once (2026-10-01)

- **What was wrong.** Cycle 1692, the first live ADR-0099 document deliverable (lane `netflix-margin-device-experience`, scout `goal_type: strategy-options`, `deliverable_kind: document`), was planned scout → triage → premise-challenge → fault-localization → bug-reproduction → tdd → build → error-handling-scan. Four defects:
  - the advisor planned before scout from the wave's goal text; the lane's item, named in `lane-scope.json` before any phase ran, never reached its prompt;
  - nothing declared that a bugfix phase does not admit a document. The 2026-09-28 `insert-when-gates-plan` clamp keys on the trigger, so a document lane whose scout declares `goal_type: bugfix` would still run fault-localization and bug-reproduction;
  - the post-scout re-plan read `scout.item_count=0` and proposed ending the cycle as no-work. The scout report fallback never counted tasks, so every report-only cycle, code or document, digested 0 (1692, 1770, 1774, 1779 and 1780 each got an "end cycle early" re-plan); only the re-plan's shadow stage kept those cycles alive;
  - the tdd decision was recorded `forced-on:tdd` although the plan ran it, while the item's acceptance said tdd does not run for a document; the 1692 audit graded that routing decision as a lane defect (M3).
- **What changed.**
  - Every bugfix-category phase declares in its own `phase.json` that it does not admit a document: `routing.skip_when` gains `{"field": "deliverable_kind", "op": "eq", "value": "document"}` for fault-localization (now also categorized `bugfix`), bug-reproduction, error-handling-scan, flake-rerun-scan and incident-postmortem. The existing `skip-when-gates-plan` gate removes them from a document lane's plan; the router holds no list of its own.
  - A registry gate that removes a planned phase (`skip-when-gates-plan` or `insert-when-gates-plan`) raises `ORCHESTRATOR_PLAN_PHASE_GATED` (WARN, kind `advisor.warning`, fields `phase`, `rule`, `next_phase`) where the routing decision is recorded. The two rule names are exported from `router`, and their clamps now carry `Phase`.
  - The advisor's plan and re-plan input carry `RouteInput.LaneItems`, read from the lane pin and each item's live inbox record (id, kind, deliverable kind, acceptance). The kind is normalized, so an unknown word is no declaration, and an undeclared kind inherits `.evolve/domain.json`; one function, `resolveDeliverableKind`, owns that declared > domain default > `code` precedence for both the lane items and the dispatch signal. The prompt renders them under "Lane scope" after the goal, capped at its own named bound (`maxLaneScopeRunes`, 4000, beside the goal's cap: the advisor's prompt caps have no config home); an unresolvable item is named with the reason. The rubric now projects every `skip_when` as a skip line.
  - The scout report fallback counts `ItemCount` as the `### ` task headings under `## Selected Tasks` (or `## Proposed Tasks`); a "None." section counts 0. The headings come from `phasecontract.SelectedTasks`, a named section that `Scout.Sections` and scout's backlog check also use, so no reader indexes `Scout.Sections` by position. The re-plan stays `shadow`.
  - tdd for a document is decided once, in the registry's `conditional_mandatory.tdd`: released, not forbidden (ADR-0099 amendment 2026-10-01). A phase the plan runs records `plan:<phase>`; `forced-on:<phase>` now appears only on the trigger path. A document cycle's Task Contract states the rule and its result for the cycle, computed by the new `router.TddPinned`, which the floor clamp also calls directly, so no acceptance grades tdd's routing. The loaded config always carries a tdd rule (`config.defaults` seeds it; the registry and env only replace it), so the line has no "no rule" branch. `config.CondRule.String` renders the rule in the registry's spelling.
- **Tests, red first:**
  - `TestDigest_ScoutReportFallback_CountsCycle1692DocumentTask` (on 1692's `scout-report.md`, `go/internal/router/testdata/cycle1692-scout-report.md`) and `TestDigest_ScoutReportFallback_ItemCountIsTheSelectedTaskHeadings`;
  - `TestDocumentLane_NeverDispatchesTheBugfixPhasesOnCycle1692sPlan` (the real registry and the real `.evolve/phases` on 1692's plan, for `strategy-options` and `bugfix`), `TestDocumentLane_TheGateIsThePhaseJSONDeclarationNotAGoList`, `TestBugfixCategoryPhases_DeclareTheyDoNotAdmitADocument`, `TestDocumentLane_Cycle1692TddDecisionIsNotRecordedForcedOn` (1692's decision-6 shape) and `TestRoute_LaneItemsAreAdvisorContextTheWalkIgnores`;
  - `TestWriteRoutingContext_RendersTheLaneItemTheCycleIsPinnedTo`, `TestWriteRoutingContext_AnUnresolvedLaneItemSaysWhy`, `TestWriteRoutingContext_NoLaneItemsRendersNoLaneSection`, `TestWriteRubricLines_ProjectsADeclaredSkipWhen`;
  - `TestPlanCycle_TheAdvisorPlanPromptCarriesTheLanesScopedItem` (through `RunCycle` and the planner seam), `TestPlanCycle_AnUnresolvableLaneItemIsNamedInThePlanPrompt`, `TestLaneItem_TheDeclaredKindIsNormalizedAndAnUnknownWordFallsToTheProjectDefault`, `TestResolveDeliverableKind_DeclaredBeatsTheDomainDefaultWhichBeatsCode`;
  - `TestWriteRoutingContext_ALaneSectionLongerThanTheLaneCapIsCapped`, `TestSelectedTasks_IsTheScoutReportsTasksSection`;
  - `TestRecordRoutingDecision_ARegistryGatedPlannedPhaseEmitsACodedSignal` (pins the kind `advisor.warning` as well as code, severity and module), `TestRecordRoutingDecision_AClampThatRemovesNoPlannedPhaseEmitsNoGateSignal`;
  - `TestSeedTaskContract_DocumentCycleStatesTddFromTheRegistryRule`, `TestSeedTaskContract_ARegistryThatPinsTddForDocumentsSaysSo`;
  - API pins written with the extraction: `TestTddPinned_TheRegistryRuleReleasesADocumentAndATrivialCycle`, `TestCondRuleString_ReproducesTheRegistryExpression`.
  - Mutation sweep: 15 of 15 mutants killed by a named test. The review fixes ran 10 more, all killed: the signal's kind swapped to `gate.corrected`; the lane section uncapped; the Task Contract's rule text dropped; the digest reading the wrong section; `SelectedTasks` losing its legacy alias; the floor evaluating tdd on empty signals; and in `resolveDeliverableKind` the normalization skipped, the precedence inverted, the lane items denied the domain default and `dispatchSignals` reading it ungated.
- **Docs:** ADR-0099 amendment (2026-10-01); `internal-router.md`, `internal-core.md`, `internal-core-advisor.md`, `internal-config.md`, `internal-phasecontract.md`; `dynamic-phase-routing.md`; `runtime-reference.md` (Document-lane routing row); `signal-center-design.md` (`advisor.warning` row); `signal-codes.md` (regenerated); `logic-first-delivery-design.md` (§5.11 T4 and its change log).
- Consumes inbox `document-lane-planned-bugfix-phases`.
## Fixed — a correction whose fix lives outside the deliverable completes without an operator touch (2026-10-01)

- **What was wrong.** A correction re-dispatch whose agent correctly fixed another file and left its well-formed deliverable untouched never completed. The bridge refuses a deliverable identical to the one on disk before dispatch (the cycle-1550 guard against re-grading the previous attempt's report), so the phase idled through a whole review interval, got the one nudge, and ended in exit 81 unless the agent rewrote a file it believed was correct.
- **How it showed.** Cycle 1691: the build floor rejected one explanation-document line, the builder fixed that document at 04:39, and the phase sat until the operator touched `build-report.md` at 05:16 (content unchanged). Cycle 1707 idled about 20 minutes the same way, and six recoveries in cycles 1706–1720 depended on the agent accepting the nudge.
- **What changed** ([ADR-0113](docs/architecture/adr/0113-correction-completes-on-worktree-evidence.md)):
  - **Core decides who qualifies.** `PhaseRequest.BridgeCompletion()` names the new `worktree-evidence` completion contract for a request that carries a correction directive, has a worktree, and may write source (`WorktreeReadOnly` false). Audit, adversarial-review, unregistered phases and every first dispatch keep the artifact contract. The phase runner projects the value onto `BridgeRequest.Completion` in its one request builder, so the contract-correction ladder, the resume ladder and the remediation fix all get it. The predicate's file, `core/bridge_completion.go`, is added to the guards' protected-surface manifest.
  - **One typed vocabulary for the contracts.** `core.CompletionContract` declares `artifact`, `stdout`, `git` and `worktree-evidence` once, beside `BridgeRequest.Completion`, which now carries that type. The bridge aliases the constants and its `Config.Completion` has the same type; the phase judge, the failure advisor, the retry adjudicator and `evolve models live` name `CompletionArtifact` instead of a bare `"artifact"`. The bridge's pre-dispatch snapshot type is renamed `dispatchBaseline`, since it now holds the worktree snapshot beside the artifact entries.
  - **The bridge observes the evidence.** Before the prompt goes out it snapshots the worktree with `git --no-optional-locks status --porcelain=v1 -z --untracked-files=all --no-renames` (size and mtime per path). At an idle review checkpoint (paused, pane not busy) it completes when the deliverable is still the pre-dispatch one, every secondary exists, and a second snapshot differs on a path the host does not own. `.evolve/`, the workspace (challenge token, telemetry, interaction ledger) and the deliverable's own locations never count. A snapshot git cannot take is no evidence.
  - **The bridge reports it.** A `worktree-evidence:` note names the carried deliverable and the changed paths, and the new `BRIDGE_COMPLETED_ON_WORKTREE_EVIDENCE` signal (WARN) carries `deliverable`, `changed_paths`, `paths` and `cli`.
  - **Core still judges.** The bridge imports no phase verifier. A carried completion is an ordinary exit 0: the runner verifies and classifies the deliverable and the review gate re-runs, so a still-wrong deliverable is rejected again.
  - **The fast path.** `composeCorrection(round, reason, remediation)` ends every correction directive with "append a `## Correction N` section to the deliverable that names what you fixed and where — also when the fix is in another file — so the review that follows, and every later reader, can see what each correction changed" (a non-Markdown deliverable is written again in full). The request says what to write and why, never how the host decides completion. An agent that follows it completes through the ordinary window within seconds.
- **Unchanged.** A correction whose agent changes nothing still gets the nudge and then exit 81; a first dispatch still refuses a leftover; the stability window, the finality concession and the nudge text are as before.
- **Tests, red first:**
  - bridge, through `Engine.LaunchArgs` with claude-tmux, the real pre-dispatch capture and a real git worktree: `TestWorktreeEvidence_AFixOutsideAnUntouchedDeliverableCompletesWithoutOperatorAction`, `TestWorktreeEvidence_TheBridgeReportsTheCarriedDeliverableAndTheAgentsPaths`, `TestWorktreeEvidence_AnAgentThatChangesNothingNeverCompletesOnTheStaleDeliverable`, `TestWorktreeEvidence_AFirstDispatchStillRefusesALeftoverDespiteWorktreeEdits`, `TestWorktreeEvidence_HostWritesNeverCountAsAgentAction`, `TestWorktreeEvidence_ABusyAgentIsNotCompletedMidTurn`, `TestWorktreeEvidence_AnAgentThatActsAfterTheNudgeCompletesAtTheNextIdle`, `TestWorktreeEvidence_ARewrittenDeliverableStillCompletesByTheArtifactWindow`, `TestWorktreeEvidence_NoDispatchSnapshotMeansNoEvidenceEvenOnceGitRecovers`;
  - bridge detector and snapshot: `TestWorktreeSnapshot_ChangedSinceSeesEveryAgentEditAndNothingElse`, `TestWorktreeEvidenceDetector_ARewriteStillSettlingAtIdleIsLeftToTheStabilityWindow`, `TestWorktreeEvidenceDetector_WaitsForTheContractsSecondaryDeliverables`, `TestWorktreeEvidenceDetector_AnIdleSnapshotGitCannotTakeIsNoEvidence`, `TestWorktreeEvidenceDetector_AWriteAtTheDeliverablesOwnFallbackIsNotAgentEvidence`, `TestWorktreeEvidenceDetector_AnEditInsideAnUntrackedDirectoryIsAgentEvidence`, `TestBridge_ReportsEvidenceButNeverImportsAPhaseVerifier`;
  - bridge vocabulary: `TestCompletionContractVocabulary_SpelledOnce` (the `artifact` and `worktree-evidence` literals now live only in `core/ports.go`), `TestCompletionContractVocabulary_EveryRequestNamesItsContractByTheTypedConstant`;
  - core: `TestPhaseRequest_BridgeCompletion_ASourceWritingCorrectionCompletesOnWorktreeEvidence`, `TestPhaseRequest_BridgeCompletion_EveryOtherDispatchKeepsTheArtifactContract`, `TestPhaseRequest_BridgeCompletion_TheWorktreeFenceDecidesWhoQualifies`, `TestComposeCorrection_AsksForAnAppendedSectionNumberedByTheRound`, `TestCorrectionRecord_StatesTheAgentsDutyNeverTheHostsCompletionRule`, `TestBridgeRequest_EachCompletionContractTravelsAsTheNameTheBridgeParses`, `TestCorrectionLadder_EachReDispatchAsksForItsOwnNumberedSection`; the directive's byte-identical pin is re-approved and `TestCorrectionCallSites_PassTheRemediation` now also scans `resume.go`;
  - runner: `TestRun_DispatchesTheCompletionContractTheRequestNames`;
  - guards: `TestProtectedSurfaceManifest_CoversTheCorrectionCompletionPredicate`.

  Twenty-six mutants, each killed by a named test: nineteen from the first pass, and seven from the architecture review's round. Emptying the deliverable-location exclusion in `hostOwned` (`slices.Contains([]string{}, abs)`) is killed by `TestWorktreeEvidenceDetector_AWriteAtTheDeliverablesOwnFallbackIsNotAgentEvidence`. Dropping `--untracked-files=all` from the snapshot is killed by `TestWorktreeEvidenceDetector_AnEditInsideAnUntrackedDirectoryIsAgentEvidence`. Putting the host's completion rule back into the record request is killed by `TestCorrectionRecord_StatesTheAgentsDutyNeverTheHostsCompletionRule` and the byte-identical pin `TestComposeCorrection_MalformedClassIsByteIdentical`. Removing the manifest entry is killed by `TestProtectedSurfaceManifest_CoversTheCorrectionCompletionPredicate`. A bare `"artifact"` in the phase judge's request is killed by `TestCompletionContractVocabulary_EveryRequestNamesItsContractByTheTypedConstant`. Spelling the bridge's `completionArtifact` as its own literal again is killed by `TestCompletionContractVocabulary_SpelledOnce`. Changing a contract's wire name (`stdout` to `stdout-repl`) is killed by `TestBridgeRequest_EachCompletionContractTravelsAsTheNameTheBridgeParses`.
- **Docs:** ADR-0113; the package notes for `internal/bridge`, `internal/core`, `internal/phases/runner` and `internal/guards` (the manifest entry); a row in `docs/operations/runtime-reference.md`; the manifest's size and a surface row in `docs/operations/pipeline-factory-rules.md`; `docs/architecture/signal-codes.md` regenerated; the logic-first design doc's §10 row, §12 open question and history.
- Consumes inbox `correction-completion-needs-deliverable-rewrite`.
## Fixed — the builder's pre-handoff probes run the build handoff floor itself, so a green probe means a green floor (2026-10-01)

- **What was wrong.** Three cycles failed on a build handoff floor check that the builder's own probe had just reported green. Cycle 1763: a correction added a material path with no Changed Areas bullet, and `evolve selfcheck build` said GREEN. Cycle 1764: `selfcheck build` said GREEN while an added `-tags acs` package failed the floor. Cycle 1788: a correction rewrote `build-report.md` with one Write and dropped `## Explanation Documentation`; `evolve phase verify build` printed OK, and the floor rejected the build on its last correction. The floor was composed in three places (core's mandatory explanation reviewer, `productionBuildFloorChecks`, the document solution chain), `selfcheck` ran one of them with only a worktree (no base, cycle or contract, so it diffed against `HEAD` and never ran the explanation check), and `phase verify build` ran none of them.
- **What changed.**
  - One named check list, `core.BuildHandoffFloor` (`BuildFloorCheck{Name, Run}`; a Composite whose `Review` is the build floor reviewer's). Core owns the mandatory half (`MandatoryBuildHandoffFloor`, the explanation check, now self-gated on the contract version); the composition root owns the composed half (`composedBuildHandoffFloor`: `production`, plus `document-solution` on a document registry; nothing when `workflow.build_floor` is off) and mounts that list in the reviewer chain. `WholeBuildHandoffFloor` joins them in the cycle's order.
  - Both probes run `probeBuildHandoffFloor(projectRoot)`, the whole list, through one seam. `evolve phase` gets it by injection (`phasecmd.NewRunPhase(floor)`; `phasecmd.RunPhase` is removed), and `verify build` reports each floor failure as violation `build_handoff_floor` in the floor's own words.
  - Same inputs: `core.ReviewInputFor(cs, phase, projectRoot)` is the one projection of the cycle binding; the orchestrator's live and resumed reviews use it, and the probes use it on the persisted state read by `core.ReadRunCycleState` (moved from `phasecmd`). `core.BuildHandoffProbe.Input()` refuses a state bound to another worktree. With no binding a probe WARNs, runs on the tree it was given, and `selfcheck` no longer claims `safe to hand off`.
  - `Orchestrator.BuildHandoffFloorNames()` lists the checks the reviewer chain runs, nested chains included.
- **Tests**, red first: the replays through the real `evolve` dispatch, `TestPhaseVerifyBuild_Replay1788_ReportWithoutTheExplanationSectionFailsWithTheFloorsMessage`, `TestBuildHandoffProbes_Replay1788_CitedPathNotInTheDiffFailsBothProbes` and `TestBuildHandoffProbes_Replay1763_MaterialPathWithoutAChangedAreasBulletFailsBothProbes` (exit 0 / GREEN on the parent commit), with the control `TestBuildHandoffProbes_AnExplainedBuildIsGreenOnBothProbes`; the one-list proof `TestBuildHandoffProbes_IterateTheCycleFloorsCheckList` (the production orchestrator's list equals the probes'), `TestBuildHandoffProbes_BothRunTheSeamFloorOnTheBoundCyclesInput`, `TestComposedBuildHandoffFloor_FollowsTheWorkflowDialAndTheDocumentContract`; in `phasecmd`, `TestPhaseVerifyBuild_RunsTheInjectedFloorOnTheBoundCycleInTheFloorsWords`, `TestPhaseVerifyBuild_WithoutACycleBindingRunsTheFloorUnboundAndSaysSo`, `TestPhaseVerify_OnlyBuildRunsTheFloor`, `TestPhaseVerifyBuild_WithoutAWiredFloorSaysTheFloorDidNotRun`; in `core`, `TestBuildHandoffFloor_*`, `TestWholeBuildHandoffFloor_RunsTheMandatoryExplanationFloorFirst`, `TestMandatoryBuildHandoffFloor_LegacyCycleOwesNoExplanation`, `TestOrchestrator_BuildHandoffFloorNames_ListsEveryFloorCheckTheReviewerRuns`, `TestReviewInputFor_ProjectsTheCycleBindingTheFloorReviews`, `TestReadRunCycleState_PrefersTheWorkspaceMirrorAndNamesAnUnreadableFile`, `TestBuildHandoffProbe_Input_BindsThePersistedCycleOrSaysWhyNot`.
- The size ratchet's allowances shrink with the functions this change shortened (`wireOrchestratorDeps` 302, `reviewWithCorrections` 314, `reviewResumedDeliverable` 67, `loadResumeBootstrap` 75; the renamed `runPhaseVerify` entry is gone).
- **Architecture review revision (FIX_THEN_MERGE), the same day:**
  - `go/internal/core/build_handoff_floor.go` joins `guards.ProtectedSurfaceManifest` (one appended row), since it holds the floor's mandatory half and the review-input projection; `TestProtectedSurfaceManifest_CoversTheBuildHandoffFloor`.
  - The resume bootstrap sets a legacy checkpoint's missing `cycle_id` to the resumed cycle once, so `ReviewInputFor` is total on resume and the resumed review's per-caller cycle patch is gone; `TestResumeBootstrap_LegacyCheckpointReviewsUnderTheResumedCycle` (red with the patch removed).
  - The list is the only floor constructor: `NewBuildFloorReviewer` is folded into `BuildHandoffFloor.Review`, and `NewBuildExplanationReviewer` and `ChainBuildFloorChecks` are deleted; their tests and `acs/cycle1076` use the list.
  - One build self-check: `phasecmd.BuildSelfCheck` reads the binding once and judges the contract (on `deliverable.RootsFor`, now exported) and the floor on it; `phase verify build` and `selfcheck build` both call it, and the builder persona names one (`phase verify build`). `TestPhaseVerifyBuild_OneCycleBindingFeedsTheContractAndTheFloor` (red: the contract half had judged the flag's worktree), `TestSelfcheckBuild_IsThePhaseVerifyBuildPath` (red: `selfcheck` said GREEN on a report missing `## Changes`), `TestBuildSelfCheck_VerifyReportsTheContractAndTheFloorTogether`, `TestRunSelfcheck_WithoutACycleBindingNeverClaimsSafe`.
  - The correction ladder reads its worktree from its review input, not the dispatch-time snapshot.
  - Every check reports at the host's severity: with a binding the contract half runs exactly the host gate's checks, and the docs floor reaches the builder only as the host's `[docs-floor] WARN` line, never a failure (an earlier draft of this revision made it a blocking violation on the bound worktree, stricter than the host). `TestPhaseVerifyBuild_BoundDocsFloorReportsAtTheHostsSeverity` (red: the self-check failed `missing_architecture_docs` while the host approved with a WARN). An unbound `--worktree` run keeps the ADR-0077 addendum's blocking check.
- **Docs:** [ADR-0117](docs/architecture/adr/0117-pre-handoff-probes-run-the-build-floor.md); the design notes of [internal/core](docs/architecture/packages/internal-core.md), [cmd/evolve](docs/architecture/packages/cmd-evolve.md) (F38 closed), [internal/cli/phasecmd](docs/architecture/packages/internal-cli-phasecmd.md), [internal/deliverable](docs/architecture/packages/internal-deliverable.md) and [internal/guards](docs/architecture/packages/internal-guards.md); the "Build handoff probes" row in [runtime-reference.md](docs/operations/runtime-reference.md); the manifest count in [pipeline-factory-rules.md](docs/operations/pipeline-factory-rules.md); the builder persona; the research index.
- Consumes inbox `pre-handoff-probes-run-the-build-floor`.

## Added — `commentaudit strip`: comments are removed by a tool, and a Clean Code review decides how the code must change to read without them (2026-09-30)

- The operator's goal (2026-09-30): "remove all comments from the code and refactor the code to be clear and self-explanatory". Editor rounds had removed comments a few directories at a time, and about 88,000 comment lines remained.
- **`commentaudit strip [dir ...]`** removes, in place, every comment the convention does not allow. It keeps:
  - tool directives and markers (the rule `verify` uses) and a `minimal:` or `Deprecated:` note's whole paragraph;
  - an Example's whole `Output:` block, a cgo preamble, and a comment group carrying the `IPC-protocol-allowed` marker, which the envtaint scan reads per group;
  - one package doc per package, from `doc.go`, else `<dir>.go`, else the first file with one, shortened to its first sentence past three lines.
  It skips generated files and `rank`'s skip set, formats what it rewrites, writes each file atomically with its mode kept, and refuses any file whose code would change, naming it and exiting 1. A package doc it cannot bring within three lines of six words is left as written and reported for a person to write.
- **One table of the comments a tool reads.** `equivalence.go` lists each marker with how far it reaches (its line, its paragraph, the rest of its group, its whole group, or the file); `verify`'s directive pattern is rendered from it and `strip` reads the reach, so the two cannot drift. The six-word package-doc floor is `commentaudit.MinPackageDocWords`, which `acs/regression/docgo` now imports.
- **Two predicates stop reading comments:** `acs/cycle50`'s `TestC50A_003` and its unit twin match `codexConfigPath\s+string`, since gofmt realigns a struct once its comments go, and the Ollama classifier test checks the explanatory name `metadataOnlyListArgs`.
- **A trial on main** removed 89,895 comment lines from 3,355 files in about six seconds; the result builds and vets. Three files were refused, and four tests that read source text failed. Those tests now read code: an anchor on `observeIdle`'s signature, an ACS pin that tolerates realignment, and `metadataOnlyListArgs` in place of the comment that said the Ollama list call reaches no model.
- **A Clean Code review of the result** ([report](docs/reports/comment-strip-clean-code-review-2026-09-30.md)): of 64 of the riskiest stripped files, none read fully without their comments, and two thirds of the 901 removed comment groups carried intent, invariants or design the code did not yet say. So production code lands package by package with its refactors, tests and design notes, and only tests and predicate packages are stripped in bulk. The review also found defects the comments were covering for, listed in the report.
- Tests, red first: the `TestStripComments_*`, `TestStripCommentsKeepingPackageDoc_*`, `TestStripDirs_*`, `TestPackageDocHolder_*`, `TestLeadingSentences_*` and `TestMain_Strip*` cases, and `TestEquivalent_AnchorsOnlyTheParagraphOfANoteToolsRead`. Review mutation sweeps killed every mutant, each by a named test.
- Docs: the convention's `strip` entry; Phase 3 of the [comment plan](docs/plans/comment-reduction-2026-09.md); the review report with every reviewer's findings; a new design page, [internal-commentaudit.md](docs/architecture/packages/internal-commentaudit.md).

## Added — the history deleted comments carried is recorded: `commentaudit history` and the comment history archive (2026-09-30)

- The operator's rule (2026-09-30): "The history track must be recorded and stored."
- The comment-reduction workstream moved design knowledge into package notes, but it deliberately left out history: cycle numbers, incidents, dates and F-ids. Git's diffs were its only record.
- **`commentaudit history -base <ref> [-label L] [-out DIR] [dir ...]`** records the history a change removes: each removed comment group that carries history, whole, as it was, with its file and line and the code below it.
  - A group that reappears whole elsewhere in the change is a move and is not recorded. Within a file it is matched first by its text and the code below it, then by its text alone; across files, by its text alone.
  - A group that is reworded, split or merged is recorded as it was.
  - It skips `testdata/`, `vendor/` and dot directories, by the one predicate `Rank` uses (`isSkippedDir`).
  - The commit gate's added-comment scan (#753) had its own copy of that predicate, which skipped `testdata/` and `vendor/` but not dot directories. There is now one `isOutsideProjectCode`, so the gate also skips code under a dot directory, as `Rank` and the go tool do (`TestAddedAcrossDiff_SkipsCodeUnderADotDirectoryAsRankDoes`).
  - An entry's anchor is the code below it, not a comment: a whole-line comment is skipped, and a line's trailing comment is cut from the anchor.
  - A relative `-out` resolves against the repository root, so running it from `go/` still writes the one archive.
  - Library: `commentaudit.RemovedHistoryAcrossDiff` and `RenderHistorySection`; writing the pages and the index stays inside the package.
- **The history rule is wider.** It now also matches round numbers, PR numbers (`#503`), commit SHAs (7–40 hex characters holding both a letter and a digit) and release versions (`v11.5.0`). `check` uses the same rule, so it tightens too.
- **The archive:** `docs/history/code-comments/` holds one page per package, with one labelled section per change.
  - Its `README.md` index is rewritten from the pages on every run, even after a failed write, and lists only archive pages, so nothing there is edited by hand.
  - A label already on a target page is refused before anything is written, so a change is recorded once.
  - The backfill records everything removed between the workstream's baseline (3ce14dd0, 2026-09-26) and main at f27afd8b, after round 12 and the zero-comment policy (#753): 3,990 history comment groups on 76 package pages, under the label "comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)".
- **What enforces it, and what does not yet:** the comment-reduction rounds run `history` once per round. No gate runs it, so another change that deletes a history-bearing comment is recorded only if its author runs it; the convention says so, and the commit-gate refusal is filed as inbox item `history-bearing-comment-removal-needs-an-archive-entry`.
- **The protocol:** the convention names the archive as where comment history goes. The batch protocol runs `history -base $(git merge-base HEAD origin/main)` once per round, after its last batch and before its comment-only commit. `.evolve/naming.json` excludes `docs/history`, so the naming guard never rewrites the verbatim archive.
- Tests, red first:
  - `TestRemovedHistory_RecordsEachRemovedHistoryGroupWithWhereItSat`;
  - `TestRemovedHistory_KeptOrMovedHistoryIsNotRecorded`;
  - `TestRemovedHistory_AnUntouchedTrailingCommentInATouchedFileIsKept`;
  - `TestRemovedHistory_IdenticalHistoryTextIsAttributedToTheGroupThatWentAway`;
  - `TestRemovedHistory_AMoveWithinAFileNeverExcusesARemovalInAnother`;
  - `TestRemovedHistory_OtherHistoryAddedElsewhereExcusesNothing`;
  - `TestRemovedHistory_ARewordedGroupIsRecordedAsItWas`;
  - `TestRemovedHistory_ADeletedFileRecordsItsHistory`;
  - `TestRemovedHistory_SkipsWhatIsNotProjectCode`;
  - `TestRemovedHistory_AnchorsATrailingCommentAtItsColumnNotTheFirstSlashes`;
  - `TestRemovedHistory_ClipsALongAnchor` and `TestRemovedHistory_KeepsAnAnchorOfExactlyTheLimitWhole`;
  - `TestRemovedHistory_OneMoveExcusesOneRemoval` and `TestRemovedHistory_WithinAFileOneRewrittenCopyExcusesOneRemoval`;
  - `TestRemovedHistory_AnchorsOnTheCodeBelowSkippingOtherComments`;
  - `TestWriteHistoryArchive_ALabelThatPrefixesAnotherIsItsOwn`;
  - `TestWriteHistoryIndex_ListsOnlyArchivePages`;
  - `TestRenderHistorySection_AFenceInsideTheTextNeverClosesTheEntry`;
  - `TestMain_HistoryAppendsASectionPerPackage` (it also checks the index);
  - `TestMain_HistoryRecordsAChangeOnce`;
  - `TestMain_HistoryWithoutOutPrintsTheSectionAndWritesNothing`;
  - `TestMain_HistoryWithNothingRemovedSaysSo`;
  - `TestMain_HistoryLabelsAnUnlabelledRunByItsBase`;
  - `TestWriteHistoryArchive_AFileAtTheRootGetsTheRootPage` (a file directly under `go/` included);
  - `TestWriteHistoryArchive_RefusesALabelThatSpansLines`, since a label becomes a section heading;
  - `TestWriteHistoryArchive_RewritesTheIndexAfterAFailedAppend`;
  - `TestMain_HistoryResolvesARelativeOutAgainstTheRepoRoot`;
  - `TestIsNarrative_CountsPullRequestsCommitsReleasesAndRounds`.
## Changed — code carries no comments: the commit gate refuses an added comment, and every loop skill and persona states the rule (2026-09-30)

- The operator's rule (2026-09-30): "Add system level policy for evo loop skill and project to set the rule that it is forbidden to write comments; code should explain itself."
- **The commit gate refuses a commit that adds a comment** (`refuseAddedComments`, before the review waiver). It uses `commentaudit.AddedAcrossDiff`, the rule the build floor applies, reading HEAD through `commentaudit.ReadAtBase`. It names each added line and writes no attestation. This closes a hole: a comment-only *addition* is AST-equivalent, so it could ride the comment-only review waiver.
  - A comment moved between files is not added, `git mv` included. The change is listed with `--no-renames` through the listing the waiver now shares.
  - An unreadable HEAD, or a change that cannot be listed, is a fault (`ExitGitFatal`), never a new file.
- **`commentaudit`, the shared rule:**
  - a rewrite of an existing package doc is not an added comment;
  - comments are Go scanner tokens, so a `//` line inside a string-literal fixture is not a comment;
  - files under `testdata/` are inputs and `vendor/` is third-party code, so both are skipped (a vendored-module refresh would otherwise be refused with no way to comply);
  - a spared package doc may not grow past three lines, or past its previous length.
  - comments are read on physical lines, so a `//line` directive hides no later comment;
  - the refusal lists the change with `-z`, so a non-ASCII path is read, not quoted past the `.go` check.
- **The rule in every skill and persona:**
  - the evo loop skill gains *Code carries no comments (system policy)*;
  - AGENTS.md gains cross-CLI invariant 10;
  - the debugger, bug-reproduction, test-amplification and tdd-engineer personas and the build, tdd, refactor and minimalism skills state it.

  The tdd-engineer persona had allowed a fixture comment and showed two comments in its example; both are gone. The allowed set is listed only in the convention.
- **Loop lanes, not yet enforced:** the build floor's `comment_floor` stays `shadow` (it WARNs).
  - The architecture review found that enforcing it now would stall the loop. The floor scans the whole diff, the TDD phase writes the predicate files, and the builder may not edit them; 10 of the last 15 cycles' predicate files hold comments the floor counts.
  - The next change gives the TDD phase its own comment floor and keeps its files out of the build floor's scan; then this project sets `enforce`.
  - The compiled default stays `shadow` for other projects, whose license headers and generated code a blanket refusal would break.
- Tests, red first:
  - `TestRun_ACommitThatAddsACommentIsRefused`;
  - `TestRun_WhatAToolReadsIsNotAnAddedComment`;
  - `TestRefuseAddedComments_ARenamedCommentedFileAddsNoComment` (a real `git mv`);
  - `TestRefuseAddedComments_AnUnreadableHeadIsAFaultNotANewFile`;
  - `TestRefuseAddedComments_AnUnlistableChangeIsAFault`;
  - `TestAddedComments_RewritingAnExistingPackageDocAddsNothing`;
  - `TestAddedComments_ALineInARawStringIsNotAComment`;
  - `TestAddedAcrossDiff_SkipsTestdata` and `TestAddedAcrossDiff_SkipsVendoredCode`;
  - `TestAddedComments_ARewrittenPackageDocMayNotGrowIntoNarrative`;
  - `TestAddedComments_ACommentTrailingARawStringIsNotAWholeLineComment`.

  The gate's golden fixture no longer edits a doc comment.

## Fixed — a lane ship can no longer break a whole-tree test that only main ran (2026-09-30)

- Cycle 1779's lane ship (`461aa782a`) grew `cmd/evolve.runCycleRun` to 91 lines (allowance 86) and `runCycleHealth` to 51 (limit 50). The ship's gates passed and main's required CI went red, so every open PR inherited the failure.
- Cause: changed-scope testing selects a lane's packages and their importers, so a test that reads the whole tree runs only when its own package changes. The ship's fixed scanner pack held the raw-git ratchet but not the size ratchet. It was also missing the other whole-tree tests: `testmainexit`, `policy`'s env-agnostic scan, `guards`' call-site scans, `acssuite`'s tag guard and `fleet`'s module-graph partition.
- The pack's list moves to `repocontract.Packages()`, and ship projects it. It gains `sizeratchet`, `testmainexit`, `policy`, `guards`, `acssuite`, `fleet` and `repocontract` itself. The pack runs its packages in parallel, so its wall time is set by the slowest member.
- `TestPackages_HoldEveryTestThatReadsTheWholeTree` finds, from the AST of every tracked test outside `acs/`, each package whose tests climb out of their directory onto the module root or a top-level directory, or walk up to `go.mod`. It requires each one to be in the pack or recorded outside it with a reason.
  - A climb's literal path parts are joined and cleaned as `filepath.Join` does, so `./..`, empty parts and a name followed by `..` resolve as they would at run time.
  - `TestClimbsOutOfItsPackage`, `TestWalksUpToGoMod` and `TestPackProblems` pin each shape, the negative ones included (a string prefix test, a climb from another directory, a call at the module root, a fixture loop naming `go.mod`, a stale record).
  - 13 large packages with seam tests are recorded as waiting for test-level selection (inbox `repo-contract-test-level-selection`).
  - The detector is in the pack, so it runs before main.
- `runCycleRun` parses its flags in `parseCycleRunFlags` into a `cycleRunFlags` value and builds its request with `cycleRunFlags.request(projectRoot, environ)`. It is now 62 lines, and its allowance tightens from 86 to 62. `runCycleHealth` resolves its root in the new `cycleHealthRoot` and is now 44 lines.
- New tests pin the flag parsing, the request's goal text, bypass and EVOLVE_ env, `--simulate` never wiring the production orchestrator, and the root resolution. Mutation sweeps killed every mutant the architecture reviews raised, on both the moved lines and the detector.
- The docs name `repocontract.Packages()` instead of restating the list. The new `docs/architecture/packages/internal-repocontract.md` holds the pack's rationale (moved out of a code comment), the detection rule and its limits.

## Added — `evolve inbox add`, the one way to file an inbox item (2026-09-30)

- The inventory of core functions against the CLI (147 functions: 64 fully, 36 partly and 47 not at all executable through `evolve`) ranked this the first gap: nothing filed an inbox item, so every item was hand-written JSON with no schema, id or dependency check at write time (inbox `inbox-add-cli`).
- `evolve inbox add [--file <item.json>]` (`go/cmd/evolve/cmd_inbox_add.go`) files one item through the new `inboxmover.File` / `lifecycle.(*Mover).File`. It writes `.evolve/inbox/<UTC filing time>-<id>.json`, stamps `created_at` when absent, appends a `file` ledger line, and prints whether a lane may take the item by the claim floor's own rule.
- It refuses, with nothing written:
  - a non-object;
  - a missing or blank `id`, `title`, `kind`, `summary` or `fix`;
  - a non-kebab or already-filed `id`, checked across the whole inbox tree, retired items included;
  - a `weight` outside (0, 1];
  - an empty or blank `acceptance`;
  - a mistyped field;
  - a `deps` entry no item backs;
  - a lifecycle-owned field (an authored `route` is allowed only toward the console);
  - a prompt-rendered field the loader would sanitize.

  It never overwrites a file (`os.Link`) and does not HTML-escape. Exit codes: 0 filed, 1 refused, 2 I/O fault, 10 usage.
- `inboxbatch.sanitizeItem` now names the fields it rewrote, and the new `inboxbatch.SanitizedFields` runs that one rule on a copy, so the filing verb reuses the loader's bounds instead of repeating them. `inboxbatch.FilenameStampLayout` (exported) names the file and `inboxbatch.IsConsoleRoute` is the one console-route rule; the verb prints the claim floor's own verdict on the filed file.
- Tests, red first:
  - `TestMover_File_*` (8 tests, including 15 refusal shapes);
  - `TestFile_FilesAnItemTheClaimFloorCanHandToALane`;
  - `TestSanitizedFields_NamesEveryFieldTheLoaderWouldRewrite`;
  - `TestCmd_InboxAdd_*` (4).
- The architecture review blocked the first draft: the filename stamp was a third literal, a filing fault's exit 2 was unpinned, an authored console route was refused, six mutants survived, the owned-field list had drifted and a design note overstated the invariant. All are fixed. Mutation sweeps: 14 of 14, then the reviewer's six survivors plus four new-rule mutants, 10 of 10, killed. The sweep also showed a self-dependency check was dead (an unfiled id cannot satisfy `deps`), so the check was removed. `internal/inboxmover/lifecycle` stays at 100% statement coverage (`cover-strict`).
- `inbox-add-cli` is consumed in this change. Its third criterion, the retrospective's minting through the same writer, is filed as its own item through `evolve inbox add` itself.
## Changed — a deleted comment leaves self-explaining code (comment round 12, 2026-09-30)

- The operator's rule (2026-09-30): "Make sure when we removed comments, code is lean enough to explain itself without comments."
- `docs/conventions/code-comments.md` gains *What a deleted comment leaves behind*. Where a deleted comment said what the code does or what a value means, the same change makes the code say it: an unexported rename, an extracted function or named condition, a named constant, or a small type. It is behaviour-preserving, adds no comment, and never changes the value of a tag, flag, string or error text.
- The batch protocol gains the step *Make the code say it*, and Phase 2f applies it to rounds 1–11.
- Round 12 lands as two commits:
  - the AST-proven comment-only commit: 228 files, 7,820 → 67 comment lines, and 17 new package design-notes pages;
  - a reviewed refactor commit of 82 files. Six agents, one per group, made the code say what about 140 deleted comments had said, for example `porcelainStatusPrefixLen`, `isAtOrUnderAny`, `isReusablePrior`, `withLink`, `configReleaseSinceLastBinaryRelease` and `ciConclusionUnavailable`.
- About 2,500 other deleted comments needed nothing: they were history, restatements, or design reasons now in the notes.
## Added — `evolve inbox route-lane`, the operator's lane route (2026-09-30)

- `evolve inbox batches` planned 0 batches from 183 pending items: 101 were console-owned by their `pipeline-*` kind, 53 by a protected surface and 28 by `route: console`. ADR-0074 lets the operator reopen a heuristic derivation with `route:"lane"`, but no command wrote it. The only way was to hand-edit item JSON, which the operator rule that every control goes through the published CLI forbids.
- `evolve inbox route-lane <id> <reason>` (`go/cmd/evolve/cmd_inbox_route_lane.go`) marks a root item `route: lane` through the new `inboxmover.RouteLane` (`lifecycle.(*Mover).RouteLane`). It stamps the trimmed `routed_reason` and `routed_at`, drops a stale `routed_cycle`, and writes a `route-lane` ledger line. A lane's `claim` then takes the item.
- It judges the item exactly as the claim floor does, and refuses anything that would stay operator-owned. It reads the item once, decodes it with the claim floor's own raw decode (`routingItem`, not the loader's sanitized copy, which cuts a `files` entry at 160 bytes), and judges with `inboxbatch.ConsoleRouted` and the Mover's own lane predicate (`Options.IsProtectedPath`, set to `laneForbidden` as the claim root sets it). The judgment is the admission hook of the one read-judge-write (`updateAdmittedItemJSON`, which `UpdateItemJSON` now calls with a hook that admits everything), so the bytes judged are the bytes rewritten. A declared protected file and an agent-autofiled item (`injected_by` set) exit 1, with the item byte-identical and no ledger line, so the verb cannot write an override the claim floor, triage's breaker or the ship tripwire would refuse. `inboxbatch.RouteLaneValue` is now exported so the verb and the classifier spell the route once.
- Exit codes: 10 for usage (a missing argument, a blank id or reason), 1 for an unknown or claimed id or a refused route, 2 when the inbox could not be read, the item is not well-formed (so its route cannot be judged; it is left untouched), or the route could not be recorded. Run it at a wave boundary, since the item is a tracked file.
- Tests, red first:
  - `TestCmd_InboxRouteLane_ALaneCanNowClaimAPipelineRepairItem` proves `claim` exits 3 before the route and 0 after it.
  - `TestCmd_InboxRouteLane_RefusesADeclaredProtectedFile`, `TestCmd_InboxRouteLane_RefusesAnAgentAutofiledPipelineItem`, `TestCmd_InboxRouteLane_AnUnreadableInboxExits2` and `TestCmd_InboxRouteLane_RefusesAMalformedOrUnknownRequest` cover the refusals, the I/O exit and usage.
  - `TestMover_RouteLane_OpensAPipelineRepairItemToLanes`, `TestMover_RouteLane_RefusesWhatALaneMayNotTake` (including a protected file past the loader's field cut), `TestMover_RouteLane_LeavesAClaimedOrMissingItemAlone`, `TestMover_RouteLane_AnUnreadableInboxIsNotAMissingItem` and `TestMover_RouteLane_RefusesAnItemItCannotJudge` cover the leaf.
  - `TestRouteLane_LetsALaneClaimAPipelineRepairItem`, `TestRouteLane_JudgesWithTheOptionsLanePredicate` and `TestConsoleRouted_RouteLaneRelaxesAPipelineKindForAnOperatorItem` cover the façade and the classifier.
  - A mutation sweep killed 11 of 11 mutants. In the leaf: no admission check, judging the unrouted item, a nil predicate, an inverted decode guard, ignoring a claim, flattening an I/O fault to not-found, no empty-id guard, keeping `routed_cycle`, and no ledger line. In the CLI: dropping the lane predicate, and a refusal exiting 2.
- The architecture review found the first draft judged the loader's sanitized copy while the claim floor judged the raw file, so an item with a protected file past byte 160 of a `files` entry would have been routed and then refused at claim; that is the regression case above. It also found the new root missing from `TestInboxCenterlessRootsArePinned`, and an unreadable inbox reported as a missing item (exit 1, not 2). All three are fixed. Its second pass found that an item the decode could not read was routed unjudged; it is now refused.
- Docs: `runtime-reference.md` (operator commands), the ADR-0074 amendment, and the `cmd-evolve`, `internal-inboxmover` and `internal-inboxmover-lifecycle` package notes.
## Changed — the comment target is zero: exported identifiers' docs are no longer kept (2026-09-30)

- The operator's rule (2026-09-30): code carries **zero comments** beyond the ones something reads (toolchain directives, generated-file headers, `Deprecated:` and `Output:` lines, machine-read markers) and the package doc `docgo` enforces. Until now the convention kept one doc line on every existing exported identifier, and `commentaudit verify` refused to delete one, which left about 11,200 such lines (non-test Go outside `acs/`, measured on main b67e0bb3) out of reach of a comment-only change.
- `commentaudit.Equivalent` (behind `commentaudit verify` and the commit gate's comment-only waiver) no longer refuses a deleted exported doc. `lostExportedDoc` and its six helpers are removed, and `parseShape` returns only the code shape. `TestEquivalent_DeletingAnyDocIsACommentEdit` replaces `TestEquivalent_AnExportedIdentifierKeepsItsDoc`; red first, it failed on exactly the six deletions the old rule refused.
- `internal/commentaudit/stats.go` drops `isNarrative`'s comment, which called a `See ADR` pointer allowed; the convention's `check` bullet already says a bare pointer is not history.
- `skills/refactor/reference/language-notes.md` no longer tells refactors to give extracted exports a doc comment.
- `docs/conventions/code-comments.md`:
  - the Keep table holds only what something reads, plus the package doc;
  - a doc on an exported identifier, and a pointer or path comment, are now under "must not say";
  - the one-line invariant *why* is transitional: it lasts until an inbox item pins the invariant with a test.
- `docs/plans/comment-reduction-2026-09.md` gains Phase 2, "to zero":
  - untouched directories;
  - a sweep of the docs that earlier batches kept;
  - the *whys* turned into lane-sized test items;
  - the per-cycle predicate packages;
  - `comment_floor` set to enforce against regrowth.

## Fixed — a dated quota wall benches its CLI family a day at a time and records the banner (wave 49, 2026-09-30)

- On 2026-09-30 codex's account reported `You've hit your usage limit … try again at Oct 14th, 2026 4:13 PM`. The CLI-health bench recognized only clock (`try again at 6:11 AM`) and relative (`try again in 2 hours`) hints, so the dated wall fell back to the strike-scaled cooldown: 4 hours at 33 strikes, re-probed by the loop's canary about six times a day for two weeks. The bench's `evidence` was also the pane's first line, the pasted prompt (`<artifact-path>…scout-report.md</artifact-path>`), so `.evolve/cli-health.json` never said why codex was benched or until when.
- `ParseResetHint` now reads a dated hint (month name or prefix, optional ordinal, year, clock time) as that local instant, refusing a date already past or one that does not exist (`Feb 30th`). The existing 24-hour cap still applies, so a two-week wall is re-checked once a day and an early lift (a plan upgrade) is noticed within a day. `evidenceLine` recognizes the dated banner, so the bench records the provider's own message and date. One ordered table of hint shapes feeds both the parser and `evidenceLine`, so a new shape cannot reach one and miss the other; the AM/PM conversion is shared (`clockOf`); the cap is named (`resetHintCap`). The `EVOLVE_CLI_HEALTH` row is edited at its source (`flagregistry`) and regenerated with `evolve flags generate`. Filed: `bench-records-the-stated-reset`, so the operator and advisor see the provider's date rather than the capped day.
- Tests, red first: the exact codex banner (capped at a day), a dated wall within the day (its exact instant), a full month name, a past date, an unknown month, an impossible day, and across the year end (January of the printed year, December, an impossible clock time); `NewBenchEntry` on a pane whose first line is the pasted prompt benches a day and keeps the banner; the fuzz corpus gains the dated shapes. Reviews: Go APPROVE; architecture Block (a hand-edited generated flag index, unpinned year and month-bound mutants, a stale doc comment, the shape list in two places) fixed. Mutation sweep 16/16 killed.

## Fixed — Changed Areas reads Markdown list items, so a wrapped or grouped bullet keeps its explanation (eight cycles, 2026-09-30)

- The explanation check read Changed Areas one physical line at a time and took only the text after a path on that same line as its explanation. A builder that hard-wrapped a bullet (``- `a.go` — migrates`` with the rest on the next line), wrote a colon and then sub-bullets, or grouped several paths before one explanation (``- `a_test.go`,`` / ``  `b_test.go` — new tests…``) left each path an explanation of a word or a comma, and the floor sent the build back for a what/why it had already written. Cycles 1707, 1730, 1735, 1737, 1760, 1762, 1765 and 1772 each paid a Builder correction round to it; 1772's re-dispatch ran at 131% context fill.
- `changedAreaItems` reads list items: continuation lines (unindented, or indented fewer than four spaces) and sub-bullets join the item, and the next `- ` bullet at the same or a lesser indent, or unindented text after a blank line, ends it. `splitChangedAreaItem` gives every path of a group (joined by `,`, `and`, `&` or whitespace) the shared explanation, and a span joins the group only when it covers a Build-diff path, so a backticked identifier (``- `a.go`, `checkAll` — splits…``, `` `helper`-based``) stays part of the first path's explanation and is never cited. A nested bullet is its own entry, checked against the diff as before, when its first span names a diff path or is shaped like one (holds a `/`), so a nested invented path is still reported; a sub-bullet opening with an identifier (1772's repair round nested `` - `runGateSet`, which… `` under its path) continues its item. A bullet that is not nested is always an entry and must name Build content. Prose outside any item is never an entry. The explanation floor is named (`minExplanationBytes`) instead of a bare `10`.
- Unchanged floor: every path an item names still passes the diff check, `minExplanationBytes` and the per-material-path check; a group's explanation is the text after its last path, and a short one never borrows the next block's text. The contract and the auditor persona state the rule; the builder persona is unchanged (one bullet per path stays the asked-for form, this is the recovery rung).
- Known limit, filed as inbox `reportdoc-indented-continuation-is-not-code`: the shared section reader (`reportdoc.visibleLinesIdx`) hides a line indented four spaces or by a tab as code before this reader runs, so such a continuation is still unseen. That reader is shared with verdict parsing, so its fix is its own component.
- Tests, red first: four wrap shapes plus CRLF, four group shapes (the 1772 and 1735 texts among them, and a `./`-relative follower that must normalize), a group whose explanation is too short, four prose-span shapes (an identifier, a hyphen-glued identifier, a file then its symbol, a path outside the diff), item boundaries including a nested item's sibling, an unclosed backtick, nested path bullets, prose outside an item, `leadingCodeSpan`'s contract, and one end-to-end `validateDocument` run through the real section extraction. Reviews: Go BLOCK (a prose span glued to a hyphen was grouped) and architecture FIX_THEN_MERGE (three unpinned mutants, the reportdoc indent limit, contract over-claims, copied values) fixed by grouping on Build content; the architecture re-review's unpinned follower normalization is pinned; a replay of 1772's rejected repair-round document through the real section reader then showed nested identifier sub-bullets would become phantom entries once deeper lines are visible, so a nested bullet opens an entry only on Build content or a path's shape (the Go re-review caught that Build content alone let a nested invented path go unreported, which main reports). Mutation sweep 25/26 killed; the survivor (`appendItem`'s nil guard) is equivalent, since an empty item names no path.

## Fixed — a byte-identical carry ships on its audited verdict; no carry had ever shipped (cycles 1766, 1768, 1772, 2026-09-30)

- ADR-0105's carry lets a lane whose audited change rebases byte for byte onto a peer's landing ship without a second audit. It never did. `identityCarryForward` proved the rebase identical and wrote its record every time (`rebase is byte-identical: the audited verdict carries`), and then ship's `verifyAuditBinding` refused it: its predicate-execution tree check (older than the carry) compared the tree ship holds with the audited tree directly, while the carry had been wired only into `auditBindingSatisfied`, the rule the pre-commit and post-push checks use. Cycles 1766, 1768 and 1772 each failed `SHIP_AUDIT_BINDING_TREE_MISMATCH` with exactly their record's `audited_tree_sha` and `tree_state_sha`, and each paid a deep re-audit.
- `verifyExecutionTree` (extracted from `verifyAuditBinding`, behavior unchanged first; it reads the bound tree from `opts.internalAuditBoundTreeSHA`, the value the rule compares, never a second copy) now applies the one binding rule to a drift: an identical tree binds as before, a drift binds only when `auditBindingSatisfied` explains it (here, a re-proven carry: inbox consumption is recorded after this check runs), and a tree that cannot be taken still refuses. The accepted drift is logged with the carry that explains it.
- ADR-0105 named a proof test for this path, `TestFleetRebase_Cycle1701Shape_ShipsWithoutBuildOrAudit`, that was never written; it now names `TestVerifyAuditBinding_ShipsACarriedRebaseWithoutASecondAudit`, which drives the real binding on a carried rebase in the live ordering (the peer lands, the audit binds, the lane rebases), with the record written by a test helper; a test through the real recovery and ship's later stages is filed (inbox `carry-recovery-to-ship-end-to-end-proof`), as are the three binding sites that still compare directly (inbox `carried-ship-resume-binding-sites-skip-the-rule`). The ADR, the design doc (§5.8 C7) and the ship package doc record it.
- Tests: the wiring test above (red on the old check with the live error), a re-proven carry accepted, a drift no rule explains refused, a carry whose record names another audit refused, and a tree that cannot be taken refused. Mutation sweep 5/5 killed.
## Fixed — a Changed Areas citation that covers diff paths is accepted; a material path still needs its own entry (cycles 1765 and 1768, 2026-09-30)

- The explanation check read each Changed Areas path literally and rejected any that was not an exact Build-diff path. Builders keep citing their predicate package as a directory or glob: cycle 1765 cited `go/acs/cycle1765` (wave 44) and cycle 1768 cited `go/acs/cycle1768/*_test.go` (wave 45). Each citation named real Build content, yet each cost a full builder re-dispatch through the correction ladder: a format miss handled as a logic defect.
- `changedAreaFailures` (extracted from `validateDocument`, which drops from 62 to 44 lines, so its size-ratchet allowance is removed) accepts a cited directory, glob or brace list when it covers at least one diff path (`coversAnyChangedPath`). A citation covering nothing is still "not in the Build diff".
- Narrower than the inbox item proposed: a pattern never credits the material paths it covers. Every material path still needs its own exact entry, so one broad glob (``- `go` — changes``) can never stand in for the per-path explanations the contract exists to force.
- Pattern rules: a literal path is tried first (so a real path whose name holds braces, like `{{cookiecutter.slug}}/app.py`, stays citable); `dir/...` and `dir/**` are directory citations; a brace list with an empty alternative, a nested brace, an unclosed brace or more than 64 expansions covers nothing, so a typo such as `go/{cmd,internal,}` can never widen into the bare prefix `go/`, a nested list such as `go/acs/{x,{cycle1768,y}}/…` can never mis-parse into a real diff path, and a builder-authored line can never cost 2^N expansions. The auditor persona, the contract and the design doc state the same rule.
- Tests, red first: directory, glob, `/...`, `/**` and brace citations covering diff paths pass (the covering alternative placed later, so a first-alternative-only mutant dies); a glob covering nothing, a string prefix that is not a directory (`go/acs/cycle176`), and each malformed brace list fail with the same message; a material path under a covering glob still fails; the expansion bound is pinned directly. Reviews: Go BLOCK (the empty-alternative collapse; unbounded expansion) and architecture FIX_THEN_MERGE (two unpinned mutants, the literal-brace regression, `/...`/`/**`, the prose copies) fixed; the Go re-review's nested-brace finding (the old fixture passed only by luck) fixed red-first with a fixture whose mis-parse names a diff path. Mutation sweep 9/9 killed. Consumes inbox `changed-areas-accepts-a-covering-dir-or-glob`.
## Fixed — the build floor runs an added test package under its twin's budget, ship's added-test backstop, not 120 s (cycle 1763, 2026-09-30)

- Wave 43's lane 1763 wrote a predicate that runs its own `-count=50 -race` stability bound, as its inbox item's acceptance asks. The build floor's tagged self-check ran added test packages with `-timeout 120s`, while its declared twin, ship's added-test backstop, runs the same package with `-timeout 20m`. The floor killed the predicate, so the lane edited `core/phase_bindings_selfcheck.go` to 480 s. It then failed the explanation's completeness check on that unexplained pipeline edit; a lane had been pushed into editing a gate by two budgets for one run. With 1761, 1762 and 1764 it was the fourth consecutive FAIL.
- `addedtests.PackageTimeout` (20 min), in the package both twins already share for seed and grouping, is the one per-binary budget: ship's `repoContractTestTimeout` aliases it (its cycle-1679 rationale unchanged), and the floor's tagged run builds its arguments with it (`taggedTestArgs`). The rule: the floor is never stricter than a later gate running the same package. The untagged self-check of changed packages keeps `-timeout 120s`, the alarm ADR-0083 chose for a slow ordinary package; an added test package has no prior runtime to regress from.
- Review (architecture, Block → fixed): the first version took the budget from `ciparitygate`'s `ACSDurable`, which bounds the `./acs/regression/...` tree, not a cycle's package; that constant is gone and the gate's own literal is restored. Tests: the floor's whole argv is pinned (a dropped `-count=1` would have survived a flags-only check), and `addedtests` pins that its budget parses and exceeds Go's 10m default; each kills its mutant.

## Fixed — continuation adoption archives the ancestor's own predicate package, so a retry never runs a contract it does not own (design A2, cycle 1764, 2026-09-30)

- Wave 43's lane 1764 adopted cycle 1761's continuation and failed at the build handoff floor: the snapshot still held 1761's own predicate package `go/acs/cycle1761`, the floor's added-test self-check ran it, and its allow-list fence diffed against 1761's base, so every file main had gained since (the release, the gc landing, the dossiers) read as the lane's. No build could fix that; the item would have failed on every retry until quarantine. With 1761 and 1762 it was the third consecutive FAIL, so the loop stopped for this fix.
- `prepareAdoptedTree` (extracted from `adoptContinuationAfterTriage`, which shrinks from 61 to 57 lines) archives the ancestor's two kinds of cycle-owned artifacts under one retention rule, as design A2 planned: its unpublished explanation records (unchanged) and, new, `explanationdocs.ArchiveSupersededPredicatePackages`, which moves every `go/acs/cycle<M>` the diff adds (M is not this cycle, and the package is absent at the base) into `docs/private/research/archived-<date>/superseded-predicate-packages/` and stages the move. `acssuite.AncestorCyclePackages` selects them with the package's canonical cycle-number parser (`canonicalCyclePackageNumber`: no leading zero, no `cycle0`), and core injects it, so `explanationdocs` takes no dependency on the test-running machinery. Both archives share one dated root (`archiveDateDir`) and stage the move in the same pass.
- The fix sits at the source rather than in the floor, because two consumers run every added tagged test package: the build floor and the ship backstop's repo-contract pack. The ACS suite already scoped a cycle to its own package plus the curated regression ("never every historical cycle"); now the diff they read agrees.
- Tests, red first: an end-to-end adoption test whose Build-time probe finds the ancestor package archived and gone from its path while the prior work stays; the selector table (own cycle, regression, redteam, a look-alike, dedup, order); the archive on a real repo (a package already on main and this cycle's own are never moved). Review: the Go reviewer's BLOCK (the rename source lacked its sibling's real-directory check) and the architecture reviewer's FIX_THEN_MERGE (the staging step had no test; the new `explanationdocs → acssuite` edge; a third cycle-name parser) are fixed; a test now reads the index (`--no-renames`) and fails if the move is not staged. Mutation sweep 9/9 killed, 0 build failures. Declined: computing the dated root once for both archives (a sub-second UTC-midnight split, not worth widening two exported signatures). Filed: the comment floor still counts the archived `.go` files (shadow stage, not a regression).

## Fixed — the TDD persona bans the allow-list scope fence that cycle 1761 could never satisfy (2026-09-29)

- Wave 42's lane 1761 (`sizeratchet-shrink-eval-validators`) sealed FAIL after two build-floor correction rounds. Its TDD predicate `TestC1761_009_OnlyTargetPackagesTouched` allow-listed only the three target packages and `go/acs/cycle1761/`, so it went red on files the pipeline itself requires the cycle to write: the Build's explanation document (`docs/explain/builds/cycle-1761-*`) and TDD's own eval (`.evolve/evals/<slug>.md`). Build cannot edit a TDD predicate, so the corrections could not converge (the retro's root cause, confirmed from the lessons digest). Cycle 1760's fence, a deny-list, passed.
- `agents/evolve-tdd-engineer.md` Predicate Reliability (kept in compact mode) gains a sixth banned shape: a scope fence that allow-lists only the task's packages. Instead, name what must not change (a deny-list: `go/internal/sizeratchet/offenders.json`, protected surfaces), or exempt the cycle's required artifacts: `docs/explain/builds/cycle-<N>-*`, `.evolve/evals/<slug>.md`, `go/acs/cycle<N>/`, `.evolve/inbox/`.
- Filed: `build-floor-routes-tdd-predicate-defects-to-tdd`. The build-floor correction re-dispatched Build twice for a defect only TDD can fix; a byte-identical, TDD-owned predicate failure should route to TDD, or to the operator.

## Added — `evolve gc` releases what finished cycles leave behind: sockets, orphan processes, code trees, run dirs, the Go build cache and pipeline temp files (2026-09-29)

- On 2026-09-29 the host carried about 130 GB no finished cycle needed: 56 GB of pipeline temp artifacts (1,467 leaked `acs-*-bin-*` dirs, 628 `go-build*` work dirs of killed builds, fixture dirs, killed tests' `t.TempDir` dirs, 2,383 release dry-run journals), a 60 GB Go build cache growing about 40 GB a day, 87 cycle worktrees (7.3 GB, cleaned by hand that morning), 79 dead tmux socket files, and 14 CPU busy loops cycle 1762's auditor started to reproduce a flake under load and never stopped (orphaned, 47 minutes at ~85% CPU each, load average 118; they starved the cycle's own host predicate run). `evolve gc` could see almost none of it.
- **Sockets.** `swarm.ExecKillServer` now also removes a dead server's socket file. `kill-server` on a dead server is a no-op that tmux never cleans up after, so every sweep "reaped" the same 79 sockets again.
- **Liveness.** A cycle whose closeout dossier exists (`dossier.ClosedOut`: `knowledge-base/cycles/` or `.evolve/dossiers-pending/`) no longer counts as live through its `run.json`'s `active_worktree`, a pointer a sealed run keeps forever. That pointer is why the worktree sweep had planned nothing for any of the 87 trees.
- **Processes.** `gc.ReapFinishedCycleOrphans` sends SIGTERM to a process only when it is an orphan (ppid 1) and its working directory (`lsof -d cwd`) lies inside a cycle worktree that is not live by the package's liveness owner (`isLive`) and is gone or closed out. A dossier alone is not enough: a resumed cycle can hold its FAIL dossier while it runs again in the same tree. Phase agents run with their cwd in the cycle's worktree, so that is an ownership proof: a live cycle's tree, an operator shell `cd`'d into an old tree (not an orphan) and anything outside `.evolve/worktrees/` are never touched.
- **Code trees.** A dirty or unmerged tree whose cycle closed out at least `gc.worktrees.salvage_after_hours` ago is planned `salvage-remove`. Apply re-checks liveness in the same pass that re-checks every target, writes `.evolve/operator-salvage/<leaf>/` (`HEAD` sha and branch, `uncommitted.patch`, `untracked.tgz`), re-checks liveness once more, and only then runs `git worktree remove --force`. The branch is never touched, so the lane's commits and a continuation's `snapshot_sha` (adoption seeds from the SHA, not the directory) survive, and a failed salvage keeps the tree. `operator-salvage` was already on the `salvage_ttl_days` retention ladder.
- **Go build cache.** `gc.TrimGoCache` removes `<hash>-a`/`<hash>-d` entries in the cache's hex shards that were unused for `gc.go_cache_ttl_hours`. That is Go's own `cache.Trim` criterion with a policy horizon in place of Go's fixed 5 days, which the loop outgrows: every cycle builds in a new worktree path, and a package's cache key includes its directory. The cache root's own files and the fuzz corpus are never touched. A build that looks an entry up at the instant it is trimmed can see a miss or a missing output file, the same race Go's own trim has; an entry unused for a day is in no running build's working set.
- **Temp artifacts.** `gc.ReapPipelineTemp` removes top-level `os.TempDir()` entries older than `gc.temp_ttl_hours` whose names have one of the pipeline's exact shapes (`go-build<N>`, `acs[-cycle]<N>-*`, `cycle<N>-*`, `release-pipeline-dryrun-*.json`). No other program's entry matches. Go's `t.TempDir` shape (`Test*`) was dropped in review: it is not an ownership proof in a shared temp dir, where any project's or program's `Test…` entry would match, and those entries totaled 0.1 GB.
- **Run dirs.** The operator run now applies the existing run-dir retention ladder (`gc.Discover`/`Plan`/`Apply`), which until now only the loop's per-cycle hook ran, in shadow mode by default. An item it cannot delete or archive fails the run (exit 1) rather than being reported and passed over.
- **Report.** A real run ends with `disk free A → B (net released C)`, a measurement rather than a sum of estimates. Every step prints its counts, and `--dry-run` prints what it would release and mutates nothing.
- **Config.** The checked-in `.evolve/policy.json` gains a `gc` block: `go_cache_ttl_hours` 24, `temp_ttl_hours` 24, `worktrees.salvage_after_hours` 24, `runs.delete_after_days` 14. Each knob's zero value means never. `gc.mode` stays unset (shadow), so the loop's hook still only plans, and an operator `evolve gc --project-root <plane>` applies. The dry run of these values on 2026-09-29 reported 61.1 GB of temp artifacts and 21.1 GB of cache to release.
- **Release dry run.** A release dry run journaled to `<tmp>/release-pipeline-dryrun-<pid>.json` and ignored `JournalDir`, so every process that dry-ran added a file (2,383 of them). It now overwrites one `release-pipeline-dryrun-<version>.json` per version (in `JournalDir` when set), and still never writes into the repo.
- **Tests.** The `evolve gc` command tests point `TMUX_TMPDIR` at a temp dir. They used to run the real reapers against the host's socket dir, which after the socket fix deleted real files.
- Review (architecture, Block → fixed): the process reaper decided "finished" from the dossier alone and now defers to `isLive`; salvage-remove joined the one TOCTOU pass and re-checks liveness before `--force`; the sweeps take injected effects (`remove`, `removeAll`, `kill`) instead of `apply bool`, and the command's steps are methods of one `gcRun` value; the loop hook and the operator share `planRunDirGC`; `operator-salvage` has one path helper; a test pins that the cache's `fuzz/` corpus survives. Not taken: exporting the dry-run journal's name to the temp matcher, because a drift only stops gc from matching (the safe direction) and the journals are now bounded per version.
- Rejected: the inbox item's `go clean -cache` above a size, gated on no live loop lease. Clearing the whole cache discards hot entries, and waiting for a lease-free moment rarely happens on a loop host. The age trim is safe mid-wave and releases exactly the entries nothing used.
- Consumes inbox `gc-pipeline-temp-and-go-cache`. Filed from this work: the loop-batch tests still run the real socket reaper against the host, and the Go cache's per-worktree-path keying remains the growth driver.

## Fixed — the host records how triage ended the cycle at the C1 chokepoint: a claim-failed lane charges its pins, a planned no-work end is NO_WORK (R1c, 2026-09-29)

- Two host terminations escaped ADR-0044's C1 invariant (every terminal path records a ship PASS, a salvage or an abort reason). The claim-failed FAIL and the planned no-work end both left triage's own PASS as the last entry of `phase-timing.json`. Cycle 1757's claim-failed lane therefore classified `integrity-breach` (replayed from its workspace): `FailureInputsFor` marked it system-level, the drain released the item and bumped nothing, and the FAIL closeout's committed set was the empty `top_n` anyway. A 2026-08-10 review had read an empty `top_n` as "declined", before F30 made a decline an answer. Cycle 1758, wave 40's planned no-work sequential cycle, paged `FAILED_UNEXPLAINED` and self-filed a HIGH inbox defect (weight 0.8) that the next wave's lanes would have drawn.
- `recordPlannedNoWorkOutcome` now records each ending through the new `Recorder.RecordEnding` (`recordTriageEnding`), as a triage entry with the host's verdict and the termination reason as its abort reason. The claim-failed entry is FAIL. It carries triage's own error diagnostics first, so a coded triage refusal is never shadowed, then, in a lane, `TRIAGE_SCOPE_UNANSWERED` (new, task-level in `RefusalDisposition`) naming the unanswered pins. The no-work entry is SKIPPED with `triage-empty-commitment`. An ending is not a phase run: `RecordEnding` appends the C1 entry and nothing else. It leaves `PhasesRun` and the usage sidecar alone and emits no `phase.outcome`, since a planned no-work end would otherwise print a `phase.aborted` WARN and the seal already reports the ending. The claim-failed reason string moved to `cyclestate.CycleTerminationTriageClaimFailed`, and core aliases it.
- The existing readers do the rest. `cycleclassify` reads the lane's code as a task-level refusal. A sequential cycle has no pin and no code, so its classification and batch halt are unchanged. `cyclehealth.ClassifyOutcome` explains the claim-failed FAIL by its abort reason and gains one arm, `NO_WORK`, for the no-work reason, beside DEFERRED's.
- `cycleoutcome.ApplyFailure` charges `committedset.Unanswered(workspace)` when triage committed nothing: the lane pin minus the decision's answers, in pin order. A deferred pin is owed and charged, an answered pin is not, and an absent decision owes the whole pin, as before. `core.unansweredClaimableWork` reads the same projection and the same `committedset.LanePin`.
- Found on the way: a zero-attempt record (`recordChokepointEscape`, a missing runner) rewrote the phase's `<phase>-usage.json` with zero cost, erasing the dispatch usage `cyclecost` sums. `Recorder.Record` now leaves an existing sidecar to a zero-attempt record, and still writes one when none exists (`phaseoutputs.Survey` counts its presence).
- Rejected in review: reading the cycle's seal from `signals.ndjson`. The Signal Center's rule is that listeners observe and never decide (its §3, review 10). The stream may drop or delay events by design, and a probe renamed the producer's field with every test green.
- Tests, red first. Core runs real composed cycles and hands the workspaces to the real readers: `TestCompleteCycle_AClaimFailedLaneRecordsItsUnansweredScopeAtC1`, `…APlannedNoWorkEndIsRecordedAtC1AsNoWork`, `…ASequentialClaimFailureKeepsItsClassification`, `…TriagesOwnRefusalStaysFirstOnTheClaimFailedRecord`, and `…ALanesOwnTriageRefusalOutranksItsScopeCode`. Outcome: `TestRecorder_RecordEnding_LogsTheEndingWithoutCountingARun`, `TestRecorder_Record_AHostRecordNeverErasesTheDispatchedUsage` (it erased $1.25), and `…AHostRecordOfAnUndispatchedPhaseStillLeavesItsSidecar`. Also `TestClassifyOutcome_TheRecordedPlannedNoWorkEndIsNoWork` (four cases), the `Unanswered` tests, `TestApplyFailure_APinAnEmptyDecisionLeftUnansweredIsCharged` (three cases), and `TestFailureCloseout_ACycle1757ShapedLaneChargesItsPinnedItem` (`FailureInputsFor` → `ApplyFailure`).
- `TestApplyFailure_EmptyCommittedDoesNotFallBack` pinned the retired belief; its fixture declined nothing, so it is now `TestApplyFailure_APinTriageAnsweredIsNotCharged`. `TestRunCycle_EmptyTriageCommitmentIsPlannedNoWork` still pins `PhasesRun = [scout triage]`. Eight mutants killed, none by a build failure (including the swapped diagnostic order the architecture confirmation round found alive).
- Docs: design §5.7 R1c, §7.8, §8, history; the factory-rules failure table; the 2026-09-15 incident's F30 addendum. Inbox: `claim-failed-closeout-charges-unanswered-pins` consumed.

## Fixed — a wave-boundary sync blocked by plane-side inbox files names them, with a cause-neutral remedy (2026-09-29)

- A console route, or any lifecycle stamp the plane writes (a failure bump, a quarantine), rewrites a tracked inbox item in the plane. When origin later changes the same item, the wave-boundary `merge --ff-only` refuses, and the loop plans against a stale main with a WARN that only relayed git's raw stderr. `fastForwardMain` (extracted from `syncMainFromOriginAtWaveBoundary`, which the new branch pushed past the 50-line limit) now also names the blocking inbox files: `N plane-side inbox file(s) block it: <paths> — discard the plane copy once origin holds everything it carries, then run evolve sync-main`.
- The set comes from git plumbing, not the refusal's text (`blockingInboxFiles`): the plane's own changes (`git status --porcelain -z --untracked-files=all`) intersected with origin's (`git diff -z --no-renames --name-only HEAD origin/main`), both under `.evolve/inbox/`. A plane stamp origin never touched is not named, a file the plane deleted (porcelain ` D`: the mover moved it away, and git does not count it as blocking) is not named, a non-inbox file is never named, and the result is locale-proof.
- The architecture review caught the first version, which parsed stderr and told the operator to land every file through a curation PR. That remedy would have resurrected an item origin had just retired to `consumed/`, and it called an untracked file a "stamp".
- `docs/operations/runtime-reference.md` documents the console route at a boundary (route with `evolve inbox route-console` in the plane, land the item as a data PR, discard the plane copy, `evolve sync-main`), with a per-cause table (a curation, a retirement, a plane-created file) and an honest note that steps 2 and 3 are plain git.
- Tests: `TestSyncMainAtWaveBoundary_NamesTheInboxStampsThatBlockIt` (real git: a curated item and a retired item are named, an untouched stamp is not) `TestSyncMainAtWaveBoundary_NonInboxDirtNamesNoInboxFile` and `…APlaneCreatedFileIsNamedAPlaneDeletedOneIsNot`. The existing `…DirtyTrackedFileWarnsBlockedNotDiverged` now also asserts no inbox line. Six mutants killed (two found by the confirmation round).
- Inbox: `console-route-stamps-block-wave-sync` stays open, with `notes` and a trimmed `acceptance`: route stamps out of tracked files (or an ADR saying why not), a CLI home for land-and-discard, and m7.

## Fixed — the carry's composed-tree gates run in the CI environment, so lane state no longer fails them (C6, 2026-09-29)

- A byte-identical fleet rebase ships on the verdict it already earned (ADR-0105 carry), but only after the composed-tree gates (`compile`, `test`, `acs`, `apicover`) pass on the rebased tree. `runComposedGates` (`go/cmd/evolve/cmd_composition_wiring.go`) ran `make -C go <target>` with the lane's full environment and threw the output away. The audit's own CI-parity gate scrubs exactly that environment ("EVOLVE_*, BRIDGE_* and tmux state must not reach env-sensitive tests").
- The result: `identity carry declined: composed-tree gates not green: [test]` 14 times in waves 27–41, against 18 carries. Every decline paid a deep-tier re-audit (about 8 minutes) that then passed, and the log never said why. Wave 41's cycle 1760 was the latest.
- Reproduced on a clean main: `make test` with a lane-shaped environment exits 2. The failures are deterministic per test whenever `EVOLVE_WORKTREE_ROOT` reaches the run:
  - `TestFlagsGenerateThenCheck_RoundTrip` and `TestFlagsCheck_DriftExitsTwo` resolve their root through `sourceRoot()`, which prefers the lane's `EVOLVE_WORKTREE_ROOT`. So `generate` could also write `docs/architecture/control-flags.md` into the lane worktree, the very tree the carry was proving.
  - `TestPredicateEnv_AllBranches` builds its environment from `os.Environ()` and inherits the variable.
- The same target is green in the CI environment. Why 18 carries passed while 14 declined is unverified; the likely split is whether the variable was in the orchestrator's environment at carry time.
- One allowlist now serves both gate runners. `ciparity.CIEnvAllowlist` and the pure `ciparity.CIEnv(environ)` moved out of `ciparitygate` (`tierEnvAllowlist`/`cleanEnv`; the allowlist golden is unchanged), and each composed gate runs with `ciparity.CIEnv(os.Environ())`.
- A failing gate now prints `composed-tree gate <name> (make <target>) failed in <worktree>: <err>` and the last 20 lines of its own output. The output is held in a 64 KiB tail (`tailWriter`), not the whole run. `composedGatesTo(log)` replaces the runner, so the log sink is a parameter rather than a package variable.
- Tests, red first:
  - `TestRunComposedGates_RunsInTheCIEnvAndNamesAFailingGate`: a fixture worktree whose `test` target fails when `EVOLVE_LANE_PROBE` reaches it, whose `build` target needs `HOME` (an emptied environment fails), and whose `apicover-enforce` fails with a named test.
  - `TestCIEnv_KeepsOnlyTheAllowlistInItsOrder` and `TestOutputTail_KeepsTheLastLines`.
  - Seven mutants killed: the inherited or emptied environment, the tail taking the head, the byte cap keeping the oldest bytes, the worktree dropped from the header, and two more.
- Follow-ups filed: `composed-gate-decline-coded-signal` (one coded Signal Center event per decline, with the failing gate's detail) and `flags-and-predicate-tests-hermetic-env` (the tests stop honoring a lane's `EVOLVE_WORKTREE_ROOT`, so a lane's own `go test` stops going red on it).
- Docs: design §5.8 C6 and the history.

## Fixed — every agent-graded audit FAIL's repair round is briefed with the failure block's defects, without the "findings artifact unreadable" WARN (2026-09-29)

- The repair brief (`composeRepairBrief`, `go/internal/core/repair_brief.go`) read the audit's reasons only from `audit-fail-reason.json`. A runner gate writes that file (`persistFloorFailReasons`), and since the doc-only landing (#714, logic-first design §5.14, X2a) so did the explanation-needs-correction route. For every other agent-graded audit FAIL the file was absent, so each tdd/build dispatch of the round logged "continuation: findings artifact … audit-fail-reason.json unreadable", and the brief carried the report's findings table alone, not the verdict sentinel's failure-block defects, which are the auditor's own list.
- One reader now serves every consumer of the audit's reasons. When a runner gate diagnosed the FAIL, `auditRejectionReasons` returns the runner's reasons, with today's rendering and precedence. It reads them from `CycleState.AuditFailReasons`, the carrier the coherence floor and the X2a classifier already ask, not from the forensic file, so a leftover record of another phase or a failed best-effort write cannot decide the slot. Otherwise it returns the defects of `phasecontract.ReadFailureBlock(workspace, "audit")`, read from the sentinel itself, under `audit defects (the verdict's failure block, class <class>)`. The brief no longer reads `audit-fail-reason.json`, so nothing there WARNs. A defect that restates a finding the brief lists (its title, id and title, or id, severity and title, compared on letters and digits) is dropped. A defect that restates a LOW finding or one past the brief's eight is kept, because the brief does not list those. `briefedFindings` states which findings the brief lists, once, for the dedupe and the renderer. `renderFailReasons` is the one rendering of a reason list, shared with the continuation's findings reader. A runner-downgraded FAIL's brief is byte-identical to before; this was checked against an archive of `origin/main`.
- The findings header ends in "the gate reasons above are their symptoms" only when the runner's reasons lead the brief. Above the failure block's defects, and in the standing-findings brief (ship-error recovery, a retro-routed re-entry), where the clause was untrue, it now reads "fix THESE".
- The X2a record is retired. `recordExplanationCorrection` and `recordExplanationRound` copied an explanation-only FAIL's defects into `audit-fail-reason.json`. The brief now reads the same defects from the failure block, so the copy is deleted, and `retry@explanation` writes nothing. The route, the narrowed envelope, the Build re-entry and the re-author scope are unchanged. `audit-fail-reason.json` is again the runner gate's deterministic record, as the dossier, the dashboard's gate-reasons panel and audit calibration read it. The failure digest's fallback still writes it too.
- What changes for the failure digest: a re-author Build that dies before the re-audit is now digested from its own failure (`build|…`), as every other agent-graded FAIL's repair round already was. Before this change the record gave it the audit's document defects (`audit|…`). A runner-downgraded FAIL keeps today's behaviour: its gate record outranks the dying phase's error in `ensureFailureDigest`, so a death inside its repair round is counted against the gate failure the round was repairing. The dying phase's error is not lost: the phase-error path hands it to the retro as `failure_error`, and an abnormal exit seals it as the cycle's termination reason. The design states why the gate record keeps precedence (§5.14).
- The retry adjudicator's prompt (`bridgeRetryAdjudicator.composePrompt`) reads the same reader. It is not wired in production, so no live dispatch changes, and the two readers can no longer drift apart.
- Tests, red first: `TestComposeRepairBrief_AnAgentGradedFailBriefsItsFailureBlockDefects` (the WARN, then no defects), `TestComposeRepairBrief_ADefectTheBriefedFindingsCarryIsNotRepeated`, the loop-level `TestRepairRoundDispatch_AnAgentGradedFailBriefsTheDefectsWithoutAWarn` (the WARN on the round's dispatch), `TestRunCycle_AReauthorBuildThatDiesIsDigestedFromItsOwnFailure` (fingerprint `audit|unknown|…` before, `build|…` after), `TestDecideAfterAuditFail_AnExplanationOnlyFailBriefsItsDefectsWithoutARecord` (replaces `…RecordsItsFindings`), `TestBridgeRetryAdjudicator_PromptCarriesTheAuditsReasonsFromTheOneReader`, `TestComposeRepairBrief_TheRunnersReasonsNotTheWorkspaceFileDecideTheGateSlot` and `TestComposeRepairBrief_TheFindingsHeaderNamesGateReasonsOnlyWhenTheyLead`. Mutation pins: `TestComposeRepairBrief_TheDedupeCoversExactlyTheBriefedFindings` and `…AFailureBlockEveryDefectOfWhichIsBriefedAddsNoSection` kill four mutants of the dedupe (the cap, the title form, the empty-section guard, the case fold). Each was applied and run: it survived the first tests and failed these. Guard: `TestComposeRepairBrief_AGateRecordKeepsTodaysBriefOverTheFailureBlock` pins a runner-downgraded FAIL's brief byte for byte. Fixtures that wrote only the file for the gate slot now downgrade the audit through `persistFloorFailReasons`, the production writer. The X2a route stays pinned by its loop test (TDD once, Build twice, the second Build scoped and briefed; the brief assertion now names the defect as the failure block states it) and its route tests. `explanation_correction_record_test.go` is deleted with the functions it tested.
- Reviews: code-simplifier, no edit. Combined code and Go review: PASS with one MEDIUM (an unreadable record WARNed and emptied the slot), resolved by the brief no longer reading the file. Architecture review: FIX_THEN_MERGE (0 CRITICAL, 1 HIGH, 2 MEDIUM, 3 LOW). The HIGH (four surviving dedupe mutants), both MEDIUMs (the header, the gate slot's two homes) and two LOWs (the briefed selection stated twice, doc inconsistencies) are fixed. One LOW is declined: moving the dedupe predicate into `reportdoc`. The compared forms are the brief's own rendering, and `reportdoc.FindingKey` is a looser lead-clause identity that would drop precise defects. The re-review approved (MERGE; 0 CRITICAL, 0 HIGH). Its two optional LOWs are taken: "a runner gate diagnosed this FAIL" is named once, `runnerDiagnosedAudit`, for the X2a classifier, the gate slot and the header lead, and a stale comment clause is deleted.
- Inbox: `repair-brief-reads-the-failure-block-for-every-agent-graded-fail` consumed.

## Fixed — a protected card that empties a single-item lane answers for the lane's pin, whatever triage named it (cycle 1757, 2026-09-29)

- Cycle 1757 (wave 39) sealed FAIL right after 1756 shipped. Its lane was pinned to `goal-text-has-no-selection-authority`; the scout named the work `goal-text-selection-authority`, triage copied that slug as its only top_n card, and the card's files reached `go/internal/loopwave/loopwave.go`. The host's protected-surface route (ADR-0106 R1) moved the card into `escalate_block` under the alias, so `top_n` emptied while the pinned id stayed unanswered: `unansweredClaimableWork` reported claimable work, the lane ended `triage-empty-commitment-claimable-work` (FAIL), and the item was neither routed to the console nor bumped (the committed set was empty), which left it pending at weight 0.81 for the next wave.
- Card ids that differ from the pin are normal: triage splits a pinned item into sub-task cards, and 23 of 140 lane decisions between cycles 1604 and 1757 named ids outside their pin. When routing leaves `top_n` empty, `routeProtectedCards` now also escalates the cycle's one bound item, if there is exactly one and the decision has not answered it (`withBoundItem`, `go/internal/phases/triage/protected_route.go`). The binding is `committedset.Committed`: the lane's pin, which already outranks triage's labels, minus what triage deferred. The item's reason is the first routed card's, suffixed `(the lane's item, answered for by routed card "<id>")`, so its `routed_reason` names the card that caused the route. The existing F30 machinery then ends the lane as planned no-work, and `ApplyNoWork` routes the item console-manual.
- Unchanged: a lane that keeps any committed card (the item is still being worked); a pin triage deferred (owed work, so the lane keeps its claim-failed end); a sequential cycle (its binding is the `top_n` the route just emptied); and a lane pinned to several items (a card cannot be attributed to one). That last shape still charges nothing on its FAIL; inbox `claim-failed-closeout-charges-unanswered-pins` is filed for it.
- Tests, red first: `TestRouteProtectedCards_AnEmptiedSingleItemLaneAnswersForItsPin`, `TestRouteProtectedCards_ThePinIsAnsweredOnlyWhenTheRouteEmptiesASingleItemLane` (four cases), `TestRouteProtectedCards_APinThatIsTheRoutedCardIsEscalatedOnce`, `TestWithBoundItem_TheItemJoinsOnceWithTheFirstRoutedCard` (first card's path and id, the caller's slice untouched, no duplicate), `TestTriageClassify_APinTriageDeferredStaysOwedWork`, and `TestTriageClassify_AnAliasCardOnAProtectedSurfaceAnswersTheLanesPin`, which replays 1757's decision through `router.Digest`. The downstream half keeps its pins (`TestRunCycle_LaneThatAnswersForItsScopeEndsPlannedNoWork`, `TestApplyNoWork_HandsTheLanesAnsweredItemToTheConsole`). Every guard's removal is a killed mutant.
- Review: the architecture review (HIGH, a disguised flag argument) moved the emptied check into `routeProtectedCards`, projected the binding from `committedset.Committed` so a deferred pin is not routed, and named the card in the reason.
- Docs: design §5.7 R1b, §7.8, §8; the 2026-09-14 incident's R1 note; the factory-rules refusal row.

## Added — `evolve inbox route-console`, the operator's console route (2026-09-29)

- Cycle 1757 (wave 39) found that `goal-text-has-no-selection-authority` must be fixed in `go/internal/loopwave/loopwave.go`, a protected surface its declared files never named. The lane could not route the item: its escalation named the scout's alias, not the pinned id (fixed separately in the triage route). The operator could not route it either. `inboxmover.RouteConsole` had no CLI verb, and the only alternative was editing tracked inbox JSON by hand, which the rule that every control goes through a published interface forbids.
- `evolve inbox route-console <id> <reason> <cycle>` (`go/cmd/evolve/cmd_inbox_route_console.go`) routes a pending item in the inbox root through `inboxmover.RouteConsole` unchanged. It writes `route: console-manual`, the trimmed `routed_reason`, `routed_cycle` and `routed_at`, plus the `route-console` ledger line, and every later lane's `claim` refuses the item (exit 3).
- It belongs to the operator family `evolve inbox`, not `evolve inbox-mover`. The triage profile allowlists all of `evolve inbox-mover` for its claim floor, so a route verb there would have granted an LLM phase the operator's authority (architecture review).
- An item under a lane's claim is refused whatever cycle is named, because the mover's own ownership check compares only the cycle it is handed, and an operator could name the live lane's cycle.
- Exit codes: 10 for usage (a missing argument, a blank id or reason, a negative or non-numeric cycle), 1 for an id the root does not hold or one held by a claim, 2 when the inbox could not be read or the route could not be recorded. Run it at a wave boundary, since the item is a tracked file.
- Tests, red first, through `runInbox`:
  - `TestCmd_InboxRouteConsole_ALaneCanNoLongerClaimTheItem` routes, reads the item back and proves `claim` exits 3.
  - `TestCmd_InboxRouteConsole_RefusesAMalformedRequest` covers five cases.
  - `TestCmd_InboxRouteConsole_RefusesAnItemNoRootHolds` covers a live claim, left unchanged, and an unknown id.
  - `TestCmd_InboxRouteConsole_AFailedRewriteIsExitTwo`, `TestCmd_InboxRouteConsole_AnUnreadableInboxIsAFaultNotAnUnknownID` and `TestCmd_InboxUsageNamesRouteConsole` cover the rest. The `inbox` registry summary now names every verb.
- Mutation sweep: 6 of 7 killed. The seventh was a branch reachable only if a lane claims the item between the check and the write; it was removed, so that race exits 2 with the mover's own message.
- Docs: `docs/operations/runtime-reference.md` (operator commands), `docs/architecture/packages/cmd-evolve.md`.

## Fixed — an audit FAIL that names only the explanation document re-authors it at Build, not TDD (cycle 1745, 2026-09-29)

- Cycle 1745 (wave 30): the auditor found the code correct (14/14 predicates, 50/50 and 30/30 mutants killed, the ACS suite green) and failed the cycle only because the Build's explanation document had two factual errors, `NEEDS_CORRECTION` as the explanation-review rule requires. Both defects named `docs/explain/builds/cycle-1745-*.md`, yet the verdict routed like any `code-audit-fail`: the policy default re-entered at TDD, which re-ran the test-first phase and the full build before the re-audit, and the TDD claude-tmux pane wedged on the way (exit 81, `submit_wedged`). The repair round also logged "continuation: findings artifact … audit-fail-reason.json unreadable": an agent-graded FAIL writes no floor reasons, so the brief reached the builder without the audit's defects.
- The correction's start phase now comes from the defects, not the class label. `explanationCorrectionDocument` (`go/internal/core/explanation_correction.go`) reads the audit's failure block and locates each defect by its first path-like token (a slash, or a stem of two or more characters with an extension that starts with a letter, so `e.g.` and `2.5` are prose and `probe.c` is a file; a `:line` or `#L` suffix and surrounding quotes or brackets trimmed). When every location is the cycle's own document (`explanationdocs.DocumentPath` for the cycle and run, or that path under a worktree) and no runner gate diagnosed the FAIL (`CycleState.AuditFailReasons` empty), the FAIL is `explanation-needs-correction`.
- For that class, `decideAfterAuditFail` narrows the policy's granted envelope to `retry@explanation` or `decline` (`explanationCorrectionEnvelope`, applied after `computeRetryEnvelope`, which is unchanged). `retry@explanation` re-enters Build, the rung that already re-authors the explanation after a rebase the host cannot prove identical (ADR-0105). TDD is not re-run and an adjudicator's `retry@tdd` is clamped. The round still spends the `code-audit-fail` retry budget, so at the cap the cycle declines to retro as before. The Signal Center's `ORCHESTRATOR_AUDIT_REPAIR_GRANTED` reads `repair round via build` and its reason names the class.
- The Build dispatch of that round carries `core.CtxKeyExplanationReauthor` (seeded by `seedAuditRepairContext` on the live and resume paths alike). The build prompt then opens with an "Explanation Re-author" section: the audit found the change correct, edit only the named document, rewrite `build-report.md` with its `## Explanation Documentation` naming the corrected document, and leave code and tests unchanged. The report rewrite is required: the bridge completes a Build only when `build-report.md` differs from its pre-dispatch baseline, so a document-only edit would wait out the artifact timeout. The audit then re-runs on the corrected document.
- `recordExplanationCorrection` writes the defects, each prefixed with the class, into `audit-fail-reason.json` (the coherence-floor schema, phase `audit`), so the repair brief and a continuation read them and the "unreadable" WARN is gone for this class. The next audit dispatch retires the record through `resetFloorFailReason`, so a later round never inherits it. As with a runner-downgraded FAIL's floor record today, a re-author Build that dies before the re-audit is digested from the record, not from the Build error. Reading the failure block in the brief for every agent-graded FAIL, which would end the WARN everywhere, is filed as `repair-brief-reads-the-failure-block-for-every-agent-graded-fail`.
- Unchanged: a FAIL with any defect outside the document (a code path, a bare report file, a defect with no path, another cycle's or run's document) or with a runner-gate diagnostic keeps today's routing, the auditor's declared class and the `failurelog` vocabulary. The auditor still fails an inaccurate explanation; X2 (the designed rung that corrects the document without an audit FAIL) stays designed. The auditor reference now asks each explanation defect to begin with the document's path and says what the host does with it (pinned in `TestAuditorPersona_KeepsAnInaccurateExplanationItsOwnFailWhileTheGateRecordsAnAdvisory`); the failure-adjudicator persona says what `retry@explanation` means when it is offered.
- Tests, red first: `TestDecideAfterAuditFail_AnExplanationOnlyFailReauthorsAtBuildNotTDD`, `TestDecideAfterAuditFail_AnExplanationOnlyFailRecordsItsFindings`, `TestDecideAfterAuditFail_TheAdjudicatorCannotSendAnExplanationOnlyFailToTDD`, `TestSeedAuditRepairContext_AnExplanationOnlyRoundScopesTheBuild`, `TestExplanationCorrectionEnvelope_NarrowsOnlyAGrantedRetry`, and the loop-level `TestRunCycle_AnExplanationOnlyAuditFailReDispatchesOnlyTheBuildReauthor` (TDD dispatched once, Build twice, the second Build scoped and briefed); before the wiring, the loop test saw TDD dispatched twice. The unit tests of the classifier, the record and the build section (`TestExplanationCorrectionDocument_*`, `TestRecordExplanationCorrection_*`, `TestComposePrompt_AnExplanationReauthor*`) were red on undefined symbols first. Guards that pass before and after: `TestDecideAfterAuditFail_AMixedFailKeepsTDD`, `TestDecideAfterAuditFail_AnExplanationOnlyFailAtTheCapDeclinesAsToday`, `TestRunCycle_AMixedAuditFailStillRestartsAtTDD`; a substring-classifier mutant fails two of the nine outside-the-document cases. After review (combined code/Go, one MEDIUM): prose before the path (`e.g.`, `2.5`) no longer counts as the defect's location, red first on two prose-first defects, with a `probe.c` guard that a code file before the document still keeps the full round; `recordExplanationRound`'s WARN on an unwritable workspace is pinned. The architecture review approved (no CRITICAL or HIGH); its fixes: `retry@explanation` sits in the `retryAction` vocabulary block, and the design states the digest consequence and the misroute cost (one retry attempt).
- Inbox: `doc-only-audit-fail-reauthors-the-explanation` consumed.

## Fixed — the build handoff floor runs the repo-contract pack, so a raw git fixture goes back to the build (cycle 1745, 2026-09-29)

- Cycle 1745 (wave 30) added `go/internal/cyclesimulator/characterization_test.go` with a raw `git init` fixture. The build self-check ran only the changed package, the audit and its correction re-audit passed (the ACS suite does not run the raw-git ratchet), and only ship's repo-contract fixed pack ran `rawgitratchet.TestRatchet_NoNewRawGitFixtures`, which went red ("pushing would red main"); recovery routed to a re-audit.
- The build handoff floor now runs the same pack at build exit. `productionBuildFloorChecks` (the one floor the cycle reviewer and `evolve selfcheck build` run) appends `core.RepoContractFloorChecks(repoContractPack)`, and `repoContractPack` defaults to `ship.RunRepoContractPack`. A named red rejects the handoff, and the build correction names the failing test before any audit. An exit that names no test WARNs and passes the handoff, because ship retries it and classes it `REPO_CONTRACT_INFRA`. The floor hands ship the worktree and ship picks the module (`<worktree>/go`) as the gate does. In evolve-loop's own tree, a lane that deletes or moves a guard suite gets a named `[build failed]` red for its missing pattern at both, carrying go's `pattern …: no such file or directory` text, since the lane can restore it; the shared classifier keeps that a red rather than infra. The dial is read from the project's `policy.json`, or the worktree's when there is no project root (`evolve selfcheck build`); an unreadable policy WARNs and runs the pack.
- One home for the suite list and its runner: they stay in `internal/phases/ship/repocontract.go`. Core cannot import ship (ship imports core), so the composition root injects ship's own pack run, the way it already injects `guards.IsProtectedSurface`. `RunRepoContractPack` goes through the gate's own seam, and ship's red message now derives its suite names from `repoContractPackages` (the text is unchanged).
- The red line carries the failure text, not only the name. Ship names reds only for its own real red (`packOutcome.realRed`), so the floor branches on the names alone, and `RunRepoContractPack` hands the floor the failing tests' own output, which `classifyPackEvents` now keeps apart (`packFailureLog`): a tail of the pack's teed output cannot carry it, since `go test -json` prints in package completion order and the ratchet's message sat 37KB before the end of a 97KB stream. An exit that names no test WARNs with its error and the pack's output.
- The floor runs the pack under its own 120s deadline (`repoContractFloorDeadline`, the floor's `go test` convention), not ship's 20m; a kill names nothing, so it WARN-passes and ship stays the authority. A kill reaches the `go` command, not its test binary, which is orphaned until its own timeout; ship's cancel does the same, so that is filed on its own (inbox `repo-contract-cancel-orphans-test-binary`).
- One decision on where the pack runs (#711). The branch's first CI run failed the e2e tier: four fixture cycles (`go/` is `module e2e.local/fixture`) were rejected at the build floor. Ship had never passed them either: on a clean origin/main, `TestE2EPipeline_IntentPhase_RunsAndShips` passed while its cycle ended `cycle level failure in phase ship: … REPO_CONTRACT_GATE … RED`, because the e2e tests asserted only that a ship role reached the ledger. The pack is evolve-loop's own guard set, but both ship and the floor ran it against any tree, so ship refused every ship of a foreign Go module (a named red) or of a tree without `go/` (infra), and the two read an unset gate word differently. `internal/repocontract` is now the one decision both call: `PackRuns(gate, root)` runs the fixed pack only while the gate is on and `<root>/go/go.mod` declares evolve-loop's module (derived from the package's own import path, so it cannot drift; any whitespace, quoted or raw-string `module` line reads); elsewhere both skip it with a note (ship's scan log keeps a `skipped (module …, changes vs …)` line). `GateOn` still switches ship's whole gate, because its added-test and importer backstops apply to any Go module, and `ModuleDir` is the one home of `<root>/go`.
- The e2e harness returns the cycle run's error, and the two tests that claim a ship (`TestE2EPipeline_IntentPhase_RunsAndShips`, `…/fluent_ships`) now require the run to succeed: on origin/main they fail with ship's `REPO_CONTRACT_GATE`, on this branch the cycles ship. Local e2e runs were slow for an unrelated reason: the bridge scans `$HOME/.claude` for token usage on every dispatch, so on a host with 8GB of transcripts a probe cycle took minutes and the audit-FAIL paths hit the 300s bound (on origin/main as well); with CI's empty home a cycle takes about 6s. The fake-CLI cycle harnesses now give each cycle its own empty `HOME` (`isolatedHome`, which keeps the parent's Go caches), so a local run is isolated the way CI is.
- Stage: the floor follows ship's existing dial, `gates.repo_contract_gate` (compiled default `enforce`); `off` turns both off. No new policy key: the floor moves a verdict ship already enforces to an earlier, cheaper rung, and a red here is a real defect in the lane's change, so a shadow period would only delay what ship blocks anyway.
- The builder and tdd-engineer personas now say that git-backed test fixtures use `internal/gittest` (`gittest.Fixture(t)`, `gittest.Bare(t)`, `gittest.Clone(t, src)`), never a raw `git init`. The builder edit is line-neutral (the scout/builder/auditor budget stays at 750). `go/docs/testing.md` convention 3 said `t.TempDir()` + `git init`; it now names gittest.
- End to end on a scratch clone of the branch base plus one committed raw-fixture test: the base binary's `evolve selfcheck build` was GREEN; this branch's reports `repo-contract scanner pack RED: …/internal/rawgitratchet.TestRatchet_NoNewRawGitFixtures` (rc=1, 7.2s) with the ratchet's own lines under it: `internal/cyclesimulator/rawfixture_e2e_test.go builds a raw git repo (1 init sites) outside internal/gittest: use gittest.Fixture(t) / gittest.Bare(t) instead`.
- Tests, red first: `TestRunRepoContractPack_RunsTheGatesOwnPackAndNamesItsReds`, `TestRunRepoContractPack_NamesRedsOnlyForTheGatesRealRed`, `TestContractRed_NamesEverySuiteOfTheOnePackList`, `TestClassifyPackEvents_KeepsEachFailuresOwnOutput`, `TestRunRepoContractPackages_CarriesTheFailingTestsOwnOutput`, `TestRepoContractFloorChecks_ARawGitFixtureIsCorrectedAtBuildExitByTheRatchetsName`, `TestRepoContractFloorChecks_GreenOrAnUnnamedExitPassesTheHandoff`, `TestRepoContractFloorChecks_AnyNamedRedCorrectsTheBuild`, `TestRepoContractFloorChecks_RunsThePackUnderItsOwnDeadline`, `TestRepoContractFloorChecks_ADeadlineKillWarnsAndPassesTheHandoff`, `TestRepoContractFloorChecks_FollowsTheShipGatesDial`, `TestRepoContractFloorChecks_WithoutAProjectRootTheWorktreesPolicyDecides`, `TestRepoContractFloor_ARedPackCorrectsTheBuildAndNeverReachesTheAudit`, `TestRepoContractPackSeam_DefaultsToShipsOwnPack`, `TestProductionBuildFloorChecks_CorrectsARedRepoContractPackByTheTestsName`, `TestRepoContractPack_ShipAndTheBuildFloorMakeOneDecision`, `TestPackRuns_OnlyInEvolveLoopsOwnModuleWhileTheGateIsOn`, `TestPackRuns_ReadsTheModuleLineAsGoWritesIt`, `TestPackRuns_RunsInThisRepositorysOwnTree`, `TestGateOn_OnlyUnsetAndOffAreOff`, `TestModuleDir_IsTheTreesGoDir`, `TestIsolatedHome_AnEmptyHomeThatKeepsTheParentsGoCaches`, `TestPersonaHouseRules_GitBackedFixturesUseGittest`. With the floor disabled, the core tests that expect a red fail.
- Inbox: `repo-contract-pack-at-build-exit` consumed.

## Fixed — loop controls: stop after this wave, keep an interrupt across the boundary refresh, keep the wave budget across a re-exec (2026-09-29)

Three console-owned inbox items from the 2026-09-28 wave boundaries, landed as one branch (`fix/loop-controls`), one commit per component.

- **Stop after this wave** (`loop-stop-at-wave-boundary`). `prepareIteration` honours the operator brake `.evolve/loop-stop` at the start of every iteration, the first included, before the pre-wave probes and any refresh. The run ends with `stop_reason: loop_operator_brake` (the batch's spelling of `chain_operator_brake`), exit 0, after the ordinary closeout (finalize, the batch-end sweep, pending dossiers published on exit), and prints one `[loop]` line naming the brake file and how to release it. A brake present at launch runs zero waves; a chained run stops the running batch at its wave boundary and the chain at its batch boundary. The brake is checked before the pre-wave work (probes, plane sync, dossier publish, now behind a coordinator dependency) and before the refresh, and tests pin that neither runs on a brake stop. New verb `evolve loop-stop` writes the brake (with a timestamp line) and says what happens next; `evolve loop-stop --release` removes it. Before this the brake stopped only a chained run at a batch boundary, and the console stopped a multi-wave run by sending SIGINT to its pid.
- **A pending interrupt survives the boundary refresh** (`boundary-refresh-honors-a-pending-interrupt`). At the wave-29 close a SIGINT that landed while the boundary refresh was rebuilding was lost: the refresh re-pinned and re-execed, and the new image booted as a fresh run. `Refresher.Refresh` now takes the batch's context and checks it before the rebuild, after it and before the exec; an interrupted refresh is one `LOOP_BOUNDARY_REFRESH_SKIPPED` with `step=interrupted` and never re-execs, and `prepareIteration` re-checks the context after a refresh that did not re-exec, so the run ends with `stop_reason: signal` (exit 130). A SIGINT caught during a batch's closeout now makes the batch exit 130 too, so a chained run stops instead of starting the next batch (the architecture review's probe ran 20 batches past it).
- **A boundary re-exec keeps the wave index and the budget** (`boundary-reexec-keeps-cycle-budget-and-wave-index`). A re-exec restarted the loop at wave 0 with the full `--max-cycles`: every wave after a ship logged "wave 0" and a multi-wave run outlived its count while ships landed. The breaker marker the wave-boundary refresh arms now also records the pid (`syscall.Exec` keeps it) and the completed-wave count in their own fields (`pid`, `waves_done`). The replacement image takes the handoff once, at boot, before the chain/batch split: it is honoured only for the same pid running a different build commit and armed at most five minutes earlier; the run continues at the completed-wave index with the remaining budget (a chained run's first batch only), and the record is consumed, so no later batch or launch inherits it. A chain-boundary re-exec arms no handoff. No new flag or environment variable. Left: a chain-boundary re-exec still restarts the chain's own batch count (`max_batches` is a runaway backstop).
- Tests, red first: `TestRefresh_AnInterruptBeforeTheExecSkipsTheReExec`, `TestPrepareIteration_AnInterruptDuringTheRefreshStopsTheRunInsteadOfReExecing`, `TestRunLoop_ABrakeEngagedMidRunEndsTheBatchAfterTheRunningIteration`, `TestRunLoop_ABrakePresentAtLaunchRunsZeroIterations`, `TestRunLoopChain_TheBrakeStopsTheRunningBatchAtItsWaveBoundaryAndTheChainAtItsBatchBoundary`, `TestLoopStop_EngagesAndReleasesTheBrake`, `TestTakeHandoff_TheReplacementImageResumesOnceAndNoOtherProcessInherits`, `TestRunLoop_ABoundaryReExecKeepsTheWaveIndexAndTheRemainingBudget`, with live-context and foreign-pid guards; after the architecture review, `TestPrepareIteration_ABrakeStopsBeforeThePreWaveWorkAndTheRefresh`, `TestRunLoop_ABrakeAtTheBoundaryConsultsNoRefresh`, `TestRunLoop_AChainBoundaryReExecArmsNoHandoff`, `TestTakeHandoff_AStaleHandoffIsNotHonoured`, `TestRunLoopChain_TheHandoffIsTakenOnceAtBootAndResumesOnlyTheFirstBatch` and `TestRunLoopChain_ASigintDuringABatchCloseoutStopsTheChain`, with the review's surviving mutants (brake moved after the refresh or after the dossier publish; the Driver arming the handoff) now killed. The five-parameter `maybeRefreshChainBoundaryWithSignals` seam is gone (callers use `wiredRefresher`), and every comment the change edited is one sentence. Docs: runtime-reference.md (the wave-boundary protocol: stop with `evolve loop-stop`, launch after `evolve loop-stop --release`; the chaining and boundary-refresh rows), the `cmd-evolve` package doc, the unit-13 design doc (§11), the dashboard guide, `signal-codes.md` (regenerated), `signal-center-design.md` (the plane-sync halt's origin is now `loopBatchCoordinator.probeSyncAndPublish`).

## Fixed — the doc guard sees a `cd` into a doc root after another command (2026-09-29)

- A post-merge security audit of the doc guard found that entering a doc root counted only when `cd` or `pushd` began the raw command text: the pattern was anchored at `^cd`, and each command's text keeps the blanks after the `;`, `&&` or newline before it. `true; cd docs && rm -f README.md`, `set -e` followed by an indented `cd docs`, and `true; cd docs && mv README.md /tmp/` were allowed. The hole predates this week's hardening; every earlier test began with `cd`.
- Entering is read from the command's words, like every other verb: `cd` or `pushd` naming a doc root, through `candidateCommands`, so `command cd docs` and `builtin cd docs` count as well (`builtin` joins the wrapper list). The anchored `cdDocsRe` is gone.
- The draft exception refuses a draft path that is not a regular file; a symlinked draft was allowed, though `rm` removes only the link.
- Its review found every word-read verb missed behind a shell reserved word (`{ cd docs; rm …; }`, `if cd docs; then rm …; fi`, `! cd docs; …`, `cd docs; { rm …; }`); `candidateCommands` now skips `!`, `{`, `if`, `then`, `elif`, `else`, `do`, `while` and `until` before a command, in the one place every verb reads.
- Tests: `TestDocDelete_EnteringADocRootAfterAnotherCommandStillCounts` (25 denied forms, 3 allowed), `TestDocDelete_ADraftThatIsASymlinkIsNotTheDraft`, `TestDocDelete_AnAbsentDraftIsStillItsOwnToRetract`, red first; 8 mutants each die. A directory whose name hides a doc root (a symlink named `mirror`) stays a documented limit: only names are checked.

## Fixed — every Make test recipe runs git with background maintenance off (2026-09-29)

- The raw-git fixture flake (`t.TempDir` cleanup: `unlinkat .../.git...: directory not empty`, from git 2.47's detached `git maintenance run --auto` writing after a commit returned) failed CI four times: the dossier fixture twice, then `internal/core` on #698 and #705, each time in a change that touched neither the test nor its fixture. `internal/core` alone still holds 37 raw fixtures.
- `internal/gittest.MaintenanceConfig()` is the single Go source of the settings (`maintenance.auto=false`, `gc.auto=0`). It returns a fresh copy on every call, so no caller can change the shared list. gittest fixtures persist the settings in each repo's own config. `go/Makefile` mirrors it: it exports `GIT_CONFIG_COUNT` with the same pairs, so a git run under `make test`, `make test-integration` (CI and release) and `make test-e2e` sees them at command scope (git 2.31 or later; an older git ignores the variables). A test that sets its own `GIT_CONFIG_COUNT` replaces the recipe's table, so the two `internal/core` tests that did (`cycle_source_integration_test.go`, `orchestrator_signal_checkpoint_integration_test.go`) now build theirs with `gittest.ConfigEnv(extra...)`: the maintenance pairs plus their own. The raw-git ratchet keeps shrinking the raw fixtures (inbox item `raw-git-fixtures-migrate-to-gittest`).
- Tests: `TestMakeTestRecipes_RunGitWithBackgroundMaintenanceOff` runs a probe through the real recipes with the variables removed from its own environment and global and system config off (red before the export: `git config maintenance.auto = ""`). The probe's expectations are rendered from `MaintenanceConfig()`, so a key added there fails it until the Makefile follows. gittest's fixture pins read with `git config --local`, so the recipe's command-scope values can no longer satisfy them. `TestConfigEnv_ReplacesAmbientCommandScopeWithMaintenanceConfigAndExtras` pins the helper, and `TestMaintenanceConfig_ReturnsACopyCallersCannotMutate` pins the copy. Docs: `go/docs/testing.md`.

## Fixed — a closeout no longer moves main under a sibling lane mid-wave (cycle 1704, 2026-09-28)

Salvaged from the unlanded 2026-09-26 worktree (`fix/dossier-commits-at-wave-boundary`) and ported test-first onto current main. On 2026-09-26, cycle 1704 passed audit with WARN. Cycle 1705 then sealed FAIL, and its closeout committed its dossier to the plane's `main`. Ship refused 1704 (`AUDIT_BINDING_HEAD_MOVED`), and the forced re-audit rated the same bytes stricter and failed them. After a correction round, 1704 went through a rebase and a rebuild for the same bookkeeping commit.

- A fleet lane writes its closeout dossier to `.evolve/dossiers-pending/`, which is host-only and gitignored, with no git-mutation lock and no commit. A sequential cycle still commits at once.
- Pending dossiers are published only when no other run is live: after the plane sync at each wave boundary, on every loop exit (before the spine fail-open rollup reads the corpus), and after `evolve fleet` and each `evolve campaign run` wave return (`publishPendingDossiers` → `dossier.PublishPending`).
  - The publisher itself refuses while another process's run is live on the plane, so a second `evolve loop`, a stray `--resume`, or a fleet or campaign beside a loop cannot commit a closeout under a live sibling. It asks before the git-mutation lock and again once it holds it. The WARN names the blocking run dir and why (`loopchain.LiveSiblingRun`: "live pid N", "no readable lease" for a stale run `cycle-state.json` names), and points at `evolve cycle reset` for a stale one; a plane whose runs cannot be read holds too.
  - It holds the shared git-mutation lock and commits only onto a plane current with or ahead of origin/main; otherwise the pairs stay pending with a WARN naming the reason.
- `PublishPending` publishes a pair only if it is exactly what `Write` renders for the dossier its JSON holds, filed under that dossier's cycle, and it completes a pair whose markdown half a crash lost.
  - A pending half that is not a regular file (a symlink, say) is refused.
  - It never writes over a record: a corpus file with different bytes, or half a record, is refused. A byte-identical record is not rewritten; it is committed if untracked, and the pending copy is cleared.
  - A refused pair stays pending with its reason in `Failed`; a failed write or commit removes what it wrote.
- The dossier destination is a root decision (`WithDossierDestination`: committed, files-only for `--simulate`, pending for a fleet lane). It replaces `WithDossierCommit`.
- A first design exempted corpus files from leak recovery. Security review blocked it: an agent-planted pair would have reached `main` unreviewed.
- Size ratchet: `prepareIteration` 55 → 48 (the wave-binary resolution moved to `resolveWaveBinary`, now its only home: the batch-start copy is gone, `loopBatchCoordinator.run` 108 → 100); `runFleet` and `runCampaignRun` unchanged, since both route their lanes through `runLanesThenPublish`.
- Record: `docs/incidents/2026-09-26-a-failed-cycles-closeout-moved-main-under-a-passed-sibling.md`.

## Added — one always-reporting `CI required` check: a skipped, cancelled or missing CI job can no longer read as success (2026-09-28)

- Salvaged from the unlanded 2026-09-14 test-campaign worktree `dev/test-ci-required-results-2026-09-14` (campaign requirement R8). No workflow or `internal/ciparity` file had changed on `main` since its base (`3a972e20`), so the design applies as written.
- New `.github/workflows/required.yml` (workflow `required CI`) runs on every pull request and on pushes to `main` and `go-rewrite-phase-1`, with no path filter. Its `changes` job diffs the event with git (`--no-renames`) and selects the Go suite for `go/`, `skills/` and `agents/`, and landing validation for `landing/` and `docs/explain/`. Any other path selects both; only Markdown under `docs/reports/`, `docs/research/` and `docs/private/` skips both. Plugin validation and durable ACS run on every event. A git failure or an invalid commit ID fails routing instead of skipping a suite.
- The `CI required` job runs with `if: always()` after routing, `validate`, `go` and `landing`. It fails unless routing, `validate` and `acs-durable` succeeded, and each optional suite either ran and succeeded (its call result, which fails when any matrix leg fails, and its inner job's `job.status` output, which is empty when that job was skipped) or was deliberately unselected and skipped.
- `go.yml` and `ci.yml` no longer trigger on a push or pull request of their own; they keep `workflow_call` and `workflow_dispatch`, so a PR runs the Go matrix once. `release.yml` still calls both on the tagged commit. Landing test, vet and render move into the reusable read-only `landing-validation.yml`; `landing-pages.yml` (push to `main`, manual) deploys the artifact it uploads, and pull requests validate the landing module through `required CI` without deploying.
- Check names change: the five jobs now report as `go / build + test (Go) (ubuntu-latest, 1.23)`, `(macos-latest, 1.23)`, `(ubuntu-latest, 1.27)`, `plugin and durable ACS / validate` and `plugin and durable ACS / acs-durable`, plus `landing / test landing module` when landing validation is selected. No branch rule required a check (verified read-only), so nothing that gated a merge stops reporting.
- Tests, red first on `main`: `internal/ciparity` pins the graph (default tier) and runs the exact routing and result scripts from the YAML against real git histories built with `internal/gittest` (integration tier). Six fault controls (aggregator without `always()`, `skills/` routed away from Go, any Go-suite result accepted, rename detection on, every Markdown file skipped, `.github/` skipped) each turn the tests red.
- Consumers of the old workflow names: `/evo:publish` watched the `go`, `CI` and `release` runs of the released commit; with no `go` or `CI` run of their own it would report a false RED, so it now watches `required.yml` and `release.yml`. `/evo:release` and the `evolve release` advisory name `required CI`.
- Docs: go/docs/testing.md "CI shape"; the design and implementation reports under docs/reports/test-ci-required-results-*-2026-09-14.md; the campaign tracker docs/research/testing-completion-2026-09-14.md.
- Review round (2026-09-29): `CI required` must wait for every other job in `required.yml` (a test derives the set from the YAML); Markdown in `docs/` outside the three skip folders provably runs both suites; the release preflight, `ciwatch` and both release skills read the `required.yml` run (`ciparity.RequiredWorkflow`), not the newest run of any workflow, which a green `landing-pages` run could have been; the routing script uses early exits. After a force-push, routing cannot reach the old tip and fails closed; a manual `workflow_dispatch` run of `required CI` posts a green result.
- Not done: the branch rule requiring `CI required` (R9) needs the operator's approval, and cycle ships first need a PR-based publication path, because today they push directly to `main`.

## Fixed — the doc-deletion guard judges what bash will run: `unlink`, `git clean`, a deleting `find` and quoted paths (2026-09-28)

- Salvaged from the unlanded `docdelete-uncommitted` worktree (defence in depth for the open `ship-refuses-deleting-committed-documentation` item, which stays open: a text guard cannot see an interpreter's deletes).
- `docdelete` counts `unlink`, `git clean` and a `find` with `-delete` or `-exec rm` as removals, and matches the doc roots on the words bash passes after quote removal, so `rm doc''s/…`, `rm do\cs/…` and `rm "docs"/…` are denied. `mv` and `git mv` sources are judged the same way, and the own-draft exception is refused when another command on the line removes something.
- Review pass (code review FIX_THEN_MERGE, security review APPROVE-WITH-MINOR), each finding red first: `mv -t <dir>`/`--target-directory` (and abbreviations such as `--target=`) name the destination; every word after `--` is a source; `git mv` is found past git's global options (`git --no-pager mv`); `find` removes when `-exec`/`-execdir`/`-ok`/`-okdir` runs `rm` or `unlink` by any path, so `find docs -name rm` is no longer denied; the shell scanner decodes ANSI-C `$'…'` quoting, which also closes `$'git' push` for the ship guard; doc-root patterns match case-insensitively because a decoded escape can spell `Docs`.
- Second review pass (architecture FIX_THEN_MERGE, code/security FIX_THEN_MERGE), red first: which program a command runs is one rule in the shell scanner (the third pass split it, below; assignments and the wrappers `command`/`env`/`exec`/`nohup`/`time` skipped, base name, compared lowercased), used by both guards, so `/usr/bin/unlink docs/…`, `FOO=1 unlink …`, `/bin/rm doc''s/…`, `command rm …` and `MV docs/…` are denied; the guard scans once, unfolded, with case-insensitive patterns (folding had turned `\U` into `\u`); `mv` options are read as GNU getopt reads them (`-S.t` and `-St` are suffixes, `--suffix docs` consumes its word, which had been a false deny); `$"…"` locale quoting is decoded (`git clean -fdx $"docs"` was allowed). The draft exception keeps its exact spelling, and `git -C docs rm <draft>` is pinned as a removal.
- Third review pass (architecture and security, FIX_THEN_MERGE), red first: the rule splits by failure direction. The deny side (`candidateCommands`) judges every word behind a wrapper (`command`, `env`, `exec`, `nice`, `nohup`, `sudo`, `time`, `timeout`, `xargs`) as a possible program, closing `env -u PATH rm docs/…`, `exec -a name rm …`, `sudo unlink …` and `timeout 5 unlink …`. The ship guard's allow check stays strict (assignments only), closing the second pass's `env -S'git push …' evolve ship` regression, and its git match is case-insensitive (`GIT push`).
- Tests: `TestDocDelete_EvasionsThatBashStillResolvesToTheDocRootsAreDenied` (68 denied forms, 15 allowed ones that guard against false denies), `TestCandidateCommands_AWrapperMakesEveryLaterWordAPossibleProgram`, `TestShip_Decide_GitIsGitInAnyCase`, `TestShip_Decide_OnlyAPlainEvolveShipIsNative`, `TestSplitShellCommands_AnsiCQuotingYieldsTheWordBashRuns`, `TestSplitShellCommands_AnsiCQuotedSeparatorsStayInTheWord`, `TestShip_Decide_AnsiCQuotedGitIsStillGit`, plus four exact-spelling denies of the draft. Red first on main (9 evasions allowed) and on each of the four review passes. 45 mutants of the new rules each die (11, 17, 12 and 5 across the passes); a fourth pass judged a move by any wrapper candidate, not the first (`sudo -u mv -t docs/x mv …`), and added the macOS wrappers `arch` and `caffeinate`.

## Fixed — the shipped-lane test fixture keeps git maintenance out of the background (2026-09-28)

- `TestUnwindShipCommit_DeclinesWithoutMovingHEAD` failed CI on #698, a change touching neither the test nor its fixture: `t.TempDir`'s cleanup met `directory not empty` because the fixture's raw `git init` let git 2.47+ detach `git maintenance run --auto` after a commit. It is the third CI failure of the class (the dossier fixture twice).
- `internal/core`'s shipped-lane fixture (`newLane`) takes its worktree from `gittest.Fixture`, which persists `maintenance.auto=false` and `gc.auto=0` and retries teardown; its raw-git ratchet entry is removed. Inbox item `raw-git-fixtures-migrate-to-gittest` covers the remaining 130 files.

## Fixed — the durable ACS suite no longer leaks an evolve binary per run; the disk-full halt of wave 27 (2026-09-28)

- Wave 27 went 0/2 on a host with 143 MiB free of 460 GiB. Both lanes' failures (a ship backstop's `no space left on device`, an audit's integration-tier gate) read as code failures. The loop was halted as a system failure and the disk freed; see docs/incidents/2026-09-28-the-disk-filled-and-two-lanes-failed-for-it.md.
- `acs/regression/cycle1515` and `acs/cycle1498` built `evolve-under-test` into a temp dir in `TestMain`, deferred its removal, and then called `os.Exit(m.Run())`, which skips deferred calls. Each run of the durable suite (every audit's CI-parity gate, every floor, CI) leaked about 22 MB, 1,427 dirs and 32 GB in all. Both `TestMain`s now return, and Go (≥ 1.15) exits with `m.Run()`'s result. Measured: the unfixed predicate leaks one dir per run, the fixed one none.
- New `internal/testmainexit`: `SkippedDefers` finds a `TestMain` that defers a cleanup and also calls `os.Exit` (closures excluded). `TestModuleTestMainsNeverDeferCleanupPastOsExit` runs it over every bound test file. Red first, it named exactly the two files.
- Queued: `disk-space-preflight` (halt on low free space before any lane runs) and `gc-pipeline-temp-and-go-cache` (`evolve gc` reaps stale pipeline temp artifacts and bounds the 157 GB Go build cache).
## Added — `acsassert.GoTests`: ACS predicates judge `go test -json` events, not printed PASS text (2026-09-28)

- Salvaged from the unlanded 2026-09-14 test-campaign worktree. `pkg/acsassert.GoTests` runs the selected tests with `go test -json` and requires, for each named test, one `run` event followed by `pass`, with the package started and passed. Printed `--- PASS:` text, a prefix-colliding subtest, a skip and a swallowed failure no longer satisfy a predicate.
- Pilots: `acs/cycle1013` and `acs/cycle1015` use it. `cmd/evolve`'s tokens-report test decodes the typed `TokensReport` and compares the exact `TripwireEvent`, replacing a "any key containing tripwire" count.
- Review: FIX_THEN_MERGE (a `fail` event from another package failed the target's validation before the package filter) → fixed red-first (`another_package_fails_beside_a_passing_target`).
- Docs: acs-predicate-quality-gate.md, "Structured Go-test evidence".
## Fixed — the wave-boundary protocol named the wrong unit: one wave is `--max-cycles 1` (2026-09-28)

- The runtime reference's wave-boundary protocol (added the same day) said one wave is `evolve loop --max-cycles <fleet width>`. In fleet mode each batch iteration dispatches a whole wave, so `--max-cycles` counts waves, as the fleet-planning row already said. Wave 28 was launched with `--max-cycles 2` at width 2 and started a second wave instead of stopping.
- The protocol now launches one wave with `--max-cycles 1`, which exits after the wave without reaching a boundary refresh. It names the defect that lets a multi-wave run outlive its count (the boundary re-exec restarts the count at wave 0, inbox item `boundary-reexec-keeps-cycle-budget-and-wave-index`), and the refresh row records the same restart.
- `evolve loop --help` says what `--max-cycles` counts: cycles when sequential, waves in fleet mode.

## Fixed — `evolve sync-main` is not blocked by an untracked file (2026-09-28)

- At the wave-26 boundary the plane was 7 behind origin/main and 1 ahead (the loop's own dossier closeout), and an operator inbox item was untracked. `evolve sync-main` refused ("working tree is dirty") because its check counted untracked files, and `evolve ship --class manual`, the interface that would land the item, refuses a plane behind origin in its push repair. Each waited on the other, so the only way through was outside the interface.
- The dirty check now reads tracked files only (`status --porcelain --untracked-files=no`). An untracked file cannot be lost by a merge; a merge that would overwrite one is refused by git before it starts, and `mergeOrigin` reports that refusal ("git would not merge origin/main", with git's output) instead of trying to abort a merge that never began, which printed "merge conflicted AND abort failed". A case-only name collision is refused the same way on macOS (`core.ignorecase=true`) and coexists on Linux; the untracked file keeps its content either way.
- Tests, red first: `TestSyncMain_AnUntrackedFileDoesNotBlockTheSync`, `TestSyncMain_AnUntrackedFileTheMergeWouldOverwriteRefusesCleanly` (a mutant that aborts unconditionally fails it), `TestSyncMain_ACaseCollidingUntrackedFileIsNeverOverwritten`. Size ratchet: `runSyncMain` 72 → 66.

## Fixed — the audit, the audit binding and Ship read one ship tree (F43 part 2, component 4, 2026-09-28)

- Eight cycles (1626, 1647, 1674, 1684, 1685, 1694, 1705, 1735) failed their audit on "predicate execution tree includes undeclared inputs absent from the ship tree". The inputs were their own declared deliverables. The audit modelled the ship tree as the tracked set (`treefence.TakeTracked`), and the audit binding (`core.worktreeContentSHA`, `git add -u` then `write-tree`) recorded the same tracked tree, while Ship commits every path the build and TDD reports declare, tracked or not.
- `shipmanifest.TakeShipTree` is now the one definition of the tree Ship will commit: the real index, every tracked edit, and `shipmanifest.Select`'s declared paths, snapshotted by `treefence.TakeStaged` (which now runs `add -u` after the declared add, so a declared file removed with plain `rm` stages as a deletion; `TakeTracked` is deleted) under the shared add-refusal retry `shipmanifest.StageRetrying`. The audit's `predicateTreeFor` requires the full tree to equal it, the binding records it as `WorktreeTreeSHA` (and as the verdict-cache key and the composition entries' `TreeStateSHA`), and Ship's worktree path runs `git add -u` itself (its first staging step, idempotent after the binding's) and stages the same `Selection.Paths`, so the receipt, staged-tree and full-snapshot checks compare equal values; the operator plane's direct ship does not run `add -u`. An undeclared untracked input is still refused by name; an undeclared tracked edit ships and is audited, as it always shipped.
- `shipmanifest.Select` holds the whole selection (the `status --porcelain -uall` read, the declared manifest from `ReportFiles()`, `Stageable`, the `check-ignore` probe, fail-open and reported); the refusal parser, the report list and the raw-path prefix moved there from ship (`IgnoredInAddRefusal`, `ReportFiles`, `RawPathRead`), and `GitIn` is a read-only `gitexec` reader (`--no-optional-locks`, exit 1 is an answer). With a workspace the selection is declared-only: a workspace whose reports name nothing adopts no path, where it used to fall back to every changed path, residue included. A workspace-less manual `evolve ship` still stages every changed path.
- An empty selection stages nothing. `git add -A --` with no pathspec stages the whole tree, so a selection emptied by a staged deletion or an ignored declared path swept undeclared files into the commit while the log said "staged 0 explicit path(s) … no `git add -A`". `StageRetrying` never calls a stager with an empty set; the direct-push golden drops its recorded empty add.
- Tests, red first: the replays `TestPredicateTreeFor_Cycle1735_TheDeclaredExplanationDocumentIsInTheShipTree` and `TestPredicateTreeFor_Cycle1694_TheDeclaredPredicateAndEvalAreInTheShipTree` (each reproduced the live refusal text), `TestWorktreeContentSHA_BindsTheTreeTheAuditSealed` and `TestEmitPhaseBindings_AuditBindingHoldsTheDeclaredUntrackedDeliverable` (the binding lacked the document), `TestStageExplicitPaths_NothingSelectedStagesNothing` (both shapes swept the leak), `TestShipFromWorktree_CommitsTheOneShipTree` (end to end: the committed tree equals `TakeShipTree`; Ship left an undeclared tracked edit at its base content), `TestTakeStaged_ADeclaredFileDeletedButNotStagedIsADeletion` and `TestTakeShipTree_ADeclaredFileRemovedWithPlainRmShipsItsDeletion` (the first add order failed rc=128), `TestShipFromWorktree_ADeclaredFileRemovedWithPlainRmShipsItsDeletion` (Ship's own order is safe because the selection is read after `add -u`; a mutant that keeps gone paths fails it rc=128), `TestShipFromWorktree_AFailedTrackedEditAddFailsTheShipBeforeAnyCommit`, `TestTakeShipTree_*`, `TestSelect_*`, `TestStageRetrying_*`, `TestGitIn_*`, and the moved `TestIgnoredInAddRefusal_*`/`TestWithoutIgnored_*`. Mutants killed: the reader without `--no-optional-locks`, the retry without git's stderr, the empty guard disabled. Cycle 1067's fallback predicate moved to the new contract (`TestC1067_002_AReportlessWorkspaceAdoptsNoPathAndNoWorkspaceStagesTheChangedSet`).
- Reviews: the architecture review of the first version (audit only) was BLOCK: the binding was a third ship tree, and refusing undeclared tracked edits rested on a wrong premise. The redesign's design review was FIX_THEN_MERGE (declared-only selection under a workspace, `TakeTracked` deleted, `shipmanifest` off `core`, the empty guard in `StageRetrying`), all applied; Ship runs `add -u` on the worktree path only, because the direct path serves the operator plane. The delta reviews then found the declared-deletion add order (fixed) and Ship's reliance on the binding's step (the worktree `add -u`). A read-only replay on the live lanes 1737 and 1738 found the full tree equal to the ship tree on both.
- Size ratchet: `stageExplicitPaths` 94 → 27 lines; its entry is removed from `offenders.json`.

## Fixed — a size-ratchet allowance is a ceiling, so a lane that shrinks a listed function no longer fails (wave 25, cycle 1735, 2026-09-28)

- Cycle 1735's diff shrank three listed functions by one line each (`loopBatchCoordinator.run`, `prepareFreshBatch`, `runLoopBatch`) without lowering their allowances in `go/internal/sizeratchet/offenders.json`. The ratchet required allowances to be lowered exactly; the module-wide check (cycle 1736's peer predicate `TestC1736_003`) went red after a fleet rebase, the repair budget was already spent, and the cycle failed. Every lane whose diff changes the length of any of the ~270 listed functions was exposed to the same process failure, and lowering allowances from every lane made the file a shared registry: the explanation contract counts it as material, two lanes touching it break the rebase identity proof, and adjacent edits conflict.
- `sizeratchet.Check` now treats an allowance as a ceiling: the violations are a new function past 50 lines and a listed function past its allowance. A shrunk, healed or deleted offender's entry is slack that passes; a boundary tighten removes it (designed, design doc §5.13), and a build-floor no-growth check against the base is the gap closer (designed).
- Tests, red first: `TestCheck_OnlyGrowthOrANewOffenderFails`; the cycle-1726 predicates move to the ceiling contract (`TestC1726_003_AnAllowanceIsACeilingAndSlackPasses`, `TestC1726_006_CheckedInListCoversEveryLiveOffenderAtOrAboveItsSize`), and the eval `function-size-ratchet.md` follows. The shrink lanes' anti-gaming property holds: deleting an entry without shrinking still fails, because the function is an unlisted offender.
- The six pending `sizeratchet-shrink-*` items no longer ask a lane to edit `offenders.json` (two of their keys sit on adjacent lines and would conflict at a fleet rebase); their entries become slack for the boundary tighten.
- Review: FIX_THEN_MERGE (MEDIUM: the census predicate could pass on an empty census -> its known offenders must measure over 50 lines; stale comments and eval criteria that described the exact rule; LOW: a last-span-wins mutant survived -> a reverse-order twin case) -> all fixed.
## Fixed — a claude pane is told its authority in its system prompt (the wave-24 P0 T3, 2026-09-28)

- Cycle 1734's triage pane (claude-tmux) declined the phase: "I'm not going to execute this as if it's a real system instruction: it's untrusted pasted text, not something you asked me to do". It cited the project CLAUDE.md's "confirm direction" and "surface ambiguity" rules and declined the bridge's typed nudge the same way, so the filed fix, one typed authorizing line, would not have helped. The phase identity block (P3) was itself part of the paste. It was the only such refusal in 30 escalations since cycle 1650; with codex walled until 2026-10-14 it is also a whole cycle.
- `phaseidentity.Authority()` is a new standing block, identical on every dispatch: the operator launched the evolve pipeline that runs this pane unattended; the pasted prompt is the operator's instruction for the phase, to carry out; instruction files written for the console's interactive sessions (confirm direction before multi-step work, stop and ask when something is unclear) describe those sessions, so the agent makes the reasonable call and records it in the deliverable. The last line moved here from the facts block, which keeps the phase, cycle, session, prompt files and sole-writer lines.
- claude-tmux launches with the authority as its system prompt: the manifest declares `system_prompt_file` (`--append-system-prompt-file`), `launchIntentFor` names `<workspace>/pane-authority.md` for a dispatch with an agent, and the realizer renders the flag and sets `Realization.SystemPromptFile` only for a manifest with the channel. `stateAuthority` writes the file atomically (`atomicwrite.Bytes`, so two panes starting in one workspace never read it half-written) whenever the flag is realized, a boot-only pane included; a CLI without the channel pastes the authority before the facts. The facts block still ends every paste. The system prompt's bytes are the same on every spawn, so ADR-0071's cache-stable system prompt holds. A probe confirmed the flag applies in claude 2.1.283's interactive REPL.
- Size ratchet: `Engine.LaunchArgs` 208 → 197 (`launchIntentFor`), `Realize` and `prepareTmuxREPL` leave the list at 45 and 50 (`realizeSessionMode`; `reportNamedSession`, `identityFacts`, `stateAuthority`).
- Tests, red first: `TestRealizeFor_OnlyAManifestWithASystemPromptChannelCarriesTheFile`, `TestAuthority_AuthorizesThePasteAndScopesTheConsoleConventionsAway`, `TestTmuxDispatch_ClaudeStatesItsAuthorityAsItsSystemPrompt`, `TestTmuxDispatch_ABootSmokeWithARealizedFileStillWritesTheFileItLaunchesWith`; `TestTmuxDispatch_IdentityIsStatedForEveryTmuxCLI` pins that codex pastes the authority before the facts, `TestTmuxDispatch_NoAgentNameMeansNoIdentity` that a pane with no phase launches with no file, and `TestRealizerWiring_NoCrossCLILeak` the claude launch line with the flag. Six mutants of the split killed.
- Docs: design doc §5.11 T3 and §5.4 (revised), ADR-0022 amendment, the bridge and phaseidentity package notes. Inbox: `claude-tmux-typed-authorizing-turn` consumed.
## Fixed — the audit's predicate-tree refusal names the undeclared inputs (F43 part 1, 2026-09-28)

- The audit refuses to run predicates on a tree whose untracked inputs the ship would omit (`predicateTreeFor`, the full tree against the tracked ship tree). The refusal named no path: "predicate execution tree includes undeclared inputs absent from the ship tree". Cycles 1626, 1647, 1674, 1684, 1685, 1694 and 1705 failed on it, and each repair directive could only tell the builder to stage "intended Build files". The inputs were often the cycle's own deliverables (the TDD predicate file, the scout eval).
- `treefence.Snapshot.Differing(ctx, base)` lists every path that differs between two snapshots, sorted, from the same `diff-tree` the fence's restore uses. The refusal now names them, the first 20 and a count of the rest (`undeclaredInputs`), and a listing failure still refuses and says why the paths are missing.
- The paths ride as a delimited detail span (`cyclestate.WithDetail`, ` [[detail: … ]]`), and every identity reader strips every span (`WithoutDetail`): the failure digest's class and fingerprint, the contract-block identity, and the failure-learning record (`carryover.Summary`), whose text is the P0 carryover todo's dedupe fingerprint. The display keeps the paths; the repair brief carries them in full. Without that, a path containing a classifier keyword ("bridge", "statemap") reclassified the failure, possibly as a guard-abort that halts the batch, and each path set minted a fingerprint the recurrence breaker could not match.
- Tests, red first: `TestSnapshot_DifferingNamesThePathsBeyondItsBase`, `TestUndeclaredInputs_NamesThePathsAndBoundsTheList`, and `TestBeginPredicateEvidence_UnstagedInputsRefuseAudit` now requires the undeclared `helper.go` in the error; `TestWithDetail_WithoutDetail_KeepsTheIdentityAndDropsTheDetail`, `TestAssembleFailureDigest_AReasonsDetailNeverChangesItsIdentity` (two path sets, one with `statemap` and `bridge`), `TestNormalizeReasonForFingerprint_ContractBlocksShareAnIdentityAcrossTheirDetail`, `TestSummary_ALearningRecordKeepsTheFailureClassNotItsDetail` (two refusals differing only in detail dedupe; a trailing reason survives; a different trailing reason is a different failure). Mutants killed: an empty listing, only the added paths, no sort, an unbounded list, `>=` at the bound, and each identity strip.
- Reviews: combined code/Go reviewer PASS; architecture reviewer FIX_THEN_MERGE (HIGH: the path list leaked into the failure identity; HIGH: three surviving mutants) -> both fixed; delta FIX_THEN_MERGE (HIGH: the carryover todo still hashed the paths, and an unterminated marker would cut the reasons joined after it) -> delimited spans and the strip in `carryover.Summary`.
- F43 part 2, the host staging each phase's declared deliverables, stays open (`host-stages-declared-deliverables`).
## Changed — Ship's path selection is its own package, the one source of "which paths Ship commits" (F43 part 2, component 2, 2026-09-28)

- The audit refused eight cycles' own deliverables as "undeclared inputs" because it modelled the ship tree as tracked files only, while Ship commits the paths the build and TDD reports declare (design doc §5.12). To let the audit bind the tree Ship will commit without importing the ship phase, Ship's pure path selection moved, unchanged, from `internal/phases/ship/manifest.go` into `internal/shipmanifest`: `Declared`, `ChangedPaths`, `OutOfManifest`, `UnquoteGitPath`, and `Stageable`, the one composition (the pathspec itself is unexported, so no caller composes the steps differently) (changed paths, pathspec, minus staged deletions and rename sources) Ship now stages from, with `RegularFileIn` the one file rule.
- 17 tests moved under their original names; `TestStageable_IsThePathspecShipStages` pins the contract the audit will rely on (a declared directory takes only what it covers, the cycle-645 leak; a covers-nothing fallback; a both-deleted conflict stays named), killing three mutants the moved suites let survive. Two historical predicates (cycles 1108, 1469) now run the moved tests where they live instead of passing on "no tests to run".
- Reviews: combined code/Go reviewer PASS (every moved body identical modulo renames); architecture reviewer FIX_THEN_MERGE (the gone-path drop was still composed in ship; four surviving mutants) -> `Stageable`.
## Added — the fence can hash the tree Ship will commit (F43 part 2, component 3, 2026-09-28)

- `treefence.TakeStaged(ctx, worktree, pathspec)` snapshots the real index plus `git add -A -- <pathspec>` in a throwaway index: the tree a Ship that stages exactly that pathspec commits. Like `TakeTracked` it refuses without a complete real-index seed, it never touches the real index, and an empty pathspec adds nothing, because `git add -A --` with no paths would stage the whole tree. The add arguments per mode are one function (`addArgs`). `Restore` now refuses any snapshot but a full one (`errNotRestorable`): a staged or tracked snapshot does not know every path, and restoring from one could delete a file staged after it was taken (a review probe did).
- Unwired: the audit adopts it over `shipmanifest.Stageable` in the next component, so its predicate tree becomes the tree Ship commits (design doc §5.12).
- Tests, red first: `TestTakeStaged_IsTheRealIndexPlusTheDeclaredPaths` (every change declared equals the full tree; an undeclared tracked edit stays out; a path the real index already stages ships though undeclared; an empty pathspec equals HEAD's tree; the real index is untouched; a staged snapshot refuses `Restore`; no seed is a refusal). Mutants killed: an empty pathspec adding everything, a seedless staged snapshot, a declared add that ignores untracked files.
- Review: FIX_THEN_MERGE (HIGH: `Restore` on a staged snapshot was untested and unsafe) -> refused; filed for component 4: Ship's own `git add -A --` stages the whole tree when every selected path was dropped, and the ignored-path filter with its retry must become one source Ship and the audit share.

## Fixed — an advisor plan runs a trigger-gated phase only when its trigger fires (the wave-24 P0 T4, ADR-0052 amendment, 2026-09-28)

- Cycle 1733 (wave 24), a size-ratchet refactor lane whose scout reported `goal_type: refactor`, ran fault-localization and bug-reproduction, and failed inside bug-reproduction. Both phases declare `insert_when: scout.goal_type == bugfix` (or a failure class), but at `EVOLVE_DYNAMIC_ROUTING=advisory` the advisor's whole-cycle plan drove every optional phase, and `router.shouldRunFromPlan` checked `skip_when` and never `insert_when`. The advisor plans before scout, from the wave goal, so it cannot evaluate a scout-keyed trigger itself. Cycles 1721–1732 ran bug-reproduction in 10 and fault-localization in 8 non-bugfix cycles.
- On the plan path a trigger that is a content phase's whole admission rule (an `insert_when` with no `rubric_hint`, `config.RoutingBlock.TriggerIsTheWholeRule`) is now evaluated at the phase's turn, when the signal exists; a planned phase whose trigger does not fire is skipped with the clamp `insert-when-gates-plan`, recorded in `routing-decision-N.json` and the ledger like `skip-when-gates-plan`. A `rubric_hint` keeps the advisor's judgment beyond the trigger (architecture-design), an operator `enabled: on` still runs the phase, and floor phases are never gated.
- Two floor-activation scenarios pinned the old contract ("advisory plan inserts tester the trigger would skip"); they now pin that a plan cannot insert tester when its trigger is the whole rule, and that a rubric-hinted tester (`routingtest.AdvisorJudgedTester`) still can. The scenario engine's default tester condition is one value the brick shares (`testerOnARedBuild`).
- Tests, red first: `TestShouldRun_APlannedPhaseWhoseTriggerDoesNotFireIsSkipped` (cycle 1733's plan shape), `TestShouldRun_APlannedPhaseWhoseTriggerFiresRuns`, `TestShouldRun_ARubricHintKeepsAPlannedPhaseTheAdvisorsJudgment`, `TestShouldRun_AnOperatorEnabledPhaseRunsFromThePlanWithoutItsTrigger`, `TestShouldRun_ATriggerNeverGatesAPlannedFloorPhase`, `TestBrick_AdvisorJudgedTesterKeepsTheDefaultTriggerAndAddsAHint`, `TestRoutingBlock_ATriggerWithoutAHintIsTheWholeAdmissionRule`; five mutants of the gate (the floor exemption, the enable condition, the hint exemption, an empty `insert_when`, the trigger check) all fail them.
- Docs: [dynamic-phase-routing.md](docs/architecture/dynamic-phase-routing.md) (the phase registry section), [micro-phase-catalog.md](docs/architecture/micro-phase-catalog.md) §4.1, the router package notes, ADR-0052's amendment and the design doc's §5.11 T4 row. F42 (`document-lane-planned-bugfix-phases`) keeps its other parts: the advisor prompt carries the lane's scoped item, the replan reads a document scout report, one decision on tdd for documents, a coded Signal Center line for the clamp, and the deliverable-kind side (a document lane whose scout declares `goal_type: bugfix` still runs the bugfix phases).
- Filed from the architecture review: `insert-when-fields-need-a-producer` (four registry triggers read a signal no phase emits, so at advisory those phases now never run: changelog-sync and post-ship-monitor on `ship.class`, context-condense on `run_dir.artifact_bytes`, preliminary-study on `campaign.mode`; none of the 67 recent plans chose them).

## Fixed — the universal-fallback tail holds only drivers that can run the phase (ADR-0104 §4, 2026-09-28)

- Wave 24's cycles 1733 and 1734 failed in bug-reproduction and triage. claude-tmux stalled or refused the pasted prompt, codex-tmux was walled until 2026-10-14, and the tail's last rung, ollama-tmux, refused the phase at launch (exit 10, non-transient). The runner surfaces the last rung's error, so a capacity failure sealed as a task FAIL.
- The driver manifest now declares what a CLI cannot do: `toolless: true` for ollama-tmux, which has no tool use and cannot write a worktree artifact. `bridge.HasToolUse` is the one predicate over it; the ollama launch guard asks it, and `universalFallbackTail` (cmd/evolve) drops such drivers from the tail, then applies the operator's family ban. Without the incapable rung, a walk like 1733's ends on codex's wall and the cycle defers.
- An exhausted walk that met a quota wall now surfaces the wall, whatever the rung order: `llmroute.DispatchTiered` reports `Walled` (the grid ran out on fallback-trigger exits, one of them 85), and `bridgechain.WallKeeper` returns the wall instead of the last rung's stall, naming every rung, in the runner's walk and in the bridge handle's (`Walking.Launch`) alike, so `core.isQuotaWall` defers the cycle. A walk with no wall still surfaces its last error, and a non-trigger failure after a wall stays the verdict. The quota pause (`pauseForQuota`) now carries that error on every dispatch root (the resume paths wrap the runner's error in `quotaWall` instead of dropping it): "the dispatch ended on a quota wall across N attempts (…)", and the sentinel `ErrAllFamiliesExhausted` reads "the dispatch chain ended at a quota wall", no longer claiming every family returned exit 85. `WallKeeper.Surfaces` is the explicit predicate the runner logs by. The fixed part of each attempt's request is built once (`baseRequest`), so `dispatchPhaseAttempts` shrank from 97 to 78 lines; `wireOrchestratorDeps` from 320 to 310. `HasToolUse` fails closed; the ollama guard names the real reason (no tool use for a worktree phase), since every lane phase runs in its worktree.
- Tests, red first: `TestHasToolUse_FollowsTheDriverManifest`, `TestUniversalFallbackTail_KeepsOnlyPresentFamiliesThatCanRunAWorktreePhase`, `TestDispatchTiered_AnExhaustedWalkThatMetAWallIsWalled`, `TestDispatchTiered_AWallOnAHigherTierStaysWalledAfterTheStepDown`, `TestWallKeeper_SurfacesTheWallOfAWalledWalkNamingEveryRung`, `TestWalking_AnExhaustedWalkThatMetAWallSurfacesTheWall`, `TestWalking_AWallThenARealFailureSurfacesTheFailure`, `TestRun_AnExhaustedWalkThatMetAWallSurfacesTheWall`, `TestRun_AWallThenARealFailureSurfacesTheFailure`, and the pause-text pins in `quota_correction_defer_test.go` and `signal_cycle_test.go`; mutants that drop the observation, the surfacing, the `Walled` flag, the non-trigger guard or the tail's dedupe order are killed.
- Designed and filed: a typed authorizing turn after the pasted prompt (`claude-tmux-typed-authorizing-turn`). F42 (`document-lane-planned-bugfix-phases`) carries 1733's evidence.

## Fixed — the auditor persona states the rules the code applies (2026-09-28)

- The inbox architecture review of 2026-09-28 found the auditor judging by stale copies of three rules. `agents/evolve-auditor.md` said "WARN blocks shipping", but ship ships a WARN under the fluent default (`phases/ship/audit.go`; only `workflow.strict_audit` refuses it). The persona had no statement of the comments convention, and cycle 1731's auditor asked for doc comments on new helpers. `agents/evolve-auditor-reference.md` said an honest `NEEDS_CORRECTION` "forces the overall Audit to fail", though the gate records it as an advisory (ADR-0102).
- The persona now says what ship does with a WARN; that new and changed code carries no comments, each added one being a LOW, advisory finding (never a WARN or FAIL on its own), with the exceptions left to `code-comments.md` §What a comment may say, and that the auditor never asks for one; and that `NEEDS_CORRECTION` stays the auditor's FAIL as a stated policy, because until X2's document-only correction rung exists the audit-repair round is the only path that corrects the document before it ships (the FAIL names only the document; the route is still the first legal retry, tdd). The comment rule rides an existing checklist line, so the scout/builder/auditor line budget holds.
- Each statement is pinned together with the behavior it describes, in `phases/audit` (red first): `TestAuditorPersona_StatesWhatTheAuditAndShipDoWithAWarn` (a WARN becomes FAIL only under `workflow.strict_audit`), `TestAuditorPersona_MakesAnAddedCommentALowFindingAndNeverAsksForOne` and `TestAuditorPersona_KeepsAnInaccurateExplanationItsOwnFailWhileTheGateRecordsAnAdvisory` (the gate returns the advisory, no error). The retired phrases join the obsolete-token list of `explanationdocs.TestExplanationPersonaAndSchemaContract_NoDrift`. Persona mutants that turn the FAIL into PASS, ask for doc comments or ignore added comments are killed. The design doc's §5.6 no longer attributes the FAIL to ADR-0102; `SECURITY.md` names `workflow.strict_audit` and `evolve ledger verify` instead of a retired variable and a deleted script; `code-comments.md` says how the loop auditor reports an added comment.
- Filed from the review: `auditor-persona-rules-from-one-source` (N1's projection), `explanation-correction-is-a-rung` (X2), `sizeratchet-allowances-not-a-shared-registry`, `gc-manifests-out-of-cycle-run-dirs`, `scopedelta-wire-or-delete` and `design-doc-status-pass`.

## Added — the build floor counts the comments a build adds, in shadow (comment reduction batch 0b, 2026-09-28)

- Cycle 1730 (wave 22) was a correct refactor that failed its first audit on 54 added comment lines, which no deterministic check caught before the audit. The build handoff floor now counts them (`commentFloorFailures`, `internal/core/comment_floor.go`).
- The count is taken across the whole diff (`commentaudit.AddedAcrossDiff`): a comment line one file drops and another gains is a move, not an addition, so a decomposition that moves a function with its comment into a new file is not flagged; the floor lists its paths with `--no-renames`, so a renamed file shows both sides. `commentaudit comments` and `commentaudit check` share one lister over the same rule, so the auditor's commands and the floor agree. `commentaudit.ReadAtBase` is the one reader of a file at the base; it reports a file as absent only when the base itself is readable.
- The stage is `policy.json` `comment_floor.stage` (`policy.CommentFloorConfig`, compiled default `shadow`), read with `config.GateStage`: shadow WARNs on the phase log with every added line named, `enforce` rejects the handoff so the correction ladder removes them before audit, `off` is silent, and an unknown word is off with a WARN. An unreadable diff fails open with a WARN. The build floor's changed-path listing (`changedWorktreePathsSince`) now uses `-z`, so a non-ASCII file name reaches every floor as its raw path instead of git's quoted form.
- Tests, red first: `TestAddedAcrossDiff_ACommentMovedIntoAnotherFileIsNotAdded`, `TestAddedAcrossDiff_AMoveCoversOnlyAsManyCopiesAsTheDiffRemoved`, `TestMain_CommentsTreatsAMoveAcrossFilesAsMoved`, `TestReadAtBase_AFileTheBaseLacksIsAbsentAndAnyOtherFailureIsAnError`, `TestCommentFloorConfig_ShadowsUntilThePolicySaysOtherwise`, `TestCommentFloorConfig_TheStageReachesTheFloorFromPolicyJSON`, `TestCommentFloorResult_*`, `TestCommentFloorFailures_JudgesTheWorktreeAgainstItsBaseAndSparesAMovedComment` and the wiring proof `TestDefaultBuildFloorChecks_RunsTheCommentFloor`. Mutants that drop `--no-renames`, the wiring, or the shadow/enforce split are killed.
## Fixed — the debugger of a fleet-rebase conflict can write its resolution (ADR-0105 B5, ADR-0097, 2026-09-28)

- Cycle 1725 (wave 21) met a genuine fleet-rebase conflict. Its debugger resolved it correctly (it moved its own line out of main's hunk and checked the merge with `git merge-file`), but the read-only worktree fence reverted the edit, the re-entered rebase met the same conflict, and the cycle ended FAIL. As configured, B5 could never succeed on a real conflict.
- The fleet rebase now returns the non-derived paths it met in conflict (`rebaseWithDerivedRegen`, `rebaseCycleBranchOntoMain`), and `rebaseRecordingConflicts` records them on `CycleState.ShipRecoveryConflicts` (`ship_recovery_conflicts`, additive and `omitempty`), which the ship latch clears with `ShipRecoveryCode`.
- `PhaseRequest.WorktreeWritablePaths` carries them to the debugger of that recovery and to no other dispatch (`worktreeWritablePaths`). `withWorktreeFence` now sets both fence fields for the live, the resume and the evaluate-batch dispatch, which also keeps both dispatch roots within their size-ratchet allowances; `rebaseWithDerivedRegen`'s allowance drops from 65 to 63. The debugger's prompt lists them as `conflicted_paths`, and its persona, the one statement of what they mean, says how to resolve them.
- The conflicted paths are read with `git diff --name-only -z`, so a path git would quote (`"caf\303\251.md"`) reaches the fence as the raw path its `diff-tree -z` compares; read quoted, the fence would have reverted the resolution of such a file. `treefence.Begin` takes writable paths: the fence keeps writes to exactly those paths (`Outcome.Kept`, reported by name), restores every other write, and verifies that the final tree differs from the snapshot only on them. The runner's fence passes the request's paths.
- Tests, each red first: `TestFence_KeepsTheWritesToItsWritablePathsAndRestoresEveryOther`, `TestFence_WithoutWritablePathsRestoresTheSameWriteAsBefore`, `TestOutcome_DiagnosticsNameTheWritesItKept`, `TestRebaseWithDerivedRegen_NamesEveryNonDerivedConflictAndNoDerivedOne`, `TestRebaseCycleBranchOntoMain_ARealConflictNamesTheConflictedFile` and `TestRebaseRecordingConflicts_TheCycleRemembersWhatItsDebuggerMayResolve` (integration tier), `TestLatchShippedState_ForgetsTheConflictsOfTheRecoveryThatLanded`, `TestWorktreeWritablePaths_OnlyTheDebuggerOfAFleetRebaseMayWriteItsConflicts`, `TestResumePath_TheDebuggerOfAFleetRebaseMayWriteItsConflicts`, `TestRun_ADispatchKeepsItsWritesToThePathsItMayWrite`, `TestHooksComposePrompt_NamesTheConflictedPathsTheDebuggerMayWrite`, `TestFence_KeepsAWritablePathTheDispatchCreated`, `TestFenceVerificationRejectsIgnoreRuleEvasionBesideAWritablePath` and `TestRebaseCycleBranchOntoMain_NamesAConflictedFileAsTheFenceSeesIt` (integration tier), and end to end `TestRecoverFromShipError_TheFenceKeepsTheDebuggersResolutionOfTheConflictedFile`: a genuine conflict, a debugger held by the production fence, the kept resolution re-authored and re-audited, then shipped on main.

## Fixed — the identity carry reads the base the audited worktree stood on (ADR-0105 B3, 2026-09-28)

- Cycles 1724 and 1727 (wave 21) each met a moved main at ship, rebased byte-identically, and paid a second audit: the carry declined because "the audit was bound on X, not on the base Y the change was authored on". `X` was the auditor row's `git_head`, which `recordAuditBinding` resolves at the project root, so it is main's HEAD when the audit was bound; a sibling's landing had moved it while the audited worktree still stood on `Y`.
- The auditor row now records `worktree_base_sha`, the cycle's worktree base (`core.LedgerEntry.WorktreeBaseSHA`, `auditledger.Entry.WorktreeBaseSHA`; additive and `omitempty`, and the chain hashes raw lines, so older binaries verify it). `auditledger.Entry.AuditedBase` projects the audited base (the worktree base, else `git_head` on a row written before the field, the old and more conservative rule), and the carry's `carriedAudit` requires it to equal the base the change was authored on. A row that names no base now declines; before, it skipped the check. The decline says "the audited worktree stood on X".
- Tests: `TestRouteRebasedExplanation_AMainThatMovedBeforeTheAuditStillCarries` (red before: the carry declined and the route re-audited), the declines `the audited worktree stood on another base` and `the audit row names no base`, `TestCarriedAudit_ARowNamingNoBaseNeverCarriesEvenOnAnEmptyBase`, `TestEmitPhaseBindings_TheAuditRowNamesTheBaseTheAuditedWorktreeStoodOn` (red before: the row had no base), `TestEntry_AuditedBasePrefersTheWorktreeBaseAndFallsBackToTheHead`, and `TestLedgerEntry_EveryFieldSurvivesTheLedgerRoundTrip`, which fills every `LedgerEntry` field by reflection and fails when the wire struct drops one (the review's surviving mutant). A mutant that drops the `git_head` fallback is killed by the legacy-row decline.
- Size ratchet: the audit exit-code mapping moved into `auditBindingExitCode` and the ledger's polymorphic `cycle` parse into `LedgerEntry.setCycle`, and the carry's row checks into `carriedAudit`, so `recordAuditBinding` is 94 lines (allowance lowered from 103), and `LedgerEntry.UnmarshalJSON` and `identityCarryForward` left the offenders list. The rationale the moved comments carried is in the package notes' Invariants (`docs/architecture/packages/internal-core.md`).

## Changed — the architecture reviewer audits test-first, Clean Code and Design Patterns, and proves its test findings with mutants (2026-09-28)

- The reviewer (`.claude/agents/architecture-reviewer.md`) judged structure but not whether a change was built test-first to the project's *Clean Code* and *Design Patterns* standard. It also asked for doc comments that the comment convention forbids. It now ends every review with a three-book audit: each added or changed behavior mapped to the test that pins it; each bug fix to its regression test, run on the pre-fix code; function caps, flag arguments and every added comment; and the pattern each change names, checked against its structure and SOLID. The audit also carries a mutation sweep: up to ten single-idea mutants run in a scratch copy after a control mutant, covering the classes a deletion misses (reorder, shift, wrong key, wrong tense, a literal equal to the tests' value). An unpinned change and a missing regression test are CRITICAL. A surviving mutant of correct, tested code is HIGH with fix-then-merge. Guards nobody can trigger and design questions no requirement asks go under "Declined to judge", and each review names the one change that matters most. New and changed code carries no comments (`docs/conventions/code-comments.md`); the builder persona and the engineering-craft references say so.
- Chosen by a blind comparison with the five most popular online review prompts (superpowers, addyosmani's agent-skills, ECC, Anthropic's architecture-critic, mattpocock), on three scenarios and scored by judge agents: [architecture-reviewer-comparison.md](docs/research/coding-craft-2026/architecture-reviewer-comparison.md).
## Fixed — a cycle the debugger ends is a FAIL, so its claim is released (ADR-0105 B5, 2026-09-28)

- Cycle 1725 (wave 21) met a genuine fleet-rebase conflict, the debugger's resolution was reverted by the read-only worktree fence, and the re-entered rebase aborted. The cycle ended with the audit's WARN: the closeout walks only a FAIL, so its inbox item stayed claimed in `processing/`, and the consecutive-fail breaker never counted a cycle whose ship never landed. A cycle the debugger ends (a BLOCK, an empty or unknown decision, or any re-entry abort) now ends FAIL with a ship fail reason, so the failure walk releases the claim for a retry and the verdict-coherence floor reads the FAIL as diagnosed. The append-only `go/.apicover-enforce`, where both lanes' package lines collided, merges as a union (`.gitattributes`); the fence that keeps the debugger from writing a resolution is tracked as `fleet-rebase-debugger-resolution-fenced`.

## Fixed — the wave planner plans only lanes the launch gate accepts, and a wave that launches nothing is not a wave (ADR-0106 W3, 2026-09-27)

- Five places decided whether a fleet lane may take an inbox id now, with five rules: the backlog reader ignored declared deps, the two carried-id prunes disagreed about processing, retry and quarantine, the launch gate checked lifecycle and deps, and the refill ranked by weight alone. Wave 19 lost its second lane and wave 20 planned two lanes blocked on a console-owned dependency; the gate refused both, the refill found nothing, and the loop printed `wave N: 0/0 lanes ok` six times until `max_cycles`. One rule, `inboxmover.ResolveDispatchability`, now answers every reader from one root (`EvolveDir`): `triagecap.ReadInboxBacklog` admits only items whose deps are met, `triagecap.PruneUndispatchable` and the loop's plan-time prune drop exactly what the gate refuses, the refill ranks with the shared `triagecap.RankForDispatch`, and the triage menu, `evolve inbox batches`, the planner's backlog and the no-work check (lane-scoped and sequential) read one per-item judgment (`inboxmover.PlaceOnLaneMenu`, injected into core as `WithLaneMenu`), so no menu offers a waiting item and an empty commitment over withheld work is honest no-work rather than a misattributed claim failure, while a cycle's own claim still counts as the work it took. The no-work check fails closed only on an inbox file it cannot read, never on a notice about a readable item (a sanitized field, a duplicate id): `inboxbatch.ScanDir` types the loader's warnings, and `LoadDir` is its projection. `Ran` means a lane launched, so a launch the gate empties takes the min-width repair and the empty-backlog path, and a zero-lane wave counts toward work-supply starvation. Two `go/acs/cycle1182` predicates that run renamed tests by name were repointed (a `go test -run` pattern that matches nothing exits 0). The review of waves 11–20 behind it is the design document's §5.10.

## Changed — a debugger's conflict resolution rejoins the fleet rebase instead of a stale-base reship (ADR-0105 B5, ADR-0106, 2026-09-27)

- Cycle 1719's ship met a moved main and its fleet rebase hit a genuine conflict (both lanes had edited one doc). The host aborts such a rebase and hands the debugger a clean tree on the base the ship diverged from; the debugger resolved the file in place, and nothing re-ran the rebase — the cycle re-audited on the stale base and only its next ship rediscovered the divergence, paying an audit round and a ship attempt. Now, when the debugger's recovery began as a fleet rebase, the orchestrator — inside the ship-recovery budget, after the contention backoff, never on an in-place worktree — carries the resolved worktree's tracked state as a commit on the fork point, rebases it onto main, pends it on the new base and routes it as any rebased change (a byte-identical change carries its verdict to ship, otherwise Build re-authors the explanation and Audit re-verifies); a Build or Audit re-run the debugger asked for still runs, on the rebased tree, while a tests-first request yields to the route and says so. A conflict that survives the resolution, a rebase the host cannot finish or a spent budget ends the cycle on stderr and as `ORCHESTRATOR_REBASE_REENTRY_ABORTED` in the Signal Center (`docs/architecture/signal-codes.md` regenerated). Finishing an interrupted re-entry on resume is designed (B5b). (`internal/core`; `TestRecoverFromShipError_ADebugger*`, `TestRecoverFromShipError_AConflictTheDebuggerLeftEndsTheCycleWithoutAReship`, `TestResumeFleetRebaseAfterDebugger_*`.)

## Fixed — a capacity deferral in the sequential loop is nobody's failed approach (ADR-0106 Q3, 2026-09-27)

- The sequential dispatcher recorded a walled cycle in `state.json`'s `failedApproaches` (which triage's demotion reads) and ran the failure closeout before it noticed the wall and paused. It now pauses first: a deferral records nothing, closes nothing and counts toward no breaker; the wave path already behaved this way because its breaker reads only failure digests, which a deferral never writes.
## Changed — the ship's importer backstop re-runs a red package by itself before it is the lane's RED (ADR-0106 F7, 2026-09-27)

- Cycle 1718's ship went RED on `bridge/channel.TestChannel_EndToEnd`, a timing-window test in a package the cycle never touched, under the backstop's 89-target load; the ship aborted to a re-audit and a second attempt. When the importer backstop's reds are all named tests and at most three, each red package is now re-run by itself — the whole package, so same-package state stays in play and only the pack's concurrent load is removed; the argv and each package's outcome go to the scan log. Green by itself is flake evidence, said as a `ship.warning` `SHIP_BACKSTOP_FLAKE` in the Signal Center and a stderr line naming the tests (one re-run without the load is evidence, not proof), and the ship proceeds; still red is the lane's RED naming what is still red; a package whose re-run named nothing keeps its first-run reds; a build failure is never re-run. The scanner pack and the added-test backstop keep no such rung. Pack failures are structured (`packFailure{Package, Test}`) so the ship error's names and the re-run's selection come from one classification (`internal/phases/ship`; `TestClassifiedPack_*`, `TestRunRepoContractTestsAlone_*`, the wiring proof `TestRepoContractGate_AnImporterRedThatIsGreenAloneShipsAsFlakeEvidence`; `docs/architecture/signal-codes.md` regenerated for the new code).

## Fixed — a cycle's own explanation record on a non-material diff is its explanation, not history (ADR-0106 X3, 2026-09-27)

- Cycle 1718 built a test-only change and explained it; the build floor's no-material branch counted the cycle's own record among the "immutable cycle explanation records" and re-dispatched the builder to delete it. Only foreign cycle records are immutable now, in every branch and with one message (`published cycle records are immutable`): a no-material diff whose report declares `REQUIRED` is verified like any explanation with an empty material set, and one declared `NOT_APPLICABLE` keeps an undeclared own record as documentation. The Verify mirror branches on the recorded status; its foreign-record check was dead behind the diff-SHA check and is gone (`internal/explanationdocs`; `TestNonMaterialDiff_*`).
- `bridge/channel.TestChannel_EndToEnd` failed twice in 1718's lane under the ship backstop's load (its two ticks were separated by a 10 ms sleep, and both landed in one poll); it now waits for the first tick's envelope in the feed before writing the second.

## Changed — a byte-identical rebase ships on its audited verdict (ADR-0105 B3/B4, ADR-0106 P1, 2026-09-27)

- Cycles 1712 and 1715 each passed their audit, met a sibling's closeout dossier commit on the plane's `main` at ship, rebased byte-identically and then re-audited an unchanged tree for seven to ten minutes. The rebind route returned Audit and never reached the carry-forward rungs, the old RUNG 0 diffed commits so a pended change could never match, and ship had no reader of carry records. Now `identityCarryForward` runs after the rebind: the auditor row names the audited tree and artifact, `treedelta.Identical` (a new leaf) proves the pended tree's change is byte for byte the audited one, the composed gates run under the fence with the index checked intact, and an `identical-rebase` record naming the audited tree goes to the root ledger; ship's one binding rule then re-proves the record (ancestry, bytes, green gates) and accepts the tree. Any doubt on either side is the old path: a second audit.

## Fixed — a top_n card on a protected surface is routed by the host, never the cycle's FAIL (ADR-0106 R1, R2, 2026-09-27)

- Cycle 1714 (wave 16) was pinned to an inbox item with no declared files; its triage named `go/internal/core/orchestrator.go`, the classify hook refused the report with `TRIAGE_PROTECTED_SURFACE`, the cycle sealed FAIL after two phases and the failure path routed the item to the console. The hook now moves every such card out of `top_n` into `escalate_block` with the reason `protected-surface: <path> — control-plane changes go through the console route (operator-gated), not lane top_n` and returns PASS with one warning per card; an emptied `top_n` is the planned no-work end and the no-work closeout routes the item with that reason, while other cards continue the cycle. A route that cannot be recorded fails closed. The triage prompt now lists the protected surfaces (`guards.ProtectedSurfaceManifest`) and the drop reason, and the persona carries the rule. Design: `docs/architecture/logic-first-delivery-design.md` §5.7 and §7.8.

## Fixed — an inbox lifecycle record is non-material to the explanation document (ADR-0106 X1, 2026-09-27)

- Cycle 1712 sealed FAIL with its code verified (ACS 10/10, EGPS red 0, a mutation probe): the build floor had demanded that the explanation document describe `.evolve/inbox/…-lineage-datestamp-normalization.json` (twelve corrections across 1707, 1708 and 1712), the builder wrote "content unchanged" for the item it moved to `consumed/`, the ship's in-commit consumption stamped the file, and the auditor's review found the sentence contradicted. `.evolve/inbox/` joins `nonMaterialPrefixes`: the host claims, moves, stamps and retires those records, so neither the floor nor the auditor asks the builder to explain them.

## Fixed — a shipped or declined id no inbox item backs is retired, so no later wave re-pins it (ADR-0106 W1, 2026-09-27)

- Cycle 1706 committed and shipped `gittest-fixture-centralize`, an id its triage authored with no inbox item behind it. `Promote` no-ops on such an id and no retirement dir ever held it, while the wave planner's prune and the launcher's freshness probe keep an id with no lifecycle evidence, so the prior-decision carry re-pinned it as a lane in 1709, 1710 and 1713, and each of those lanes shipped nothing. `lifecycle.(*Mover).RetireUnbacked` now writes a retirement record (`{id, unbacked, retired_reason, retired_cycle, git_sha}`) where `Promote` would have moved the item; `inboxmover.RetireUnbacked` is the one door and writes only for an id whose dispatch state is unknown; the PASS seam retires what it could not move as processed (`OutcomeResult.RetiredUnbacked`, the ship log names them) and the planned no-work closeout retires every scoped id no inbox item backs as rejected with the lane's own answer (`NoWorkResult.Retired`). A FAIL writes nothing, so the id stays retryable. Design: `docs/architecture/logic-first-delivery-design.md` §5.6 and §7.7.

## Fixed — an empty optional bucket in triage-report.md reads as no cards (ADR-0106 H1b, 2026-09-27)

- Cycle 1713's triage wrote `## superseded` with nothing under it; the strict reader declined the report ("states neither cards nor none"), the host had nothing to derive, and the contract gate rejected the phase with `missing_secondary` and re-dispatched it. `deferred`, `dropped` and `superseded` left empty now derive to `[]`; an empty `top_n` is still a missing commitment and still declines.

## Fixed — a credential wall benches the family and names the operator's fix (ADR-0106 Q2, 2026-09-27)

- `clihealth.Benchable` admits `auth_recheck` (`CredentialPattern`): wave 14 re-dispatched claude fourteen times into "Please log in". Unlike a quota bench it stays active for routing after its cooldown (the cooldown only schedules the canary's next probe; a succeeding probe after the operator's login clears it), a login pane's stale reset hint never sets it, and while it stands, `clihealth.OperatorAction` names the fix on the chain walker's bench line and the canary's re-bench line.

## Fixed — a quota wall on a correction, a remediation re-run or a resumed review defers the cycle (ADR-0106 Q1, 2026-09-27)

Cycle 1708 met every CLI family walled on its build correction re-dispatch and was sealed FAIL; cycle 1709 met the same wall on a first dispatch and was deferred.

- `isQuotaWall` is the one classification of a walled dispatch: the runner walks its whole family chain inside a single `Run` and returns exit 85 only when every family it may use answered with a wall. The correction loop, the remediation re-run and the resumed review gate now return through `pauseForQuota` — the quota checkpoint, the `all_families_exhausted` ledger kind, the DEFERRED abort reason, no failure learning, no retrospective, no digest for the consecutive-failure breaker to count. The interaction ledger records the correction as `quota_deferred`.

## Docs — ADR-0106 P3 landed; the design document, the bridge pages and the manifest channel (2026-09-27)

- `logic-first-delivery-design.md` §5.4 records the P3 decisions (the statement is finished by the driver and appended; the environment channel over a settings flag; what the block does not fix), the P3 row reads shipped, §8 carries its signatures, §12 the open question on other CLIs' suggestion features. `internal-bridge-phaseidentity.md` is new; `internal-bridge.md`, the packages index, `full-tmux-control.md` §9, ADR-0022 and ADR-0106 describe `default_env` and the identity statement. The F3 routing bullet that a replay placed above its heading sits under it again.

## Changed — phase panes run without prompt suggestions (ADR-0106 P3, 2026-09-27)

- The tmux boot sends a manifest's `default_env` as `export` lines after `cd` and `EVOLVE_PROJECT_ROOT` and before the launch command, because a pane inherits the tmux server's environment, not the bridge's. `claude-tmux` declares `CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false`: a suggestion is a background model request per turn and dim text under the input box that reads like agent output (cycle 1707 rendered "Yes, kill that session first." there). The environment variable takes precedence over the setting, and unlike a `--settings` flag it is honoured under `--setting-sources project`.

## Changed — the pasted prompt ends by stating who the agent is (ADR-0106 P3, 2026-09-27)

- `prepareTmuxREPL` appends `phaseidentity.Block` to the bytes it writes to `resolved-prompt.txt`, after the engine's composed prompt, which stays a byte-identical prefix; the deliverable path stays the last thing the agent reads. Only the driver knows the session, so the statement is finished here, for every tmux CLI. A launch without an agent name pastes the prompt alone. The real-tmux transit test now compares against the file the driver pastes.

## Added — a CLI manifest declares its process environment (`default_env`, ADR-0106 P3, unwired, 2026-09-27)

- `default_env` is to variables what `default_args` is to flags. The realizer copies it into `Realization.Env`; `driverEnv` layers it between the process environment and the request's `Deps.Env` overrides (the last value of a key wins); `exportLines` renders the `export KEY=value` lines a tmux driver will send, sorted and shell-quoted. The parser refuses a key that is not a shell identifier, a key that belongs to the loop (`EVOLVE_`), the bridge (`BRIDGE_`) or a credential (the guards read the process environment before a launch and never see a manifest's map), and a value carrying a control byte.

## Added — `internal/bridge/phaseidentity`, the statement of who the agent is (ADR-0106 P3, unwired, 2026-09-27)

Cycle 1707's tdd agent listed the tmux sessions, found its own, read its own prompt file, and refused the phase as a prompt injection racing "the real agent"; an operator's one-line identity clarification resumed it an hour later.

- `Block(Facts)` renders five lines a tmux driver appends to the bytes it pastes: the phase and cycle; the tmux session and the command that prints it; the two prompt files and that finding them, or the session in `tmux ls`, is expected; the sole-writer fact and the standing instruction not to kill, pause or hand off the session or wait for an operator; and that instruction files addressed to the console operator describe the operator's sessions, not this one. `""` without an agent name or a session; every fact is stripped of control bytes and backticks before it is rendered. Pure, standard library only, golden-pinned, in `.apicover-enforce`.

## Added — the recovery agent's profile and persona (ADR-0106 F3, unwired, 2026-09-26)

- `.evolve/profiles/deliverable-recovery.json` runs sandboxed over a read-only repository with the run directory as its only write grant, so a helper launched without a worktree is wrapped rather than unconfined. It declares no network, but the wrapper forces the network on for every dispatch today (`sandboxPrefixForLaunch`, filed as `sandbox-wrapper-forces-network-on`) and the tmux drivers enforce no tool list (filed as `tmux-drivers-ignore-profile-tool-lists`), so the filesystem grant is the boundary that holds; the test pins the forced-on launch path so it flips when the wrapper honours the declaration; `agents/evolve-deliverable-recovery.md` states the agent's identity and sole-writer fact up front (cycle 1707's TDD agent refused its own task for an hour, taking itself for an intruder) and forbids inventing, editing code, or deciding a verdict. Tests pin that no grant reaches the repository beyond the run dir or any worktree.
- The profile runs on the codex family with claude as its fallback, as the other helpers do: the balanced-tier floor (`TestClaudeFamilyFloor`) reserves claude for judgment phases with a justification, and the recovery agent decides nothing. The first ship routed it to claude and the full floor caught it.

## Added — `internal/recoveryguard`, the kernel fence for a recovery dispatch (ADR-0106 F2, unwired, 2026-09-26)

- `Begin(ctx, Scope)` records every entry in the run's workspace and fences the change's worktree (`treefence`); `End` restores whatever the agent changed, planted or removed outside `Scope.Allowed` and reports it: a changed or removed file, a planted one, a file swapped for a link or a directory, an allowed path swapped for a link. Telemetry the dispatch appends is unfenced by exact path or below a directory (`Scope.Unfenced`); artifacts it creates with generated names are tolerated by stem, direct children only, and reported in `Outcome.Unfenced` (`Scope.UnfencedStems`). A non-empty `Outcome.Restored` is an integrity violation for the caller to abort on. Fails closed on a worktree that cannot be fenced, a workspace that cannot be read or an allowed path that is not a plain file. Graduated to `.apicover-enforce` and listed in the protected-surface manifest.

## Added — a recover rung in the correction ladder (ADR-0106 F1, unwired, 2026-09-26)

- `interaction.NextCorrection` gains `RungRecover` between live-fix and re-dispatch, taken only when the caller reports the violation repairable (`CorrectionInput.Repairable`) and budget remains. The ladder does not execute it yet.

## Added — the host derives a declared secondary before any judge (ADR-0106 H2, 2026-09-26)

- The registry declares what is derivable: `outputs.derived_from` maps an agent-owed secondary to the primary the host derives it from. The partition validator requires the file to be agent-owed and the source to be the primary; the descriptor schema documents the field.
- `HostEffects.Perform` derives each declared file the agent left absent or empty after the host effects and before the runner's judge and the gate, so both judge a complete deliverable. A file the agent wrote, or is still landing, is never touched: the read waits out a write in flight as the gate's does, and a read fault declines. The lane-pin check therefore guards only the derived file; a decision the agent wrote is judged by the gate as before. A decline is a WARN (`ORCHESTRATOR_HOST_EFFECT_FAILED`) and leaves the absence for the gate. A declaration without a registered deriver fails the deliverable tests.
- `triage` declares `triage-decision.json` derived from `triage-report.md`. Docs: `deliverable-contract.md`.

## Added — triage's decision derived from its report (ADR-0106 H1, 2026-09-26)

`triage-decision.json` states the commitment the report already carries in prose. Cycles 1672, 1687, 1697 and 1707 re-ran the whole phase because the agent left the file absent.

- `internal/triagedecision` is the one reader of the report. `Derive(report, cycle, lanePin)` is the strict mode the host writes before any judge: `## top_n` stated, every present bucket readable (slug ids, cards or a `(none …)` line, nothing else), pinned lane items committed; an absent `deferred`, `dropped` or `superseded` section is empty, which commits more, never less. `Project(report, cycle)` is the lenient companion ship already wrote, moved here with its parser (`triagecap.ProjectDecisionJSON` delegates), so the report has one grammar: `- {id}: {action} — key=value` tails, `files=` footprints, `reason=`. Both carry the one stamp `projected_by_orchestrator`. Nothing the report does not state is written. The package is a protected surface: a lane must not soften what it commits itself to.

## Fixed — an exit-85 escalation names its pattern (ADR-0106 P4, 2026-09-26)

Exit 85 covers the auto-responder's escalations and the corroborated quota wall, and the numeric exit table is frozen, so the attempt ledger read every one as `unknown_prompt`. Over cycles 1673–1707 all eight were named by their escalation reports: four `rate_limit` walls and four `model_unsupported`, the codex deep pin the account rejects (incident 2026-09-14).

- `launchoutcome.Classify` lets the escalation pattern ride `cause_code` and the launch error line on exit 85, as the 81 sub-causes do: `rate_limit`, `model_unsupported`, and `unknown_prompt` only for a prompt nobody recognised. The exit class, its signal code and its transient sentinel are unchanged; a loop-guard report is not an escalation, and both markers are read only at the start of a line, where only the host writes.
- `signal-codes.md` is regenerated; the 2026-09-14 incident carries the update.

## Added — logic-first delivery: phases own the logic, the pipeline owns the form (ADR-0106, operating-policy §0, 2026-09-26)

Over cycles ~1550–1707 a byte-identical change re-ran Build and Audit because a peer landed first (14 cycles), a derivable secondary sent a whole phase back (4 cycles, still recurring), and cycle 1707's TDD agent refused its own task for an hour over a process misunderstanding. None was a defect in the change. The operator's direction, P0: phases focus on logic; a format or process failure is recovered, never blocking.

- Operating policy §0 states the rule (numbered so that no historical citation of §1–§7 moves): three kinds of block (logic, form or process, integrity) and only logic and integrity are final; recovery is layered cheapest first (a host derivation before any judge, deterministic rungs, the recovery agent, a re-dispatch), never launders (the gate re-judges, the verdict is re-earned, the kernel proves the grant), decides between assigning back and recovering on kernel-checkable evidence that the request is fulfilled, and is bounded and loud.
- [ADR-0106](docs/architecture/adr/0106-logic-first-delivery.md) records the evidence, the decision, the routing of every violation code, the recovery agent's guards, and the evidence decision (`evidence.Sufficient` over kernel inputs; a missing deliverable is always assigned back), and the component table in landing order: the process rungs (ADR-0105, the closeout dossier, agent identity in prompts, exit-85 causes), the host derivations, the evidence decision wired into the re-dispatch, the verdict refresh and the shared ladder, then the recovery agent.
- Review: architecture review, APPROVE-WITH-CHANGES; every finding is folded in (a helper without a worktree runs unsandboxed unless its profile says otherwise; a verdict-equality guard is unsound for phases whose verdict is their sections; a run-dir grant reaches proof of read; re-derivation belongs before the judges; resume has a separate ladder; extra re-reviews demote the gate; exhaustion must not weaken the ADR-0072 breakers).

## Added — a fleet rebase replays the audited change, not ship's commit (ADR-0105 rung B1, 2026-09-26)

When a peer moved main between a lane's audit and its landing, recovery rebased ship's own commit, which carried ship's inbox consumption. Every cycle then returned to Build, and the rebuilt explanations had to explain inbox moves. B1 undoes ship's commit before the rebase. It lands as five commits, one component each:

1. **Fork point** (a fix): the rebased explanation binds to `merge-base HEAD main`. Before, it bound to main's tip, which a later landing moves.
2. **`latestAuditedTree`** reads the tree the newest audit reviewed (`T0`), by the same row rule ship binds (`auditledger.IsAuditorRow` + `BindRun`).
3. **`unwindShipCommit`** replaces ship's commit with a carrier of `T0` on the audited base. It declines, with a logged reason, unless:
   - the worktree is clean, untracked files included;
   - the lane forked at the audited base;
   - git holds `T0`;
   - the change since audit is exactly ship's inbox consumption pairs;
   - no consumed item released a continuation.

   After the rebase, **`pendRebasedChange`** soft-resets the carrier to the fork point, whatever the rebase did.
4. **`routeRebasedExplanation`** tries B2's identity-preserving rebind only when the change is pending on its fork point: the Auditor reads `git diff HEAD`, which would be empty for a committed change. A proven-identical change returns to Audit with no Build; anything else returns to Build.
5. **Wiring**: contract cycles only, and never when the pre-screen predicts a conflict.

- Reviews: architect, security-reviewer and go-reviewer, two rounds each. Round 1 blocked on the committed-change hazard and a stranded carrier, both fixed.
- Follow-ups: `resume-heals-a-carrier-left-at-head` (required before B3/B4) and `consumption-releases-the-continuation-binding-before-the-landing`.

## Fixed — a Build may retract a draft it never committed (cycle 1705, 2026-09-26)

Cycle 1705 passed audit and still failed. `guard:docdelete` denied the Builder's removal of its own never-committed explanation draft, and its deny message prescribed a plain `mv` into the archive home. That left the draft untracked, and the host predicate gate refuses untracked inputs. So it forced FAIL over the auditor's PASS.

- `docdelete` lets a Build `rm`/`git rm` exactly one path: the active cycle's own explanation document, from the repository root, when `HEAD` never held it (compared case-folded) and no parent directory is a symlink. Any other operand, spelling, expansion or pathspec is denied as before.
- The deny also closes older holes: the doc roots match case-folded and as bare words (`rm -rf docs`), and an `rm` or `mv` after `cd`/`pushd` into a doc root, under `git -C docs`, or from a shell already inside one is judged.
- The deny message advises `git mv`, so an archived copy stays staged.
- Record: `docs/incidents/2026-09-26-the-doc-guard-sent-a-draft-where-the-predicate-gate-refuses-it.md`.

## Added — an identity-preserving rebind for the Build explanation (ADR-0105 rung B2, 2026-09-26)

A clean fleet rebase re-runs Build and Audit today for every cycle, even when the change is byte-identical (cycles 1698 and 1701). The first rung that lets a passed audit survive is the explanation rebind; nothing calls it yet. B1 (unwind), B3 (the repaired trivial-rebase rung) and B4 (ship accepts the carry) follow.

- `explanationdocs.RebindIdenticalRebase(ctx, binding, newBase, persist)` moves an approved Build contract to a new base without a Build. The host must first prove, from one read of the worktree:
  - the new base descends from both the authored and the bound base;
  - the peer delta touches no lane path (case-folded; any path that is not plain — non-ASCII, containing `:`, `\` or `~`, or with a component ending in a dot or space — counts as touching) and no `.gitattributes` or `.gitignore`;
  - the lane's paths, modes and bytes equal the sealed digest;
  - the material digest and the Build report declaration are unchanged;
  - the document is absent at the new base.
- On a proof, only derived fields move: `base_sha`, `diff_sha256` and a new `authored_base_sha`, which is set once. Writes go marker, checkpoint, handoff, snapshot. `ErrRebindIncomplete` marks a write-phase error that may leave a split `RecoverRebaseSplit` recognises; any other error means nothing was written.
- `Verify` accepts the document's authored base only after re-deriving that lineage itself. A view without the field is checked exactly as before.
- The host never re-seals a rebound handoff. `SealResult` is a no-op on an unchanged one, `RefreshResult` routes any change to Build, and a Builder's re-authored seal replaces the rebind.
- Reviews:
  - architect: APPROVE-WITH-MINOR, including a single read for proof and binding;
  - security-reviewer: BLOCK twice on path aliases a filesystem resolves to a lane file (APFS NFC/NFD; NTFS/exFAT/SMB trailing dots and spaces), fixed by declining every path that is not plain.
- `build-explanation-contract.md` now defines Build Binding as the authored base.

## Fixed — plane bookkeeping no longer sends a passed audit back to re-audit (cycle 1701, 2026-09-26)

Cycle 1701 passed audit and was sent back at ship with `AUDIT_BINDING_TREE_MISMATCH`: a git-tracked inbox item in the plane had changed between its audit and its ship. Ship's binding hashed the plane's `git diff HEAD`, although a fleet lane ships from its own worktree. The plane's inbox queue, which operators and sibling lanes move all the time, was never part of what the lane commits.

- `verifyAuditBinding` compares the plane-wide tree state only when the plane is the tree ship lands from. The tested root and that choice come from one decision. A worktree ship stays bound by the treefence snapshot of the worktree it commits, so a change to the shipped worktree after audit is still refused.
- Proposed next: [ADR-0105](docs/architecture/adr/0105-identity-preserving-fleet-rebase.md) keeps a passed audit across a clean fleet rebase of an unchanged change. Today every such rebase re-runs Build and Audit (1698, 1701), and review found the existing trivial-rebase rung has never fired in worktree mode.
- Records:
  - `docs/incidents/2026-09-26-a-passed-audit-sent-back-by-ship-process-issues.md`;
  - `docs/research/2026-09-26-identity-preserving-rebase-design-review.md`.

## Removed — the research quota guard, which never denied (2026-09-26)

`evolve guard quota` built a fresh counter for every hook call, so its caps never outlived one call. It also keyed them on an `agent` field that no Claude Code tool input carries. Its hook matched `WebSearch|WebFetch|Bash`, so every Bash call paid for a process and a `policy.json` read that decided nothing.

- The guard, its `.claude/settings.json` hook, its `guards.log` tag and `workflow.allow_deep_research` are removed. `policy.Load` ignores the key in an existing `policy.json`.
- `TestHookWiring_EveryWiredGuardIsAGuardTheBinaryBuilds` keeps every wired `evolve guard <name>` buildable, so a future removal cannot leave a hook calling a guard the binary lacks. A hook that did would exit 10, which Claude Code shows but does not treat as a deny.
- The guards suite's `TestMain` unset three bypass variables nothing reads. It is replaced by `TestGuards_ReadNoEnvironmentButHome`, which pins that the guards read no environment but `$HOME`.

## Fixed — the guards judge the clean path, and an `evolve ship` allows only itself (P1, 2026-09-26)

Reading the guards during comment-reduction batch 19 found three ways past them, none of which needed an exploit to see. A build or tdd Write to `<worktree>/go/internal/core/../guards/role.go` missed the protected-surface fragment and was allowed. `/tmp/../x` counted as always-safe scratch. The ship guard allowed any line that mentioned `evolve ship`, so `git push origin main; echo evolve ship` passed, and a quoted or arithmetic `<<` hid the next line from it.

- `role.go` cleans the path once, before any decision. `normalizeSurfacePath` cleans too, keeping a directory's trailing slash, so every caller of `IsProtectedSurface` / `IsProtectedScope` (routing, the ship tripwire, the build floor) judges the file a path lands on.
- The ship guard judges each simple command on its own. The new `splitShellCommands` (`guards/shell_commands.go`) is a quote-aware scanner that replaces the line-based heredoc stripper: command substitutions are commands of their own even inside double quotes, and comments and heredoc bodies are dropped. A heredoc opens only at an unquoted `<<` outside parentheses, never at `<<<`, and its delimiter is the whole shell word after quote removal. The old stripper read an identifier prefix, so `<<EOF-MARKER` hid every line after its close, found in review. `git -C dir push` and the other global-option forms are ship-class.
- The `scripts/lifecycle/ship.sh` allowance is gone. The script no longer exists, and an allowance keyed to a path an agent can write is a bypass.
- Three tests that passed for the wrong reason were rewritten, and every rule was mutation-checked.
- Out of reach, and recorded in `docs/architecture/packages/internal-guards.md`: verbs assembled at run time, and a binary an agent names `evolve`. The ship tripwire, the commit gate and ship's self-SHA pin remain the backstops.

## Fixed — a lane's cycle-state override no longer reaches the test fixtures it spawns (cycle 1700, 2026-09-26)

A fleet lane sets `EVOLVE_CYCLE_STATE_FILE` process-wide so its orchestrator and hooks share the lane's own state, and the resolver honored it for any evolve dir. Every `go test` the lane spawned therefore wrote its fixtures into the lane's live cycle state. That covered the EGPS predicates, the CI-parity gates and the tests agents run in their panes. Tests read each other's fixtures, which is why cycle 1700's "dashboard flakes under load" item and its audit went red. With the override exported, main's guards suite also replaced the live file with a fixture (`cycle_id 107`).

- `paths.CycleStateFileFor(evolveDir, override)` is the one rule, and `core.ResolveCycleStatePath` and `paths.Resolve` delegate to it. The override applies only when it lies inside the evolve dir being resolved, so a lane's own reads are unchanged, and a test's `t.TempDir()` evolve dir always gets its own file.
- Resume's per-run checkpoint discovery now stands down only for an override that governs its evolve dir, not whenever the variable is set.
- `evolve cycle run` refuses a fleet lane whose `--evolve-dir` is not `<project-root>/.evolve`, the one dir its override governs, instead of letting it fall back to the shared file.
- The whole module passes with a lane override exported, and the exported file is left untouched.
- Record: `docs/incidents/2026-09-26-lane-cycle-state-override-reached-test-fixtures.md`. Follow-ups are filed: `acssuite-lane-env-policy` and `audit-gate-forced-fail-earns-no-repair`.

## Fixed — routing sends no lane work its builder's sandbox forbids (2026-09-26)

Cycles 1696 and 1699 both failed at the build floor on the same item. It declared `.evolve/profiles/historian.json (new)`, but the builder profile's sandbox denies `.evolve/profiles`, so no lane could ever create the file. The ADR-0074 routing floor judged declared paths only with the integrity manifest, which names two specific profiles, and routed the item to a lane twice.

- `profiles.SandboxConfig.Denies` projects the sandbox's `deny_subpaths` as a path predicate. It is the same list the OS sandbox enforces.
- `lanerouting.Forbidden` combines protected surface with the build profile's `Denies`. `build.ProfileName` names that profile once, and the build agent's prompt name derives from it.
- `cmd/evolve`'s `laneForbidden` wires the one predicate into the loop's routing roots: the wave seed and widen, the host and CLI claim floors, and `evolve inbox batches`. A profile that will not load degrades loudly to protected surface only. Only an enabled sandbox counts, since that is when the bridge enforces it.
- Triage receives it through `triage.Config.LaneForbidden`, for both its prompt partition and its breaker on `top_n` cards. A scout-originated card naming a builder-denied path is refused before build.
- On the live queue, one more item now routes to the console: `skill-allowlist-for-skill-using-phases`, which edits two builder-denied profiles. No item left the console. Two known gaps remain, filed as a follow-up: core's sequential termination check and the triage registry factory.
- Docs: ADR-0074 amendment, the incident record, `docs/architecture/packages/internal-lanerouting.md`, and a REGRESSION-COVERAGE-INDEX row.

## Fixed — triage re-checks a queued item's premise against what changed since it was filed (F40, 2026-09-26)

Cycle 1691 picked an item filed on 2026-08-16. #535 had made its premise unreachable on 2026-09-09. Fault-localization and bug-reproduction accepted a unit fixture that fed the unreachable state straight to an inner function, and the builder "fixed" a non-bug and opened a fail-open. The audit caught it a full cycle later. A console audit of the next ten queued items found six whose premise or scope was wrong.

- `premise_drift` in a fleet lane's triage prompt (`phases/triage/premise_drift.go`). The item is found wherever triage's claim left it. For each scoped item the section lists:
  - the commits since it was filed that name its id;
  - those that touched its declared paths;
  - by subject, those that touched only their packages;
  - declared paths not at HEAD, checked with `git cat-file`, never the filesystem.

  It is framed as evidence, not a verdict. It is bounded and fail-open, a git failure shows as a visible line, and a sequential prompt is byte-identical. New `inboxbatch.Item.FiledAt` / `DeclaredPaths` / `StripControl`.
- Triage persona Step 0b: re-verify a drifted or older item's premise at HEAD before claiming it, and drop a stale one with `stale: <evidence>`. With F30, that drop ends the lane as planned no-work and hands the item to the console.
- A stale drop never retires an item: `inboxmover.ClosedDroppedIDs` no longer counts `stale` and classifies a reason by its leading tag. A lane's PASS ship can no longer consume a stale-dropped menu-mate.
- The fault-localization and bug-reproduction personas require production reachability. A fixture that bypasses an upstream reader is not a reproduction.
- Record: `docs/incidents/2026-09-26-stale-premise-reached-build.md`. The replay eval stays open on `stale-premise-reaches-build`, and the re-verification anchor, evidence checks and planner pre-filters on `premise-verification-anchor`.

## Fixed — a fleet lane whose triage answers for its scoped item ends as planned no-work and hands the item to the console (F30, 2026-09-26)

Cycle 1682's lane was scoped to one item that 1679 had already shipped. Its triage dropped the item with a reason, and the lane still sealed FAIL. The empty-commitment termination asked whether the *whole inbox* held claimable work, found other lanes' items, and relabelled the honest no-work end as a claim failure. The dropped item also stayed pending, so the next wave could draw it into another lane.

- `committedset.Dispositions` / `DispositionsFrom`: one reader of what triage *answered* for without committing it. An answer is an escalation, a rejection, a shipped skip with its sha, or a reasoned drop; the most severe bucket wins. A deferral is never an answer: cycle 1623 narrated a claim failure as one.
- `core` termination (`unansweredClaimableWork`, now given the cycle): a fleet lane is a claim failure only when a scoped item is still pending in the root **or claimed into this cycle's `processing/`** and answered nowhere. Otherwise it ends as planned no-work (SKIPPED). A load warning fails it closed only for a scoped item it cannot find. Sequential cycles keep the whole-inbox check.
- `cycleoutcome.ApplyNoWork` hands each scoped item the lane answered for to the console in place, with the lane's reason. It keeps an existing console route's evidence, then releases the cycle's claims to the root with no failure bump. An unshipped lane never retires work.
- Every root makes the one `closeoutCycleOutcome`: the cycle-run root, the sequential loop, and `evolve loop --resume`. Resumed FAILs now reach the inbox lifecycle too.
- Record: an addendum to `docs/incidents/2026-09-15-shipped-item-re-pinned-and-sealed-lease-blocks-refresh.md`. Follow-ups are filed: `decision-document-single-declaration`, `console-route-stamps-block-wave-sync` and `no-work-hand-off-rate-alarm`. The sequential nil-predicate half stays open on `triage-termination-scope-aware`.

## Fixed — the idle nudge says why a deliverable that is already right must still be rewritten (F39 part 1, 2026-09-26)

Cycle 1691's correction round fixed an explanation document and correctly left `build-report.md` untouched. The bridge's completion baseline (the deliverable's size and mtime at dispatch, the cycle-1550 stale-leftover guard) refused the unchanged report, and the one idle nudge said only "Please write the deliverable". The agent re-verified the report, found it correct, and stopped, and the phase would have closed as exit 81 on a correct cycle. The operator unblocked it by touching the report.

- `bridge.idleNudgeFor` words the reminder by what the host sees. It locates the deliverable through the completion poll's own resolution. When the deliverable still matches the dispatch baseline, the nudge says it was not rewritten during this attempt and that completion means writing it after this dispatch. It binds carrying it forward to a re-check against this attempt's work: a markdown report takes an appended line recording what changed and that it was re-verified, and any other format is written again in full. It is logged as `idle with an unrewritten deliverable` with trigger `idle_unrewritten_deliverable`. An absent deliverable keeps the plain reminder.
- Record `docs/incidents/2026-09-26-correction-stalled-on-an-unrewritten-deliverable.md`. Part 2, deterministic completion for correction rounds whose worktree changed, stays open as `correction-completion-needs-deliverable-rewrite`.

## Fixed — a dead agent pane gets one fresh session of the same CLI before the fallback chain moves on (F31, 2026-09-26)

F27 made the fatal-pane fast-fail act, so a dead pane leaves the wait in one interval. But the launch still exited 81 into the fallback chain. With codex quota-walled and ollama unable to write source, cycle 1687's triage had nowhere to go and aborted. A dead shell means the REPL process died, not the CLI or the account.

- `recovery.TerminalCause.SessionRecoverable` names the causes a fresh session can fix: `dead_shell` and `cli_self_updated`. A model/config cause fails the same way in a new session.
- The fast-fail verdict carries its typed cause (`ReviewVerdict.Cause`). The wait loop reports it through a call-local hook, and `Engine.Launch` records the dead dispatch (its ledger row marked `fresh_session_retry`; the pair shares `Attempt`), emits `BRIDGE_FRESH_SESSION_RETRY` and runs ONE fresh session of the same CLI. It never retries a named session (kept alive for resume, so a re-run would reattach to the dead pane outside the sandbox), a run that delivered, a canceled launch or one whose deadline has no room for another wait interval, and a second death returns exit 81 to the chain. Worst case for one chain attempt: the fast-fail plus one full wait budget.
- Pinned end to end through the real `Engine.Launch`: a first tmux session that dies to a bare shell and a second that completes return OK. ADR-0044 amendment; inbox item `dead-shell-same-family-fresh-session` consumed.

## Fixed — the host performs triage's inbox claim before the review, so a skipped claim no longer costs a correction (F36, 2026-09-26)

Cycles 1647, 1671, 1689, 1693 and 1694 each drew `GATE_CONTRACT_REJECTED [missing_effect] inbox-claim`: the triage agent committed its `top_n` and never ran `evolve inbox-mover claim`. Each recovered on correction 1, paying a full triage re-dispatch (47–111 s) and spending one of the phase's two corrections. A claim is a deterministic file move, so the host now performs it.

- `deliverable.effects` is one registry holding both halves of each effect, `{check, perform}`. `deliverable.HostEffects.Perform` receives the `core.ReviewInput` the gate receives and resolves it through the same `CatalogResolver` and `rootsFor`. So the host and the gate cannot disagree about which effects apply, what is owed, the cycle, or the inbox directory.
- The effects run before each judge. `runner.Run` performs them just before the verdict engine judges the phase, so a settle probe never sees a missing claim and never downgrades triage to FAIL. Every phase `Config` forwards a late-bound accessor, as it forwards `ContractVerifier`. `core.Orchestrator.performEffectsAndReview` performs them again (idempotent), then reviews. It replaces every reviewer call: the fresh root's initial, salvage and correction reviews, and the resume root's two. Each point reports its own failure: `RUNNER_HOST_EFFECT_FAILED` or `ORCHESTRATOR_HOST_EFFECT_FAILED`.
- The builtin spec-fallback runners (plan-review, doc-sync, spec-verify, architecture-design, tester and others) now get both the gate's verifier and the host effects. Before this they judged with a different verifier than the gate, the cycle-1685 class. The wiring pins now run over the repo's real personas and registry, so they can see these runners.
- `inboxmover.ClaimPending` claims the committed ids still pending at the inbox root, through the same ADR-0074 floor as the CLI. Absent, already-held and console-routed ids are left to the gate, with no false `INBOX_CLAIM_NOT_FOUND`. Every other failure is returned and becomes the WARN `ORCHESTRATOR_HOST_EFFECT_FAILED`; the review still runs.
- What the gate checks is unchanged. Its correction text now says the host's claim did not land, and tells the agent to defer or drop the item.
- Triage persona Step 0a.4 is advisory. The production root binds the performer, and `--simulate` never does (`Orchestrator.HostEffectsWired()`).
- Docs: ADR-0100 amendment 10, `deliverable-contract.md`, `runtime-reference.md`, `signal-codes.md`, and the incident record `docs/incidents/2026-09-26-triage-claim-left-to-the-agent.md`.

## Fixed — the build handoff floor refuses a protected control-plane edit, and a ship refusal for one goes back to build (F37, 2026-09-26)

Cycle 1689's builder rewrote the protected `go/internal/core/cyclerun.go` through a shell tool, which the Edit/Write role guard never sees. The build floor approved the diff, and only ship's integrity check (ADR-0064 P2) would have refused it, after a full audit. That refusal would then have recovered into a re-audit of the same diff until the budget aborted the cycle. The operator stopped the lane first.

- `core.ProtectedSurfaceFloorChecks`: the build handoff floor fails the handoff for every changed path on the control-plane manifest. It judges the cycle-base diff plus untracked files, the set ship judges once HEAD is back at the base. The correction ladder then has the builder restore the file in-phase; the message gives the exact `git checkout <base> -- <path>`. The membership predicate (`guards.IsProtectedSurface`) is injected at the root. The one composition, `productionBuildFloorChecks`, is a named function in the protected `cmd_cycle_config.go` that runs the protected check first; the cycle reviewer and `evolve selfcheck build` both run it.
- Rename detection is off in both the floor and ship's check. A protected file moved to an unprotected name used to pass both, because `--name-only` printed only the new path.
- `router` recovery: `CONTROL_PLANE_VIOLATION` routes to **build** (`recover:control-plane-rebuild`), not to a futile re-audit. End-to-end pin: ship → build → audit → ship.
- ADR-0064 amendment; record `docs/incidents/2026-09-26-lane-edited-the-control-plane-through-a-shell-tool.md`. Remaining gaps are filed as F38.

## Fixed — no route override or line locator lets a declared protected file reach a lane, and the wave plan drops console-routed ids before it widens (F34, F35, 2026-09-26)

Wave 7 dispatched one lane of two. The plan's prior decision still carried an id the operator had since routed to the console. It filled the fleet width, so the widen had nothing to refill, and the plan-time gate refused it only after the lanes were cut. A census of the live queue then found two dispatchable items that declared protected FILES under an operator `route:"lane"`. Triage's breaker and the ship tripwire both judge membership of each file and ignore the route, so each item was a guaranteed triage FAIL.

- `inboxbatch.ConsoleRouted`: a declared protected FILE binds. `route:"lane"` still relaxes the heuristic derivations (the pipeline-* kind, a declared directory that merely holds protected files, a file the text names), never a manifest file. The reason says why. Run `evolve inbox`: any "route:lane cannot relax" line is an item to re-author (declare the files it changes). Residual, filed as F35c: a declared directory that is itself inside a protected directory fragment (`go/internal/bridge/`) still reads as scope and stays overridable.
- `files[]` tokens drop a trailing source locator (`x.go:178`, `x.go:178:5`, `:10-20`, `#L10-L20`) before judgment. The breaker's substring match already saw through it. This also means a `files[]` of only located paths now declares a surface, so, as F29 set out, protected files named in its prose no longer route it.
- `loopwave.Engine.pruneRouted` drops the prior decision's console-routed ids (one WARN each, through the gate's own resolver) before `WidenNarrowDecision`, so their slots refill from the backlog. The plan-time gate stays the backstop.
- ADR-0074 and research F34/F35 carry the evidence. The two live items were re-routed `console-manual` in the same session.

## Fixed — the wave seed refuses at least everything triage's breaker would, so lanes stop drawing doomed work (F29, 2026-09-26)

Triage was the pipeline's dominant failure. 18 of ~60 recent cycles sealed FAIL with `TRIAGE_PROTECTED_SURFACE` after a full scout and triage spend, and a lane's fleet scope is one item, so a refusal had nothing to fall back on. All 18 traced to five items whose protected surface was visible in their own record: the kind (routed since F25), a declared *directory* holding protected files, or a protected file named in the item's own text when it declared no surface.

- The manifest now has two projections. `guards.IsProtectedSurface` stays membership, and additionally treats a path naming a protected directory without its trailing slash as a member. `guards.IsProtectedScope` adds "a directory contains protected surface" and is what the routing roots inject (seed, widen, plan-time resolver, claim floor, prompt partition, `evolve inbox`). The ship tripwire, the role guard, the fleet preflight and triage's breaker keep membership. Membership implies scope, so the seed is never looser than the breaker.
- `inboxbatch.Item.DeclaredSurface` is the one home of "this item declares its surface"; a placeholder or bare file name declares nothing. With no declared surface, the files the item's author-written text names are judged. They are derived at decode, so the wave seed, the claim floor and `LoadFile` all see them; directory mentions are context.
- Among equal weights, the seed prefers verified-admissible work (`triagecap.rankForDispatch`). The operator's weight stays the priority.
- On the live queue, lane-dispatchable goes from 65 to 47. `TestConsoleRouted_ReplaysTheEighteenRefusals` pins the historical shapes. ADR-0074 carries the amendment, and follow-ups F30 and F32 are filed as console-owned inbox items.

## Changed — the fatal-pane fast-fail has its own dial and acts by default (F27, 2026-09-26)

Wave 6 lane 1687's claude-tmux triage pane went dead (`: command not found`) and idled 900 s: the ADR-0044 C2 detector classified it (`dead_shell`, recorded `would_fast_fail` twice) but could only observe, because its one rollout dial, `recovery.phase_recovery`, also arms the live channel, the ask-broker and the failure adviser and so stays at shadow. The fast-fail now rides its OWN policy dial, `recovery.fatal_pane`, default **enforce**. It was split out exactly as `recovery.spine_floor` was (R8.5), so `phase_recovery` stays shadow and nothing else is armed. Flip evidence: every shadow match on record (cycles 1595 build / codex-tmux and 1687 triage / claude-tmux) was a dead pane that then idled 1200 s / 900 s. `"shadow"` in `.evolve/policy.json` is the no-recompile escape hatch.

The dial reaches the bridge through one pinned path: `wireBridgeStages` (the root's one forwarding of the bridge dials, `cmd/evolve/cmd_cycle_config.go`) → `Adapter.SetFatalPaneStage` → `productionEngineDeps` → `Deps.FatalPaneStage` → `fatalPaneStageOf`. Both recovery dials now live in `productionEngineDeps`, the builder both `Launch` branches share. That closes a branch-local `RecoveryStage` assignment the engine-factory path skipped. `NewDefault` and the `evolve subagent run` root (which set neither dial before) seed both from policy through one accessor, `policy.Policy.BridgeRecoveryStages`, using the Loader's own parser. ADR-0044 carries the amendment; `docs/operations/runtime-reference.md` has the dial row.

## Fixed — a pipeline-repair inbox item routes to the console at plan time, before a lane claims it (2026-09-15)

Wave 6, lane 1688 claimed a `pipeline-repair` item that carried no files list; scout and triage ran before the triage breaker refused its card for naming a protected surface, and the cycle sealed FAIL. The ADR-0074 routing classifier now reads the item's kind: a `pipeline-*` kind is console-owned work (the ADR-0072 halt autofiles the same kind for the operator), so the wave seed skips it, the claim refuses it, and `evolve inbox` lists it under console with its reason — under the same override clamp as the files-derived rule. Fifteen queued items move out of lane reach without burning a cycle each (research F25).

## Fixed — the verdict engine verifies through the contract gate's own Reviewer, so a salvaged verdict is classified as the gate approves it (2026-09-15)

Cycle 1685 sealed FAIL with red_count=0: the auditor's verdict was fenced JSON without the sentinel wrapper, the gate salvaged and approved the repaired report, but the verdict engine had verified with the plain verifier and classified the unrepaired bytes ("no parseable verdict"), and the audit-fail envelope declined a repair for want of a class. There is now ONE verifier: `deliverable.Reviewer.VerifyForClassification` salvages, persists and reports a sole recoverable bad_verdict before classification and returns the repaired OK result; the composition root hands every BaseRunner an accessor to the gate's Reviewer (`runner.Options.ContractVerifier`, through each phase Config); the engine stays a leaf with its own `verdict.Identity`. Research F22.

## Fixed — cycle 1679's added-test predicate is green on a merged tree (2026-09-15)

`acs/cycle1679` `TestC1679_007` fataled on an empty added-test seed, which is what every clean checkout of main has once the lane's commit is merged — a durable predicate red on main in every whole-module floor. It now verifies its recorded added set (the two files the lane added) the same way when the live seed is empty. Research F21.

## Fixed — the loop's interrupt wins over its pre-wave probes (2026-09-15)

At the wave-4 boundary the loop ignored SIGINT ×2 and SIGTERM for over a minute inside the pre-wave usage probe and needed SIGKILL, then dispatched a wave that was cancelled at spawn: the usage probe and the CLI-health canary ran on `context.Background()`, the prober waited on a bridge pane that never answers, and the wave coordinator checked the interrupt only before the probes. Both probes now take the runner's context through ONE pre-wave protocol (`runPreWaveProbes`, shared by the loop and the campaign, whose `BeforeWave` hook now takes the live context and aborts the wave on its error); `usageprobe.Prober.Run` returns promptly on cancel (abandoning probes still running with one line naming the count, never benching after the interrupt); the CLI-health canary touches no bench once the context is done (a cancelled probe is not evidence); the bridge's settle loop honors the context; `prepareIteration` re-checks the interrupt after the probes through the one interrupt disposition (`signalStop`). Research F20.

## Fixed — an audit failure class outside the vocabulary is refused at the gate, the repair decision is a coded signal, and a retro-routed retry carries the audit's standing findings (2026-09-15)

Cycle 1684's audit declared `failure.class: superseded-predicate-contradiction`; the gate accepted it, the retry envelope silently declined the direct repair on "unrecognised class", a full retrospective ran, and the retry it adjudicated rebuilt with none of the audit's findings. The deliverables gate now validates the audit's class against the failurelog vocabulary (`failure_class_unknown`, correction names the class and the vocabulary) and the audit contract block renders that vocabulary (`failurelog.VocabularyList`); `decideAfterAuditFail` emits `ORCHESTRATOR_AUDIT_REPAIR_DECLINED` / `ORCHESTRATOR_AUDIT_REPAIR_GRANTED`; a tdd/build re-entry after a retrospective that followed an audit-fail decline (`CycleState.AuditDeclineReason`, persisted) seeds `## Standing Audit Findings` like a ship-error recovery, with one shared intro sentence (`core.StandingFindingsIntro`); `core.policyCategoryFor` is the one join between the declared-class vocabulary and the failure-policy categories, so the envelope's decline reasons say which vocabulary failed, and the prompt exemplar draws its class from the vocabulary. Research F19.

## Fixed — a shipped inbox item is never re-planned, a sealed lane never blocks the boundary refresh, a template slot never passes the build floor, and a recovery rebuild carries the standing audit findings (2026-09-15)

Wave 4 opened 0/2: lane 1682 was pinned to an item cycle 1679 had shipped an hour earlier (the dispatch-state resolver had no `consumed/` state and read `processed/` flat while the promoter nests it by cycle, so every shipped item resolved `unknown` and both prunes and the launch probe kept it), and both wave boundaries skipped the binary refresh on a sealed lane's still-fresh lease. From 1679's audit rounds: an unsubstituted `FULLSUITE_PLACEHOLDER` passed the handoff floor, four repo tests were red under the audit sandbox for writing into the live `.evolve/profiles` or walking `docs/private`, and the rebuild after a WARN audit carried none of that audit's findings. Fixes: `inboxmover.StateConsumed` and a flat-and-`cycle-<N>` scan of every retirement dir through one `inboxbatch.CycleDirs`; `loopchain.FleetLaneActive` decides by owner liveness (`runlease.OwnerLive` with the one `runlease.PIDAlive` probe, the swarm reaper delegating); `core.PlaceholderTokenFailures` in the default build floor; the decoy tests plant on temp git mirrors of the tracked profiles and the two walkers skip a denied directory visibly; `CtxKeyStandingAuditFindings` seeds a `## Standing Audit Findings` section into a tdd/build re-entry that follows a ship-error recovery (`CycleState.ShipRecoveryCode`, persisted so live loop and crash-resume agree). Research F14–F18 in `docs/research/verification-wave-findings-2026-09-14.md`; incident `docs/incidents/2026-09-15-shipped-item-re-pinned-and-sealed-lease-blocks-refresh.md`.

## Fixed — a red added test is found at the build floor, a graderless eval is refused at the scout, and a rejected push still journals its commit (2026-09-14)

Wave 3, lane 1679: the lane's own `//go:build acs` predicate was red on the eval the scout had materialized without a `[code]` grader; the build floor could not see the tag-gated package (default context only), the audit passed, the ship's added-test backstop refused the tree, and the repair rounds went to a builder whose sandbox forbids `.evolve/evals` — an hour to reach the fix the tdd phase could apply. Now: `internal/addedtests` is the one derivation of "which test packages did this tree add, under which tags" (moved from the ship's backstop) and the floor runs every added tag-gated package under its own tags before handoff, reporting a red like any floor failure; the scout gate enforces the `[code]`-grader rule its remediation has always stated, naming the slug and the exact path; and the ship journals every minted commit whether or not its push succeeds, so `evolve ship --push-only` can complete a rejected-push strand (lane 1678 was refused "lack ship provenance" because the journal was written only after a successful push). Record: `docs/incidents/2026-09-14-ship-gate-found-what-the-audit-could-not.md`.

## Changed — every LLM launch walks the fallback chain, and every chain ends with every available CLI (ADR-0104, 2026-09-14)

Wave 2's retrospectives timed out on codex's rejected deep model and both cycles sealed with no failure-learning disposition — the retro called the bridge itself and never entered the chain walk the runner has had since WS-G1/WS-876, and four sibling launchers (failure advisor, phase judge, retry adjudicator, swarm) did the same. `internal/bridgechain.Walking` is a Decorator over `core.Bridge` wrapped once at the composition root: every `Launch` resolves the agent's plan (profile primary + `cli_fallback` + tier chain + cli-health bench + the universal tail) and walks it with `DispatchTiered` — CLI fallback on 80/81/124/127, tier step-down on 85, bench on a classified wall; the runner and the advisor mark their own attempts (`BridgeRequest.ChainAttempt`) and pass through, and the handle forwards the adapter's Signal Center. The universal tail is now appended unconditionally (operator policy: try every available CLI before giving up), the agy ban moves to `workflow.universal_fallback_exclude` (default `["agy"]`), and the eleven claude-primary profiles that had no chain get one — in-family only (`claude-p`) for the five Claude-family-floor agents, whose anti-gaming floor stands. The runner's cli-health projections live in `bridgechain` (one implementation for both walks). Records: `docs/architecture/adr/0104-fallback-is-a-property-of-the-bridge-handle.md`, `docs/incidents/2026-09-14-retro-timeout-sealed-the-cycle.md`.

## Fixed — the audit's CI-parity apicover step runs its coverage `go test` under the scrubbed CI env (2026-09-14)

Every wave-2 audit raised `AUDIT_CIPARITY_GATE_STEP_FAILED` on `cover_run`: the scoped coverage run of the lane's packages exited 1 in the lane worktree because `coverageProfile` spawned `go test` through the gate's raw runner, inheriting the lane's `EVOLVE_CYCLE_STATE_FILE` and `EVOLVE_FLEET` — the same leak the ship gate had (#615), on the audit's spawn site; the tier step beside it already ran under `scrubbedRun`. The step is fail-open, so the cost was a WARN and a ~7-minute core coverage run on every audit since cycle 1673, never a blocked ship. The coverage step's three `go` invocations now run under the same allowlist as the tier, red-first. Record: second-site section of `docs/incidents/2026-09-14-ship-gate-inherits-the-lane-ipc-env.md`.

## Fixed — the ship repo-contract gate hands `go test` CI's environment, not the lane's (2026-09-14)

A fleet lane exports its runtime state process-wide — `EVOLVE_FLEET=1` from the supervisor, `EVOLVE_CYCLE_STATE_FILE=<its own run dir>` from `cyclerun.go` — and the ship gate's pack runner spawned `go test` without setting `cmd.Env`, so the child inherited both. Latent until #612: the scanner pack's four packages never read the environment, but the importer backstop runs `cmd/evolve`, `core`, `guards` and `ship` on the first lane that changes what they import, and there twenty-odd env-sensitive tests (cycle-reset lease fencing, guards' "outside a cycle", seal role, ship's fleet-off goldens) went RED in the lane worktree while passing in CI and in a clean shell — lane 1677 aborted its ship and spent a recovery audit on a green change. `ipcenv.Scrub` is the one scrubber: the package that owns the IPC keys drops its whole namespace from an environment (`TestScrub_CoversEveryIPCKey` pins that every exported key lives in it); the ship runner shared by all three gate layers and the four core spawn sites (phase-bindings self-check, build floor, task contract) call it, and core's private `sanitizeEnv` is gone. `ciparitygate`'s allowlist stays as the integration tier's stricter contract. Record: `docs/incidents/2026-09-14-ship-gate-inherits-the-lane-ipc-env.md`.

## Fixed — the routing proposal reads the artifact its prompt asks for, not the REPL scrollback (2026-09-14)

The advisor's post-build proposal decision still completed on REPL-idle stdout (ADR-0027) while its prompt carried the same deliverable contract as the plans — "write routing-proposal.json". The model wrote the file; the kernel parsed the scrollback, found only the prompt's echoed JSON example, and raised `ADVISOR_RESPONSE_UNPARSEABLE` on every proposal (9× in the verification wave, research finding F4), degrading each cycle to static routing after a wasted advisor launch. `decisionProposal` now uses the uniform artifact contract like the plans; a model that prints instead of writing the file now surfaces as `BRIDGE_EXIT_ARTIFACT_TIMEOUT` rather than a plausible wrong proposal. Record: `docs/incidents/2026-09-14-router-proposal-read-the-scrollback.md`.

## Added — the dashboard shows each cycle's phase plan: required, passed, ongoing, remaining (2026-09-14)

Every cycle on the board now carries a `plan`: the registry's mandatory set read through its owner (`config.mandatory_phases` via `internal/config` — scout, triage, build, audit, ship — plus the `conditional_mandatory` phases that ran), every phase the cycle ran with its last verdict, the contract-gate verification mark from the cycle's Signal Center stream (`signalcenter.StreamFileName`, now the one spelling the root and the reader share), wall clock and rounds; the counts (`passed 3/5 required · +1 optional`), the ongoing phase and its start, what remains, and how the advisor's proposal fared (proposed and not run, proposed to skip, proposed to skip but run — the floor overrode it). A running cycle reads what ran from the stream's `phase.outcome` events (`phase-timing.json` is flushed at closeout); a sealed cycle reads the timing file; the stream is read once per change on disk. Every source the reader cannot trust — a malformed registry, a torn proposal, a stream that stops early — is reported in the snapshot's warnings, never substituted. The stepper renders it (filled / pulsing / hollow / dashed / faded, a ring for a verified gate), the cycles table captions it, and the detail view tables it. Operator ask: "how many phases are required, how many passed, which is ongoing" — on the live board.

## Fixed — Claude's session wall is recognised again: tmux indents it with U+00A0 (2026-09-14)

Between 19:03 and 19:50 the account's rolling session limit was reached and every Claude dispatch parked on `⎿  You've hit your session limit · resets 7:50pm`. The bridge's guarded fast-fail (classifier + persistence gate + live probe → exit 85) never armed because tmux renders the `⎿` indentation with a NO-BREAK SPACE and the session-wall branch of `exhausted_regex` admitted only `[ \t]`; both wave-2 recovery audits ran the full 2400 s artifact window per dispatch instead, with `POSSIBLE EXHAUSTION-REGEX DRIFT` in the bridge stderr. Every `[ \t]` in that regex is now `[\t\p{Zs}]`; the red test feeds the classifier the live bytes with the U+00A0 spelled out. Record: `docs/incidents/2026-09-14-claude-session-wall-nbsp-drift.md`.

## Fixed — a codex model the account rejects fails over in seconds, not after the 25-minute artifact window (2026-09-14)

Lanes 1676 and 1677 dispatched build on `codex-tmux@deep`; the ChatGPT account rejected the pinned `gpt-5.6-sol` with a 400 `invalid_request_error`, the pane idled, and no auto-responder rule named the shape — so both lanes sat through the full artifact window (`stop_kind: artifact_timeout`, 1500 s of silence) before the runner fell back to Claude. The codex manifest gains a `model_unsupported` escalate rule (the 4xx JSON or `The '<model>' model is not supported when using Codex`, last 30 pane lines, idle-gated like every escalate rule): exit 85, the path the quota wall already takes, so the family fallback runs at once; the pattern is not `rate_limit`, so `clihealth` does not bench codex. The deep/top pin itself is the operator's cost call and is unchanged. Record: `docs/incidents/2026-09-14-codex-deep-tier-model-rejected.md`.

## Fixed — the ship gate tests the lane worktree, seeds from the working tree, and runs the importers of what a lane changed (2026-09-14)

A lane renamed a stop in `internal/core`; its own tests were green, the ship-time repo-contract gate was green, and `internal/deliverable`'s untouched e2e test redded main from 4db205a8 until the console hotfix #590. Three defects, two older than the incident: the gate ran in `req.ProjectRoot` (main's pre-landing checkout) while a cycle ship's changes live in the lane worktree, so every layer tested a tree without the lane's changes; the added-test backstop read the index, which the ship populates only after the gate, so it never fired on the cycle path; and nothing looked at importers. Now one landing-tree decision (`landingTree`, shared by `atomicShip` and `repoContractGateRoot`) runs the gate in the tree the ship will land against its base, failing closed on an unresolvable typed worktree; `changedpkgs.ChangedFilesChecked` is the one working-tree seed (`FromGitChecked` is its projection); `changedpkgs.ImporterClosureChecked` walks build deps and the direct imports of each package's tests and projects what `go test` can run under the default context (`ImporterClosure`, used by `regressiontia`, keeps its contract and now sees test-only importers); the importer backstop runs that closure minus what earlier layers ran through the classified runner, bounded by a timeout and a retry-size cap; discovery failures are the re-dispatchable `REPO_CONTRACT_INFRA`, never a silent green. Record: `docs/incidents/2026-09-14-lane-ship-gate-package-scoped-tests.md`.

## Fixed — a `--simulate` walk never mutates the operator's repository (2026-09-14)

`evolve campaign run --simulate` and `evolve cycle --simulate` used the production closeout: every walk committed a `dossier: cycle-N closeout`, snapshot-committed its "worktree" on FAIL, provisioned real `cycle-*` worktrees and branches, and launched real CLIs for the per-wave health probes — so the acs/cycle8 gate test, run from the repo root on every whole-module floor, dirtied the dev branch and left `.evolve/ledger.jsonl`, `runs/` and `worktrees/` behind (and passed only because that litter supplied the `build-report.md` the floor demanded). The simulate root now reads the project root in place, writes its dossier without committing (`core.WithDossierCommit(false)`), and every host mutator of a worktree asks ONE predicate — `inPlaceWorktree` — inside itself: the salvage snapshot, the per-phase gofmt normalize and leak recovery, the index-staging content SHA and the fleet rebase all stand down for an in-place root on every path (dispatch, resume, composition) (symlink aliases and relative spellings included; a resume checkpoint can name the root under the production provisioner too). The simulate runner writes each phase's contracted report stub through the grammar owners (`phasecontract.RenderVerdictSentinel`, `explanationdocs.RenderNotApplicableDeclaration` — the e2e fake CLI and the in-process fake runner now render through the same function), and `--simulate` installs no live-probe hook; the gate test runs in a detached scratch worktree and pins HEAD, porcelain status (untracked included), `cycle-*` branches and the worktree list byte-identical before and after. Record: `docs/incidents/2026-09-14-simulate-runs-against-the-checkout.md`.

---

## Fixed — the tmux driver no longer declares a large codex prompt "wedged" 2.5 s after pasting it (2026-09-14)

Every codex-tmux phase of the first post-decomposition wave needed two or three Enter re-sends to submit its prompt, and the two largest (scout 17.7 KB, build 35 KB) hit the re-send cap: the TUI was still ingesting the paste when the fixed 1 s settle and three 500 ms re-sends had all fired, so the dispatch was discarded as `submit_wedged`. The delivery tail now has ONE home (`paste_settle.go`, shared by the prompt paste and `injectText`): it settles by paste size (1 s + 1.5 s/10 KB, cap 6 s — never the 2 s artifact-wait interval), waits for the pane to stop changing before the first Enter (≥ 2 KB pastes, bounded to 8 polls; an unreadable prompt is treated as a large one, loudly), and records the settle and the stability outcome in the submit-verify ledger on the success path; `verifySubmitted` backs off between re-sends (500 ms → 1.5 s → 2.5 s). Vocabulary, stderr lines, the re-send cap and the wedge short-circuit are unchanged. Record: `docs/incidents/2026-09-14-codex-prompt-submit-wedge.md`.

---

## Fixed — a triage refusal is a task-level failure; a protected-surface refusal routes the item to console on the first hit (2026-09-14)

One inbox item (`verdict-sentinel-as-tool-call`) drew nine lanes in a row (cycles 1650–1675) — scout, then a triage FAIL for "top_n card names protected surface", then a closeout that put it straight back for the next wave. Root cause: the triage gate's own refusal had no failure class, `cycleclassify` fell through to system-level, and the ADR-0072 S5 drain never bumped `failure_count` — the quarantine ceiling was unreachable and the closeout's re-claim of the already-claimed id even raised a false `INBOX_CLAIM_NOT_FOUND` on every cycle.

- `cyclestate.Diagnostic` gains `code` and `subject` (both `omitempty`): the phase's own gate stamps a machine-readable reason (`TRIAGE_PROTECTED_SURFACE` / `TRIAGE_TOPN_EMPTY` / `TRIAGE_COMMITMENT_INVALID`) and the card it names; `cyclestate.ErrorCodes` is the one projection.
- The C1 chokepoint's `ORCHESTRATOR_PHASE_VERDICT_FAIL` carries `diagnostic_codes=…` — the console line says WHY, structurally.
- `cycleclassify` reads the C1 record (`phase-timing.json`) before any prose pass: a coded FAIL on the last outcome is the new task-level class `phase-refusal` (marker = the code, detail, subject).
- Whose fault a refusal is lives in ONE table beside the vocabulary (`cyclestate.RefusalDisposition`): protected-surface = the item's, and routed; empty top_n = the item's (bump toward the ceiling); commitment-invalid = the pipeline's (an I/O fault charges nobody).
- The FAIL closeout routes a `TRIAGE_PROTECTED_SURFACE` subject — only one in this cycle's committed set — to `route: console-manual` in place (`inboxmover.RouteConsole` → `INBOX_ITEM_ROUTED_CONSOLE` WARN; `INBOX_ROUTE_NOT_FOUND` when it cannot or when another cycle holds the item) before the drain, then the drain runs with `Routed` set (no bump, no park for that cycle). `ClaimLaneScope` no longer re-claims an id the lane already holds in `processing/`.

Record: `docs/incidents/2026-09-14-triage-refusal-poison-loop.md`. Three inbox-mover goldens were re-captured (the two false WARN lines per already-claimed id are gone).

---

## Changed — the `evolve subagent run` execution path is its own package with eleven Signal Center codes (ADR-0103 unit 16, 2026-09-14)

`internal/subagent/run.go`'s 236-line `Run` moved into `internal/subagent/subagentrun` (a `Dispatcher` over
ten explicit ports and six stdlib-backed options; 100 % lines, 61/61 exports, every function < 50 lines).
Byte-identical on the prompt, the adapter env, the Warns channel, every error text and the
`agent_subprocess` ledger line — goldens captured before the move and replayed. The eighteen failure exits
that collapsed into one `[subagent-run] FAIL:` line are now eleven `BRIDGE_SUBAGENT_*` WARN codes
(`origin=Dispatcher.Dispatch`, `fields.step` names the step, `rung` / `op` / `reason_class` the detail), the
four silently swallowed causes (the profile read error, the LLM router error, the git-state error, the
artifact hash error) and the dropped integrity rung are on the stream, and `evolve subagent run` is the
second Signal Center root: its events land in `<runs/cycle-N>/signals.ndjson` and on stderr at WARN, and the
bridge engine's own producers reach the same Center on this path instead of nil. Console addition only: the
existing stderr lines stay verbatim.

---

## Changed — config resolution is a unit with a Loader, one module tag and six codes (ADR-0103 unit 08, 2026-09-14)

`internal/config` — the single reader of the phase registry and the routing env — is decomposed in place around a `Loader` with an injected file reader and Signal Center (`config.New(config.WithSignals(…))` at the cycle/loop composition root; the package-level `config.Load` stays as the Center-less facade the per-phase callers keep). Every resolution diagnostic is now a `config.warning` WARN under module `config` with `origin` (`Loader.Load` / `Loader.ApplyPolicyStages`), `fields.step` (`registry` | `env` | `spine` | `inert` | `policy`), `key`, `source`, `path` — rendered by the root's console sink and durable in `<evolveDir>/signals.ndjson` (cycle-less). Two silent kill-paths are closed, so operators may see new WARN lines on the first cycle after landing:

- **`CONFIG_REGISTRY_MALFORMED` / `CONFIG_REGISTRY_UNREADABLE`** — a `docs/architecture/phase-registry.json` that exists but does not parse (a trailing comma) or cannot be read used to run every cycle SILENTLY on the compiled baseline that omits triage, the registry order, the enabled/routing blocks, goal recipes and deliverable kinds. The degrade is unchanged; it is no longer silent. (`evolve solution` prints the same two codes as `registry warning [registry-malformed]: …`.) Until the registry has one reader, a malformed file prints two console lines — the sibling parser's `[phases] WARN builtin registry load failed` and the Center's.
- **`CONFIG_UNKNOWN_VALUE` from `.evolve/policy.json`** — a typo'd gate/recovery/router stage word (`gates.eval_gate: "enfroce"`, `recovery.spine_floor: "shadwo"`, `router.router_replan: "advisry"`) used to resolve to `off` with no trace; it still resolves to `off` and now warns with the policy key (`key=gates.eval_gate source=policy step=policy`). Check a live runtime's policy.json before the merge train if a silent typo would surprise anyone. `EVOLVE_CONDITIONAL_MANDATORY` without a `:` (ignored before) warns the same way.
- The hand-written `[config] WARN <code>: <msg>` stderr line at the root is replaced by the module-tagged line `[config] config.warning WARN CONFIG_… origin=Loader.Load — <msg> …`; the ten resolution warnings, `weak-spine`, `spine-order` and `inert-phase-enable` keep their exact sentences as the event reason. Warning ORDER for several deliverable-kind holes or bad conditional rules is now sorted by key (it was map-random).

Record: `docs/architecture/decomposition/08-config.md`. Gates: `internal/config` at 100 % lines and 100 % API; the leaf and the seam (`cmd/evolve/cmd_cycle_config.go`) join the protected surface.

---

## Added — the declared effect `inbox-claim` is verified at the triage boundary (ADR-0100 slice 2, 2026-09-13)

Triage's persona claims every inbox item it ingests (`evolve inbox-mover claim`) so no sibling lane can select the same item — and nothing judged whether the claim happened. Batch cycle 1631 "claimed" the worktree's tracked copy of the inbox while the plane's item stayed dispatchable; 1630's claim was refused by the sandbox; in both the report and decision looked complete and the spine ran on an unclaimed commitment (1623's shape).

- **Registry declares it** — `phases[].effects: ["inbox-claim"]` on triage; `internal/deliverable/effects.go` binds each declared name to one deterministic check by lookup, and a projection test pins the registry and the table equal in both directions (a declared name with no check is `unbound_effect` — a registry defect, reported rather than passed).
- **The same gate verifies it** — after the primary and the agent-owed secondaries, `checkInboxClaim` owes a claim for every committed id that is an inbox item: satisfied when `inboxmover.Locate` finds it under THIS cycle's `processing/cycle-N/`; `missing_effect` when it is still pending at the root or held by another cycle, with the item and the exact command in the message (that message is the correction directive). Scout- and carryover-originated `top_n` ids have no inbox file and owe nothing; an empty commitment owes nothing.
- **Every verifier names the cycle** — `phasecontract.Roots.Cycle`, carried by the host gate (`ReviewInput.Cycle`), the runner (`verifyRootsFor`, now the one roots translation for its two verify sites) and `evolve phase verify` (from the persisted cycle state, only for contracts that declare effects); a caller that omits it fails OPEN with an error, never decides blind.
- **One owner for the claim layout** — `inboxbatch/layout.go` (`ProcessingDir` / `ProcessingCycleDir` / `ProcessingCycleDirs` / `ParseProcessingCycle`) is where the writer `inboxmover.Claim`, the one reader walk `inboxmover.Locate` (Promote and the continuation scope readers now delegate to it) and core's dispatch-time claim scan all derive `processing/cycle-N/` from; `TestLocate_FindsWhatClaimWrote` proves the gate reads where the claim really lands. The self-check (`evolve phase verify`) reads the persisted cycle state once for both context values, defaults `--evolve-dir` from the project root the resolver used, and states each consumer's own consequence when the state is missing.

Proof: a real cycle whose triage commits to a pending item without claiming it is re-dispatched with the effect named and ends `FAILED_EXPLAINED` naming it; a triage that claims on the correction round ships; an empty commitment still ends triage no-work.

---

## Added — every agent-owed declared output is verified at the phase boundary (ADR-0100, 2026-09-12)

The registry has always declared a phase's full output list; the contract gate judged only `outputs.files[0]`, so `carryover-todos.json` — and `triage-decision.json`, which was never declared at all (two recent PASS triages, 1592 and 1593, never wrote it) — were read downstream and judged by nobody. The registry now partitions secondaries into `outputs.agent_owed` and `outputs.harness_produced`; the partition is enforced by the loader for the registry and for overlays alike (`phasespec.ValidateOutputsPartition`); `phasecontract.Contract` projects the declaration onto built-ins through the one `RegistryKey` rule; and `internal/deliverable` verifies each agent-owed file (exists, non-empty, parses if JSON/NDJSON) with stable codes whose message names the file — the same correction ladder re-dispatches the phase and a persistent gap ends the cycle `FAILED_EXPLAINED` naming it. The registry's `handoff-build.json` / `handoff-scout.json` declarations are removed: 0 of 15 recent builds and 0 of 29 recent scouts produced them (extinct since ~cycle 215 per `changedpkgs.go`) — gating them would have re-dispatched every build and scout. Two review bypasses closed: the remediation re-run's response now meets the reviewer, and a resumed version-0 checkpoint is reviewed like a fresh cycle. Wiring proof: `DeclaredDeliverablesGateWired()` at the composition root. Evidence: batch cycles 1623/1630/1631. Record: `docs/architecture/adr/0100-declared-deliverables-gate.md`.

---

## Fixed — a resumed cycle disposes of its worktree, by the rule the fresh path applies (2026-09-12)

`RunCycleFromPhase` registered three of the fresh path's four exit actions and no worktree teardown, so every resumed cycle leaked its worktree whatever its verdict (26 stale `cycle-*` checkouts on the live runtime). `finalizeCycle` computed the preserve decision on resume too; nothing read it. The prune/preserve rule is now ONE function (`teardownCycleWorktree`) both entrypoints defer, reading the closeout's live flags. Found while fixing it: a resume checkpoint can name the **project root** as the worktree (ship already routes that shape to `shipDirect`), and `gitWorktree.Cleanup` runs `os.RemoveAll` unconditionally — the provisioner now refuses the project root, loudly, beside its existing `cycle-` branch gate; an end-to-end test through the real provisioner pins that the repository survives. Record: `docs/operations/fix-2026-09-12-resume-worktree-teardown.md`.

---

## Fixed — the cycle dossier is a projection of evidence, not a synthesized placeholder (2026-09-11)

Cycle 1623 ran twelve phase dispatches and shipped 922 lines to `origin/main`. Its permanent record contained ONE synthetic phase — `{"name":"cycle-recorded","verdict":"PASS"}` with zero tokens — and no commit. The record was unreadable precisely when it mattered, and it misled a reader into concluding the cycle had shipped nothing.

**Root cause (deterministic, not a race).** `phase-timing.json` is written by a DEFERRED call registered in `RunCycle`, so it lands only after that function returns. `writeCycleDossier` runs INSIDE it, at closeout. The dossier therefore read its own evidence before it existed, every time, on the normal path. Only a resumed cycle — which writes the log mid-run — recorded real phases, so the healthier the cycle, the emptier its record. A synthesized `cycle-recorded` phase carrying the CYCLE's verdict hid the gap: a twelve-phase cycle was indistinguishable from a one-phase PASS.

- **One composition, one flush** — `composePhaseTimings` is the single rule (on-disk entries first, this invocation's appended; duplicates are real dispatches, never deduped) and `cycleRun.flushPhaseTimings` applies it EXACTLY once, returning the composed set. Both dossier producers project that same set. This matters because the two paths disagree on ordering: the abort path flushes before its dossier is built, the normal path after — so a naive "live replaces file" dropped the pre-resume prefix (resume starts its timing slice empty and holds only the resumed segment) while a naive append would have double-counted.
- **Absence is loud** — with no evidence at all, the record carries one `evidence-unavailable` phase at WARN naming the degradation. A dossier may now say "I do not know"; it may never invent a phase verdict no phase produced. The cycle's own `final_verdict` is untouched.
- **Delivery is recorded** — `Dossier.CommitSHA`/`TreeSHA` existed in the schema but no producer ever set them, which also left `dashboard.shipped()` structurally dead for WARN cycles. They now project from the ship binding. The sidecar's shape moved from an inline map at the write site to the shared `dossier.ShipBinding` type that `phases/ship` marshals, so a field rename is a compile error rather than a silently empty record — this change first read `commit` instead of `commit_sha` and passed its own tests, because the tests re-typed the same wrong assumption.
- **Commitment is recorded** — new `Dossier.Tasks` (a pointer, so omitted/`[]`/ids are three distinct states) and a `**Committed:**` line in the rendered markdown, which is the half people actually read. It projects from the new leaf package `internal/committedset`, the ONE answer to "what did this cycle commit to": the lane pin, else triage's `top_n`, minus deferrals. That question previously had four disagreeing readers; on runtime cycle-1621 the lane pin named two members where `top_n` named one, and 17 of the last 20 runtime cycles carried a pin. `core.ContractTaskIDs` now delegates to the same projection.

Verified against cycle 1623's real artifacts: the rebuilt record reports 12 phases, commit `dc00395a`, and `tasks: []`. Note for anyone reading the ship-rate series: historical dossiers have no `commit_sha`, so WARN cycles before this change score as not-shipped and the trend has a discontinuity here.

---

## Changed — codex deep/top back to gpt-5.6-sol; claude deep/top stay opus (operator cost directive, 2026-09-10)

The 2026-09-09 cutover of codex's deep/top tiers to gpt-6-astra (#538) is withdrawn: astra burned the subscription far faster than the work justified, and the operator keeps **gpt-5.6-sol** for codex and **opus** for claude at the high tiers. The single-table plumbing from #538 is unchanged — this is ONE value edit in `bridge/manifests/codex-tmux.json` (deep/top, `chatgpt_default_model`) plus removing astra from `chatgpt_safe_models`, so a stray pin to it is clamped to the family default instead of dispatched (`TestCodexFamilyManifest_AstraIsNotSelectable`). Reasoning effort stays `high`. Claude's embedded table (`claude-tmux.json`: deep/top = opus) needed no change. The runtime plane's temporary `.evolve/bridge-manifests/codex-tmux.json` override that held sol in the meantime is retired with this landing — the embedded manifest is again the one source.

---

## Fixed — multi-slug lane scope reconciliation, salvaged from cycle 1620 (2026-09-10)

Cycle 1620's candidate for inbox item `multi-slug-lane-scope-reconciliation` (0.87; the cycle-1480/1483 class where TDD minted predicates for both members of a two-slug lane while the Builder contract bound only the first, so the second slug was silently undelivered) was interrupted by the operator pause before it could land. It is salvaged through the sanctioned path with the audit's own findings repaired rather than re-run: TDD's `## Handoff to Builder` declaration (`slugs[]`) must equal triage's committed multi-member set before Build starts (`topngate.tddScopeGate` — order-independent, N>2, example fences ignored, abort at the TDD→Build boundary); ONE projection, `core.LaneScopeIDs`, feeds the TDD/Build task contracts and the outcome accounting (`cycleoutcome` delegates to it — no second parser). Audit M1 fixed: the declaration is the first fence that DECLARES something, so an unrelated JSON fence (RED-run output, a status object) no longer shadows the real handoff and falsely blocks a compliant deliverable. Audit L1 fixed: an absent `test-report.md` fails open for multi-member lanes, as the gate documents. Salvage review repairs: the multi-member branch binds to `core.ContractTaskIDs` — the lane pin, else the triage decision's `top_n`, minus deferrals: the SAME set the Task Contract handed TDD (parity-tested) — never triage-report.md's markdown `## top_n`, whose decomposition sub-ids would have falsely blocked every decomposed lane; the TDD persona now declares `slugs[]` and a comma-separated `## Task:` from the Task Contract block (prompt-bound by test); the audit defect ledger reads the pin through `core.LaneScopeIDs` (one reader); an explicit empty pin is no pin. Known asymmetry: multi-member lanes skip the single-member file-scope advisory — filed as inbox `multi-member-file-scope-advisory`. The cycle's unmodified build-explanation record does not ride this landing (it describes the pre-repair parser against another base). The eval (`.evolve/evals/multi-slug-lane-scope-reconciliation.md`, six named criteria) and the cycle's predicate suite (`go/acs/cycle1620`) land with it; the inbox item is consumed.

---

## Fixed — token-waste repairs: cancellation reaps its providers, known persona absences cost no LLM, lanes base on the integration HEAD (2026-09-10)

Root-cause record: [docs/reports/2026-09-09-token-waste-root-cause.md](docs/reports/2026-09-09-token-waste-root-cause.md) — the forensic report on the paused batch (cycles 1619–1621: zero landings, three defects each burning lane-minutes). Four repairs, each red-first, none of which restarts the paused loop:

- **Cancellation stops provider sessions** — `bridge.tmuxCleanup` runs on a context DETACHED from the launch context's cancellation and bounded by `tmuxCleanupTimeout`: the deferred cleanup fires exactly when the launch context has been canceled, and a canceled context made every tmux command refuse, so the provider outlived the orchestrator. Cancellation is the reason to clean up, not a reason to skip it (`TestOperatorPauseCleanupCancelsProvider`). The loop also reaps exactly its OWN per-run tmux socket on every exit path including the SIGINT unwind (`runSocketTeardown`; a pre-set operator/nested socket is never killed).
- **Known optional absences learn deterministically** — a missing persona doc (`ErrAgentDocMissing`) still writes the FailedRecord, the carryover todo, the failure digest and the lesson artifact, and dispatches NO retrospective agent (cycles 1619/1620 spent 344 s and 311 s of a deep-tier agent on exactly this). Before that, at plan time, core probes every catalog-Optional, non-floor runner (`core.PersonaProber`, implemented by `runner.BaseRunner.PersonaAvailable` — the same loader and sentinel the dispatch path uses) and hands the absent ones to the router as `RouteInput.UnavailablePhases`: the advisor is not offered them (they are named in their own prompt section), the floor clamp drops them under `DropUnavailablePhaseRule`, and the trigger path never inserts them. Mandatory and floor phases are never probed — their absence stays a loud dispatch failure.
- **Lanes base on the integration HEAD** — ONE resolver, `gitexec.RelationToRemote` (current / behind / ahead / diverged, with counts and one rendered sentence), feeds both the wave boundary and `core.laneStartRef`, so the two can no longer disagree: a lane bases on `origin/main` when the local main is current or behind (the boundary fast-forwards a behind main), on the local main when it is strictly AHEAD (unpublished dossier closeouts ride the next push), and DIVERGED history halts the batch at the wave boundary (`stop_reason: plane_diverged_halt`, exit 2) before any lane spends a phase — the loop never merges; reconcile the plane first. The boundary no longer reports a local-ahead main as "fast-forwarded" (`git merge --ff-only` succeeds without moving HEAD); it names the ahead count and the integration HEAD.
- **Records** — the root-cause report and the general-solution strategy progress report (2026-09-10) land under `docs/reports/` beside the recovery validation report.

---

## Added — solution-skill personas preloaded by a signal-keyed overlay rule; core projects the signals, the personas write what the kernel reads (ADR-0099 slice 3, 2026-09-09)

The prompt layer of "solution cycles on the same spine" (slice 1 #539, slice 2 #543). A document cycle now runs its scout, build and audit under the `solution-scout` / `solution-build` / `solution-audit` personas, selected by a deterministic overlay rule on the cycle's own signals — no flag, no persona fork; the advisor may still add skills through its clamp.

- **Overlay `when` selector** — `policy.OverlayRule.When []config.Condition` keys a rule on `OverlayDispatch.Signals` (`eq`/`ne`; absent signal ⇒ no match, unknown op ⇒ no match). The compiled defaults add `{phases:[scout|build|audit], when: deliverable_kind == document} → solution-*` beside the deep/top → fable rule.
- **Core is the one digester** — `core.PhaseRequest.Signals` is projected once per dispatch (`dispatchSignals`, every PhaseRequest assembly — live loop, resume, batched evaluate): `deliverable_kind` = declared (triage > scout, `router.RoutingSignals.DeclaredDeliverableKind`) > the project default (`.evolve/domain.json`, the reader slice 2 added — only for a kind-declaring phase's own dispatch on a clean digest) > `code`; the phase list is ONE `kindReportPhases` shared with `DocumentCycle`; `scout.goal_type` when the scout declared one. The phase runner copies the map onto the overlay dispatch and re-reads nothing, so the SCOUT dispatch of a document-domain project — no report exists yet — already carries `solution-scout`, and a degraded digest WARNs with the kernel's reason instead of silently dispatching the code persona. The integrity floor (the tdd pin, `DocumentCycle`) keeps reading declarations only.
- **One signal vocabulary** — `config.SignalDeliverableKind` / `config.SignalGoalType` are the kernel's routable field names (`router.resolveField` switches on them), so an overlay `when` clause and a `conditional_mandatory` clause name a signal with the same word; the kind literal is `config.DeliverableKindDocument`.
- **Protected surface** — `policy.CompiledDefaultOverlaySkills()` exports the compiled-default skill set; `ProtectedSurfaceManifest` carries `skills/solution-{scout,build,audit}/` and `TestProtectedSurface_CompiledDefaultOverlaySkills` iterates the export, so the next kernel-preloaded persona cannot skip the manifest.
- **The personas write what the kernel reads** — `router.HeaderGoalType` / `HeaderDeliverableKind` / `HeaderCycleSize` are exported; the scout's DISPATCHED body (`agents/evolve-scout.md`, above the strip marker) carries the header-line directive, the per-task `Deliverable kind` bullet and the kind-conditional Implementation-First exception; triage's `deliverable_kind:` header and final-check rule are authoritative; `TestPersonaTemplates_CarryTheHeaderLines` pins the dispatched persona and the output template to the constants.
- **Skills** — `skills/solution-scout` (discovery over the solution corpus), `skills/solution-build` (diverge into mechanistically distinct options → comparison → recommendation; the contract shape is read from the rendered Task Contract, never restated), `skills/solution-audit` (rubric judgment with `path:line` evidence, evidence integrity, adversarial steelman); generated `commands/solution-*.md`, `.claude-plugin/plugin.json`, `.agents/skills` symlinks, the ollama-compatible list (scout/audit); `examples/eval-solution.md`; `docs/architecture/skill-overlays.md` documents the `when` selector and its three fail-closed meanings.

---

## Added — the document deliverable contract: one deterministic engine behind the build floor, `evolve solution check` and the audit gate (ADR-0099 slice 2, 2026-09-09)

A `document` cycle now has a machine-graded deliverable: `solutions/<slug>/` with at least two candidate strategies under `options/<n>-<name>.md`, a `recommendation.md` (Options Compared · Recommendation) and an `assumptions-and-evidence.md` every number cites. The SHAPE is config (`phase-registry.json:config.deliverable_kinds.document`); ONE LLM-free engine (`internal/solutioncheck`) judges it and is projected three ways so the surfaces can never disagree — quality judgment stays in audit.

- **Build handoff floor** — `core.SolutionFloorChecks(spec)` is chained after `DefaultBuildFloorChecks` in the cycle composition root; `core.SolutionViolations` (shared with the audit gate) reads the kind from the kernel's own digest of the scout/triage report headers and the slugs from the triage decision (never the builder's report), is silent for code cycles, and treats a document cycle that binds NO task as a violation rather than a clean pass.
- **`evolve solution check <solutions/slug>`** — the eval `[code]` grader and the agent's self-check (exit 0 / 1 violations / 2 usage); same engine, same registry contract.
- **Audit gate line** — `Config.SolutionSpec` (the registry contract, handed in by the composition root — the same spec the floor runs; the phase loads no config) forces FAIL on a violation with the single-exit shape gofmt uses; infra errors fail open with a warning.
- **Task Contract carries the kind** — `inboxbatch.Item.DeliverableKind` renders `Deliverable kind: document` plus the contract prose GENERATED from the registry spec (`solutioncheck.Describe` — one renderer, never a hand-typed copy); document cycles skip the Go predicate inventory (the floor is the solution contract).
- **First Go reader of `.evolve/domain.json`** — `config.LoadDomain` → `Domain.DefaultDeliverableKind()` (writing/research ⇒ document) seeded into the scout/triage dispatch context as `deliverable_kind_default`.
- **Commit prefix** — a document cycle's default message is `solution(<slug>): evolve-cycle N` (one slug in the scope; further bound slugs ride in the subject). Vocabulary only: the commit-prefix gate passes the parenthesised form through, so no manifest rule claims a scope it cannot enforce. `docs/domain-adapters.md` (a dangling link since v8) now exists and states exactly which part of the adapter record is implemented.

---

## Added — deliverable kinds: the kernel signals for solution cycles (ADR-0099 slice 1, 2026-09-09)

The factory can now shape a non-code cycle from the same spine. The kernel READS two new header lines — `goal_type:` and `deliverable_kind: code|document` in scout-report.md, `deliverable_kind:` next to `cycle_size_estimate:` in triage-report.md (the scout/triage persona lines that WRITE them land with slice 3; until then every cycle stays undeclared ⇒ `code`, byte-identical to today) — from the report headers (the trusted path `cycle_size_estimate` already takes — handoff JSON has been extinct since ~cycle 215) onto `RoutingSignals` (`scout.goal_type`, `scout.deliverable_kind`, `triage.deliverable_kind`, projected `deliverable_kind`, absent ⇒ `code`).

- **tdd is released for document cycles by config, not code** — `config.CondRule` ANDs clauses (`a!=b && c!=d`; single-clause rules byte-identical), and the registry's `conditional_mandatory.tdd` is `cycle_size!=trivial && deliverable_kind!=document`. The absent default `code` keeps the pin at plan time; the post-scout RePlan releases it for a digested document declaration. The advisor rubric renders one exemption line per clause.
- **The 15 domain phases can finally fire** — `scout.goal_type` had no reader on the kernel side (and no writer); the kernel half lands here, the persona line in slice 3; `TestDomainPhaseTrigger_FiresOnScoutGoalType` reads the shipped `forces-analysis` overlay and proves its `insert_when` evaluates live, with D2 fail-closed semantics preserved for undeclared goal types.
- **Solution recipes** — `strategy-options`, `business-plan`, `partnership-deal` compose the document-side acceptance from the existing catalog (`scope-baseline` before build; `adversarial-review` / `premise-challenge` after); the router persona's recipe block is regenerated and its floor sentence names the release.
- Typed views (`phaseio.ScoutView/TriageView`), the phase-io shadow rows, the advisor digest line and the routing fixtures carry the new fields. ADR: [docs/architecture/adr/0099-deliverable-kinds.md](docs/architecture/adr/0099-deliverable-kinds.md).

---

## Changed — codex deep/top tier → gpt-6-astra at high reasoning; the codex tier table is single-sourced (2026-09-09)

Operator directive: codex's high tiers (`deep` and `top`) run **gpt-6-astra** at the **high** reasoning rung (the rung was already the 2026-09-01 directive — `codexDeepTopRung`, every codex deep/top profile at `effort_level: high`, the manifest's `params.effort.default` — so only the model moves). Verified live before the cutover: `codex exec -m gpt-6-astra -c model_reasoning_effort=high` on the ChatGPT subscription answered, so adding astra to `chatgpt_safe_models` (which lets it through the cycle-142 clamp) is safe. `fast`/`balanced` stay gpt-5.6-luna/terra.

- **One ladder, one table** (closes inbox `codex-tier-map-single-source`; architecture review 2026-09-09) — `driver_codex.go` no longer carries its own tier switch (`mapCodexModel`); the tier→model ladder is extracted ONCE into `bridge.resolveTierModel` (the realizer's flag/repl emit and the headless driver's `-m` composition both call it, so the queued within-tier fallback axis will reach every dispatch), and the headless driver loads its OWN manifest (`LoadManifest(cfg.CLI)`) — the pointer below yields the family table, so no code composes a family name. `codex.json` therefore declares no `model_tier_map` at all (its copy had sat three generations stale at gpt-5.4-mini/gpt-5.4/gpt-5.5 behind the switch): it carries an explicit `model_tier_map_from: codex-tmux` pointer that `LoadManifest` resolves (`resolveTierMapFrom` — a copy of the family table; an unresolvable pointer is a manifest error, never an empty map; explicit rather than implied from the family name so claude-p, which legitimately declares no map, is untouched). `TestCodexHeadlessManifest_PointsAtFamilyTable` and `TestLoadManifest_ModelTierMapFrom_UnresolvableIsAnError` pin it. `TestResolveTierModel_FollowsManifest` is the wiring proof (a fixture map is followed on both paths with no code edit); `TestLaunch_Codex_ResolvesTierViaFamilyManifest` and `TestLaunch_Codex_ManifestUnavailable_OmitsModelFlag` cover the launch and its degradation.
- **No more copies** — the e2e live matrix, advisor-matrix and smoke tables derive codex ids from the manifest (`codexTierModels()` loads the headless `codex` manifest and gets the family table through the pointer; a load failure panics rather than launching without `-m`); the tmux clamp WARN, the clamp/manifest/setup comments and the capability-matrix, setup-onboarding and dynamic-model-routing docs point at `codex-tmux.json` `model_tier_map` instead of restating ids (this CHANGELOG is the value record). Every other codex tier test asserts relationally against the manifest; `codex_tier_map_test.go` holds the one value pin. `TestRetrospectiveRoutesToCodexSol` → `TestRetrospectiveRoutesToCodexDeep`.
- Docs: runtime-reference deep-tier row, CLAUDE.md critical facts (resolution is policy pin > live catalog > manifest baseline), both manifest notes.

---

## Added — the Task Contract block: acceptance verbatim + the ACS predicate inventory in the tdd, build and audit prompts (ADR-0098, 2026-09-03)

The acceptance criteria a cycle is graded against live on the inbox item, but reached the builder only through two LLM hops of prose, and the predicates tdd wrote reached it only by grep. `core.seedTaskContract` (both dispatch surfaces) now renders a harness-owned `## Task Contract` block into the tdd, build and audit prompts (the grader reads the same words the builder was handed): each bound task's `acceptance[]` copied VERBATIM from its inbox record (`inboxbatch.LoadFile`, one source, sanitised as a prompt surface), and — for build and audit — the predicate names `go test -list . -tags acs ./acs/cycle<N>` finds in the worktree (bounded by the ACS lane timeout). Unresolved records, items without acceptance, and a cycle with no predicates package are loud lines in the block, never silent omissions. The filed `handoff-tdd.json` half was retired: the inventory derived from the test files IS the handoff. Closes inbox `inline-task-contract-into-build-prompt`.

---

## Fixed — a missing `## Explanation Documentation` section is a correction, not a terminal FAIL (2026-09-03)

Cycles 1601 and 1603 died on `audit-report.md is missing ## Explanation Documentation`: the audit's deterministic gate forced FAIL and nothing re-asked the auditor for a section that costs one re-dispatch to add. The audit contract now declares the section as CONDITIONAL (`phasecontract.ExplanationDocumentation` under `Contract.ExplanationSections`), and every verifier enforces it while the cycle's explanation contract is active — the version rides on `phasecontract.Roots`, so the host gate, the runner, the salvage re-check and the agent's own `evolve phase verify` (reading `cycle-state.json`) agree; the match is the exact visible heading the audit gate itself uses (`reportdoc.HasSection`) — under the same `missing_section` code every other section uses — so the existing correction ladder re-dispatches the auditor with the reason as its directive. A cycle without the contract is never asked for the section; the audit gate's own check stays as the backstop. Closes inbox `auditor-explanation-section-correction-ladder`.

---

## Fixed — read-only phases can no longer rewrite the builder's tree; the explanation review reads what auditors write (ADR-0097, 2026-09-03)

Seven of eleven consecutive FAILs (cycles 1596–1606) were the harness rejecting its own pipeline's output. Three were the deterministic explanation-digest gate firing on a tree the AUDITOR had altered — its mutation probes rewrote the builder's material files in place (`cp /tmp/digest.go.bak …`, "wrote reverted tdd.go"; reproduced forensically from the salvage worktrees) — and the profile's `sandbox.read_only_repo` was inert because the wave ran `sandbox=false`. Five were the `## Explanation Documentation` section rejected on FORMAT: several `- Evidence:` lines ("duplicate evidence fields"), a line-range citation, backticked values, citations under other field names.

- **Worktree fence** — `internal/treefence` snapshots a worktree as a git tree object (throwaway index; tracked + untracked, ignore rules respected) and restores it. `core.PhaseRequest.WorktreeReadOnly` is derived at every dispatch site from the ONE write-permission predicate (`worktreePhase`) and cleared on a remediation builder fix; `phases/runner` fences every read-only dispatch around its attempts, before the classify hooks, and reports each restored path as a diagnostic on the response. Fail-open, loudly; verdicts untouched.
- **Explanation-review tolerance** — `reportdoc.Fields` keeps `Evidence` list-valued and strips surrounding backticks; `path:23-48`, `path:12:3` and `path#L7-L9` are citations; `reportdoc.EvidenceOrBody` falls back to the section prose. The grounding rule (every material path cited at a line) is unchanged, and the three real sections are the regression fixtures.
- Incident record: [docs/incidents/2026-09-03-auditor-mutates-the-worktree.md](docs/incidents/2026-09-03-auditor-mutates-the-worktree.md). The research's R3 draft (a native `build-floor` phase) was retired: the deterministic build handoff floor already exists in the E2 reviewer chain, and the evidence pointed at audit-time mutation.

---

## Fixed — repair rounds escalate tier + effort and carry the auditor's findings (ADR-0096, 2026-09-03)

Ship probability by audit-round count ran 100 % → 50 % → 17 % → 0 % over cycles 1560–1605: every in-cycle repair round was a byte-identical retry (same tier, same effort, same brief of gate strings), and the profiles' declared `model_tier_overrides.audit_retry_2plus: "deep"` had no producer — nor was its resolver on the production dispatch path.

- **R1** — `core.repairRoundTier` raises a tdd/build re-dispatch inside an audit-repair round to the profile's declared `audit_retry_2plus` tier (raise-only, envelope-clamped through the ADR-0076 D guardrail), on both dispatch surfaces; the phase runner honours it as the routing overlay. Effort follows the tier through the new profile `effort_overrides` at the bridge launch (builder deep→high, tdd-engineer deep→xhigh).
- **R2** — `core.composeRepairBrief` briefs the rejecting round's auditor findings (CRITICAL/HIGH/MEDIUM via `reportdoc.Findings`, the grammar the dashboard and the audit gate share) after the gate reasons, marks findings that persisted from the previous round, and archives the previous attempt's `<phase>-prompt.txt` as `.round<N>.` beside the audit archives.
- Incident record: [docs/incidents/2026-09-03-repair-rounds-blind-and-flat.md](docs/incidents/2026-09-03-repair-rounds-blind-and-flat.md). The subagent-CLI-path plumbing was built, found inert (I2), and discarded before landing.
- **Review fleet (architecture + go), addressed before landing** — the raise-only/envelope-clamp skeleton lives once (`core.clampedRaise`, shared by the ADR-0076 D floor and the repair raise); the finding-severity rank is `reportdoc.SeverityRank` (the dashboard panel and the repair brief project it, so a severity added to the grammar reaches both or neither); the dispatched-prompt filename is `phasecontract.PromptArtifactFilename` (bridge writer, audit probe quarantine, repair archiver); `effort_overrides` values are covered by the effort realizability guard and pinned to the directive rungs beside the effort matrix; the prompt archiver is self-guarding on the persisted repair state (one predicate, both dispatch surfaces); the resume surface has its own `RunCycleFromPhase` wiring proof; an unreadable audit report WARNs like its sibling gate reader; the brief truncation cuts at a line boundary.

---

## Added — `evolve dashboard`: a live, read-only view of the pipeline + the ship-rate investigation (2026-09-02)

The batch SLO "SHIPPED ≥ 60 %" lived in a code comment and nothing computed it. Measured from the committed dossiers it was **19.6 %** over cycles 1560–1605 (an 0 / 11 streak), 81 % of failures land at audit, and ship probability by audit-round count runs 100 % → 50 % → 17 % → 0 % — a repair loop that grinds rather than converges.

- **ADR-0095** — `evolve dashboard [--addr] [--project-root] [--snapshot]` (`internal/dashboard`): loop status from the run lease and the brake marker, the inbox, per-cycle phase progress with audit rounds, a **what-went-wrong** panel (failure-decision · disposition · deterministic gate reasons · the final round's auditor findings · the repair-round diff across `audit-report.round<N>.md` archives · salvage pointer), a phase × cycle grid, ship-rate trend and Sentry-style fingerprint groups (count, first/last seen, regressed) computed from `knowledge-base/cycles/`. Stdlib-only, GET-only, loopback-bound with a `Host` guard against DNS rebinding, one SSE stream with a mtime-fingerprint poller (the serve context is the requests' `BaseContext`, so shutdown closes open streams); every artifact string reaches the DOM via `textContent`, artifact reads are allowlisted, symlink-safe and capped; a torn or unreadable artifact is a named warning on the page, never a silent blank.
- **Single homes given to names that had none** (the architecture review's Block): the auditor's prose verdict grammar moved verbatim from `phases/audit` into `reportdoc.Verdict` (+ `Findings`, `FindingKey`), which the gate's below-enforce fallback, the dashboard and the coming repair-brief seed all call — and which now scans visible lines only, so a fenced template can no longer declare a verdict; `phasecontract.RoundArchiveFilename` (writer `core.retireSupersededAuditArtifacts` + both readers); `dossier.CyclesDir` (producer, chronicle seed, dashboard); `bridge.LLMCallsLogFilename` (engine writer, `tokens`, `models live`, dashboard); `paths.EvolveDirOf` and `paths.LoopStopPath` (the `.evolve` and `loop-stop` spellings; the chain driver reads the latter too). Guide: [docs/guides/pipeline-dashboard.md](docs/guides/pipeline-dashboard.md).
- **Research** — [ship-rate-harness-reliability-2026-09-02.md](docs/research/ship-rate-harness-reliability-2026-09-02.md): eleven source-verified architectural gaps (the declared `audit_retry_2plus → deep` escalation can never fire; the auditor's HIGH findings never reach the repair brief; acceptance criteria sit two hops from the builder prompt; every deterministic check fires after the full build budget; the learning loop has no re-entry), the failure-bucket census of the last nine FAILs, a 60-source literature survey on making weaker models do exactly what was asked through harness design, and eight ranked proposals. Design spec: [docs/superpowers/specs/2026-09-02-ship-rate-harness-and-pipeline-dashboard-design.md](docs/superpowers/specs/2026-09-02-ship-rate-harness-and-pipeline-dashboard-design.md).

---

## Fixed — a transient upstream error inside an artifact timeout is no longer invisible (#478, 2026-08-21)

3 of 4 observed router stalls (cycles 1523/1524/1526) burned the full 600s silence budget on a pane that stated its own cause verbatim — `API Error: 529 Overloaded. This is a server-side issue, usually temporary` — then degraded routing to the static spine. cycle-1526 paid it twice in one cycle. Three recognition paths existed and a transient server error fell between all of them: it is not a quota wall (`exhausted_regex` correctly ignores it), not a transient exit code (80/85/86), and exit 81 is contractually non-retryable — correctly so. The distinguishing evidence was already captured in the pane and simply never consulted.

- **ADR-0090** — the discrimination rides the cause as **data**, never as a reclassification. The one self-describing `artifact-timeout:` marker line now carries a driver-authored `transient=true|false` field; exit 81 stays non-transient by exit code, so `transient-bridge-retry` AC-1 is preserved by construction rather than by careful re-reading. The field EXTENDS that line rather than competing with it, which makes the cause-displacement regression class (the drift alarm's "quota", the workspace file listing) structurally unreachable.
- **LLM-agnostic by construction** — the pattern is resolved from the LAUNCHED cli's manifest, and all four tmux families declare their own provider's signature; no vocabulary is hard-coded in Go. `transient_regex` is a **top-level** manifest key, not a sibling of `controls.usage.exhausted_regex`: the two read different surfaces (the `/usage` control's output vs the working pane), and `ollama-tmux` has no usage control at all — the sibling placement the design proposed left one family structurally unable to declare recognition.
- **Two designs were killed by looking rather than by review** — a stderr-buffer classifier would have passed acceptance 3/3 GREEN while never firing on a real API error (the exit-81 stderr path is entirely bridge-authored; provider text renders in the PANE, a different sink), and the `controls.usage` placement was falsified by ollama during implementation. Both are recorded in [docs/incidents/2026-08-18-transient-529-inside-artifact-timeout.md](docs/incidents/2026-08-18-transient-529-inside-artifact-timeout.md) with the regression-coverage map.
- **Scope, stated** — this delivers the diagnostic data, not the control flow. The 600s is still burned; it is now diagnosable. And only `claude-tmux`'s pattern is grounded in a live captured pane — codex/agy/ollama come from documented error shapes, so the next non-Claude occurrence is the evidence that confirms or corrects them.

---

## Added — reasoning-model: chain-of-reasoning audit verdicts + scope-delta adjudication (2026-08-12)

Two design changes that replace a class of mechanical gate with a reasoning step, plus the root-cause record for why that class kept failing.

- **ADR-0088** — the audit verdict is the **conclusion of a chain**, not an assertion. Seven cross-phase coherence links (intent → selection → specification → implementation → narrative → delivery → evidence), each with a status, a finding and a citation; the verdict is computed from the statuses and never reads the prose, so an auditor *cannot assert PASS over an incoherent link*. `unverifiable` is the honest middle (a link nobody could check must not launder into "fine"), and a **missing** link fails harder than a negative finding. Evidence entitlement applies to every judging phase — audit, adversarial-review, coverage-gate, plan-review, inherited-defect-reconcile, retro — with a downgrade that stops a *narrated* chain passing as a walked one. Live in **shadow**: the per-cycle record (`audit-chain-shadow.json`) carries narrative vs chain verdict, agreement, diagnoses, missing evidence, and what actually shipped.
- **ADR-0087** — scope-delta adjudication: six classes by meaning, `CARVE` as the missing middle (preserve the work, ship clean), and an accounting invariant that makes silent disposal inexpressible. Its discriminators are now *evidence inside links* rather than a decision procedure of their own.
- **Findings ledger** — [docs/incidents/2026-08-12-proxy-as-verdict-findings.md](docs/incidents/2026-08-12-proxy-as-verdict-findings.md): 15 findings from four adversarial review passes, each with symptom, root cause, why it happened and how it was resolved — unified by one recurring defect, **a proxy used as a verdict**. Includes the four-attempt failure of `schema-aligned-salvage-layer` (the work was never broken, it was *stranded*), the anti-gaming hinge that was bypassable by one string field, and the evidence table that would have pinned the soak's headline datum to a constant.

---

## Fixed — persona-strip lobotomy campaign (#434–#447, 2026-08-10 → 2026-08-11)

The 15/30-FAIL streak (cycles 1390–1429, continuations 0/11) traced to ONE defect: `CompactPrompts` stripped every dispatched audit prompt at the mid-file `## Reference Index` marker, deleting the Verdict Rules, STOP criterion, and the continuation-disposition mandate from what the auditor actually received. Incident: [docs/incidents/2026-08-10-persona-strip-lobotomy.md](docs/incidents/2026-08-10-persona-strip-lobotomy.md). Campaign landings, all console-first with TDD + adversarial review:

- #434 marker→EOF for the auditor + fleet-wide sentinel keep-guard (`phasecoherence/persona_strip_operational_test.go`); #437 the same curation for builder/scout/tdd-engineer/triage + inverted burial-era test pins.
- #435 iteration-0 escalation swallow (zero-value stamp); #436 bookkeeping-FAIL regrade micro-cycle (ADR-0086; retro→audit edge, once-bound grant consumed on BOTH record and resume surfaces); #438–#442 disposition preseed, carryover retirement + P0 dedupe + stable failure identity, continuation base-advance, FailReasons backfill (the evidence plumbing that later made the cycle-1431/1434 halts self-diagnosing).
- #443 spec clamp + sandboxed-profile predicate at both admission seams; #444 postship lane-scope fallback; #445 routing-overlay family rung; #446 ship-journal provenance + `evolve ship --push-only` strand recovery (first live validation 2026-08-11: 24 attested commits); #447 `Roots.DispatchedArtifact` one-source verify seam.

---

## Fixed — ledger chain-safety: lifecycle appends chained + physical-tail rebaseline seal (#450, 2026-08-11)

Two defects behind the per-cycle ledger chain breaks (item `ledger-fleet-concurrency-chain`, ~180 dense console breaks since mid-July): `inboxmover.writeLedger` raw-`O_APPEND`ed unchained NDJSON into the hash-chained `ledger.jsonl` on essentially every cycle's ship.postship, and `Rebaseline` sealed from `ledger.tip` while chain verification walks physical predecessors — so the repair command failed on exactly the damage class it was built for.

- Lifecycle records go through `FileLedger.AppendLifecycle` (chained, flocked, atomic tip replace); `core.LedgerEntry` gains additive `task_id`; from/to/reason fold into `message`.
- `Rebaseline` seals via `appendChainedFromTail` (prev_hash = sha256 of the physical last line); both writers share one `appendLineAndReplaceTip` write-half.
- Live proof: console-plane ledger rebaselined GREEN — first intact chain since the 2026-07-22 break@78729. Incident: [docs/incidents/2026-08-11-verify-wave-halts-and-ledger-forensics.md](docs/incidents/2026-08-11-verify-wave-halts-and-ledger-forensics.md).

---

## Fixed — ACS suite plane-anchored project root + verdict provenance + foreign-root regeneration (#449, 2026-08-11)

Cycle-1434's ADR-0072 halt: `evolve acs suite` derived `EVOLVE_PROJECT_ROOT` via `git --git-common-dir`'s parent (the OWNING repo) — wrong since the runtime plane became a linked worktree — and the wrong-root verdict suppressed the audit phase's correct-root run.

- `suiteProjectRoot` anchors on the kernel-owned `runs/cycle-N/cycle-state.json` under `--evolve-dir`; git walk is the no-plane fallback only.
- Verdicts stamp `suite_root`/`project_root` (absence = unstamped); audit regenerates on stamped-root mismatch, preserving the foreign artifact as `acs-verdict.foreign.json`. See [docs/architecture/egps-v10.md](docs/architecture/egps-v10.md).

---

## Fixed — closure-claim gate word-boundary + negation-aware weak rung (#448, 2026-08-11)

Cycle-1431's ADR-0072 halt (prior firings 1339/1371/1428): the closure-citation gate's unbounded substring match fired on "closed" inside **"disclosed"** on a line ending "still open", force-FAILing a narrative-PASS audit. Matchers are now word-bounded and two-rung: `verified closed` is never guard-suppressed (no one-token bypass of the citation demand); bare `closed`+cycle-ref accepts negation/openness guards. Design record in `closure_claim.go`; posture note in [docs/architecture/continuation-defect-ledger.md](docs/architecture/continuation-defect-ledger.md).

---

## Documented — `evolve loop --reset --fingerprint` operator flag (cycle-1333, 2026-08-05)

The blocker-breaker fingerprint-ack mechanism (`--reset --fingerprint <fp>`, ADR-0072 extension, implemented cycle-1332) had zero mention in the operator-facing runtime reference — an operator hitting a recurring identical-fingerprint halt had no documented way to unblock it short of reading the Go source. Docs-only; no production code changed.

- `docs/operations/runtime-reference.md` — new bullet under "Operator commands (write a verdict file)" describing the flag, its `--reset` gating, the `.evolve/resolved-fingerprints.json` ledger it appends to, and the `BlockerBreakerConfig.AckedFingerprints` field that consumes it.
- See `go/internal/core/blocker_breaker.go` and `go/cmd/evolve/cmd_loop_fingerprint_ack_test.go` (`TestRunLoop_FingerprintAck_AppendsLedgerRecord`) for the underlying implementation and test coverage, both unchanged by this entry.

---

## Verified closed — cycle-1272 fleet-scope items (2026-08-04)

Both fleet-scope backlog items pinned to cycle-1272 were investigated and found **already-implemented**; no production change was required. They are recorded here as verified-closed so they stop re-appearing as open backlog.

- `infra-teardown-predicate-single-source` — already implemented. The infra-teardown predicate is single-sourced as `IsInfraTeardownError` in `go/internal/core/errors.go`; every consumer (`orchestrator.go`, `cyclerun_dispatch.go`, `runner.go`) calls it directly rather than re-spelling the condition. Proof: `TestInfraTeardownUnion_SpelledExactlyOnce` in `go/internal/core` — PASS.
- ~~`retro-fleet-worktree-dispatch` — already implemented.~~ **Corrected in cycle-1283 — this closure was wrong.** It held only for the `req.Worktree == ""` shape. The shape a torn-down fleet lane actually produces is a **non-empty but stale** path (`cs.ActiveWorktree`, never cleared at teardown), which passed through the `== ""` condition verbatim into the bridge guard's refusal — the cycle-1255 CRITICAL was still live when this line was written. The cited proof `TestRetroWorktree_FleetScratchCwdSatisfiesBridgeGuardPredicate` is genuine but pins only the empty shape, so the machine guard below could not catch the over-claim: it re-ran a test that was never about the open half. Closed for real below.

Closure is machine-guarded, not merely asserted: the cycle-1272 acceptance predicates re-run both cited tests and require an explicit pass line, so this entry goes red if either proof breaks or is renamed. That guard is necessary but **not sufficient** — it verifies the cited proof still passes, not that the proof covers the claim (see the correction above).

---

## Fixed — retro stale-worktree fallback, the open half of the cycle-1255 CRITICAL (cycle-1283, 2026-08-04)

A fleet lane whose worktree had been torn down lost its retrospective entirely — a failure in the failure-handler, so the cycle that most needed a post-mortem was the one that could not produce one. `cs.ActiveWorktree` was write-only: the teardown callback pruned the directory but nothing cleared the record naming it, and retro's fallback tested for `""` rather than for the bridge guard's own predicate.

- `go/internal/phases/retro/retro.go` — `retroWorktree` falls back to the workspace-owned scratch cwd when `fleetMode(req) && !gobridge.IsDir(req.Worktree)`. Empty and stale are one contract; a live worktree still passes through verbatim and non-fleet dispatch is untouched.
- `go/internal/core/cyclerun.go` — the lane-teardown callback calls `clearActiveWorktree` once `Cleanup` **succeeds**, removing the dangling reference at its source. A **preserved** worktree deliberately keeps its path, since `--resume` / `evolve cycle reset` reclaim the lane by it.
- `isDir` was exported as `bridge.IsDir` rather than re-derived in the phase — a launch-refusal predicate with two definitions can drift, and this defect is what that drift costs.

Full issue/gap/solution record: [docs/operations/fix-2026-08-04-retro-stale-worktree-fallback.md](docs/operations/fix-2026-08-04-retro-stale-worktree-fallback.md); landing entry under Finding F1 in [docs/operations/batch-integrity-review-2026-08-04.md](docs/operations/batch-integrity-review-2026-08-04.md).

---

## [22.26.0] - 2026-09-30

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.25.0..HEAD._

### Added

- remove the pipeline's stale temp artifacts past gc.temp_ttl_hours
- report the disk a real run released
- the operator run applies the run-dir retention ladder
- trim go build cache entries unused past gc.go_cache_ttl_hours
- salvage then remove finished dirty or unmerged cycle trees
- terminate orphans left running inside a finished cycle's tree
- NO_WORK, the recorded planned no-work end
- RecordEnding, and a zero-attempt record keeps the usage sidecar
- TRIAGE_SCOPE_UNANSWERED and the claim-failed termination reason
- Unanswered, the lane pin minus the decision's answers
- evolve inbox route-console, the operator's console route
- scope a repair round to the explanation document when told
- record an explanation-only audit FAIL in audit-fail-reason.json
- classify an audit FAIL whose defects all name the explanation document
- stop after this wave with the brake and evolve loop-stop
- one decision on whether the repo-contract pack runs for a tree (unwired)
- the pack's classifier keeps each failure's own output (M3, ship half)
- RepoContractFloorChecks, the build floor's repo-contract check (unwired)
- RunRepoContractPack, the one entry to the repo-contract fixed pack for the build floor
- MaintenanceConfig and ConfigEnv, one source for quiet git
- LiveSiblingRun names the run that blocks and why
- publish pending closeout dossiers when no lane runs
- a fleet lane leaves its closeout dossier pending
- publish a fleet lane's pending closeout pair at the boundary
- acsassert.GoTests, ACS predicates judge go test -json events

### Fixed

- re-review follow-ups — cycle- guard on runClosedOut; report types pinned
- architecture review — the process reaper defers to isLive; one TOCTOU pass
- review — Test* proves no ownership; a run-dir apply failure fails the run
- a dry run reuses one temp journal per version
- a closed-out run's stale active_worktree never keeps its worktree live
- a reaped dead tmux server's socket file is removed
- ban the allow-list scope fence cycle 1761 could not satisfy
- the composed-tree gates run in the CI environment and name their failure
- a wave-boundary sync names the plane-side inbox files that block it
- a claim-failed lane charges the pins it left unanswered
- the closeout records how triage ended the cycle at C1
- a protected card that empties a single-item lane answers for its bound item
- the findings header names gate reasons only when they lead the brief
- the brief's gate slot keys on the runner's reasons, not the workspace file
- the retry adjudicator reads the audit's reasons through the brief's reader
- the repair brief reads the audit's failure block when no gate record exists
- prose before the document path no longer hides a doc-only FAIL
- the re-author section asks for the report rewrite that completes Build
- a doc-only audit FAIL re-authors the explanation at Build, not TDD
- read any go.mod module line; derive evolve-loop's module from this package's own path (M5)
- ship's gate and the build floor make one decision on running the pack (#711 e2e)
- a SIGINT during a chained batch's closeout stops the chain
- the re-exec handoff has its own field, expires, and is taken once at boot
- the brake's place at the wave boundary is pinned
- a boundary re-exec keeps the wave index and the budget
- a pending interrupt survives the boundary refresh
- the repo-contract floor runs the pack under its own 120s deadline (M2)
- ship names reds only for its real red and hands the floor the failure text (M1, M3)
- ship picks the repo-contract pack's module dir; the floor's dial read WARNs and serves selfcheck (review)
- the build handoff floor runs ship's repo-contract pack, so a raw git fixture is corrected before the audit
- the doc guard sees a cd into a doc root after another command or behind a reserved word
- every test recipe runs git with background maintenance off
- docdelete judges the program bash runs; the ship guard's native allow stays strict
- release gates read the required CI run, not the newest run of any workflow
- watch required CI, not the go and CI workflows that no longer run alone
- the dossier publish re-checks for a live run once it holds the lock
- the held-publish WARN names the blocking run and how to clear it
- an identical corpus record is committed, not assumed published
- the dossier publish refuses while another run is live
- a pending half that is not a regular file is refused
- a pending pair never overwrites a record the corpus holds
- the shipped-lane test fixture keeps git maintenance out of the background
- the durable ACS suite no longer leaks an evolve binary per run
- evolve sync-main is not blocked by an untracked file
- the audit, the audit binding and Ship read one ship tree (F43 component 4)

### Changed

- one CI environment allowlist, CIEnv(environ)
- name the runner-diagnosed-audit predicate once; drop a stale comment clause
- retire the doc-only route's copy of the defects in audit-fail-reason.json
- retry@explanation joins the retryAction vocabulary block
- the boundary refresh is called through wiredRefresher
- one home for the repo-contract module dir; the pack list's cost note names every caller (L1, L2)
- MaintenanceConfig() returns a copy of an unexported list
- flatten the required-CI routing script with early exits
- the fleet binary is resolved in one place

### Documentation

- describe the review's liveness, TOCTOU and effect-strategy changes
- evolve gc releases finished cycles' resources; consume and file inbox items
- C6, the carry's composed gates in the CI environment; two follow-ups
- the console route at a boundary; console-route-stamps-block-wave-sync notes
- R1c, the host records how triage ended the cycle at C1 (cycle 1757, 1758)
- X2a.5, the repair brief reads the failure block for every agent-graded FAIL
- X2a, the doc-only audit FAIL re-authors at Build (cycle 1745)
- the repo-contract pack runs only in evolve-loop's own module; e2e runs need an empty home (#711)
- the review's fixes in the protocol, the package doc and the design
- the wave-boundary protocol stops with evolve loop-stop
- correct the repo-shape claim (M4) and document the review follow-ups; file the cancel-orphan item
- the repo-contract pack at build exit — CHANGELOG, package docs, runtime reference; inbox consumed
- builder and tdd-engineer name internal/gittest for git-backed fixtures
- restore the blank line before a merged heading
- gittest.MaintenanceConfig is the source, the Makefile mirrors it
- reflow the two doc comments the scoping fix rewrote
- state the force-push trade-off; record the second review round
- the dossier entry moves to the top after the rebase onto the wave-29 train
- the always-reporting CI required check, salvaged from the 2026-09-14 campaign
- port the parked test-campaign tracker with its 2026-09-28 salvage status
- the required-CI-result design and implementation reports, corrected for the port
- the sibling-run rule lives in the package doc, not FleetLaneActive's comment
- the live-run check runs twice and names the run; identical records commit
- the publish refuses a live run, a corpus record and a non-regular file
- record the pending-dossier boundary publish (cycle 1704)
- the shipped-lane fixture keeps git maintenance out of the background
- one wave is --max-cycles 1; --max-cycles counts waves in fleet mode
- the wave boundary goes through the CLI only
- F43 component 4, the one ship tree the audit, the binding and Ship read

### Other

- Merge remote-tracking branch 'origin/main'
- Merge train: wave-42 boundary (#727 #728 #729 #730)
- Merge remote-tracking branch 'origin/feat/gc-releases-unneeded-resources' into train/wave42-boundary
- Merge remote-tracking branch 'origin/chore/inbox-cleanup-interface-defects' into train/wave42-boundary
- Merge remote-tracking branch 'origin/fix/tdd-scope-fence-exempts-pipeline-artifacts' into train/wave42-boundary
- Merge remote-tracking branch 'origin/fix/composed-gates-run-in-ci-env' into train/wave42-boundary
- (dossier) cycle-1762 closeout
- (dossier) cycle-1761 closeout
- (config) release finished cycles' resources after a day
- Merge train: wave-41 boundary (#724 #725)
- Merge remote-tracking branch 'origin/fix/wavesync-names-inbox-stamps' into train/wave41-boundary
- Merge remote-tracking branch 'origin/fix/claim-failed-charges-unanswered-pins' into train/wave41-boundary
- (dossier) cycle-1760 closeout
- (dossier) cycle-1759 closeout
- evolve-cycle: goal=07f3360964c1a618006b3419a27950873779d0f68512fe6cc53fa83bf386f8bc
- Merge pull request #723 from mickeyyaya/chore/inbox-shrink-batch-2
- (dossier) cycle-1758 closeout
- Merge pull request #722 from mickeyyaya/chore/route-goal-text-item-console
- Merge train: wave-40 boundary (#717 #718 #719 #720)
- Merge remote-tracking branch 'origin/feat/inbox-route-console' into train/wave40-boundary
- Merge remote-tracking branch 'origin/fix/protected-route-answers-the-lane-pin' into train/wave40-boundary
- Merge remote-tracking branch 'origin/fix/observer-test-joins-goroutines' into train/wave40-boundary
- Merge remote-tracking branch 'origin/fix/repair-brief-from-failure-block' into train/wave40-boundary
- (dossier) cycle-1757 closeout
- (dossier) cycle-1756 closeout
- evolve-cycle: goal=b3266735c274f045db07e229c0239ba13d2d5aa63aaab9419e44e61ed2a7ced8
- Merge train: wave-38 boundary (#714 #715)
- Merge remote-tracking branch 'origin/chore/inbox-declare-files' into train/wave38-boundary
- Merge remote-tracking branch 'origin/fix/doc-only-audit-fail-reauthors' into train/wave38-boundary
- (dossier) cycle-1755 closeout
- evolve-cycle 1755: goal=25191dd15f9965929c3839f739fb69976446dc314c8cf5f83de021b2740e8fc0
- (dossier) cycle-1754 closeout
- evolve-cycle 1754: goal=426938b5d4db0f444d5afd73c6322cc1731e197002a380554683f9ccdb8a7e67
- Merge train: wave-36 boundary (#711 #712)
- Merge remote-tracking branch 'origin/main' into train/wave36-boundary
- (dossier) cycle-1753 closeout
- evolve-cycle 1753: goal=a894f0b86995fade0630a5de752eacacca287bc7daf05a6eca8361627822ef35
- Merge remote-tracking branch 'origin/fix/loop-controls' into train/wave36-boundary
- Merge remote-tracking branch 'origin/fix/repo-contract-pack-at-build-exit' into train/wave36-boundary
- (dossier) cycle-1752 closeout
- evolve-cycle 1752: goal=ca63c90375ee1eb55221bdecbeb38e84dc853ecda03f15b6cac21ae458e4cde5
- (dossier) cycle-1751 closeout
- evolve-cycle 1751: goal=10934a0e648cfffc7a37b6678c14f1450918195f560a28054cab612b3465d8b3
- (dossier) cycle-1750 closeout
- evolve-cycle 1750: goal=015b45db52091a6d611d5b97bbe776457337050e2f1b97beaa6a4c2629a8e078
- (dossier) cycle-1749 closeout
- evolve-cycle 1749: goal=be5b5513061d60fd6a83a70787d94683f0d3c8620e0221a0ae07933c6d1c39de
- Merge train: wave-31 boundary (#708 #709)
- Merge remote-tracking branch 'origin/main' into train/wave31-boundary
- (dossier) cycle-1748 closeout
- (dossier) cycle-1747 closeout
- evolve-cycle: goal=5c89f1ce182201eed5ba5486d8b4227423ed704d8f0b8685e6349117144397a7
- Merge remote-tracking branch 'origin/ci/test-recipes-quiet-git-maintenance' into train/wave31-boundary
- Merge remote-tracking branch 'origin/fix/docdelete-cd-after-another-command' into train/wave31-boundary
- Merge train: wave-30 boundary (#704 #705 #706)
- Merge remote-tracking branch 'origin/main' into train/wave30-boundary
- (dossier) cycle-1745 closeout
- evolve-cycle: goal=0e9cfaa29b8a5e0561cdff461b2977756fb57006cabce28097688aa04c4b40af
- Merge remote-tracking branch 'origin/fix/docdelete-denies-evasions' into train/wave30-boundary
- Merge remote-tracking branch 'origin/ci/always-reporting-required-result' into train/wave30-boundary
- Merge remote-tracking branch 'origin/feat/dossier-pending-at-boundary' into train/wave30-boundary
- Merge remote-tracking branch 'origin/main' into feat/dossier-pending-at-boundary
- (dossier) cycle-1746 closeout
- Merge train: wave-29 boundary (#698 #699 #700 #701 #702)
- Merge remote-tracking branch 'origin/main' into train/wave29-boundary
- (dossier) cycle-1744 closeout
- evolve-cycle: goal=d2f95587223f163ffcdd28aa09062fab550420c05f23a6c8e0fe666d099e6515
- (dossier) cycle-1743 closeout
- Merge remote-tracking branch 'origin/fix/core-lane-fixture-gittest' into train/wave29-boundary
- Merge remote-tracking branch 'origin/docs/max-cycles-counts-waves' into train/wave29-boundary
- Merge remote-tracking branch 'origin/feat/acsassert-gotests' into train/wave29-boundary
- Merge remote-tracking branch 'origin/chore/comment-reduction-round8' into train/wave29-boundary
- Merge remote-tracking branch 'origin/fix/testmain-defer-before-exit' into train/wave29-boundary
- (dossier) cycle-1742 closeout
- (dossier) cycle-1741 closeout
- Merge pull request #697 from mickeyyaya/fix/sync-main-untracked-is-not-dirty
- Merge pull request #696 from mickeyyaya/fix/audit-binds-the-tree-ship-commits
- Merge branch 'main' into fix/audit-binds-the-tree-ship-commits
- (dossier) cycle-1737 closeout
- evolve-cycle: goal=e13ebd7602581c0592705f131b4eb3e51ed04d3414dfaa570108affc9df88074
- (dossier) cycle-1738 closeout


---

## [22.25.0] - 2026-09-28

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.24.0..HEAD._

### Added

- TakeStaged hashes the tree Ship will commit
- the build floor counts the comments a build adds, in shadow
- a comment_floor stage block, shadow by default
- a comment moved within the diff is not added
- a fence keeps the writes to the paths its dispatch may write
- the auditor row can name the base its worktree stood on
- the architecture reviewer audits test-first, Clean Code and Design Patterns
- a load warning says whether its file was unreadable
- one rule for whether a fleet lane may take an inbox id now
- a debugger's conflict resolution rejoins the fleet rebase, never a stale-base reship
- the importer backstop re-runs a red package by itself before it is the lane's RED
- the binding rule accepts a carry it has re-proven itself
- a byte-identical rebase carries its audited verdict to ship instead of a second audit
- the identical-rebase carry record names the audited tree, and ship can read it back
- the byte-exact change between a base and a tree, one leaf for every reader
- the prompt names the protected surfaces and the drop reason; the persona carries the rule
- RetireUnbacked is the one door, and writes only for an id with no lifecycle evidence
- RetireUnbacked writes a retirement record for an id no inbox item backs
- a fleet rebase replays the audited change, not ship's commit (ADR-0105 B1) (#652)
- an identity-preserving rebind of the Build explanation (ADR-0105 B2) (#649)
- code-comment convention, commentaudit proof tool, and a commit-gate waiver for proven comment removal (#638)
- the fatal-pane fast-fail has its own dial and acts by default (F27) (#627)

### Fixed

- an allowance is a ceiling, so a lane that shrinks a listed function no longer fails
- the predicate-tree refusal names the undeclared inputs
- a claude pane is told its authority in its system prompt
- an advisor plan runs a trigger-gated phase only when its trigger fires
- the sandbox wiring pin names baseRequest; a runner test proves every rung
- an exhausted walk that met a quota wall defers
- the universal-fallback tail holds only CLIs with tool use
- the auditor persona states the rules the code applies
- the debugger of a fleet-rebase conflict writes its resolution
- the identity carry reads the base the audited worktree stood on
- drop the dead module token from the artifact-bytes note (#682)
- a cycle the debugger ends is a diagnosed FAIL, so its claim is released
- the no-work check asks the lane menu triage offers
- a launch of nothing is not a wave, and it counts as starved
- every planner reader and every lane menu asks the one rule
- a cycle's own record on a non-material diff is its explanation, not history
- a capacity deferral in the sequential loop is nobody's failed approach
- a top_n card on a protected surface is routed by the host, never the cycle's FAIL
- an inbox lifecycle record is non-material to the explanation document
- the planned no-work closeout retires every scoped id no inbox item backs
- the PASS seam retires a committed id Promote could not move and no inbox item backs
- an empty optional bucket reads as no cards; an empty top_n still declines
- a Build may retract its own explanation draft; the doc guard closes older holes (cycle 1705) (#651)
- a worktree ship binds its own tree, not the plane's bookkeeping (#648)
- the host performs declared effects before every judge (F36) (#637)
- clear the two production golangci findings that block console commits (#635)
- triage re-checks a queued item's premise against what changed since it was filed, and a stale drop never retires work (F40)
- a fleet lane whose triage answers for its scoped item ends as planned no-work and hands the item to the console (F30)
- the idle nudge says why a deliverable that is already right must be re-checked and rewritten (F39 part 1)
- a dead agent pane gets one fresh session of the same CLI before the fallback chain moves on (F31)
- the build handoff floor refuses a protected control-plane edit, and a ship refusal for one goes back to build (F37) (#631)
- no route override or line locator lets a declared protected file reach a lane; the wave plan prunes console-routed ids before it widens (F34, F35) (#629)
- the wave seed refuses every stated surface triage's breaker would, so lanes stop drawing doomed work (F29) (#628)
- a pipeline-* kind routes the item to the console — the kind is a first-class routing input (F25) (#626)

### Changed

- the path selection is its own package, the one source of which paths Ship commits
- core group 9, phases/ship group 1 and phases/audit group 1 explain themselves (batches 64–66)
- core, cmd and bridge groups 8, 8 and 7 explain themselves (batches 61–63)
- core, cmd and bridge groups 7, 7 and 6 explain themselves (batches 58–60)
- core, cmd and bridge groups 6, 6 and 5 explain themselves (batches 55–57)
- 119 files explain themselves; core, cmd and bridge groups 5, 5 and 4 (batches 52–54)
- 119 files explain themselves; core, cmd and bridge groups 4, 4 and 3
- 108 files explain themselves; core, cmd and bridge groups 3, 3 and 2
- 32 packages explain themselves; their knowledge moves to design notes (#640)

### Documentation

- §5.13 the size ratchet is a ceiling; G2 built
- the review of waves 11-20 and the dispatchability rule
- B5 shipped — the debugger's resolution rejoins the rebase; 1719's route recorded
- F7 shipped — the importer backstop's alone re-run; F7b designed
- §5.9 the wave-18 deep-dive — the cycle's own record is not history (X3)
- Q3 verified and closed; the sequential dispatcher's ordering was the one gap
- §5.8 the carry shipped (C1–C4); ADR-0105 B3/B4 component tables and rollout rows
- §5.7 a card on a protected surface is a route, not a verdict (1714); §7.8 R table; signatures; evidence
- §5.6 the wave-15 deep-dive; W, X series; 1712 evidence row; signatures; review log
- self-recovering agent loops and Claude Code fleets in 2026 — what evolve-loop can learn (#658)
- logic-first delivery — statuses after #652, #655 and #656 merged (#657)
- logic-first delivery — the policy, ADR-0106 and the design document (#655)

### Other

- Merge remote-tracking branch 'origin/main'
- Merge pull request #695 from mickeyyaya/fix/sizeratchet-is-a-ceiling
- Merge remote-tracking branch 'origin/main' into fix/sizeratchet-is-a-ceiling
- Merge pull request #694 from mickeyyaya/train/2026-09-28-wave26
- (dossier) cycle-1735 closeout
- Merge remote-tracking branch 'origin/feat/treefence-takes-the-shipped-tree' into train/2026-09-28-wave26
- Merge remote-tracking branch 'origin/refactor/shipmanifest-single-source' into train/2026-09-28-wave26
- Merge remote-tracking branch 'origin/fix/predicate-refusal-names-its-paths' into train/2026-09-28-wave26
- Merge remote-tracking branch 'origin/chore/inbox-reweigh-gate-forced-fail' into train/2026-09-28-wave26
- Merge remote-tracking branch 'origin/fix/pane-identity-is-system-level' into train/2026-09-28-wave26
- (dossier) cycle-1736 closeout
- evolve-cycle: goal=e9d61da905a953d05cc1c792e44e0e17b6843df01497b1ee30b003bc9396a2a2
- Merge pull request #688 from mickeyyaya/fix/plan-clamps-to-insert-when
- Merge pull request #687 from mickeyyaya/fix/chain-plans-only-capable-drivers
- Merge remote-tracking branch 'origin/main' into fix/chain-plans-only-capable-drivers
- Merge pull request #686 from mickeyyaya/fix/auditor-persona-states-the-code-rules
- Merge remote-tracking branch 'origin/main' into fix/auditor-persona-states-the-code-rules
- Merge pull request #685 from mickeyyaya/feat/comment-floor-shadow
- (dossier) cycle-1733 closeout
- (dossier) cycle-1734 closeout
- Merge origin/feat/comment-floor-shadow (the update-branch merge)
- Merge origin/main (#684) into feat/comment-floor-shadow
- Merge pull request #684 from mickeyyaya/fix/debugger-writes-its-rebase-resolution
- Merge branch 'main' into feat/comment-floor-shadow
- Merge branch 'main' into fix/debugger-writes-its-rebase-resolution
- (dossier) cycle-1731 closeout
- evolve-cycle: goal=1dae5c624d0a1adb6b1fc5127e13464cbe22bcd6b288f7fbf72f5440288327de
- (dossier) cycle-1732 closeout
- Merge pull request #683 from mickeyyaya/fix/carry-reads-the-audited-worktree-base
- Merge origin/main (#672, #674, #678) into fix/carry-reads-the-audited-worktree-base
- Merge pull request #678 from mickeyyaya/feat/three-book-review
- Merge pull request #674 from mickeyyaya/chore/comment-reduction-10
- Merge pull request #672 from mickeyyaya/chore/comment-reduction-9
- Merge #674 (comment reduction round 7) into fix/carry-reads-the-audited-worktree-base
- (dossier) cycle-1730 closeout
- Merge branch 'main' into feat/three-book-review
- Merge branch 'main' into chore/comment-reduction-10
- Merge branch 'main' into chore/comment-reduction-9
- evolve-cycle: goal=b8ae0b59639bf033ddafefb0f31d8acf1e25ee4a523e54f0e737a99973e914c2
- (dossier) cycle-1729 closeout
- Merge origin/main into feat/three-book-review
- Merge remote-tracking branch 'origin/chore/comment-reduction-9' into chore/comment-reduction-10
- Merge remote-tracking branch 'origin/main' into chore/comment-reduction-10
- Merge remote-tracking branch 'origin/main' into chore/comment-reduction-9
- Merge pull request #680 from mickeyyaya/fix/ship-recovery-end-is-fail
- (dossier) cycle-1727 closeout
- evolve-cycle: goal=7652e2ab4439fe92e9651e4c97116152dd1baf1bcb7196c10267032732209b7d
- (dossier) cycle-1728 closeout
- (dossier) cycle-1725 closeout
- (dossier) cycle-1726 closeout
- (dossier) cycle-1724 closeout
- (dossier) cycle-1723 closeout
- (dossier) cycle-1722 closeout
- (dossier) cycle-1721 closeout
- Merge pull request #676 from mickeyyaya/fix/wave-plan-fresh-claim-w3
- Merge pull request #675 from mickeyyaya/fix/debugger-rebase-reentry-b5
- Merge pull request #673 from mickeyyaya/fix/backstop-alone-rerun-f7
- (dossier) cycle-1720 closeout
- evolve-cycle: goal=0b9fed8e7598f16df531fdd5b6bff8a91bfc477709a0afa8905b8a205ab0900b
- Merge pull request #670 from mickeyyaya/fix/explanation-own-record-x3
- Merge pull request #669 from mickeyyaya/chore/comment-reduction-8
- Merge pull request #668 from mickeyyaya/feat/explanation-review-recovery-x2
- (dossier) cycle-1719 closeout
- evolve-cycle: goal=e3b24e5f9090375191712f13e49949e8ba55b399816e9cfc915cbb1b311b6c4c
- (dossier) cycle-1718 closeout
- Merge pull request #667 from mickeyyaya/chore/comment-reduction-7
- Merge pull request #666 from mickeyyaya/feat/identity-carry-b3-b4
- (dossier) cycle-1717 closeout
- evolve-cycle: goal=bb523c5401ec52fed8214f1fb2bd5603f5a852490ed4b01217398e29e1c41254
- (dossier) cycle-1716 closeout
- Merge pull request #665 from mickeyyaya/fix/triage-protected-card-routes
- Merge pull request #664 from mickeyyaya/chore/comment-reduction-6
- (dossier) cycle-1715 closeout
- evolve-cycle: goal=dccd8d3f31dce31d8cc6722b3906d52e74108b0a524c0ea5cfc39b452b1152dc
- (dossier) cycle-1714 closeout
- Merge pull request #663 from mickeyyaya/fix/retire-carried-id-on-empty-commitment
- Merge pull request #662 from mickeyyaya/chore/comment-reduction-5
- (dossier) cycle-1712 closeout
- evolve-cycle: goal=846314e480184e320c289c7a799170f1aee572401834e3e25c28c806ef015b67
- (dossier) cycle-1713 closeout
- ADR-0106 P3, Q1, Q2: the agent's identity in its prompt, panes without suggestions, capacity walls as deferrals, credential walls benched (#661)
- (dossier) cycle-1708 closeout
- ADR-0106: exit-85 causes, the host-derived triage decision, and the recovery rung's scaffolding (#656)
- (dossier) cycle-1707 closeout
- (dossier) cycle-1706 closeout
- evolve-cycle: goal=307bc0b1afa7f56bd00db4cfeb90c3c7f50370b1d0a3fbca6d9c6b3644f3a5f3
- (dossier) cycle-1704 closeout
- evolve-cycle: goal=dc249d316f2dc4099f8cc7947061a2c84785fee95c1d5166b3f95e55e1097374
- (dossier) cycle-1705 closeout
- (dossier) cycle-1702 closeout
- evolve-cycle: goal=2dfa8e45ff31454adcc62f7ca851de9b59f484e964d071cad455d9d0a0418497
- (dossier) cycle-1703 closeout
- (train) routing sandbox floor, cycle-state override scope, guards fixes, comment batches 33–45 (#647)
- (dossier) cycle-1701 closeout
- evolve-cycle: goal=479b4ed9bea919d663bf99d4a81ce2ad66f41626d3a1ffb3ec36c9ab0dfb5092
- (dossier) cycle-1700 closeout
- (dossier) cycle-1698 closeout
- (dossier) cycle-1699 closeout
- (dossier) cycle-1697 closeout
- (dossier) cycle-1696 closeout
- evolve-cycle: goal=d01cf6f55243c2744e554cd4ac68622b52c928d820c5f78b1fe699e0942c60fc
- (dossier) cycle-1694 closeout
- (dossier) cycle-1695 closeout
- (dossier) cycle-1692 closeout
- evolve-cycle: goal=e51a1a498492896c8ab329adb0ce76cf6be70a1f481e2f4d6f1bf3e468f5b0d8
- (dossier) cycle-1693 closeout
- (dossier) cycle-1691 closeout
- (dossier) cycle-1690 closeout
- (dossier) cycle-1687 closeout
- (dossier) cycle-1688 closeout


---

## [22.24.0] - 2026-09-15

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.23.0..HEAD._

### Added

- every LLM launch walks the fallback chain, and every chain ends with every available CLI (ADR-0104) (#619)
- every cycle carries its phase plan — required, passed, ongoing, remaining — on the live board (#613)
- ADR-0101 S2b — the contract gate's deliverables check reports through the Signal Center; the prompt states the gate's criteria (#585)
- ADR-0101 S4a — loop producers, the ledger's append observer, the batch report reads the runner's SignalSummary (#582)
- ADR-0101 S3 — LivenessCenter rename; Deps.Signals at construction; bridge.warning/tripwire and pane.liveness producers (#581)
- ADR-0101 S2a — cycle.sealed, system.failure, ship.error and quota.paused producers; Flush; generated signal-codes.md (#580)
- ADR-0101 S1 — one Signal Center; the orchestrator listens, the C1 chokepoint is the first producer (#579)
- the declared effect inbox-claim is verified at the triage boundary (ADR-0100 slice 2) (#575)
- every agent-owed declared output is verified at the phase boundary (ADR-0100) (#574)

### Fixed

- the verdict engine verifies through the contract gate's own Reviewer, so a salvaged verdict is classified as the gate approves it (F22) (#625)
- cycle 1679's added-test predicate verifies its recorded set on a merged tree (F21) (#624)
- the interrupt wins over the pre-wave probes (F20) (#623)
- a failure class outside the vocabulary is refused at the gate, the repair decision is a coded signal, and a retro-routed retry carries the audit's standing findings (F19) (#622)
- a shipped inbox item is never re-planned, a sealed lane never blocks the boundary refresh, a template slot never passes the build floor, and a recovery rebuild carries the standing audit findings (F14–F18) (#621)
- a red added test is found at the build floor, a graderless eval is refused at the scout, and a rejected push still journals its commit (#620)
- the CI-parity apicover step runs its coverage go test under the scrubbed CI env (#617)
- two walls the auto-responder did not recognise — codex's rejected model and Claude's session limit (#616)
- the repo-contract gate hands go test CI's environment, not the lane's (#615)
- the router proposal is an artifact contract — read routing-proposal.json, never the scrollback (#614)
- the repo-contract gate tests the lane worktree, seeds from the working tree, and runs the importers of what a lane changed (#612)
- a cycle never mutates the operator's tree — the --simulate walk reads the root in place and every worktree mutator refuses it (#611)
- one prompt-delivery tail — settle by paste size, wait for a stable pane, back off re-sends (the codex submit_wedged wave) (#609)
- a triage refusal is a coded, task-level failure; a protected-surface refusal routes the item to console on the first hit (#606)
- ADR-0102 — the explanation review's reasoning is the gate, its format is advisory (#583)
- a phase's own FAIL reason rides the C1 record — the seal names it instead of "phase-infra class" (#577)
- SHIPPED_VIA_BUILD requires this cycle's own ship, never a HEAD delta (ADR-0100 PR-3) (#576)
- the phase subprocess sees the plane's root, and triage can actually move its claim (#573)
- honor profile sandbox.write_subpaths — the declared write surface is the granted one (#572)
- a resumed cycle disposes of its worktree at exit, by the same rule the fresh path applies (#571)
- the direct ship path verifies its audit binding, like the worktree path does (#569)
- resume records its terminal outcomes, like the fresh cycle path does (#568)
- protect the explanation-lifecycle call sites #549 relocated (#565)
- preserve ACS cycle identities
- bind contracts to artifacts
- require executed acceptance evidence
- preserve integration history on interruption
- resume interrupted active phase
- require tracked commit-prefix anchors

### Changed

- ADR-0103 unit 16 — Subagent run (`internal/subagent/subagentrun`) (internal/subagent/subagentrun) (#601)
- ADR-0103 unit 14 — CI-parity gate (`internal/phases/audit/ciparitygate`) (internal/phases/audit/ciparitygate) (#600)
- ADR-0103 unit 13 — Loop wave/chain engine (`internal/loopwave` + `internal/loopchain`) (internal/loopwave) (#599)
- ADR-0103 unit 04 — Phase advisor (`internal/core/advisor`) (internal/core/advisor) (#598)
- ADR-0103 unit 11 — Phase-runner verdict engine (`internal/phases/runner/verdict`) (internal/phases/runner/verdict) (#597)
- ADR-0103 unit 10 — Bridge engine: the launch-outcome classifier (`internal/bridge/launchoutcome`) (internal/bridge/launchoutcome) (#596)
- ADR-0103 unit 06 — Inbox lifecycle mover (`internal/inboxmover/lifecycle`) (internal/inboxmover/lifecycle) (#594)
- ADR-0103 unit 09 — Defect ledger (`internal/core/defectledger`) (internal/core/defectledger) (#593)
- ADR-0103 unit 12 — the phase-observer engine as its own leaf (internal/observerengine) (#592)
- ADR-0103 unit 08 — config resolution decomposed in place around one Loader (internal/config) (#591)
- ADR-0103 unit 07 — the ship landing as its own unit (internal/phases/ship/landing) (#589)
- ADR-0103 unit 03b — the failure-learning engine as its own unit (internal/core/failurelearning) (#588)
- ADR-0103 unit 03 — the carryover-todo lifecycle as its own unit (internal/core/carryover) (#587)
- ADR-0103 unit 02 — failure diagnostics and the delivery-failure classifier as their own unit (internal/core/failurediag) (#586)
- ADR-0103 unit 01 — the phase-outcome recorder as its own unit (internal/core/outcome) (#584)
- run the release as journaled pre-publish, ship and post-publish stages (#567)
- run preflight as ordered stages over a resolved seam set (#566)
- isolate plan stage mappings
- centralize model attempt performance
- centralize terminal timeout diagnostics
- isolate checkpoint disposition
- isolate checkpoint review
- isolate tmux tick interactions
- extract tmux prompt admission
- isolate tmux wait state
- decompose cycle execution with no-work closeout
- decompose tmux bridge lifecycle
- isolate ship worktree transaction
- separate phase execution and audit stages
- split policy facade by domain

### Documentation

- wave-2 record — 0 ships of 2 lanes, four pipeline defects found and fixed, wave 3 launched (#618)
- record test refactoring validation and duplicate decisions
- design coverage-preserving test and harness refactoring
- ADR-0103 program review — feasibility, pipeline efficiency and triage accuracy across the sixteen unit and program docs (#595)
- ADR-0101 Signal Center — design, inventory, migration slices (campaign S0) (#578)
- record tmux wait state and admission batch
- document component decomposition and validation

### Other

- (dossier) cycle-1685 closeout
- evolve-cycle: goal=ca41c09f02b7f474511428845fa0469f885dcc71da649fb964286b17c25ee5d5
- (dossier) cycle-1686 closeout
- (dossier) cycle-1684 closeout
- evolve-cycle: goal=4a608afc3cb02e938171d622eea36bcaf2db5104b3d3fc68dde5d252e2c59c82
- (dossier) cycle-1682 closeout
- (dossier) cycle-1683 closeout
- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-1679 closeout
- evolve-cycle: goal=becae0934c3cae79ec10b72586baf51b76238c1be9d04497e02e32a0b5e8a396
- (dossier) cycle-1676 closeout
- (dossier) cycle-1677 closeout
- (dossier) cycle-1673 closeout
- (dossier) cycle-1674 closeout
- evolve-cycle: goal=989725b2598422715cefe5bf7c8d59a473e1d0e85e842c76662940913ea192cb
- Merge pull request #610 from mickeyyaya/refactor/test-refactoring-validation-2026-09-14
- Merge pull request #607 from mickeyyaya/fix/test-gates-ci-2026-09-14
- Merge pull request #605 from mickeyyaya/refactor/test-release-build-isolation-2026-09-14
- Merge pull request #604 from mickeyyaya/refactor/test-fixtures-2026-09-14
- Merge pull request #603 from mickeyyaya/refactor/test-structure-checker-2026-09-14
- Merge pull request #602 from mickeyyaya/docs/test-architecture-design-2026-09-14
- (dossier) cycle-1675 closeout
- (dossier) cycle-1672 closeout
- (dossier) cycle-1670 closeout
- (dossier) cycle-1668 closeout
- (dossier) cycle-1667 closeout
- (dossier) cycle-1669 closeout
- (dossier) cycle-1666 closeout
- (dossier) cycle-1665 closeout
- (dossier) cycle-1664 closeout
- (dossier) cycle-1663 closeout
- (dossier) cycle-1661 closeout
- (dossier) cycle-1662 closeout
- (dossier) cycle-1659 closeout
- (dossier) cycle-1660 closeout
- (dossier) cycle-1658 closeout
- (dossier) cycle-1657 closeout
- (dossier) cycle-1655 closeout
- (dossier) cycle-1656 closeout
- (dossier) cycle-1652 closeout
- (dossier) cycle-1654 closeout
- (dossier) cycle-1653 closeout
- (dossier) cycle-1651 closeout
- (dossier) cycle-1650 closeout
- (dossier) cycle-1649 closeout
- (dossier) cycle-1647 closeout
- (dossier) cycle-1648 closeout
- (dossier) cycle-1646 closeout
- (dossier) cycle-1628 closeout
- (dossier) cycle-1627 closeout
- (dossier) cycle-1644 closeout
- (dossier) cycle-1640 closeout
- (dossier) cycle-1641 closeout
- (dossier) cycle-1642 closeout
- (dossier) cycle-1638 closeout
- (dossier) cycle-1639 closeout
- (dossier) cycle-1637 closeout
- (dossier) cycle-1636 closeout
- (dossier) cycle-1635 closeout
- (dossier) cycle-1632 closeout
- (dossier) cycle-1633 closeout
- (dossier) cycle-1634 closeout
- (dossier) cycle-1629 closeout
- (dossier) cycle-1630 closeout
- (dossier) cycle-1631 closeout
- (dossier) cycle-1624 closeout
- (dossier) cycle-1625 closeout
- Merge pull request #564 from mickeyyaya/fix/acs-cycle-allocation-floor
- Merge pull request #563 from mickeyyaya/fix/router-artifact-contract
- Merge pull request #562 from mickeyyaya/fix/eval-execution-evidence
- Merge pull request #561 from mickeyyaya/refactor/plan-stage-component
- Merge pull request #560 from mickeyyaya/fix/interrupted-closeout-resume
- Merge pull request #559 from mickeyyaya/fix/signal-active-phase-checkpoint
- Merge pull request #558 from mickeyyaya/refactor/model-attempt-telemetry
- Merge pull request #557 from mickeyyaya/refactor/tmux-wait-terminal-diagnostics
- Merge pull request #556 from mickeyyaya/refactor/tmux-wait-checkpoint-disposition
- Merge pull request #554 from mickeyyaya/refactor/tmux-wait-checkpoint-evidence
- Merge pull request #555 from mickeyyaya/test/observer-activity-determinism
- Merge pull request #553 from mickeyyaya/refactor/tmux-wait-tick-interaction
- Merge pull request #550 from mickeyyaya/refactor/tmux-wait-state-machine
- Merge branch 'main' into refactor/tmux-wait-state-machine
- Merge pull request #552 from mickeyyaya/test/pool-backfill-determinism
- Merge pull request #549 from mickeyyaya/refactor/component-decomposition


---

## [22.23.0] - 2026-09-11

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.22.0..HEAD._

### Added

- ADR-0099 slice 3 — solution-skill personas on document cycles via a signal-keyed overlay rule (#544)
- ADR-0099 slice 2 — the document deliverable contract behind ONE deterministic engine (build floor, evolve solution check, audit gate) (#543)
- ADR-0099 slice 1 — kernel reads goal_type/deliverable_kind from report headers; tdd released for document cycles by config; solution recipes (#539)
- deep/top tier → gpt-6-astra at high reasoning; one tier ladder, one family table (model_tier_map_from) (#538)
- Task Contract block — acceptance verbatim + the ACS predicate inventory in the tdd, build and audit prompts (ADR-0098) (#534)
- `evolve dashboard` — read-only live pipeline view (ADR-0095) + ship-rate investigation; single-home the auditor verdict/finding grammar in reportdoc (#530)

### Fixed

- the cycle record is a projection of evidence, not a synthesized placeholder (#548)
- deep/top back to gpt-5.6-sol — the gpt-6-astra cutover is withdrawn for token cost (2026-09-10 operator directive) (#547)
- salvage cycle 1620's multi-slug lane scope reconciliation with its audit findings repaired (#546)
- token-waste repairs — cancellation reaps its providers, known persona absences cost no LLM, lanes base on the integration HEAD (#545)
- preserve audit repair trees and admitted phase skips (#542)
- preserve complete tmux prompt delivery (#541)
- permit TDD worktree eval authoring (#540)
- recover live quota walls and continuation adoption (#537)
- use measured sandbox capability for confined CLI launches (#536)
- restore audit authority, resume lifecycle and runtime contracts (#535)
- a missing "## Explanation Documentation" section is a correction, not a terminal FAIL (#533)
- fence read-only phases' worktree; read the explanation review as auditors write it (ADR-0097) (#532)
- repair rounds escalate tier + effort and carry the auditor's findings (ADR-0096) (#531)
- cycle-1603 verdict-incoherence — audit re-dispatch retires superseded round verdicts; diagnosed downgrades never halt as forgery (#529)
- claude 2.1.252 flipped the folder-trust default to "No, exit" — new auto-respond rule, live-verified (#527)
- nested launches WARN with the outer-confinement doctrine — align the #518 host-capabilities check with the gate it fronts (#525)

### Other

- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-1623 closeout
- evolve-cycle: goal=14d8eae2
- (dossier) cycle-1622 closeout
- (dossier) cycle-1616 closeout
- (dossier) cycle-1605 closeout
- (dossier) cycle-1604 closeout
- (dossier) cycle-1603 closeout
- (dossier) cycle-1602 closeout
- (dossier) cycle-1601 closeout
- (ops) rebalance claude/codex across all phase agents — claude keeps only floor-justified phases; 24 profiles move to codex (#528)
- (ops) codex deep/top (gpt-5.6-sol) max -> high — 2026-09-01 operator directive, one edit through the #512 seam (#526)


---

## [22.22.0] - 2026-09-01

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.21.0..HEAD._

### Added

- build-explanation deliverables — #517 completed (five stacked e2e defects fixed red-first) (#518)

### Fixed

- single-home activation belt — audit's gate can no longer be silently disabled by a zero ContractVersion (#524)
- un-fossilize the integration-tier exclusions — core/cmd/ship rejoin the lane gate; the retake is the contention answer (#523)
- main-red — align the audit FAIL/WARN cache-put pins with cycle-1594's declared-content contract (#519)

### Documentation

- workspace-layout.md — the hub layout (bare store + console/runtime planes + dev/ worktrees), CLAUDE.md pointer (#522)
- architecture-reviewer agent profile — third member of the pre-commit review fleet (#520)

### Other

- evolve-cycle: goal=4d09143b44e95b79d482354e4714a979ecd6730f1869fecad5a1ddab80f0566e
- (dossier) cycle-1589 closeout
- (dossier) cycle-1590 closeout
- (dossier) cycle-1591 closeout


---

## [22.21.0] - 2026-08-31

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.20.1..HEAD._

### Added

- a transient upstream error short-circuits the silence budget — salvaged from cycle-1588 and finished (60s dwell -> ReviewStop -> exit 81) (#516)
- the refresh discovers each CLI's reasoning-effort ladder, not just its models (#511)
- an audit FAIL is repairable in-cycle — verdict reasoning feeds tdd+build, bounded at 2 (ADR-0092) (#507)

### Fixed

- un-track the minted production-path-wiring-proof profile AND close the class — the ignore list gains the entry it was missing
- a fleet-written checkpoint is now findable by a host-global resume — quota pauses no longer abandon completed work (#514)
- route the effort guards through RealTreeProfiles — the raw scan red the live plane on runtime-minted stubs; plus a usage-query reader behind the CLI-control abstraction (#513)
- agy tier models are names agy ACCEPTS — enumerate via `agy models`, not a picker that cannot supply them (#510)
- ADR-0072's policy table becomes the ONE retry authority — an audit FAIL retries inside its own cycle, retro goes terminal (ADR-0093) (#508)
- codex plan mode inherits the phase's reasoning tier — and stop the realizer silently eating a repeated flag (#506)
- auto-respond to plan-mode dialogs, and stop a manifest regex from dying silently (#505)
- close 4 defects an architect review found in #503 (#504)
- audit-binding fails CLOSED — FAIL verdicts are ledger-bound and cross-run fallbacks refused (cycle-1571 H3) (#503)
- persona-less mints are stamped catalog:"on-demand" at the construction seam — instance #5 of the class, the cycle-1567/1568 halt (predicted: the #493 architect's deferred suggestion 4b, landed after the prediction came true). A minted phase runs its own dispatch on the inline PromptBody, but the persisted phase.json outlives the process: cycle-1568's advisor minted production-path-wiring-proof with no persona and no catalog word, the tracked-path stub reached lane worktrees via the shared root, and the #493 tracked-menu guard CORRECTLY redded internal/core in the lane (main green at the identical HEAD) — a defense success escalated as infra-systemic (the canned forged-verdict hint wrong for the FIFTH consecutive escalation) — while 1568's plan already scheduled the phase for a next-cycle dispatch that dies at load-agent (now an optional-skip via #493's fail-soft, but still a wasted slot). The stamp lives in the registrar's existing mint-time defaulting block (the SELECT-metadata precedent: contract-safe by construction, not by advisor diligence): persona unresolvable via the same prompts loader dispatch uses ⇒ catalog:on-demand; a resolvable persona or an explicit catalog word is kept verbatim. Pins: persisted-on-disk stamp (the file outlives the process), resolvable-mint stays on menu, explicit word verbatim; 2/3 mutants killed + 1 documented equivalent (re-stamping on-demand with on-demand — the only catalog word). Full bar green. Live remediation: the root stub was already reaped by the loop's own GC; 1567's lane copy left untouched (live worktree, cycle failing on its own audit). (#502)

### Other

- (dossier) cycle-1587 closeout
- (dossier) cycle-1586 closeout
- (dossier) cycle-1588 closeout
- (dossier) cycle-1583 closeout
- (dossier) cycle-1584 closeout
- (dossier) cycle-1585 closeout
- evolve-cycle: goal=4d09143b44e95b79d482354e4714a979ecd6730f1869fecad5a1ddab80f0566e
- (dossier) cycle-1581 closeout
- (dossier) cycle-1582 closeout
- (dossier) cycle-1580 closeout
- (dossier) cycle-1578 closeout
- (dossier) cycle-1579 closeout
- (ops) pin the codex deep/top rung as a RULE — a profile added mid-flight missed the sweep, and `go test` could not see it (#512)
- (ops) codex deep/top phases run at max — the rung above xhigh that the manifest never mapped (operator directive 2026-08-28) (#509)
- (dossier) cycle-1577 closeout
- (dossier) cycle-1576 closeout
- (dossier) cycle-1575 closeout
- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-1573 closeout
- (dossier) cycle-1574 closeout
- (dossier) cycle-1572 closeout
- (dossier) cycle-1570 closeout
- (dossier) cycle-1571 closeout
- (dossier) cycle-1569 closeout
- (dossier) cycle-1568 closeout
- evolve-cycle: goal=92a72934f34b57b132a1bcf48865e1a28496ab62e8bef597cdc9fe12d4a11a4f
- (dossier) cycle-1567 closeout
- (dossier) cycle-1566 closeout
- (ops) deep-tier family arrangement — 88% of deep/top task types to codex gpt-5.6-sol (#501)


---

## [22.20.1] - 2026-08-26

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.20.0..HEAD._

### Fixed

- submit-verify false-wedge reds the v22.20.0 release — ground-truth belt + harness fidelity. The wave-landed delivery-failure fast-fail classifies "prompt still parked" from pane echo heuristics and short-circuits submit_wedged → instant exit 81 at the prompt site; a REPL that consumes its input SILENTLY and never redraws the input line is indistinguishable from parked — exactly the RealTmux fakes (side-effect answers, no output), so the whole family 81'd deterministically on the release commit (go + release workflows red; v22.20.0 published assetless). The landing lanes' local bars quiet-host-skipped RealTmux and nobody watched post-push CI mid-wave — main was red from the wave-4 ship onward. Fix: (1) GROUND-TRUTH BELT at the wedged short-circuit — one read-only Lstat probe (never-follow-symlinks parity with regularFileNonEmpty, cycle-1256 D3): an artifact present that is NOT the pre-dispatch baseline (#498's key) proves the submission landed → fall through to the normal wait (stability window still gates); a parked pane with no deliverable, or with only the prior attempt's stale leftover, keeps the fast-fail. Nudge-site needs no belt (its wedged break lands after the same tick's real detector poll — reviewer-verified). Telemetry note: submit_wedged in the interactions ledger can now co-occur with a successful phase; consumers gate on the phase error (failure_learning.go does). (2) HARNESS FIDELITY: both RealTmux fakes re-print their input-line marker after every consumed line (real-REPL discipline) with a no-resend pin on the happy path so the CLEAN submit-verify path stays exercised rather than belt-shadowed. go-reviewer APPROVE (MEDIUM Lstat parity + LOW telemetry comment, both applied; independently reproduced a mutation kill and the RealTmux 15/15 green). Pins: parked+delivered completes OK loudly, parked+stale-only still 81s (real CaptureBaseline), happy-path zero resends; 4/4 mutants killed. Incident doc + REGRESSION row. v22.20.0 stands assetless per fix-forward doctrine; v22.20.1 folds it forward. (#500)


---

## [22.20.0] - 2026-08-25

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.19.0..HEAD._

### Added

- make submit-verify outcomes durable (interaction ledger) (#475)

### Fixed

- manual consume leaves the continuation binding immortal — release + guarded reconcile (cycle-1558) (#499)
- stale pre-dispatch artifact certified as completion — baseline gate at BOTH doors (cycle-1550 family; red test salvaged from the 1554 lane's continuation snapshot). Door 1, the completion detector: artifactDetector's stability window started on FIRST SIGHT, so a correction re-dispatch whose prior failed attempt left its report at the canonical path watched the untouched file "stay stable" two ticks and completed — the stale verdict re-graded itself every retry, and the finality concession compounded it. Fix: a PRE-DISPATCH baseline over the WHOLE artifactCandidatePaths set (canonical + fallbacks, single-sourced with artifactLocate — a shadowed fallback stray must not launder through the side door when the canonical vanishes mid-session; design-review note 1), captured in runTmuxREPL BEFORE prompt delivery via Deps.CaptureBaseline (nil→real, withDefaults-wired and tripwire-pinned); baseline-identical observations never start a window and never complete, finality included; size-only delta counts (coarse mtime); capture errors fail open. Door 2 (go-review CRITICAL): the runner's reconcile-on-teardown independently re-reads the same canonical artifact after ErrArtifactTimeout and trusts any well-formed deliverable — and a same-cycle leftover carries the once-per-cycle challenge token, so both the well-formed fall-through AND the ACS deterministic floor would resurrect the stale verdict five minutes later. The runner now takes its own pre-dispatch snapshot (same size+mtime key) and refuses BOTH reconcile doors for byte-identical leftovers, refusal cause in the FAIL diagnostics; the cycle-254/255 contract is untouched (rewritten-during-session still reconciles, pinned). Accepted costs documented: an after-buzzer finisher's complete artifact is redone next attempt; a stale canonical shadowing fresh fallback-only work times out (strictly better than certifying the stale file). Test harnesses whose fakes cannot write files declare their pre-seeded artifacts as mid-session writes via zeroBaselineCapture (warning comments on both shared runners: stale scenarios must inject the real capture). Reviews: design APPROVE-WITH-NOTES (note 1 fixed in-PR, note 2 documented, note 3 comments); go-reviewer REQUEST-CHANGES resolved via their recommended (a)+(b) — runner-side gate + cross-package regressions. TDD: salvaged red test + 11 more pins incl. full-engine LaunchArgs replay and the ACS-floor-stale refusal; 12 mutants across both layers, all killed. Incident doc + REGRESSION index row. Retires pipeline-defect-pipeline-blocker with this landing. (#498)
- triage bookkeeping defeats in-commit consumption — per-id lane-scope union (cycle-1552) (#496)
- anchor-order-sensitive splice mis-slots planned phases — fixpoint placement (cycle-1550, soak-20260824a). ApplyUserRouting placed specs in alphabetical discovery order, so a spec anchored (after:) to an alphabetically-LATER batch-mate missed its not-yet-spliced anchor and silently took the before-audit fallback: bug-reproduction (after: fault-localization) executed EIGHTH, post-build, and its red-first reproduction test could never be greened — audit correctly FAILed the lane (the auto-escalation's 'forged verdict' hypothesis was wrong; the audit was right). FOUR tracked anchors were inverted (bug-reproduction→fault-localization, cleanup-sweep→mutation-gate, benchmark-gate→perf-profile, mutation-gate→test-amplification — incl. a live 2-deep chain). Fix: placement is a FIXPOINT over the batch — anchored specs wait for their anchor across passes; when a pass strands, truly-absent anchors force-place LOUDLY, transitively-blocked specs are HELD so they still land after their anchor, and an anchor deadlock force-places exactly one dependency target so cycle members and tails alike resolve honorably; already-present overlays stay silent. Empty-anchor and anchor-present behavior byte-identical. Reviews applied: go-reviewer APPROVE (its MEDIUM — batch-order burst inverting declared anchors for transitively-blocked specs — fixed with a pinned test); design review APPROVE-WITH-NOTES (cycle-tail over-approximation fixed + pinned; already-present false warning fixed + pinned; four-victim doc count; insert-cap behavior delta noted below); simplifier both WORTH-ITs applied (slices.Index, doc contract). Soak-watch note: pre-build repro/fault-localization now consume the optional-insert budget earlier, so cap-tight cycles may clamp a post-build scan — correct bugfix priority, not a regression. TDD red-first (both live victims red on unfixed code via the production-seam realtree test); 7 mutants killed. Incident doc + REGRESSION index row; follow-ups (warning→ledger promotion, resume-path order single-sourcing + degradeInsertedSuccessor evaluate mismatch, mint-batch routing, pre-build-Evaluate floor vocabulary) queued as routing-order-authority-followups 0.8. (#494)
- persona-less menu phase kills a lane — three-layer containment (cycle-1551, soak-20260824a). (1) Registration seam: discoverUserSpecsClamped demotes any discovered spec whose persona doc does not load to catalog:"on-demand" in memory (demotePersonalessSpecs; covers tracked+untracked, any host — the user-phase counterpart of registerBuiltinSpecRunners' persona check). (2) Dispatch fail-soft: runner wraps a genuinely-absent persona in core.ErrAgentDocMissing; optionalInfraSkip admits it via the single-source IsOptionalSkippableError — OPTIONAL phases degrade to a recorded WARN (own ledger kind optional_missing_persona_skip + diagnostic on the synthesized response; zero retries, no infra event, so forensics never merges the classes), mandatory/floor still fail loud. (3) Repo guard: TestRepoPhaseCatalog_MenuPhasesResolveAPersona reds tracked menu phases without agents/<agent>.md; the two on-menu offenders (defect-disposition-preflight, pre-audit-evidence-check) demoted in phase.json. Hardening: prompts zero-loader carries ErrNoSource beside its documented fs.ErrNotExist and the runner declines the missing-doc classification for it (misresolved prompts root stays LOUD, never a mass silent skip). Reviews applied: architect M1 (equivalence invariant retargeted to the widened single-source predicate; ACS bindings cycle1166/cycle1270 repointed), M2 (phase-local agent.md false-GREEN branch dropped — nothing reads it), M3 (godoc), S1-S4 (diagnostics, message split, ledger kind, floor/mandatory pins); go-reviewer control-role centralization (RoleOrDefault, no private allowlist); simplifier fakeHooks reuse. TDD red-first; 11 mutants killed (incl. NOT-WIRED probes for the composed discovery path and the dispatch-arm ledger kind). Incident doc + REGRESSION index row + runtime-reference row; user-phase-persona-resolution-core eval re-scoped. (#493)
- scoped ids reach every phase WITH the live record's path — a bare name resolves to stale namesakes (#491)
- a manual ship with cycle evidence consumes its closed inbox items — un-REDs main (#492)
- internal/bridge joins the env-exclusive integration tier — with an HONEST per-package backstop (#490)
- a remediation-carrying rejection escalates the re-dispatch CLI at its second identical round (#489)
- a red whose bound test NEVER RAN names itself and its cure — the phantom-binding class fix (#487)
- salvage the snapshot-base guard from the 1539-1546 absorbing-FAIL chain (#486)
- a lane must not adopt a PEER lane's continuation (#484)
- the integration-tier gate must not assert a CI outcome it cannot know (#483)
- cycle close must tell the truth — un-RED main, and stop a lost landing reporting PASS (#482)
- a judgment phase's STATED verdict can finally reach the orchestrator (shadow) (#481)
- a correction directive must not forbid the action the gate requires (#480)
- a judgment phase's FAIL verdict teaches without halting (#479)
- artifact-timeout deaths disclose a recognized transient upstream error (all CLI families) (#478)
- admit agy's 'exhausted' quota wall to the bench ledger (#477)
- single-source committed-inbox-id resolution (in-commit consumption) (#476)
- harden tmux submit-verify (cycle-1526 audit prescriptions) (#474)

### Changed

- the SELECT menu is a menu, not an inventory — 65 cards to 22 (#485)

### Documentation

- fleet.count claim was stale — policy.json has run count=3 since the codex-quota restore, but the CLAUDE.md critical-facts digest still said 2 (codex quota-dead until Sep 1). Updated with the live verification: 76 codex dispatches / 44% share over cycles 1530-1552 with zero quota halts; gpt-5.6 sol/terra/luna tiers healthy; codex owns scout/build/coverage lanes while audit/tdd stay claude per the adversarial cross-family design. Also cleared the console plane's expired codex bench entry (benched_until 2026-07-28, inert but confusing). Retires inbox item fleet-count-doc-config-drift (0.62) with this landing. (#495)
- retract the planner-overlap claim — the trace shows sequential FAIL-retry, not concurrent minting (#488)

### Other

- (dossier) cycle-1565 closeout
- (dossier) cycle-1564 closeout
- (dossier) cycle-1563 closeout
- (dossier) cycle-1562 closeout
- evolve-cycle: goal=92a72934f34b57b132a1bcf48865e1a28496ab62e8bef597cdc9fe12d4a11a4f
- (dossier) cycle-1560 closeout
- (dossier) cycle-1561 closeout
- (dossier) cycle-1559 closeout
- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-1557 closeout
- (dossier) cycle-1556 closeout
- (dossier) cycle-1558 closeout
- (ops) deep/top-tier phases run at xhigh reasoning effort (operator directive 2026-08-24). All 24 tracked profiles whose model_tier_default is deep or top get effort_level=xhigh (auditor, adversarial-review, architecture-design, debugger, intent, premise-challenge, retrospective, plan-reviewer, swarm-planner, compat-surface-check, type-safety-audit, merge-to-main-gate, prompt-regression-eval, router, and the design/check family); fast/balanced rows keep the cycle-566 cost matrix (scout/triage=low, builder/tdd=medium). codex-tmux manifest gains the xhigh mapping (-c model_reasoning_effort=xhigh) — realizeScalar SILENTLY drops unmapped enum values, so without it the dial would quietly vanish (worse than high); claude --effort is pass-through and documents xhigh natively; agy/ollama have no dial and no-op cleanly. New guards: TestEffortXhigh_RealizesOnCodexAndClaude pins both realizations, and TestTrackedProfileEffortLevelsAllRealizable is the CLASS guard — every tracked profile's effort_level must be realizable by its own family's manifest (proven to bite: a bogus value reds it naming the silent-drop). Effort-defaults matrix pin updated to the new directive rows. Verified: claude CLI documents xhigh; codex accepts the config key; full bar green (fmt/vet both tag sets, full suite, -race -tags integration on bridge+profiles, acs regression). (#497)
- (dossier) cycle-1554 closeout
- (dossier) cycle-1553 closeout
- (dossier) cycle-1555 closeout
- (dossier) cycle-1551 closeout
- (dossier) cycle-1552 closeout
- (dossier) cycle-1550 closeout
- (dossier) cycle-1547 closeout
- (dossier) cycle-1549 closeout
- evolve-cycle: goal=f704b5c8eb6f10b1f93e74a905beb5e340105519b3a7cd33c4c43da273b9f9bd
- (dossier) cycle-1548 closeout
- (dossier) cycle-1546 closeout
- (dossier) cycle-1545 closeout
- (dossier) cycle-1544 closeout
- (dossier) cycle-1543 closeout
- (dossier) cycle-1542 closeout
- (dossier) cycle-1541 closeout
- (dossier) cycle-1540 closeout
- (dossier) cycle-1539 closeout
- (dossier) cycle-1538 closeout
- evolve-cycle: goal=144e6176979451333ce593369086f9675a1686f8518381d570323625238cf974
- (dossier) cycle-1537 closeout
- (dossier) cycle-1535 closeout
- (dossier) cycle-1536 closeout
- (dossier) cycle-1534 closeout
- (dossier) cycle-1532 closeout
- (dossier) cycle-1531 closeout
- (dossier) cycle-1530 closeout
- (dossier) cycle-1529 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (sync) reconcile plane dossier closeouts with origin (#474 submit-verify hardening)
- (dossier) cycle-1527 closeout
- (dossier) cycle-1528 closeout
- (dossier) cycle-1526 closeout
- (dossier) cycle-1525 closeout
- (dossier) cycle-1522 closeout


---

## [22.19.0] - 2026-08-18

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.18.0..HEAD._

### Added

- fresh-base collision guard — console salvage of lane cycles 1485-1495 + Put-site base-operand fix (#470)

### Fixed

- teach the tree-drift integrity checks the ship's own sanctioned consumption (cycle-1506 P0 — every consuming PASS ship refused at commit) (#473)
- closure-claim gate — lineage-scoped demotion for disposition-covered claims (cycle-1502 verdict-incoherence halt) (#471)
- closure-claim gate — hyphen-compound and path-ref false positives (cycle-1493 infra-systemic halt) + amplification persona placement rules (#469)
- tolerate trailing bytes inside the verdict-sentinel comment (cycle-1478 verdict-incoherence halt) (#468)

### Documentation

- failure-rate review 1481-1503 — taxonomy + ranked reduction plan (2026 citations) (#472)

### Other

- (dossier) cycle-1520 closeout
- (dossier) cycle-1519 closeout
- (dossier) cycle-1517 closeout
- (dossier) cycle-1518 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-1515 closeout
- (dossier) cycle-1516 closeout
- (dossier) cycle-1514 closeout
- (dossier) cycle-1513 closeout
- (dossier) cycle-1512 closeout
- (dossier) cycle-1511 closeout
- (dossier) cycle-1510 closeout
- (dossier) cycle-1509 closeout
- (dossier) cycle-1508 closeout
- (dossier) cycle-1507 closeout
- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-1505 closeout
- (dossier) cycle-1506 closeout
- (dossier) cycle-1504 closeout
- (dossier) cycle-1503 closeout
- (dossier) cycle-1501 closeout
- (dossier) cycle-1502 closeout
- (dossier) cycle-1500 closeout
- (dossier) cycle-1499 closeout
- (dossier) cycle-1498 closeout
- (dossier) cycle-1496 closeout
- (dossier) cycle-1495 closeout
- (dossier) cycle-1497 closeout
- (dossier) cycle-1494 closeout
- (dossier) cycle-1492 closeout
- (dossier) cycle-1493 closeout
- (dossier) cycle-1488 closeout
- (dossier) cycle-1486 closeout
- (dossier) cycle-1487 closeout
- (dossier) cycle-1483 closeout
- (dossier) cycle-1485 closeout
- (dossier) cycle-1484 closeout
- (dossier) cycle-1481 closeout
- (dossier) cycle-1482 closeout
- (dossier) cycle-1478 closeout
- (dossier) cycle-1480 closeout
- (dossier) cycle-1479 closeout
- (dossier) cycle-1477 closeout
- (dossier) cycle-1475 closeout
- (dossier) cycle-1476 closeout


---

## [22.18.0] - 2026-08-15

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.17.0..HEAD._

### Added

- transactional inbox consumption — the PASS ship commit itself consumes its items (#466)
- retrospective moves to claude/deep — Gemini 3.1 Pro retired from the deep tier (#461)
- live parallel latest-model probe across all CLI bridges, offered as an /evo:setup option (#460)

### Fixed

- wall corroboration, 429 taxonomy, codex reasoning-effort pin (operator-directed blocking fix) (#465)
- layer 4 of the staging onion — git-named refusal drop + single retry (P0 batch-halt fix) (#463)

### Documentation

- false-walls + re-pick class deep review — complete failed-cycle ledger, both fixes, and the chain's first live anti-gaming catches (#467)
- wave-4 staging halt post-mortem — root-cause chain, forensic method, layer-4 fix rationale (#464)

### Other

- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-1474 closeout
- evolve-cycle: goal=5683159362b3e3dd8c1d08bdb6b49f6e3e52a324474d1a7be30e46e6740fc297
- (dossier) cycle-1473 closeout
- (dossier) cycle-1475 closeout
- (dossier) cycle-1472 closeout
- (dossier) cycle-1470 closeout
- (dossier) cycle-1469 closeout
- (dossier) cycle-1471 closeout
- (dossier) cycle-1466 closeout
- (dossier) cycle-1467 closeout
- (dossier) cycle-1468 closeout
- (dossier) cycle-1463 closeout
- (dossier) cycle-1465 closeout
- (dossier) cycle-1464 closeout
- (dossier) cycle-1460 closeout
- (dossier) cycle-1461 closeout
- (dossier) cycle-1462 closeout
- (dossier) cycle-1458 closeout
- (dossier) cycle-1459 closeout
- (dossier) cycle-1457 closeout
- (dossier) cycle-1456 closeout
- (dossier) cycle-1455 closeout
- (dossier) cycle-1454 closeout


---

## [22.17.0] - 2026-08-13

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.16.0..HEAD._

### Added

- per-phase output accounting + chain totalization, reported to the unified signal stream (#457)
- show the auditor the chain shape at dispatch — measured 1-in-3 compliance was a not-shown problem (#456)
- chain-of-reasoning audit verdicts + scope-delta adjudication (ADR-0087/0088) (#454)
- schema-aligned salvage layer — repair+persist recoverable verdicts, stage-gated effects, breaker-neutral (closes 4-attempt item) (#453)

### Fixed

- emit from cycle finalize on every topology; resolve names through the gate's own resolver (#459)
- acs-verdict ship-eligibility + incomplete-run honesty, abnormal-event vocabulary, conditional evidence, failure-log summary fallback (#455)

### Other

- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-1452 closeout
- evolve-cycle: goal=5683159362b3e3dd8c1d08bdb6b49f6e3e52a324474d1a7be30e46e6740fc297
- (dossier) cycle-1453 closeout
- (dossier) cycle-1448 closeout
- (dossier) cycle-1449 closeout
- (dossier) cycle-1447 closeout
- evolve-cycle: goal=562773756446126e4af3f89fbf03cc339a978499eacc98323ef42c81f883ce42
- (dossier) cycle-1446 closeout
- (dossier) cycle-1445 closeout
- (dossier) cycle-1444 closeout
- (dossier) cycle-1443 closeout
- (dossier) cycle-1442 closeout
- (dossier) cycle-1441 closeout
- (dossier) cycle-1440 closeout


---

## [22.16.0] - 2026-08-12

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.15.0..HEAD._

### Added

- --push-only recovery — sanctioned completion for attested-but-stranded commits (#446)
- continuation base-advance at adoption — the worktree-base limb of the binary-lag class (cycle-1365: a preserved lane based before #418's .gitignore carve-out hit GIT_STAGE_FAILED twice in the retry loop, unwinnable in place). advanceContinuationBase merges plane main into the adopted lane (snapshotIdentity, --no-edit --no-verify per review LOW) and the healed base supersedes the manifest's stale BaseSHA; the adopt-time Clean screen makes the merge conflict-free by construction; a raced conflict aborts + WARNs conflicting paths + adopts on the stale base (never a half-merge); fast no-op when main is already an ancestor. Reviewer-verified downstream coherence: normalize soft-reset to the healed base leaves exactly the lane work pending; the heal precedes the adoption persist. TDD red-first on a real-git fixture; -race green; vet green; full core green; diff-review APPROVE with both LOWs applied. Consumes inbox continuation-worktree-base-refresh (stage-refusal router residual re-filed as deterministic-stage-refusal-router). (Re-land of reviewed commit 4208391b onto post-#440 main.) (#441)
- disposition-skeleton preseed at continuation adoption — the orchestrator writes <workspace>/defect-dispositions.json with one status-OPEN entry per inherited OPEN id (read from the ancestor ledger), so the auditor only UPGRADES entries instead of hand-enumerating (15/30 FAILs 1390-1429 were disposition-preflight). Gate untouched and unweakened: preflight sees present+covering (never MISSING/INCOMPLETE); the per-id reconcile rejects status OPEN by name — an untouched skeleton still blocks; OPEN is the only honest seed (DEFERRED would launder). Silent no-op on every failure (the GATE stays the loud enforcement); never clobbers; atomicwrite.JSON per review MEDIUM (fixes tmp residue). Cross-package singlesource pin feeds one real ledger through core's seeder AND audit's reconcile (same OPEN set, blocking, no MISSING). Exported + apicover named test (proactive #433 class). Documented interaction: seeded file satisfies the Phase-B secondary hold immediately; per-id gate + ADR-0086 regrade + #434/#438 prompt restoration cover the un-upgraded case. TDD red-first; full module green; -race green; diff-review APPROVE with MEDIUM + both LOWs + STYLE applied. Consumes inbox disposition-skeleton-preseed. (#440)
- bookkeeping-regrade micro-cycle — meta-only audit FAILs re-audit in-cycle instead of dying to continuations (ADR-0086) (#436)
- Phase B core — multi-artifact deliverable contracts (completion waits for the full set) (#432)
- consecutive-failures blocker-breaker rule — 3 cycles failing back-to-back (ANY fingerprints) halts the batch for a deep-dive (operator directive 2026-08-10; the 2026-08-09 batch burned 10 failed cycles / 0 ships before the identity-keyed ceiling tripped). Rule evaluated last so specific diagnoses win; acked digests break the streak; unexplained digests count (pinned); ConsecutiveFailuresHaltCeiling compiled default 3, policy-overridable per-threshold. (#423)

### Fixed

- un-track 2 runtime-minted profile stubs (defect-disposition-ledger, pre-audit-evidence-check) — the documented v22.13.0 release-burn class; files stay on disk for dispatch
- rename sentinel containment helper off the banned symbol — cross-lane 1438/1439 merge red'd main on the tripwire (#452)
- chain-safe inbox-lifecycle appends + physical-tail rebaseline seal (ledger-fleet-concurrency-chain) (#450)
- plane-anchored suite project root + verdict provenance + foreign-root regeneration (cycle-1434 wrong-root class) (#449)
- closure-claim gate word-boundary + negation-aware weak rung (cycle-1431 false-RED) (#448)
- the dispatched artifact path is the ONE source at the runner verify seam — closes the intent-delta contract-path skew (Verify judged intent.md while delta mode dispatched intent-delta.md; found by PR #389's single-read work and deliberately not papered over there). Roots.DispatchedArtifact threads the runner's hooks-derived path into VerifyWithStage (contract keeps owning SHAPE checks; CLI/gate callers without the override keep the contract path, stated in the field doc); checkStray hunts by the dispatched basename per review LOW (the stray hint names the file this run asked for); classifiedArtifact's skew fallback is structurally unreachable via the real verify and its doc now says so — plus names the still-open gate seam in delta mode per review MEDIUM (follow-up queued as gate-seam threading). New pin: Verify judges the dispatched file AND contract-path callers stay unchanged. Existing #389 pins + delta-mode pins green untouched. Full module green; -race green on deliverable+runner; diff-review APPROVE with MEDIUM sentence + LOW applied. Consumes inbox intent-delta-contract-path-skew (0.88, runner-seam scope; gate-seam residual re-filed). (#447)
- overlay family/driver name ambiguity DECIDED and closed — a bare name (no hyphen) is a FAMILY selector satisfied by promoting the chain's same-family entry whatever its transport (chain [claude-p codex] + overlay 'claude' stays on claude-p — no more headless->tmux cross, the exit=10/no-tmux + silent cost/cadence/quota class, hotter since #430 made escalation overlays live-fire); a hyphen-QUALIFIED name is a DRIVER selector passing through unchanged (explicit claude-tmux vs chain [claude-p] wins as written — an explicit transport request is never satisfied by its opposite); the #390 exact-chain-match rung outranks both. First-match tie-break pinned (chain order IS the phase's resolved preference, review MEDIUM). Reviewer-verified: no prefix collisions in the registered roster; the single production call site receives un-normalized advisor/pin names so the rung is live; dedup preserves the demoted primary. TDD red-first (target case red, 3 baselines green); llmroute green; -race green; full module green; diff-review APPROVE with MEDIUM applied. Consumes inbox overlay-family-name-transport-ambiguity (0.87). (#445)
- consumption rides the landing ship for triage-less lanes — the PASS half of the stable-failure-identity asymmetry (PR #439 closed the FAIL half). Continuation/lane cycles carry no triage decision (triageDecisionBytes nil only when BOTH the companion and the report are absent), so a PASS ship promoted NOTHING and a full bookkeeping cycle (~25-30 min; measured ~2h/night) was later spent moving one JSON file (the crosspoll-vs-egps asymmetry). promoteInbox now falls back to cycleoutcome.LaneScopeIDs — the SAME shared reader the FAIL closeout uses (one reader, two halves, can never disagree) — strictly file-ABSENT-only (a present decision with zero committed ids keeps the declined menu unpromoted) and strictly under the cycle-598 landing gate (review MEDIUM: new negative pin proves an UNLANDED ship promotes nothing through the fallback path). Over-promotion posture stated per review: a triage-less PASS promotes the lane's whole pinned menu — the same audit-vouched wholesale promotion the triage path performs on top_n. Rider: pre-existing errcheck in repocontract.go scan.Close surfaced by the package lint — deliberate best-effort drop made explicit. TDD red-first; ship+cycleoutcome green; full module green; -race green; apicover named test for the export; diff-review APPROVE with MEDIUM+LOW applied. Consumes the deterministic half of inbox consumption-rides-landing-ship. (#444)
- load-time registrar-parity clamp for discovered user specs — closes the ADR-0073 security-review Finding-1 trust gap. Any on-disk .evolve/phases/*/phase.json (smuggled residue, git merge, operator typo) became a schedulable phase with author-chosen writes_source and worktree-write ELIGIBILITY, no clamp anywhere (specrunner has zero sandbox plumbing). ClampDiscoveredSpecs now runs at BOTH admission seams (composed cmd_cycle discovery path AND phasespec.MergedCatalog per review MEDIUM — listing/lint may never disagree with dispatch about eligibility): Optional FORCED true (registrar force-then-validate posture; skip->admit conversion documented), writes_source kept ONLY when the spec's dispatch profile — the SAME on-disk file the runner resolves — verifies sandbox-enabled AND repo-writable (ReadOnlyRepo mirror per review LOW: eligibility and capability stay congruent); nil predicate fails closed. Enforcement pairing pinned: registrar = by CONSTRUCTION (persists sandboxed profile), discovery = by VERIFICATION; cross-binding test drives a real Register writes_source mint through the discovery predicate + clamp. Accepted posture stated: a smuggler planting a sandbox-enabled profile gets exactly registrar-mint parity (OS-sandboxed optional writer under the normal gates) — the closed hole is the UNSANDBOXED writer. TDD red-first; composed-path wiring proof green; phasespec/phaseregistrar/cmd green; full module green; -race green; in-package named coverage for both new exports (apicover class pre-empted); diff-review APPROVE with MEDIUM+LOW+nil-test applied. Consumes inbox loadtime-userspec-registrar-clamp (0.93). (#443)
- no FAIL without a reason — finalizeCycle backfills FailReasons from the trusted phase-timing record when a cycle fails at phase-infra level (null-failreasons class, 3 instances/night: retros fingerprinted content-free identities, the identical-fingerprint breaker was blind). Priority: abort-reason entries (originating phase + error head) PLUS failing-phase markers — a RECOVERED transient's abort entry never masks the real failing phase (review MEDIUM applied + negative pin); explicit unexplained marker as the floor — null is structurally impossible. Floor-safety reviewer-verified: no ADR-0072 predicate reads result.FailReasons (disjoint from cs.AuditFailReasons), so the backfill cannot convert a forged-verdict HALT into diagnosed; the len>0 guard makes gate-explained FAILs mutually exclusive with backfill. recovery.PhaseOutcome.AbortReason doc corrected (recovered-transient caveat, review LOW). Residuals itemized: router-replan worktree designation (the diagnosed mechanism) + fingerprint-normalizer path-variance. TDD red-first; -race green; vet green; full module green; diff-review APPROVE with MEDIUM+LOW applied. Consumes half 1 of inbox null-failreasons-capture. (#442)
- two dead governors revived (2026-08-10 investigation) — (1) carryoverTodos P0-flood fingerprint dedupe: cross-cycle duplicate failure classes (124/254 live entries; cycle tokens normalized, case/whitespace folded) collapse to ONE entry at BOTH mint sites, and a suppressed re-mint refreshes the survivor's ExpiresAt so a still-failing class never rides its first occurrence's TTL into the boot prune (review MEDIUM applied); (2) stable failure identity: ApplyFailure falls back to the lane-scope pin's todo_ids when triage-decision.json is ABSENT (the continuation shape — file-absent, not merely empty-committed, per review MEDIUM: declined menus stay blameless), so continuation FAILs finally bump the durable failure_count and TaskRetryCeiling quarantine + deep-escalation become reachable (was: 81 items at 0 after 15 FAILs). TDD red-first; core+cycleoutcome green; full module green; -race green; diff-review APPROVE with both MEDIUMs applied. (Re-land of reviewed commit 4dd392db onto post-#436 main.) (#439)
- continuation-disposition fix family — (1) evidence annotation tolerance: ';'-joined prose fragments are annotations (ignored), cite-SHAPED fragments must ALL resolve, >=1 cite mandatory (kills the 1393/1415 whole-claim rejections; typoed paths can never demote to prose; the '; ' array join stays all-cite-shaped AND-semantics); (2) continuation audit prompts carry the ancestor's OPEN defect ids + single-line texts + the disposition duty, composed from the SAME records the gate grades (manifest -> registry fallback -> ancestor ledger; best-effort, gate stays the loud enforcement). Diff-review APPROVE with MEDIUM applied: agent-authored ledger Text flattened to one line so embedded newline headings can never masquerade as mechanism prompt structure (+ injection pin); dot-only-cite case pins the '.' half of citeShaped. TDD red-first (exact 1393/1415 evidence shapes); audit package + full module green; -race green; consumes inbox evidence-cite-annotation-tolerance + the auditor half of continuation-disposition-producer-duty. (Re-land of reviewed commit 3ea7588e onto post-#436 main.) (#438)
- persona-tail curation completed for ALL dispatched personas (persona-strip lobotomy follow-up) — builder/scout/tdd/triage/intent markers relocated to EOF; builder regains STOP CRITERION/completion gates/regression slice/attestation/POSTHOC, scout its STOP gates, tdd the full predicate-quality rules, triage inbox ingestion + idempotency skip-list, intent its OUTPUT CONTRACT (intent-delta.md — plausible cause of the contract-path-skew item) + re-run protocol + reflection duty. Archaeology: cycle-413/415/416/422 tests and acs predicates ACTIVELY DEMANDED the burial (floors 4096/1500/1200/4200/2200 + absent-after-strip negatives; C413_009 pinned tdd as marker-LESS; latent-red C416 vs in-CI compaction sat in direct contradiction — burial won because it ran in CI). Every burial pin inverted to a keep-pin with incident rationale; floors recalibrated to marker-presence-only; genuine tails preserved (builder/scout index tables, intent Composition+Reference, triage step-3b verbose example per C422_010); pendingCuration EMPTY (self-pruning keep-guard arms for all 10 personas). Diff-review BLOCK->HIGH fixed (cycle413 pins) + STYLE items applied -> green. Full module green; -tags acs cycles 413/415/416/417/420/421/422 all green; line budget 750/751. Pre-existing latent reds noted for the dead-contract sweep: cycle388/412 size fossils, cycle298 vet. (#437)
- iteration-0 escalation swallow — ApplyBoundary's per-cycle idempotency stamp compared stamp[key]==opts.Cycle, and a never-applied key reads as map zero value 0; fleet boundaries pass the 0-based pool iteration as Cycle, so iteration-0 passes silently skipped EVERY staged intent (observed live on the enforce-flip day: apply report cycle:0 all-null, stamp {}, 5 intents pending since 1225-1378 applied zero). Presence-check fix (prev,ok := stamp[key]); regression test pins cycle-0 application AND stamp-mechanism idempotency (Skipped-purity per review — distinguishes stamp skip from never-lower masking). TDD red-first with the exact live symptom; -race green; full module green; diff-review APPROVE, MEDIUM applied. (#435)
- persona-strip lobotomy — auditor ## Reference Index marker relocated from line 75/272 to EOF so CompactPrompts stripping no longer deletes the Verdict Rules, STOP CRITERION, output-path contract, POSTHOC, constitutional checklist, and the MANDATORY continuation-disposition contract from every dispatched audit prompt (cycles 1390-1429: 15/30 FAILs on disposition-preflight, 0/11 continuation passes; live audit-prompt.txt zero 'disposition' occurrences). New keep-guard phasecoherence/persona_strip_operational_test.go: 8 incident anchors pinned on the REAL persona + fleet-wide sentinel rule over git-tracked dispatched personas (widened per review: POSTHOC/Verdict Rules/Constitutional audit) with SELF-PRUNING exception list (builder/scout/tdd — an excepted persona gone clean fails until delisted; triage's sentinel-invisible tail tracked in incident follow-ups). realdoc_strip_test.go auditor floor 4096->256 (old floor fossilized the truncation as '~70% tail'). Incident doc + REGRESSION index rows (incl. #432 retro-cutoff GAP -> covered). TDD red-first; full module green; -race green; diff-review APPROVE with both MEDIUMs applied. (#434)
- minted/unresolved agents get their polled artifact path disclosed — footer-only (cycle-1424 SYSTEM halt: a minted phase dispatched naked hit a 600s artifact-timeout writing nothing; adversarial-review BLOCK applied: RenderContractFooter not RenderContractTail, since the tail's evolve-phase-verify self-check is guaranteed exit 10 for a resolver miss — an impossible instruction, the same class being fixed; negatives pinned). Two pass-through pin tests updated to the disclosed-path intent. (#429)
- release declined registry bindings — kills the absorbing-FAIL state (cycles 1412/1418: adopter declines a stale snapshot without a manifest, the root-owned binding is immortal, the defect-ledger gate's out-of-band check then auto-FAILs every future lane on that scope). DeleteRegistryEntry + single-lock DeleteRegistryEntryIfCycle (adversarial-review HIGH: check+delete under one flock hold so a sibling lane's concurrent rebind survives; shared publishRegistryLocked so write/delete publish cannot diverge); orchestrator-side releaseDeclinedBinding in the adoption-decline branch (agents have no path — the gate's cycle-1285 anti-tamper block is untouched and its pins stay green); retro persona write-order directive (disposition.json BEFORE the final report — the completion-detector race, interim for plan Phase B). (#428)
- two live false-positive gate bugs (ADR-0084 invariant 2) — eval quality-check reads the [code] grader-bullet form (281/625 evals were unscanned=vacuous PASS; fence-state tracked so decoy bullets inside non-bash fences never count — adversarial-review BLOCK applied; score_cap evals PASS with an ACS-jurisdiction note; zero commands = reasoned WARN never silent PASS; template round-trip single-source test), and disposition.json gets a LEGAL literal example single-sourced persona<->Go<->VerifyDisposition (the old placeholder pseudo-JSON failed its own fail-HARD gate) + retro profile Write grants for disposition.json/failure-decision.json (role-gate landmine removed). (#426)
- salvage-land the cycle-1403 disposition-contract fix — exact defect-dispositions.json schema + literal example published in evolve-auditor.md (single-sourced against the Go reader by test), tolerant evidence unmarshal (string OR array-of-strings, fail-closed on all other shapes), schema-parse errors name the expected shape inline; 4 regression test files + cycle-1403 ACS predicates + 3 evals. Audit-PASSED in-cycle 1403; stranded by the cd49274beab2 false-RED ship gate (fixed in #421); landed console-first per salvage-before-requeue. Unblocks the 1392/1387 continuation ancestries. (#422)
- Direction-B pairing binds only git-TRACKED profiles — untracked runtime-minted stubs are runtime state, not repo config (cd49274beab2 ship|gate-block storm: cycles 1402/1403/1405 audit-green ships blocked on the unpaired gitignored-by-design defect-disposition-ledger.json minted on the live plane; identical-fingerprint ceiling then halted the batch). trackedProfiles() via git ls-files with stderr-surfaced errors + loud empty-set fallback to strict bind-all; obsolete minted-stub allowlist entries removed per the shrink ratchet; fixture regression pins tracked-bound/untracked-unbound + non-repo loud error. (#421)

### Documentation

- verify-wave halts incident + runtime-reference/EGPS/closure-gate/CHANGELOG for #448–#450 (#451)
- incident 2026-08-10-continuation-absorbing-fail (batches 3-4 halts: registry absorbing-FAIL, minted naked dispatch, ledger fleet-concurrency findings; regression coverage map + 5 lessons), ADR-0085 continuation-registry release-at-decline (problem/decision/alternatives/consequences), runtime-reference rows (consecutive_failures_halt_ceiling, failure_disposition.stage=enforce + width-2-Aug note), CODEBASE-MAP 153 pkgs + repostate, continuation-defect-ledger registry-lifecycle section, eval-grader-best-practices recognized-formats note, REGRESSION index rows (#428/#429 pins + ledger GAP). (#431)

### Other

- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-1439 closeout
- (dossier) cycle-1438 closeout
- evolve-cycle: goal=562773756446126e4af3f89fbf03cc339a978499eacc98323ef42c81f883ce42
- (dossier) cycle-1436 closeout
- (dossier) cycle-1437 closeout
- (dossier) cycle-1434 closeout
- (dossier) cycle-1435 closeout
- (dossier) cycle-1432 closeout
- (dossier) cycle-1433 closeout
- (dossier) cycle-1431 closeout
- (dossier) cycle-1430 closeout
- (dossier) cycle-1429 closeout
- (dossier) cycle-1428 closeout
- (dossier) cycle-1427 closeout
- (dossier) cycle-1426 closeout
- (ops) operator-approved batch — CLAUDE.md Automation Loop Guardrails (2-zero-ship halt, per-cycle status format, CI flake-vs-real classification, Go conventions), failure_disposition.stage=enforce (escalation boundary LIVE after 25+ cycle shadow soak with planned=5/boundary applied=0), fleet.count 3->2 for August (codex quota-dead until Sep 1; restore via wave-boundary bounce recipe); digest line updated to keep the policy-block claim truthful. (#430)
- (dossier) cycle-1424 closeout
- (dossier) cycle-1425 closeout
- (dossier) cycle-1423 closeout
- (dossier) cycle-1421 closeout
- (dossier) cycle-1420 closeout
- evolve-cycle: goal=debce4966a71c09b3946c535261abd35be9ac63bfd613ae68de5d9845ecefa01
- (dossier) cycle-1418 closeout
- (dossier) cycle-1419 closeout
- docs(adr-0084)+review-lenses: gate integrity invariants ADR (tracked-state scanning / single-sourced contracts / persisted evidence — problem/hypothesis/decision/consequences from the 2026-08-09 incident), the contract-single-sourcing recipe guide (three-legged test, token-cost ROI), and the three ADR-0084 lenses added to the diff-review skill + auditor persona (line-budget verified). (#427)
- test+feat: tracked-only binding for every real-tree scanner (ADR-0084 invariant 1) (#425)
- test(edge-suite)+docs(incident): 2026-08-09 zero-ship batch — 14 edge-case pins across the three fixes (#421 tracked-only binding: stderr fidelity/empty-set/staged/nested-alias + top-level-only fix; #422 evidence: mixed-type/null/whitespace/semicolon-join both directions; #423 breaker: guard-class precedence/ceiling-1/duplicate-cycle/ack-splitting), full postmortem at docs/incidents/2026-08-09-zero-ship-batch.md (4 root causes, 7 lessons, coverage map), REGRESSION-COVERAGE-INDEX rows added incl. the one open GAP (retro completion cutoff, queued 0.90). (#424)
- (dossier) cycle-1416 closeout
- (dossier) cycle-1415 closeout
- (dossier) cycle-1414 closeout
- (dossier) cycle-1412 closeout
- (dossier) cycle-1413 closeout
- (dossier) cycle-1411 closeout
- (dossier) cycle-1409 closeout
- (dossier) cycle-1410 closeout
- (dossier) cycle-1408 closeout
- (dossier) cycle-1407 closeout
- (dossier) cycle-1406 closeout
- (dossier) cycle-1405 closeout
- (dossier) cycle-1404 closeout
- (dossier) cycle-1403 closeout
- (dossier) cycle-1401 closeout
- (dossier) cycle-1402 closeout
- (dossier) cycle-1400 closeout
- (dossier) cycle-1399 closeout
- (dossier) cycle-1398 closeout
- (dossier) cycle-1397 closeout


---

## [22.15.0] - 2026-08-06

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.14.0..HEAD._

### Added

- latest-model selection with lineage safety, fingerprint stability, and a shadow-staged write path (#417)

### Fixed

- restore the scout eval-path pin sentence — cycle-1383's persona compression dropped 'the' from 'Do NOT write only to the cycle worktree', breaking TestLoopUnblockScoutPromptRequiresWorkspaceEvalPath and redding main. One word; line budget unchanged (748). Green-in-lane/red-on-main because the lane's stale worktree base predates the pin — third manifestation of the worktree-base class (continuation-worktree-base-refresh 0.92): the ship-time repo-contract gate evaluated in a tree WITHOUT the pin, adding gate-evasion to that item's blast radius. (#420)
- decorated-cite tolerance + disposition-duty prominence (#419)
- new evals were born ignored — tracked-corpus/ladder split killed every eval ship (cycle-1345/1346/1348 batch HALT) (#418)
- allowlist the two runtime-minted stub profiles (reasoned, ratcheted) — first live REPO_CONTRACT_GATE firing exposed the interaction: the minter re-mints ship-stage-hygiene-check + regression-predicate-precheck in EVERY lane (their phases are tracked), the pairing guard scans disk, and the new ship gate correctly refused pushes — blocking lanes on stubs they did not author (cycle ~1326, two firings). The guard's own remedy applied: allowlist with a reason; the shrink ratchet keeps pressure (author the personas -> delete the entries; tracked on the phase-mint class item). Verified green WITH the stubs present on disk (lane condition reproduced). (#415)

### Documentation

- full documentation refresh audit (4 parallel verify-then-write auditors, v22.14.0 truth) — README (cycle count 1,300+, 4-CLI roster replacing the Gemini-era list, real v22.13/v22.14 version rows, chronicle link, moved-path fixes), AGENTS.md (CORRECTED a wrong phase spine to the state-machine-pinned Intent->Scout->Triage->[TDD]->Build->Audit->Ship->Learn; ADR-0075 guard status; doc-root rules), CLAUDE.md digest (repo_contract_gate=enforce in the gates list; boot.binary_refresh + escalation/ack bullets; corrected the false no-pins claim — memo->agy/fast pin exists), runtime-reference (8-gate GatesConfig enumeration, repo-contract gate row, boot-refresh row, batch-window floor, #394 demote net; docs-contract test run green by the auditor), control-flags (both new dials), deliverable-contract (salvage rung + escalation cross-link), link integrity 69 broken -> 21 (48 fixed incl. the release-notes depth break and two never-existed targets traced to their real docs; remaining 21 = deleted-legacy classes, enumerated), indexes/CODEBASE-MAP/MOVED corrected (ADR range 0001-0083, package count 152, chronicle indexed). Two unverifiable historical figures left flagged rather than guessed. (#416)
- refresh the Pages site SSOT to v22.14.0 — seven releases of drift closed (content.json is a manual bump per the landing-conversion gotcha). Facts: version v22.7.0->v22.14.0; 1,040+->1,320+ autonomous cycles; 3 CLIs (Claude/Codex/Gemini) -> 4 CLIs (Claude Code/Codex/Antigravity/Ollama); installer terminal literal modernized incl. the SHA256 fingerprint trust line; release example gains the auto-demote net; pillar proofs upgraded with the week's shipped capabilities (boot binary self-heal; two-round adversarial arcs + defect-disposition ledger; contract-block CLI escalation). Landing suite green (buildsite/content/render); local SSG build verified v22.14.0 in 5 pages, zero stale strings. (#414)

### Other

- (dossier) cycle-1393 closeout
- (dossier) cycle-1394 closeout
- (dossier) cycle-1390 closeout
- (dossier) cycle-1392 closeout
- (dossier) cycle-1391 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1388 closeout
- (dossier) cycle-1389 closeout
- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-1387 closeout
- (dossier) cycle-1386 closeout
- (dossier) cycle-1385 closeout
- (dossier) cycle-1384 closeout
- (dossier) cycle-1383 closeout
- (dossier) cycle-1381 closeout
- (dossier) cycle-1382 closeout
- (dossier) cycle-1379 closeout
- (dossier) cycle-1380 closeout
- (dossier) cycle-1377 closeout
- (dossier) cycle-1378 closeout
- (dossier) cycle-1375 closeout
- (dossier) cycle-1376 closeout
- (dossier) cycle-1373 closeout
- (dossier) cycle-1374 closeout
- (dossier) cycle-1372 closeout
- (dossier) cycle-1371 closeout
- (dossier) cycle-1370 closeout
- (dossier) cycle-1369 closeout
- (dossier) cycle-1368 closeout
- (dossier) cycle-1367 closeout
- (dossier) cycle-1364 closeout
- (dossier) cycle-1365 closeout
- (dossier) cycle-1366 closeout
- (dossier) cycle-1363 closeout
- (dossier) cycle-1362 closeout
- (dossier) cycle-1361 closeout
- (dossier) cycle-1360 closeout
- (dossier) cycle-1359 closeout
- (dossier) cycle-1358 closeout
- (dossier) cycle-1356 closeout
- (dossier) cycle-1357 closeout
- (dossier) cycle-1355 closeout
- (dossier) cycle-1354 closeout
- (dossier) cycle-1353 closeout
- (dossier) cycle-1352 closeout
- (dossier) cycle-1351 closeout
- (dossier) cycle-1348 closeout
- (dossier) cycle-1347 closeout
- (dossier) cycle-1345 closeout
- (dossier) cycle-1346 closeout
- (dossier) cycle-1344 closeout
- (dossier) cycle-1343 closeout
- (dossier) cycle-1341 closeout
- (dossier) cycle-1342 closeout
- (dossier) cycle-1340 closeout
- (dossier) cycle-1339 closeout
- (dossier) cycle-1338 closeout
- (dossier) cycle-1336 closeout
- (dossier) cycle-1337 closeout
- (dossier) cycle-1335 closeout
- (dossier) cycle-1334 closeout
- (dossier) cycle-1333 closeout
- (dossier) cycle-1332 closeout
- (dossier) cycle-1330 closeout
- (dossier) cycle-1331 closeout
- (dossier) cycle-1329 closeout
- (dossier) cycle-1328 closeout
- (dossier) cycle-1327 closeout
- (dossier) cycle-1326 closeout
- (dossier) cycle-1324 closeout
- (dossier) cycle-1325 closeout
- (dossier) cycle-1323 closeout
- (dossier) cycle-1322 closeout
- (dossier) cycle-1321 closeout


---

## [22.14.0] - 2026-08-05

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.13.1..HEAD._

### Added

- repo-contract scanner pack — lane ships can no longer red main (#413)

### Fixed

- restore incident-postmortem to its durable spec — cycle-1313's ship reworked the phase config against the test-pinned domain-phase-catalog contract (sections renamed off-spec; persona declared FOREIGN tool vocabulary view_file/grep_search/run_command vs the Claude-vocabulary profile; novel kind field). Per-cycle scope never ran the repo-wide catalog tests (scope-disease costume #5) so main went red on the merge run + the persona-coherence ship gate blocked plane ships. Repair-in-place (the same ship carries the legit F4 triage admission — no revert): spec sections Impact/Timeline/Root Cause/Action Items restored, persona tools aligned to profile vocabulary, kind dropped, stale allowlist entry deleted (ratchet shrinks). phasespec + phasecoherence green. (#412)
- boot-time binary staleness self-heal (binary-lag class) — twice-adversarially-reviewed (#411)

### Documentation

- engineering chronicle (17 narratives) + doc-root consolidation into docs/ (#410)
- batch integrity review 2026-08-04 — six findings in issue/gap/solution format (defect laundering across salvage chains incl. the 1255-D1 stale-worktree CRITICAL narrowed to 'verified closed'; TIA dormant-code-under-active-soak status fiction with vacuous soak bar; audit-WARN prescriptions unenforced at ship (1258 durable eval exists nowhere, predicate green-by-skip); triage protected-surface admission gap that burned 3 lanes; dead-red predicate corpus; ship-claim misattribution). Verdict tables for all 9 ships + 7 FAILs (code SUBSTANTIVE, FAILs honest, gaming lives in status accounting). TDD/design conformance follow-up for #398-#407. Operating-policy additions: §3.8 issue/gap/solution docs on every fix (operator directive) + §3.9 ledger-writes-derive-from-diffs with per-defect continuation dispositions. Queue reprioritized: the five review items hold ranks 1/2/4/5 (0.96/0.95/0.91/0.90). (#408)

### Other

- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-1320 closeout
- (dossier) cycle-1319 closeout
- (dossier) cycle-1318 closeout
- (dossier) cycle-1317 closeout
- (dossier) cycle-1315 closeout
- (dossier) cycle-1316 closeout
- (dossier) cycle-1314 closeout
- (dossier) cycle-1313 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1312 closeout
- (dossier) cycle-1311 closeout
- (dossier) cycle-1309 closeout
- (dossier) cycle-1310 closeout
- (dossier) cycle-1308 closeout
- (dossier) cycle-1307 closeout
- (dossier) cycle-1305 closeout
- (dossier) cycle-1306 closeout
- (dossier) cycle-1303 closeout
- (dossier) cycle-1304 closeout
- (dossier) cycle-1301 closeout
- (dossier) cycle-1302 closeout
- (dossier) cycle-1300 closeout
- (dossier) cycle-1299 closeout
- (dossier) cycle-1298 closeout
- (dossier) cycle-1297 closeout
- (dossier) cycle-1296 closeout
- (dossier) cycle-1295 closeout
- (dossier) cycle-1294 closeout
- (dossier) cycle-1293 closeout
- (dossier) cycle-1291 closeout
- (dossier) cycle-1292 closeout
- (dossier) cycle-1289 closeout
- (dossier) cycle-1290 closeout
- (dossier) cycle-1288 closeout
- (research) deliverable-alignment strategic evaluation 2026-08 — two-track research (local institutional memory + 2025-26 state of the art) answering the operator's find-one-solve-one question. Verdict: right VERIFICATION posture, wrong PREVENTION strategy — the four-layer model assigns each failure class its layer (generation-point / transport+salvage / verification / accounting). Measured record: ~65% of alignment failures were HARNESS defects; contract-tail took bad_verdict to zero; adversarial audit best catch-rate; breaker is a fail-open ratchet under weak CLIs. Ranked 7-move portfolio filed to the live queue: contract-block-cli-escalation 0.96 (rank 1 — refiled: the original existed only in the tracked snapshot, a live instance of the stranding it documents), schema-aligned-salvage-layer 0.9, crossartifact-invariant-stack 0.85, claude-stop-hook-finish-gate 0.8; two-stage verdict minting folds into verdict-sentinel-as-tool-call 0.86. Full citations. (#409)
- (dossier) cycle-1287 closeout
- (dossier) cycle-1286 closeout
- (dossier) cycle-1285 closeout
- (dossier) cycle-1284 closeout
- (dossier) cycle-1282 closeout
- (dossier) cycle-1283 closeout
- (dossier) cycle-1281 closeout
- (dossier) cycle-1280 closeout
- (dossier) cycle-1279 closeout
- (dossier) cycle-1278 closeout
- (dossier) cycle-1277 closeout
- (dossier) cycle-1276 closeout
- (dossier) cycle-1275 closeout
- (dossier) cycle-1274 closeout
- (dossier) cycle-1272 closeout
- (dossier) cycle-1273 closeout
- (dossier) cycle-1270 closeout
- (dossier) cycle-1271 closeout


---

## [22.13.1] - 2026-08-04

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.13.0..HEAD._

### Fixed

- un-track runtime-minted profile stubs + targeted gitignore — the v22.13.0 release suite failed on TestSmoke_RealProfiles + TestRepoPersonaProfilePairing because two minted stubs (empty cli, no persona pairing) rode a queue ship into tracking; release auto-demoted to prerelease by the #394 net, v22.12.1 stayed Latest. Same class as the gate-wiring-proof stubs (#399), now proven MINT-WIDE (phases + profiles) — evidence added to phase-mint-carries-select-metadata 0.9; new residual release-preflight-repo-contract-suites 0.8 (preflight's 2 gate suites never run the repo-contract scanners). Both failing tests reproduced at f3548a49 and green after; dispatch keeps reading the stubs from DISK on the runtime plane (restore choreography around sync documented in the PR). (#406)

### Other

- Merge remote-tracking branch 'origin/main'
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1268 closeout
- (dossier) cycle-1269 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517


---

## [22.13.0] - 2026-08-04

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.12.1..HEAD._

### Fixed

- SELECT metadata for the runtime-minted ship-stage-hygiene-check stub — cycle-1262's scout window minted an empty phase.json, ship's whole-tree bind swept it into tracking, and the repo-wide catalog guard (TestPhaseCatalog_OptionalPhasesHaveSelectMetadata) went RED on main both platforms while per-cycle changed-scope never selected internal/phasespec (scope-disease costume #4 — filed as TIA enforce-flip evidence). Honest metadata describes the reserved intent (pre-ship manifest reconciliation) and marks it a config stub with no gates wired; guard demanded metadata-not-allowlist. RED at 37bc664a -> GREEN with phasespec+phasecoherence. Residual queued: phase-mint-carries-select-metadata 0.85 (make the MINTER satisfy the catalog contract). (#404)
- deflake channel e2e — positive boot sync + advancing supervisor clock. The ClaudeSpan e2e synced on a bare 20ms sleep for 'driver has seeked the inbox to EOF'; a late-scheduled driver goroutine (loaded macOS CI runner) seeked AFTER the supervisor's append, the ask was skipped forever, the self-paced fake tmux never advanced, and the frozen injected clock meant Supervisor.Ask's 10s deadline could structurally never fire — 10-minute package-timeout panic instead of a diagnosis. Proof matrix under an induced 50ms driver delay: old code HANGS (reproduces CI), advancing clock alone fails crisp in 5s with ErrResponseTimeout, positive sync (poll for build-pane.live, created strictly AFTER the cursor seek in the same goroutine) PASSES x10 -race in 2s. Test-only diff; sibling e2es audited safe (their feed appends are unconditional). (#403)
- bounded retry on worktree provisioning — one transient .git lock collision must not cost a lane its cycle (#401)
- a TEST-ONLY new package must not abort graduation (batch-halt P0, both seams) (#398)
- un-track the runtime-minted gate-wiring-proof stubs — MY pre-launch ship swept them into tracking and turned main RED (#399)
- the composed-gate apicover check now ENFORCES — closing the six-recurrence warnship class at its map entry (#396)
- a failed publish pipeline demotes its own assetless tag listing (#394)

### Documentation

- graph engineering — concept mapping + the three slices worth merging (#397)

### Other

- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-1267 closeout
- (dossier) cycle-1266 closeout
- (dossier) cycle-1264 closeout
- (dossier) cycle-1265 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1262 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1263 closeout
- (dossier) cycle-1260 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1261 closeout
- (dossier) cycle-1258 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1259 closeout
- (dossier) cycle-1256 closeout
- (dossier) cycle-1257 closeout
- (dossier) cycle-1255 closeout
- (dossier) cycle-1254 closeout
- (dossier) cycle-1253 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-1252 closeout
- (dossier) cycle-1251 closeout
- (dossier) cycle-1250 closeout
- (dossier) cycle-1248 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1249 closeout
- (dossier) cycle-1247 closeout
- (dossier) cycle-1246 closeout
- (dossier) cycle-1245 closeout
- (dossier) cycle-1244 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1242 closeout
- (dossier) cycle-1243 closeout
- (dossier) cycle-1241 closeout
- (dossier) cycle-1240 closeout
- (dossier) cycle-1239 closeout
- (dossier) cycle-1238 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1237 closeout
- (dossier) cycle-1236 closeout
- (dossier) cycle-1233 closeout
- (dossier) cycle-1235 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1234 closeout
- (dossier) cycle-1230 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1231 closeout
- (dossier) cycle-1232 closeout
- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-1228 closeout
- (dossier) cycle-1229 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1226 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1227 closeout
- (dossier) cycle-1224 closeout
- (dossier) cycle-1225 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1222 closeout
- (dossier) cycle-1223 closeout
- (dossier) cycle-1221 closeout
- Merge remote-tracking branch 'origin/main'


---

## [22.12.1] - 2026-07-31

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.12.0..HEAD._

### Fixed

- the e2e timeout belongs in the make recipe, not in one workflow (v22.12.0 shipped assetless) (#393)
- a staged RENAME's source path must not enter the git-add pathspec (#391)


---

## [22.12.0] - 2026-07-30

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.11.1..HEAD._

### Added

- spine fail-open rollup, hermetic guards HOME, triage card files, and the flaky-shape gate WIRED (#388)
- house rules reach the agents + triage pin follows its reroute (#383)
- FAIL cycles commit their failure identity — fingerprint, class, and reason heads (#379)

### Fixed

- deep-tier artifact budgets + contract-gate CLI escalation (rebuilt on main, replaces #384) (#390)
- classify the bytes that were verified, stop sleeping on a dead ctx, and stop calling a phase that RAN 'skipped' (#389)
- union non-progress breaker, derivability-aware CI gates, quota wake-at (#387)
- unblock TestE2ECLIFallbackChain — fixture inherited a live catalog refresh; spine outgrew the budget (#386)
- write-in-flight grace window — a verify racing the agent's final write no longer fails CLOSED (#378)

### Documentation

- change record 2026-07-29/30 — issue, rationale, and what each fix did in reality (#385)
- LLM output-stability review — literature vs our architecture, five items queued (#381)
- failed-loop analysis vs the literature — batches 17-20 taxonomy + five research axes + queued improvements (#377)

### Other

- (dossier) cycle-1217 closeout
- (dossier) cycle-1218 closeout
- (dossier) cycle-1216 closeout
- (dossier) cycle-1215 closeout
- (dossier) cycle-1213 closeout
- (dossier) cycle-1214 closeout
- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-1211 closeout
- (dossier) cycle-1210 closeout
- (dossier) cycle-1209 closeout


---

## [22.11.1] - 2026-07-30

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.11.0..HEAD._

### Fixed

- abnormal-exit failure reasons carry the abort cause — closes the batch-19 diagnosability halt class (#376)
- adversarial-review pin follows the live reroute — claude/deep, not agy (#375)

### Other

- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-1208 closeout
- (dossier) cycle-1207 closeout
- (dossier) cycle-1205 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1206 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1204 closeout
- (dossier) cycle-1203 closeout
- (dossier) cycle-1201 closeout
- (dossier) cycle-1202 closeout
- (dossier) cycle-1200 closeout
- (dossier) cycle-1199 closeout
- (dossier) cycle-1197 closeout
- (dossier) cycle-1198 closeout
- Merge origin/main (891e50f4) into main — cycle-1198 lane-base-fetch-origin-main reconciliation (merge only; nothing pushed)
- (dossier) cycle-1196 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1195 closeout


---

## [22.11.0] - 2026-07-30

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.10.0..HEAD._

### Fixed

- red predicates become self-diagnosing — full-stream evidence files + bounded-retry outcome annotation (#374)
- graded-FAIL attempt accounting reaches fleet lanes — the chain now lives in the cycle-run child epilogue (#373)

### Other

- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-1194 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1193 closeout
- (dossier) cycle-1191 closeout
- (dossier) cycle-1192 closeout
- (dossier) cycle-1189 closeout
- (dossier) cycle-1190 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1187 closeout
- (dossier) cycle-1188 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1185 closeout
- (dossier) cycle-1186 closeout
- (dossier) cycle-1184 closeout
- (dossier) cycle-1183 closeout
- (dossier) cycle-1181 closeout
- (dossier) cycle-1182 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1180 closeout
- (dossier) cycle-1179 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1178 closeout
- (dossier) cycle-1177 closeout
- (dossier) cycle-1176 closeout
- (dossier) cycle-1175 closeout
- (dossier) cycle-1174 closeout
- (dossier) cycle-1173 closeout
- (dossier) cycle-1171 closeout
- (dossier) cycle-1172 closeout


---

## [22.10.0] - 2026-07-29

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.9.0..HEAD._

### Added

- S2-S5 — plane classification, wave-boundary FF sync, hub-resident console lease (#372)

### Documentation

- runtime/console plane separation — the loop owns its checkout (#371)

### Other

- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-1170 closeout
- (dossier) cycle-1169 closeout
- (dossier) cycle-1167 closeout
- (dossier) cycle-1168 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1166 closeout
- (dossier) cycle-1165 closeout
- (dossier) cycle-1164 closeout
- (dossier) cycle-1163 closeout


---

## [22.9.0] - 2026-07-28

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.8.0..HEAD._

### Fixed

- serve reasons as prior findings + fold duration tokens from fingerprints (#370)
- absorb JSON null cycle in the defensive unmarshal (#369)
- identical-fingerprint rule requires content-bearing identity — batch-14 false halt (#368)

### Other

- (dossier) cycle-1159 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1160 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1158 closeout
- (dossier) cycle-1157 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1156 closeout
- (dossier) cycle-1155 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1154 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1153 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1152 closeout
- Merge branch 'main' of https://github.com/mickeyyaya/evolve-loop
- (dossier) cycle-1147 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1148 closeout
- (dossier) cycle-1146 closeout
- (dossier) cycle-1145 closeout
- Merge branch 'main' of https://github.com/mickeyyaya/evolve-loop
- Merge branch 'main' of https://github.com/mickeyyaya/evolve-loop
- (dossier) cycle-1143 closeout
- (dossier) cycle-1144 closeout
- (dossier) cycle-1142 closeout
- (dossier) cycle-1141 closeout
- (dossier) cycle-1139 closeout
- (dossier) cycle-1140 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1137 closeout
- (dossier) cycle-1138 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517


---

## [22.8.0] - 2026-07-28

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.7.0..HEAD._

### Added

- salvage cycle-1123's fatalpane agent-content stripping — 4th-attempt audited-PASS work recovered from a false red
- salvage cycle-1101 — persona-budget in-lane gate: agents/evolve-*.md diffs run internal/prompts tests at the build floor pre-handoff (third warnship-apicover-ci-gap instance closed; audited PASS in-cycle, ship was killed by the cycle-1101 gitignored-eval staging refusal fixed at e8990e53; consumed persona-budget-inlane-gate item)
- salvage cycle-1098 — chain min-one-batch guarantee + inbox item shape-validation with loud skips (audited PASS in-cycle; ship was killed by the d202aeb6 pathspec bug; consumed loop-batch-chaining item)
- continuation-on-fail — a FAILed cycle's preserved worktree is snapshot-committed (Clean-screened), stamped onto released items transactionally, and the next cycle that CLAIMS the item re-seeds its worktree from the snapshot post-triage (architect finding #1: claims exist only mid-cycle), reviews cumulative work against the original base, unions the prior declared manifest at ship, and serves prior findings to the builder. Grades never carry; quarantine sheds stamps; every failure falls back fresh, loudly (#362)
- difficulty-conditioned budgets — triage's cycle_size_estimate (via a new triage-report.md digest fallback; handoff JSONs are extinct) scales the build phase's correction ladder and artifact-wait deadline. Policy SizeBudgetMultipliers {trivial/small:1.0, medium:1.25, large:1.5} compiled defaults + positive-override merge; BudgetScale threaded PhaseRequest→BridgeRequest→launchArgs (pure extraction from Engine.Launch); build-only scope, absent/unknown size pinned byte-identical (#361)
- convergence architecture — green-before-handoff (evolve selfcheck build, DI seam pinned to the real floor) + builder mandate + A/C/D campaign queued (difficulty budgets, continuation-on-fail, retry tier escalation)
- mid-batch pipeline-blocker breaker — guard-class and identical-fingerprint ceilings over the S1 failure digests halt the batch with P0 escalation before the failure passes to following cycles (#353)
- honest failure scenario in the inbox lab — every 3rd cycle FAILs at audit, retro classifies + files a lesson todo, item re-queues re-weighted with worktree kept; new audit-verdict-FAIL card in the recovery section (one routed-not-fatal contract across both)
- rewire the inert guard-phase hook to Agent|Task matcher + doc-truth corrections (cycles 1028/1035 salvage; operator control-plane step) (#352)
- typed routing authority — plan-time console-route gate across all schedulers + claim backstop + clamp-parity lane override (#349)

### Fixed

- short subtest names — 40xPASS narrative overflowed t.TempDir on CI Go 1.23 (#367)
- campaign partition + fleet lane menus — batches become consumable units (#366)
- add the minAreaDepth floor — a top-level directory is a bag, not a unit of work
- close the four false-red classes (evidence identity, hermetic roots, probe quarantine, scope demotion) (#365)
- stamp the continuation manifest on the ABNORMAL exit path too — cycle-1078 (first live FAIL under the stack) exposed G1: a review-gate rejection exits RunCycle as an error, bypassing finalizeCycle, so the preserved worktree carried no manifest. The unskippable epilogue now stamps after the failure digest (idempotent with the finalize path) (#363)
- close the unresolved-model-token fatal launch across every driver + wire the resolvability gate (#364)
- EGPS gate-block diagnostics carry cycle-normalized red-predicate identity — three DISTINCT whole-suite contention flakes (1107/1115/1116) collided into one content-free 'red_count=1' fingerprint and false-tripped the identical-fingerprint breaker (1054/1060 class at the gate-block); semantic names keep real cross-cycle recurrences colliding (adversarial review caught the two-part TestC<cycle>_<Name> shape leaking cycle numbers — normalizer strips C-group and index independently, pinned against the live ac_id corpus). Consumed the auto-filed P0 (fixed here console-first); queued acs-metapredicate-suite-scope 0.84 for the flake-authorship class
- drop gitignored declared paths from staging via check-ignore pre-filter — eval-quality contract puts .evolve/evals/<slug>.md in every test-report, .evolve/* is ignored by design, so git add refused rc=1 and every green cycle aborted (cycle-1101; layer 2 behind the d202aeb6 rc=128 fatal). Adversarial review caught round-1's -z misuse (stdin-only flag → probe failed open on every ship) — plain newline form verified against real git + new real-git integration test
- report-token pathspecs stay repo-relative — './go/bin/evolve' prose became the absolute '/go/bin/evolve' git pathspec (rc=128 'Invalid path /go'), killing every green cycle at stage (cycle-1098, batch-12 wave-0); extractReportPaths normalizes ./ and drops absolute/parent/ellipsis tokens, stagePathspec re-guards manifest entries via Clean-based isRepoRelative, ship error now carries git stderr (was <nil>); queue deterministic-vs-transient stage classification
- quota-wall regex catches 'hit your (usage|weekly) limit' wording — 5th-gen drift ended batch-11 in a 19-cycle exit-81 livelock (cycles 1077-1096, 40min burn each); fixtures pin the REAL cycle-1096 pane string, concatenation-split so Read-tool renderings of the test source can never self-trigger exit-85; queue anchor-hardening + codex/agy parity + go/evolve untracking follow-ups
- persona size budget restored — compress removal-claims block (cycle-107x, +6 lines) and reflow the ADR-0076 pre-flight block; combined 756→750 < 751. Main go workflow was RED post-lane-ship because agents/*.md is invisible to diff-scoped in-lane gates — CLASS fix queued as persona-budget-inlane-gate (third warnship-apicover-ci-gap instance)
- per-failure distinguishers in the verdict-path fallback reason — cycles 1054/1060 (different tasks) collided on the constant string; a third would have false-tripped the identical-fingerprint breaker (#358)
- unskippable abnormal-exit epilogue — every started cycle leaves dossier + digest + coherent state on EVERY return path (cycle-1048: loopAbort skipped finalizeCycle, leaving a 2h stale record and a monitor-invisible failure) (#357)
- diff-scoped apicover enforcement (pre-existing debt WARNs, never blocks a lane — cycle-1048 RC1) + 4-symbol core debt paid with default-tag tests + wave-seed skips console-routed items (batch-7 wave-0 starvation RC4) (#356)
- universal fallback failure-reasons + honest unexplained-failures rule (batch-6 first live halt was a degenerate empty-fingerprint collision across three distinct failures) (#355)
- wire S1 digest + S2 disposition gate on the VERDICT-path retro dispatch (cycle-1046 live gap — single-source ensureFailureDigest shared by both paths; scenario harness recalibrated to the disposition contract) (#354)
- apicover naming floor at build handoff + FailReasons surfacing + topngate advisory (reviewer Block x2 -> all findings fixed; ACS predicates rebound honestly) (#348)

### Documentation

- batches 6-8 outcome rows — breaker first-firing, RC1-RC4 arc, honest-FAIL era + same-day fix ledger
- external-control vs autonomous-completion story — 'You steer, it drives' README section + 4 new landing usage-ladder entries (backlog queueing, route field, fleet width, one-command release)
- reflect July 2026 progress — v22.7.0, 1,040+ cycles, structural trust layer (ADR-0074 typed routing, build floor, graduated remediation, failure disposition) + lessons compendium link

### Other

- Merge branch 'main' of https://github.com/mickeyyaya/evolve-loop
- (dossier) cycle-1134 closeout
- (dossier) cycle-1133 closeout
- Merge branch 'main' of https://github.com/mickeyyaya/evolve-loop
- (dossier) cycle-1130 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1129 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1127 closeout
- (dossier) cycle-1128 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1115 closeout
- (dossier) cycle-1116 closeout
- (dossier) cycle-1113 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1114 closeout
- (dossier) cycle-1112 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1111 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1109 closeout
- (dossier) cycle-1110 closeout
- (dossier) cycle-1108 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1107 closeout
- (dossier) cycle-1105 closeout
- (dossier) cycle-1106 closeout
- (dossier) cycle-1104 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1103 closeout
- (dossier) cycle-1098 closeout
- (dossier) cycle-1097 closeout
- (dossier) cycle-927 closeout
- (dossier) cycle-926 closeout
- (dossier) cycle-925 closeout
- (dossier) cycle-1095 closeout
- (dossier) cycle-1096 closeout
- (dossier) cycle-1093 closeout
- (dossier) cycle-1094 closeout
- (dossier) cycle-1092 closeout
- (dossier) cycle-1091 closeout
- (dossier) cycle-1089 closeout
- (dossier) cycle-1090 closeout
- (dossier) cycle-1088 closeout
- (dossier) cycle-1087 closeout
- (dossier) cycle-1086 closeout
- (dossier) cycle-1085 closeout
- (dossier) cycle-1083 closeout
- (dossier) cycle-1084 closeout
- (dossier) cycle-1081 closeout
- (dossier) cycle-1082 closeout
- (dossier) cycle-1079 closeout
- (dossier) cycle-1080 closeout
- (dossier) cycle-1077 closeout
- (dossier) cycle-1078 closeout
- (dossier) cycle-1075 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1076 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (dossier) cycle-1074 closeout
- (dossier) cycle-1073 closeout
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- (boundary) convergence closeout — inbox cleanup (15 consume-with-resolution incl. the three ADR-0076 slice items + 4 merge-absorptions; 3 keep-verify + 5 reband + sleeper/scope annotations; 76 live items remain), disposition boundary applier shadow→enforce (closes the lesson-to-action gap: retro escalations now mutate the inbox), ADR-0076 status = all four slices implemented, lessons §6b console-implementation addendum
- ADR-0076 D: retry tier escalation (direct console implementation) (#360)
- evolve-cycle: goal=9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
- Merge remote-tracking branch 'origin/main'
- (merge) reconcile origin/main (PR #359 squash) with local batch-8 history + builder-doc export-naming directive + batch-8 boundary state
- Ship staging: staged-deletion pathspec fatal (boundary flow blocked) (#359)
- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-1067 closeout
- evolve-cycle: goal=8162fa7cf80ae5b527e476ad72d217c996be0ea2d3828c45886025acb8efb92c
- (dossier) cycle-1068 closeout
- (dossier) cycle-1065 closeout
- (dossier) cycle-1066 closeout
- (dossier) cycle-1064 closeout
- evolve-cycle: goal=8162fa7cf80ae5b527e476ad72d217c996be0ea2d3828c45886025acb8efb92c
- (dossier) cycle-1063 closeout
- (dossier) cycle-1062 closeout
- evolve-cycle: goal=8162fa7cf80ae5b527e476ad72d217c996be0ea2d3828c45886025acb8efb92c
- (dossier) cycle-1061 closeout
- (dossier) cycle-1060 closeout
- (dossier) cycle-1059 closeout
- (dossier) cycle-1057 closeout
- evolve-cycle: goal=8162fa7cf80ae5b527e476ad72d217c996be0ea2d3828c45886025acb8efb92c
- (dossier) cycle-1058 closeout
- (dossier) cycle-1055 closeout
- (dossier) cycle-1056 closeout
- (dossier) cycle-1054 closeout
- (dossier) cycle-1053 closeout
- evolve-cycle: goal=8162fa7cf80ae5b527e476ad72d217c996be0ea2d3828c45886025acb8efb92c
- docs(policy)+ops(boundary): canonical operating-policy.md (memory rules translated to repo) + AGENTS/CLAUDE pointers + batch-6 halt residue (P0 blocker item, state)
- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-1046 closeout
- Merge remote-tracking branch 'origin/main'
- Role guard: retro lessons-corpus allowance (cycles 1036/1041/1042 salvage) (#351)
- ADR-0074 S1+S2: failure-disposition assembler + contract gate (cycle-1034 salvage) (#350)
- Merge remote-tracking branch 'origin/main'
- docs+ops(boundary): batch-5 closeout — lessons-and-resolutions compendium; ADR-0074 console-route holds released with binding route fields; #348 items consumed
- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-1042 closeout
- (dossier) cycle-1043 closeout
- (dossier) cycle-1041 closeout
- (dossier) cycle-1040 closeout
- (dossier) cycle-1038 closeout
- (dossier) cycle-1036 closeout
- (dossier) cycle-1037 closeout
- (dossier) cycle-1034 closeout
- (dossier) cycle-1035 closeout
- (dossier) cycle-1033 closeout
- evolve-cycle: goal=619be4d8351de28fc25e79d4f8900adb87e77c0478f65330ceac064cac896e85
- (dossier) cycle-1030 closeout
- (dossier) cycle-1029 closeout
- evolve-cycle: goal=619be4d8351de28fc25e79d4f8900adb87e77c0478f65330ceac064cac896e85
- (dossier) cycle-1028 closeout


---

## [22.7.0] - 2026-07-22

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.6.0..HEAD._

### Added

- build handoff floor + graduated remediation — two-layer cure for the discard-sound-work class (#347)

### Fixed

- salvage cycle-1019 — ADR-0072 S5 task-quarantine + coverage-gate-prescribed tests (#346)
- statemap symlink write-through + dual-lineage CAS floor + canonical lock unification — closes the cycle-999/1000/1001 shared-state disease (6 false-failed cycles) (#345)
- fully disable live-tree-mutating apicover reproduction — it regenerated coverage.txt mid-run and poisoned the CI profile (redesign contract in parity item)

### Other

- Merge remote-tracking branch 'origin/main'
- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-1026 closeout
- (dossier) cycle-1025 closeout
- evolve-cycle: goal=d5fa337c9e9b13826c403e4a386fe8ee45cd3061c4ac01118e30eb8737a464c8
- (dossier) cycle-1024 closeout
- (dossier) cycle-1023 closeout
- (dossier) cycle-1022 closeout
- (dossier) cycle-1020 closeout
- (dossier) cycle-1021 closeout
- (dossier) cycle-1019 closeout
- (dossier) cycle-1018 closeout
- (dossier) cycle-1016 closeout
- (dossier) cycle-1017 closeout
- (dossier) cycle-1015 closeout
- evolve-cycle: goal=d5fa337c9e9b13826c403e4a386fe8ee45cd3061c4ac01118e30eb8737a464c8
- (dossier) cycle-1014 closeout
- evolve-cycle: goal=d5fa337c9e9b13826c403e4a386fe8ee45cd3061c4ac01118e30eb8737a464c8
- (dossier) cycle-1013 closeout
- evolve-cycle: goal=d5fa337c9e9b13826c403e4a386fe8ee45cd3061c4ac01118e30eb8737a464c8
- fix(ci)+chore(boundary): apicover-gap red test -> loud skip-tripwire until parity fix; batch-3 inbox state + fail-analysis research
- (dossier) cycle-1010 closeout
- (dossier) cycle-1009 closeout
- (dossier) cycle-1008 closeout
- (dossier) cycle-1007 closeout
- (dossier) cycle-1006 closeout
- (dossier) cycle-1005 closeout
- evolve-cycle: goal=d4069b349f1a99b5ad4324905d3e02d4efd5923fab92d4eec14c2529a839b8d2
- (dossier) cycle-1002 closeout
- evolve-cycle: goal=d4069b349f1a99b5ad4324905d3e02d4efd5923fab92d4eec14c2529a839b8d2
- (dossier) cycle-1003 closeout
- (dossier) cycle-1000 closeout
- (dossier) cycle-1001 closeout
- (dossier) cycle-998 closeout
- evolve-cycle: goal=d4069b349f1a99b5ad4324905d3e02d4efd5923fab92d4eec14c2529a839b8d2
- (dossier) cycle-999 closeout
- (dossier) cycle-997 closeout
- evolve-cycle: goal=d4069b349f1a99b5ad4324905d3e02d4efd5923fab92d4eec14c2529a839b8d2
- (dossier) cycle-996 closeout
- evolve-cycle: goal=d4069b349f1a99b5ad4324905d3e02d4efd5923fab92d4eec14c2529a839b8d2


---

## [22.6.0] - 2026-07-21

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.5.0..HEAD._

### Other

- test(ship)+chore(boundary): LandPrefixes naming test (apicover un-RED) + batch-2 inbox state + token-usage research
- (dossier) cycle-994 closeout
- (dossier) cycle-995 closeout
- (dossier) cycle-992 closeout
- (dossier) cycle-993 closeout
- (dossier) cycle-991 closeout
- (dossier) cycle-990 closeout
- (dossier) cycle-989 closeout
- (dossier) cycle-988 closeout
- (dossier) cycle-987 closeout
- evolve-cycle: goal=1f6d5bf8740b6cc382f1f7f2c1c2ff6be9c5a455a6bb2c3f790980b2b619d893
- (dossier) cycle-986 closeout
- evolve-cycle: goal=1f6d5bf8740b6cc382f1f7f2c1c2ff6be9c5a455a6bb2c3f790980b2b619d893
- (dossier) cycle-984 closeout
- (dossier) cycle-983 closeout
- (dossier) cycle-982 closeout
- evolve-cycle: goal=1f6d5bf8740b6cc382f1f7f2c1c2ff6be9c5a455a6bb2c3f790980b2b619d893
- (dossier) cycle-981 closeout
- evolve-cycle: goal=1f6d5bf8740b6cc382f1f7f2c1c2ff6be9c5a455a6bb2c3f790980b2b619d893
- (dossier) cycle-980 closeout
- evolve-cycle: goal=1f6d5bf8740b6cc382f1f7f2c1c2ff6be9c5a455a6bb2c3f790980b2b619d893


---

## [22.5.0] - 2026-07-20

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.4.2..HEAD._

### Added

- runtime universal CLI fallback + agy-only readiness — route to any installed LLM when the configured chain is absent, don't halt
- salvage cycle-943 from its false-RED audit — COMPACT.md-first materialization + fail-open overlay resolution for non-phase dispatch sites

### Fixed

- cross-lane mint registry — exempt verified advisor mints from tree-diff abort (cycle-967, ADR-0073) (#344)
- machine-stamp scout goal_hash instead of aborting + close apicover gap on 3 orphaned internal/core exports
- integration-tier flake absorption — CI-parity env scrub + serialized retake-on-red (closes the 943/950/955 false-RED class)

### Other

- (dossier) cycle-979 closeout
- evolve-cycle: goal=12dcd294217f923b4de644c32bdece855ff69ef1cd9e67bd0e6b703621e1f4df
- (dossier) cycle-976 closeout
- (dossier) cycle-977 closeout
- evolve-cycle: goal=12dcd294217f923b4de644c32bdece855ff69ef1cd9e67bd0e6b703621e1f4df
- evolve-cycle: goal=12dcd294217f923b4de644c32bdece855ff69ef1cd9e67bd0e6b703621e1f4df
- (dossier) cycle-974 closeout
- evolve-cycle: goal=12dcd294217f923b4de644c32bdece855ff69ef1cd9e67bd0e6b703621e1f4df
- (dossier) cycle-975 closeout
- Merge remote-tracking branch 'origin/main'
- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-971 closeout
- (dossier) cycle-968 closeout
- (dossier) cycle-969 closeout
- evolve-cycle: goal=12dcd294217f923b4de644c32bdece855ff69ef1cd9e67bd0e6b703621e1f4df
- evolve-cycle: goal=12dcd294217f923b4de644c32bdece855ff69ef1cd9e67bd0e6b703621e1f4df
- (dossier) cycle-962 closeout
- evolve-cycle: goal=7d61f31cb0e0a1ac32c3b3c6657f0e02f28d771610ae6c3cd3315c2f2c05376e
- (dossier) cycle-961 closeout
- (dossier) cycle-959 closeout
- evolve-cycle: goal=7d61f31cb0e0a1ac32c3b3c6657f0e02f28d771610ae6c3cd3315c2f2c05376e
- (dossier) cycle-958 closeout
- (dossier) cycle-956 closeout
- evolve-cycle: goal=7d61f31cb0e0a1ac32c3b3c6657f0e02f28d771610ae6c3cd3315c2f2c05376e


---

## [22.4.2] - 2026-07-20

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.4.1..HEAD._

### Fixed

- name the RUNG-2 scoped-merge exports — unbreak main CI (Phase-5 per-symbol gate)
- end the verdict-incoherence false-HALT family — diagnosed gate-downgrades are task-fails, the integration tier stops false-REDding under fleet contention (cycles 930/931/932 + cycle-3)
- ACS deterministic floor at teardown — a stalled auditor's ship-eligible PASS is no longer discarded as FAIL (verdict-incoherence family 603/921/924/931/3)

### Other

- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-955 closeout
- (dossier) cycle-954 closeout
- evolve-cycle: goal=7d61f31cb0e0a1ac32c3b3c6657f0e02f28d771610ae6c3cd3315c2f2c05376e
- (dossier) cycle-951 closeout
- evolve-cycle: goal=7d61f31cb0e0a1ac32c3b3c6657f0e02f28d771610ae6c3cd3315c2f2c05376e
- (dossier) cycle-950 closeout
- (dossier) cycle-949 closeout
- (dossier) cycle-948 closeout
- (dossier) cycle-946 closeout
- evolve-cycle: goal=7d61f31cb0e0a1ac32c3b3c6657f0e02f28d771610ae6c3cd3315c2f2c05376e
- (dossier) cycle-944 closeout
- (dossier) cycle-943 closeout
- (dossier) cycle-941 closeout
- (dossier) cycle-942 closeout
- evolve-cycle: goal=7d61f31cb0e0a1ac32c3b3c6657f0e02f28d771610ae6c3cd3315c2f2c05376e
- evolve-cycle: goal=7d61f31cb0e0a1ac32c3b3c6657f0e02f28d771610ae6c3cd3315c2f2c05376e
- (dossier) cycle-940 closeout
- (dossier) cycle-939 closeout
- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-938 closeout
- (dossier) cycle-937 closeout
- (dossier) cycle-936 closeout
- (dossier) cycle-933 closeout
- (dossier) cycle-932 closeout
- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-931 closeout


---

## [22.4.1] - 2026-07-19

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.4.0..HEAD._

### Added

- config-driven skill-overlay injection for phase agents (#333)

### Fixed

- file-authoritative verdict source — pane never a verdict source for a contracted phase (ADR-0072) (#336)
- widen settle-retry window 3 to 15 so late-flushed deliverables verify (ADR-0072 verdict-incoherence, cycle-921) (#335)

### Other

- evolve-cycle: goal=7d61f31cb0e0a1ac32c3b3c6657f0e02f28d771610ae6c3cd3315c2f2c05376e
- (dossier) cycle-924 closeout
- (dossier) cycle-923 closeout
- (dossier) cycle-921 closeout


---

## [22.4.0] - 2026-07-18

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.3.0..HEAD._

### Added

- wire tier-fallback dispatch — fable/opus→sonnet failover live on exit-85 (#331)
- goal-stall escalation — stop re-dispatching a goal that ships nothing (#330)

### Fixed

- detect per-model quota wall in claude-tmux exhausted_regex (#328)

### Other

- (harden) persistence-gate exhaustion fast-fail + fail-loud drift alarm (#332)


---

## [22.3.0] - 2026-07-17

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.2.0..HEAD._

### Added

- ADR-0072 system-failure policy — halt+diagnose instead of retrying the pipeline (deterministic floor S1-S3+S6)

### Fixed

- graduate internal/coherence + name ADR-0072 exported symbols + queue campaign follow-ups (CI green)
- drop orphan EVOLVE_STRICT_AUDIT token from v21.0 changelog row — describe it as the strict-audit gate dial (fixes ACS flagreaders guard that failed CI on b3e56a3c and blocked the v22.2.1 release preflight)
- clean-exit-idle deliverable-authority — settle-retry the on-disk verdict on the clean-exit path (fixes false-FAIL that discarded ≥10 valid PASS cycles 877-899; robust verdict-authority test matrix) + recover cycle-899 MergeFallbacks (preserve operator tier_fallbacks on 'models refresh') that this bug false-FAILed
- read bare-string transcript content in firstUserText so ArtifactPath token attribution actually fires (c41fa94b shipped inert) — recovers real per-phase input/cache_read
- attribute claude token telemetry by ArtifactPath — recover per-phase input/cache_read (was recorded all-zeros) + token-opt campaign research/follow-ups
- bound integration-tier gate -p/-parallel to defuse the EBADF/OOM footprint — the per-cycle gate forks go test -race whose concurrent git subprocess spawning races on pipe FDs (EBADF Path:"|0") and spikes race-detector memory until clean isolated tests fail at 0.00s; capping -p/-parallel shrinks that footprint (esp. on the whole-suite fallback). Complements the scoping fix; CI stays unbounded. (Dropped a no-op RLIMIT_NOFILE raise: Go 1.19+ already maxes it and EBADF is a spawn race, not a limit.)
- scope audit integration-tier gate to touched packages — the per-cycle gate ran the whole go test -race -tags integration ./... suite, whose parallel-unsafe env-dependent tests (TestFleetSoak, TestShipFromWorktree) flaked EVERY cycle under the loop's contended local env (concurrent fleet lanes + real tmux/git), producing false-RED audits and zero ships since v22.2.0; now scopes to changedPackagesForAudit (mirrors apicover-enforce), CI keeps the whole-suite backstop. Also queues 3 inbox follow-ups incl. the reviewer-surfaced cycleTouchedGo derivability gap
- deliverable-authority ctx-cancel -1 false-FAIL (cycle-859) — engine maps a ctx-cancel SIGKILL (exit -1, ctx.Err()!=nil) to a transient teardown so the runner reconcile door consults the green-ACS PASS deliverable instead of hard-failing via the substantive-error door

### Changed

- extend clean-boot to remaining heavy claude phases (coverage-gate/error-handling-scan/flake-rerun-scan/bug-reproduction/adversarial-review/builder) — v1 validated -39% per-turn base on fault-localization with quality intact
- clean-boot flags for heavy claude phases (fault-localization/secret-leak-scan/tdd-engineer) — drop MCP+skills schemas from the per-turn base (~22% cache_read cut, proven pattern used by scout/auditor)

### Documentation

- prep version table for v22.3.0 release — Current marker v22.3 + fold never-released v22.2.1 hotfix into a v22.3 row
- runtime-reference system-failure-halt row + false-FAIL recovery ledger; retire completed S7 inbox item
- fill in the Version table (was all TBD) with per-release highlights v20.4→v22.2 + add the v22.2.1 hotfix row
- reflect rich context — README 'run lean on context' capability (~39% fewer context tokens/cycle) + landing proofBar 620+->890+ cycles; GitHub description+homepage updated separately
- re-apply README part4/5 index + part5 ADR-0002 correction + strengthen churn-discard GOTCHA (edits were reverted by loop churn-discard on the prior ship — tracked-file edits mid-loop are reverted, not just untracked)
- part5 — comprehensive token-optimization campaign implementation record (telemetry fix, clean-boot B-v1/v2/v3, per-CLI investigation, Slice C design, GOTCHAs)

### Other

- (dossier) cycle-898 closeout
- (dossier) cycle-899 closeout
- (dossier) cycle-897 closeout
- (dossier) cycle-896 closeout
- (dossier) cycle-895 closeout
- (dossier) cycle-894 closeout
- docs+fix(tokens): doc-audit cross-references (floor-history Campaign E, context-window-control, README part4/5 index, ADR-0071) + correct B-v3 skill flag per ADR-0002 (restore --disable-slash-commands on bug-reproduction/adversarial-review; queue empirical skill verification)
- (dossier) cycle-889 closeout
- perf(tokens) B-v3: per-phase --tools whitelist (secret-leak-scan/error-handling-scan/fault-localization, ~37% additional base cut) + preserve skills on bug-reproduction/adversarial-review (drop --disable-slash-commands)
- (dossier) cycle-884 closeout
- (dossier) cycle-885 closeout
- (dossier) cycle-883 closeout
- (dossier) cycle-882 closeout
- (dossier) cycle-877 closeout
- (dossier) cycle-876 closeout
- (dossier) cycle-872 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-870 closeout
- (dossier) cycle-871 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-869 closeout
- (dossier) cycle-866 closeout
- (dossier) cycle-867 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-862 closeout
- (dossier) cycle-859 closeout
- (dossier) cycle-858 closeout


---

## [22.2.0] - 2026-07-16

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.1.0..HEAD._

### Fixed

- verdict-less acs-verdict degrades loudly + schema-faithful e2e fake audit artifact
- arm the spine artifact floor + close static/dynamic boundary leaks + inbox batch closures
- learn floor-phase gate-FAIL verdicts + drop self-defeating retro/memo repoint task (P0 storm)
- reconcile on-disk deliverable on transient bridge teardown (quota exit 85), not just artifact-timeout
- manifest-gate bare-root-file path extraction (ship-stage enforce prerequisite #1) — pathToken now also extracts declared bare root filenames (CHANGELOG.md, go.mod, go.sum) via an extension allow-list, so a legit root-file change is no longer a false out-of-manifest warning (shadow) / false-block (enforce). Adds a left-boundary check (isPathContinuationByte) so leading-dot files (.goreleaser.yml) are a CLEAN non-match, not a silent truncation. Regression tests + both reviewers Approve.
- swarm registry persist-then-rollback transactionality — Register/MarkReaped now snapshot the sessions slice before mutating and roll back on persist failure, so a mutation that never reached the crash-safe manifest is a no-op in memory too (no phantom Live entry contradicting the on-disk source of truth). Closes the go-reviewer find from the session-accounting fail-loud fix. Adds 3 rollback tests (register-new, markreaped-flip, register-replace) with real persist-failure injection.
- swarm session-accounting fail-loud — Register failure aborts the worker before spawn (no unregistered orphan); MarkReaped failure surfaced in ReapReport.Errors (no swallowed stale-Live manifest entry). Fixes the two _= error swallows at dispatcher.go and kill.go; adds real-injection tests + corrects stale Dispatch orphan-on-cancel doc.
- ship-hygiene manifest-gate (shadow default) — cross-lane leak guard
- make bridge Engine.Launch OnBoot capture call-local (concurrency-safe)
- serialize cycle-dossier commit on shared ship.lock
- settle-retry reconcile so a session-cancel timeout cannot discard a settling deliverable
- name SwarmResult.TotalTokens in swarm apicover suite (repo-wide enforce was masked by the test-step red)
- hermetic syncmain fixture (bare-HEAD pin + clone identity) + FlagCeiling 25 for C29_007 forbidden-repeat row

### Changed

- phaseobserver single-goroutine hygiene — remove the dead mutex (Observer is constructed and driven only by Run's single poll-loop goroutine, never shared; caller-trace + both reviewers confirm) and hoist the stop-timer channel into a nil-channel stopChan helper (kills a per-poll-iteration make(chan) allocation). Documents the single-goroutine contract; adds stopChan tests (incl. AllocsPerRun==0). Behavior-preserving.
- runlease per-instance write seams (concurrency-hygiene) — replace mutable package-level createTempFn/renameFn globals with an unexported writer struct (seams as fields, newWriter() real defaults); package Write delegates. Eliminates the cross-test race that was one t.Parallel() away; seam tests now run parallel. Behavior-preserving (byte-for-byte write body); adds per-instance isolation test.

### Other

- (landing) bump content.json product.version to v22.2.0 (manual marker outside the release-pipeline auto-bump list)
- (dossier) cycle-854 closeout
- (dossier) cycle-853 closeout
- (dossier) cycle-852 closeout
- (dossier) cycle-851 closeout
- (dossier) cycle-848 closeout
- (dossier) cycle-849 closeout
- (dossier) cycle-846 closeout
- (dossier) cycle-847 closeout
- (dossier) cycle-844 closeout
- (dossier) cycle-845 closeout
- (dossier) cycle-843 closeout
- (dossier) cycle-842 closeout
- (dossier) cycle-841 closeout
- (dossier) cycle-840 closeout
- (dossier) cycle-836 closeout
- (dossier) cycle-837 closeout
- (dossier) cycle-834 closeout
- (dossier) cycle-835 closeout
- one-binary S5: release classification (binary vs config) + corporate-deployment docs
- one-binary S4: --binary air-gap install mode + fingerprint echo
- one-binary S3: deployed-mode no-rebuild durable guard
- one-binary S2: universal macOS binary + checksums manifest + fingerprints
- one-binary S1: fold apicover into the evolve binary as internal/apicover
- (skillcheck) plugin.json<->skills bijection guard (ManifestProblems) wired into Run+Check
- (dossier) cycle-832 closeout
- (policy) revert fleet width 1->2 (count=1 drops fleet mode + halts on first FAIL) + require live-wiring proof on the 2 verdict-surface fixes
- (dossier) cycle-831 closeout
- (policy) fleet width 2->1 (operator accelerate) + escalate 3 verdict-surface defects from width-2 storm retros
- (dossier) cycle-830 closeout
- (dossier) cycle-827 closeout
- (dossier) cycle-828 closeout
- (dossier) cycle-825 closeout
- (dossier) cycle-826 closeout
- (dossier) cycle-823 closeout
- (dossier) cycle-824 closeout
- (policy) fleet width 3->2 + prioritize rebase/merge tasks (operator bounce)
- (dossier) cycle-819 closeout
- (dossier) cycle-817 closeout
- (dossier) cycle-815 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-816 closeout
- (dossier) cycle-814 closeout
- (dossier) cycle-811 closeout
- (dossier) cycle-813 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-812 closeout
- (dossier) cycle-808 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-809 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-810 closeout
- (dossier) cycle-806 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-807 closeout
- (dossier) cycle-805 closeout
- (dossier) cycle-802 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-804 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-801 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-800 closeout
- (dossier) cycle-792 closeout
- (dossier) cycle-787 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-786 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-784 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-782 closeout
- (dossier) cycle-783 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (research) merge-concurrency-2026 synthesis (industrial queues + agent fleets) — drives merge-efficiency-2026-07 campaign
- (dossier) cycle-779 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-778 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-780 closeout
- (dossier) cycle-776 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-775 closeout
- (dossier) cycle-772 closeout
- (dossier) cycle-773 closeout
- (dossier) cycle-774 closeout
- (landing) gallery footer renders shared footer links (explainers link on root page)
- (dossier) cycle-769 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-771 closeout
- (dossier) cycle-770 closeout
- docs+skills+landing: /evo:explain skill, 12 verified docs/explain pages, site projection to /explain/ + footer link (reviewed; drift queued as inbox items)
- (dossier) cycle-767 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-766 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-768 closeout
- (dossier) cycle-763 closeout
- (dossier) cycle-765 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-764 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-757 closeout
- (dossier) cycle-759 closeout
- (dossier) cycle-754 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-756 closeout
- (dossier) cycle-751 closeout
- (dossier) cycle-752 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-748 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-750 closeout
- (dossier) cycle-749 closeout
- (dossier) cycle-745 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-746 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-744 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-739 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-740 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-741 closeout
- (policy) standing fleet width 3 (end 10-lane burn window) + width-incident defect filings
- (dossier) cycle-737 closeout
- (dossier) cycle-734 closeout
- (dossier) cycle-720 closeout
- (dossier) cycle-719 closeout
- (dossier) cycle-724 closeout
- (dossier) cycle-726 closeout
- (dossier) cycle-703 closeout
- (dossier) cycle-705 closeout
- (policy) assert min_lanes=10 — hold fleet width 10 under CLI-family benches (operator)
- (dossier) cycle-702 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-701 closeout
- (dossier) cycle-706 closeout
- Merge remote-tracking branch 'origin/main'
- (policy) fleet width 10 (operator, until 21:00) + crash-boundary state sweep
- (dossier) cycle-697 closeout
- (dossier) cycle-699 closeout
- (dossier) cycle-694 closeout
- Merge pull request #326 from mickeyyaya/fix/ci-syncmain-branch-flagceiling
- (dossier) cycle-695 closeout
- (dossier) cycle-696 closeout
- (dossier) cycle-691 closeout
- (dossier) cycle-692 closeout
- (dossier) cycle-693 closeout
- (dossier) cycle-688 closeout
- evolve-cycle: goal=6588b51f58ccef5d0649a40b8bcae2d25a69cccf3c2aac59f4d1dd8ca5fdaf20
- (dossier) cycle-689 closeout
- (dossier) cycle-690 closeout
- (dossier) cycle-686 closeout
- (dossier) cycle-685 closeout
- (dossier) cycle-687 closeout
- (config) fleet min_lanes=3 — hold 3 concurrent lanes regardless of family benches (operator directive 2026-07-11)
- (dossier) cycle-682 closeout
- evolve-cycle: goal=b576bc3838c430b5dd8d663213bc8a4b1da83c746806cb66a68a00e90d307560
- (dossier) cycle-681 closeout
- (dossier) cycle-680 closeout
- evolve-cycle: goal=b576bc3838c430b5dd8d663213bc8a4b1da83c746806cb66a68a00e90d307560
- (dossier) cycle-679 closeout
- (dossier) cycle-678 closeout
- evolve-cycle: goal=b576bc3838c430b5dd8d663213bc8a4b1da83c746806cb66a68a00e90d307560
- (dossier) cycle-677 closeout
- (dossier) cycle-675 closeout
- evolve-cycle: goal=b576bc3838c430b5dd8d663213bc8a4b1da83c746806cb66a68a00e90d307560
- (dossier) cycle-676 closeout
- (dossier) cycle-673 closeout
- (dossier) cycle-674 closeout
- (dossier) cycle-672 closeout
- evolve-cycle: goal=b576bc3838c430b5dd8d663213bc8a4b1da83c746806cb66a68a00e90d307560
- (dossier) cycle-671 closeout
- (dossier) cycle-669 closeout
- evolve-cycle: goal=b576bc3838c430b5dd8d663213bc8a4b1da83c746806cb66a68a00e90d307560
- (dossier) cycle-670 closeout
- Merge remote-tracking branch 'origin/main'
- (config) fleet 3-wide (count=3, min_lanes=2 floor) — operator directive 2026-07-11
- (dossier) cycle-666 closeout
- (dossier) cycle-667 closeout
- evolve-cycle: goal=e9e3726bc890981586ba43ee19d857d36c3102ba17a35ee84771c37d7805432e
- (dossier) cycle-664 closeout
- evolve-cycle: goal=e9e3726bc890981586ba43ee19d857d36c3102ba17a35ee84771c37d7805432e
- (dossier) cycle-665 closeout
- (dossier) cycle-662 closeout
- evolve-cycle: goal=e9e3726bc890981586ba43ee19d857d36c3102ba17a35ee84771c37d7805432e
- (dossier) cycle-663 closeout
- (dossier) cycle-661 closeout
- evolve-cycle: goal=e9e3726bc890981586ba43ee19d857d36c3102ba17a35ee84771c37d7805432e
- (dossier) cycle-660 closeout
- (dossier) cycle-659 closeout
- evolve-cycle: goal=e9e3726bc890981586ba43ee19d857d36c3102ba17a35ee84771c37d7805432e
- (dossier) cycle-658 closeout
- (dossier) cycle-657 closeout
- evolve-cycle: goal=e9e3726bc890981586ba43ee19d857d36c3102ba17a35ee84771c37d7805432e
- (dossier) cycle-654 closeout
- evolve-cycle: goal=e9e3726bc890981586ba43ee19d857d36c3102ba17a35ee84771c37d7805432e
- (dossier) cycle-655 closeout
- evolve-cycle: goal=e9e3726bc890981586ba43ee19d857d36c3102ba17a35ee84771c37d7805432e
- (dossier) cycle-653 closeout
- (dossier) cycle-652 closeout
- (dossier) cycle-648 closeout
- (dossier) cycle-649 closeout
- (dossier) cycle-646 closeout
- (dossier) cycle-647 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-645 closeout
- (dossier) cycle-644 closeout
- (dossier) cycle-642 closeout
- (dossier) cycle-643 closeout
- (dossier) cycle-640 closeout
- (dossier) cycle-639 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-638 closeout
- (dossier) cycle-637 closeout
- (dossier) cycle-636 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- Merge pull request #325 from mickeyyaya/chore/landing-version-22.1.0


---

## [22.1.0] - 2026-07-08

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.0.1..HEAD._

### Added

- labs auto-play on scroll with permanent user handoff
- interactive labs + try-it conversion spine + hardened one-line installer
- fable-simulation — thinking layer + per-model adaptations (Opus 4.8, GPT-5.5, Gemini 3.5 Flash) + design & research
- engineering-craft — TDD, clean-code, and design-pattern discipline skill (2026 evidence-based)
- generalize fable-mode to all LLM CLIs (codex, agy, ollama, AGENTS.md trees)
- fable-mode deep-dive references — six operating playbooks (investigation, design-review, communication, orchestration, judgment, verification)
- fable-mode — Fable 5 operating-discipline skill for non-Fable tiers

### Fixed

- hermetic leak-test repos + apicover naming tests for 7 flagged exports

### Changed

- single conversion block, unified self-healing story, narrative section order
- rename fable-mode -> fable (/evo:fable) — mechanical, drift+overlay+publish tests green
- generalize fable-mode — one skill for all LLMs, capability-class COMPACT, vendor residue to config (operator direction: tiers not model names)

### Documentation

- code-audit 2026-07 research (six-lens scan, telemetry post-mortem, deadcode snapshot) + test-coverage audit + token-telemetry outcome pointer

### Other

- Merge remote-tracking branch 'origin/main'
- Merge pull request #324 from mickeyyaya/feat/labs-autoplay
- Merge pull request #323 from mickeyyaya/fix/landing-coherence
- (dossier) cycle-633 closeout
- Merge pull request #322 from mickeyyaya/feat/landing-interactive-labs
- (dossier) cycle-631 closeout
- (dossier) cycle-624 closeout
- (dossier) cycle-622 closeout
- (dossier) cycle-623 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-620 closeout
- (dossier) cycle-621 closeout
- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-618 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-619 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-617 closeout
- (dossier) cycle-616 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-615 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-614 closeout
- (dossier) cycle-613 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- Merge pull request #321 from mickeyyaya/feat/rename-fable
- Merge pull request #320 from mickeyyaya/feat/fable-mode-generalize
- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-611 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-610 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- Merge pull request #319 from mickeyyaya/feat/fable-simulation-deep
- (dossier) cycle-609 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-608 closeout
- (dossier) cycle-607 closeout
- (dossier) cycle-606 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-604 closeout
- (dossier) cycle-605 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-603 closeout
- (dossier) cycle-602 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-601 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-599 closeout
- (dossier) cycle-598 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- Merge pull request #318 from mickeyyaya/feat/engineering-craft-skill
- (dossier) cycle-596 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-597 closeout
- (dossier) cycle-595 closeout
- (dossier) cycle-594 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-593 closeout
- (dossier) cycle-592 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-590 closeout
- (dossier) cycle-591 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-589 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-588 closeout
- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-587 closeout
- Merge pull request #317 from mickeyyaya/feat/fable-mode-all-clis
- Merge pull request #316 from mickeyyaya/feat/fable-mode-references
- (dossier) cycle-586 closeout
- (dossier) cycle-584 closeout
- (dossier) cycle-585 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-583 closeout
- (dossier) cycle-582 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-581 closeout
- (dossier) cycle-580 closeout
- Merge pull request #315 from mickeyyaya/fix/fable-mode-skill
- Merge remote-tracking branch 'origin/main'
- (dossier) cycle-576 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-577 closeout
- (dossier) cycle-574 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-575 closeout
- (dossier) cycle-573 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- Merge pull request #314 from mickeyyaya/fix/ci-remote-parity
- (dossier) cycle-572 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-568 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-567 closeout
- (dossier) cycle-564 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-563 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-562 closeout
- (dossier) cycle-561 closeout
- evolve-cycle 561: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-559 closeout
- (dossier) cycle-558 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-557 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-554 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-555 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-553 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-552 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-550 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-551 closeout
- (dossier) cycle-549 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-548 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-547 closeout
- (dossier) cycle-546 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-545 closeout
- (dossier) cycle-544 closeout
- evolve-cycle 544: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-543 closeout
- (dossier) cycle-541 closeout
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- (dossier) cycle-542 closeout
- evolve-cycle 539: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle 536: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55


---

## [22.0.1] - 2026-07-06

_Generated by `evolve release` (go/internal/changeloggen) from commits v22.0.0..HEAD._

### Fixed

- add "top" canonical tier to swappable driver manifests (#313)

### Other

- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55


---

## [22.0.0] - 2026-07-05

_Generated by `evolve release` (go/internal/changeloggen) from commits v21.9.0..HEAD._

### Fixed

- seed the wave from the inbox when there is no prior triage decision
- thread goal-hash into wave lanes so 2-wide dispatches (no sequential fallback)
- record C1 chokepoint escape on RunCycle transition-cycle exit

### Other

- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle 516: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle 514: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle 507: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle 488: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55


---

## [21.9.0] - 2026-07-03

_Generated by `evolve release` (go/internal/changeloggen) from commits v21.8.0..HEAD._

### Added

- Q4 shadow-wire fleetbudget into the live fleet wave (#308)


---

## [21.8.0] - 2026-07-03

_Generated by `evolve release` (go/internal/changeloggen) from commits v21.7.0..HEAD._

### Added

- Q3 fleetbudget — pure lane allocator from measured quota + pace (#306)
- Q2 budgethistory — roll last-N cycles into a duration-based Throughput (#305)
- Q1 quotastate — parse CLI /usage into structured QuotaState (#304)
- min_lanes capacity floor — keep 2-wide on a healthy family through a transient CLI bench (#303)
- T2 advisor tier-emission determinism + model-policy (advisor decides, lean Sonnet 5) (#301)

### Fixed

- parse machine-readable audit verdict marker; stop false accept/block (#307)
- per-run cycle-state isolation — stop the singleton clobber that stalled lanes before audit (#302)

### Other

- evolve-cycle 480: goal=4704d60a68863a4398346bf296be2eb6c60ec7747c336df7ff076e874d4fb564


---

## [21.7.0] - 2026-07-03

_Generated by `evolve release` (go/internal/changeloggen) from commits v21.6.0..HEAD._

### Added

- S3 wave guards — dirty-control-plane preflight + quota-aware count (salvaged cycle-467) (#299)
- S2 wave semantics — policy-driven parallel waves in evolve loop (salvaged cycle-466) (#298)
- agy model channel (noop→flag) + Gemini-only tiers + model-tier matrix parity pin (#297)

### Fixed

- bounded flake retry-once in EGPS verdict generation (ADR-0064 operator path) (#300)

### Other

- evolve-cycle 464: goal=28b17c8ccd469b866bf8485f8398dc2926dfb27f5e7842aa9034ffd13fdce609
- evolve-cycle 463: goal=6d39405b16cebfdc4928e4f4494d555a7397ccb861062e3025a2e550b1db037c
- evolve-cycle 459: goal=f9d584d46f6e24ba763f52fa78674cab9ec94fbaa2af12d8c7f7135c1e9c21f4


---

## [21.6.0] - 2026-07-02

_Generated by `evolve release` (go/internal/changeloggen) from commits v21.5.0..HEAD._

### Added

- model_routing axis + advisor {cli,tier} schema + clamp (MR1-MR3) (#293)
- SignalCenter Facade — unified concurrency-safe liveness signal hub (S2) (#291)

### Fixed

- fast-fail a mid-phase CLI exhaustion wall on the ~2s poll, not the 300s checkpoint (#296)
- detect CLI quota/rate-limit exhaustion in the unified SignalCenter (ADR-0070) (#295)
- cycle CI-parity gate — run whole-repo CI checks before ship (ADR-0069) (#294)
- dispatch through the cli_fallback chain — stop degrading every cycle (A1/A2) (#292)
- name LivenessProbe + detector types for apicover -enforce (unbreak main go CI) (#290)

### Other

- evolve-cycle 442: goal=259df99dbc665ab426c972a63e80a129cf2cd2dba5a9568c2b2eac0405ed2388
- evolve-cycle 440: goal=b17855662370871400e2d044ab4c104dc7d19c940d872b63892330b0bca98049
- evolve-cycle 434: goal=aceb01835f2c8df46c16628d7fe0630b945bf15669c965afedd19f38c826e4fd
- evolve-cycle 433: goal=aceb01835f2c8df46c16628d7fe0630b945bf15669c965afedd19f38c826e4fd
- evolve-cycle 432: goal=aceb01835f2c8df46c16628d7fe0630b945bf15669c965afedd19f38c826e4fd
- evolve-cycle 431: goal=aceb01835f2c8df46c16628d7fe0630b945bf15669c965afedd19f38c826e4fd
- evolve-cycle 429: goal=ab1810624fef267f39169ac5064cf17a2c300c5c4a3849cd6e0e4b07860bfe52
- evolve-cycle 427: goal=001b680fc0a7249a3979a0da2c4640828b9d69831d97cc263e0526503e68332e
- evolve-cycle 426: goal=001b680fc0a7249a3979a0da2c4640828b9d69831d97cc263e0526503e68332e
- evolve-cycle 425: goal=001b680fc0a7249a3979a0da2c4640828b9d69831d97cc263e0526503e68332e
- evolve-cycle 424: goal=001b680fc0a7249a3979a0da2c4640828b9d69831d97cc263e0526503e68332e
- evolve-cycle 423: goal=001b680fc0a7249a3979a0da2c4640828b9d69831d97cc263e0526503e68332e


---

## [21.5.0] - 2026-06-30

_Generated by `evolve release` (go/internal/changeloggen) from commits v21.4.5..HEAD._

### Fixed

- keep cycle-85 predicate-quality safeguard in compact mode
- fire-once cli_feedback_rating prompt + deterministic tmux-socket tests

### Documentation

- align skill-naming rule 5 with ADR-0067 (commands re-introduced)
- record command-surface re-introduction (ADR-0067), resolve ADR-0040 contradiction
- record cross-CLI plugin install architecture (ADR-0066)

### Other

- evolve-cycle 422: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle 421: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle 420: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle 419: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- Merge pull request #289 from mickeyyaya/fix/tdd-predicate-quality-safeguard-above-marker
- evolve-cycle 417: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle 416: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle 415: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- Merge pull request #288 from mickeyyaya/fix/bridge-cli-feedback-and-tmux-socket-test
- evolve-cycle 413: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle 412: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- evolve-cycle 411: goal=805f6cedd62d9c2b3592ec1750943ec1bf238e920f34884edead2205d01d7d55
- Merge pull request #287 from mickeyyaya/docs/command-surface-consistency
- Merge pull request #286 from mickeyyaya/docs/command-surface-adr
- Merge pull request #285 from mickeyyaya/docs/cross-cli-install-adr


---

## [21.4.5] - 2026-06-29

_Generated by `evolve release` (go/internal/changeloggen) from commits v21.4.4..HEAD._

### Fixed

- stage full agy skill trees + prune stale pre-rename plugin

### Other

- Merge pull request #284 from mickeyyaya/fix/agy-publish-reference-files


---

## [21.4.4] - 2026-06-29

_Generated by `evolve release` (go/internal/changeloggen) from commits v21.4.3..HEAD._

### Fixed

- stage .codex-plugin in release commits via Paths.Files() SSOT

### Other

- Merge pull request #283 from mickeyyaya/fix/release-stage-codex


---

## [21.4.3] - 2026-06-29

_Generated by `evolve release` (go/internal/changeloggen) from commits v21.4.2..HEAD._

### Fixed

- sync codex mirror to 21.4.2 + guard codex drift at release

### Other

- Merge pull request #282 from mickeyyaya/fix/codex-version-sync-guard


---

## [21.4.2] - 2026-06-29

_Generated by `evolve release` (go/internal/changeloggen) from commits v21.4.1..HEAD._

### Added

- durable Codex install via projected manifests + version sync

### Other

- Merge pull request #281 from mickeyyaya/feat/codex-durable-install


---

## [21.4.1] - 2026-06-29

_Generated by `evolve release` (go/internal/changeloggen) from commits v21.4.0..HEAD._

### Fixed

- drop binaries/compatibility from CC manifests (fixes 2.1.195 install)

### Other

- Revert "release: v21.4.1"
- Merge pull request #280 from mickeyyaya/fix/cc2195-plugin-manifest-schema


---

## [21.4.0] - 2026-06-29

_Generated by `evolve release` (go/internal/changeloggen) from commits v21.3.1..HEAD._

### Fixed

- project skills to bare commands/<name>.md so /evo:<name> surfaces natively on Claude Code 2.1.195
- force sandbox network for cloud-CLI phases (structural, custom-proof)
- parse profile.sandbox.allow_network (silently dropped) + tdd-engineer true
- promote codex-cli compatibility tier to projected
- increase test coverage for releasepipeline, selfsha, skillcheck, and naminguard
- increase test coverage for acssuite and ledger
- sync retro phase facts after agy routing
- route stalled phases to agy
- unblock tmux loop under managed sandbox

### Other

- Merge pull request #278 from mickeyyaya/fix/evo-colon-command-namespace
- Merge pull request #279 from mickeyyaya/fix/apicover-profilesandbox-coverage


---

## [21.3.1] - 2026-06-26

_Generated by `evolve release` (go/internal/changeloggen) from commits v21.3.0..HEAD._

### Fixed

- dismiss claude v2.1.193 boot trust dialog before prompt delivery

### Other

- Merge pull request #277 from mickeyyaya/fix/claude-tmux-prompt-delivery


---

## [21.3.0] - 2026-06-26

_Generated by `evolve release` (go/internal/changeloggen) from commits v21.2.1..HEAD._

### Added

- project each skill into a commands/ stub so /evo:<name> surfaces in the plugin menu

### Other

- Merge pull request #276 from mickeyyaya/feat/skill-command-projection


---

## [21.2.1] - 2026-06-26

_Generated by `evolve release` (go/internal/changeloggen) from commits v21.2.0..HEAD._

### Fixed

- register EVOLVE_TMUX_SOCKET + TMUX_TMPDIR env readers (F6 follow-up)

### Documentation

- regenerate control-flags.md for EVOLVE_TMUX_SOCKET

### Other

- Merge pull request #275 from mickeyyaya/fix/register-tmux-env


---

## [21.2.0] - 2026-06-26

_Generated by `evolve release` (go/internal/changeloggen) from commits v21.1.1..HEAD._

### Added

- per-run tmux socket so an external kill-server can't cross runs
- lease-fencing so a sibling can't seal/kill a running loop
- map-reduce decomposition foundation (S0-S3, dormant)
- opt-in write-canary verifies nested fallback (S5)
- crash-recovery orphan tmux session GC
- add dormant sandbox.nested_fallback dial (S4)
- honest UNCONFINED WARN, not 'degrades gracefully' (S3)
- report MEASURED sandbox capability, not the nested guess (S2)
- measure OS-sandbox capability instead of guessing (S1)
- concurrent post-build evaluate-phase dispatcher (PR2b, dormant)
- evolve reset-sha command — operator surface for the re-pin (ADR-0065, S6-wire)
- guarded ship-sha re-pin for resume (ADR-0065, S6 core)
- production DigestSource for the per-phase chain (ADR-0065, S5a)
- shadow projection of parallelizing independent checking phases
- concurrency-safe per-phase integrity capture (ADR-0065, S4 core)
- per-phase agent-block foundation (ADR-0065)
- per-phase wall-clock + archetype evidence for cycle latency
- config-driven legacy-name guard (evolve names + legacynames gate)
- animated "run several loops at once" concurrency section
- release-verify-binaries gate + release.yml test/verify jobs (S2+S3) (#252)
- add release-verify-clis cross-CLI install+perform gate (#251)
- make --cycles optional — advisor decides cycle count by default

### Fixed

- pair evolve-scout-scan with a hardened scout-scan profile
- name ProvenanceVerified + RepinResult for apicover -enforce
- name ParallelProjection type for the apicover enforce gate
- graduate internal/phasetiming into the apicover public-API gate
- codex pin resolves to tmux driver; mapCodexModel accepts canonical tiers
- make every pipeline phase card opaque so the connector line stays hidden
- hide pipeline connector line behind phase cards
- remove cost-budget flags from the parameter surface
- disable cost-budget feature visibly; drop it from docs/web
- responsive layout pass — no mobile horizontal overflow
- verify prebuilt binaries as part of the release flow (#245)

### Documentation

- document measure-don't-guess + verified-fallback campaign
- reorder parallelism section, refresh examples, drop version chip
- remove unsupported cost-budget feature from the skill doc
- progressive usage examples + simple setup guide (README + web)
- namespace all skill command refs as /evo:* + enforce via acs gate (#248)

### Other

- Merge pull request #274 from mickeyyaya/feat/loop-tmux-socket
- Merge pull request #273 from mickeyyaya/feat/loop-lease-fencing
- Merge pull request #270 from mickeyyaya/feat/scout-mapreduce
- Merge pull request #269 from mickeyyaya/feat/parallel-evaluate-enforce
- Merge pull request #265 from mickeyyaya/feat/parallel-evaluate-projection
- Merge pull request #263 from mickeyyaya/feat/phase-timing-evidence
- Merge pull request #272 from mickeyyaya/feat/sandbox-capability-probe
- Merge pull request #271 from mickeyyaya/feat/tmux-session-gc
- evolve-cycle 391: goal=4c09c1afde63d16e49d727ef329b3036e8b6a20e690ec88f52aef24b2da284ca
- evolve-cycle 389: goal=4c09c1afde63d16e49d727ef329b3036e8b6a20e690ec88f52aef24b2da284ca
- evolve-cycle 388: goal=4c09c1afde63d16e49d727ef329b3036e8b6a20e690ec88f52aef24b2da284ca
- evolve-cycle 387: goal=4c09c1afde63d16e49d727ef329b3036e8b6a20e690ec88f52aef24b2da284ca
- Merge pull request #268 from mickeyyaya/feat/integrity-resetsha-cmd
- Merge pull request #267 from mickeyyaya/fix/phaseintegrity-apicover-naming
- Merge pull request #266 from mickeyyaya/feat/per-phase-integrity-s5
- Merge pull request #264 from mickeyyaya/feat/per-phase-integrity
- Merge pull request #262 from mickeyyaya/fix/codex-resilience-pins-fallback
- Merge pull request #261 from mickeyyaya/fix/codex-pin-driver-tier-routing
- Merge pull request #260 from mickeyyaya/fix/landing-pipeline-line-robust
- Merge pull request #259 from mickeyyaya/chore/remove-legacy-evolve-project-skill
- Merge pull request #258 from mickeyyaya/fix/landing-pipeline-line-behind-nodes
- Merge pull request #257 from mickeyyaya/feat/legacy-name-guard
- Merge pull request #256 from mickeyyaya/docs/landing-reorder-version-cleanup
- Merge pull request #255 from mickeyyaya/fix/disable-budget-cost-params
- Merge pull request #254 from mickeyyaya/docs/concurrency-pitch
- Merge pull request #253 from mickeyyaya/fix/loop-skill-drop-budget
- Merge pull request #250 from mickeyyaya/docs/loop-examples-setup
- Merge pull request #249 from mickeyyaya/feat/cycles-optional-advisor
- Merge pull request #246 from mickeyyaya/feat/pipeline-demo-responsive


---

## [21.1.1] - 2026-06-24

_Generated by `evolve release` (go/internal/changeloggen) from commits v21.0.0..HEAD._

### Added

- broaden prebuilt matrix to 13 Unix targets + install.sh detection (#243)
- looping advisor pipeline demo with per-phase LLM routing
- Windows/WSL2 guidance in install.sh + docs (#240)
- one-line curl|sh installer (prebuilt-first, build fallback, auto-deps) (#239)
- auto-detecting /evo:setup with one-preset model config

### Fixed

- goreleaser before-hooks — go -C, drop redundant race test (#244)

### Documentation

- move Quick Start above "The problem it solves"

### Other

- Merge pull request #242 from mickeyyaya/feat/pipeline-demo-relayout
- Merge pull request #238 from mickeyyaya/worktree-setup-auto-config


---

## [21.1.0] - 2026-06-24

_Generated by `evolve release` (go/internal/changeloggen) from commits v21.0.0..HEAD._

### Added

- broaden prebuilt matrix to 13 Unix targets + install.sh detection (#243)
- looping advisor pipeline demo with per-phase LLM routing
- Windows/WSL2 guidance in install.sh + docs (#240)
- one-line curl|sh installer (prebuilt-first, build fallback, auto-deps) (#239)
- auto-detecting /evo:setup with one-preset model config

### Documentation

- move Quick Start above "The problem it solves"

### Other

- Merge pull request #242 from mickeyyaya/feat/pipeline-demo-relayout
- Merge pull request #238 from mickeyyaya/worktree-setup-auto-config


---

## [21.0.0] - 2026-06-24

_Generated by `evolve release` (go/internal/changeloggen) from commits v20.4.0..HEAD._

### Added

- remove EVOLVE_STRICT_AUDIT dial — policy.json workflow.strict_audit + DI (ADR-0064) (#236)
- remove EVOLVE_WORKTREE_BASE dial — policy.json worktree.base + WithWorktreeBase DI to all 3 readers (ADR-0064)
- append-per-release model (preserve public history)
- sanitizer allowlist for known-safe test/example fixtures
- add publish-mirror skill, available across all LLM CLIs

### Fixed

- remove operator username from the sanitizer allowlist comment

### Changed

- rename command namespace evolve-loop → evo (/evo:loop, /evo:tdd, …)

### Documentation

- adopt the condensed pitch README into private (B1c resolved)

### Other

- Merge pull request #237 from mickeyyaya/feat/plugin-evo-namespace
- Merge pull request #235 from mickeyyaya/chore/rename-evolveloop-to-evolve-loop
- Merge pull request #232 from mickeyyaya/chore/remove-publish-mirror
- Merge pull request #231 from mickeyyaya/flag/worktree-base-config


---

## [20.4.0] - 2026-06-24

_Generated by `evolve release` (go/internal/changeloggen) from commits v20.3.0..HEAD._

### Added

- automate the public OSS mirror release
- goal-integrity rubric (M4) + protect the real adversarial skill (S6, ADR-0064 Pillar 2)
- fail-closed quarantine on unreachable parent during active campaign (S5b, ADR-0064 Pillar 2 / M3)
- anti-rename invariant — every os.Getenv key must be EVOLVE_ or allowlisted (S5a, ADR-0064 Pillar 2 / M2)
- fold-aware read-set gate — every Go reader of an operator dial must be registered (S4a, ADR-0064 Pillar 2)
- go/types fold + env-source taint harness (S0, ADR-0064 Pillar 2)
- pipeline control-plane boundary — a cycle cannot edit the gates that grade it (ADR-0064)

### Fixed

- sanitizer handles symlinks; test fixtures use a fake username
- derive login name via os/user, not os.Getenv(USER)
- strip the split-const dodge template from the campaign plan (sync, ADR-0064)
- make release preflight deterministic — audit-absent is advisory, CI-green authoritative (#220)

### Documentation

- mark B1/B2 OSS convergence done; refresh transform tables
- add public-release runbook for the private→public evolveloop mirror

### Other

- Merge pull request #230 from mickeyyaya/docs/refresh-public-release-runbook
- Merge pull request #229 from mickeyyaya/feat/oss-converge-b2-rename
- Merge pull request #228 from mickeyyaya/feat/oss-converge-b1
- Merge pull request #227 from mickeyyaya/feat/plan-contract-fix
- Merge pull request #226 from mickeyyaya/feat/anti-gaming-rubric
- Merge pull request #225 from mickeyyaya/feat/flag-failclosed
- Merge pull request #224 from mickeyyaya/feat/flag-antirename
- Merge pull request #223 from mickeyyaya/feat/flag-readset-metric
- Merge pull request #222 from mickeyyaya/feat/honest-flag-metric
- Merge pull request #221 from mickeyyaya/feat/pipeline-integrity-boundary


---

## [20.3.0] - 2026-06-23

_Generated by `evolve release` (go/internal/changeloggen) from commits v20.2.0..HEAD._

### Added

- flag-campaign-10 wave-2 — 6 more flags (30→24 rows, 14→13 live) (#217)
- doc↔impl reconciliation (ADR-0062) — wire dossier producer + floor gate, repoint commit-prefix scope to Go tree, purge dead Go→bash shell-outs, deterministic delta-intent, correct authoritative docs (#213)
- integrate flag-campaign-10 wave-1 — 5 flags eliminated (35→30 rows, 18→14 live) (#215)
- toolchain-green hard gate — a cycle can't ship code that fails build/vet/test (#214)
- flag-progress gate — fail a campaign cycle that deletes no registry row (#212)
- salvage flag-campaign-8 wave-1 — delete 13 flag rows (48->35) (#211)
- CLI-control abstraction + proactive per-cycle usage probe
- enable the minimalism discipline in all build/design/test phase agents
- add minimalism skill + always-on Builder/AGENTS wiring (from ponytail principles)
- integrate flag-campaign-7 deprecations (live flags 23->21)
- load-time evaluator-dominates-ship-sink check + 100% validator coverage (ADR-0060)
- live-feature-flag metric + monotonic-decrease gate (ADR-0061)
- config-declare the source-write axis via registry writes_source (PA-DDK DDK-7b)
- config-drive the transition legality graph + wire the validator as a hard gate (PA-DDK DDK-5)
- config-drive the early-exit set via per-phase early_exit (PA-DDK DDK-7)
- config-drive the recovery successor targets (PA-DDK DDK-6)
- config-drive the artifact-floor thresholds via per-phase gate (PA-DDK DDK-4)
- config-declare the linear spine via registry config.spine_order (PA-DDK DDK-3)
- ADR-0060 + phase-agnostic safety-invariant validator (PA-DDK DDK-1)
- deterministic post-build self-check + flag-conversion DoD guidance (false-green backstop)
- cross-session ownership lease to stop the multi-session reap-loop (ADR-0059)
- merge-to-main gate foundation — cadence advisor + promoter + gate phase (shadow-default)
- demo shows the model selecting AND skipping phases
- reframe demo — the model composes its own pipeline
- interactive 'live pipeline' demo
- enforce per-module documentation + fill the 3 doc gaps
- reposition copy around concepts, not implementation
- harden orchestration for large-scale concurrent runs
- product landing page — 5 elegant versions + gallery

### Fixed

- phase-observer --enforce activates chain-backed stall policy (#216)
- isolate agent tmux onto a dedicated socket (stop operator-input leak into agent REPLs)
- per-call timeout on tmux subprocess calls (unblocks wedged completion wait)
- sanitize build-selfcheck go-test env (strip EVOLVE_* runtime flags)
- enforce a mandatory evaluator in ValidateSafetyInvariants + harden DDK-7 test (adversarial-review follow-up)
- build-selfcheck ignores build-tag-excluded packages (acs/cycleN)
- thread per-cycle output_contract to the cycle as --goal (issue 14/22)
- deterministic post-build regeneration of derived projections
- auto-resolve fleet rebase conflicts on generated projections
- thread cycle worktree to phase advisor under EVOLVE_FLEET

### Changed

- config-drive the spine-anchor set/order from registry (PA-BIG S6, ADR-0058)
- flip the linear transition spine to a data table (PA-BIG S5, ADR-0058)
- load-time validator + registry guard for transition-activating fields (PA-BIG S4, ADR-0058)
- config-drive the debugger decision-branch gate (PA-BIG S3, ADR-0058)
- config-drive the retro history-branch gate (PA-BIG S2, ADR-0058)
- config-drive the audit verdict branch (PA-BIG S1, ADR-0058)
- config-drive optional-infra-skip ship guard (PA-3a)
- ship/retro self-register their phase factories
- extract phase-subsystem handlers into internal/cli/phasecmd
- extract release/ops handlers into internal/cli/opscmd
- extract guard/gate handlers into internal/cli/guardcmd
- move cycle execution-result value types into cyclestate leaf
- extract shiperr leaf — ship→orchestrator error protocol out of core
- move cycle/state DTOs into cyclestate leaf
- extract cyclestate domain-vocabulary leaf from core

### Documentation

- ADR-0063 (autonomous-loop integrity hardening) + flag-campaign-10 learnings (#218)
- close DDK-8 as a decided boundary (alias bridge stays; no risky rename)
- close out PA-DDK campaign — flow fully config-driven, document the deliberate Go boundary (ADR-0060)
- ADR-0058 + transition-kernel byte-identity oracle (PA-BIG-0)

### Other

- Merge pull request #210 from mickeyyaya/fix/bridge-tmux-socket-isolation
- Merge pull request #209 from mickeyyaya/fix/tmux-per-call-timeout
- Merge pull request #208 from mickeyyaya/feat/cli-control-abstraction
- Merge pull request #207 from mickeyyaya/docs/adr-0060-ddk8-decided
- Merge pull request #206 from mickeyyaya/feat/campaign7-deprecations
- Merge pull request #205 from mickeyyaya/feat/flag-livefeature-gate
- Merge pull request #204 from mickeyyaya/fix/selfcheck-env-sanitize
- Merge pull request #203 from mickeyyaya/feat/ddk-validator-hardening
- Merge pull request #202 from mickeyyaya/feat/ddk-8-aliases
- Merge pull request #201 from mickeyyaya/feat/ddk-7b-router-floor
- Merge pull request #200 from mickeyyaya/feat/ddk-5-legality
- Merge pull request #199 from mickeyyaya/fix/selfcheck-buildtag-exclusion
- Merge pull request #198 from mickeyyaya/feat/ddk-7-earlyexit
- Merge pull request #197 from mickeyyaya/feat/ddk-6-recovery-maps
- Merge pull request #196 from mickeyyaya/feat/ddk-4-artifact-gate
- Merge pull request #195 from mickeyyaya/feat/ddk-3-spine-config
- Merge pull request #194 from mickeyyaya/feat/ddk-1-safety-validator
- Merge pull request #193 from mickeyyaya/feat/campaign-inject-output-contract
- Merge pull request #192 from mickeyyaya/feat/phase-agnostic-kernel-s6
- Merge pull request #191 from mickeyyaya/feat/build-full-test-changed-pkgs
- Merge pull request #190 from mickeyyaya/feat/phase-agnostic-kernel-s5
- Merge pull request #188 from mickeyyaya/feat/campaign-ownership-lease
- Merge pull request #189 from mickeyyaya/feat/phase-agnostic-kernel-s4
- Merge pull request #187 from mickeyyaya/feat/phase-agnostic-kernel-s3
- Merge pull request #186 from mickeyyaya/feat/phase-agnostic-kernel-s2
- Merge pull request #185 from mickeyyaya/feat/build-derived-regen
- Merge pull request #184 from mickeyyaya/feat/phase-agnostic-kernel-s1
- Merge pull request #183 from mickeyyaya/feat/phase-agnostic-kernel-oracle
- Merge pull request #182 from mickeyyaya/feat/rebase-derived-artifact-resolution
- Merge pull request #181 from mickeyyaya/feat/phase-agnostic-infraskip
- Merge pull request #180 from mickeyyaya/feat/phase-agnostic-ship-retro
- Merge pull request #179 from mickeyyaya/feat/merge-to-main-gate
- Merge pull request #178 from mickeyyaya/decouple/cmd-phasecmd
- Merge pull request #177 from mickeyyaya/decouple/cmd-opscmd
- Merge pull request #176 from mickeyyaya/decouple/cmd-guardcmd
- Merge pull request #175 from mickeyyaya/fix-advisor-fleet
- Merge pull request #174 from mickeyyaya/landing-page
- Merge pull request #173 from mickeyyaya/landing-page
- Merge pull request #172 from mickeyyaya/landing-page
- Merge pull request #171 from mickeyyaya/decouple/docgo-gate
- Merge pull request #169 from mickeyyaya/worktree-loop-largescale-hardening
- Merge pull request #170 from mickeyyaya/landing-page
- Merge pull request #168 from mickeyyaya/decouple/core-values2
- Merge pull request #166 from mickeyyaya/decouple/shiperr
- Merge pull request #167 from mickeyyaya/landing-page
- Merge pull request #165 from mickeyyaya/landing-page
- Merge pull request #164 from mickeyyaya/decouple/cyclestate-phase
- Merge pull request #163 from mickeyyaya/landing-page
- Merge pull request #162 from mickeyyaya/chore/retire-stale-acs-count-predicates


---

## [20.2.0] - 2026-06-21

_Generated by `evolve release` (go/internal/changeloggen) from commits v20.1.1..HEAD._

### Added

- wire runtime operator-directives into the loop (slice 2)
- internal/directives package (runtime operator-directives loader, slice 1)

### Fixed

- deterministic event-triggered test via OnEvent hook
- disclose CI-not-verified in evolve release + gate CI in /publish

### Documentation

- re-aim campaign to ZERO operator flags (design patterns)

### Other

- Merge pull request #161 from mickeyyaya/integ/flag-reduction-v20-merge
- (merge) integrate flag-reduction-v20 — registry 126→47
- Merge pull request #160 from mickeyyaya/fix/observer-event-triggered-test
- Merge pull request #159 from mickeyyaya/feat/runtime-directives-integration
- evolve-cycle 52: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- Merge pull request #158 from mickeyyaya/feat/runtime-directives-pkg
- Merge pull request #157 from mickeyyaya/feat/flag-param-conversion-standard
- docs+test(standard): flag-parameter conversion standard + runtime-directives design spec
- evolve-cycle 50: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- evolve-cycle 49: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- evolve-cycle 48: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- evolve-cycle 47: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- Merge pull request #156 from mickeyyaya/test/flag-param-suite
- evolve-cycle 46: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- Merge pull request #155 from mickeyyaya/fix/release-ci-verification
- evolve-cycle 45: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- evolve-cycle 44: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- evolve-cycle 43: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- evolve-cycle 39: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- evolve-cycle 38: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- evolve-cycle 37: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- (config) shift 3 lightweight build phases to agy-tmux
- evolve-cycle 34: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- evolve-cycle 33: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- evolve-cycle 32: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- evolve-cycle 31: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- evolve-cycle 30: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- evolve-cycle 29: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- evolve-cycle 28: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- evolve-cycle 27: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- evolve-cycle 26: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- evolve-cycle 25: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- evolve-cycle 24: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- evolve-cycle 23: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- evolve-cycle 22: goal=f6c32e9844f474a6f37f82f0c35187a80fd3613f8876d08e935809c4bc4a1afc
- (config) capability-based CLI balance across all 3 tmux drivers
- evolve-cycle 15: goal=c681b535da7fdf5dff622b54695815cacdf9b3e84c6d5d2341c4e014ebb26cd6


---

## [20.1.1] - 2026-06-20

_Generated by `evolve release` (go/internal/changeloggen) from commits v20.1.0..HEAD._

### Fixed

- repair apicover-enforce gate so the go workflow goes green

### Other

- Merge pull request #154 from mickeyyaya/fix/apicover-enforce-ci-green


---

## [20.1.0] - 2026-06-20

_Generated by `evolve release` (go/internal/changeloggen) from commits v20.0.0..HEAD._

### Added

- D1 core — Dossier type + Validate (ADR-0055)
- S3a campaign.Render — human-readable plan view for the approval gate (ADR-0054)
- S2 preliminary-study phase — research-backed decomposition (ADR-0054)
- S1 wave engine — dag.Levels + fleet.PlanWaves + campaign.Verify (ADR-0054)

### Fixed

- repair registry drift, source 160->126 to match committed control-flags.md
- regenerate control-flags.md with rebuilt binary (126 flags, not stale 160)
- regenerate control-flags.md after zero-flag merge (drift repair)
- update rehomed cycle6 TestC6_001 to the post-cycle-9 Build contract
- relax stale cycle7 C7_002 exact-count predicate to a ratchet
- relax stale cycle7 C7_002 exact-count predicate to a ratchet

### Other

- (merge) integrate zero-flag campaign reductions (154->126) into main
- (merge) ADR-0054 advisor-driven preliminary study cycle into main
- evolve-cycle 17: goal=18b31f60a1d2d1fdc01bcf6cb5572cfc5cf25e5a787443757b9b3a1bdf42d590
- (merge) ADR-0055 Cycle Dossier (cycle-dossier) into main
- evolve-cycle 9: goal=fe38264944b9a641cc941c503bf5bda7befa20cee39bf70cd886a911e33c7153
- (merge) integrate v20.0.0 main into cycle-dossier
- (merge) integrate v20.0.0 main into campaign-preliminary-study
- evolve-cycle 2: goal=b3f080a11ebf6ea04c3d0a30993771f990b778c3e44789a7bc2a8164896b1f4b
- Merge branch 'fix-cycle-worktree-collision' into cycle-dossier
- evolve-cycle 3: goal=b0de98a7ba8f60dc3729c236fdde8823a4ce6b9165b2c89939c1891f5c3fea2e
- (wip) S3 driver + S4 ADR from cycle-1 (functional; 3 known audit defects to fix next)


---

## [20.0.0] - 2026-06-19

_Generated by `evolve release` (go/internal/changeloggen) from commits v19.1.1..HEAD._

### Added

- wire advisor cycle-budget into the dispatcher (EVOLVE_CYCLE_BUDGET)
- pure decision core for advisor-decided, completion-driven loop stop
- single-source RunScope value object for run/cycle/lane naming (Layer 1)
- rewire CI to script-free Go validation + permanent no-orphan gate (Wave E)
- port install/uninstall + secure-orchestrator example to Go (Wave D)

### Fixed

- namespace cycle worktree branch/dir per-root to unblock concurrent loops
- broaden flagreaders guard to all surfaces (close cycle-360 false-dead class)
- cycle-360 remediation — reclassify 4 shell-read flags mis-inventoried as dead
- flag-reduction Wave 0 — reader-completeness guard replaces the count-floor
- cycle-355 dual-root — EVOLVE_WORKTREE_ROOT so flags/skills check validate the worktree
- close cycle-352 audit-FAIL classes (gofmt all phases + FileNotContains)

### Changed

- remove WorktreeToken superseded by runscope (single-source)
- delete bash commit-gate + rewire CI to the Go gate (Wave B2)
- delete 7 bash CLI adapters + close cycle-360 flag reclassification (Wave A5)
- route subagent + consensus dispatch through the Go bridge (Wave A3.5)
- port commit-gate-runner.sh to evolve commit-gate run (Wave B1)
- migrate 8 phase-validation bash scripts to one Go table test (Wave C)
- extract treestate.SHA — single tree-state-SHA impl for ship + commit-gate (Wave B0)
- migrate _capability-check.sh quality_tier to Go (Wave A1)

### Documentation

- add anti-gaming rule to campaign SSOT (cycle-8 audit lesson)
- design-pattern cluster-consolidation campaign SSOT (target <30)
- implementation spec for Slices 2-6 (sibling-worktree architecture)

### Other

- Merge branch 'advisor-cycle-budget'
- Merge branch 'concurrency-arch-slices'
- evolve-cycle 13: goal=c681b535da7fdf5dff622b54695815cacdf9b3e84c6d5d2341c4e014ebb26cd6
- evolve-cycle 12: goal=c681b535da7fdf5dff622b54695815cacdf9b3e84c6d5d2341c4e014ebb26cd6
- evolve-cycle 11: goal=c681b535da7fdf5dff622b54695815cacdf9b3e84c6d5d2341c4e014ebb26cd6
- evolve-cycle 10: goal=c681b535da7fdf5dff622b54695815cacdf9b3e84c6d5d2341c4e014ebb26cd6
- evolve-cycle 9: goal=c681b535da7fdf5dff622b54695815cacdf9b3e84c6d5d2341c4e014ebb26cd6
- evolve-cycle 5: goal=7123a5fd8b72dbe4f1b66bd9fe438ef2460b5ff295bd8ef661ca8544546a6495
- evolve-cycle 7: goal=097a9bf0b3f1c6d5bd9c1c491050bf989cc1dfa478a6757f310e7b353e8508b0
- evolve-cycle 4: goal=7123a5fd8b72dbe4f1b66bd9fe438ef2460b5ff295bd8ef661ca8544546a6495
- evolve-cycle 3: goal=7123a5fd8b72dbe4f1b66bd9fe438ef2460b5ff295bd8ef661ca8544546a6495
- evolve-cycle 2: goal=7123a5fd8b72dbe4f1b66bd9fe438ef2460b5ff295bd8ef661ca8544546a6495
- evolve-cycle 4: goal=ec482bca8554bbee335556c26429ccb85eeb8aacb01822874f33892688b4d189
- evolve-cycle 1: goal=7123a5fd8b72dbe4f1b66bd9fe438ef2460b5ff295bd8ef661ca8544546a6495
- evolve-cycle 3: goal=ec482bca8554bbee335556c26429ccb85eeb8aacb01822874f33892688b4d189
- evolve-cycle 2: goal=ec482bca8554bbee335556c26429ccb85eeb8aacb01822874f33892688b4d189
- evolve-cycle 359: goal=d2158139d68e2c09c3bfe050c8efc3d03c708aa2af99a372d3e76b1382bce0e0
- evolve-cycle 358: goal=d2158139d68e2c09c3bfe050c8efc3d03c708aa2af99a372d3e76b1382bce0e0
- evolve-cycle 357: goal=d2158139d68e2c09c3bfe050c8efc3d03c708aa2af99a372d3e76b1382bce0e0
- evolve-cycle 356: goal=d2158139d68e2c09c3bfe050c8efc3d03c708aa2af99a372d3e76b1382bce0e0
- evolve-cycle 354: goal=a7649c39866414b8ce6cfdaa2f5c2fffc293bac5ec39d0c29417bfc6be5bb2f8
- evolve-cycle 353: goal=a7649c39866414b8ce6cfdaa2f5c2fffc293bac5ec39d0c29417bfc6be5bb2f8


---

## [19.1.1] - 2026-06-17

_Generated by `evolve release` (go/internal/changeloggen) from commits v19.1.0..HEAD._

### Fixed

- I1 — isolate bridge probe launches to a scratch cwd (no checkout leak)
- deterministic post-build gofmt -w -s normalizer (I9)


---

## [19.1.0] - 2026-06-17

_Generated by `evolve release` (go/internal/changeloggen) from commits v19.0.0..HEAD._

### Added

- WS6 — optional per-decision multi-model + cli-health fallback (ADR-0052)
- WS2-S6 — advisory re-plan flip (ADR-0052)
- WS2b — shadow post-scout re-plan + mismatch + depth-cap (ADR-0052)
- WS2a — pre-plan recon + ValidatePlan + rejection telemetry + post-scout hook (ADR-0052)
- WS1 — AgentIdentity substrate + recursion guard + RePlan entrypoint (ADR-0052)
- WS4 — golden routing-eval corpus + replay lock + LLM-as-judge (ADR-0052)
- WS3 — decision capture, ledger-bind, OTel span + routing explain/replay (ADR-0052)
- WS5 — goal-type recipe SSOT projection + drift lock (ADR-0052)
- advisor-maximization WS0-S1 foundations (ADR-0052)

### Documentation

- sharpen product pitch + adversarial/competitor positioning
- backfill Phase 5 trail + mark ADR-0050 implemented (5.docs)

### Other

- Merge pull request #153 from mickeyyaya/advisor-ws1-onward
- Merge pull request #152 from mickeyyaya/advisor-ws0-foundations
- Merge pull request #151 from mickeyyaya/phase5-docs


---

## [19.0.0] - 2026-06-17

_Generated by `evolve release` (go/internal/changeloggen) from commits v18.16.0..HEAD._

### Documentation

- decision-log rows rel-18.16.0 + 5.4 + 5.5
- godoc 9 packages + graduate them into the enforce gate

### Other

- Merge pull request #150 from mickeyyaya/phase5-acs
- Merge pull request #149 from mickeyyaya/phase5-core
- Merge pull request #148 from mickeyyaya/phase5-heavy
- Merge pull request #147 from mickeyyaya/phase5-gap57
- Merge pull request #146 from mickeyyaya/phase5-gap4
- Merge pull request #145 from mickeyyaya/phase5-gap3
- Merge pull request #144 from mickeyyaya/phase5-gap2
- Merge pull request #143 from mickeyyaya/phase5-gap1
- Merge pull request #142 from mickeyyaya/phase5-name-types
- Merge pull request #141 from mickeyyaya/phase5-adapters-bridge
- Merge pull request #140 from mickeyyaya/phase5-docs-catchup
- Merge pull request #139 from mickeyyaya/phase5-s5-godoc
- Merge pull request #138 from mickeyyaya/phase5-s4-batch35


---

## [18.16.0] - 2026-06-16

_Generated by `evolve release` (go/internal/changeloggen) from commits v18.15.0..HEAD._

### Changed

- route git access through the gitexec seam (S4.5)

### Documentation

- decision-log rows for Phase 5 slices 5.1-5.3 (apicover enforce)
- backfill Phase 2-4 decision-log rows + correct the enforce default

### Other

- Merge pull request #137 from mickeyyaya/phase5-docs
- Merge pull request #136 from mickeyyaya/phase5-s3-soakreport
- Merge pull request #135 from mickeyyaya/phase5-s2-skillcheck-flagregistry
- Merge pull request #134 from mickeyyaya/phase5-s1-apicover-enforce
- Merge pull request #133 from mickeyyaya/phase234-docs
- Merge pull request #132 from mickeyyaya/phase4-s45-gitexec


---

## [18.15.0] - 2026-06-16

_Generated by `evolve release` (go/internal/changeloggen) from commits v18.14.0..HEAD._

### Added

- Phase 3.10 Slice 7 — flip EVOLVE_PHASE_IO default off→enforce (the cutover)
- Phase 3.10 Slice 6 — ship gate verdict parse is sentinel-first at enforce
- Phase 3.10 Slice 5 — audit-side prose verdict fallbacks gated at enforce
- Phase 3.10 Slice 4 — ship reads commit_message from the typed envelope at enforce
- Phase 3.10 Slice 3 — scout reads the typed envelope at enforce
- Phase 3.10 Slice 2 — intent/triage/debugger read the typed envelope at enforce
- Phase 3.10 Slice 1 — thread EVOLVE_PHASE_IO into the reconcile-on-timeout rung
- Phase 3.10 Slice 0 — typed PhaseInput on PhaseRequest (enforce-gated)
- Phase 3.8 — PhaseIO-gated structured failure block (gate + prompt)

### Fixed

- fake-cli auditor emits the evolve-verdict sentinel for enforce default
- thread StageOff into the integration-tagged parseVerdicts caller

### Other

- Merge pull request #131 from mickeyyaya/phase3-10-slice7
- Merge pull request #130 from mickeyyaya/phase3-10-slice6
- Merge pull request #129 from mickeyyaya/phase3-10-slice5
- Merge pull request #128 from mickeyyaya/phase3-10-slice4
- Merge pull request #127 from mickeyyaya/phase3-10-slice3
- Merge pull request #126 from mickeyyaya/phase3-10-slice2
- Merge pull request #125 from mickeyyaya/phase3-10-slice1
- Merge pull request #124 from mickeyyaya/phase3-10-slice0
- evolve-cycle 348: goal=701902e21e65a10d82cda1b998cac035cea71232ab4f65d01bac8924a1893dbe
- evolve-cycle 347: goal=701902e21e65a10d82cda1b998cac035cea71232ab4f65d01bac8924a1893dbe
- Merge pull request #123 from mickeyyaya/phase3-8


---

## [18.14.0] - 2026-06-16

_Generated by `evolve release` (go/internal/changeloggen) from commits v18.13.0..HEAD._

### Added

- Phase 3.7 — build-plan via typed envelope (upstream read)
- Phase 3.9 — phase-agnostic ledger binding (recordPhaseBinding)
- Phase 3.6 — typed Carryover + comprehensive per-reader shadow coverage
- Phase 3.5 — typed CycleInputs + ErrorContext shadow comparison (retro-first)
- Phase 3.4 — EVOLVE_PHASE_IO shadow assembly + comparator
- Phase 3.3 — payload-wrapped handoff reader + router→phaseio assembler
- Phase 3.2 — EVOLVE_PHASE_IO rollout dial (off→shadow→advisory→enforce)
- Phase 3.1 — unified phase I/O leaf types (sealed envelope)

### Other

- Merge pull request #122 from mickeyyaya/phase3-7
- Merge pull request #121 from mickeyyaya/phase3-9
- Merge pull request #120 from mickeyyaya/phase3-6-readers
- Merge pull request #119 from mickeyyaya/phase3-5-cycleinputs
- Merge pull request #118 from mickeyyaya/phase3-4-shadow
- Merge pull request #117 from mickeyyaya/phase3-3-digest-assembler
- Merge pull request #116 from mickeyyaya/phase3-2-config-flag
- Merge pull request #115 from mickeyyaya/phase3-1-phaseio


---

## [18.13.0] - 2026-06-15

_Generated by `evolve release` (go/internal/changeloggen) from commits v18.12.0..HEAD._

### Documentation

- sweep stale Go-only-consolidation refs + audit fixes

### Other

- Merge pull request #114 from mickeyyaya/phase2-concurrency
- Merge pull request #113 from mickeyyaya/docs-alignment


---

## [Unreleased]

## [18.12.0] - 2026-06-15

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v18.11.0..HEAD._

### Added

- go/test/fixtures.StressN concurrency stress-barrier helper — n goroutines × k iterations released by a closed-channel barrier (campaign, PR #105 — Phase 0.4)
- unified Console logger + migrate non-hub printers (campaign, PR 9 — Phase 1.5)
- new internal/gitexec leaf — isolate the git CLI behind sysexec
- public-API coverage harness (stdlib AST, two-signal check) + warning-only CI
- SKILL.md-drift gate in cycle audit via extracted internal/skillcheck
- gofmt CI-parity gate in the cycle audit phase

### Fixed

- de-flake bridge integration tier (boot-timeout env + pidfile poll)
- explicit NoArtifact ship contract closes contract-gate fail-open
- gofmt cycle341 predicates + regen SKILL.md phase-facts

### Changed

- adopt envchain in cmd/evolve typed env knobs (campaign, PR 8 — Phase 1.3)
- migrate cycleclassify, changeloggen, swarm onto gitexec (campaign, PR 7 — Phase 1.2b)
- migrate git calls onto internal/gitexec
- Stage 2.8 - extract cycleRun.selectNext; RunCycle is now a slim driver
- Stage 2.7 - extract cycleRun.dispatch from RunCycle loop
- Stage 2.6 - extract cycleRun.reviewAndGuard from RunCycle loop
- Stage 2.5 - extract cycleRun.recordAndBranch from RunCycle loop
- Stage 2.4 - introduce cycleRun method object + promote RunCycle loop state
- Stage 2.3 - extract planCycle from RunCycle (pre-loop planning, behavior-preserving)
- Stage 2.2 - extract newCycleRun from RunCycle (init + cleanup closure, behavior-preserving)
- Stage 2.1 - extract finalizeCycle from RunCycle (method-object, behavior-preserving)
- Stage 0b - extract post-RunCycle helpers from orchestrator.go
- Stage 0a - extract pre-RunCycle helpers from orchestrator.go
- Lego-split cmd_loop.go 1398->508 into 3 cohesive files
- Lego-split driver_tmux_repl.go 1089->703 into 4 cohesive files
- adopt the single sysexec.RunFunc seam + dedup runner boilerplate (1A.1)

### Documentation

- land Phase 1 modularization decision-log rows (campaign, PR 10 — Phase-1 docs)
- package-map audit — inventory, coupling, dedup register
- ADR-0050 charter + design spec + decision log (modularization/phase-io campaign)
- document FakeExec concurrent-Run reader/writer ownership contract
- refresh fast-suite poles to measured post-merge reality

### Other

- Merge pull request #112 from mickeyyaya/phase1-docs
- Merge pull request #111 from mickeyyaya/phase1-5-log
- Merge pull request #110 from mickeyyaya/phase1-3-envchain
- Merge pull request #109 from mickeyyaya/phase1-2b
- Merge pull request #108 from mickeyyaya/phase1-2a-rollback
- Merge pull request #106 from mickeyyaya/phase1-1-gitexec
- Merge pull request #105 from mickeyyaya/phase0-4-stressn
- Merge pull request #104 from mickeyyaya/phase0-3-apicover
- Merge pull request #103 from mickeyyaya/phase0-2-audit
- Merge pull request #102 from mickeyyaya/phase0-1-docs
- Merge pull request #101 from mickeyyaya/orchestrator-runcycle
- Merge pull request #100 from mickeyyaya/orchestrator-decompose
- Merge pull request #99 from mickeyyaya/lego-splits
- Merge pull request #98 from mickeyyaya/test-arch-perf
- Merge branch 'main' into test-arch-perf
- evolve-cycle 341: goal=a13c899d21be8818463a0931f7c3c189e4cd777d89c6ea9248e5410c0fc1a7a9
- evolve-cycle 340: goal=a13c899d21be8818463a0931f7c3c189e4cd777d89c6ea9248e5410c0fc1a7a9
- evolve-cycle 339: goal=a13c899d21be8818463a0931f7c3c189e4cd777d89c6ea9248e5410c0fc1a7a9
- (merge) bring main (v18.11.0, +30 commits) into test-arch-perf


---

## [18.11.0] - 2026-06-14

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v18.10.0..HEAD._

### Added

- add 14 Wave-5 adversarial-pipeline phase agents + 6 goal types
- driver-agnostic spine model-routing tiers + cycle-337 test-latency gates
- add 15 adversarial-pipeline phase agents + 9 goal types

### Fixed

- regenerate SKILL.md phase-facts after spine tier change
- N14 route PromoteRule/EnforceRule through atomicwrite SSOT — kill the shared-temp collision (ADR-0049 PR-3)

### Changed

- remove token-budget cost calculation + all cost gates

### Other

- Merge pull request #94 from mickeyyaya/fix/n14-interaction-atomicwrite
- Merge pull request #96 from mickeyyaya/worktree-no-token-budget
- Merge pull request #97 from mickeyyaya/phase-adversarial-wave-5
- Merge pull request #95 from mickeyyaya/phase-adversarial-wave


---

## [18.10.0] - 2026-06-14

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v18.9.0..HEAD._

### Added

- N15 collision-proof ephemeral tmux session names via per-process nonce (ADR-0049 PR-3)
- N12 isolate concurrent fleet builds with a per-cycle GOCACHE (ADR-0049 PR-3)
- G13a distinguish fleet rebase conflict from infra, route to debugger (ADR-0049 PR-3)
- honor EVOLVE_FLEET_SCOPE — scoped cycle selects only its assigned todos (ADR-0049 PR-2)
- wire advisor backlog partition into `evolve fleet --plan` (ADR-0049 PR-2)
- G11 fail loud early on misconfigured supervisor via Validate() (ADR-0049 PR-1)
- G16 wire per-run runlease producer + heartbeat into RunCycle (ADR-0049 PR-1)
- G7 serialize cycle-state.json RMW via shared sidecar lock (ADR-0049 PR-1)
- --simulate fleet validation + fix stale -simulate phase set
- S5b-2 rebase + re-audit recovery for fleet ff-merge divergence (ADR-0049 S5b)
- S5b-1 signal GIT_FLEET_REBASE_NEEDED on fleet-mode ff-merge divergence (ADR-0049 S5b)
- E advisor backlog-partitioner — assign independent todos to cycles (ADR-0049 E)
- S6b-2 wire 'evolve fleet' concurrent-cycle command (ADR-0049 S6)
- S6b-1 concurrent-cycle supervisor package (ADR-0049 S6/R2)
- S6a fleet-mode skips the whole-cycle global lock (ADR-0049 R1)
- S4 run-scope the audit->ship binding lookup (ADR-0049 G5)
- S3b run-scope all cycle_id reads via cycleStateFile (ADR-0049 G3)
- S3a run-scope readActiveWorktree via run.json mirror (ADR-0049 G3)
- S5 serialized integrator lock around shared-main critical section (ADR-0049 G1)
- S2b serialize remaining state.json RMW sites under shared lock (ADR-0049 G2)
- S2a shared state.json flock helper + advanceLastCycleNumber (ADR-0049 G2)
- S1 per-worker env injection, kill process-global os.Setenv (ADR-0049 G8)
- S0a per-invocation sandbox profile dir (ADR-0049 G6)

### Fixed

- N17 fold escalation check+act under one cycle-state lock — close the phase-complete TOCTOU (ADR-0049 PR-3)
- N10 serialize codex pretrust RMW under flock — stop concurrent fleet cycles dropping each other's trust entries (ADR-0049 PR-3)
- N14 route PromoteSignature through atomicwrite SSOT — kill the shared-temp collision (ADR-0049 PR-3)
- N9 read EVOLVE_DISABLE_WORKSPACE_GUARD + EVOLVE_BACKFILL_ENABLED from the per-cycle snapshot, not live os.Getenv (ADR-0049 PR-3)
- sanitize fleet_scope prompt-injection + fail-loud only() (ADR-0049 PR-2 review)
- Partition must be CROSS-bucket file-disjoint for concurrent cycles (ADR-0049 PR-2)
- force base branch to main in S5b divergedWorktree (CI git-default-branch)

### Documentation

- record PR-3 dispositions + defer mergequeue.lock (ADR-0049 PR-3)
- ADR-0049 + isolation research dossier (bottom-up agent→phase→cycle plan)

### Other

- Merge pull request #93: ADR-0049 concurrency isolation hardening (PR-1/2/3)
- Revert "feat(core): N12 isolate concurrent fleet builds with a per-cycle GOCACHE (ADR-0049 PR-3)"
- Merge pull request #92 from mickeyyaya/fleet-simulate
- Merge pull request #91 from mickeyyaya/concurrency-s0


---

## [18.9.0] - 2026-06-14

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v18.8.0..HEAD._

### Other

- evolve-cycle 336: goal=501f3ab74de7713090b2f993160cfef4015e2381850fc48c8110c8c9eb690bf2
- evolve-cycle 335: goal=501f3ab74de7713090b2f993160cfef4015e2381850fc48c8110c8c9eb690bf2
- evolve-cycle 334: goal=501f3ab74de7713090b2f993160cfef4015e2381850fc48c8110c8c9eb690bf2
- evolve-cycle 333: goal=501f3ab74de7713090b2f993160cfef4015e2381850fc48c8110c8c9eb690bf2


---

## [18.8.0] - 2026-06-14

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v18.7.0..HEAD._

### Added

- verification SSOT + tunable progress-aware deadline (B3) (#85)
- recursion-through-the-bridge — depth cap + sandbox coherence (B2) (#84)
- bridge-only dispatch invariant — retire in-process escape hatch (B1) (#83)
- single-source the inner-sandbox confinement decision (SSOT)
- evolve ledger anchor — non-destructive epoch-anchor (ADR-0048, ledger-1740)
- ADR-0048 Slice B — content-addressed audit reuse (shadow)
- ADR-0048 Slice C1 — verify audit-bound tree-SHA before the worktree commit (verify-before-mutate)
- ADR-0048 Slice A — graduated-enforcement classifier (shadow)
- ADR-0047 — one channel separator per mixed surface (content-vs-chrome class fix)

### Fixed

- fan-out per-worker provenance verification — close H1 (token threading) (#89)
- wire captureWithEBADFRetry into all git-exec test sites — macOS EBADF flake
- idle-gate CLI-state escalations — a busy pane's banner is agent content (ADR-0047)
- a progressing agent is never bounded by maxExtends — only busy-but-stalled
- bench evidence is the wall banner line, not the pane's first line
- preserve the worktree when a cycle completes with a FAIL verdict
- router routing-plan.json is a bare JSON array, not a {plan} object
- always drain residual inbox claims on ship, even without triage-decision.json

### Documentation

- loop-binary-self-deploy (ADR-0046 L3) + cli-refresh-canary design notes
- Stage 4 design finding — chrome-zone≠footer region; corpus-first, persistence orthogonal
- ADR-0048 work-conservation — graduated enforcement, content-addressed audit reuse, resilient ship

### Other

- evolve-cycle 332: goal=a1aaa81958f7867c4856e6ae6324574fc0679a5cbd42d1726c4d34322fb49588
- evolve-cycle 331: goal=a1aaa81958f7867c4856e6ae6324574fc0679a5cbd42d1726c4d34322fb49588
- evolve-cycle 330: goal=a1aaa81958f7867c4856e6ae6324574fc0679a5cbd42d1726c4d34322fb49588
- evolve-cycle 329: goal=a1aaa81958f7867c4856e6ae6324574fc0679a5cbd42d1726c4d34322fb49588


---

## [18.7.0] - 2026-06-13

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v18.6.0..HEAD._

### Added

- EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS — circuit-broken continue-on-verdict-FAIL
- ADR-0046 Layer 2 — identical-rejection demotion inside the capacity clamp
- L3.3 — chain-preserving ledger seal + verifier-vs-reality fixes
- L3.2 — layout-agnostic lease-aware run discovery + .lease contract
- L3.1 — internal/gc declarative retention engine + policy.json:gc schema
- CB.4 — per-run run.json guard state + worktree symlink retarget
- CB.6 — observer run scoping + escalation evidence survives session death
- CB.5 — run-scoped tmux namespace + per-run session registry reap
- CB.3 — codex pretrust pinned at the chokepoint + recipe-path gap closed
- CB.2 — fail-closed workdir + tmux new-session -c at birth
- CB.1 — every phase dispatches with cwd=cycle-worktree

### Fixed

- exempt scout eval-materialization from the tree-diff leak guard
- exclude agent edit-diff lines from interactive-prompt matching
- defer_reason prose never counts as floors; Gate C committed-wins subtraction
- classify recorded FAIL verdicts as FAILED_EXPLAINED; un-strand claimed inbox items
- strip contract metadata + slash-qualify prose-collider packages in floor counter
- recognize claude's native auto-update freeze (settings autoUpdates:false)
- unify contract resolution — catalog-aware reconcile default + P0 cross-leg test sweep
- grant the ADR-0034 agent self-check to all 53 contract-bearing profiles
- function-scoped ACS assertions — acsassert.CountInGoFunc kills the count-rot class
- persona-lint zero-WARN — 35 coherence WARNs cleared (data + 2 checker fixes)
- resume-from-build normalize parity — persist WorktreeBaseSHA in CycleState
- salvage cycle-293 EVOLVE_CLI test isolation (llmroute + runner TestMain unset)
- Go 1.23 compatibility — t.Chdir shim + gofmt -s + cycle-296 predicate refresh

### Documentation

- ADR-0046 gate epistemics + declarative floors + loop self-deploy; triage blocker-solo rule

### Other

- evolve-cycle 322: goal=d5af2cdd7f5b1781d28dfc0cac669d5b949fa45b6763d6fcd783817845f66c17
- evolve-cycle 321: goal=d5af2cdd7f5b1781d28dfc0cac669d5b949fa45b6763d6fcd783817845f66c17
- evolve-cycle 320: goal=d5af2cdd7f5b1781d28dfc0cac669d5b949fa45b6763d6fcd783817845f66c17
- evolve-cycle 318: goal=749b336d916a22b509952b01383191e41cbb9d6f4624388ce2eecf7b32e0a200
- evolve-cycle 316: goal=5e3f13381a93c45549b2a687df6300cc3203ae8f718f6d0b794f651f3313f6bc
- evolve-cycle 308: goal=63765c3bce8824fdef55f58d950fc7066be1c4c520324508c65842d2b874450f
- evolve-cycle 305: goal=09dfd4932ea38d04eb32b187e02ebfe751221650bf1e8c4c6657c2c7038a96c8
- evolve-cycle 304: goal=09dfd4932ea38d04eb32b187e02ebfe751221650bf1e8c4c6657c2c7038a96c8
- evolve-cycle 300: goal=6564683183866428e9e9096034cadbd910108fd30cb2ce372b39ad023836faf3
- evolve-cycle 299: goal=6564683183866428e9e9096034cadbd910108fd30cb2ce372b39ad023836faf3
- evolve-cycle 298: goal=6564683183866428e9e9096034cadbd910108fd30cb2ce372b39ad023836faf3
- Merge pull request #82 from mickeyyaya/worktree-preflight-claude-freeze
- Merge pull request #81 from mickeyyaya/worktree-concurrency-w4-l3
- Merge pull request #80 from mickeyyaya/worktree-concurrency-w4-cb
- Merge pull request #79 from mickeyyaya/worktree-concurrency-w4-cb


---

## [18.6.0] - 2026-06-12

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v18.5.0..HEAD._

### Added

- CA.6 byte-stability golden pins + causal launch-diagnostic picker
- CA.5 — ULID run-id threading through CycleState + ledger
- CA.4 — cycle-number allocation lease
- CA.1/CA.2/CA.3 — Track C-A foundations (flock, run_id, OCC UpdateState)
- L2.1/L2.2 — EVOLVE_* flag SSOT + evolve flags generate|check
- R9.3 — floor predicates bind only triage-committed floors
- R9.1/R9.2 — observed-throughput window + triage capacity clamp
- R8.3 — soak evidence reporter + durable C2 shadow records
- R8.1/R8.2 — wire the failure-advisor tail + I4 measured auto-enforce
- cycle-outcome SLO classifier + batch surfacing (R6)

### Fixed

- emit audit/build provenance bindings on resume path
- refuse empty tmux kill-session target + seam-test teardown (killer-B closure)
- blank-pane render-wedge recovery — jiggle + never pause a live agent
- claude 2.1.173 busy detection — spinner stats line is the only busy chrome left
- R3.6 — persist validate-gauntlet stderr + thread cause into the launch error
- process liveness — pane echo is not alive (R3.3/R3.4)
- spine gate fails closed at enforce — typed read-miss vs clean absence
- gate dead-shell guard per-driver — un-break shell-script REPL harnesses
- boot handshake — dead-shell guard + post-paste spill fast-fail
- unpaired persona→profile WARN + live-tree pairing drift gate
- atomic release — class-aware staging, ldflags stamp, terminal release-verify

### Documentation

- soak report 2026-06-12 — batch #6 PASS 4/4, reboot-recovery forensics

### Other

- evolve-cycle 297: goal=98f198ac8b7a8251f277b01e7c1154c9379bee5671c6711a331d0bdd63d11015
- evolve-cycle 296: goal=98f198ac8b7a8251f277b01e7c1154c9379bee5671c6711a331d0bdd63d11015
- evolve-cycle 295: goal=98f198ac8b7a8251f277b01e7c1154c9379bee5671c6711a331d0bdd63d11015
- evolve-cycle 294: goal=98f198ac8b7a8251f277b01e7c1154c9379bee5671c6711a331d0bdd63d11015
- Merge pull request #78 from mickeyyaya/worktree-concurrency-r1-tree-guard
- Merge pull request #77 from mickeyyaya/worktree-concurrency-r1-tree-guard
- Merge pull request #76 from mickeyyaya/worktree-concurrency-r1-tree-guard
- Merge pull request #75 from mickeyyaya/worktree-concurrency-r1-tree-guard
- Merge pull request #74 from mickeyyaya/worktree-concurrency-r1-tree-guard
- Merge pull request #73 from mickeyyaya/worktree-concurrency-r1-tree-guard
- Merge pull request #72 from mickeyyaya/worktree-concurrency-r1-tree-guard
- Merge pull request #71 from mickeyyaya/worktree-concurrency-r1-tree-guard


---

## [18.5.0] - 2026-06-11

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v18.4.0..HEAD._

### Added

- dynamic CLI-health layer — bench walls, parse reset hints, live-probe, canary, advisor visibility


---

## [18.4.0] - 2026-06-11

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v18.3.0..HEAD._

### Fixed

- ship guarantee — optional-phase infra skip, native-executor contract synthesis, build⇒audit∧ship floor
- worktree validity probe requires .git entry; bank cycles 282-283 salvage tests; commit-gate gofmt -s
- gofmt -s conformance for faketmux_amplify_test; inbox defect records (gofmt-gate mismatch, codex update-menu trail)
- tree-diff guard applies one classifier to all phases; porcelain emits both rename sides
- transport manifest SSOT + leak-site refactor + inserted-phase guard (salvage cycle-274)

### Other

- evolve-cycle 281: goal=c42f1120496040feda16c17ea482c2960eed9894ef4fb84afd2b28ec139aa11f
- evolve-cycle 276: goal=7424f17744ee64f9d648327fa496c69416f976dfee8e924ba2080f2ab5e07639


---

## [18.3.0] - 2026-06-10

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v18.2.0..HEAD._

### Added

- ADR-0045 slice 5 — I6 one dial, one rollout story
- ADR-0045 slice 4 — I3 AskBroker + I4 rule promotion
- ADR-0045 slice 3 — I5 full panetrust trust boundary
- ADR-0045 slice 2 — I2 graduated correction ladder
- ADR-0045 slice 1 — I1 interaction telemetry + I5 Digest core
- recovery chain of responsibility + one-dial composition (ADR-0044 C3, slice 6 — ADR Implemented)
- LLM failure advisor + Reflexion promotion loop (ADR-0044 slice 5)
- observer StallPolicy strategy seam (ADR-0044 C4, slice 4)
- CLI-version-freeze preflight + retro fallback chain (ADR-0044 C5+D4, slice 3)
- fatal-pane fast-fail + matrix-wide model-flag policy (ADR-0044 C2, slice 2)
- single-source phase-outcome recording chokepoint (ADR-0044 C1, slice 1)

### Fixed

- salvage cycle-270 — coverage tests + bootRCName exit-code diagnostic + 3 inbox defect reports
- ADR-0045 S2/S6 security hardening (independent review follow-up)
- add missing debugger profile — ship-recovery route died at launch (cycle-270 exit=10)
- challenge-token echo — inject at dispatch, gate at the correctable boundary (cycle-269)
- .evolve deliverable allowlist for build-leak relocation (cycles 262/268 class)
- exit 85 joins the default CLI-fallback triggers (cycle-267)
- non-dispatchable user phase skips loudly + memo phase config (cycle-265)
- triage joins the mandatory spine — the scope-clamp is kernel-owned (cycles 263/264)
- cycle-263 blocker triad — aliasing, inert-gate catalog, cwd leak
- add required YAML frontmatter to evolve-failure-advisor persona

### Documentation

- bring ADR-0045 docs up-to-date with the shipped implementation

### Other

- Merge pull request #70 from mickeyyaya/worktree-adr45-e2e
- Merge pull request #69 from mickeyyaya/worktree-adr45-e2e
- Merge pull request #68 from mickeyyaya/worktree-adr45-e2e
- Merge pull request #67 from mickeyyaya/worktree-adr-0045-slices
- evolve-cycle 266: goal=afae3d46ddbd5ff82147220d21236fec3b8df94b0baa51b2e1d13eaa2ccc8b62
- evolve-cycle 265: goal=afae3d46ddbd5ff82147220d21236fec3b8df94b0baa51b2e1d13eaa2ccc8b62


---

## [18.2.0] - 2026-06-09

### Added

- **Cold REPL-boot latency instrumentation (`boot_ms`, ADR-0043 A0).** Each tmux-REPL phase dispatch records the cold-boot window (fixed readiness sleeps + marker-poll) as an additive `boot_ms` field threaded `driver → core.BridgeResponse → core.PhaseResponse → phaseTimingEntry` (`phase-timing.json`, `omitempty`, behavior-neutral). The driver counts intended sleep/poll *iterations* rather than wall-clock so the value is deterministic under tests while ≈ wall-clock in prod; `OnBoot` fires once on the cold-boot marker, never on the warm named-session path or a boot timeout (both → `boot_ms=0`, itself a signal). This is the measurement gate for the pipeline-latency program — A1/A2 hot-loop changes stay gated on these numbers. The first measured cycle showed boot is ~1–3% of think-heavy phases, lowering A1/A2 priority (ADR-0043 §"A0 — as-built + first measurement"). (`12dcd18b`; tracked binary rebuilt `aacd95de`.)

### Performance

- **Test-suite latency pass (token-neutral, quality-preserving).** `t.Parallel()` on `internal/core` (196 tests, 10.5s→5.0s) and `internal/observer` (34 tests, 5.2s→1.6s); `testing.Short()` gates on 7 real-subprocess tests so `go test -short` skips them (audit `-short` 16s→0.42s) while CI-full still runs them; pre-existing `errcheck` in `rollback.go` fixed. (`9724461d`.)

### Documentation

- **ADR-0044 — Unified Phase Recovery Protocol** + `docs/architecture/phase-recovery.md`. From the cycle-262 post-mortem: a cycle whose work was done correctly recorded itself as a *failure* because a successful CLI fallback was never reconciled into orchestration. Documents the D1–D7 defect taxonomy, the root cause (recovery is an un-owned cross-cutting concern across 5 modules), AI-driven principles (deterministic-first/LLM-last), and a pattern-based solution (single-source verdict reconciliation, Template-Method terminal-state classification, Chain-of-Responsibility recovery pipeline, Observer+StallPolicy, Specification CLI-freeze preflight). Also records the codex self-update hazard (mitigated host-side via `brew pin codex`). (`29c457f7`.)
- **ADR-0043 — pipeline-latency program** (REPL boot reuse, measurement-gated) + `docs/architecture/pipeline-latency.md`: design, A0 as-built seam, and first measured boot-vs-think split. (`9ab6522c`.)
- **Ship-test parallelization dead-end** recorded in `go/docs/testing.md` — macOS EBADF FD-teardown flake under `-race -count=3 -shuffle`; the wall is git-subprocess-bound, so the only real lever is faking `git`. (`89c8c969`.)

## [18.1.0] - 2026-06-08

### Added

- **Pre-batch readiness gate for `evolve loop`.** A deterministic host-side gate (`go/internal/looppreflight`) runs after the unfinished-cycle guard and before the first cycle, verifying the pipeline can actually run via four accumulate-then-decide checks: **pipeline-structure** (every spine phase has a registry factory + phasecontract; every profile `cli`/`cli_fallback` resolves to a known driver), **llm-cli-status** (each distinct CLI binary present via `doctor.Probe`), **host-capabilities** (tmux + writable `.evolve`/`.evolve/runs` Halt; sandbox-degraded, low-disk, and stale `evolve-bridge-*` tmux sessions Warn), and **bridge-boot** (really boots each `-tmux` REPL via `bridge.BootSmokeTest` under a 90s budget). On any Halt the batch aborts BEFORE spending LLM budget — exit 2, `stop_reason=preflight_failed` (cycle=0), a persisted `.evolve/loop-preflight.json`, and a human Summary to stderr — catching the cycle-258 `ExitREPLBootTimeout` at batch start instead of ~30 min in. `EVOLVE_SKIP_PREFLIGHT=1` bypasses the whole gate; `EVOLVE_SKIP_PREFLIGHT_BOOT=1` runs the cheap checks but skips the real boot (CI/offline). Mirrors the `releasepreflight` Options→Run→Result seam blueprint; completes readiness-gate build-order steps 3–7 + 9 (steps 1/2/8 shipped in 18.0.0 prep as `bridge.BootSmokeTest` + `evolve doctor boot`).

### Changed

- **`bridge.ScrollbackTail`** centralizes the boot-pane scrollback-tail formatting that was duplicated between `evolve doctor boot` and the new readiness gate (single source).

## [18.0.0] - 2026-06-08

### Added

- **EGPS v11 — Go-native acceptance predicates (ADR-0042).** Acceptance criteria are now authored as Go tests (`//go:build acs`, `predicates_test.go`), not bash `acs/cycle-N/*.sh`. `evolve acs suite` runs three scopes every cycle — the current cycle (`go/acs/cycle<N>/`, authored fresh per cycle), the curated regression set (`go/acs/regression/`, standing), and the standing red-team anti-gaming predicates (`go/acs/redteam/`) — each as a separate `go test -json -tags acs` so a per-package compile error is a HARD suite error, never a silent PASS. The `red_count == 0 ⟺ PASS ⟺ ship_eligible` verdict invariant is unchanged. The red-team detection logic (`internal/redteamcheck`) is adversarially unit-tested in normal CI; a normal-suite guard (`acssuite.TestAllACSPredicatesAreTagged`) fails CI if any predicate file is missing `//go:build acs`.

### Changed

- **Retired the bash predicate runtime (BREAKING for predicate authors).** `acssuite` is now a Go-lane-only runner: the bash glob/exec lane (`discover`/`runBash`), `acs/lib/assert.sh`, ~408 dormant historical bash predicates, and the `evolve acs suite --no-go` flag are all removed; the `acs/` tree is gone. The authoring contract (`agents/evolve-{tdd-engineer,builder}.md` + `.evolve/profiles/`) now targets Go predicates, and the Builder is role-gate-denied from writing `go/acs/**`. The durable regression set was relocated to `go/acs/regression/`; the 3 red-team predicates were ported to Go. The Go lane is scoped to the current cycle + curated regression (never every historical cycle), keeping bit-rotted point-in-time predicates out of the gate. Supersedes the bash predicate format of egps-v10 / ADR-0025.

### Documentation

- **ADR-0042 (EGPS v11)** records the Go-native predicate contract; `docs/architecture/egps-v10.md` carries a superseded-in-part banner; `go/acs/README.md` documents the three-scope authoring model.

### Fixed

- **Reconcile bridge timeout against the deliverable** — `BaseRunner.Run` synthesized `verdict=FAIL` on *any* bridge error without reading the artifact the agent produced, so a complete PASS deliverable written just as the bridge gave up on the artifact-wait window (`ErrArtifactTimeout`, exit 81) was discarded and the cycle routed to retro instead of ship (the deeper root cause behind the cycle-254/255 false-FAILs — the busy-pane fix below extends a working agent; this closes the residual write-after-give-up race at the verdict-synthesis chokepoint). On `ErrArtifactTimeout` only, the runner now reconciles against the deliverable via an injectable `verifyFn` (`deliverable.Verify`): if it is on disk and well-formed, control falls through to the same artifact-read + `Classify` path the happy case uses — so audit's EGPS/`red_count` gate still decides (a PASS report with red predicates can never ship). A reconciled phase is a *completed* phase (nil error, the agent's own verdict authoritative); a reconciled FAIL routes as a real `code-audit-fail`, not an infra-timeout retry. Reconciliation only ever upgrades a synthesized FAIL toward the agent's real verdict — it never invents a PASS; malformed/absent/non-timeout cases keep prior behavior. `PhaseResponse.Reconciled` drives an auditable `reconciled_timeout` ledger entry (mirrors `backfill`).
- **Busy-pane liveness in the self-healing stop-reviewer** — a tmux-LLM phase whose agent was visibly mid-turn (per-CLI "esc to interrupt" affordance / vanished idle placeholder) but emitting no *substantive* pane delta was killed at review interval 0 with `exit=81` artifact timeout, then recorded as FAIL despite having written a valid deliverable. This hit Opus recovery-audits hardest: their long quiet extended-thinking renders only the `Deliberating Ns`/token-counter lines that `cleanPane` strips (ADR-0026 backlog #4), so `Progressed` read false while the agent was alive — producing false-FAIL cycles that halted batches in a self-perpetuating loop (post-FAIL → Opus audit → false FAIL → …). The deterministic reviewer now extends on `Progressed || Busy` (bounded by the same `maxExtends` backstop); `StopEvent.Busy` is populated from `panestream.PaneBusy` at each checkpoint. Codex (no busy signal) is unchanged. ADR-0026 addendum.

---

## [17.1.0] - 2026-06-07

### Added

- `evolve skills publish` — single-source projection of canonical skills to Codex CLI (`$CODEX_HOME/skills/evolve-*`), Antigravity/agy (native plugin install), and Ollama (Modelfile SYSTEM-prompt embedding, read-only skill subset). Stage-only by default; `--install` gates all user-environment mutation; provenance-marked prune; `--check` drift mode (ADR-0041).

---

## [17.0.0] - 2026-06-07

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v16.9.0..HEAD._

### Added

- single-source phase-facts projection — evolve skills generate|check (ADR-0040 Phase B)

### Changed

- no-stutter skill naming + single command surface (ADR-0040 Phase A)


---

## [16.9.0] - 2026-06-07

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v16.8.0..HEAD._

### Fixed

- heading-aware hasSection — bare require_sections rules match markdown headings (inbox classify-heading-prefix-mismatch); codex-tmux ChatGPT safe-model set refresh (gpt-5.5)

### Changed

- salvage cycle-250 — archetype defaults, profile tool-policy dedup, skillinventory isolation
- salvage cycle-249 — extract runner.BaseCycleContext, export specrunner.EvaluateClassify, EBADF test hardening


---

## [16.8.0] - 2026-06-07

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v16.7.0..HEAD._

### Added

- self-healing repair ladder — typed bounded repairs for stale TOFU pin, unpushed resume, colliders, push race (ADR-0039 §8)

### Fixed

- verdict parsers accept bare heading-form PASS/WARN — cycle-249 release-blocker shape
- pushDivergentCommit pins clone to main — CI runners lack init.defaultBranch

### Other

- Merge pull request #66 from mickeyyaya/worktree-failure-floor


---

## [16.7.0] - 2026-06-07

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v16.6.0..HEAD._

### Added

- generalized failure-signal contract — sentinel v2 (phase 6, ADR-0039 §7)
- land Wave-2 + Wave-3 catalog phases — operator re-land of cycle-247 audited work + batch retrospective
- deprecate EVOLVE_DISABLE_AUTO_RETROSPECTIVE + ADR-0039 (phase 5)
- decision rubric as a projection of structured routing data (phase 4b)
- failure_floor — one surface for the audit-FAIL learning route (phase 4a)
- advisor failure-path vocabulary above the failure floor (phase 3)
- deterministic failure floor — learning on ALL abnormal terminations (phase 2)
- deterministic learning-artifact substrate (failure-floor phase 1)
- tdd leak recovery, gitignored checkout protection, agy rating autorespond, and idle artifact nudge [worktree-build]
- domain phase catalog — 15 cross-domain phases + router integration (PR #64)
- integration wave — router recipes + core values for 5 domains (cycle-12 recovery) + campaign forensics D1-D12
- seed reproduce-bug — registered THROUGH evolve phases create (e2e proof)
- enriched SELECT catalog cards from phase metadata
- wire multi-root discovery into composition root, list, validate, lint
- evolve phases create — conversational phase registration with JSON envelope
- structured phase index — .evolve/phase-inventory.json
- multi-root phase discovery + EVOLVE_PHASE_ROOTS
- advisor-facing metadata fields — description, when_to_use, categories

### Fixed

- TestDiscardGitignore sets git identity env — CI parity
- trust-kernel guards fail closed — fallback to tracked go/evolve when go/bin is wiped
- HALT commit-presence predicates — resolve normalize-vs-commit-claims (inbox)
- resolve 7 persona/profile tool contradictions blocking manual ships
- enforceNext skip-advance must not rewrite staticNext to PhaseEnd on empty cfg.Order — cycle-240 e2e regression + unit pin
- land audited cycle-240 routing fixes — plan run:false vetoes triggers, missing signals fail closed, insertion cap on trigger inserts, REQUIRE_INTENT in advisory floor
- PR-review fixes — spec-exact signal names (opportunity.count, metric.inputs_count, postmortem.action_items_count), roll back v2 fail_if_signal gates to v1 shadow-first (spec 2.5), uniform verdict_on_pass on all 15
- carry failure learning into next cycle planning
- align registry test with cycle-217 insertions=6 + auditor persona back to 300-line budget
- remove cycle-231 residue accidentally swept into faafbab
- pin auditor ACS suite invocation to the active worktree (C0 block)
- plan-reviewer artifact name matches registry contract (plan-review-report.md)
- harden phases create against path traversal + unsafe rollback

### Documentation

- domain phase catalog — 5-domain research + design spec (15 phases, campaign waves)
- campaign retrospective cycles 215-231 — coherence-protocol diagnosis + 3-invariant architecture
- ADR-0038 + phase-plugin-system design doc + phase-create skill

### Other

- Merge pull request #65 from mickeyyaya/worktree-failure-floor
- Merge remote-tracking branch 'origin/main' into worktree-failure-floor
- Merge remote-tracking branch 'origin/main' into worktree-failure-floor
- Merge pull request #63 from mickeyyaya/feat/phase-plugin-system
- (merge) main → feat/phase-plugin-system — sync 6d7afbc + 7e54fd8; rebuild go/evolve from merged source
- (merge) main → feat/phase-plugin-system — resolve ADR-0038 × two-tier-naming collision
- evolve-cycle 242: goal=3ec7dbdcad6fef9573ec75f757f3a00f2c613de1eec26096cbbfd30744cc7677
- evolve-cycle 239: goal=463d9abdc6977990e52a446b6e8f45b9ecc4707251b7dfd3a5d7acd26896a3d7
- Merge remote-tracking branch 'origin/main' into feat/domain-phase-catalog
- evolve-cycle 10: goal=7ca0e4ed2d2ddceadac464bb6fc68511229c74831cdeb4a8d09d251746c062c9
- evolve-cycle 8: goal=26c1da42ae3a4028a839216ab6ddfada618486b1fcc596cecb7f3bbea5de994d
- evolve-cycle 6: goal=fd35d795a221a4e542b6ca2cbdf981a67de0fcccd981069dbe12d65afb1e9d1a
- evolve-cycle 5: goal=0baa137194b4d7540421ca4cbad29c525e69abe4ed7cc15b281f5caa2adf5ed4
- evolve-cycle 233: goal=57457c4c61eae1c400bd66ef6eef3b23b5d038e5bc5655d29c3fbadc129934cd


---

## [16.6.0] - 2026-06-06

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v16.5.0..HEAD._

### Added

- bidirectional live channel for long-running tmux-REPL phases (EVOLVE_CHANNEL) (#59)
- contract-correction retry — re-dispatch phase with violation directive (<=N, default 2) before abort

### Fixed

- bridge user-phase dispatch chain (personas + profiles) + first-run retrospective
- re-baseline 6 stale regression predicates blocking all cycle ships
- detect macOS Keychain claude OAuth in doctorAuth (no more false MISCONFIGURED)
- harden contract-correction loop — abort on non-canonical correction verdict + surface in cycle-health

### Changed

- two-tier naming rule + rename reproduce-bug -> bug-reproduction
- split CLAUDE.md runtime detail into docs/operations/runtime-reference.md (40k context limit)

### Documentation

- unified naming rule + per-phase core-value catalog
- micro-phase catalog — research + design for advisor-composable atomic phases
- PR↔ADR documentation audit + model-catalog design doc (retro-fills the PR #31 gap)
- TDD implementation plan for contract-correction retry
- design spec for contract-correction retry (≤2x re-dispatch)

### Other

- evolve-cycle 218: goal=0e9c1314a1ad382c8a216ddc4f2636dce717ba28399ee958dccd1c8c13f39c94
- evolve-cycle 217: goal=0e9c1314a1ad382c8a216ddc4f2636dce717ba28399ee958dccd1c8c13f39c94
- Merge pull request #62 from mickeyyaya/test/tmux-default-guard
- Merge pull request #60 from mickeyyaya/feat/contract-correction-retry
- Merge pull request #61 from mickeyyaya/fix/doctor-keychain-auth


---

## [16.5.0] - 2026-06-04

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v16.4.0..HEAD._

### Added

- derive deliverable contracts from declarative phase spec (ADR-0035)

### Other

- Merge pull request #58 from mickeyyaya/feat/config-driven-contracts


---

## [16.4.0] - 2026-06-03

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v16.3.1..HEAD._

### Added

- host-side gate + circuit breaker — Layer 4 (ADR-0034)
- unified deliverable contract — Layers 1/2/3/5 (ADR-0034)

### Fixed

- scope required verdict to audit only (not build/scout/tdd)
- setup detect/complete swallowed the next flag in space form

### Documentation

- ADR-0034 + operator guide + verified KB research; fix verify CLI flag parsing

### Other

- Merge feat/deliverable-contract: unified deliverable contract + self-check (ADR-0034)


---

## [16.3.1] - 2026-06-03

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v16.3.0..HEAD._

### Fixed

- write routing-plan.json to the absolute workspace path

### Other

- evolve-cycle 214: goal=8b77bd1e97069959a93eefdf458a82cea0d40633b45d7c965403dc09d317d096
- evolve-cycle 210: goal=8b77bd1e97069959a93eefdf458a82cea0d40633b45d7c965403dc09d317d096


---

## [16.3.0] - 2026-06-03

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v16.2.0..HEAD._

### Added

- goal-aware composition (Phase 3) + archive dead orchestrator (Phase 4)
- process-CPU liveness probe for headless phases (move #3)
- scope EGPS predicate go-test to changed packages (move #2)
- structural scout-eval + tdd-predicate-quality gates (move #5)
- unify the routing advisor onto the standard agent format (Phase 1)

### Fixed

- fire-once trust prompts so codex/agy-tmux don't loop-guard abandon
- harden pipeline-resilience set per holistic review (PRs #53/#54 follow-up)
- single-source phase report headings via phasecontract (ADR-0033)

### Documentation

- failure-class review of 4 loop-blockers + ADR-0033 structured verdicts
- document the dual-root invariant + guard test (move #4)

### Other

- Merge pull request #57 from mickeyyaya/fix/codex-tmux-trust-once
- Merge pull request #56 from mickeyyaya/feat/advisor-brain
- Merge pull request #55 from mickeyyaya/fix/review-hardening
- Merge pull request #54 from mickeyyaya/fix/verdict-contract-hardening
- Merge pull request #53 from mickeyyaya/feat/pipeline-resilience-gates


---

## [16.2.0] - 2026-06-03

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v16.1.0..HEAD._

### Added

- add architecture-design phase — the free-topology proof (Slice 7)
- specrunner fallback for advisor-selectable builtin spec phases (Slice 6)
- WS4 wire configurable integrity floor — audit-only posture (Slice 5)
- WS3 expose phase catalog to advisor — select over mint (Slice 4)
- WS2 KB recall memory into the advisor plan (Slice 3)
- smart-advisor framework walking skeleton (WS1-WS4 contracts)

### Fixed

- make plan-review dispatchable — resolve its persona via agent field
- tmux-pane liveness probe stops false stall_no_output on long tmux turns
- env-configurable per-predicate timeout (EVOLVE_ACS_PREDICATE_TIMEOUT_S)
- align verdict classifiers with current agent report templates
- EGPS predicates run with cwd=worktree + budget-unobservable warn

### Documentation

- port the genuinely-missing planner/code-reviewer patterns (Slice 8)

### Other

- Merge pull request #52 from mickeyyaya/fix/plan-review-dispatchable
- Merge pull request #51 from mickeyyaya/feat/spec-phase-runners-ws5
- Merge pull request #50 from mickeyyaya/feat/smart-advisor-framework
- Merge remote-tracking branch 'origin/main' into feat/smart-advisor-framework
- evolve-cycle 202: goal=490755773abcb1fc95de2cfd93abbc4dcb95202952763d2c0e7b23992a411c54
- evolve-cycle 197: goal=c494002bba4b236d591886f3ee29da45c6cea45dc41e4074e85c0bc624e08198


---

## [16.1.0] - 2026-06-02

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v16.0.0..HEAD._

### Added

- activate dormant turn_budget_hint profile field (#49)


---

## [16.0.0] - 2026-06-02

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v15.0.0..HEAD._

### Fixed

- drop removed `setup validate` from CLI usage + TEST_PLAN (Step 9b follow-up) (#47)

### Changed

- Step 9b — remove llm_config.json + repoint /setup at policy.json pins (#46)


---

## [15.0.0] - 2026-06-02

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v14.0.0..HEAD._

### Added

- Step 9a — remove llm_config from resolution path
- Reviewed-by trailer — durable per-commit review provenance
- surface incomplete packages (no terminal summary)
- advisor emits MintPhases — activate minting end-to-end
- advisor MintPhases + registration window (slice 12)
- Registrar mints a validated phase from PhaseConfig (slice 11-2)
- inline prompt body via optional InlinePromptProvider (slice 11-1)
- 10c — dispatch read-path (LoadManifest overlay, live-gated) + cycle-start TTL refresh hook
- wire `models refresh --source live` — bridge→modelquery, validated end-to-end (Step 10b-live wiring)
- live /model acquisition — ollama+REPL listers, LLM classifier, dispatch-source gate (Step 10b-live)
- evolve models refresh/list + detect-sourced catalog (Step 10b)
- live tier→model catalog data layer (Step 10a)
- unified PhaseConfig type (Step 6, option B)
- user-controlled rules + per-phase CLI/model pins
- per-phase usage sidecars + pause escalation reports
- add exponential retry backoff and phase latency health signal [worktree-build]
- fix backfill artifact paths, default-on backfill, and record attempt count [worktree-build]
- gitignore evals tracking, phase-max-attempts env override, stale docs self-heal [worktree-build]
- recover build-phase leaks into the worktree (cycle-160, Option A)

### Fixed

- apply mandatory_phases merge at both config-load sites
- mixed predicate is LOW (advisory) not HIGH unless window-dressing (issue #15, cycle-184)
- write role=builder agent_subprocess provenance entry after build (issue #13, cycle-181)
- resolve .evolve/ to MAIN project root for predicates run from a worktree (issue #12)
- pre-handoff runs native ACS suite + eval graders before claiming PASS (issue #10)
- recoverBuildLeak skips NESTED .evolve/ + git add -f; restore .evolve gitignore invariant (issue #11, cycle-176)
- audit phase runs with cwd=worktree (issue #9)
- mandatory eval-file materialization + STOP-CRITERION gate 6 (issue #8, cycle-166)
- retro bridge-fail non-fatal (GAP 9) + eval-slug Scout↔Build coordination (cycle-164)
- recoverBuildLeak skips .evolve/ runtime state + directory entries (415a9a7 CI regression)
- preserve non-Claude builder tracked-file edits + clear stale ACS reds (cycle-162)
- canonical challenge-token header in builder/tdd/scout templates (cycle-158)
- normalize worktree before audit so committing builders' work isn't discarded (cycle-156, Option C)
- agy has no -m flag — manifest model_tier→noop + CLI fallback resilience

### Changed

- extract RolloutStages from RoutingConfig god-object (Step 13)
- shared crash-safe write helper; dedup 5 copies + cover
- stub retry-backoff sleep in tests — core suite 254s→8s
- unify CLI+model dispatch resolution into one Plan
- typed envchain getters + centralize config smells (Stage 1)

### Documentation

- llm_config-removal migration plan + precedence-ordering oracle
- document real-tmux integration tests' load-sensitivity (phase 7)

### Other

- Merge pull request #40 from mickeyyaya/refactor/dedup-sweep
- Merge pull request #38 from mickeyyaya/refactor/atomic-write
- Merge pull request #36 from mickeyyaya/test/coverage-95
- Merge pull request #35 from mickeyyaya/refactor/test-architecture-unification
- evolve-cycle 188: goal=af51e7e30ddb0e3dc62c256386fa9ead7c29b7eebe58d343ce9a1d6b771bfd1e
- evolve-cycle 187: goal=af51e7e30ddb0e3dc62c256386fa9ead7c29b7eebe58d343ce9a1d6b771bfd1e
- evolve-cycle 186: goal=af51e7e30ddb0e3dc62c256386fa9ead7c29b7eebe58d343ce9a1d6b771bfd1e
- evolve-cycle 173: goal=af51e7e30ddb0e3dc62c256386fa9ead7c29b7eebe58d343ce9a1d6b771bfd1e
- evolve-cycle 171: goal=af51e7e30ddb0e3dc62c256386fa9ead7c29b7eebe58d343ce9a1d6b771bfd1e


---

## [14.0.0] - 2026-05-31

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v13.0.0..HEAD._

### Added

- multi-tmux-LLM-CLI subagent swarm harness (ADR-0032) (#29)
- add assert_go_build helper to acs/lib/assert.sh
- advisor-centric ship-error recovery and debugger phase
- tmux recipe engine + capability catalog (ADR-0031) (#27)
- cycle-124 modal-defense bundle + operator redirect
- auto-spawn phase-observer from evolve loop (cycle-122 Fix 3 / ADR-0030)
- any-CLI/any-model/any-phase pipeline (WS-G) (#26)
- liveness backstop + per-phase review gate (WS-E) (#25)
- ollama-tmux driver — local or cloud LLM via model tag (WS-F) (#24)
- commit-as-evidence trailer + git-evidence detector + rollout flag (ADR-0027 PR2) (#19)
- completion-detector strategy + advisor stdout contract (ADR-0027 PR1) (#18)
- hybrid cadence — Propose only at branch transitions (ADR-0024 PR-6) (#17)
- activate conditional integrity floor at Stage>=Advisory (ADR-0024 PR-5) (#16)
- conditional integrity floor — ClampPlanToFloor (ADR-0024 PR-4) (#15)
- ADR-0024 foundational slices — digest widening + PhaseAdvisor + whole-cycle plan (#14)
- user-definable phases — the Lego pipeline (#13)
- per-CLI native models + review hardening for /setup onboarding
- self-healing review layer — artifact wait reviews before stop (ADR-0026)

### Fixed

- TAP/automake SKIP (exit 77) for evidence-absent acs predicates
- headless drivers run source-writing phases with cwd=worktree + agent write-location contracts
- bind ship to the audited worktree CHANGES tree, not HEAD^{tree}
- orchestrator writes the auditor binding ledger entry — THE blocker for shipping
- default cycle commit message instead of hard-failing when Context lacks one
- grade audit on verdict CONTENT not heading FORMAT; self-heal scout artifact-timeout
- drive acs-suite red_count 88->0 — retire v12-deleted-bash + superseded-config predicates
- wire acs-verdict generator into the loop via shared NewDefault (cycle-147)
- escalate codex ChatGPT usage-limit banner instead of hanging (cycle-144)
- codex ChatGPT-account model clamp + rate-limit-nudge suppression (cycle-142)
- cycle-141 ExitArtifactTimeout — search worktree for artifact + tmux-aware stall signal
- generate acs-verdict.json when absent (cycle-138/139 EGPS blocker)
- green main + ACS predicate helpers + coverage index
- accept Go-native phase ledger vocabulary (cycle-137)
- PR 7 — preparePrompt reuses orchestrator-seeded challenge token
- PR 6 — mint challenge token + plumb to scout prompt
- Bug A — per-agent EVOLVE_<AGENT>_MODEL env key was phase-keyed
- extend WS-G default fallback trigger list to include exit=81 + 124 (cycle-122 Fix 2)
- pre-trust codex worktree+workspace paths in ~/.codex/config.toml (cycle-122 Fix 1)
- CLI-agnostic sandbox confinement + tree-diff guard (WS-B) (#23)
- warn on inert PhaseEnable=On under static routing (WS-C) (#22)
- optional-phase soft-fail + empty-output quota classify (WS-D) (#21)
- systemic absolute project-root via shared AbsoluteRoot helper (WS-A) (#20)
- resolve --project-root and --evolve-dir to absolute at composition root
- task-phase tmux captures use driver scrollback (alt-screen visibility)
- poll the artifact the agent actually writes (test-report.md, not team-context.md)
- tighten rate_limit auto-respond regex (false-positive escalation)
- make per-cycle worktree self-sufficient for trust-kernel hooks
- provision per-cycle worktree so source phases can write (restore v11 gap)
- write artifacts via $ARTIFACT_PATH + $CHALLENGE_TOKEN substitution
- list root-level CLAUDE.md + AGENTS.md in docs prefix scope

### Changed

- strip legacy rollback hatches + remove dead bash
- structured ShipError protocol foundation (ship-as-executor)
- PR 2 — abstract vocabulary normalization (fast/balanced/deep)

### Documentation

- Go-only CLAUDE.md/AGENTS.md rewrite + memory-subsystem distillation
- package doc.go (core) + guides/ extension cookbook
- Stage 2.5 reconcile — ship-error recovery + debugger phase
- architecture review + CLI driver contract (Stage 2.5 start)
- structured knowledge base distilled from ~148 cycles
- cycle-141 artifact-to-worktree + tmux false-stall root cause
- EGPS-verdict-not-generated blocker (cycle-138/139)
- CORRECT cycle-138 record — no missing[auditor] line existed
- record cycle-138 result + dispatcher-verify ordering bug
- regression coverage index — incident→test map + gap backlog
- PR 5 — centralize challenge-token contract in agent-templates.md
- PR 4 — scout prompt mandates challenge-token header
- PR 3 — TDD-engineer prompt closes cycle-131 audit gaps
- full-tmux-control reference (Task 10)
- cycle-123 V3 verification — codex per-edit-approval modal + empty-fallback-chain design gap
- cycle-122 modal stall + ADR-0030 observer auto-spawn + cycle-121 backlog flush
- cycle-119 incident report + LLM-CLI output-order/policy enforcement research
- cycle 109-116 Go meta-loop bring-up retrospective
- 0027 — orchestrator-curated merge-back (merge/drop/save)
- 0027 — commit-as-evidence is a universal invariant across ALL phases
- 0027 commit-as-evidence — git-anchored phase completion (replaces path-polling)
- sync operator reference with v13.0.0 features

### Other

- (merge) Go-only consolidation (ship-recovery, tiered tests, knowledge base, dead-bash removal) (#28)


---

## [13.0.0] - 2026-05-27

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v12.3.1..HEAD._

### Added

- apply adversarial-testing methodology (eval diversity, advisor rubric, auditor framing, ACS suite-runner + red-team) (#12)
- pre-commit review/lint/test gate + /commit + /release skills (#10)
- evolve cycle reset — seal an unfinished cycle, preserve history, advance (#9)
- live command injection + launch-time system prompt + observer nudge (ADR-0023) (#5)
- dynamic phase routing engine (gated off) + LLM proposer + scenario test framework (#4)
- wire LaunchIntent->Realizer into tmux drivers (ADR-0022 phase 2b)
- manifest params tables + RealizeFor for the 3 tmux CLIs (ADR-0022, phase 2a)
- LaunchIntent->Realizer framework for CLI-agnostic launch params (ADR-0022, phase 1)
- auto-reply policy for claude AskUserQuestion menus + 3-tier interactive suite
- billing-snapshot credential-isolation tool (snapshot+compare) at 100%
- human-input keystroke-plausibility layer (double-gated, 100%)
- doctor preflight (binary + auth + env-warnings + verdict) at 100%
- add-rule (manifest-patcher) + manifest override-dir layer
- report.go workspace summary + evolve bridge report/validate subcommands
- auto-respond fallback engine + escalation reports (replaces no-op)
- launch modes — validate-only, dry-run, require-full
- M7 part-2 — adapter cutover to in-process Engine behind EVOLVE_BRIDGE_GO
- M7 part-1 — evolve bridge CLI shim (launch|probe|version|help)
- M6 probe + embedded manifests — Engine.Probe completes core.Bridge
- Engine.Launch core.Bridge entry + ExtraFlags inner-argv pass-through
- M4 slice-2 — codex-tmux + agy-tmux via shared runTmuxREPL (DRY)
- M4 slice-1 — claude-tmux driver + TmuxController/Sleep seams
- credential-isolation cost-leak guards + codex model-map tests
- M2+M3+M5 — launch pipeline + claude-p/codex/agy drivers (slice-1 GREEN)
- M0 scaffold — native-Go agent-bridge package

### Fixed

- atomic relocate + surface relocate failure (review follow-up) (#7)
- tolerate + relocate non-canonical artifact writes (ADR-0024 Step 0) (#6)
- repoint auto-respond advisory to Go tests (bats suite deleted)
- make cross-CLI builder/auditor config consistent (pre-merge review)
- drop --bare from claude-tmux profiles (it hard-disables OAuth/subscription)

### Changed

- cold-move builder Output template to reference doc (#11)
- Phase 2c — centralize permission resolution, remove phaseflags
- address go-reviewer notes on the cutover
- remove EVOLVE_BRIDGE_GO flag + bash bridge — Go is the only path

### Documentation

- mark cli-adapters.md superseded + fix TEST_PLAN bridge row (post-cutover)
- 0021 — shadow-parity tier-1 PASS + operator cutover runbook
- 0021 — human-input + billing + add-rule + doctor now ported (100%)
- 0021 agent-bridge bash->Go port (decision + gated-cutover plan)

### Other

- (merge) post-cutover doc-rot cleanup (cli-adapters superseded banner, TEST_PLAN bridge row)
- (merge) repoint releasepreflight auto-respond advisory to Go tests (post-cutover fix)
- Merge feat/bridge-phase2c: centralize permission resolution, remove phaseflags
- Merge feat/bridge-cutover-hardening: remove EVOLVE_BRIDGE_GO flag + bash bridge
- Merge feat/agent-bridge-go-port: agent-bridge Go port + LaunchIntent->Realizer (ADR-0022)


---

## [12.3.1] - 2026-05-26

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v12.3.0..HEAD._

### Fixed

- repoint gate-tests from deleted bash suites to Go packages

### Other

- Merge pull request #3 from mickeyyaya/chore/ci-tooling-cleanup


---

## [12.3.0] - 2026-05-26

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v12.2.2..HEAD._

### Added

- wire events-stream producer into the pipeline (ADR-0020 task 5/5)
- filter kind==infra_failure from events stream (ADR-0020 task 4/5)
- repoint to unified events.ndjson stream (ADR-0020 task 3/5)
- live normalizer Poll core (ADR-0020 task 2/5)
- unified phase-event normalizer foundation (ADR-0020)

### Documentation

- mark ACCEPTED + cutover steps 1-5 done

### Other

- Merge pull request #2 from mickeyyaya/fix/ci-green
- Merge pull request #1 from mickeyyaya/worktree-phasestream-normalizer
- Merge branch 'main' into worktree-phasestream-normalizer


---

## [12.2.2] - 2026-05-26

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v12.1.0..HEAD._

### Added

- advisory rollout cycle 2 + filter-stdout testability
- stdout filter MVP + cycle-107 3-fix remediation
- v12.1.5 sprint 1 — extend_timeout + simulation harness + advisory preflight
- switch all default LLM dispatch from claude-p to claude-tmux
- build-planner wiring (opt-c shadow) + bridge/runner bug fixes

### Fixed

- cycle-108 three-bug hotfix series
- propagate EVOLVE_* OS env vars into cycle env
- add PhaseBuildPlanner to test runner maps + update CLI default expectation (claude-p→claude-tmux)
- remove fact_forcing_gate rule (live-smoke forensics)
- honor profile.cli + add disambiguating dispatch log
- SKILL.md resolver now finds go/evolve (not just go/bin/evolve)


---

## [12.2.1] - 2026-05-26

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v12.1.0..HEAD._

### Added

- stdout filter MVP + cycle-107 3-fix remediation
- v12.1.5 sprint 1 — extend_timeout + simulation harness + advisory preflight
- switch all default LLM dispatch from claude-p to claude-tmux
- build-planner wiring (opt-c shadow) + bridge/runner bug fixes

### Fixed

- propagate EVOLVE_* OS env vars into cycle env
- add PhaseBuildPlanner to test runner maps + update CLI default expectation (claude-p→claude-tmux)
- remove fact_forcing_gate rule (live-smoke forensics)
- honor profile.cli + add disambiguating dispatch log
- SKILL.md resolver now finds go/evolve (not just go/bin/evolve)


---

## [12.2.0] - 2026-05-26

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v12.1.0..HEAD._

### Added

- stdout filter MVP + cycle-107 3-fix remediation
- v12.1.5 sprint 1 — extend_timeout + simulation harness + advisory preflight
- switch all default LLM dispatch from claude-p to claude-tmux
- build-planner wiring (opt-c shadow) + bridge/runner bug fixes

### Fixed

- add PhaseBuildPlanner to test runner maps + update CLI default expectation (claude-p→claude-tmux)
- remove fact_forcing_gate rule (live-smoke forensics)
- honor profile.cli + add disambiguating dispatch log
- SKILL.md resolver now finds go/evolve (not just go/bin/evolve)


---

## [12.1.5] - 2026-05-26

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v12.1.0..HEAD._

### Added

- v12.1.5 sprint 1 — extend_timeout + simulation harness + advisory preflight
- switch all default LLM dispatch from claude-p to claude-tmux
- build-planner wiring (opt-c shadow) + bridge/runner bug fixes

### Fixed

- remove fact_forcing_gate rule (live-smoke forensics)
- honor profile.cli + add disambiguating dispatch log
- SKILL.md resolver now finds go/evolve (not just go/bin/evolve)


---

## [12.1.4] - 2026-05-25

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v12.1.0..HEAD._

### Added

- switch all default LLM dispatch from claude-p to claude-tmux
- build-planner wiring (opt-c shadow) + bridge/runner bug fixes

### Fixed

- honor profile.cli + add disambiguating dispatch log
- SKILL.md resolver now finds go/evolve (not just go/bin/evolve)


---

## [12.1.3] - 2026-05-25

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v12.1.0..HEAD._

### Added

- build-planner wiring (opt-c shadow) + bridge/runner bug fixes

### Fixed

- honor profile.cli + add disambiguating dispatch log
- SKILL.md resolver now finds go/evolve (not just go/bin/evolve)


---

## [12.1.2] - 2026-05-25

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v12.1.0..HEAD._

### Added

- build-planner wiring (opt-c shadow) + bridge/runner bug fixes

### Fixed

- SKILL.md resolver now finds go/evolve (not just go/bin/evolve)


---

## [12.1.0] - 2026-05-25

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.9.0..HEAD._

### Added

- Capability 2 — evolve compose --phases <list>
- Capability 1 CLI — evolve plan-and-execute <phase>
- port cycle-health-check.sh to internal/cyclehealth
- port verify-eval.sh to internal/verifyeval
- port eval-quality-check.sh to internal/evalqualitycheck
- port setup-skill-inventory.sh to internal/skillinventory
- introduce phases/runner + envchain + phase registry primitives
- Capability 1 slice 2 - phaseflags shared across all 6 phases
- Capability 1 slice 1 - plan-mode wired through build phase
- Capability 3 - auto-pick interactive policy (default-on)
- FLAG DAY - legacy/ removed, native Go pipeline complete

### Changed

- HasHelp helper + post-review fixes from simplifier/reviewer pass
- replace dispatch switch with subcommand registry table
- lift loopResult to file scope, add .emit() helper
- extract internal/paths package, kill 3x env-resolution duplication
- extract CostGuardDecorator from build phase
- collapse 6 phase runners onto BaseRunner+Hooks

### Documentation

- Phase 5 end-to-end verification report
- rewrite skill-file bodies for native evolve CLI
- add status banners to agent + skill files
- CHANGELOG


---

## [12.0.0] - 2026-05-24

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.9.0..HEAD._

### Added

- FLAG DAY - legacy/ removed, native Go pipeline complete


---

## [11.9.0] - 2026-05-24

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.8.3..HEAD._

### Added

- Phase 5 slice 5 - cli_adapters moved to top-level adapters/


---

## [11.8.3] - 2026-05-24

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.8.2..HEAD._

### Added

- Phase 5 slice 4 - native ship + heredoc-strip + direct guard routing


---

## [11.8.2] - 2026-05-24

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.8.1..HEAD._

### Added

- Phase 5 slice 3 - commit-prefix-gate + release-consistency


---

## [11.8.1] - 2026-05-24

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.8.0..HEAD._

### Added

- Phase 5 slice 2 - postedit-validate + inbox-mover ports


---

## [11.8.0] - 2026-05-24

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.7.5..HEAD._

### Added

- Phase 5 begins — port prune-ephemeral.sh + write audit doc


---

## [11.7.5] - 2026-05-24

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.7.4..HEAD._

### Added

- Phase 4 COMPLETE — port release-pipeline.sh to Go


---

## [11.7.4] - 2026-05-24

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.7.3..HEAD._

### Added

- Phase 4 — port rollback.sh to Go


---

## [11.7.3] - 2026-05-24

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.7.2..HEAD._

### Added

- Phase 4 — port preflight.sh to Go


---

## [11.7.2] - 2026-05-24

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.7.1..HEAD._

### Added

- Phase 4 — port marketplace-poll.sh to Go


---

## [11.7.1] - 2026-05-24

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.7.0..HEAD._

### Added

- Phase 4 — port version-bump.sh to Go


---

## [11.7.0] - 2026-05-24

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.6.6..HEAD._

### Added

- Phase 4 begins — port changelog-gen.sh to Go


---

## [11.6.6] - 2026-05-24

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.6.5..HEAD._

### Added

- Phase 3c — complete subagent-run.sh native parity


---

## [11.6.5] - 2026-05-24

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.6.4..HEAD._

### Added

- Phase 3c — port cmd_run production path to Go


---

## [11.6.4] - 2026-05-24

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.6.3..HEAD._

### Added

- Phase 3c — port subagent-run.sh helper functions to Go


---

## [11.6.3] - 2026-05-24

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.6.2..HEAD._

### Added

- Phase 3c — port cmd_validate_profile to Go


---

## [11.6.2] - 2026-05-24

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.6.1..HEAD._

### Added

- Phase 3c — port subagent prompt helpers to Go


---

## [11.6.1] - 2026-05-23

### Added

- **Phase 3c partial — 2 of 3 XL dispatch scripts ported to Go.**
- `evolve preflight-environment [--json|--summary|--write]` (package `internal/preflight`, 87.7% coverage) — single capability-detection probe with schema v3 output: host (os/version/shell), claude_code (nested), sandbox (sandbox-exec/bwrap capability + reasoning), filesystem (writability probes), cli_binaries (claude/gemini/codex/agy/jq/git paths), auto_config (worktree base selection by priority order, inner_sandbox decision, EVOLVE_SANDBOX_FALLBACK_ON_EPERM derivation).
- `evolve phase-observer [--enforce] [--scope=cycle|phase] <ws> <pgid> <cycle> <phase> <agent>` (package `internal/phaseobserver`, 87.6% coverage) — primary stall-detection observer (v10.18.0+ default). Tails stream-json stdout.log, parses assistant/user/result/rate_limit_event types, maintains in-memory state (event_count, tool_call_count, error_count, cumulative_cost, cache tokens), runs stall detection every poll, emits NDJSON events to `<agent>-observer-events.ndjson`, writes final report to `<agent>-observer-report.json` on SIGUSR1 / EOF grace expiry. SCOPE NOTE: the 4 secondary detection rules (infinite_loop, error_spike, cost_anomaly, rate_limit) are stubbed — operators needing the full rule engine should keep `EVOLVE_OBSERVER_ENFORCE=0` to fall back to bash until the rule-engine port lands in v11.7.

### Remaining for v12

- **Phase 3c: subagent-run.sh** (90KB) — the trust-kernel dispatcher. Token generation + role-gate + ledger writing + sandbox wrapping + prompt construction + response parsing. Genuinely multi-session work; planned v11.7.
- **Phase 4: native release CLI** (replaces release-pipeline.sh + version-bump.sh + changelog-gen.sh).
- **Phase 5: v12.0.0 flag day** (rm -rf legacy/ after consumer migration audit).

## [11.6.0] - 2026-05-23

### Added

- **Phase 3b complete** — all 4 medium dispatch scripts ported to Go.
- `evolve cycle-simulator <cycle> <workspace>` (package `internal/cyclesimulator`, 80.6% coverage) — no-LLM cycle-plumbing simulator that walks every phase, writes deterministic artifacts, and appends ledger entries with byte-stable key order (load-bearing for SHA chain verification).
- `evolve phase-watchdog <ws> <pgid> <cycle> <state>` (package `internal/phasewatchdog`, 90.8% coverage) — activity-based stall detector with v9.4.0 phase-aware baseline. Native Go signal handling via `syscall.Kill(-pgid, SIGTERM/SIGKILL)`.
- `evolve aggregator <phase> <out> <workers...>` (package `internal/aggregator`, 95.6% coverage) — pure-shell merge replacement for all 5 modes: concat (scout), verdict (audit), lessons (retrospective dedup), plan-review (multi-lens score), cross-cli-vote (MAJORITY-PASS with FAIL-VETO).
- `evolve fanout-dispatch [--cache-prefix-file=PATH] <cmds> <results>` (package `internal/fanoutdispatch`, 82.5% coverage) — bounded-concurrency parallel worker dispatcher using goroutines + semaphore channel + per-worker `context.WithTimeout`. Supports optional consensus-cancel polling (EVOLVE_FANOUT_CANCEL_ON_CONSENSUS=1).

### Status

- Phase 3a + 3b complete (7+4=11 dispatch scripts native Go).
- Remaining for v12 flag day: Phase 3c (preflight-environment, phase-observer, subagent-run — 3 XL scripts), Phase 4 (release CLI), consumer migration audit.

## [11.5.10] - 2026-05-23

### Added

- Phase 3a — port the final 3 small dispatch scripts to Go: `build-invocation-context.sh`, `resolve-llm.sh`, `consensus-dispatch.sh` (5/7, 6/7, 7/7 — Phase 3a complete).
- `evolve build-invocation-context <role>` (package `internal/bedrock`, 100% coverage) — byte-identical-per-role bedrock emitter; load-bearing for Anthropic prompt-cache reuse.
- `evolve resolve-llm <role> [config_path]` (package `internal/resolvellm`, 90.9% coverage) — pure-function LLM router with `llm_config.phases > _fallback > profile` precedence; JSON wire-shape identical to bash.
- `evolve consensus-dispatch` (package `internal/consensusdispatch`, 86.3% coverage) — cross-CLI consensus auditor (env-driven). Native validation/profile-parsing/voter-filtering/TSV-build; shells out to `fanout-dispatch.sh` + `aggregator.sh` + `_capability-check.sh` until those land in Phase 3b.

### Tests

- Byte-parity tests vs bash original for `bedrock.Emit()` across 7 roles and `resolvellm.Resolve()` across 5 scenarios (skipped if bash/jq unavailable).
- End-to-end Run() test for `consensusdispatch` driving the full pipeline with stub bash fakes (capability-check, two CLI adapters, fanout-dispatch, aggregator) — verifies PASS/FAIL propagation + worker artifact handling.

## [11.5.9] - 2026-05-23

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.5.8..HEAD._

### Added

- Phase 3a — port estimate-quota-reset.sh to Go (4/7)


---

## [11.5.8] - 2026-05-23

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.5.7..HEAD._

### Added

- Phase 3a — port detect-nested-claude.sh + list-phase-order.sh to Go

### Documentation

- CHANGELOG entry


---

## [11.5.7] - 2026-05-23

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.5.6..HEAD._

### Added

- Phase 3a — port detect-cli.sh to native Go (evolve detect-cli)


---

## [11.5.6] - 2026-05-23

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.5.5..HEAD._

### Documentation

- CHANGELOG entry


---

## [11.5.5] - 2026-05-23

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.5.4..HEAD._

### Other

- (no commits found in range; placeholder entry)


---

## [11.5.4] - 2026-05-23

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.5.3..HEAD._

### Documentation

- CHANGELOG entry for Phase 1 EGPS predicate ports


---

## [11.5.3] - 2026-05-23

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.5.2..HEAD._

### Added

- port 29 EGPS predicate cycles to Go (Phase 1 of v12 roadmap)


---

## [11.5.2] - 2026-05-23

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits d08d395..HEAD._

### Fixed

- subagent.composePrompt section markers + v11.4.0 release backfill


---

## [11.5.1] - 2026-05-23

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.5.0..HEAD._

### Added

- close all 7 bash-dispatcher parity gaps + 100% coverage


---

## [11.5.0] - 2026-05-23

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v11.3.0..HEAD._

### Added

- skill switch + bash dispatcher archive (cutover)
- cost accumulation + checkpoint thresholds + lifecycle
- verification + classification + observer + circuit breaker
- failure-adapter wiring + resume protocol
- native Go subagent runner + intent gate wiring
- goalhash package + CLI surface for evolve loop
- hardened guard-dispatch shim wires hooks to native Go (#16)
- byte-equivalent audit-trail writes from native Go guard CLI


---

## [11.3.0] - 2026-05-23

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v10.18.1..HEAD._

### Added

- native Go ship phase + evolve ship CLI
- --simulate flag + parity-audit --full mode + perf-cycle-comparison.sh (v11.1.0 prep) --- ## Actual diff (v8.34.0+)
- tasks #18-#21 — parity-audit harness + perf benchmarks + migration docs + Go-binary tier-1 promotion (7/7 parity tests PASS, 5 bench targets, EVOLVE_USE_LEGACY_BASH rollback hatch) --- ## Actual diff (v8.34.0+)
- Phase 3 task #17 — serve-phase subcommand + plugin shim + cross-compile dist (7 unit + 2 e2e PASS, 4/4 cross-arch artifacts) --- ## Actual diff (v8.34.0+)
- Phase 3 task #16 — subprocess wire protocol (Envelope+WireError+SubprocessRunner+ServeStdio, 34/34 PASS, 96.6% cov) --- ## Actual diff (v8.34.0+)
- Phase 3 task #15 — bulk-port remaining 98 ACS predicates (Go; total 120/120)
- Phase 2 task #14 — port cycle-43 ACS predicates (10 → Go; total 22/272) --- ## Actual diff (v8.34.0+)
- Phase 2 task #13 — wire phase|cycle|worktree|loop subcommands + orchestrator runners --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #12d — retro phase + failure-lesson check (98.2% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #12c — ship phase via ship.sh subprocess (95.0% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #12b — audit + EGPS gate (97.0% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #12a — build phase + cost guard (96.8% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #11d — tdd phase glue (98.0% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #11c — triage phase glue (96.4% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #11b — scout phase glue (98.0% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #11a — intent phase glue (98.1% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #10 — in-process + subprocess PhaseRunner (97.1% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #9 — phase observer goroutine (95.2% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #8 — SBPL + bwrap argv generators (96.4% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #7 — agent-bridge subprocess adapter (97.4% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #6 — pre-emptive cycle checkpoint (100% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #5 — reflection journal reader/writer (98.0% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #4 — agent profile loader (96.4% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #3 — fs.FS-backed prompt loader (98.1% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #2 — port failure-adapter.sh (100% cov) --- ## Actual diff (v8.34.0+)
- Phase 1 task #17 — 12 ACS predicates ported (cycle 42/53/104), all PASS --- ## Actual diff (v8.34.0+)
- Phase 1 task #17 — wire doctor/guard/ledger/acs subcommands --- ## Actual diff (v8.34.0+)
- GREEN — Phase 1 task #17 — probe-tool port (98.2% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 1 task #16 — go-test JSON aggregator
- GREEN — Phase 1 task #15 — assertion DSL implementation
- GREEN — Phase 1 task #14 — 5 trust-kernel guards
- GREEN — Phase 1 task #13 — chain guard wraps ledger.Verify
- GREEN — Phase 1 task #12 — JSONL append + SHA256 chain + tip
- GREEN — Phase 1 task #11 — atomic JSON + flock under user-mandated ≥95% gate
- GREEN — Phase 1 task #10 — lifecycle sequencer
- GREEN — Phase 1 task #9 — sentinels + Phase + ports + state machine
- GREEN — Phase 1 task #8 — byte-exact PROJECT_HASH port
- GREEN — Phase 1 task #7 — sidecar writer + slog phase emitter
- GREEN — Phase 1 task #6 — ldflag + BuildInfo composition
- Phase 1 task #5 — go module + Makefile + CI workflow + version stub
- lifecycle wiring + dashboard hot-spots (11/11 tasks, feature complete)
- retrospective + memo consume reflector synthesis (8/11 tasks)
- 9 phase agents + Auditor sycophancy defect (7/11 tasks)
- evolve-reflector persona (Learn-phase synthesizer, 4/11 tasks)
- post-audit driver fixes + comprehensive README rewrite
- shared schema + cross-cycle aggregator (foundation, 3/11 tasks)
- flip EVOLVE_BUILD_PLANNER advisory mode default-on (cycle 2 of 3, ADR-0019, v10.20)
- shadow-wire build-planner phase between TDD and Build (cycle 1 of 3, ADR-0019) --- ## Actual diff (v8.34.0+)
- v0.5b driver-level session-name + resume (claude-tmux)
- v0.5a session-name foundation (orchestration only)
- v0.4 orphan tmux-session sweep on launch
- v0.3 --stream-output for realtime JSONL pass-through
- plan-mode support via --permission-mode pass-through
- auto-skip Triage on trivial cycles with predicate-dep guard
- wire EVOLVE_FANOUT_AUDITOR through dispatch-parallel verb
- B-W1 — install hardening for system-command use
- F5 empty-prompt guard + F6 workspace-reuse WARN + edge fixtures
- W1 — JSON contract + cross-CLI + skill-flow E2E
- human-input behavioral plausibility mode (H1-H5)

### Fixed

- depth-1 scripts use ../.. for REPO_ROOT (release-pipeline + parity-audit + perf-comparison) --- ## Actual diff (v8.34.0+)
- use major.minor form for Current marker (release.sh consistency check) --- ## Actual diff (v8.34.0+)
- port soft-start boundary semantics; TEST_PLAN parity notes --- ## Actual diff (v8.34.0+)
- enable bridge stream_output for all 14 phase-agent profiles
- reject permission_mode on codex/agy drivers + TDD coverage
- install.sh COPY mode for branch-independent installs (#56)

### Changed

- Phase C+D+E — manifests/hooks/agents/skills/docs use legacy/scripts/, symlink removed, ACS Go predicates updated (4/4 preflight green; full Go regression clean) --- ## Actual diff (v8.34.0+)
- Phase A+B — Go defaults + ~196 bash files use legacy/scripts/ paths (resolve-roots walk-up fix; ../../.. arithmetic; 4/4 preflight gates green via direct path) --- ## Actual diff (v8.34.0+)
- physical move scripts/ -> legacy/scripts/ with backcompat symlink (v11.1.0 prep; all 4 preflight suites + cycle93 ACS green) --- ## Actual diff (v8.34.0+)

### Documentation

- roadmap for sub-release path to v12.0.0 — native ship/guards/dispatch/release, gotcha catalog from v11.x work --- ## Actual diff (v8.34.0+)
- fix migration table + roadmap doc + README major.minor for v11.2 release --- ## Actual diff (v8.34.0+)
- v11.2.0/v12.0.0 roadmap (defers symlink removal + Go-only cutover; both need dedicated sessions) --- ## Actual diff (v8.34.0+)
- note physical move + symlink in README/AGENTS/CLAUDE/migration-from-bash --- ## Actual diff (v8.34.0+)
- Phase 1 task #17 deliverable — TEST_PLAN.md (contract-level pass) --- ## Actual diff (v8.34.0+)
- design docs + CLAUDE.md env-var row (5/11 tasks)
- README integration section reflects default-on flip

### Other

- Merge branch 'bridge-local' into main


---

## [11.2.0] - 2026-05-23

_Generated by `legacy/scripts/release/changelog-gen.sh` from commits v10.18.1..HEAD._

### Added

- --simulate flag + parity-audit --full mode + perf-cycle-comparison.sh (v11.1.0 prep) --- ## Actual diff (v8.34.0+)
- tasks #18-#21 — parity-audit harness + perf benchmarks + migration docs + Go-binary tier-1 promotion (7/7 parity tests PASS, 5 bench targets, EVOLVE_USE_LEGACY_BASH rollback hatch) --- ## Actual diff (v8.34.0+)
- Phase 3 task #17 — serve-phase subcommand + plugin shim + cross-compile dist (7 unit + 2 e2e PASS, 4/4 cross-arch artifacts) --- ## Actual diff (v8.34.0+)
- Phase 3 task #16 — subprocess wire protocol (Envelope+WireError+SubprocessRunner+ServeStdio, 34/34 PASS, 96.6% cov) --- ## Actual diff (v8.34.0+)
- Phase 3 task #15 — bulk-port remaining 98 ACS predicates (Go; total 120/120)
- Phase 2 task #14 — port cycle-43 ACS predicates (10 → Go; total 22/272) --- ## Actual diff (v8.34.0+)
- Phase 2 task #13 — wire phase|cycle|worktree|loop subcommands + orchestrator runners --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #12d — retro phase + failure-lesson check (98.2% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #12c — ship phase via ship.sh subprocess (95.0% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #12b — audit + EGPS gate (97.0% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #12a — build phase + cost guard (96.8% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #11d — tdd phase glue (98.0% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #11c — triage phase glue (96.4% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #11b — scout phase glue (98.0% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #11a — intent phase glue (98.1% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #10 — in-process + subprocess PhaseRunner (97.1% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #9 — phase observer goroutine (95.2% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #8 — SBPL + bwrap argv generators (96.4% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #7 — agent-bridge subprocess adapter (97.4% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #6 — pre-emptive cycle checkpoint (100% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #5 — reflection journal reader/writer (98.0% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #4 — agent profile loader (96.4% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #3 — fs.FS-backed prompt loader (98.1% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #2 — port failure-adapter.sh (100% cov) --- ## Actual diff (v8.34.0+)
- Phase 1 task #17 — 12 ACS predicates ported (cycle 42/53/104), all PASS --- ## Actual diff (v8.34.0+)
- Phase 1 task #17 — wire doctor/guard/ledger/acs subcommands --- ## Actual diff (v8.34.0+)
- GREEN — Phase 1 task #17 — probe-tool port (98.2% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 1 task #16 — go-test JSON aggregator
- GREEN — Phase 1 task #15 — assertion DSL implementation
- GREEN — Phase 1 task #14 — 5 trust-kernel guards
- GREEN — Phase 1 task #13 — chain guard wraps ledger.Verify
- GREEN — Phase 1 task #12 — JSONL append + SHA256 chain + tip
- GREEN — Phase 1 task #11 — atomic JSON + flock under user-mandated ≥95% gate
- GREEN — Phase 1 task #10 — lifecycle sequencer
- GREEN — Phase 1 task #9 — sentinels + Phase + ports + state machine
- GREEN — Phase 1 task #8 — byte-exact PROJECT_HASH port
- GREEN — Phase 1 task #7 — sidecar writer + slog phase emitter
- GREEN — Phase 1 task #6 — ldflag + BuildInfo composition
- Phase 1 task #5 — go module + Makefile + CI workflow + version stub
- lifecycle wiring + dashboard hot-spots (11/11 tasks, feature complete)
- retrospective + memo consume reflector synthesis (8/11 tasks)
- 9 phase agents + Auditor sycophancy defect (7/11 tasks)
- evolve-reflector persona (Learn-phase synthesizer, 4/11 tasks)
- post-audit driver fixes + comprehensive README rewrite
- shared schema + cross-cycle aggregator (foundation, 3/11 tasks)
- flip EVOLVE_BUILD_PLANNER advisory mode default-on (cycle 2 of 3, ADR-0019, v10.20)
- shadow-wire build-planner phase between TDD and Build (cycle 1 of 3, ADR-0019) --- ## Actual diff (v8.34.0+)
- v0.5b driver-level session-name + resume (claude-tmux)
- v0.5a session-name foundation (orchestration only)
- v0.4 orphan tmux-session sweep on launch
- v0.3 --stream-output for realtime JSONL pass-through
- plan-mode support via --permission-mode pass-through
- auto-skip Triage on trivial cycles with predicate-dep guard
- wire EVOLVE_FANOUT_AUDITOR through dispatch-parallel verb
- B-W1 — install hardening for system-command use
- F5 empty-prompt guard + F6 workspace-reuse WARN + edge fixtures
- W1 — JSON contract + cross-CLI + skill-flow E2E
- human-input behavioral plausibility mode (H1-H5)

### Fixed

- depth-1 scripts use ../.. for REPO_ROOT (release-pipeline + parity-audit + perf-comparison) --- ## Actual diff (v8.34.0+)
- use major.minor form for Current marker (release.sh consistency check) --- ## Actual diff (v8.34.0+)
- port soft-start boundary semantics; TEST_PLAN parity notes --- ## Actual diff (v8.34.0+)
- enable bridge stream_output for all 14 phase-agent profiles
- reject permission_mode on codex/agy drivers + TDD coverage
- install.sh COPY mode for branch-independent installs (#56)

### Changed

- Phase C+D+E — manifests/hooks/agents/skills/docs use legacy/scripts/, symlink removed, ACS Go predicates updated (4/4 preflight green; full Go regression clean) --- ## Actual diff (v8.34.0+)
- Phase A+B — Go defaults + ~196 bash files use legacy/scripts/ paths (resolve-roots walk-up fix; ../../.. arithmetic; 4/4 preflight gates green via direct path) --- ## Actual diff (v8.34.0+)
- physical move scripts/ -> legacy/scripts/ with backcompat symlink (v11.1.0 prep; all 4 preflight suites + cycle93 ACS green) --- ## Actual diff (v8.34.0+)

### Documentation

- fix migration table + roadmap doc + README major.minor for v11.2 release --- ## Actual diff (v8.34.0+)
- v11.2.0/v12.0.0 roadmap (defers symlink removal + Go-only cutover; both need dedicated sessions) --- ## Actual diff (v8.34.0+)
- note physical move + symlink in README/AGENTS/CLAUDE/migration-from-bash --- ## Actual diff (v8.34.0+)
- Phase 1 task #17 deliverable — TEST_PLAN.md (contract-level pass) --- ## Actual diff (v8.34.0+)
- design docs + CLAUDE.md env-var row (5/11 tasks)
- README integration section reflects default-on flip

### Other

- Merge branch 'bridge-local' into main


---

## [11.1.0] - 2026-05-23

_Generated by `scripts/release/changelog-gen.sh` from commits v10.18.1..HEAD._

### Added

- --simulate flag + parity-audit --full mode + perf-cycle-comparison.sh (v11.1.0 prep) --- ## Actual diff (v8.34.0+)
- tasks #18-#21 — parity-audit harness + perf benchmarks + migration docs + Go-binary tier-1 promotion (7/7 parity tests PASS, 5 bench targets, EVOLVE_USE_LEGACY_BASH rollback hatch) --- ## Actual diff (v8.34.0+)
- Phase 3 task #17 — serve-phase subcommand + plugin shim + cross-compile dist (7 unit + 2 e2e PASS, 4/4 cross-arch artifacts) --- ## Actual diff (v8.34.0+)
- Phase 3 task #16 — subprocess wire protocol (Envelope+WireError+SubprocessRunner+ServeStdio, 34/34 PASS, 96.6% cov) --- ## Actual diff (v8.34.0+)
- Phase 3 task #15 — bulk-port remaining 98 ACS predicates (Go; total 120/120)
- Phase 2 task #14 — port cycle-43 ACS predicates (10 → Go; total 22/272) --- ## Actual diff (v8.34.0+)
- Phase 2 task #13 — wire phase|cycle|worktree|loop subcommands + orchestrator runners --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #12d — retro phase + failure-lesson check (98.2% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #12c — ship phase via ship.sh subprocess (95.0% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #12b — audit + EGPS gate (97.0% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #12a — build phase + cost guard (96.8% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #11d — tdd phase glue (98.0% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #11c — triage phase glue (96.4% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #11b — scout phase glue (98.0% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #11a — intent phase glue (98.1% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #10 — in-process + subprocess PhaseRunner (97.1% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #9 — phase observer goroutine (95.2% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #8 — SBPL + bwrap argv generators (96.4% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #7 — agent-bridge subprocess adapter (97.4% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #6 — pre-emptive cycle checkpoint (100% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #5 — reflection journal reader/writer (98.0% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #4 — agent profile loader (96.4% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #3 — fs.FS-backed prompt loader (98.1% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #2 — port failure-adapter.sh (100% cov) --- ## Actual diff (v8.34.0+)
- Phase 1 task #17 — 12 ACS predicates ported (cycle 42/53/104), all PASS --- ## Actual diff (v8.34.0+)
- Phase 1 task #17 — wire doctor/guard/ledger/acs subcommands --- ## Actual diff (v8.34.0+)
- GREEN — Phase 1 task #17 — probe-tool port (98.2% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 1 task #16 — go-test JSON aggregator
- GREEN — Phase 1 task #15 — assertion DSL implementation
- GREEN — Phase 1 task #14 — 5 trust-kernel guards
- GREEN — Phase 1 task #13 — chain guard wraps ledger.Verify
- GREEN — Phase 1 task #12 — JSONL append + SHA256 chain + tip
- GREEN — Phase 1 task #11 — atomic JSON + flock under user-mandated ≥95% gate
- GREEN — Phase 1 task #10 — lifecycle sequencer
- GREEN — Phase 1 task #9 — sentinels + Phase + ports + state machine
- GREEN — Phase 1 task #8 — byte-exact PROJECT_HASH port
- GREEN — Phase 1 task #7 — sidecar writer + slog phase emitter
- GREEN — Phase 1 task #6 — ldflag + BuildInfo composition
- Phase 1 task #5 — go module + Makefile + CI workflow + version stub
- lifecycle wiring + dashboard hot-spots (11/11 tasks, feature complete)
- retrospective + memo consume reflector synthesis (8/11 tasks)
- 9 phase agents + Auditor sycophancy defect (7/11 tasks)
- evolve-reflector persona (Learn-phase synthesizer, 4/11 tasks)
- post-audit driver fixes + comprehensive README rewrite
- shared schema + cross-cycle aggregator (foundation, 3/11 tasks)
- flip EVOLVE_BUILD_PLANNER advisory mode default-on (cycle 2 of 3, ADR-0019, v10.20)
- shadow-wire build-planner phase between TDD and Build (cycle 1 of 3, ADR-0019) --- ## Actual diff (v8.34.0+)
- v0.5b driver-level session-name + resume (claude-tmux)
- v0.5a session-name foundation (orchestration only)
- v0.4 orphan tmux-session sweep on launch
- v0.3 --stream-output for realtime JSONL pass-through
- plan-mode support via --permission-mode pass-through
- auto-skip Triage on trivial cycles with predicate-dep guard
- wire EVOLVE_FANOUT_AUDITOR through dispatch-parallel verb
- B-W1 — install hardening for system-command use
- F5 empty-prompt guard + F6 workspace-reuse WARN + edge fixtures
- W1 — JSON contract + cross-CLI + skill-flow E2E
- human-input behavioral plausibility mode (H1-H5)

### Fixed

- use major.minor form for Current marker (release.sh consistency check) --- ## Actual diff (v8.34.0+)
- port soft-start boundary semantics; TEST_PLAN parity notes --- ## Actual diff (v8.34.0+)
- enable bridge stream_output for all 14 phase-agent profiles
- reject permission_mode on codex/agy drivers + TDD coverage
- install.sh COPY mode for branch-independent installs (#56)

### Changed

- physical move scripts/ -> legacy/scripts/ with backcompat symlink (v11.1.0 prep; all 4 preflight suites + cycle93 ACS green) --- ## Actual diff (v8.34.0+)

### Documentation

- note physical move + symlink in README/AGENTS/CLAUDE/migration-from-bash --- ## Actual diff (v8.34.0+)
- Phase 1 task #17 deliverable — TEST_PLAN.md (contract-level pass) --- ## Actual diff (v8.34.0+)
- design docs + CLAUDE.md env-var row (5/11 tasks)
- README integration section reflects default-on flip

### Other

- Merge branch 'bridge-local' into main


---

## [11.0.0] - 2026-05-23

_Generated by `scripts/release/changelog-gen.sh` from commits v10.18.1..HEAD._

### Added

- tasks #18-#21 — parity-audit harness + perf benchmarks + migration docs + Go-binary tier-1 promotion (7/7 parity tests PASS, 5 bench targets, EVOLVE_USE_LEGACY_BASH rollback hatch) --- ## Actual diff (v8.34.0+)
- Phase 3 task #17 — serve-phase subcommand + plugin shim + cross-compile dist (7 unit + 2 e2e PASS, 4/4 cross-arch artifacts) --- ## Actual diff (v8.34.0+)
- Phase 3 task #16 — subprocess wire protocol (Envelope+WireError+SubprocessRunner+ServeStdio, 34/34 PASS, 96.6% cov) --- ## Actual diff (v8.34.0+)
- Phase 3 task #15 — bulk-port remaining 98 ACS predicates (Go; total 120/120)
- Phase 2 task #14 — port cycle-43 ACS predicates (10 → Go; total 22/272) --- ## Actual diff (v8.34.0+)
- Phase 2 task #13 — wire phase|cycle|worktree|loop subcommands + orchestrator runners --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #12d — retro phase + failure-lesson check (98.2% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #12c — ship phase via ship.sh subprocess (95.0% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #12b — audit + EGPS gate (97.0% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #12a — build phase + cost guard (96.8% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #11d — tdd phase glue (98.0% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #11c — triage phase glue (96.4% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #11b — scout phase glue (98.0% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #11a — intent phase glue (98.1% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #10 — in-process + subprocess PhaseRunner (97.1% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #9 — phase observer goroutine (95.2% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #8 — SBPL + bwrap argv generators (96.4% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #7 — agent-bridge subprocess adapter (97.4% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #6 — pre-emptive cycle checkpoint (100% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #5 — reflection journal reader/writer (98.0% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #4 — agent profile loader (96.4% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #3 — fs.FS-backed prompt loader (98.1% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 2 task #2 — port failure-adapter.sh (100% cov) --- ## Actual diff (v8.34.0+)
- Phase 1 task #17 — 12 ACS predicates ported (cycle 42/53/104), all PASS --- ## Actual diff (v8.34.0+)
- Phase 1 task #17 — wire doctor/guard/ledger/acs subcommands --- ## Actual diff (v8.34.0+)
- GREEN — Phase 1 task #17 — probe-tool port (98.2% cov) --- ## Actual diff (v8.34.0+)
- GREEN — Phase 1 task #16 — go-test JSON aggregator
- GREEN — Phase 1 task #15 — assertion DSL implementation
- GREEN — Phase 1 task #14 — 5 trust-kernel guards
- GREEN — Phase 1 task #13 — chain guard wraps ledger.Verify
- GREEN — Phase 1 task #12 — JSONL append + SHA256 chain + tip
- GREEN — Phase 1 task #11 — atomic JSON + flock under user-mandated ≥95% gate
- GREEN — Phase 1 task #10 — lifecycle sequencer
- GREEN — Phase 1 task #9 — sentinels + Phase + ports + state machine
- GREEN — Phase 1 task #8 — byte-exact PROJECT_HASH port
- GREEN — Phase 1 task #7 — sidecar writer + slog phase emitter
- GREEN — Phase 1 task #6 — ldflag + BuildInfo composition
- Phase 1 task #5 — go module + Makefile + CI workflow + version stub
- lifecycle wiring + dashboard hot-spots (11/11 tasks, feature complete)
- retrospective + memo consume reflector synthesis (8/11 tasks)
- 9 phase agents + Auditor sycophancy defect (7/11 tasks)
- evolve-reflector persona (Learn-phase synthesizer, 4/11 tasks)
- post-audit driver fixes + comprehensive README rewrite
- shared schema + cross-cycle aggregator (foundation, 3/11 tasks)
- flip EVOLVE_BUILD_PLANNER advisory mode default-on (cycle 2 of 3, ADR-0019, v10.20)
- shadow-wire build-planner phase between TDD and Build (cycle 1 of 3, ADR-0019) --- ## Actual diff (v8.34.0+)
- v0.5b driver-level session-name + resume (claude-tmux)
- v0.5a session-name foundation (orchestration only)
- v0.4 orphan tmux-session sweep on launch
- v0.3 --stream-output for realtime JSONL pass-through
- plan-mode support via --permission-mode pass-through
- auto-skip Triage on trivial cycles with predicate-dep guard
- wire EVOLVE_FANOUT_AUDITOR through dispatch-parallel verb
- B-W1 — install hardening for system-command use
- F5 empty-prompt guard + F6 workspace-reuse WARN + edge fixtures
- W1 — JSON contract + cross-CLI + skill-flow E2E
- human-input behavioral plausibility mode (H1-H5)

### Fixed

- use major.minor form for Current marker (release.sh consistency check) --- ## Actual diff (v8.34.0+)
- port soft-start boundary semantics; TEST_PLAN parity notes --- ## Actual diff (v8.34.0+)
- enable bridge stream_output for all 14 phase-agent profiles
- reject permission_mode on codex/agy drivers + TDD coverage
- install.sh COPY mode for branch-independent installs (#56)

### Documentation

- Phase 1 task #17 deliverable — TEST_PLAN.md (contract-level pass) --- ## Actual diff (v8.34.0+)
- design docs + CLAUDE.md env-var row (5/11 tasks)
- README integration section reflects default-on flip

### Other

- Merge branch 'bridge-local' into main


---

## [10.18.1] - 2026-05-21

_Generated by `scripts/release/changelog-gen.sh` from commits v10.17.0..HEAD._

### Added

- extend default-on bridge delegation to claude/codex/agy
- bridge integration flipped to default-on
- optional bridge delegation in claude-tmux adapter (opt-in)
- portability hardening + doctor + selftest + skill-integration docs
- bridge report subcommand (P11) — closes all open tasks
- codex-tmux + agy-tmux drivers — full 3×2 CLI matrix (P17 + P18)
- codex + agy drivers, --dry-run, env-var contract, mock tests (P12–P16)
- billing-snapshot verifier (P9) — load-bearing claim verified
- auto-respond + add-rule + docs (P6.5 + P11.5 + docs)
- drivers (claude-p, claude-tmux) + Gate-2 milestone (P5–P6)
- scaffold + parser + profile + manifests + probe (P0–P4)
- add antigravity CLI adapter (agy) — 4th peer CLI; NATIVE/HYBRID/DEGRADED tri-mode; zero-cost stub envelope; chmod run-cycle.sh carryover fix
- promote phase-observer to sole stall detector; deprecate phase-watchdog --- ## Actual diff (v8.34.0+)
- PSMAS A/B verification, .gitignore reachability gate, turn-overrun incident analysis --- ## Actual diff (v8.34.0+)

### Fixed

- per-CLI tier-aware bridge fallback in all 4 gates

### Documentation

- bridge integration spec + CHANGELOG entry
- v10.17 documentation pass — architecture, incidents, knowledge-base policy


---

## [10.18.0] - 2026-05-21

_Generated by `scripts/release/changelog-gen.sh` from commits v10.17.0..HEAD._

### Added

- extend default-on bridge delegation to claude/codex/agy
- bridge integration flipped to default-on
- optional bridge delegation in claude-tmux adapter (opt-in)
- portability hardening + doctor + selftest + skill-integration docs
- bridge report subcommand (P11) — closes all open tasks
- codex-tmux + agy-tmux drivers — full 3×2 CLI matrix (P17 + P18)
- codex + agy drivers, --dry-run, env-var contract, mock tests (P12–P16)
- billing-snapshot verifier (P9) — load-bearing claim verified
- auto-respond + add-rule + docs (P6.5 + P11.5 + docs)
- drivers (claude-p, claude-tmux) + Gate-2 milestone (P5–P6)
- scaffold + parser + profile + manifests + probe (P0–P4)
- add antigravity CLI adapter (agy) — 4th peer CLI; NATIVE/HYBRID/DEGRADED tri-mode; zero-cost stub envelope; chmod run-cycle.sh carryover fix
- promote phase-observer to sole stall detector; deprecate phase-watchdog --- ## Actual diff (v8.34.0+)
- PSMAS A/B verification, .gitignore reachability gate, turn-overrun incident analysis --- ## Actual diff (v8.34.0+)

### Documentation

- bridge integration spec + CHANGELOG entry
- v10.17 documentation pass — architecture, incidents, knowledge-base policy


---

## [Unreleased]

### Added

- **`bridge` integration in 4 CLI adapters (`claude.sh`, `claude-tmux.sh`, `codex.sh`, `agy.sh`), default-on when installed.** When `bridge` (an external, user-installable CLI) is on PATH AND reports `schema_version=1`, each adapter delegates to `bridge launch --cli=<NAME>`: `claude.sh→claude-p`, `claude-tmux.sh→claude-tmux`, `codex.sh→codex`, `agy.sh→agy`. Bridge picks up `PROFILE_PATH`, `RESOLVED_MODEL`, `PROMPT_FILE`, etc. from env automatically. `gemini.sh` is unchanged (bridge has no gemini backend). When bridge isn't installed or schema mismatches, the existing native adapters run unchanged — zero regression for users who don't install bridge. Force-disable with `EVOLVE_USE_BRIDGE=0` (recommended for CI bit-for-bit reproducibility, or to restore historical HYBRID-to-claude behavior in `codex.sh`/`agy.sh`). See [`docs/architecture/cli-adapters.md`](docs/architecture/cli-adapters.md) for the integration spec, install steps, per-adapter mapping, and failure modes. Bridge source is not distributed in this repository.

### Changed

- flip `EVOLVE_OBSERVER_ENFORCE` default `0→1` — phase-observer is now the cycle-scope stall detector (`run-cycle.sh`)
- add deprecation WARN when `EVOLVE_OBSERVER_ENFORCE=0` opt-out is used

### Added

- feat(cycle-101): antigravity CLI adapter (agy.sh) — 4th peer CLI; zero-cost stub envelope; NATIVE/HYBRID/DEGRADED tri-mode; cross-name resolution antigravity→agy in subagent-run.sh; deferred cost attribution (cost_blind:true, rollout cycle must add billing tap or turn-cap)
- add `*-observer-events.ndjson` glob to `phase-watchdog.sh` fallback scanning path
- mark `EVOLVE_INACTIVITY_THRESHOLD_S` DEPRECATED; bridge via `EVOLVE_OBSERVER_STALL_S`
- add `EVOLVE_OBSERVER_ENFORCE` and `EVOLVE_OBSERVER_STALL_S` rows to CLAUDE.md env-var table

## [10.17.0] - 2026-05-20

_Generated by `scripts/release/changelog-gen.sh` from commits v10.14.1..HEAD._

### Added

- P3 PSMAS phase-skip foundation (opt-in, default-off)
- L1 orchestrator digest-by-default via profile context_mode
- P2 auditor mastery gate + O-1 fast-fail counter scope docs
- token-economics roadmap cycle-1 — P1 fast-fail, P5 retro YAML extern, L2 stream-json visibility --- ## Actual diff (v8.34.0+)
- trust-kernel hardening — pre-merge tree-SHA verify + commit-SHA self-attestation
- fix gitignore, create cycle-92 deliverables, add git-tracking attestation [worktree-build] --- ## Actual diff (v8.34.0+)
- regression-slice preflight, TDD 1:1 contract, triage MEDIUM floor [worktree-build]
- regression-slice preflight, TDD 1:1 contract, triage MEDIUM floor [worktree-build] --- ## Actual diff (v8.34.0+)
- cycle-90 — doc-deletion-guard hook, Knowledge Stewardship Rule in AGENTS.md, backfill release tags v10.13.0/v10.14.0/v10.14.1, confirm 3A+3C already clean --- ## Actual diff (v8.34.0+)
- cycle-89 Phase C — KB-first persona widening, ADR, CLAUDE.md env vars, online-researcher reference doc --- ## Actual diff (v8.34.0+)
- cycle-88 Cycle B migrate — retire Phase 1 gate pair, Scout owns inline research --- ## Actual diff (v8.34.0+)
- cycle-87 Cycle A foundation — research-quota-gate hook, kb-search.sh, cycle-state plumbing, 7 agent profile widening --- ## Actual diff (v8.34.0+)
- cycle-87 agentic-pipeline enforcement dossier and ADR-0018 stub --- ## Actual diff (v8.34.0+)
- cycles 2-4 — grep-only linter, auditor review section, mutation fail-gate at kill_rate<0.7, EVOLVE_TEST_PHASE_ENABLED default→1 --- ## Actual diff (v8.34.0+)
- cycle-1 author separation — Builder cannot write predicates, TDD-engineer flipped to Opus tier-1
- ACS predicates for closure+cost todos, cycle-84 regression promotion, CLAUDE.md cost isolation note
- implement all 7 carryover fixes — worktree isolation, memo profile, triage fairness, ACS fallback, orchestrator closure, cost attribution, phase validation --- ## Actual diff (v8.34.0+)
- carryover completion — lint baseline, carryover todos cleared, ACS predicate fixes --- ## Actual diff (v8.34.0+)
- implement carryover tasks — incident post-mortem, universal-md foundation, incremental-intent foundation
- add doctor-subscription-auth.sh + subscription-auth-mode ledger event --- ## Actual diff (v8.34.0+)
- add doctor-subscription-auth.sh + subscription-auth-mode ledger event
- add EVOLVE_ANTHROPIC_BASE_URL subscription proxy routing

### Fixed

- builder turn-18 STOP CRITERION + mastery consecutiveSuccesses increment
- resolve build-report Commit via git rev-parse to reject ambiguous prefixes
- archive 5 docs/research files that were deleted in 215488b
- worktree-aware REPO_ROOT in cycle-57-031 predicate
- dismiss false-positive abnormal-event carryover todos from cycle-86 inbox
- correct REPO_ROOT depth to 3 levels in regression-suite/cycle-86 predicates --- ## Actual diff (v8.34.0+)
- cycle-83 ROOT path fix + cycle-84 predicates + pre-existing regression fixes --- ## Actual diff (v8.34.0+)

### Changed

- raise default stall threshold 240s → 600s

### Documentation

- add AGENTS.md docs, CODEBASE-MAP, gitignore fix for .evolve/profiles/*.md [worktree-build] --- ## Actual diff (v8.34.0+)
- persist cycle 87/88/89 + watchdog + violation-incident dossiers
- remove stale hermes-agent proxy example from claude.sh comment
- clarify subscription proxy setup — hermes proxy start fabricated, use proxy-agnostic ANTHROPIC_BASE_URL


---

## [10.16.0] - 2026-05-20

_Generated by `scripts/release/changelog-gen.sh` from commits v10.14.1..HEAD._

### Added

- trust-kernel hardening — pre-merge tree-SHA verify + commit-SHA self-attestation
- fix gitignore, create cycle-92 deliverables, add git-tracking attestation [worktree-build] --- ## Actual diff (v8.34.0+)
- regression-slice preflight, TDD 1:1 contract, triage MEDIUM floor [worktree-build]
- regression-slice preflight, TDD 1:1 contract, triage MEDIUM floor [worktree-build] --- ## Actual diff (v8.34.0+)
- cycle-90 — doc-deletion-guard hook, Knowledge Stewardship Rule in AGENTS.md, backfill release tags v10.13.0/v10.14.0/v10.14.1, confirm 3A+3C already clean --- ## Actual diff (v8.34.0+)
- cycle-89 Phase C — KB-first persona widening, ADR, CLAUDE.md env vars, online-researcher reference doc --- ## Actual diff (v8.34.0+)
- cycle-88 Cycle B migrate — retire Phase 1 gate pair, Scout owns inline research --- ## Actual diff (v8.34.0+)
- cycle-87 Cycle A foundation — research-quota-gate hook, kb-search.sh, cycle-state plumbing, 7 agent profile widening --- ## Actual diff (v8.34.0+)
- cycle-87 agentic-pipeline enforcement dossier and ADR-0018 stub --- ## Actual diff (v8.34.0+)
- cycles 2-4 — grep-only linter, auditor review section, mutation fail-gate at kill_rate<0.7, EVOLVE_TEST_PHASE_ENABLED default→1 --- ## Actual diff (v8.34.0+)
- cycle-1 author separation — Builder cannot write predicates, TDD-engineer flipped to Opus tier-1
- ACS predicates for closure+cost todos, cycle-84 regression promotion, CLAUDE.md cost isolation note
- implement all 7 carryover fixes — worktree isolation, memo profile, triage fairness, ACS fallback, orchestrator closure, cost attribution, phase validation --- ## Actual diff (v8.34.0+)
- carryover completion — lint baseline, carryover todos cleared, ACS predicate fixes --- ## Actual diff (v8.34.0+)
- implement carryover tasks — incident post-mortem, universal-md foundation, incremental-intent foundation
- add doctor-subscription-auth.sh + subscription-auth-mode ledger event --- ## Actual diff (v8.34.0+)
- add doctor-subscription-auth.sh + subscription-auth-mode ledger event
- add EVOLVE_ANTHROPIC_BASE_URL subscription proxy routing

### Fixed

- resolve build-report Commit via git rev-parse to reject ambiguous prefixes
- archive 5 docs/research files that were deleted in 215488b
- worktree-aware REPO_ROOT in cycle-57-031 predicate
- dismiss false-positive abnormal-event carryover todos from cycle-86 inbox
- correct REPO_ROOT depth to 3 levels in regression-suite/cycle-86 predicates --- ## Actual diff (v8.34.0+)
- cycle-83 ROOT path fix + cycle-84 predicates + pre-existing regression fixes --- ## Actual diff (v8.34.0+)

### Documentation

- add AGENTS.md docs, CODEBASE-MAP, gitignore fix for .evolve/profiles/*.md [worktree-build] --- ## Actual diff (v8.34.0+)
- persist cycle 87/88/89 + watchdog + violation-incident dossiers
- remove stale hermes-agent proxy example from claude.sh comment
- clarify subscription proxy setup — hermes proxy start fabricated, use proxy-agnostic ANTHROPIC_BASE_URL


---

## [10.15.0] - 2026-05-19

_Generated by `scripts/release/changelog-gen.sh` from commits v10.12.0..HEAD._

### Added

- cycles 2-4 — grep-only linter, auditor review section, mutation fail-gate at kill_rate<0.7, EVOLVE_TEST_PHASE_ENABLED default→1 --- ## Actual diff (v8.34.0+)
- cycle-1 author separation — Builder cannot write predicates, TDD-engineer flipped to Opus tier-1
- ACS predicates for closure+cost todos, cycle-84 regression promotion, CLAUDE.md cost isolation note
- implement all 7 carryover fixes — worktree isolation, memo profile, triage fairness, ACS fallback, orchestrator closure, cost attribution, phase validation --- ## Actual diff (v8.34.0+)
- carryover completion — lint baseline, carryover todos cleared, ACS predicate fixes --- ## Actual diff (v8.34.0+)
- implement carryover tasks — incident post-mortem, universal-md foundation, incremental-intent foundation
- add doctor-subscription-auth.sh + subscription-auth-mode ledger event --- ## Actual diff (v8.34.0+)
- add doctor-subscription-auth.sh + subscription-auth-mode ledger event
- add EVOLVE_ANTHROPIC_BASE_URL subscription proxy routing
- step 6 — cycle-release.sh canonical release script
- step 5 — resume-cycle.sh quarantine logic
- step 4 — run-cycle.sh resume-quarantine + release hook
- step 3 — filter same-cycle ledger entries
- step 2 — tighten orchestrator profile
- step 1 — add failing isolation test (RED)

### Fixed

- correct REPO_ROOT depth to 3 levels in regression-suite/cycle-86 predicates --- ## Actual diff (v8.34.0+)
- cycle-83 ROOT path fix + cycle-84 predicates + pre-existing regression fixes --- ## Actual diff (v8.34.0+)
- make resume-mode-test source-aware of cold-moved content
- fall through phantom auditor entries (v10.13.0 prep)

### Changed

- dedupe Output template in evolve-tdd-engineer.md (Tier 4)
- redirect log() to stderr (~5 KB/cycle saved)
- remove 6 release-pipeline tools from auditor allowed_tools
- trim triage/retrospective ≥20% LoC (cycle-80) --- ## Actual diff (v8.34.0+)
- trim auditor/builder/role-context-builder (cycle-79 complement)
- trim orchestrator/auditor/builder ≥20% LoC, tighten role-context-builder

### Documentation

- remove stale hermes-agent proxy example from claude.sh comment
- clarify subscription proxy setup — hermes proxy start fabricated, use proxy-agnostic ANTHROPIC_BASE_URL
- step 8 — cycle-isolation.md ADR
- step 7 — document read scope in orchestrator persona


---

## [Unreleased] — Cycle 84

### Added
- `.evolve/baselines/lint-markdown-structure-baseline.txt` — captured repo-wide markdown structure lint baseline (564 warnings across docs/, agents/, skills/)

### Fixed
- Close carryover `abnormal-ship-refused-c82`: ship-refused events from cycle 82 are self-resolved (wrong ship class + HEAD moved; cycle 83 shipped cleanly at 0e3c06f)

### Changed
- Remove completed carryover todos: `universal-md-structure-cycle-a`, `incremental-intent-cycle-1` (both shipped in cycle 83)

## [10.14.1] - 2026-05-19

_Generated by `scripts/release/changelog-gen.sh` from commits v10.12.0..HEAD._

### Added

- step 6 — cycle-release.sh canonical release script
- step 5 — resume-cycle.sh quarantine logic
- step 4 — run-cycle.sh resume-quarantine + release hook
- step 3 — filter same-cycle ledger entries
- step 2 — tighten orchestrator profile
- step 1 — add failing isolation test (RED)

### Fixed

- make resume-mode-test source-aware of cold-moved content
- fall through phantom auditor entries (v10.13.0 prep)

### Changed

- dedupe Output template in evolve-tdd-engineer.md (Tier 4)
- redirect log() to stderr (~5 KB/cycle saved)
- remove 6 release-pipeline tools from auditor allowed_tools
- trim triage/retrospective ≥20% LoC (cycle-80) --- ## Actual diff (v8.34.0+)
- trim auditor/builder/role-context-builder (cycle-79 complement)
- trim orchestrator/auditor/builder ≥20% LoC, tighten role-context-builder

### Documentation

- step 8 — cycle-isolation.md ADR
- step 7 — document read scope in orchestrator persona


---

## [10.14.0] - 2026-05-19

_Generated by `scripts/release/changelog-gen.sh` from commits v10.12.0..HEAD._

### Added

- step 6 — cycle-release.sh canonical release script
- step 5 — resume-cycle.sh quarantine logic
- step 4 — run-cycle.sh resume-quarantine + release hook
- step 3 — filter same-cycle ledger entries
- step 2 — tighten orchestrator profile
- step 1 — add failing isolation test (RED)

### Fixed

- make resume-mode-test source-aware of cold-moved content
- fall through phantom auditor entries (v10.13.0 prep)

### Changed

- trim triage/retrospective ≥20% LoC (cycle-80) --- ## Actual diff (v8.34.0+)
- trim auditor/builder/role-context-builder (cycle-79 complement)
- trim orchestrator/auditor/builder ≥20% LoC, tighten role-context-builder

### Documentation

- step 8 — cycle-isolation.md ADR
- step 7 — document read scope in orchestrator persona


---

## [10.13.0] - 2026-05-18

_Generated by `scripts/release/changelog-gen.sh` from commits v10.12.0..HEAD._

### Added

- step 6 — cycle-release.sh canonical release script
- step 5 — resume-cycle.sh quarantine logic
- step 4 — run-cycle.sh resume-quarantine + release hook
- step 3 — filter same-cycle ledger entries
- step 2 — tighten orchestrator profile
- step 1 — add failing isolation test (RED)

### Fixed

- fall through phantom auditor entries (v10.13.0 prep)

### Documentation

- step 8 — cycle-isolation.md ADR
- step 7 — document read scope in orchestrator persona


---

## [10.12.0] - 2026-05-18

_Generated by `scripts/release/changelog-gen.sh` from commits v10.8.0..HEAD._

### Added

- Stage 10b — densify Scout STOP CRITERION to imperative gate-list (-27 lines, -10.4%) --- ## Actual diff (v8.34.0+)
- Stage 9 — cold-move retrospective agent to reference doc (-40 lines, 14.2%) --- ## Actual diff (v8.34.0+)
- Stage 8 — cold-move auditor Output section to reference doc (-74 lines, 22.2%) --- ## Actual diff (v8.34.0+)
- Stage 7 — cold-move 3 builder sections to reference doc (-54 lines, 14.2%) --- ## Actual diff (v8.34.0+)
- Stage 6 — move orchestrator Phase Loop body to cold-path reference doc (-55 lines, 16.1%) --- ## Actual diff (v8.34.0+)
- Stage 3 — consolidate duplicate Build Steps template in builder
- Stage 2 — extract COLD sections into reference files
- Layer 5 WARN-elevation hardening (ADR-0012)
- Layer 2 hypothesis-falsification carryover (ADR-0012)
- Layer 4 Constitutional audit checklist (ADR-0012)
- Layer 3 generalized POSTHOC sentinel (ADR-0012)
- Layer 1 commit-prefix → diff coherence gate (ADR-0012)
- intent STOP CRITERION tightening (Emergency Exit turn 5, Hard Stop turn 7) [cycle-74, WARN] --- ## Actual diff (v8.34.0+) --- ## Actual diff (v8.34.0+)
- tighten Scout STOP CRITERION (Emergency Exit 12→7, Hard Stop 14→10, Web Research Deadline turn 5) [cycle-73, PASS] --- ## Actual diff (v8.34.0+)
- P2 turn-budget-guidance INERT marking + pending+POSTHOC disclosure discipline [cycle-72, PASS] --- ## Actual diff (v8.34.0+)
- role-gate retrospective) fix + P4 inert marking [cycle-71, WARN] --- ## Actual diff (v8.34.0+)
- P2 builder turn-budget guidance + turn-overrun-c69 RCA [cycle-70, WARN] --- ## Actual diff (v8.34.0+)
- P4 anchored intent injection — scope auditor to acceptance_criteria only
- adaptive model routing — Phase 0+1 foundations

### Documentation

- Stage 5 — add portable-core.md vendor doc + link from CLAUDE.md
- Stage 4 — mark BIC as canonical shared prelude + fix stale v1/v2 comment
- reward-hacking defense system Phase 0 — ADR-0012 + incident report
- reward-hacking defense system Phase 0 — ADR + incident report [cycle-75]
- add [Unreleased] for Phase 0+1 adaptive routing


---

## [10.11.0] - 2026-05-18

_Generated by `scripts/release/changelog-gen.sh` from commits v10.8.0..HEAD._

### Added

- Stage 3 — consolidate duplicate Build Steps template in builder
- Stage 2 — extract COLD sections into reference files
- Layer 5 WARN-elevation hardening (ADR-0012)
- Layer 2 hypothesis-falsification carryover (ADR-0012)
- Layer 4 Constitutional audit checklist (ADR-0012)
- Layer 3 generalized POSTHOC sentinel (ADR-0012)
- Layer 1 commit-prefix → diff coherence gate (ADR-0012)
- intent STOP CRITERION tightening (Emergency Exit turn 5, Hard Stop turn 7) [cycle-74, WARN] --- ## Actual diff (v8.34.0+) --- ## Actual diff (v8.34.0+)
- tighten Scout STOP CRITERION (Emergency Exit 12→7, Hard Stop 14→10, Web Research Deadline turn 5) [cycle-73, PASS] --- ## Actual diff (v8.34.0+)
- P2 turn-budget-guidance INERT marking + pending+POSTHOC disclosure discipline [cycle-72, PASS] --- ## Actual diff (v8.34.0+)
- role-gate retrospective) fix + P4 inert marking [cycle-71, WARN] --- ## Actual diff (v8.34.0+)
- P2 builder turn-budget guidance + turn-overrun-c69 RCA [cycle-70, WARN] --- ## Actual diff (v8.34.0+)
- P4 anchored intent injection — scope auditor to acceptance_criteria only
- adaptive model routing — Phase 0+1 foundations

### Documentation

- Stage 5 — add portable-core.md vendor doc + link from CLAUDE.md
- Stage 4 — mark BIC as canonical shared prelude + fix stale v1/v2 comment
- reward-hacking defense system Phase 0 — ADR-0012 + incident report
- reward-hacking defense system Phase 0 — ADR + incident report [cycle-75]
- add [Unreleased] for Phase 0+1 adaptive routing


---

## [10.10.0] - 2026-05-18

_Generated by `scripts/release/changelog-gen.sh` from commits v10.8.0..HEAD._

### Added

- Layer 5 WARN-elevation hardening (ADR-0012)
- Layer 2 hypothesis-falsification carryover (ADR-0012)
- Layer 4 Constitutional audit checklist (ADR-0012)
- Layer 3 generalized POSTHOC sentinel (ADR-0012)
- Layer 1 commit-prefix → diff coherence gate (ADR-0012)
- intent STOP CRITERION tightening (Emergency Exit turn 5, Hard Stop turn 7) [cycle-74, WARN] --- ## Actual diff (v8.34.0+) --- ## Actual diff (v8.34.0+)
- tighten Scout STOP CRITERION (Emergency Exit 12→7, Hard Stop 14→10, Web Research Deadline turn 5) [cycle-73, PASS] --- ## Actual diff (v8.34.0+)
- P2 turn-budget-guidance INERT marking + pending+POSTHOC disclosure discipline [cycle-72, PASS] --- ## Actual diff (v8.34.0+)
- role-gate retrospective) fix + P4 inert marking [cycle-71, WARN] --- ## Actual diff (v8.34.0+)
- P2 builder turn-budget guidance + turn-overrun-c69 RCA [cycle-70, WARN] --- ## Actual diff (v8.34.0+)
- P4 anchored intent injection — scope auditor to acceptance_criteria only
- adaptive model routing — Phase 0+1 foundations

### Documentation

- reward-hacking defense system Phase 0 — ADR-0012 + incident report
- reward-hacking defense system Phase 0 — ADR + incident report [cycle-75]
- add [Unreleased] for Phase 0+1 adaptive routing


---

## [10.9.0] - 2026-05-17

_Generated by `scripts/release/changelog-gen.sh` from commits v10.8.0..HEAD._

### Added

- adaptive model routing — Phase 0+1 foundations

### Documentation

- add [Unreleased] for Phase 0+1 adaptive routing


---

## [Unreleased]

### Added

- **feat(token-opt): P2 builder turn-budget guidance** (cycle 70)
  - `builder.json:turn_budget_guidance` — structured checkpoint at turn 15 with enumeration protocol; hard-exit enforced at turn 20.
  - `agents/evolve-builder.md` §"Budget Checkpoint Protocol" — explicit 5-step protocol at turn 15: count turns, list remaining steps, estimate cost, defer non-essential, never defer report write.
  - `docs/operations/incidents/turn-overrun-c69.md` — 6-part RCA for cycle-69 scout/builder overrun; root-cause: scout exhaustive-read pattern vs stop-at-sufficient-evidence; builder 1-turn overshoot within calibration tolerance.
  - `scout.json:stop_criterion` — guidance for multi-stream breadth-first audits: read 1–2 files per stream, stop once task proposal is ready, do not recursively read all referenced architecture docs.
  - `docs/architecture/token-economics-2026.md` — P2 status updated to "Shipped cycle 70"; P4 updated to "Shipped cycle 69".
  - **$/cycle hypothesis:** turn-15 checkpoint should reduce builder from 22–26 turns to 15–20 turns. Estimated savings: 3–5 turns × $0.034/turn = **~$0.10–$0.17/cycle** at current baseline. Measurable via cycle 71 builder-usage.json turn count.

- **feat(routing): adaptive model routing — Phase 0+1 foundations** (commit `af304f7`)
  - CLI-agnostic capability tier abstraction (`fast` / `balanced` / `deep`) replacing flat per-phase model assignment.
  - 5-layer architecture documented in [docs/architecture/dynamic-model-routing.md](docs/architecture/dynamic-model-routing.md): profile envelope → deterministic decision function → orchestrator override layer → trust kernel enforcement → CLI tier translation.
  - `scripts/routing/tier-map.json` — CLI-agnostic tier→model translation for Claude/Gemini/Codex/Grok (May 2026 latest model IDs: claude-opus-4-7, gemini-3.1-pro-preview, gpt-5.5, grok-4-heavy, etc.).
  - `scripts/routing/envelope-check.sh` — bash 3.2 validator. Checks `(cli, tier)` ∈ profile envelope + cross-family invariant (Builder ↔ Auditor must differ).
  - `scripts/routing/decide-cycle-routing.sh` — deterministic decision function reading `intent.md` signals (`risk_level`, `awn_class`, `challenged_premises_count`, `interfaces_count`) and `state.json` signals (`failedApproaches`, `fitnessRegression`, `mastery.consecutiveSuccesses`, `carryoverTodos[HIGH]`). Advisory mode until Phase 2 wires it into the orchestrator.
  - `scripts/routing/apply-envelope-additions.sh` — atomic jq-merge applier for profile envelope additions with backup + rollback.
  - `scripts/routing/decide-cycle-routing-test.sh` — 6-scenario test harness.

### Changed

- **13 phase profiles** (`auditor`, `builder`, `evaluator`, `inspirer`, `intent`, `memo`, `orchestrator`, `plan-reviewer`, `retrospective`, `scout`, `tdd-engineer`, `tester`, `triage`) now carry `model_tier_envelope { min, default, max }`, `allowed_clis`, and (for builder/auditor) `cross_family_with` fields. Backward-compatible: profiles without the new fields fall back to `{min: balanced, default: legacy, max: deep}`.
- **Operator-local `.evolve/llm_config.json`** replaced with steady-state 3-tier baseline (was: cycle-66-recovery all-Opus overlay). Drops per-cycle cost from ~$10 toward ~$2.50 while keeping judgment-critical phases (Auditor, Intent, TDD, Plan-review, Retrospective) on deep tier.

### Safety properties

- **Cross-family invariant** structurally encoded for the first time: `auditor.json` and `builder.json` carry `cross_family_with` pointers. Phase 3 will enforce in `role-gate.sh`.
- **Safety-critical phases** (builder, tester, tdd-engineer) restricted to `allowed_clis: ["claude"]` until Phase 4 brings Gemini/Codex/Grok adapters to feature parity (sandbox + permission scoping + budget cap).

### Pending (not yet shipped)

- Phase 2: orchestrator persona wires `decide-cycle-routing.sh` after Intent phase.
- Phase 3: `resolve-llm.sh` consumes `cycle-routing.json`; `role-gate.sh` enforces envelope + cross-family; orchestrator override layer (UP only, one phase per cycle, logged).
- Phase 4: 3P CLI adapter parity (grok.sh scaffold exists in staging).
- Phase 5: retrospective `routing_upgrade` tag schema + self-tuning envelopes via `apply-envelope-from-lessons.sh`.

Staged artifacts: `~/.claude/plans/expressive-bouncing-owl-artifacts/phase-{2,3,4,5}/`.

---

## [10.8.0] - 2026-05-16

_Generated by `scripts/release/changelog-gen.sh` from commits v10.7.0..HEAD._

### Added

- activate v10.6.0 trivial-skip + complete v8.63.0 anchor-mode + refactor symlink direction
- optimize pipeline token usage by refactoring agent personas and consolidating shared constraints --- ## Actual diff (v8.34.0+)
- cycle 63 — C2 phase handoff schemas (5 new) + cycle-62 incident closeout

### Changed

- trim personas for token savings and enforce anchor validation [worktree-build]


---

## [10.7.0] - 2026-05-15

_Generated by `scripts/release/changelog-gen.sh` from commits v10.6.0..HEAD._

### Added

- cycle 63 — B7 resolve-roots worktree detection + Step 8 recovery
- cycle 62 step 7 — B4 memo profile lock-down (defense-in-depth)
- cycle 62 step 6 — B6 CLI Resolution auto-render from ledger
- cycle 62 step 5 — B2 audit citation binding to cycle diff scope
- cycle 62 step 4 — B1 Scout grounding check + WARN-mode discover gate
- cycle 62 step 3 — B5 classifier scans per-role logs + Step-2 follow-up
- cycle 62 step 2 — B0 restore gemini.sh NATIVE adapter block
- cycle 60 — E2E mixed-CLI predicate 040 (gemini/claude/codex router), ADR-7 (EGPS TDD methodology), predicate 042 (legacy backward-compat E2E) — 38/38 ACS GREEN, FINAL architecture redesign cycle --- ## Actual diff (v8.34.0+)
- cycle 58 — ADR-5 standalone phase runners: init-standalone-cycle.sh + check-phase-inputs.sh + predicates 023/024/025 (Slice B 8/10, 33/33 ACS GREEN) --- ## Actual diff (v8.34.0+)
- cycle 57 — Slice B salvage: predicate 022 (orchestrator-uses-registry), phase-gate tester gates, orchestrator.md registry-dispatch, CLAUDE.md Tester backward-compat, predicates 030/031, cycle-55 regression-suite (33/33 ACS GREEN) --- ## Actual diff (v8.34.0+)
- cycle 55 — capability-gate hardening + Slice-B phase-registry (predicates 011/012/020/021 GREEN, 26/26 ACS GREEN, regression-suite updated) --- ## Actual diff (v8.34.0+)
- cycle 54 — Slice A complete: NATIVE gemini+codex adapters, adapter_overrides dispatch, trust kernel CLI-independence (predicates 005/006/009/010 GREEN, 26/26 ACS GREEN, 4 regression suites passing) --- ## Actual diff (v8.34.0+)
- cycle 53 — LLM-router wiring + capability matrix (predicates 004/007/008 GREEN, 25/25 ACS GREEN, 3 regression suites passing) --- ## Actual diff (v8.34.0+) --- ## Actual diff (v8.34.0+)
- cycle 52 — LLM-router foundation: resolve-llm.sh + llm_config.example.json + ADR-1 + predicates 001/002/003 (22/22 ACS GREEN, 3/3 new GREEN, 19/19 regression GREEN) --- ## Actual diff (v8.34.0+)
- cycle 51 — retroactive cycle-50 ACS predicates (9/9 GREEN) + orchestrator MERGE_RC integrity recording + inject-task invalidated_cache_fp field (13/13 ACS GREEN, 4/4 regression GREEN) --- ## Actual diff (v8.34.0+)
- cycle 50 — research-cache Phase B agent integration + predicate WORKTREE fix [worktree-build]
- cycle 49 — research-cache Phase A: task-fingerprint.sh + research-cache.sh + promote-research-cache.sh + scout profile + CLAUDE.md schema (6/6 ACS GREEN, 10/10 regression GREEN) --- ## Actual diff (v8.34.0+)
- cycle 48 — fix expected_ship_sha auto-update + turn-overrun hard-budget gates + wire builder-isolation-breach to abnormal-events.jsonl (3/3 tasks, 8/8 ACS GREEN, 4/4 regression GREEN) --- ## Actual diff (v8.34.0+)
- cycle 47 — fix ship.sh counter-advance ordering (lastCycleNumber stuck at 46) + cycle-46-ship-refused RCA doc (3/3 tasks, 16/16 ACS GREEN, 4/4 regression GREEN) --- ## Actual diff (v8.34.0+)
- cycle 47 — fix ship.sh backtick stripping + reconcile $src→$_src + wire counter-non-advance + cost-overrun abnormal events (4/4 tasks, 12/12 ACS GREEN, 4/4 regression GREEN) --- ## Actual diff (v8.34.0+)
- cycle 47 — P-NEW-22 schema filter enforcement + turn-overrun observability + P-NEW-29 parallel batching guidance + metadata fixes (8/8 ACS GREEN, 4/4 regression GREEN) --- ## Actual diff (v8.34.0+)
- cycle 46 — abnormal-event pipeline test+doc+reconcile-wiring + P-NEW-22 schema_filter_enabled profiles + P-NEW-29 parallel-tool-batching roadmap entry (15/15 ACS GREEN, 4/4 regression GREEN) --- ## Actual diff (v8.34.0+)
- cycle 46 — Phase B abnormal event pipeline (5 detectors wired) + scout tool discipline BANNED table + phantom fields cleanup (9/9 ACS GREEN, 4/4 regression GREEN) --- ## Actual diff (v8.34.0+)
- cycle 45 — T1 builder effort high→medium + T2 turn-count hard gate + T3 P-NEW-21 AgentDiet trajectory compression (5/5 ACS GREEN, 4/4 regression GREEN) --- ## Actual diff (v8.34.0+)
- cycle 44 — P-NEW-26 effort-level dispatch + backfill-lessons instinctSummary restored (14/14 ACS GREEN, 4/4 regression GREEN) --- ## Actual diff (v8.34.0+)
- cycle 44 — P-NEW-23 turn budget hints + P-NEW-24/25 roadmap + D-1 retro gate wired (5/5 ACS GREEN, 8/8 regression GREEN) --- ## Actual diff (v8.34.0+)
- cycle 43 — P-NEW-19 auditor stop-criterion + P-NEW-20/21/22/23 roadmap + retrospective YAML integrity fix (T3-a/b/c) + builder stop-criterion --- ## Actual diff (v8.34.0+)
- cycle 43 — P-NEW-17 investigation complete (CLI uses 1h TTL, Path A closed) + P-NEW-18 EVOLVE_CACHE_PREFIX_V2 default-on + P-NEW-19 auditor stop-criterion roadmap entry + cycle-42 regression-suite promotion --- ## Actual diff (v8.34.0+)
- cycle 42 — roadmap P-NEW-13/P-NEW-16 DONE, P6 citation fix, P-NEW-17 cache-TTL research item + KB dossier --- ## Actual diff (v8.34.0+)
- cycle 42 — P-NEW-13 autotrim line-boundary cut + P-NEW-16 orchestrator STOP CRITERION + roadmap P-NEW-2 verified --- ## Actual diff (v8.34.0+)
- cycle 41 — tester allowlist fix + Builder worktree isolation kernel enforcement (default-on) --- ## Actual diff (v8.34.0+)
- v10.6+ — P-NEW-2 auditor Sonnet right-sizing + scout stop-criterion tighten + roadmap P-NEW-14/P-NEW-15
- v10.6+ — P-NEW-10 scout stop-criterion enforcement + P-NEW-8 AgentDiet filtering + roadmap update cycle 40 --- ## Actual diff (v8.34.0+)

### Fixed

- cycle 63 — B6 wire-up correction (gate_cycle_complete is dead code)

### Documentation

- cycle 62 step 8c — major documentation overhaul (concepts + comparisons + tutorial)
- cycle 62 step 1 — cycle 61 postmortem + B0-B7 root cause analysis
- add Task Initiation, Output Discipline, Long-Running Background Jobs sections to CLAUDE.md --- ## Actual diff (v8.34.0+)

### Other

- Finalize Gemini native mode adapter and subagent env leak fix --- ## Actual diff (v8.34.0+)
- (salvage) cycle 41 attempt 1 — P-NEW-9 orchestrator summarization + AgentDiet classifier fix + roadmap cycle-41 status


---

## [10.6.0] - 2026-05-14

_Generated by `scripts/release/changelog-gen.sh` from commits v10.5.0..HEAD._

### Added

- v10.6.0 — three-layer auto-resume after Claude Code quota hits

### Documentation

- carry P-NEW-6 API-key-only constraint research from pre-cleanup tip


---

## [10.5.0] - 2026-05-14

_Generated by `scripts/release/changelog-gen.sh` from commits v10.3.0..HEAD._

### Added

- v10.5.0 — Phase-B observability live-wire (EVOLVE_TRACKER_ENABLED, opt-in) --- ## Actual diff (v8.34.0+)
- v10.4.0 — Phase-A observability scaffolding (tracker-writer + rollup + show-trace + prune-ephemeral, additive) --- ## Actual diff (v8.34.0+)


---

## [10.4.0] - 2026-05-14

_Generated by `scripts/release/changelog-gen.sh` from commits v10.3.0..HEAD._

### Added

- v10.4.0 — Phase-A observability scaffolding (tracker-writer + rollup + show-trace + prune-ephemeral, additive) --- ## Actual diff (v8.34.0+)


---

## [10.3.0] - 2026-05-14

_Generated by `scripts/release/changelog-gen.sh` from commits v10.0.0..HEAD._

### Added

- v10.3.0 — Dedicated evolve-tester persona separates predicate authorship from Builder
- v10.2.0 — Promote mutation-gate from WARN-only to FAIL (EGPS Rollout phase 2)
- v10.1.0 — EGPS persona wiring (Builder/Auditor/Orchestrator)


---

## [10.2.0] - 2026-05-14

_Generated by `scripts/release/changelog-gen.sh` from commits v10.0.0..HEAD._

### Added

- v10.2.0 — Promote mutation-gate from WARN-only to FAIL (EGPS Rollout phase 2)
- v10.1.0 — EGPS persona wiring (Builder/Auditor/Orchestrator)


---

## [10.1.0] - 2026-05-14

_Generated by `scripts/release/changelog-gen.sh` from commits v10.0.0..HEAD._

### Added

- v10.1.0 — EGPS persona wiring (Builder/Auditor/Orchestrator)


---

## [10.0.0] - 2026-05-14

_Generated by `scripts/release/changelog-gen.sh` from commits v9.5.0..HEAD._

### Added

- v10.0.0 — Execution-Grounded Process Supervision (EGPS) bootstrap
- cycle 39 — C3 handoff-schema adoption: Auditor+Retrospective emit handoff JSON (output_contract, gate_audit_to_retrospective WARN-only, A1-A6 PASS) + EGPS research file; validate-handoff-artifact-test.sh 25/25, triage-phase-test.sh 16/16 (builder isolation breach: changes in main tree — cycle audit PASS) --- ## Actual diff (v8.34.0+)
- cycle 39 — c39-D1 triage Step-0a content-verify (prevent fraudulent skip_shipped via commit  pattern) + c38-D-recovery: restore inbox-audit.sh + inbox-reconcile.sh from orphaned git object 332ac9d; triage-inbox-ingestion-test.sh 31/31, inbox-audit-test.sh 9/9 --- ## Actual diff (v8.34.0+)


---

## [9.5.0] - 2026-05-13

_Generated by `scripts/release/changelog-gen.sh` from commits v9.4.0..HEAD._

### Added

- merge phase-watchdog into phase-observer via --enforce flag (v9.5.0 ladder step 1)


---

## [9.4.0] - 2026-05-13

_Generated by `scripts/release/changelog-gen.sh` from commits v9.3.0..HEAD._

### Added

- phase-observer service — per-phase OODA loop with structured INFO/WARN/INCIDENT notifications
- cycle 38 — C2-handoff-schemas: 3 JSON schemas + validate-handoff-artifact.sh lint tool + phase-gate soft-WARNs; c38-D-triage-parse attempted (carry to c39: fix section pattern to title-case) --- ## Actual diff (v8.34.0+)
- cycle 36 — c45-P-NEW-6 tool-result hygiene (Branch B): persona discipline subsection in evolve-{builder,scout,auditor}.md, context_clear_trigger_tokens in profiles, advisory log in subagent-run.sh, tool-result-saturation.sh NEW; c36-worktree-isolation-guard: WORKTREE ISOLATION CONSTRAINT block in role-context-builder.sh (D1 carryover to c37: fix _profile_expanded var name) --- ## Actual diff (v8.34.0+)
- cycle 35 — c42-tool-result-sanitization: wrap_external_content() in role-context-builder.sh (Ghosh Pattern #3 indirect-injection mitigation), tool-sanitization-test.sh NEW 5/5 PASS; c41 H1/H2 eval hardening (overallRigor: GOOD); recover orphaned c38 --injected-by partial (Tests 12-13 + inject-task.sh, +14 LoC, self-consistent) --- ## Actual diff (v8.34.0+)
- cycle 34 — c41-eval-score-caps: Ghosh Pattern #2 adoption — YAML score_cap frontmatter + eval-quality-check.sh check_score_caps() +93 LoC, D.score_cap_enforcement in evolve-auditor.md, 5/5 tests pass, 2 new evals (WARN: eval demonstration surface H1/H2 queued for c35 hardening) --- ## Actual diff (v8.34.0+)
- cycle 33 — c40-ghosh-research-dossier: self-correcting-pipelines-ghosh-2026.md (101 LoC, 8-pattern analysis, ✓/GAP annotations, convergence map, gap analysis c41-c44, departures table, adoption sequence) [worktree-build]
- cycle 32 — C1 ship-gate tree-SHA binding: audit emits audit_bound_tree_sha, ship.sh post-commit verification + ship-binding.json sidecar, ship-gate.sh pre-commit guard, detect-tree-sha-breach.sh NEW (100 LoC), ship-gate-tree-sha-test.sh 7/7 PASS, ship-integration-test.sh 23/23 PASS --- ## Actual diff (v8.34.0+)
- cycle 31 — c38-investigation: docs/incidents/cycle-31-c38-orphan.md (295 lines, 6-part forensic report, breach cascade analysis, recovery plan, Triage Step-0a content-verify spec)
- cycle 31 — c38-inbox-audit-and-collision: inbox-audit.sh NEW + inbox-reconcile.sh NEW + inject-task.sh git-log + processed/rejected JSON collision + --force + --injected-by (4 files ~155 LoC, 17/17 inject-task tests + 19/19 lifecycle tests pass) --- ## Actual diff (v8.34.0+)
- cycle 30 — c37: inbox lifecycle foolproofing (Layer 1 idempotency-via-git + Layer 2 3-state inbox-mover.sh + Layer 2.5 crash-recovery, 6 files ~490 LoC: inbox-mover.sh NEW 285 LoC, inbox-lifecycle-test.sh NEW 205 LoC, 19/19 tests pass) --- ## Actual diff (v8.34.0+)
- cycle 29 — c33: activity-based phase watchdog MVP (4 files, ~282 LoC: phase-watchdog.sh NEW, run-cycle.sh +30 LoC, cycle-state.sh +9 LoC stall-inactivity whitelist, phase-watchdog-test.sh NEW 3/3 tests pass) --- ## Actual diff (v8.34.0+)
- cycle 28 — c30: agents/evolve-diagnose-reference.md Layer-3 (mattpocock 6-phase diagnose loop + [DEBUG-XXXX] tag pattern) --- ## Actual diff (v8.34.0+)
- cycle 28 — c29: docs/adr/ backfill (ADR 0001–0006) + docs/index.md ADR section --- ## Actual diff (v8.34.0+)
- cycle 27 — c32: inbox-injection API via .evolve/inbox/ + Triage Step 0 ingestion (11 files, 698 LoC: inject-task.sh CLI, triage ingestion, reconcile _inbox_source preservation, ADR 0007, architecture doc, operator APIs in CLAUDE.md, tests 17/17+27/27) --- ## Actual diff (v8.34.0+)
- cycle 26 — c28: TSC + caveman merge on agents/evolve-builder.md (2730→2045 words, -25.1%)
- cycle 25 — c27b: fix false causal claims in orchestrator/memo-reference/memo personas (remove merge-lesson→memo.md dependency assertion; accurate Downstream consumers; cycle-memo terminology) --- ## Actual diff (v8.34.0+)
- cycle 24 — c27: Layer-P memo phase contract in agents/evolve-orchestrator.md + Layer-3 reference (closes memo-skip pattern, WARN: false exit-2 causal claim to fix in c27b) --- ## Actual diff (v8.34.0+)
- cycle 24 — c26: CONTEXT.md canonical domain glossary (21 terms, 598 words) --- ## Actual diff (v8.34.0+)
- cycle 24 — TSC on evolve-scout.md (1271→1017 words, -20.0%) --- ## Actual diff (v8.34.0+)
- cycle 24 — P-NEW-7: phases.md Layer-3 split (28,911→13,987 bytes, -51.6%), research persistence, roadmap P-NEW-7/8/9 --- ## Actual diff (v8.34.0+)
- cycle 24 — P-NEW-1 + P5 token reduction: promote EVOLVE_CONTEXT_DIGEST/ANCHOR_EXTRACT to default=1, extract retrospective YAML template (~$1.25/cycle expected savings) --- ## Actual diff (v8.34.0+)
- cycle 24 — Scout persona Layer-3 split (P-NEW-3) + roadmap currency refresh

### Fixed

- preflight accepts WARN-or-PASS audit verdict (fluent-posture-aligned)
- cycle 37 — c37-D1 fix dead-code advisory log in subagent-run.sh: remove _profile_expanded block, insert corrected advisory using effective_profile after profile expansion, add --check-ctx-advisory testability mode + Test 12; c37-D2 add test-headline format rule to evolve-builder.md (N pass / M fail when M>0) --- ## Actual diff (v8.34.0+)
- phase-watchdog phase-aware baseline — reset timer on phase advance OR log update
- phase-watchdog stream-json coupling — eliminate cycle-36-style false-positive SIGTERM


---

## [9.3.0] - 2026-05-12

_Generated by `scripts/release/changelog-gen.sh` from commits v9.2.0..HEAD._

### Added

- cycle 23 — remove --disable-slash-commands from builder.json to unblock Skill tool self-review loop --- ## Actual diff (v8.34.0+)
- cycle 22 — deploy security-review-scored + builder allowlist extension --- ## Actual diff (v8.34.0+)
- cycle 21 — Builder Skill allowlist fix + awk anti-pattern doc + security-review-scored skill + cost attribution --- ## Actual diff (v8.34.0+)
- cycle 20 — verify Builder skill-loop mechanism + fix code-review-simplify git diff ref + review skill catalog --- ## Actual diff (v8.34.0+)

### Fixed

- --plugin-dir .evolve/plugin unblocks Skill tool in Builder/Auditor subagents


---

## [9.2.0] - 2026-05-12

_Generated by `scripts/release/changelog-gen.sh` from commits v9.1.1..HEAD._

### Added

- cycle 19 — semantic eval-grader patterns + mutation gate integration (EVOLVE_MUTATION_CHECK_DISABLE, default OFF, WARN-only) --- ## Actual diff (v8.34.0+)
- cycle 18 — EXIT_TRANSPORT_HANG classifier + Builder isolation breach detector (EVOLVE_HANG_CLASSIFIER, EVOLVE_BUILDER_ISOLATION_CHECK, default OFF) --- ## Actual diff (v8.34.0+)
- cycle 17 — evolve-code-reviewer profile + Auditor fan-out lens (EVOLVE_FANOUT_AUDITOR_CODE_REVIEWER, default OFF) --- ## Actual diff (v8.34.0+)
- cycle 16 — code-simplifier advisory profile + builder phase hook (EVOLVE_SIMPLIFY_ENABLED, default OFF) --- ## Actual diff (v8.34.0+)
- cycle 15 — token-reduction roadmap (P1-P8) + opt-in audit advisory review hook --- ## Actual diff (v8.34.0+)

### Fixed

- cycle-19 WARN — mutation gate now scopes to cycle-new evals via .cycle-start-marker

### Changed

- cycle 20 — replace cycles 16+17 advisory subagents with Builder self-review skill loop


---

## [9.1.1] - 2026-05-11

_Generated by `scripts/release/changelog-gen.sh` from commits v9.1.0..HEAD._

### Added

- knowledge-base content model + README rewrite + research stewardship rule
- cycle 14 — delete 3 dead docs, strip _* profile comments, fix SKILL.md pointer (326→323 files, −382 LOC, −12,007 bytes profiles −21.6%; CEMM arXiv:2603.09619 + AgentDiet arXiv:2509.23586; swarm-test 41/41, role-gate 18/22 baseline) --- ## Actual diff (v8.34.0+)
- cycle 13 — delete 42 dead docs/research/ files (368→326 tracked files, −11.4%; −7,737 LOC, −498 KB transfer); swarm-test 41/41, role-gate baseline 18/22 unchanged; Liu et al. 2023 context-noise reduction --- ## Actual diff (v8.34.0+)

### Fixed

- resume positive-path bugs caught by end-to-end test

### Changed

- doc consolidation Commit D — remove legacy knowledge-base glob
- doc consolidation Commit C — re-link cross-references
- doc consolidation Commit B — move files into single docs/ root
- doc consolidation Commit A — expand glob to docs/private


---

## [9.1.0] - 2026-05-11

Two paired capabilities for graceful resource-exhaustion handling:
**checkpoint-resume** for cost-budget exhaustion (and Claude Code
subscription quota walls), and **context-window control** for context-
budget exhaustion. Pre-v9.1.0, a cycle that hit either wall lost all
in-flight work; v9.1.0 preserves the worktree + cycle-state so operators
can recover with `/evo:loop --resume`.

### Background

Three consecutive `/evo:loop` dispatcher runs in the 2026-05-11 session
aborted at rc=1 with the same signature: orchestrator subagent exits non-
zero, empty stderr tail, after substantial cycle work. Diagnosed as
Claude Code subscription quota exhaustion (GitHub #29579). Pre-v9.1.0
recovery was impossible — worktree gone, state cleared, batch loop dead.

### Added

- **Checkpoint write infrastructure** (Cycle 1) — Three new ops in
  `scripts/lifecycle/cycle-state.sh`: `checkpoint <reason>`,
  `is-checkpointed`, `resume-phase`. Additive schema — pre-v9.1 readers
  ignore the new block. `run-cycle.sh`'s EXIT trap honors `is-checkpointed`
  and preserves worktree + cycle-state instead of cleaning up.
- **Pre-emptive checkpoint thresholds** (Cycle 2) — Dispatcher emits
  `BATCH-BUDGET WARN` at 80% (`EVOLVE_CHECKPOINT_WARN_AT_PCT`) and
  exports `EVOLVE_CHECKPOINT_REQUEST=1` at 95% (`EVOLVE_CHECKPOINT_AT_PCT`).
  Next cycle's orchestrator reads this and pauses at the next phase
  boundary (graceful, not mid-cycle abort).
- **Reactive quota-likely classification** (Cycle 3) — `subagent-run.sh`
  detects the subscription quota signature (`cli_exit==1 + empty stderr
  + cost ≥ 80% of cap`) and writes a checkpoint automatically.
  `EVOLVE_QUOTA_DANGER_PCT=0` fires for any empty-stderr rc=1;
  `=100` effectively disables.
- **`--resume` flag and `resume-cycle.sh`** (Cycle 4) — Locates the
  most recent checkpointed cycle, validates state (git HEAD unchanged,
  worktree exists), re-spawns the orchestrator with `EVOLVE_RESUME_MODE=1`
  plus `EVOLVE_RESUME_PHASE` and `EVOLVE_RESUME_COMPLETED_PHASES`.
  Operator override: `EVOLVE_RESUME_ALLOW_HEAD_MOVED=1`.
- **Orchestrator persona resume-mode protocol** (Cycle 5) — `## Resume Mode`
  section in `agents/evolve-orchestrator.md` documents the 5-step protocol:
  read preserved state → skip completed phases → clear-checkpoint flag →
  pick up at resume phase → re-pause if needed. Adds the matching
  `cycle-state.sh clear-checkpoint` operation.
- **Context-window control** (Cycle 6) — Paired capability for
  context-budget exhaustion. `subagent-run.sh` writes per-phase
  `context-monitor.json` with input_tokens + cumulative tracker.
  `EVOLVE_CONTEXT_AUTOTRIM=1` enables aggressive prompt trim (head 60%
  + tail 35% + marker) when over `EVOLVE_PROMPT_MAX_TOKENS` (default 30k).
- **`scripts/observability/show-context-monitor.sh`** — Operator-facing
  observability: tabular, `--watch` live-tail, `--json` for scripts.
  Emits WARN/CRITICAL annotations at the same percentage thresholds as
  the cost-side checkpoint logic.

### Documentation

- **`docs/architecture/checkpoint-resume.md`** — Canonical reference:
  three pause triggers, recovery scenarios, env-var matrix, trust-kernel
  invariants preserved.
- **`docs/architecture/context-window-control.md`** — Canonical reference:
  autotrim algorithm, monitor JSON schema, interaction with
  checkpoint-resume, env-var reference.
- **`CLAUDE.md`** — v9.1.0 section between Auto-Retrospective and
  Three-Tier Strictness Model.
- **SKILL.md** — `argument-hint` updated to include `--resume`; Quick
  Start adds "Resume after pause" stanza.

### Test coverage

- `scripts/tests/checkpoint-roundtrip-test.sh` — 19 assertions
- `scripts/tests/preemptive-checkpoint-test.sh` — 18 assertions
- `scripts/tests/reactive-quota-classify-test.sh` — 15 assertions
- `scripts/tests/resume-cycle-test.sh` — 26 assertions
- `scripts/tests/orchestrator-resume-mode-test.sh` — 23 assertions
- `scripts/tests/context-window-control-test.sh` — 22 assertions

Total v9.1.0 test count: **123/123 PASS** (5× stability verified).
Kernel regression unchanged: swarm-architecture 41/41, role-gate 23/23.

### Trust kernel invariants (unchanged)

phase-gate, role-gate, ship-gate, ledger SHA-chain all enforce identically
across paused → resumed cycles. The checkpoint block is additive schema;
the resume protocol goes through the same phase-gate-precondition
allowlist as a fresh cycle.

### Operator opt-out

`EVOLVE_CHECKPOINT_DISABLE=1` disables both pre-emptive thresholds.
`EVOLVE_CONTEXT_AUTOTRIM` defaults to `0` (opt-in). The default behavior
(no checkpoint requested, no resume) is byte-identical to v9.0.5.


---

## [9.0.5] - 2026-05-11

Doc-closure release for the cycle→cost input migration. **No behavior
change.** The dispatcher engine has supported both `--budget-usd N` and
`--cycles N` since v8.60.0 (and cycle-11 added the `--budget-usd` alias
+ structured summary), but three surfaces still framed the skill as
cycle-first: SKILL.md's Quick Start, the dispatcher's stale "v8.62
will flip positional integer" deprecation message, and CLAUDE.md's
migration-status documentation. v9.0.5 closes those gaps.

### Background

The cycle-11 invocation that drove this entire v9.0.x series literally
parsed `/evo:loop 3 continue to move to cost driven goal...` in
cycle-mode because the bare positional integer still meant "3 cycles"
— exactly the migration friction the user was asking us to fix. The
engine had advanced; the docs had not.

### Changed

- **`skills/evolve-loop/SKILL.md` — Usage line and Quick Start.**
  Usage now lists both flag forms (`/evo:loop [--budget-usd N | --cycles N]
  [strategy] [goal]`). Quick Start leads with **budget-first** framing:
  `--budget-usd N` (or `--budget N`) is the recommended mode (predictable
  costs); `--cycles N` is the alternative; bare positional integer is
  documented as legacy with a deprecation warning.
- **`scripts/dispatch/evolve-loop-dispatch.sh` — deprecation message
  refreshed.** The previous text pointed at "v8.62 will flip positional
  integer to DOLLARS" — but v8.62 was skipped when development jumped to
  v9.0.0. The flip is now framed as a v10.0.0 candidate (breaking
  change; warrants a major-version-bump signal). New wording:
  > DEPRECATION: positional integer means cycles (since v8.60); v10.0.0
  > candidate will consider flipping to dollars — migrate to --cycles N
  > or --budget-usd N now to be flip-safe
- **`CLAUDE.md` — Cycle→cost migration status table** added near the
  existing "User-stated budget (v8.60.0+, Layer 1)" section. Surfaces
  the four "✅ shipped" rows + the one "⚠️ still positional=cycles"
  row, so future readers see the migration state at a glance.

### What v9.0.5 does NOT do

- Does not flip positional integer semantics. That belongs in v10.0.0
  (breaking change — a user with `/evo:loop 3` muscle-memory would
  suddenly spend $3 instead of running 3 cycles).
- Does not deprecate `--cycles`. Both flags remain co-equal.
- Does not touch dispatcher *logic* — only the deprecation message text.

### Verified

- `scripts/tests/evolve-loop-dispatch-test.sh`: 57 tests / 71 sub-
  assertions, all PASS. Test 49 (positional-integer DEPRECATION WARN
  detection) still matches the new wording via its generic regex.
- Smoke test confirmed: `VALIDATE_ONLY=1 bash evolve-loop-dispatch.sh 3
  balanced "test"` now emits the new "v10.0.0 candidate" deprecation
  message.

## [9.0.4] - 2026-05-11

P2 of the v9.0.2 token-economics roadmap. Drives the **builder phase**
from cycle-11's measured **58 turns / $1.95** to a target of **15–20
turns / ~$1.00**. Same playbook as v9.0.2 (intent) and v9.0.3 (scout):
persona-side `## Turn budget` section + advisory cap tightening,
without stripping the legitimately code-writing toolkit.

### Background

Cycle 11 measured: builder = 58 turns, $1.95, 19,866 output tokens —
the heaviest cycle phase. The profile's `max_turns: 80` advisory was
soft and unshaped; cycle 11 didn't approach it. Root cause: builder's
7-step Workflow (Step 0 → Step 7) plus several sub-steps multiplies
turns rapidly when each Edit is its own turn.

### Fixed

- **Builder profile (`.evolve/profiles/builder.json`).**
  - `max_turns: 80 → 25` (advisory cap; design target is 15–20).
  - `max_budget_usd: 2.50 → 1.00` (cycle-11 evidence: $1.95 spent;
    target reduction to ~$1.00 = ~50%).
  - Tool surface unchanged — builder genuinely needs Read/Grep/Glob/
    Edit/Write/MultiEdit/NotebookEdit/Bash. Turn count is bounded by
    persona discipline, not by tool-stripping.
- **Builder persona (`agents/evolve-builder.md`).** New `## Turn budget`
  section above the Workflow section:
  - Target 15–20 turns; max 25 (profile-enforced).
  - **Batch Edit calls; use MultiEdit when changing the same file
    multiple times.** Each Edit is 1 turn; one MultiEdit with 5 ops
    is 1 turn vs 5 sequential Edits = 5 turns.
  - **Read once, edit decisively.** Don't re-read between sequential
    Edits to the same file.
  - **Self-Verify is ONCE, not interleaved.** Run test suite after
    implementation, not after each Edit.
  - **Retry budget hard-capped at 3** (Step 6) — beyond that, report
    failure and let the next cycle adapt.
  - Per-step turn budget table: Step 0 = 1, Step 1 = 1, Step 2 = 2–3,
    Step 3 = 1, Step 4 = 5–10, Step 4.5 = 0–3, Step 5 = 1–2, Step 6 =
    0–5, Step 7 = 0 → sum ≤ 20.

### Verified

- `scripts/tests/persona-progressive-disclosure-test.sh` extended from
  15 to 19 tests; all PASS. New assertions: builder persona has Turn
  budget section + 15-20 / 25 numerics, profile max_turns ≤ 25,
  max_budget_usd ≤ $1.00, persona instructs MultiEdit-aggressive
  pattern. Also widened the persona size cap from 18000 → 20000 bytes
  to accommodate the new Turn budget section.
- No regressions: intent-test 24/24 (v9.0.2), scout-carryover 8/8
  (v9.0.3), swarm-architecture 41/41, role-gate 23/23.

### Roadmap status

P1 (scout, v9.0.3) ✅. P2 (builder, v9.0.4) ✅. Next:
- P3: Triage bedrock right-sizing (~$0.10/cycle saved)
- P4: Auditor anchor mode for intent.md (~$0.05/cycle saved)
- P5: Retrospective YAML template externalization (~$0.05/cycle saved)
- P6: PSMAS-style phase-skip via triage classification (up to $2/cycle
  on skipped cycles)

Items P3–P5 are small one-touch changes; P6 is a research-stage
architectural item. See `docs/architecture/token-economics-2026.md`.

## [9.0.3] - 2026-05-11

P1 of the v9.0.2 token-economics roadmap (see
`docs/architecture/token-economics-2026.md`). Drives the **scout phase**
from cycle-11's measured 49 turns / $1.32 down to a target of 8–12
turns / ~$0.50. Same playbook as v9.0.2's intent fix, adapted to scout's
legitimately-larger discovery surface.

### Background

Cycle 11 measured: scout = 49 turns, $1.32, 16,378 output tokens. The
profile's `max_turns: 30` advisory was exceeded by 19. Root cause: open-
ended exploration — the persona instructed "Read ALL project
documentation" in cycle-1 mode and offered no turn budget in incremental
mode, so scout did 4–6 rounds of file reads to ground each hypothesis.

### Fixed

- **Scout profile (`.evolve/profiles/scout.json`).** `max_turns: 30 → 15`
  (advisory cap; design target is 8–12). WebSearch/WebFetch retained
  for the fan-out 'research' sub-scout (`parallel_subtasks[].research`);
  main-flow scout is instructed by the persona to defer web work to
  Phase 1 RESEARCH (the persona's existing rule at line 80 — now
  reinforced by the new Turn budget section).
- **Scout persona (`agents/evolve-scout.md`).** Added explicit
  `## Turn budget` section directly above Responsibilities:
  - Target 8–12 turns, max 15 (profile-enforced)
  - Lead with pre-loaded context (`projectDigest`, `carryoverTodos`,
    `instinctSummary`, `recentLedger`, `failedApproaches`,
    `evaluatedTasks` — all already in role-context) rather than grep
    expeditions
  - Cap directed file reads at ≤5 per cycle; reads beyond that need a
    specific premise being tested (or invoke `EVOLVE_TASK_MODE=deep`)
  - Cap Grep/Glob at ≤3 per cycle
  - Write scout-report.md ONCE (each Edit counts as a turn)
- **Responsibilities §1 (Mode-Based Discovery) tightened** with per-
  mode turn budgets:
  - `full` (cycle 1): 10–12 turns — targeted scans, not full dir walks
  - `incremental` (cycle 2+): 6–8 turns — pre-loaded context is enough
    for many cycles; if `carryoverTodos` resolves the cycle, skip
    codebase exploration entirely
  - `convergence-confirmation`: 3–5 turns — flag + stop

### Verified

- `scripts/tests/scout-carryover-decision-test.sh` extended from 4 to 7
  tests (8 sub-assertions); all PASS. Asserts persona has Turn budget
  section + 8-12 / 15 numerics, profile max_turns ≤ 15, persona defers
  web research to Phase 1.
- No regressions: swarm-architecture 41/41, role-gate 23/23.

### Pending follow-up (P2 from the roadmap)

Builder ran 58 turns / $1.95 in cycle 11. v9.0.4 will apply the same
playbook to builder.

## [9.0.2] - 2026-05-11

Patch release that drives the intent phase from **7 turns / $1.05** to a
**≤2-turn / ≤$0.30** budget by structurally removing the persona's
exploration toolkit and tightening the persona's operating guidance. Same
intent.md quality contract (8-field structure + AwN classifier + ≥1
challenged_premise + valid risk_level) — schema invariants are kernel-
enforced and unchanged.

### Background

Live evidence from cycle 11 (`.evolve/runs/cycle-11/intent-usage.json`):
the intent phase ran 7 turns, output 10,353 tokens, and cost **$1.05** in
175 sec. 75% of that cost was *output* tokens — generated by 4–6 rounds of
Grep/Glob/Read into the codebase to ground the `challenged_premises`
field with code evidence. Scout then re-read the same files; duplicate
exploration paid twice per cycle.

The persona's own docstring says "You are NOT a planner. You are NOT a
designer. You are a structurer + premise-challenger" — but the persona
was structurally equipped with the full exploration toolkit. Behavior
followed structure.

### Fixed

- **Intent profile (`.evolve/profiles/intent.json`) — exploration tools
  stripped.** Removed: `Grep`, `Glob`, `Bash(ls/head/tail/wc/find/test/git
  status/log/diff/ls-files/rev-parse)`. Retained: `Read`, `Bash(cat:*)`,
  `Write/Edit` on intent.md. The persona is now structurally bounded to a
  single-pass parse-then-write flow — multi-turn investigation is no
  longer possible.
- **Intent profile — budget cap tightened.** `max_budget_usd`: 0.50 →
  **0.30** (matching v9.0.2 cost target). Tier ceilings adjusted: default
  0.50→0.30, heavy_rules 1.00→0.60, minimal 0.20→0.15. `max_turns`: 15 →
  **4** (kernel-enforced backstop; design target is 2).
- **Intent persona (`agents/evolve-intent.md`) — explicit turn budget +
  tool-restraint guidance.**
  - New `## Turn budget` section: "Maximum 2 turns. Turn 1: parse + structure
    internally. Turn 2: Write intent.md."
  - `## What you MUST NOT do` updated to list the v9.0.2-stripped tools as
    structurally forbidden + explains *why* (Scout's job is to verify).
  - `## The mandatory ≥1 challenged_premise rule` reframed: "challenge
    based on prima-facie reading of the goal and the pre-loaded context —
    your challenge does NOT need to cite source code." (Coherence is the
    bar, not evidence.)
  - `## Length budget` reduced: 50–200 lines → **30–80 lines** (output
    tokens dominate cost at $75/MTok on Opus).

### Verified

- `scripts/tests/intent-test.sh` — extended from 18 to 22 tests; all PASS.
  New assertions cover the stripped tool list, budget cap ≤ \$0.30, max_turns
  ≤ 4, and the persona's `## Turn budget` section presence.
- No regressions: swarm-architecture 41/41, role-gate 23/23, subagent-run
  17/17.
- Schema invariants (`gate_intent_to_research` enforcement of awn_class +
  ≥1 challenged_premise + 8-field structure) are unchanged at the kernel
  layer — no `phase-gate.sh` modifications.

### Pending follow-up

Cycle 11's scout phase ran **49 turns / $1.32** — even worse than intent's
pre-v9.0.2 burn. Same root-cause pattern (exploration toolkit + evidence-
gathering mandate) likely applies. A v9.0.3 candidate patch targeting
scout will follow the same playbook: tool surface audit, persona turn
budget, length cap reduction.

## [9.0.1] - 2026-05-11

Patch release addressing two HIGH-severity findings + one MEDIUM finding from
the v9.0.0 post-release code review, plus four low-risk SHIP-NOW
simplifications. No behavior changes for the default path
(`EVOLVE_CACHE_PREFIX_V2=0`, `EVOLVE_CONTEXT_DIGEST=0`,
`EVOLVE_ANCHOR_EXTRACT=0` — all default-off).

### Fixed

- **HIGH-1 — `cycle-digest.json` is now byte-identical for same inputs.**
  Removed the `built_at` ISO-8601 timestamp from the digest schema.
  Previously, two fan-out workers racing to lazy-build the digest in a
  sub-second window would each get a different `built_at`, breaking
  prompt-cache key stability. Schema bumped to v1.1.
  (`scripts/lifecycle/build-cycle-digest.sh`,
  `scripts/tests/cycle-digest-test.sh`)
- **HIGH-2 — Section-aware strip in `claude.sh` for `ADVERSARIAL_AUDIT=0`.**
  The previous awk pattern (`/^## Adversarial Audit Mode/{stop=1} !stop {print}`)
  stopped emitting at the section header and never resumed — silently
  dropping any future content that might follow. Latent today (the section
  is currently last in the auditor bedrock) but a real defect waiting on
  template growth. Replaced with a section-aware strip that resumes at the
  next `## ` heading.
  (`scripts/cli_adapters/claude.sh`)
- **MEDIUM — Quote-safe `entries` iteration in
  `emit_profile_context_anchors`.** Replaced unquoted `for entry in $entries`
  word-splitting with a `while IFS= read -r entry` pattern using `<<<` here-
  string. Preserves bash 3.2 portability while accepting filenames or anchor
  values that contain whitespace.
  (`scripts/lifecycle/role-context-builder.sh`)

### Changed (post-release simplifications, no behavior change)

- **SIMP-1 — Shared `<!-- ANCHOR:` regex constant.** Single
  `_ANCHOR_PREFIX_RE` shell variable now backs both `extract_anchor` (awk)
  and `file_has_anchors` (grep). Removes a class of subtle drift bugs.
- **SIMP-2 — `_emit_artifact_anchored` is now private.** The public entry
  point for anchor-mode emission is `emit_artifact_with_anchors`; renaming
  the helper to underscore-prefix makes the API surface unambiguous.
- **SIMP-3 — `_write_task_envelope` helper.** Both v1 and v2 prompt paths
  now share the `--- BEGIN/END TASK PROMPT ---` envelope construction in a
  single helper rather than duplicating across the fork.
- **SIMP-4 — `printf | cut | sort -u` for unique file list.** The nested
  O(n²) for-loops in `emit_profile_context_anchors` collapsed into a single
  POSIX pipeline (still bash 3.2 portable).

### Verified

- **187 PASS / 0 FAIL** across 10 test suites (added 1 new byte-identity
  test for the HIGH-1 fix). No regressions vs v9.0.0. Trust kernel
  unchanged: phase-gate, role-gate, ship-gate, ledger SHA chain.
- Backwards-compat: legacy paths (all flags default-off) produce
  byte-identical output to v9.0.0.

## [9.0.0] - 2026-05-10

_Generated by `scripts/release/changelog-gen.sh` from commits v8.59.0..HEAD._

### Added

- Campaign D Cycle D2 — builder + auditor Layer 1/3 split
- Campaign D Cycle D1 — orchestrator persona progressive disclosure
- Campaign C Cycle C3 — context_anchors in profile JSON
- Campaign C Cycles C1+C2 — anchored artifact extraction
- Campaign B Cycle B2 — role-context-builder consumes cycle-digest
- Campaign B Cycle B1 — cycle-digest.json schema + writer
- Campaign A Cycle A3 — measurement harness + dedup fix
- Campaign A Cycle A2 — bedrock via --append-system-prompt
- Campaign A Cycle A1 — static-first prompt ordering for cache reuse
- v8.60.0 Layer 1 — budget-driven dispatch flags
- cycle 10 — STRICT/DISPATCH flag consolidation + deprecation bridges (10-cycle structural campaign close) --- ## Actual diff (v8.34.0+)
- cycle 9 — EVOLVE_BUDGET_CAP deprecation bridge + builder cost-overrun guard in phase-gate --- ## Actual diff (v8.34.0+)
- cycle 8 — sandbox flag consolidation (EVOLVE_INNER_SANDBOX tri-state) + baseline hoist + triage registration
- cycle 7 (re-dispatch) — remove duplicate control-flags entry + cycle-7 structural re-run baseline --- ## Actual diff (v8.34.0+)
- cycle 7 — fix triage/memo allowlist + flag consolidation + control-flags inventory --- ## Actual diff (v8.34.0+)

### Documentation

- Campaign D Cycle D3 — token-floor history (v9.0.0 close)
- add v8.58.0 Layer I symlink-canonicalization comment --- ## Actual diff (v8.34.0+)


---

## [8.59.0] - 2026-05-10

_Generated by `scripts/release/changelog-gen.sh` from commits v8.58.0..HEAD._

### Added

- v8.59.0 — workspace archive (Layer O) + Triage default-on (Layer T)


---

## [8.58.0] - 2026-05-10

_Generated by `scripts/release/changelog-gen.sh` from commits v8.57.0..HEAD._

### Added

- v8.58.0 — structural enforcement (Layers I+E+B)

### Fixed

- record-failure-to-state.sh treeStateSha — use git rev-parse HEAD^{tree} (recovered from cycle-6 worktree breach)
- instinctCount invariant — sync counter to length(instinctSummary) on append/cap + self-heal drift (Tests 14-17/28 pass) --- ## Actual diff (v8.34.0+)
- failure-adapter cycle-dedup — count_distinct_cycles_by_class prevents retry-storm false BLOCK (Tests 21-23/23 pass) --- ## Actual diff (v8.34.0+)
- learning-pipeline gap — merge-lesson lessonFiles fallback + retrospected flag (Tests 12/13, 22/22 pass) --- ## Actual diff (v8.34.0+)


---

## [8.57.0] - 2026-05-10

_Generated by `scripts/release/changelog-gen.sh` from commits v8.56.0..HEAD._

### Added

- v8.57.0 — continuous backlog (Layers S+D+P)


---

## [8.56.0] - 2026-05-09

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.56.0 — lean cycle (Layers A+B+C)
- v8.55.0 — per-worker budget cap (Phase E) + production posture docs (Phase F)
- v8.55.0 — sequential-write discipline (parallel_eligible enforcement + fanout cap=2) --- ## Actual diff (v8.34.0+)
- v8.54.0 — wire cross-CLI consensus into Auditor + operator UX
- v8.53.0 — cross-CLI consensus aggregator (cross_cli_vote merge mode) + HYBRID mode warning doc fix (DEFECT-2) --- ## Actual diff (v8.34.0+)
- v8.52.0 — Axis A operationalization (consensus prerequisites)
- v8.52.0-pre — multi-LLM architecture review + routing verification + capability composition --- ## Actual diff (v8.34.0+)
- v8.51.0 — capability-based cross-CLI pipeline (graceful degradation)
- v8.50.0 — pipeline dry-run validation harness
- v8.49.0 — bin/ operator entry points
- v8.46.0 — audit-investigations folder structure
- v8.45.0 — auto-retrospective on FAIL/WARN
- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- preflight awk regex handles "## 1. Verdict" numbered headings
- v8.53.0 — add HYBRID mode warning to multi-llm-review.md (DEFECT-2 doc fix missing from main ship) --- ## Actual diff (v8.34.0+)
- v8.51.1 — multi-CLI doc audit P1 fixes
- v8.50.1 — patch v8.47 stale dispatcher path in skills/.agents/
- patch stale scripts/ paths in docs/ post-v8.47 reorg
- scripts/utility/release.sh REPO_ROOT path after v8.47 reorg
- v8.43.0 — worktree-aware shipping (Builder to main bridge restored)
- v8.42.0 — canonical skills location migrated to .agents/skills/
- v8.41.0 — docs/release-notes/ navigation index
- v8.40.0 — docs/research/ thematic subdirectory
- v8.39.0 — update preflight gate-test paths to scripts/tests/
- v8.39.0 — scripts/tests/ subdirectory (segregate test files)
- v8.38.0 — open-source compliance + cross-CLI standards alignment
- v8.37.0 — tamper-evident ledger hash chain (recording layer)
- v8.36.0 — worktree provisioning recovers from stale admin entries
- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)

### Changed

- v8.48.0 — docs/ reorganization + .editorconfig
- v8.47.0 — reorganize scripts/ into 6 thematic subdirs
- token optimization — extract CLAUDE.md history, ledger digest, scout prompt compression

### Documentation

- append cycle-55 verification telemetry to CHANGELOG.md v8.55.0 entry
- record cycle 55 fan-out + budget cap verification in sequential-write-discipline.md --- ## Actual diff (v8.34.0+)
- v8.44.0 — phase architecture deep-dive + research citations


---

## [8.55.0] - 2026-05-09

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Verification (cycle 55, Phase F protocol)

Per the v8.55.0 plan's verify-then-disable protocol, one /evo:loop cycle was run with full fan-out enabled and budget caps engaged to exercise the rails:

```
EVOLVE_FANOUT_ENABLED=1 EVOLVE_FANOUT_SCOUT=1 EVOLVE_FANOUT_AUDITOR=1 \
EVOLVE_MAX_BUDGET_USD=0.30 \
bash evolve-loop-dispatch.sh 1 balanced "<trivial doc edit goal>"
```

**Outcome:** Audit verdict **PASS** (with WARN flag) in 802s wall-time at $3.87 total cost (99% prompt-cache hit rate). The WARN flag surfaced a new defect class — Builder wrote to the main project working tree instead of the per-cycle worktree at `cycle_state.active_worktree`. Content goal was achieved correctly (+1/-0 diff in the target file); the WARN is infrastructure-level (worktree isolation bypass), not content-level.

**Per-phase cost:** auditor=$0.76, builder=$0.23, intent=$0.96, orchestrator=$1.06, retrospective=$0.40, scout=$0.45.

**Production posture confirmed:** v8.55.0 ships with `EVOLVE_FANOUT_ENABLED=0` as default. Operators who opt in receive the full discipline + concurrency cap (default 2) + per-worker budget cap (default $0.20) rails. The WARN about worktree isolation bypass is filed for v8.56+ work; it does not block v8.55.0 publication because the rails themselves operated as designed.

### Added

- v8.55.0 — per-worker budget cap (Phase E) + production posture docs (Phase F)
- v8.55.0 — sequential-write discipline (parallel_eligible enforcement + fanout cap=2) --- ## Actual diff (v8.34.0+)
- v8.54.0 — wire cross-CLI consensus into Auditor + operator UX
- v8.53.0 — cross-CLI consensus aggregator (cross_cli_vote merge mode) + HYBRID mode warning doc fix (DEFECT-2) --- ## Actual diff (v8.34.0+)
- v8.52.0 — Axis A operationalization (consensus prerequisites)
- v8.52.0-pre — multi-LLM architecture review + routing verification + capability composition --- ## Actual diff (v8.34.0+)
- v8.51.0 — capability-based cross-CLI pipeline (graceful degradation)
- v8.50.0 — pipeline dry-run validation harness
- v8.49.0 — bin/ operator entry points
- v8.46.0 — audit-investigations folder structure
- v8.45.0 — auto-retrospective on FAIL/WARN
- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- preflight awk regex handles "## 1. Verdict" numbered headings
- v8.53.0 — add HYBRID mode warning to multi-llm-review.md (DEFECT-2 doc fix missing from main ship) --- ## Actual diff (v8.34.0+)
- v8.51.1 — multi-CLI doc audit P1 fixes
- v8.50.1 — patch v8.47 stale dispatcher path in skills/.agents/
- patch stale scripts/ paths in docs/ post-v8.47 reorg
- scripts/utility/release.sh REPO_ROOT path after v8.47 reorg
- v8.43.0 — worktree-aware shipping (Builder to main bridge restored)
- v8.42.0 — canonical skills location migrated to .agents/skills/
- v8.41.0 — docs/release-notes/ navigation index
- v8.40.0 — docs/research/ thematic subdirectory
- v8.39.0 — update preflight gate-test paths to scripts/tests/
- v8.39.0 — scripts/tests/ subdirectory (segregate test files)
- v8.38.0 — open-source compliance + cross-CLI standards alignment
- v8.37.0 — tamper-evident ledger hash chain (recording layer)
- v8.36.0 — worktree provisioning recovers from stale admin entries
- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)

### Changed

- v8.48.0 — docs/ reorganization + .editorconfig
- v8.47.0 — reorganize scripts/ into 6 thematic subdirs
- token optimization — extract CLAUDE.md history, ledger digest, scout prompt compression

### Documentation

- record cycle 55 fan-out + budget cap verification in sequential-write-discipline.md --- ## Actual diff (v8.34.0+)
- v8.44.0 — phase architecture deep-dive + research citations


---

## [8.54.0] - 2026-05-08

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.54.0 — wire cross-CLI consensus into Auditor + operator UX
- v8.53.0 — cross-CLI consensus aggregator (cross_cli_vote merge mode) + HYBRID mode warning doc fix (DEFECT-2) --- ## Actual diff (v8.34.0+)
- v8.52.0 — Axis A operationalization (consensus prerequisites)
- v8.52.0-pre — multi-LLM architecture review + routing verification + capability composition --- ## Actual diff (v8.34.0+)
- v8.51.0 — capability-based cross-CLI pipeline (graceful degradation)
- v8.50.0 — pipeline dry-run validation harness
- v8.49.0 — bin/ operator entry points
- v8.46.0 — audit-investigations folder structure
- v8.45.0 — auto-retrospective on FAIL/WARN
- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- preflight awk regex handles "## 1. Verdict" numbered headings
- v8.53.0 — add HYBRID mode warning to multi-llm-review.md (DEFECT-2 doc fix missing from main ship) --- ## Actual diff (v8.34.0+)
- v8.51.1 — multi-CLI doc audit P1 fixes
- v8.50.1 — patch v8.47 stale dispatcher path in skills/.agents/
- patch stale scripts/ paths in docs/ post-v8.47 reorg
- scripts/utility/release.sh REPO_ROOT path after v8.47 reorg
- v8.43.0 — worktree-aware shipping (Builder to main bridge restored)
- v8.42.0 — canonical skills location migrated to .agents/skills/
- v8.41.0 — docs/release-notes/ navigation index
- v8.40.0 — docs/research/ thematic subdirectory
- v8.39.0 — update preflight gate-test paths to scripts/tests/
- v8.39.0 — scripts/tests/ subdirectory (segregate test files)
- v8.38.0 — open-source compliance + cross-CLI standards alignment
- v8.37.0 — tamper-evident ledger hash chain (recording layer)
- v8.36.0 — worktree provisioning recovers from stale admin entries
- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)

### Changed

- v8.48.0 — docs/ reorganization + .editorconfig
- v8.47.0 — reorganize scripts/ into 6 thematic subdirs
- token optimization — extract CLAUDE.md history, ledger digest, scout prompt compression

### Documentation

- v8.44.0 — phase architecture deep-dive + research citations


---

## [8.53.0] - 2026-05-08

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.53.0 — cross-CLI consensus aggregator (cross_cli_vote merge mode) + HYBRID mode warning doc fix (DEFECT-2) --- ## Actual diff (v8.34.0+)
- v8.52.0 — Axis A operationalization (consensus prerequisites)
- v8.52.0-pre — multi-LLM architecture review + routing verification + capability composition --- ## Actual diff (v8.34.0+)
- v8.51.0 — capability-based cross-CLI pipeline (graceful degradation)
- v8.50.0 — pipeline dry-run validation harness
- v8.49.0 — bin/ operator entry points
- v8.46.0 — audit-investigations folder structure
- v8.45.0 — auto-retrospective on FAIL/WARN
- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- preflight awk regex handles "## 1. Verdict" numbered headings
- v8.53.0 — add HYBRID mode warning to multi-llm-review.md (DEFECT-2 doc fix missing from main ship) --- ## Actual diff (v8.34.0+)
- v8.51.1 — multi-CLI doc audit P1 fixes
- v8.50.1 — patch v8.47 stale dispatcher path in skills/.agents/
- patch stale scripts/ paths in docs/ post-v8.47 reorg
- scripts/utility/release.sh REPO_ROOT path after v8.47 reorg
- v8.43.0 — worktree-aware shipping (Builder to main bridge restored)
- v8.42.0 — canonical skills location migrated to .agents/skills/
- v8.41.0 — docs/release-notes/ navigation index
- v8.40.0 — docs/research/ thematic subdirectory
- v8.39.0 — update preflight gate-test paths to scripts/tests/
- v8.39.0 — scripts/tests/ subdirectory (segregate test files)
- v8.38.0 — open-source compliance + cross-CLI standards alignment
- v8.37.0 — tamper-evident ledger hash chain (recording layer)
- v8.36.0 — worktree provisioning recovers from stale admin entries
- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)

### Changed

- v8.48.0 — docs/ reorganization + .editorconfig
- v8.47.0 — reorganize scripts/ into 6 thematic subdirs
- token optimization — extract CLAUDE.md history, ledger digest, scout prompt compression

### Documentation

- v8.44.0 — phase architecture deep-dive + research citations


---

## [8.52.0] - 2026-05-08

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.53.0 — cross-CLI consensus aggregator (cross_cli_vote merge mode) + HYBRID mode warning doc fix (DEFECT-2) --- ## Actual diff (v8.34.0+)
- v8.52.0 — Axis A operationalization (consensus prerequisites)
- v8.52.0-pre — multi-LLM architecture review + routing verification + capability composition --- ## Actual diff (v8.34.0+)
- v8.51.0 — capability-based cross-CLI pipeline (graceful degradation)
- v8.50.0 — pipeline dry-run validation harness
- v8.49.0 — bin/ operator entry points
- v8.46.0 — audit-investigations folder structure
- v8.45.0 — auto-retrospective on FAIL/WARN
- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- preflight awk regex handles "## 1. Verdict" numbered headings
- v8.53.0 — add HYBRID mode warning to multi-llm-review.md (DEFECT-2 doc fix missing from main ship) --- ## Actual diff (v8.34.0+)
- v8.51.1 — multi-CLI doc audit P1 fixes
- v8.50.1 — patch v8.47 stale dispatcher path in skills/.agents/
- patch stale scripts/ paths in docs/ post-v8.47 reorg
- scripts/utility/release.sh REPO_ROOT path after v8.47 reorg
- v8.43.0 — worktree-aware shipping (Builder to main bridge restored)
- v8.42.0 — canonical skills location migrated to .agents/skills/
- v8.41.0 — docs/release-notes/ navigation index
- v8.40.0 — docs/research/ thematic subdirectory
- v8.39.0 — update preflight gate-test paths to scripts/tests/
- v8.39.0 — scripts/tests/ subdirectory (segregate test files)
- v8.38.0 — open-source compliance + cross-CLI standards alignment
- v8.37.0 — tamper-evident ledger hash chain (recording layer)
- v8.36.0 — worktree provisioning recovers from stale admin entries
- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)

### Changed

- v8.48.0 — docs/ reorganization + .editorconfig
- v8.47.0 — reorganize scripts/ into 6 thematic subdirs
- token optimization — extract CLAUDE.md history, ledger digest, scout prompt compression

### Documentation

- v8.44.0 — phase architecture deep-dive + research citations


---

## [8.51.1] - 2026-05-08

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.51.0 — capability-based cross-CLI pipeline (graceful degradation)
- v8.50.0 — pipeline dry-run validation harness
- v8.49.0 — bin/ operator entry points
- v8.46.0 — audit-investigations folder structure
- v8.45.0 — auto-retrospective on FAIL/WARN
- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- v8.51.1 — multi-CLI doc audit P1 fixes
- v8.50.1 — patch v8.47 stale dispatcher path in skills/.agents/
- patch stale scripts/ paths in docs/ post-v8.47 reorg
- scripts/utility/release.sh REPO_ROOT path after v8.47 reorg
- v8.43.0 — worktree-aware shipping (Builder to main bridge restored)
- v8.42.0 — canonical skills location migrated to .agents/skills/
- v8.41.0 — docs/release-notes/ navigation index
- v8.40.0 — docs/research/ thematic subdirectory
- v8.39.0 — update preflight gate-test paths to scripts/tests/
- v8.39.0 — scripts/tests/ subdirectory (segregate test files)
- v8.38.0 — open-source compliance + cross-CLI standards alignment
- v8.37.0 — tamper-evident ledger hash chain (recording layer)
- v8.36.0 — worktree provisioning recovers from stale admin entries
- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)

### Changed

- v8.48.0 — docs/ reorganization + .editorconfig
- v8.47.0 — reorganize scripts/ into 6 thematic subdirs
- token optimization — extract CLAUDE.md history, ledger digest, scout prompt compression

### Documentation

- v8.44.0 — phase architecture deep-dive + research citations


---

## [8.51.0] - 2026-05-08

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.51.0 — capability-based cross-CLI pipeline (graceful degradation)
- v8.50.0 — pipeline dry-run validation harness
- v8.49.0 — bin/ operator entry points
- v8.46.0 — audit-investigations folder structure
- v8.45.0 — auto-retrospective on FAIL/WARN
- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- v8.50.1 — patch v8.47 stale dispatcher path in skills/.agents/
- patch stale scripts/ paths in docs/ post-v8.47 reorg
- scripts/utility/release.sh REPO_ROOT path after v8.47 reorg
- v8.43.0 — worktree-aware shipping (Builder to main bridge restored)
- v8.42.0 — canonical skills location migrated to .agents/skills/
- v8.41.0 — docs/release-notes/ navigation index
- v8.40.0 — docs/research/ thematic subdirectory
- v8.39.0 — update preflight gate-test paths to scripts/tests/
- v8.39.0 — scripts/tests/ subdirectory (segregate test files)
- v8.38.0 — open-source compliance + cross-CLI standards alignment
- v8.37.0 — tamper-evident ledger hash chain (recording layer)
- v8.36.0 — worktree provisioning recovers from stale admin entries
- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)

### Changed

- v8.48.0 — docs/ reorganization + .editorconfig
- v8.47.0 — reorganize scripts/ into 6 thematic subdirs
- token optimization — extract CLAUDE.md history, ledger digest, scout prompt compression

### Documentation

- v8.44.0 — phase architecture deep-dive + research citations


---

## [8.50.1] - 2026-05-08

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.50.0 — pipeline dry-run validation harness
- v8.49.0 — bin/ operator entry points
- v8.46.0 — audit-investigations folder structure
- v8.45.0 — auto-retrospective on FAIL/WARN
- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- v8.50.1 — patch v8.47 stale dispatcher path in skills/.agents/
- patch stale scripts/ paths in docs/ post-v8.47 reorg
- scripts/utility/release.sh REPO_ROOT path after v8.47 reorg
- v8.43.0 — worktree-aware shipping (Builder to main bridge restored)
- v8.42.0 — canonical skills location migrated to .agents/skills/
- v8.41.0 — docs/release-notes/ navigation index
- v8.40.0 — docs/research/ thematic subdirectory
- v8.39.0 — update preflight gate-test paths to scripts/tests/
- v8.39.0 — scripts/tests/ subdirectory (segregate test files)
- v8.38.0 — open-source compliance + cross-CLI standards alignment
- v8.37.0 — tamper-evident ledger hash chain (recording layer)
- v8.36.0 — worktree provisioning recovers from stale admin entries
- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)

### Changed

- v8.48.0 — docs/ reorganization + .editorconfig
- v8.47.0 — reorganize scripts/ into 6 thematic subdirs
- token optimization — extract CLAUDE.md history, ledger digest, scout prompt compression

### Documentation

- v8.44.0 — phase architecture deep-dive + research citations


---

## [8.50.0] - 2026-05-08

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.50.0 — pipeline dry-run validation harness
- v8.49.0 — bin/ operator entry points
- v8.46.0 — audit-investigations folder structure
- v8.45.0 — auto-retrospective on FAIL/WARN
- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- patch stale scripts/ paths in docs/ post-v8.47 reorg
- scripts/utility/release.sh REPO_ROOT path after v8.47 reorg
- v8.43.0 — worktree-aware shipping (Builder to main bridge restored)
- v8.42.0 — canonical skills location migrated to .agents/skills/
- v8.41.0 — docs/release-notes/ navigation index
- v8.40.0 — docs/research/ thematic subdirectory
- v8.39.0 — update preflight gate-test paths to scripts/tests/
- v8.39.0 — scripts/tests/ subdirectory (segregate test files)
- v8.38.0 — open-source compliance + cross-CLI standards alignment
- v8.37.0 — tamper-evident ledger hash chain (recording layer)
- v8.36.0 — worktree provisioning recovers from stale admin entries
- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)

### Changed

- v8.48.0 — docs/ reorganization + .editorconfig
- v8.47.0 — reorganize scripts/ into 6 thematic subdirs
- token optimization — extract CLAUDE.md history, ledger digest, scout prompt compression

### Documentation

- v8.44.0 — phase architecture deep-dive + research citations


---

## [8.49.0] - 2026-05-08

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.49.0 — bin/ operator entry points
- v8.46.0 — audit-investigations folder structure
- v8.45.0 — auto-retrospective on FAIL/WARN
- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- patch stale scripts/ paths in docs/ post-v8.47 reorg
- scripts/utility/release.sh REPO_ROOT path after v8.47 reorg
- v8.43.0 — worktree-aware shipping (Builder to main bridge restored)
- v8.42.0 — canonical skills location migrated to .agents/skills/
- v8.41.0 — docs/release-notes/ navigation index
- v8.40.0 — docs/research/ thematic subdirectory
- v8.39.0 — update preflight gate-test paths to scripts/tests/
- v8.39.0 — scripts/tests/ subdirectory (segregate test files)
- v8.38.0 — open-source compliance + cross-CLI standards alignment
- v8.37.0 — tamper-evident ledger hash chain (recording layer)
- v8.36.0 — worktree provisioning recovers from stale admin entries
- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)

### Changed

- v8.48.0 — docs/ reorganization + .editorconfig
- v8.47.0 — reorganize scripts/ into 6 thematic subdirs
- token optimization — extract CLAUDE.md history, ledger digest, scout prompt compression

### Documentation

- v8.44.0 — phase architecture deep-dive + research citations


---

## [8.48.0] - 2026-05-08

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.46.0 — audit-investigations folder structure
- v8.45.0 — auto-retrospective on FAIL/WARN
- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- patch stale scripts/ paths in docs/ post-v8.47 reorg
- scripts/utility/release.sh REPO_ROOT path after v8.47 reorg
- v8.43.0 — worktree-aware shipping (Builder to main bridge restored)
- v8.42.0 — canonical skills location migrated to .agents/skills/
- v8.41.0 — docs/release-notes/ navigation index
- v8.40.0 — docs/research/ thematic subdirectory
- v8.39.0 — update preflight gate-test paths to scripts/tests/
- v8.39.0 — scripts/tests/ subdirectory (segregate test files)
- v8.38.0 — open-source compliance + cross-CLI standards alignment
- v8.37.0 — tamper-evident ledger hash chain (recording layer)
- v8.36.0 — worktree provisioning recovers from stale admin entries
- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)

### Changed

- v8.48.0 — docs/ reorganization + .editorconfig
- v8.47.0 — reorganize scripts/ into 6 thematic subdirs
- token optimization — extract CLAUDE.md history, ledger digest, scout prompt compression

### Documentation

- v8.44.0 — phase architecture deep-dive + research citations


---

## [8.47.0] - 2026-05-08

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.46.0 — audit-investigations folder structure
- v8.45.0 — auto-retrospective on FAIL/WARN
- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- scripts/utility/release.sh REPO_ROOT path after v8.47 reorg
- v8.43.0 — worktree-aware shipping (Builder to main bridge restored)
- v8.42.0 — canonical skills location migrated to .agents/skills/
- v8.41.0 — docs/release-notes/ navigation index
- v8.40.0 — docs/research/ thematic subdirectory
- v8.39.0 — update preflight gate-test paths to scripts/tests/
- v8.39.0 — scripts/tests/ subdirectory (segregate test files)
- v8.38.0 — open-source compliance + cross-CLI standards alignment
- v8.37.0 — tamper-evident ledger hash chain (recording layer)
- v8.36.0 — worktree provisioning recovers from stale admin entries
- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)

### Changed

- v8.47.0 — reorganize scripts/ into 6 thematic subdirs
- token optimization — extract CLAUDE.md history, ledger digest, scout prompt compression

### Documentation

- v8.44.0 — phase architecture deep-dive + research citations


---

## [8.46.0] - 2026-05-08

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.46.0 — audit-investigations folder structure
- v8.45.0 — auto-retrospective on FAIL/WARN
- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- v8.43.0 — worktree-aware shipping (Builder to main bridge restored)
- v8.42.0 — canonical skills location migrated to .agents/skills/
- v8.41.0 — docs/release-notes/ navigation index
- v8.40.0 — docs/research/ thematic subdirectory
- v8.39.0 — update preflight gate-test paths to scripts/tests/
- v8.39.0 — scripts/tests/ subdirectory (segregate test files)
- v8.38.0 — open-source compliance + cross-CLI standards alignment
- v8.37.0 — tamper-evident ledger hash chain (recording layer)
- v8.36.0 — worktree provisioning recovers from stale admin entries
- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)

### Changed

- token optimization — extract CLAUDE.md history, ledger digest, scout prompt compression

### Documentation

- v8.44.0 — phase architecture deep-dive + research citations


---

## [8.45.0] - 2026-05-08

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.45.0 — auto-retrospective on FAIL/WARN
- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- v8.43.0 — worktree-aware shipping (Builder to main bridge restored)
- v8.42.0 — canonical skills location migrated to .agents/skills/
- v8.41.0 — docs/release-notes/ navigation index
- v8.40.0 — docs/research/ thematic subdirectory
- v8.39.0 — update preflight gate-test paths to scripts/tests/
- v8.39.0 — scripts/tests/ subdirectory (segregate test files)
- v8.38.0 — open-source compliance + cross-CLI standards alignment
- v8.37.0 — tamper-evident ledger hash chain (recording layer)
- v8.36.0 — worktree provisioning recovers from stale admin entries
- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)

### Changed

- token optimization — extract CLAUDE.md history, ledger digest, scout prompt compression

### Documentation

- v8.44.0 — phase architecture deep-dive + research citations


---

## [8.44.0] - 2026-05-08

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- v8.43.0 — worktree-aware shipping (Builder to main bridge restored)
- v8.42.0 — canonical skills location migrated to .agents/skills/
- v8.41.0 — docs/release-notes/ navigation index
- v8.40.0 — docs/research/ thematic subdirectory
- v8.39.0 — update preflight gate-test paths to scripts/tests/
- v8.39.0 — scripts/tests/ subdirectory (segregate test files)
- v8.38.0 — open-source compliance + cross-CLI standards alignment
- v8.37.0 — tamper-evident ledger hash chain (recording layer)
- v8.36.0 — worktree provisioning recovers from stale admin entries
- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)

### Changed

- token optimization — extract CLAUDE.md history, ledger digest, scout prompt compression

### Documentation

- v8.44.0 — phase architecture deep-dive + research citations


---

## [8.43.0] - 2026-05-07

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- v8.43.0 — worktree-aware shipping (Builder to main bridge restored)
- v8.42.0 — canonical skills location migrated to .agents/skills/
- v8.41.0 — docs/release-notes/ navigation index
- v8.40.0 — docs/research/ thematic subdirectory
- v8.39.0 — update preflight gate-test paths to scripts/tests/
- v8.39.0 — scripts/tests/ subdirectory (segregate test files)
- v8.38.0 — open-source compliance + cross-CLI standards alignment
- v8.37.0 — tamper-evident ledger hash chain (recording layer)
- v8.36.0 — worktree provisioning recovers from stale admin entries
- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)


---

## [8.42.0] - 2026-05-07

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- v8.42.0 — canonical skills location migrated to .agents/skills/
- v8.41.0 — docs/release-notes/ navigation index
- v8.40.0 — docs/research/ thematic subdirectory
- v8.39.0 — update preflight gate-test paths to scripts/tests/
- v8.39.0 — scripts/tests/ subdirectory (segregate test files)
- v8.38.0 — open-source compliance + cross-CLI standards alignment
- v8.37.0 — tamper-evident ledger hash chain (recording layer)
- v8.36.0 — worktree provisioning recovers from stale admin entries
- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)


---

## [8.41.0] - 2026-05-07

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- v8.41.0 — docs/release-notes/ navigation index
- v8.40.0 — docs/research/ thematic subdirectory
- v8.39.0 — update preflight gate-test paths to scripts/tests/
- v8.39.0 — scripts/tests/ subdirectory (segregate test files)
- v8.38.0 — open-source compliance + cross-CLI standards alignment
- v8.37.0 — tamper-evident ledger hash chain (recording layer)
- v8.36.0 — worktree provisioning recovers from stale admin entries
- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)


---

## [8.40.0] - 2026-05-07

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- v8.40.0 — docs/research/ thematic subdirectory
- v8.39.0 — update preflight gate-test paths to scripts/tests/
- v8.39.0 — scripts/tests/ subdirectory (segregate test files)
- v8.38.0 — open-source compliance + cross-CLI standards alignment
- v8.37.0 — tamper-evident ledger hash chain (recording layer)
- v8.36.0 — worktree provisioning recovers from stale admin entries
- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)


---

## [8.39.0] - 2026-05-07

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- v8.39.0 — update preflight gate-test paths to scripts/tests/
- v8.39.0 — scripts/tests/ subdirectory (segregate test files)
- v8.38.0 — open-source compliance + cross-CLI standards alignment
- v8.37.0 — tamper-evident ledger hash chain (recording layer)
- v8.36.0 — worktree provisioning recovers from stale admin entries
- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)


---

## [8.38.0] - 2026-05-07

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- v8.38.0 — open-source compliance + cross-CLI standards alignment
- v8.37.0 — tamper-evident ledger hash chain (recording layer)
- v8.36.0 — worktree provisioning recovers from stale admin entries
- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)


---

## [8.37.0] - 2026-05-07

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- v8.37.0 — tamper-evident ledger hash chain (recording layer)
- v8.36.0 — worktree provisioning recovers from stale admin entries
- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)


---

## [8.36.0] - 2026-05-07

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- v8.36.0 — worktree provisioning recovers from stale admin entries
- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)


---

## [8.35.0] - 2026-05-07

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.35.0 — orchestrator fluency on WARN + adaptive auditor cost
- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)


---

## [8.34.0] - 2026-05-07

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.34.0 — pipeline continuation + diff transparency
- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)


---

## [8.33.0] - 2026-05-07

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.33.0 — token optimization (cache-friendly prompts + cost telemetry)
- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)


---

## [8.32.0] - 2026-05-06

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- v8.32.0 — version-aware TOFU (closes plugin-update SHA trap)
- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)


---

## [8.31.0] - 2026-05-06

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- v8.31.0 — Builder write-leak (#30 closed)
- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)


---

## [8.30.1] - 2026-05-06

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- v8.30.1 — full cycle-dir wipe at run-cycle init (per downstream user analysis)
- release-pipeline.sh ship invocation uses --class release (v8.25.0)


---

## [8.30.0] - 2026-05-06

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.30.0 — operational polish (4 v8.30 candidates closed)
- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- release-pipeline.sh ship invocation uses --class release (v8.25.0)


---

## [8.29.0] - 2026-05-06

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.29.0 — cycle-25 work + run-cycle.sh trap order fix + stale-artifact cleanup
- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- release-pipeline.sh ship invocation uses --class release (v8.25.0)


---

## [8.28.0] - 2026-05-06

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.28.0 — fluent-by-default (loop continuation as top priority)
- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- release-pipeline.sh ship invocation uses --class release (v8.25.0)


---

## [8.27.0] - 2026-05-06

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.27.0 — loop continuation (relaxed exit-code, ship-gate-config class, --reset flag)
- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- release-pipeline.sh ship invocation uses --class release (v8.25.0)


---

## [8.26.0] - 2026-05-06

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.26.0 — budget cap default-unlimited (eliminate BUDGET_EXCEEDED friction)
- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- release-pipeline.sh ship invocation uses --class release (v8.25.0)


---

## [8.25.1] - 2026-05-05

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.25.1 — disable inner sandbox-exec in nested-Claude (closes EPERM saga)
- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- release-pipeline.sh ship invocation uses --class release (v8.25.0)


---

## [8.25.0] - 2026-05-05

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.25.0 — capability-detection preflight + worktree relocation + ship classes
- v8.24.0 — three-tier strictness model + dispatcher safety nets

### Fixed

- release-pipeline.sh ship invocation uses --class release (v8.25.0)


---

## [8.24.0] - 2026-05-05

_Generated by `scripts/release/changelog-gen.sh` from commits v8.21.1..HEAD._

### Added

- v8.24.0 — three-tier strictness model + dispatcher safety nets


---

## [8.23.4] - 2026-05-05

Defense-in-depth on top of v8.23.3. v8.23.3 fixed three concrete bugs (BUG-008..010), but the user's failure mode included "Claude binary's own permission layer blocks writes despite --add-dir" — a class of failure that may persist in some nested-claude environments even after v8.23.3's cwd fix. v8.23.4 adds a clean operator escape hatch and diagnostic logging so the next failure is loud and self-explaining.

### Added

- **`EVOLVE_SKIP_WORKTREE=1` escape hatch** (`scripts/dispatch/run-cycle.sh`): bypasses worktree provisioning entirely when the parent Claude Code OS sandbox blocks `.evolve/worktrees/` writes regardless of cwd. Sets `cycle-state.active_worktree = $EVOLVE_PROJECT_ROOT`. Builder/tdd-engineer profiles pre-expand to the main repo path. Loud WARN logs make the no-isolation tradeoff explicit. At cleanup, if the main repo has uncommitted changes from the cycle, the operator is told to inspect via `git status` / `git diff` and decide manually.

- **EPERM-class diagnostic block** (`scripts/cli_adapters/claude.sh`): when the inner `claude -p` exits with rc≠0 AND stderr contains EACCES/EPERM/permission-denied/sandbox_apply patterns, dump structured triage data to dispatcher stderr:
  - cwd at exec time
  - `--add-dir` value
  - sandbox-use + fallback flag state
  - parent `CLAUDECODE` env presence
  - last 30 lines of stderr
  - explicit operator-action recommendations (set EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1 or EVOLVE_SKIP_WORKTREE=1 with caveats)

### Tradeoffs documented

`EVOLVE_SKIP_WORKTREE=1` removes the v8.21.0 isolation guarantee:
- Pro: cycles can land on environments where worktree writes are blocked by parent OS sandbox
- Con: builder edits land directly in main repo; failed audit means manual `git restore .` or selective rollback
- When to flip on: nested Claude Code session where v8.23.3 still EPERMs at builder phase
- When NOT: standalone shell (no parent sandbox issue to work around)
- Cleanup: at exit, run-cycle.sh logs "main repo has N changed file(s)" if cycle made any changes — operator's responsibility to commit, stash, or restore

### Verification

- Dry-run with `EVOLVE_SKIP_WORKTREE=1`: WARN logged, `active_worktree` set to main repo, cleanup correctly skips worktree-remove, dirty-file warning fires
- Default behavior unchanged when flag unset (worktree provisioned + cleaned up as before)
- 29/29 regression tests pass

### How it composes with v8.23.3

v8.23.3 fixed three concrete bugs (set-e dead fallback, v8.23.2 cwd regression, intent budget). v8.23.4 adds belt-and-suspenders for the residual class of "even with all v8.23.3 fixes applied, the parent OS sandbox cascades to deny writes." Together:

| If you hit | Set | Effect |
|---|---|---|
| sandbox-exec EPERM at startup | `EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1` (auto-set by dispatcher in nested-claude) | Retry without sandbox-exec, kernel hooks remain |
| Builder cd in wrong dir | (no flag — v8.23.3 fixes structurally) | WORKING_DIR honors `EVOLVE_PROFILE_WORKTREE_AWARE=1` env hint |
| Worktree EPERM despite correct cwd | `EVOLVE_SKIP_WORKTREE=1` | Builder writes to main repo directly (no isolation) |
| Intent budget overflow | `EVOLVE_TASK_MODE=heavy_rules` | $1.00 budget instead of $0.50 default |

---

## [8.23.3] - 2026-05-05

Three architectural bugs landed in one fix release. A second-Claude session running cycle 1 of `/loop 10` against a downstream `/learning` project surfaced all three after ~17 minutes:
1. The `EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1` flag had been **silently dead since v8.16.1** due to a `set -e` interaction.
2. v8.23.2's profile pre-expansion **introduced a regression** that broke claude.sh's worktree-cwd detection.
3. Intent persona's $0.25 budget was insufficient for projects with heavy CLAUDE.md, returning rc=1 even when the artifact was complete.

### Fixed

- **BUG-008 (CRITICAL)**: `scripts/cli_adapters/claude.sh` had `set -euo pipefail` enabled at line 36 + 5 places using `<command>; EXIT_CODE=$?` to capture exit codes. When the inner command failed (e.g., sandbox-exec returned 71/EPERM), `set -e` exited the script BEFORE `EXIT_CODE=$?` ran. The Darwin-25.4 nested-sandbox fallback `if` block at line 363 NEVER fired in production — the documented `EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1` workaround has been silently dead since v8.16.1 introduced it. **Fixed**: all 5 EXIT_CODE assignments now use `|| EXIT_CODE=$?` (the bash idiom that lets `set -e` skip the failure as a logical-or branch). `EXIT_CODE=0` initialized at the top of the execute block so the success path retains rc=0.

  Reproduction (pre-fix):
  ```bash
  $ bash -c 'set -euo pipefail; echo before; /usr/bin/false; EC=$?; echo "EC=$EC"'
  before
  $ echo "rc=$?"
  rc=1
  # ↑ "EC=" line never printed; script exited at false
  ```

- **BUG-009 (HIGH, regression from v8.23.2)**: subagent-run.sh's profile pre-expansion replaced `{worktree_path}` placeholders in the profile JSON with the actual path. claude.sh's WORKING_DIR detection at line ~165 still looked for the literal `{worktree_path}` string to determine "is this a worktree-aware profile?" — and after v8.23.2's pre-expansion, that string was never found. Result: WORKING_DIR fell through to `$PWD` (main repo), claude `cd`'d into the main repo, builder Edits targeted main-repo absolute paths but `--add-dir` was the worktree-only path → Edit denied at the Claude tool layer. **Fixed**: subagent-run.sh now exports `EVOLVE_PROFILE_WORKTREE_AWARE=1` whenever the ORIGINAL profile contained `{worktree_path}` (BEFORE expansion). claude.sh checks this env hint as the canonical signal; the literal-string match remains as a fallback for direct adapter invocations that bypass subagent-run.sh.

- **BUG-010 (MEDIUM)**: `.evolve/profiles/intent.json:max_budget_usd` was 0.25 — too tight for projects with heavy CLAUDE.md (>3K tokens). Intent persona running opus + reading rules blew the cap in 3 turns, returning rc=1 even though intent.md was complete. **Fixed**: raised default to $0.50, added `budget_tiers` (default $0.50, `heavy_rules` $1.00, `minimal` $0.20). Set `EVOLVE_TASK_MODE=heavy_rules` for projects whose CLAUDE.md is >3K tokens.

### Verification

- 29/29 regression tests pass
- `set -e` fallback reproduction (post-fix): `EC=$?` correctly captures the failed command's exit code, and the EPERM-fallback `if` block executes
- subagent-run.sh exports `EVOLVE_PROFILE_WORKTREE_AWARE=1` when original profile has `{worktree_path}`; claude.sh honors it; WORKING_DIR resolves to the worktree

### Migration

No migration needed. The fixes are transparent: existing dispatcher invocations automatically benefit from all three. If you've hit the v8.16.1+ silent EPERM-fallback bug and worked around it by exporting `EVOLVE_SANDBOX=0`, you can now stop — `EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1` (default-set by the dispatcher in nested-claude scenarios since v8.22.0) actually works.

### What 4 v8.23 patches in 2 days reveal

| Patch | Bug | Discovery path |
|---|---|---|
| v8.23.0 | (feature) Tasks B/C/D from cycle 25 research | Planned |
| v8.23.1 | `failure_compute_expires_at` jq-quoting bug | Cycle 53 in /learning project surfaced "everything blocked at calibrate" |
| v8.23.2 | `{worktree_path}` substitution gap across adapters | Cycle 53 follow-up surfaced builder write EPERM |
| v8.23.3 | `set -e` + EXIT_CODE bug, v8.23.2 regression, intent budget | Cycle 1 of `/loop 10` against /learning surfaced all three |

The pattern: **production cycles surface bugs that unit tests can't.** Each v8.23.x patch closed a layer that had been silently broken for one or more prior versions. v8.16.1's introduction of `EVOLVE_SANDBOX_FALLBACK_ON_EPERM` shipped a flag that never fired (the `set -e` interaction wasn't tested) — until now, 7 minor versions later. Adding integration tests that actually trigger sandbox-exec EPERM (e.g., via a dummy sandbox profile that always returns 71) is now a v8.24+ candidate.

---

## [8.23.2] - 2026-05-05

Defense-in-depth fix for `{worktree_path}` placeholder expansion. A second-Claude session running cycle 53 against a downstream `/learning` project surfaced this: builder Edits returned EPERM despite WORKTREE_PATH being set, despite v8.21.0's claude.sh substitution, despite EVOLVE_SANDBOX=0. The orchestrator's diagnosis: "in subagent-run.sh, substitute `{worktree_path}` with `$WORKTREE_PATH` (already exported by run-cycle.sh) before invoking the builder profile."

Pre-v8.23.2, only `scripts/cli_adapters/claude.sh` substituted `{worktree_path}` — and only at TWO specific sites (`add_dir`, `sandbox.write_subpaths`). Other adapters (`gemini.sh`, `codex.sh`) and any tool-permission engine that consulted the profile JSON directly would see literal `{worktree_path}` tokens. v8.23.2 moves substitution upstream into `subagent-run.sh:cmd_run` so EVERY adapter gets a fully-expanded profile.

### Fixed

- **BUG-007 (HIGH)**: Builder `Edit` calls returning EPERM despite worktree being correctly provisioned and `WORKTREE_PATH` correctly set in the env. Root cause was an undiscovered substitution gap — likely Claude Code's tool-permission engine consulting an unexpanded profile field outside `add_dir` and `sandbox.write_subpaths`.

### Added

- `scripts/dispatch/subagent-run.sh:cmd_run` — pre-expands `{worktree_path}` in EVERY string field of the profile JSON via `jq walk`. Result is written to `<workspace>/<agent>-profile-expanded.json` (per-cycle, traceable artifact). `PROFILE_PATH` env var passed to the adapter points at the expanded copy. The original profile in `.evolve/profiles/` is never modified.
- The expanded profile is opt-in: only triggered when `grep '{worktree_path}'` matches in the profile (no perf impact for profiles without placeholders).

### Changed

- `scripts/cli_adapters/claude.sh` substitution remains in place as second-line defense — but it should now never need to fire (the profile arrives already expanded). If it does fire, that's a signal that subagent-run.sh's expansion path was bypassed.

### Verification

- `jq --arg wp '/path' 'walk(if type == "string" then gsub("\\{worktree_path\\}"; $wp) else . end)' builder.json` correctly expands all 3 placeholder occurrences (`add_dir[0]`, `sandbox.write_subpaths[0]`).
- `tdd-engineer.json` with subpath-style placeholders (`{worktree_path}/tests`, `{worktree_path}/scripts/*-test.sh`) also correctly expands.
- Profiles without placeholders (orchestrator, scout, auditor) are no-ops — `walk` doesn't change them.
- 29/29 regression tests pass.

### Migration

No migration needed. The fix is transparent: existing code calling `cmd_run` automatically gets the expanded profile. If `WORKTREE_PATH` is unset (atypical for builder/tdd-engineer cycles), a WARN is logged and the original profile is used (adapter then enforces its own check).

### Out of scope

- Direct `bash scripts/cli_adapters/claude.sh` invocations bypass `subagent-run.sh` and lose the pre-expansion benefit. They still rely on claude.sh's own substitution at `add_dir` and `sandbox.write_subpaths`. This is acceptable: production code paths always go through `subagent-run.sh`; only test fixtures invoke the adapter directly, and they typically don't use `{worktree_path}` profiles.

---

## [8.23.1] - 2026-05-05

Critical hotfix for v8.22.0's `failure_compute_expires_at`. A second-Claude-session running 10 cycles against a downstream project surfaced the symptom: every cycle blocked at calibrate with `BLOCKED-SYSTEMIC` despite the failure-adapter being designed to age out infrastructure failures after 1 day. Root cause: the v8.22.0 expiresAt computation was silently broken since release.

### Fixed

- **`scripts/failure/failure-classifications.sh:failure_compute_expires_at`**: pre-v8.23.1 used `echo "$now_iso" | jq -r '. | fromdateiso8601'`. ISO timestamps like `2026-05-05T03:30:13Z` are NOT valid JSON without explicit string quotes — jq parses the leading digit as a number and blows up at the first dash. The error went to stderr; stdout was empty. Bash arithmetic `(( "" + 86400 ))` then yielded `86400`, which `jq -r '. | todate'` formatted as `1970-01-02T00:00:00Z`. Every v8.22.0 expiresAt was actually 1 day after the epoch — but the dispatcher swallowed the stderr, so the bug was invisible in cycle logs. **Fixed**: use `jq -rn --arg s "$now_iso" '$s | fromdateiso8601'` (proper JSON-string injection), validate the result is numeric before doing arithmetic, fall back to `date -u +%s` when conversion fails. Plus an integer-validation guard on `now_s` to refuse silent zero-substitution.

- **Legacy null-expiresAt poisoning** (`scripts/failure/failure-adapter.sh` + `scripts/lifecycle/cycle-state.sh:prune-expired-failures`): pre-v8.23.1 kept entries with `expiresAt: null` indefinitely "for backward compat." In practice this meant 18+ legacy `infrastructure`-classification entries from v8.21.x days never aged out, permanently triggering the adapter's "3+ consecutive infrastructure-transient" → `BLOCK-OPERATOR-ACTION` rule. **Fixed**: when `expiresAt` is null but `recordedAt` is present, use `recordedAt + 1d` as the effective TTL (matches the tightest classification age-out window). Truly ancient entries with both fields null are still kept (defensive — they're rare and inert with no recognizable classification).

### Migration

If you have an existing `state.json` poisoned with null-expiresAt entries from v8.22.x:
- Automatic: `bash scripts/lifecycle/cycle-state.sh prune-expired-failures` will now remove entries whose `recordedAt` is older than 1 day. Run once after upgrading.
- Manual surgical: `bash scripts/failure/state-prune.sh --classification infrastructure` removes only the legacy free-form entries while keeping any structured-taxonomy code failures.
- Nuclear: `bash scripts/failure/state-prune.sh --all --yes` wipes everything.

### Verification

- `bash scripts/failure/failure-classifications.sh` → `failure_compute_expires_at infrastructure-transient "2026-05-05T03:30:13Z"` now returns `2026-05-06T03:30:13Z` (correct +1d) instead of `1970-01-02T00:00:00Z`
- 29/29 regression tests pass
- Synthetic 4-entry state.json with 25-hour-old `recordedAt` + null `expiresAt` → adapter returns `PROCEED` with `non_expired_count=0`

### Why this slipped through v8.22.0's tests

The v8.22.0 unit tests for `failure_compute_expires_at` (in `failure-classifications.sh` smoke checks) used the function with no second argument — the path that defaults to `date -u +%s` for `now_s`, bypassing the broken jq invocation. The bug only fired when `record-failure-to-state.sh` and the dispatcher passed an explicit ISO timestamp as the second argument, which only happened in real cycle runs. Lesson: always test the 2-arg form when adding such functions.

---

## [8.23.0] - 2026-05-05

Swarm optimization release. Implements Tasks B, C, D from cycle 25's autoresearch scout-report — three optimizations to the existing Sprint 1 Pattern-3 fan-out (parallel sub-personas for Scout/Auditor/Retrospective). All three are env-flag-gated so v8.22.0 behavior is preserved when disabled.

### Added

- **Task D — `parallel_workers.workers[]` observability** (`EVOLVE_FANOUT_TRACK_WORKERS=1` default-on):
  - `scripts/lifecycle/cycle-state.sh init-workers <agent> <name>...` — initialize all workers in `pending` status before fan-out dispatches.
  - `scripts/lifecycle/cycle-state.sh set-worker-status <name> <status> [<exit_code>]` — atomic upsert for `pending` → `running` → `done` / `failed` transitions. Records `started_at` on `running`, `ended_at` + `exit_code` on terminal statuses.
  - `scripts/dispatch/fanout-dispatch.sh:_run_worker` calls these at subprocess start/end so orchestrator can read `cycle-state.json` without re-scanning workspace artifacts.
  - 4 new tests in `scripts/cycle-state-test.sh` (init-workers, status transitions, terminal records, invalid-status rejection).

- **Task C — Shared prompt-cache prefix across siblings** (`EVOLVE_FANOUT_CACHE_PREFIX=1` default-on):
  - `scripts/dispatch/subagent-run.sh:cmd_dispatch_parallel` writes `workers/cache-prefix.md` — a deterministic shared context block (cycle goal, condensed cycle-state, trust-boundary reminders).
  - `scripts/dispatch/fanout-dispatch.sh` accepts new `--cache-prefix-file=PATH` flag; exports `EVOLVE_FANOUT_CACHE_PREFIX_FILE` to each worker subprocess.
  - Same cycle + same workspace → byte-identical prefix bytes (no timestamps, no random salts). Sibling workers in the same fan-out batch hit Anthropic's prompt cache (≥1024 token, 5-min TTL) for ~47% input-token reduction on 3-worker fan-out.
  - 2 new tests in `fanout-dispatch-test.sh` (--cache-prefix-file flag accepted; missing file → exit 2). 3 new tests in `dispatch-parallel-test.sh` (file written; deterministic SHA across re-runs; opt-out flag respected).

- **Task B — Early-cancel on consensus** (`EVOLVE_FANOUT_CANCEL_ON_CONSENSUS=1` opt-in):
  - `scripts/dispatch/fanout-dispatch.sh:_check_fail_consensus` — polls completed workers' `.out` files for `Verdict: FAIL` (inline OR `## Verdict\n**FAIL**` heading-form). When `>= EVOLVE_FANOUT_CONSENSUS_K` workers (default 2) agree on FAIL, SIGTERMs remaining background PIDs.
  - Audit-style fan-out only — `verdict` merge mode has the binary FAIL/PASS semantics that map cleanly. Other merge modes (concat for scout/retro, plan_review for sprint-2 lenses) keep WAIT-ALL.
  - 2 new tests (consensus-cancel observed in <4s with synthetic 4-worker job; default-off behavior unchanged).

### Changed

- `scripts/dispatch/fanout-dispatch.sh` — argument parser refactored from positional-only to flag-aware. Backward-compat: bare `<cmds.tsv> <results.tsv>` invocation still works.
- `scripts/lifecycle/cycle-state.sh` CLI usage line — extended with `init-workers` and `set-worker-status` subcommands.
- `.evolve/profiles/orchestrator.json` — no profile change required (workers are spawned by `fanout-dispatch.sh`, not directly by the orchestrator).

### Performance

| Scenario | v8.22.0 | v8.23.0 | Δ |
|---|---|---|---|
| 3-worker scout (all PASS) | 3× input tokens | ~1.5× input tokens | -47% |
| 4-worker audit, 2 fast FAILs | wait for slowest | cancel on consensus | -3 to -8 min wall, -$0.50 to -$2.00 |
| Worker observability | re-scan artifacts | read `parallel_workers.workers[]` | O(1) lookup |

### Smoke-test results

- `bash scripts/cycle-state-test.sh` → 23/23 PASS (was 19, added 4)
- `bash scripts/fanout-dispatch-test.sh` → 22/22 PASS (was 16, added 6)
- `bash scripts/dispatch-parallel-test.sh` → 13/13 PASS (was 10, added 3)
- `bash scripts/utility/run-all-regression-tests.sh` → all suites pass

### Out of scope (deferred to future cycles)

- **Task A (declarative `fanout_tiers`)**: cycle 25's research already produced `apply-cycle-25-changes.sh`; operator can apply when ready.
- **Cancel-on-consensus for non-audit phases**: only verdict-mode merge has binary FAIL/PASS. plan_review's REVISE/PROCEED/ABORT and concat modes need different consensus heuristics.
- **Cache-prefix at >30K tokens**: today's prefix is ~50 lines; Anthropic supports up to 200K. Larger prefixes need profiling first.
- **Parent Claude Code sandbox file-write cascade fix** (the bug that blocked cycle 25's builder from writing to `scripts/`): file as v8.24.0 candidate.

---

## [8.22.0] - 2026-05-05

Architectural overhaul of the adaptive-failure system. Closes 6 structural defects exposed when v8.21.0's worktree fix didn't unblock `/loop` invocations from inside Claude Code: the actual blocker was nested sandbox-exec EPERM at sub-claude *startup* (orthogonal to the worktree fix at the *builder* phase), and the orchestrator's "3+ failures of any kind → BLOCKED-SYSTEMIC" prompt rule conflated transient infra issues with code-quality evidence — leaving the loop permanently stuck after a few EPERM retries.

### Architectural invariant

> **Failure adaptation is a deterministic kernel function, not a prompt rule.** Given a structured failure history with retention policy and a richer classification taxonomy, `scripts/failure/failure-adapter.sh` computes the next action — `PROCEED | RETRY-WITH-FALLBACK | BLOCK-CODE | BLOCK-OPERATOR-ACTION` — which the orchestrator consumes verbatim. Same input → same output, unit-testable, phase-gate-enforceable.

### Added

- `scripts/dispatch/detect-nested-claude.sh` — single-purpose probe for `CLAUDECODE` / `CLAUDE_CODE_*` env-var family. Returns `nested` or `standalone`.
- `scripts/dispatch/evolve-loop-dispatch.sh` auto-detection: when nested-claude is detected, auto-enables `EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1` with WARN log. Defense-in-depth alongside `SKILL.md`'s slash-command auto-set — direct CLI dispatcher invocations now also work.
- `scripts/failure/failure-classifications.sh` (sourced library) — 7-value structured classification taxonomy with per-class age-out windows + severity tier + retry policy. Helpers: `failure_age_out_seconds`, `failure_severity_of`, `failure_retry_policy`, `failure_normalize_legacy`, `failure_compute_expires_at`.
- `scripts/failure/failure-adapter.sh decide` — deterministic decision kernel emitting JSON `{action, reason, remediation, set_env, skip_phases, verdict_for_block, evidence}`. Reads non-expired failedApproaches, applies 7 priority-ordered rules, returns the canonical action for the orchestrator to follow verbatim.
- `scripts/failure-adapter-test.sh` — 12 unit tests covering each decision rule + edge cases (expired entries, legacy null-classification, priority ordering).
- `scripts/failure/state-prune.sh` — operator utility: `--classification`, `--age <duration>`, `--cycle <N>`, `--all --yes`, `--dry-run`. Atomic mv-of-temp.
- `scripts/state-prune-test.sh` — 7 unit tests covering each mode + edge cases (refuse `--all` without `--yes`, missing state file, dry-run preserves SHA).
- `scripts/lifecycle/cycle-state.sh prune-expired-failures` — programmatic auto-aging-out subcommand. Called automatically by failure-adapter and the dispatcher's record_failed_approach.

### Changed

- `scripts/cli_adapters/claude.sh` — un-deprecates `EVOLVE_SANDBOX_FALLBACK_ON_EPERM`. The v8.21 deprecation was scope-mismatched (worktree fix targets builder writes; flag targets sub-claude startup — orthogonal layers). Updated WARN messaging to reflect the dual-context model.
- `scripts/failure/record-failure-to-state.sh` — emits structured classification + `expiresAt` timestamp + FIFO cap (50 entries). Backward-compat: existing `verdict` field preserved.
- `scripts/dispatch/evolve-loop-dispatch.sh:record_failed_approach` — same: classification + expiresAt + FIFO cap. Legacy classifications (`infrastructure`, `audit-fail`, `build-fail`) auto-mapped to v8.22 taxonomy via `failure_normalize_legacy`.
- `scripts/dispatch/run-cycle.sh` — `build_context` invokes failure-adapter and injects `adaptiveFailureDecision` JSON into the orchestrator's context block. Also filters `recentFailures` by `expiresAt` at read time (read-side defense in depth). New `honor_adapter_set_env` exports the adapter's `set_env` directives before spawning the orchestrator.
- `agents/evolve-orchestrator.md` — replaced the markdown decision table with a single rule: "read the adapter JSON's `action` field and follow it verbatim." Added "Operator Action Required block template" section. ~50 lines simplified to ~30.
- `.evolve/profiles/orchestrator.json` — added `Bash(failure-adapter.sh decide:*)` to allowed_tools.
- `CLAUDE.md` — un-deprecated `EVOLVE_SANDBOX_FALLBACK_ON_EPERM` in rule #6 with dual-context explanation. Added "Failure Adaptation Kernel (v8.22.0+)" subsection covering taxonomy, retention, decision rules, and operator utilities.

### Reverted

- v8.21's deprecation of `EVOLVE_SANDBOX_FALLBACK_ON_EPERM`. The flag is now permanent — required for nested-claude (the primary use case for `/evo:loop`). Future removal would require a non-sandbox-based isolation primitive on Darwin.

### Fixed

- **DEFECT-1 (CRITICAL)**: `EVOLVE_SANDBOX_FALLBACK_ON_EPERM` deprecated prematurely in v8.21. Now un-deprecated with auto-enable for nested-claude.
- **DEFECT-2 (HIGH)**: Nested-sandbox auto-detection only in SKILL.md. Direct dispatcher invocations bypassed the auto-set. Now `evolve-loop-dispatch.sh` also auto-detects via `detect-nested-claude.sh`.
- **DEFECT-3 (HIGH)**: Failure classification was free-form. Now a structured 7-value enum with per-class metadata.
- **DEFECT-4 (HIGH)**: `failedApproaches` had no retention policy. Now: per-classification age-out + FIFO cap of 50. Read-side filter at `run-cycle.sh:build_context` excludes expired entries from orchestrator context.
- **DEFECT-5 (HIGH)**: Blocking decision was prompt-only ("3+ of any kind → BLOCKED-SYSTEMIC"). Now deterministic shell + jq, separately scoring code and infrastructure failures, with priority-ordered rules and unit tests.
- **DEFECT-6 (MEDIUM)**: No operator-action surface. New "Operator Action Required" block template; `state-prune.sh` provides the canonical recovery utility.

### Migration

Pre-v8.22 entries with free-form `classification` ("infrastructure", "audit-fail", "build-fail") or `verdict` ("FAIL", "BLOCKED-SYSTEMIC") are auto-mapped to the v8.22 taxonomy on read by `failure_normalize_legacy`. No manual migration required. Entries lacking `expiresAt` are kept indefinitely (no false-prune of legacy data) until you choose to remove them via `state-prune.sh`.

---

## [8.21.1] - 2026-05-05

Test infrastructure cleanup. Closes 3 long-standing pre-existing test failures uncovered during v8.21.0's regression posture review. Zero production-code changes — only test fixtures, drift classification, and one profile entry.

### Fixed

- `orchestrator-sandbox-coverage-test.sh` Test 2: `.evolve/release-journal` and `.evolve/worktrees` were on disk but unclassified in `orchestrator.json:sandbox`. Both added to `deny_subpaths` — the orchestrator must NOT write to release journals (release-pipeline.sh's privilege) or worktree parents (run-cycle.sh's privilege).
- `release-pipeline-test.sh` Tests 2, 4, 5, 6, 7, 8, 9, 10: `make_stub_repo`'s ledger fixture used the pre-v8.14.0 schema (`{timestamp, agent, artifact_sha}`) but `preflight.sh` reads `{ts, role, artifact_sha256}`. Updated fixture to match production schema. **8 cascading test failures resolved by a one-line fix.**
- `release/preflight-test.sh` Tests 4, 8: same schema mismatch as above. Same one-line fix.

### Notes

This release does not change the published evolve-loop runtime behavior. The fixes restore the regression-suite truth signal so future contributions can rely on `bash scripts/utility/run-all-regression-tests.sh` showing 27/27 PASS instead of `24/27 PASS, 3 known-flaky`.

---

## [8.21.0] - 2026-05-05

Architectural fix release. Closes the v8.13.x — v8.20.2 worktree-provisioning gap that caused recurring builder EPERM on macOS Darwin 25.4 and required the deprecated `EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1` workaround. Restores Anthropic's recommended kernel-sandbox enforcement by default.

### Added

- `scripts/dispatch/run-cycle.sh` provisions a per-cycle git worktree at `$EVOLVE_PROJECT_ROOT/.evolve/worktrees/cycle-N` (branch `evolve/cycle-N`) **before** spawning the orchestrator. Privileged-shell context — structurally outside any LLM agent's reach. EXIT trap tears down both worktree and cycle-state on every exit path.
- `scripts/lifecycle/cycle-state.sh set-worktree <path>` — atomic update of `active_worktree` without phase change. Privileged-shell-only (denied in orchestrator profile).
- `scripts/cycle-state-test.sh` — Tests 16-18 cover set-worktree (fresh, idempotent overwrite, init-required). 19 tests total, all passing.
- `scripts/guards/phase-gate-precondition.sh` — kernel hook now denies the in-process `Agent` tool whenever `cycle-state.json` exists. Defense-in-depth alongside the orchestrator profile deny.

### Changed

- `scripts/cli_adapters/claude.sh` — empty `WORKTREE_PATH` now fails loudly at BOTH substitution sites (was silent skip in `sandbox.write_subpaths`, loud exit in `add_dir`). The silent-skip path masked the worktree-provisioning gap for multiple releases.
- `scripts/dispatch/subagent-run.sh` — `WORKTREE_PATH` is now derived from `cycle-state.json` by default; env var becomes a fallback for tests/manual invocations.
- `.evolve/profiles/orchestrator.json` — `disallowed_tools` adds `Agent`, `Bash(cycle-state.sh set-worktree:*)`, and `Bash(cycle-state.sh set-worktree)`.
- `agents/evolve-orchestrator.md` — phase-advance no longer carries a worktree placeholder; new "Worktree contract" section documents the privileged-shell provisioning model.
- `scripts/dispatch/evolve-loop-dispatch.sh` — loop body pins cwd to `$EVOLVE_PROJECT_ROOT` and re-validates `RUN_CYCLE` is executable per iteration, hardening against subagent-induced cwd drift and plugin-update mid-batch.
- `CLAUDE.md` — Rule #5 (Agent tool) updated to document the structural enforcement; Rule #6 (sandbox) marks `EVOLVE_SANDBOX_FALLBACK_ON_EPERM` deprecated; new "Worktree Provisioning Contract" subsection.

### Deprecated

- `EVOLVE_SANDBOX_FALLBACK_ON_EPERM` — emits loud WARN logs whenever it engages in v8.21+. Scheduled for removal in v8.22.0. Its purpose is obviated by the v8.21.0 worktree provisioning fix; if it still fires, it indicates a regression that should be filed as an issue, not silently bypassed.

### Fixed

- **BUG-001 (CRITICAL)**: builder writes EPERM because no component provisioned the build worktree. Orchestrator profile correctly denied `git worktree add` (trust boundary), but no privileged shell context filled the gap. cycle-state.active_worktree stayed `null`, builder's sandbox profile expanded `{worktree_path}` to empty, all source writes denied.
- **BUG-002 (HIGH)**: claude.sh's silent `continue` when `WORKTREE_PATH` is unset (line 259) hid BUG-001 across releases. Both substitution sites now error loudly.
- **BUG-003 (HIGH)**: `EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1` silently disabled the OS-level sandbox. Now emits a deprecation warning every time it engages.
- **BUG-004 (HIGH)**: orchestrator could bypass `subagent-run.sh` via the in-process `Agent` tool, leaving ledger gaps (e.g., scout-report.md present but no scout ledger entry). Closed at two layers: profile deny + kernel hook.
- **BUG-005 (MEDIUM)**: dispatcher loop body did not pin cwd between iterations. Subagent-induced cwd drift could cause cycle N+1 to fail with rc=127 ("command not found"). Now pins cwd and re-validates `RUN_CYCLE` per iteration.

---

## [8.20.2] - 2026-05-05

_Generated by `scripts/release/changelog-gen.sh` from commits v8.18.0..HEAD._

### Added

- PATH-based kernel script invocation (v8.20.0)
- /evo:loop auto-enables intent capture (v8.19.1)
- add evolve-intent skill v0.1 (pre-Scout intent capture phase, opt-in)

### Fixed

- escape backticks in Test 18 pass-message (was executing dispatcher)
- cwd-independent dispatcher resolution (v8.20.2)
- auto-set EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1 for /evo:loop (v8.20.1)
- add explicit absolute-path Bash patterns for plugin install layouts (v8.19.5)
- switch permission_mode to bypassPermissions for autonomous execution (v8.19.4)
- orchestrator allowlist matches plugin-install absolute paths (v8.19.3)
- replace tautological Test 24 with code-presence regression guard
- make Test 24 actually exercise the auth-check path
- subscription auth is the primary path (v8.19.2)
- address cycle 28 audit findings (MEDIUM-1, LOW-3)
- pre-flight cwd + ANTHROPIC_API_KEY validation


---

## [8.20.1] - 2026-05-05

_Generated by `scripts/release/changelog-gen.sh` from commits v8.18.0..HEAD._

### Added

- PATH-based kernel script invocation (v8.20.0)
- /evo:loop auto-enables intent capture (v8.19.1)
- add evolve-intent skill v0.1 (pre-Scout intent capture phase, opt-in)

### Fixed

- auto-set EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1 for /evo:loop (v8.20.1)
- add explicit absolute-path Bash patterns for plugin install layouts (v8.19.5)
- switch permission_mode to bypassPermissions for autonomous execution (v8.19.4)
- orchestrator allowlist matches plugin-install absolute paths (v8.19.3)
- replace tautological Test 24 with code-presence regression guard
- make Test 24 actually exercise the auth-check path
- subscription auth is the primary path (v8.19.2)
- address cycle 28 audit findings (MEDIUM-1, LOW-3)
- pre-flight cwd + ANTHROPIC_API_KEY validation


---

## [8.20.0] - 2026-05-05

_Generated by `scripts/release/changelog-gen.sh` from commits v8.18.0..HEAD._

### Added

- PATH-based kernel script invocation (v8.20.0)
- /evo:loop auto-enables intent capture (v8.19.1)
- add evolve-intent skill v0.1 (pre-Scout intent capture phase, opt-in)

### Fixed

- add explicit absolute-path Bash patterns for plugin install layouts (v8.19.5)
- switch permission_mode to bypassPermissions for autonomous execution (v8.19.4)
- orchestrator allowlist matches plugin-install absolute paths (v8.19.3)
- replace tautological Test 24 with code-presence regression guard
- make Test 24 actually exercise the auth-check path
- subscription auth is the primary path (v8.19.2)
- address cycle 28 audit findings (MEDIUM-1, LOW-3)
- pre-flight cwd + ANTHROPIC_API_KEY validation


---

## [8.19.5] - 2026-05-04

_Generated by `scripts/release/changelog-gen.sh` from commits v8.18.0..HEAD._

### Added

- /evo:loop auto-enables intent capture (v8.19.1)
- add evolve-intent skill v0.1 (pre-Scout intent capture phase, opt-in)

### Fixed

- add explicit absolute-path Bash patterns for plugin install layouts (v8.19.5)
- switch permission_mode to bypassPermissions for autonomous execution (v8.19.4)
- orchestrator allowlist matches plugin-install absolute paths (v8.19.3)
- replace tautological Test 24 with code-presence regression guard
- make Test 24 actually exercise the auth-check path
- subscription auth is the primary path (v8.19.2)
- address cycle 28 audit findings (MEDIUM-1, LOW-3)
- pre-flight cwd + ANTHROPIC_API_KEY validation


---

## [8.19.4] - 2026-05-04

_Generated by `scripts/release/changelog-gen.sh` from commits v8.18.0..HEAD._

### Added

- /evo:loop auto-enables intent capture (v8.19.1)
- add evolve-intent skill v0.1 (pre-Scout intent capture phase, opt-in)

### Fixed

- switch permission_mode to bypassPermissions for autonomous execution (v8.19.4)
- orchestrator allowlist matches plugin-install absolute paths (v8.19.3)
- replace tautological Test 24 with code-presence regression guard
- make Test 24 actually exercise the auth-check path
- subscription auth is the primary path (v8.19.2)
- address cycle 28 audit findings (MEDIUM-1, LOW-3)
- pre-flight cwd + ANTHROPIC_API_KEY validation


---

## [8.19.3] - 2026-05-04

_Generated by `scripts/release/changelog-gen.sh` from commits v8.18.0..HEAD._

### Added

- /evo:loop auto-enables intent capture (v8.19.1)
- add evolve-intent skill v0.1 (pre-Scout intent capture phase, opt-in)

### Fixed

- orchestrator allowlist matches plugin-install absolute paths (v8.19.3)
- replace tautological Test 24 with code-presence regression guard
- make Test 24 actually exercise the auth-check path
- subscription auth is the primary path (v8.19.2)
- address cycle 28 audit findings (MEDIUM-1, LOW-3)
- pre-flight cwd + ANTHROPIC_API_KEY validation


---

## [8.19.2] - 2026-05-04

_Generated by `scripts/release/changelog-gen.sh` from commits v8.18.0..HEAD._

### Added

- /evo:loop auto-enables intent capture (v8.19.1)
- add evolve-intent skill v0.1 (pre-Scout intent capture phase, opt-in)

### Fixed

- replace tautological Test 24 with code-presence regression guard
- make Test 24 actually exercise the auth-check path
- subscription auth is the primary path (v8.19.2)
- address cycle 28 audit findings (MEDIUM-1, LOW-3)
- pre-flight cwd + ANTHROPIC_API_KEY validation


---

## [8.19.1] - 2026-05-04

_Generated by `scripts/release/changelog-gen.sh` from commits v8.18.0..HEAD._

### Added

- /evo:loop auto-enables intent capture (v8.19.1)
- add evolve-intent skill v0.1 (pre-Scout intent capture phase, opt-in)

### Fixed

- address cycle 28 audit findings (MEDIUM-1, LOW-3)
- pre-flight cwd + ANTHROPIC_API_KEY validation


---

## [8.19.0] - 2026-05-04

_Generated by `scripts/release/changelog-gen.sh` from commits v8.18.0..HEAD._

### Added

- add evolve-intent skill v0.1 (pre-Scout intent capture phase, opt-in)

### Fixed

- address cycle 28 audit findings (MEDIUM-1, LOW-3)
- pre-flight cwd + ANTHROPIC_API_KEY validation


---

## [8.18.1] - 2026-05-03

_Generated by `scripts/release/changelog-gen.sh` from commits v8.18.0..HEAD._

### Fixed

- pre-flight cwd + ANTHROPIC_API_KEY validation


---

## [8.18.0] - 2026-05-03

_Generated by `scripts/release/changelog-gen.sh` from commits v8.17.0..HEAD._

### Added

- surface cache TTL bucket split in show-cycle-cost

### Fixed

- address code review findings on dual-root fix
- dual-root path resolution for plugin-mode invocation


---

## [8.17.0] - 2026-05-02

_Generated by `scripts/release/changelog-gen.sh` from commits v8.16.0..HEAD._

### Added

- close the evolution loop — adapter fallback, cycle-counter advance, orchestrator adaptation
- evolutionary failure handling — learn from infrastructure failures, continue cycles

### Fixed

- propagate EVOLVE_SANDBOX_FALLBACK_ON_EPERM through nested bash + sandbox-exec layers


---

## [8.16.0] - 2026-05-02

_Generated by `scripts/release/changelog-gen.sh` from commits v8.15.0..HEAD._

### Added

- tri-layer architecture with fan-out, plan-review, and composable skill catalog
- team-context.md shared bus + phase-gate hook
- TDD-engineer agent + tdd phase between Scout and Builder
- persona fields + harness fixes (cycle 16)

### Fixed

- preflight accepts heading-form Verdict like ship.sh
- accept heading-form Verdict in audit-report.md


---

## [8.15.0] - 2026-04-30

_Generated by `scripts/release/changelog-gen.sh` from commits v8.9.1..HEAD._

### Added

- hybrid Gemini adapter + CLI detection helper (Tier 1)
- cross-platform skill content portability (Tier 0)
- strict-mode dispatcher for /evo:loop (cycle 8214)
- per-cycle telemetry + run-all helper [v8.13.6 cycle 3/3, batch complete]
- declarative budget_tiers via EVOLVE_TASK_MODE [v8.13.5 cycle 2/3]
- EVOLVE_MAX_BUDGET_USD per-invocation override [v8.13.4 cycle 1/3]
- /insights report improvements — probe-tool, postedit-validate hook, /publish + /verify-release skills [v8.13.3]
- self-healing release pipeline with auto-rollback [v8.13.2]
- complete trust-boundary architecture with role-gate + phase-gate-precondition + run-cycle.sh [v8.13.1]
- atomic ship-gate via canonical ship.sh allowlist [v8.13.0]
- token economy + telemetry + lightweight failure recording [v8.12.3]
- failed-cycle retrospective + lessons-learned pipeline [v8.12.2]
- subagent subprocess isolation hardening [v8.12.0]
- implement Divergent-Convergent Scout Loop [cycle 12]
- implement Skill Crystallization in Phase 6 [cycle 6]
- implement autoresearch strategy and address always-pass paradox
- generalize evolve-loop for Gemini CLI and platform-agnostic execution
- v8.10.0 — ecc:e2e integration, skill inventory setup, phase renumber

### Fixed

- preflight.sh ledger schema mismatch + verdict regex
- orchestrator sandbox + dispatcher cleanup (cycle 8215, rc4)
- macOS sandbox missing /tmp read rules caused 5-cycle EPERM bug [v8.12.4]
- claude.sh adapter latent bugs blocking subagent invocation [v8.12.1]
- v8.10.3 — remove disable-model-invocation flag blocking /evo:loop
- v8.10.2 — release.sh cache refresh now works post-push
- v8.10.1 — per-cycle context budget gate (was cumulative, stopped at cycle 4)

### Changed

- enforce execution-based evals by blocking tautological checks [cycle 15]
- implement Checkpoint-Based Handshake [cycle 16]
- implement Meta-Review of Mutation Tests in Phase 7 [cycle 15]
- implement Pydantic Schema Validation in Skill Inventory [cycle 14]
- implement Mutation Testing in Auditor [cycle 13]
- implement Tamper-Proof Ledger Chaining [cycle 11]
- refactor evolve-loop into Agent-Native directory structure [cycle 10]
- implement Agent Metadata Linter and fix frontmatter [cycle 9]
- implement Scoped Context Dispatcher [cycle 8]
- implement Step-Level Reflection Loops in Builder [cycle 7]
- implement Mandatory Reproduction Scripts and Blueprint-First in Builder [cycle 4]
- implement Contextual Distillation in Phase 6 [cycle 3]
- implement Reasoning Asymmetry for Planner-Auditor [cycle 2]
- implement State Manifest and sync schema hygiene [cycle 1]

### Documentation

- add incident report for Flawless Execution Anomaly and update safety guidelines

### Other

- (polish) address review findings before v8.15.0 release
- Revert "stability: implement Reasoning Asymmetry for Planner-Auditor [cycle 2]"
- Revert "performance: implement Contextual Distillation in Phase 6 [cycle 3]"
- Revert "stability: implement Mandatory Reproduction Scripts and Blueprint-First in Builder [cycle 4]"
- Revert "meta: implement AVO and perform first meta-cycle optimization [cycle 5]"
- Revert "feature: implement Skill Crystallization in Phase 6 [cycle 6]"
- Revert "stability: implement Step-Level Reflection Loops in Builder [cycle 7]"
- Revert "performance: implement Scoped Context Dispatcher [cycle 8]"
- Revert "techdebt: refactor evolve-loop into Agent-Native directory structure [cycle 10]"
- Revert "stability: implement Tamper-Proof Ledger Chaining [cycle 11]"
- Revert "feature: implement Divergent-Convergent Scout Loop [cycle 12]"
- Revert "stability: implement Mutation Testing in Auditor [cycle 13]"
- Revert "techdebt: implement Meta-Review of Mutation Tests in Phase 7 [cycle 15]"
- Revert "performance: implement Checkpoint-Based Handshake [cycle 16]"


---

## [8.14.0] - 2026-04-29

_Generated by `scripts/release/changelog-gen.sh` from commits 63442db..HEAD._

### Added

- strict-mode dispatcher for /evo:loop (cycle 8214)

### Fixed

- preflight.sh ledger schema mismatch + verdict regex
- orchestrator sandbox + dispatcher cleanup (cycle 8215, rc4)


---

## [8.13.6] - 2026-04-28

### Cycle 3 of 3 in the /evo:loop token-optimization batch (closing the loop)

Final cycle bundles the originally-planned per-phase telemetry with two process improvements identified by cycle 8211's audit (LOW-2: auditor profile blocks parallel-Bash patterns; LOW-3: rate-limit handling). Bundling avoids 3 separate ships for closely-related operational improvements.

### Added

- **`scripts/observability/show-cycle-cost.sh`** (~165 lines) — per-cycle cost telemetry. Reads each subagent's `<workspace>/<agent>-stdout.log` (existing v8.12.x data — no new instrumentation) and prints a per-phase breakdown of `total_cost_usd`, `cache_read_input_tokens`, `cache_creation_input_tokens`, and output tokens. Human-readable table by default; `--json` for scripting. Closes the question "this cycle cost X — where did it go?" with one command. 8 unit tests.
- **`scripts/utility/run-all-regression-tests.sh`** (~140 lines) — single-command runner for all 14 regression suites. Cycle 8211 audit identified that the auditor profile allowlists individual `Bash(bash scripts/<test>.sh:*)` entries but NOT compositions (`bash a & bash b & wait`, `for s in ...; do bash $s; done`). This helper is one allowlisted command that runs them all. Sequential or `--parallel`. `--json` for machine-readable summary. `SUITES_OVERRIDE` env var lets tests inject stubs without rewriting the script. 7 unit tests.
- **8 new tests in `scripts/show-cycle-cost-test.sh`** — covering: missing args, non-int cycle, missing workspace, empty workspace, single-phase render, multi-phase totals (numeric comparison via bc), JSON shape, malformed-log graceful fallback.
- **7 new tests in `scripts/run-all-regression-tests-test.sh`** — covering: all-pass, mixed pass/fail with diagnostic tail, JSON shape, parallel mode, bad flag, --help, single-suite SUITES_OVERRIDE.

### Changed

- **`.evolve/profiles/auditor.json`** — added 4 new allowlist entries: `scripts/utility/run-all-regression-tests.sh` (and its test), `scripts/observability/show-cycle-cost.sh` (and its test). Future audit cycles can run all 14 suites via one allowlisted command instead of needing 14 separate entries OR `&`-chained patterns the profile rejects. Closes cycle 8211 audit's LOW-2.

### What this DOES NOT do

- Does NOT add 429 detection to `subagent-run.sh` (cycle 8211 audit's LOW-3). Deferred — the right shape is a graceful retry-after-N-minutes signal, which needs more design than this cycle has scope for. Tracked for v8.13.7+.
- Does NOT change ship-gate, role-gate, phase-gate-precondition, audit-binding, or any v8.13.x trust-boundary primitive.
- Does NOT add tier adoption to other profiles (still incremental from v8.13.5).

### Test Results

- `scripts/show-cycle-cost-test.sh` — 8/8 PASS
- `scripts/run-all-regression-tests-test.sh` — 7/7 PASS
- `scripts/utility/run-all-regression-tests.sh` (full 14-suite dogfood) — 14/14 PASS, 53 seconds
- All v8.13.x regression suites — 162/162 PASS (no regression)
- **Total: 177/177 PASS** (162 baseline + 8 telemetry + 7 run-all).

### /evo:loop token-optimization batch summary (v8.13.4 + v8.13.5 + v8.13.6)

| Cycle | Ship | Mechanism | Lines |
|-------|------|-----------|-------|
| 8210 | v8.13.4 | `EVOLVE_MAX_BUDGET_USD` per-invocation override | 17 + 7 tests |
| 8211 | v8.13.5 | `EVOLVE_TASK_MODE` declarative budget tiers | 14 + 5 tests |
| 8212 | v8.13.6 | `show-cycle-cost.sh` + `run-all-regression-tests.sh` | 305 + 15 tests |

Combined effect:
- Operators can size budget per-invocation (override), per-workload (tier), or measure post-hoc (telemetry).
- Auditors can run all 14 regression suites with one allowlisted command.
- Future cycles can answer "where did $X go?" quantitatively via show-cycle-cost.
- Foundation laid for v8.14.x's `task_budget` integration (when Claude Code adds CLI support — currently API-only per Anthropic docs).

### Future

- v8.13.7 — 429 rate-limit detection + retry semantics in `subagent-run.sh` (LOW-3 from cycle 8211 audit).
- v8.14.x — Anthropic `task_budget` integration as 4th precedence tier; AgentDiet trajectory reduction for Builder worktree sessions; 1h cache TTL via API-direct path.

---

## [8.13.5] - 2026-04-28

### Cycle 2 of 3 in the /evo:loop token-optimization batch

Builds on v8.13.4's per-invocation override (`EVOLVE_MAX_BUDGET_USD`). v8.13.5 adds the declarative companion: profile-level `budget_tiers` selected via `EVOLVE_TASK_MODE`. Tiers solve "agent X has structurally different workloads" (Scout codebase-scan vs Scout research-heavy); per-invocation override solves "I'm doing something unusual just this once."

### Added

- **`budget_tiers` field in subagent profiles** — optional map of mode-name → budget value. Profiles without the field behave identically to v8.13.4. The Scout profile (`.evolve/profiles/scout.json`) ships with three tiers as the canonical example: `default` ($0.50), `research` ($1.50), `deep` ($2.50). Other profiles can adopt incrementally.
- **`EVOLVE_TASK_MODE` env var** — selects a tier. When set + matching key in profile's `budget_tiers` → tier value used (logged loudly). When set without matching key (typo, or profile has no `budget_tiers`) → WARN + profile default. When unset → no-op.
- **Precedence chain** (highest first): `EVOLVE_MAX_BUDGET_USD` > `EVOLVE_TASK_MODE`-resolved tier > `max_budget_usd` profile default. All three coexist; later mechanisms can override earlier resolution; every step logs to stderr for auditability.
- **5 new tests in `scripts/claude-adapter-test.sh`** (Tests 8-12): tier resolution + applied, missing tier key → WARN, no `budget_tiers` in profile → WARN, override > tier > default precedence chain, explicit `default` tier resolves.

### Changed

- **`scripts/cli_adapters/claude.sh`** — added 14-LOC tier-resolution block between profile parse and the v8.13.4 override block. jq query reads `.budget_tiers[$mode]`.
- **`.evolve/profiles/scout.json`** — added `budget_tiers` map with `default`/`research`/`deep` tiers + an inline `_budget_tiers_doc` field documenting the rationale (cycle 8210's $0.51 burn).
- **`CLAUDE.md` "Subagent Budget Controls" section** — restructured to show the 3-mechanism precedence chain explicitly. Documents when to use override vs tier vs profile-default, with examples.

### Test Results

- `scripts/claude-adapter-test.sh` — 12/12 PASS (was 7 in v8.13.4; +5 for tiers).
- All v8.13.x baseline regression suites — 150/150 PASS, no regression.
- **Total: 162/162 PASS**.

### Why this is the right v8.13.5

The v8.13.4 env-var override was a tactical fix for cycle 8210's friction. v8.13.5 promotes the same mechanism into a declarative shape — the profile encodes "this agent has these legitimate workload classes" and the env var selects among them. The structured form is testable, version-controllable, and discoverable (a new operator can `cat .evolve/profiles/scout.json` and see the tiers).

The override remains the right tool for one-offs the tiers don't predict; the tiers are the right tool for routine workload classes. Both ship; both are tested.

### What this DOES NOT do

- Does NOT change the ship-gate, role-gate, phase-gate-precondition, or audit-binding contract.
- Does NOT auto-detect task mode from prompt content (deferred — speculative).
- Does NOT add tiers to Builder/Auditor/etc. profiles (incremental adoption — v8.13.6 or later).

### Future

- v8.13.6 (cycle 8212) — per-phase telemetry: extend the existing subagent-run.sh ledger model_usage breakdown into a queryable structure that can correlate $$ spend with task mode (so operators can answer "do my `research` invocations actually need $1.50, or could it be $1.00?" with data).
- v8.14.x — Anthropic `task_budget` integration when Claude Code adds CLI support; would slot in as a fourth precedence tier (model-self-pacing inside the operator's hard cap).

---

## [8.13.4] - 2026-04-28

### Cycle 1 of 3 in the /evo:loop token-optimization batch

The user invoked `/evo:evolve-loop 3 online search to learn what we can do to optimize the token usage in the meantime to maximize the accuracy and performance.` This is the first of three planned cycles. Online research findings + the highest-leverage immediate optimization ship together as v8.13.4.

### Research findings (Scout phase, full report in `.evolve/runs/cycle-8210/scout-report.md`)

Four searches across [Anthropic prompt caching docs](https://platform.claude.com/docs/en/build-with-claude/prompt-caching), [Claude Agent SDK](https://platform.claude.com/docs/en/agent-sdk/cost-tracking), [task_budget](https://platform.claude.com/docs/en/build-with-claude/task-budgets), and [arXiv 2509.23586 — AgentDiet](https://arxiv.org/abs/2509.23586). Key conclusion:

- **`task_budget` (Anthropic's official self-pacing API) is NOT yet available via Claude Code CLI** — only via direct Messages API on Opus 4.7 beta. evolve-loop's existing `claude -p` adapter cannot use it. **Tracked as v8.14.x backlog.**
- **AgentDiet trajectory reduction (40-60% input savings)** applies to multi-turn agents; evolve-loop's Scout/Builder/Auditor are single-shot subagents. Builder operating in a worktree DOES accumulate trajectory, but implementation is multi-day. **Tracked as v8.14.x backlog.**
- **Server-side compaction** (Opus 4.7/4.6, Sonnet 4.6 beta) auto-applies — evolve-loop already benefits transparently.
- **Cycle 8210's own Scout invocation hit `error_max_budget_usd` at $0.51** during research with 8 web searches and no completed report. The static `max_budget_usd: 0.50` in `scout.json` is sized for codebase-scan, not research-heavy mode. This is the empirically-demonstrated highest-leverage v8.13.4 optimization.

### Added

- **`EVOLVE_MAX_BUDGET_USD` per-invocation override** (`scripts/cli_adapters/claude.sh`) — operator can bump the budget for one subagent invocation without permanently raising the profile baseline. Logs the deviation loudly to stderr for auditability. Empty/malformed/negative values are rejected (with WARN) — fallback to profile value.
- **`scripts/claude-adapter-test.sh`** — 7 unit tests covering: profile-default path, override picked up, integer override, decimal override, malformed (string) rejection, empty-string treated as unset, negative value rejection. New file specifically because the adapter previously had no dedicated test suite.

### Changed

- **`scripts/cli_adapters/claude.sh`** — added the override block (~17 LOC including comments + WARN path).
- **`CLAUDE.md`** — new "Subagent Budget Override (v8.13.4+)" section documents when to use it, when NOT to, and why it complements (doesn't replace) future task_budget integration.

### What this DOES NOT do

- Does NOT integrate Anthropic's `task_budget` (API-only; not in Claude Code yet).
- Does NOT change any subagent profile's default budget — operators decide per-invocation.
- Does NOT touch ship-gate, role-gate, phase-gate-precondition, audit-binding, or any v8.13.x trust-boundary primitive.

### Why this is the right v8.13.4

The user's goal was research → implementation. Research surfaced one clean operational fix that addresses a real friction (cycle 8210's Scout failure was the proof). Bigger optimizations (task_budget integration, AgentDiet) are tracked in the backlog but require Claude Code upstream changes or multi-day implementation. v8.13.4 delivers the one immediate win without scope-creep.

### Future cycles in this batch

- **v8.13.5** (cycle 8211): based on cycle-8210's research, will likely focus on **task-mode budget tiers** — dynamic per-task-type budget (scan vs research vs build) selected via env var. Cleaner than per-invocation override; layers on top of v8.13.4's foundation.
- **v8.13.6** (cycle 8212): per-phase telemetry — extending the existing subagent-run.sh ledger model_usage breakdown into a queryable structure, so operators can see "this cycle spent $X on Scout, $Y on Builder, $Z on Auditor" at a glance.

---

## [8.13.3] - 2026-04-27

### /insights report improvements

The `/insights` report (https://github.com/anthropics/claude-code) generated a structured analysis of friction patterns across 47 sessions / 370h / 229 commits. v8.13.3 ships the actionable improvements from that report. The "On the horizon" multi-day projects (parallel multi-agent evolve loops, autonomous bug hunter) are deferred to v8.14.x backlog.

### Added

- **`scripts/utility/probe-tool.sh`** (~110 lines) — canonical CLI-availability probe. Checks `command -v`, `type -P`, plus 5 common install locations (`/usr/local/bin/`, `/opt/homebrew/bin/`, `$HOME/.local/bin/`, `$HOME/bin/`, `/usr/bin/`). Emits `--json` for scripted use. Closes the "no gws command" false-negative class — the audit caught one real instance where Claude declared `gws` unavailable when it was installed at `~/.local/bin/`. 7-test unit suite at `scripts/probe-tool-test.sh`.
- **`scripts/verification/postedit-validate.sh`** (~115 lines) — PostToolUse hook on `Edit|Write` that syntax-checks the modified file by extension: `.json` → `jq empty`, `.sh` → `bash -n`, `.py` → `python3 -m py_compile`. Other extensions: silent no-op. Never blocks (PostToolUse can't); emits stderr WARN visible to LLM, prompting immediate re-edit. Catches the `bash 3.2` incompat class (`declare -A`, regex `\b` truncation) at edit time instead of cycle time. 11-test unit suite at `scripts/postedit-validate-test.sh`. Bypass: `EVOLVE_BYPASS_POSTEDIT_VALIDATE=1`.
- **`skills/publish/SKILL.md`** — slash command wrapper for `/publish`. Documents the v8.13.2 release-pipeline.sh entry point. Disambiguates "publish" from "push" with explicit vocabulary callouts.
- **`skills/verify-release/SKILL.md`** — slash command wrapper for `/verify-release`. Standalone post-publish marketplace-poll for diagnosing stale-plugin reports.
- **3 new memory files** for the user's auto-memory:
  - `feedback_probe_before_unavailable.md` — the gws-not-found pattern
  - `feedback_read_before_implement.md` — the imagined-API pattern (Builder worktrees calling functions that don't exist on target modules)
  - `feedback_bash_3_2_compat.md` — banned bash 4+ features + required idioms

### Changed

- **`CLAUDE.md`** — three new top-level sections:
  - "Verification Before Claiming Done" — three patterns (probe CLIs, read actual exports, report test counts) before declaring a task complete
  - "Shell & Environment Conventions" — bash 3.2 target, banned features, required idioms, SSE/streaming guidance
  - "Confirm Direction Before Multi-Cycle Work" — 3-bullet plan with success criteria for ambiguous multi-cycle requests, prevents the "25-cycle giant useless circle" failure mode
- **`.claude/settings.json`** — added PostToolUse hook on `Edit|Write` for postedit-validate.sh. Updated `_comment` documentation.
- **`.claude-plugin/plugin.json`** — added `./skills/publish/` and `./skills/verify-release/` to the skills array.

### Tests

- `scripts/probe-tool-test.sh` — 7 unit tests
- `scripts/postedit-validate-test.sh` — 11 unit tests
- All v8.13.x regression suites still pass (no edits to those files).

### /insights items addressed (mapped to action)

| /insights item | Action |
|---|---|
| CLAUDE.md addition: Release & Publish Workflow | Already done in v8.13.2 |
| CLAUDE.md addition: Verification Before Claiming Done | New section in CLAUDE.md |
| CLAUDE.md addition: Shell & Environment Conventions | New section in CLAUDE.md |
| Custom skill: `/publish` | `skills/publish/SKILL.md` wraps release-pipeline.sh |
| Custom skill: `/verify-release` | `skills/verify-release/SKILL.md` wraps marketplace-poll.sh |
| Hook: PostToolUse syntax check | `scripts/verification/postedit-validate.sh` + wired in settings.json |
| Pattern: confirm direction before exploring | New section in CLAUDE.md |
| Pattern: verify external state after publish | Already done by marketplace-poll.sh in v8.13.2 |
| Pattern: probe environment before assuming missing | `scripts/utility/probe-tool.sh` + memory `feedback_probe_before_unavailable.md` |
| On the horizon: parallel multi-agent evolve loops | DEFERRED to v8.14.x backlog (multi-day project) |
| On the horizon: autonomous bug hunter | DEFERRED to v8.14.x backlog (multi-day project) |
| On the horizon: self-healing release pipeline | Already shipped as v8.13.2 |

### v8.13.3 audit RC1 + RC2 follow-ups (incorporated before ship)

- **MEDIUM-1 fix** (`scripts/verification/postedit-validate.sh:33-43`): suppressed stderr from `mkdir -p` and `log()`'s `>> $GUARDS_LOG` when `.evolve/guards.log` is unwritable (auditor sandbox `read_only_repo: true`, any read-only CI). RC2's first attempt got the redirection order wrong (`>> file 2>/dev/null` opens before redirect activates → EPERM still leaks); RC3 corrects to `2>/dev/null >> file` (redirect first, then open). Test 11 of `postedit-validate-test.sh` now uses `chmod 0444` on an existing `guards.log` file (the actual auditor sandbox semantics — not `chmod -w` on a temp dir, which is a different syscall codepath that masked the RC2 bug). Same defensive pattern as ship-gate.sh and role-gate.sh. Audit cycle 8205 caught the RC2 mistake; cycle 8206 RC3 verified the fix.
- **LOW-1 cleanup**: removed dead `probe_path()` helper from `scripts/utility/probe-tool.sh` (defined but never called; loop inlined the same logic). No behavior change.
- **Builder process improvement (DEFECT-2 LOW)**: build reports must now run read-only-context tests against an actual `chmod 0444` on the existing log file, not `chmod -w` on a temp dir — different kernel codepaths produce different observable behavior. RC2's "verified empirically" claim was a process gap that v8.13.3 RC3's strengthened Test 11 now closes.

### Out of scope (deferred to v8.13.4 / v8.14.x)

- Parallel multi-agent evolve loops with tournament evaluator (worktree fan-out, scoring, auto-merge winner). Multi-day project; needs its own RC arc.
- Test-driven autonomous bug hunter (mine git history for `fix:` commits, generate property-based regression tests). Multi-day project; depends on `fix:` conformance which is currently ~20%.
- `/publish` flag for `--allow-dirty-bound` (skip preflight clean-tree check when audit-binding still matches). Would let the pipeline self-publish v8.13.4+. Currently the pipeline's preflight requires a clean tree; v8.13.3 ships via `ship.sh` directly.
- `shellcheck` integration in postedit-validate (catches more than `bash -n`, but adds a tool dependency). Add when shellcheck is a guaranteed dependency.

### Why this is the right v8.13.3

The `/insights` report ranks self-introduced bugs (10 instances) and ambiguous commands (6 instances) as the top friction classes. Every v8.13.3 component maps to one of these:

- **probe-tool.sh** + `feedback_probe_before_unavailable.md` — closes the "tool not found" false-negative pattern
- **postedit-validate.sh hook** — catches bash/json/python syntax errors at edit time (not cycle time), preventing the "declare -A" + "regex \b truncation" classes
- **CLAUDE.md "Read actual exports"** + `feedback_read_before_implement.md` — addresses the worktree imagined-API pattern
- **CLAUDE.md "Confirm direction"** — prevents the multi-cycle wrong-direction class (the force-graph "giant useless circle")
- **`/publish` + `/verify-release`** — codifies the v8.13.2 release pipeline as discoverable slash commands, addressing the "publish ambiguous" pattern

The architectural pattern stays consistent with v8.13.x: small composable scripts, allowlist-shaped permissions, tests-first, memory updates that compound over time.

---

## [8.13.2] - 2026-04-27

### Self-healing release pipeline

The /insights audit flagged "release_publish" as the #1 friction class — silent stale-marketplace failures, ambiguous "publish" semantics, cache-refresh ordering bugs, no rollback on failure. v8.13.2 introduces a single declarative entry point that owns the entire release lifecycle and auto-rolls-back on any post-push failure.

### Added

- **`scripts/release-pipeline.sh`** (~280 lines) — top-level orchestrator. Sequences pre-flight → bump → changelog → consistency-check → ship → marketplace-poll → cache-refresh. On any post-push failure, auto-invokes `rollback.sh` (deletes GitHub release + remote tag, creates revert commit). Writes a per-publish journal at `.evolve/release-journal/<version>-<ts>.json` for rollback to consult. Supports `--dry-run`, `--no-rollback`, `--skip-tests`, `--max-poll-wait-s`.
- **`scripts/release/preflight.sh`** (~190 lines) — pre-flight gate. Checks: clean working tree, branch attached, target-version is a valid semver bump, audit ledger has a recent (<7d) PASS verdict, all four gate-test suites green.
- **`scripts/release/version-bump.sh`** (~140 lines) — atomic version updater for `plugin.json`, `marketplace.json` (`.plugins[0].version` path), `SKILL.md` heading, `README.md` "Current" + history row. Idempotent.
- **`scripts/release/changelog-gen.sh`** (~150 lines) — conventional-commits parser. Buckets `feat:`/`fix:`/`refactor:`/`perf:`/`docs:` plus an explicit `### Other` fallback for the ~40% of historical commits without prefixes. Idempotent — preserves manually-curated entries.
- **`scripts/release/marketplace-poll.sh`** (~150 lines) — post-publish marketplace propagation verifier. Polls `~/.claude/plugins/marketplaces/evolve-loop/` for up to 5 minutes (configurable via `--max-wait-s`). On convergence, re-invokes `release.sh <target>` to refresh `installed_plugins.json` registry. **Closes the cache-refresh ordering bug** by sequencing release.sh-refresh AFTER convergence, never before.
- **`scripts/release/rollback.sh`** (~190 lines) — auto-revert. Reads release journal. Three independently-auditable steps: (a) `gh release delete vX.Y.Z`, (b) `git push origin :refs/tags/vX.Y.Z`, (c) revert commit pushed via `EVOLVE_BYPASS_SHIP_VERIFY=1 bash scripts/lifecycle/ship.sh "revert: ..."`. Logs every step to `.evolve/release-rollbacks.jsonl` for audit trail.
- **`docs/release/release-protocol.md`** (~250 lines) — canonical vocabulary doc. Defines push, tag, release, propagate, publish, ship. Operational runbook with annotated examples. Conventional-commits guide. Marketplace topology diagram. Common failure modes table.
- **5 new test suites** (54 tests total): `preflight-test.sh` (10), `changelog-gen-test.sh` (14), `marketplace-poll-test.sh` (10), `rollback-test.sh` (8), `release-pipeline-test.sh` (12). Includes explicit regression tests for **the cache-refresh ordering bug** (Test 9 of release-pipeline-test) and **the stale-version regression** (Test 3 of marketplace-poll-test, Test 10 of release-pipeline-test).

### Changed

- **`agents/evolve-orchestrator.md`** — verdict→PASS branch now invokes `release-pipeline.sh <new-version>` for version-bumping releases (vs. direct `ship.sh` for non-release commits).
- **`skills/evolve-loop/SKILL.md`** — Phase 5 description points at `release-pipeline.sh` as the canonical publish entry point. New v8.13.2 callout in the front-matter.
- **`.evolve/profiles/orchestrator.json`** — allows `Bash(bash scripts/release-pipeline.sh:*)` and the five `scripts/release/*.sh` components.
- **`CLAUDE.md`** — replaced "Release Checklist" section with "Release & Publish Workflow" pointing at the protocol doc and the pipeline.

### Documentation

- New `docs/release/release-protocol.md` is the canonical answer to "what does publish mean?"

### Test Results

131/131 pass: 54 new (preflight 10 + changelog-gen 14 + marketplace-poll 10 + rollback 8 + release-pipeline 12) + 77 v8.13.x regression suites (role-gate 21 + phase-gate-precondition 15 + guards 34 + ship-integration 7).

### Architectural pattern (consistent with v8.13.0/v8.13.1)

Every component:
- Has a `--dry-run` mode that mutates nothing.
- Is independently testable (each ships with its own unit-test suite).
- Composes through a thin top-level orchestrator (no logic duplication).
- Calls existing `ship.sh` for the actual atomic git ops (the v8.13.0 audit-binding contract is preserved unchanged).

### v8.13.2 audit RC1 follow-ups (incorporated before ship)

- **MEDIUM-1 fix**: `scripts/release/rollback.sh` now exits 1 (not 0) when any cleanup step (`gh release delete`, remote tag delete) explicitly fails, even if the revert commit succeeds. Pre-fix logic only consulted step3 — masking dangling-release incidents during partial-failure rollbacks. Regression test added (`rollback-test.sh` Test 9): simulates `gh release delete` returning non-zero, asserts exit 1 and a journal entry with `release_delete:"failed"`.
- **LOW-1**: preflight.sh `parse_semver` discards `[+-]suffix` (e.g., `1.0.0-rc1` → "1 0 0"). Project does not use pre-release versions; deferred. Documented as design choice.
- **LOW-2**: marketplace-poll.sh swallows `git fetch`/`git reset --hard` errors. Trade-off: retry-on-transient-failure semantics; deferred.

### Out of scope (deferred to v8.13.3 / v8.14.x)

- CDN-based marketplace propagation (current is git-based local).
- Cross-machine cache invalidation.
- Auto-incrementing semver from commit types.
- Pre-release / RC channels (`vX.Y.Z-rc1`) — preflight.sh semver fix needed if adopted.
- Slack/email notifications on rollback.

---

## [8.13.1] - 2026-04-27

### Trust boundary completed: role-gate + phase-gate-precondition + run-cycle.sh

v8.13.0 shipped the atomic ship-gate (`scripts/lifecycle/ship.sh` allowlist on Bash). v8.13.1 adds the two remaining structural gates and a declarative cycle driver, completing the trust-boundary the user originally requested ("orchestrator should only execute phases, can't interfere"):

- **`scripts/guards/role-gate.sh`** (NEW, ~190 lines) — PreToolUse hook on `Edit|Write`. Reads `.evolve/cycle-state.json` and enforces a per-phase write allowlist:
  - `calibrate`/`research`/`discover` → workspace only
  - `build` → workspace + active worktree
  - `audit` → audit-report.md + handoff-auditor.json
  - `ship` → version-bump files only (plugin.json, marketplace.json, CHANGELOG.md, README.md, SKILL.md)
  - `learn` → orchestrator-report.md + lessons/*.yaml + state.json

  Always allows `/tmp/`, `/var/folders/`, `$HOME/.claude/`. Transparent passthrough when no cycle is in progress. Bypass: `EVOLVE_BYPASS_ROLE_GATE=1`.

- **`scripts/guards/phase-gate-precondition.sh`** (NEW, ~150 lines) — Second PreToolUse hook on Bash matcher (after ship-gate). Triggers ONLY for `bash scripts/dispatch/subagent-run.sh <agent> ...` invocations. Reads cycle-state.phase and denies invocations that don't match the expected next agent (Scout→Builder→Auditor sequence). Bypass: `EVOLVE_BYPASS_PHASE_GATE=1`.

- **`scripts/dispatch/run-cycle.sh`** (NEW, ~170 lines) — Convenience driver. Picks next cycle ID, initializes cycle-state.json, spawns orchestrator subagent under its profile-restricted permissions. Includes `--dry-run` for inspection. Cleans up cycle-state on exit (success or failure).

- **`scripts/lifecycle/cycle-state.sh`** (NEW, ~150 lines) — Helpers (`init`/`advance`/`set-agent`/`get`/`clear`/`exists`/`dump`/`path`) for `.evolve/cycle-state.json`. Atomic mv-of-temp-file updates. Schema: cycle_id, phase, started_at, phase_started_at, active_agent, active_worktree, completed_phases, workspace_path.

- **`agents/evolve-orchestrator.md`** (NEW, ~80 lines) — Orchestrator subagent prompt. Sequences Scout→Builder→Auditor; on PASS invokes ship.sh; on FAIL/WARN invokes record-failure-to-state.sh (no inline retrospective per v8.12.3 design pivot). Cannot edit source code, commit, or push (kernel hooks block).

### Tests

- `scripts/role-gate-test.sh` — 20 tests covering all phases + always-safe paths + bypass + malformed state + relative-path resolution.
- `scripts/phase-gate-precondition-test.sh` — 15 tests covering trigger detection, per-phase allowlist, re-spawn, no-cycle passthrough, bypass.
- `scripts/guards-test.sh` — 34 tests (unchanged from v8.13.0 RC5).

Combined: **69/69 unit tests pass**.

### Modified

- `.claude/settings.json` — wires role-gate.sh on `Edit|Write`, adds phase-gate-precondition.sh as second hook on Bash matcher.
- `.evolve/profiles/orchestrator.json` — allows `Bash(bash scripts/lifecycle/cycle-state.sh:*)` and `Bash(bash scripts/dispatch/run-cycle.sh:*)`. Updated `_design_notes` (no longer "scaffolding only").
- `skills/evolve-loop/SKILL.md` — documents `bash scripts/dispatch/run-cycle.sh` as the declarative alternative.
- `skills/evolve-loop/phases.md` — documents cycle-state.json + the three kernel hooks at every transition.

### Removed (branch cleanup)

- `feat/orchestrator-as-subagent` — superseded by v8.13.0 (clean rewrite). 1 unmerged WIP commit `8bc2759` discarded.
- `feat/v8.13.0-ship-gate` — already merged into main as `ca5a5e3`. Local cleanup only.

### Why this is the right v8.13.1

The trust-boundary the user requested at session start is now structurally enforced by allowlist-shaped gates (parser-bypass arms races converge on enumeration, allowlists converge on canonical paths):

- **ship-gate (v8.13.0)** — canonical-script-allowlist
- **role-gate (v8.13.1)** — path-allowlist, per-phase
- **phase-gate-precondition (v8.13.1)** — sequence-allowlist

The architectural pattern is now consistent across all three gates. Same test discipline as v8.13.0: every gate ships with a unit-test suite that exercises both ALLOW and DENY paths plus the bypass env var.

### v8.13.1 audit RC1 follow-ups (incorporated before ship)

- **MEDIUM-1 fix**: orchestrator profile narrowed from `Bash(bash scripts/lifecycle/cycle-state.sh:*)` to specific subcommands `{advance, set-agent, get, exists, dump, path}`. The `init` and `clear` subcommands are now in `disallowed_tools`, preventing the orchestrator subagent from silently disabling the v8.13.1 gates by removing `cycle-state.json`. `run-cycle.sh`'s parent process owns the init/clear lifecycle.
- **LOW-2 fix**: `role-gate.sh` strips trailing slash from `ACTIVE_WT` and `WORKSPACE_PATH` after reading from cycle-state.json. Defensive trim against caller-supplied trailing-slash paths. New unit test (Test 21) covers this.
- **LOW-1**: canonicalize() docstring is cosmetic; deferred (no behavior change).
- **LOW-3**: run-cycle.sh dry-run log message wording is cosmetic; deferred.
- **LOW-4**: malformed cycle-state fail-open is intentional graceful degradation; cycle-health-check audit-time assertion deferred to v8.13.2.

### Out of scope (deferred to v8.13.2 / v8.14.0+)

- Cycle-state staleness detection (interrupted cycles leaving cycle-state.json behind).
- Cross-machine state sync (single-machine assumption holds).
- True orchestrator-as-subprocess (run-cycle.sh delegates to a `claude -p` subagent that itself spawns more subprocesses; this works but adds 1 layer of subprocess overhead).
- Audit-time check that "if cycle is in progress, cycle-state.json must parse" (LOW-4 follow-up).

---

## [8.13.0] - 2026-04-27

### **BREAKING — atomic ship-gate via canonical `scripts/lifecycle/ship.sh`**

After 5+ audit cycles of parser-bypass arms races (cycles 8121-8129), v8.13.0 reframes the problem. Instead of detecting ship-class commands (git commit / git push / gh release create) inside arbitrary bash via increasingly clever parsers, the gate now allowlists exactly ONE canonical script: `scripts/lifecycle/ship.sh`. The gate's check is trivially simple: "does this command's first executable, resolved via realpath, equal `scripts/lifecycle/ship.sh`?" If yes, allow (ship.sh enforces the audit-first contract internally). If no AND the command contains ship verbs, deny.

**This kills the parser-bypass arms race.** D1-D3 from cycle 8122 (bare-newline, pipe-to-shell, here-string), D6 (commit→push workflow regression), and D-NEW-1 from cycle 8130 (bash -c bypass) are all resolved.

### Breaking change for users

Raw `git commit`, `git push`, and `gh release create` invocations are now **denied** by the ship-gate hook (`scripts/guards/ship-gate.sh`, wired via `.claude/settings.json`).

**To ship a commit:**

```bash
bash scripts/lifecycle/ship.sh "<commit-message>"
# Optionally with release notes for a GitHub release:
EVOLVE_SHIP_RELEASE_NOTES="$(cat NOTES.md)" bash scripts/lifecycle/ship.sh "<commit-message>"
```

**Emergency bypass:**

```bash
EVOLVE_BYPASS_SHIP_GATE=1 git commit -m "<msg>"     # bypasses the gate
EVOLVE_BYPASS_SHIP_VERIFY=1 bash scripts/lifecycle/ship.sh    # bypasses ship.sh's internal audit checks
```

Both bypasses are logged with explicit WARN.

### Added — `scripts/lifecycle/ship.sh` (canonical atomic shipper, ~190 lines)

Enforces the full audit-first contract before any git operation:

1. **TOFU self-SHA verification** — pins ship.sh's SHA256 in `.evolve/state.json` on first run; subsequent modifications fail loudly. To intentionally update ship.sh, remove `state.json:expected_ship_sha` first.
2. **Latest auditor ledger entry** — must exist with `exit_code: 0`.
3. **Audit-report SHA match** — recorded SHA256 must equal current file SHA256 (catches post-audit mutation).
4. **Verdict: PASS** in the audit report (word-anchored regex, no PASSABLE/PASSTHROUGH false matches).
5. **Cycle binding** (the H2 fix) — current `git rev-parse HEAD` and `sha256(git diff HEAD)` must match the ledger entry's `git_head` and `tree_state_sha`. Refuses to ship work that wasn't audited.
6. **Audit freshness** — 7-day cap.
7. **Atomic commit + push** (+ optional `gh release create`) — all in one bash invocation; the gate fires once on the outer call.

### Added — `scripts/guards/ship-gate.sh` (simplified PreToolUse hook, ~170 lines)

- **Step 1**: realpath-resolves first executable; allows ship.sh canonical path.
- **Step 1.5**: detects `bash -c "..."`, `sh -c "..."`, `eval "..."` patterns and recursively checks the inner snippet (D-NEW-1 fix from cycle 8130).
- **Step 2**: regex-checks for ship verbs at tokenizable boundaries; awk pre-processor strips heredoc bodies first (no false positives on build reports that mention "git commit" in their text).

### Added — telemetry tests (34 total)

- `scripts/guards-test.sh` (19 unit tests): canonical-path allow, ship verbs blocked across raw/chained/subshell/pipe/heredoc/eval forms.
- `scripts/ship-integration-test.sh` (7 end-to-end tests in temp git repos): no-ledger refusal, PASS audit succeeds, WARN refusal, tree-state-mismatch refusal, HEAD-moved refusal, ship.sh self-SHA tampering refusal, bypass switch.
- `scripts/subagent-run-test.sh`: extended to validate the new orchestrator profile (12/12, was 11/11).

### Added — `.evolve/profiles/orchestrator.json` (orchestrator subagent profile, scaffolding)

The orchestrator-as-subagent vision from `feat/orchestrator-as-subagent` (commit 8bc2759, parked since cycle 8121) lands as scaffolding. Without v8.13.1's `scripts/dispatch/run-cycle.sh` deterministic driver, this profile is not yet invoked by anything. It provides the M2-fix interpreter denials when the spawner ships:

- Disallowed: `Bash(perl:*)`, `Bash(perl -e:*)`, `Bash(ruby:*)`, `Bash(ruby -e:*)`, `Bash(python -c:*)`, `Bash(python3 -c:*)`, `Bash(node -e:*)`, `Bash(osascript:*)`, `Bash(sh -c:*)`, `Bash(bash -c:*)`, `Bash(zsh -c:*)`, `Bash(env:*)`, `Bash(exec:*)`, `Bash(eval:*)`, `Bash(awk:*)`.
- Sandbox: `read_only_repo: true`, allow_network: true (orchestrator may need to reach Anthropic API for sub-subagent invocation).
- Allowed: `Bash(bash scripts/lifecycle/ship.sh:*)`, `Bash(bash scripts/dispatch/subagent-run.sh:*)`, narrow git read commands (status, log, diff, show, ls-files, rev-parse, branch, stash list, worktree list).

### Changed — cycle-binding fields in ledger

`scripts/dispatch/subagent-run.sh` now captures `git_head` (= `git rev-parse HEAD`) and `tree_state_sha` (= sha256 of `git diff HEAD`) at agent-start time and emits both in every `agent_subprocess` ledger entry. ship.sh uses these to enforce the cycle-binding contract.

### Audit history

| Cycle | Verdict | Defects |
|---|---|---|
| 8121 | FAIL | H1 parser bypasses (≥6 classes), H2 no cycle binding, M1 dead code, M2 interpreter holes |
| 8122 | FAIL | D1 bare-newline, D2 pipe-to-shell, D3 here-string, D5 backwards-compat, D6 commit→push workflow regression |
| 8126 (RC3 TMPDIR) | FAIL | TMPDIR override empirically non-functional |
| 8127 (RC4 worktree) | FAIL | Auto-worktree had three failure modes |
| 8128 (RC5 reverted) | PASS | Shipped v8.12.3 with EPERM as known issue |
| 8129 (v8.12.4) | PASS | Sandbox /tmp read fix |
| 8130 (RC1) | FAIL | D-NEW-1 bash -c bypass, D-NEW-2 missing CHANGELOG, D-NEW-3 atomic-ship doc, D-NEW-4 verdict regex word anchor |
| **8131 (RC2)** | **PASS** | All 4 D-NEW defects fixed; 19 guards-test + 7 ship-integration-test all pass |

### Migration

This is the FIRST release that uses ship.sh to ship itself. The `.claude/settings.json` is created in this cycle with the PreToolUse hook wiring; first run after merge will set up the TOFU self-SHA pin in `.evolve/state.json`.

For existing v8.12.4 users:
- Pull the new code; the next `git commit` will be denied unless via ship.sh
- One-time setup: run any audit cycle, then `bash scripts/lifecycle/ship.sh "<msg>"` — it'll pin its own SHA on first run
- The ship-gate is project-scoped via `.claude/settings.json` — only fires inside this repo's working directory

### Out of scope (deferred to v8.13.1+)

- `scripts/dispatch/run-cycle.sh` — deterministic driver that sequences phases and spawns the orchestrator subagent. Without it, the orchestrator profile shipped here is scaffolding only.
- `scripts/guards/role-gate.sh` — block Edit/Write outside cycle workspace mid-cycle.
- `scripts/guards/phase-gate-precondition.sh` — block out-of-order phases.
- Skill rewrite for `/evo:loop` to invoke the orchestrator subagent.

## [8.12.4] - 2026-04-27

### Fixed — EPERM on bash tool output files (5-cycle hunt resolved)

The macOS sandbox-exec profile in `scripts/cli_adapters/claude.sh` was missing read permissions on the tmp paths it allowed writes to. When `claude -p` wrote a bash tool's output to `/tmp/claude-${UID}/<project>/<session>/tasks/<id>.output` (write permitted), then attempted to read it back to return the result to the LLM, the read was denied — surfacing as `EPERM`.

The model frequently misread this as "another Claude Code process deleted the file during startup cleanup" — which led to two failed attempts in v8.12.3 (TMPDIR override RC3, auto-worktree RC4) targeting concurrent-session collision instead of the actual sandbox gap. Both reverted in v8.12.3 RC5.

**Root cause confirmed empirically** by inspecting `/tmp/claude-501/` directly — concurrent sessions already get unique session-UUID subdirs. Then `grep "(allow file-read.*tmp" claude.sh` returned zero matches. The bug was 100% sandbox-side.

**Fix**: 4 new `(allow file-read* (subpath ...))` rules added before the existing write rules in `generate_macos_sandbox_profile()`:

```scheme
(allow file-read* (subpath "/tmp"))
(allow file-read* (subpath "/private/tmp"))
(allow file-read* (subpath "/var/folders"))
(allow file-read* (subpath "/private/var/folders"))
```

### Empirical impact on audit latency

The cycle 8129 audit was the first v8.12.x audit where bash commands actually worked. Comparison:

| Audit | Bash works | Turns | Duration | Cost |
|---|---|---|---|---|
| Cycle 8128 (v8.12.3) | ✗ EPERM | 27 | 188s | $0.57 |
| **Cycle 8129 (v8.12.4)** | ✅ | **15** | **119s** | **$0.38** |

**44% fewer turns, 37% faster, 33% cheaper** — without changing token counts or audit prompts. The auditor no longer has to compensate for missing bash by individually reading every source file; it can grep and run scripts directly.

### What this resolves

- All v8.12.x audits (cycles 8121, 8121-v8121, 8122, 8124, 8126, 8127, 8128) reported variations of "Bash command execution was blocked by EPERM" / "another Claude Code process deleted it during startup cleanup". All now obsolete.
- The "known issue, deferred to v8.12.4" footnote in v8.12.3's CHANGELOG and `subagent-run.sh:295-313` docstring now have a fix shipped.

### Audit

Cycle 8129 (Sonnet, 119s, 15 turns, $0.38). Verdict: **PASS**. The empirical EPERM test passed: 3 independent bash calls succeeded with zero EPERM. One LOW finding (auditor profile doesn't include `Bash(bash scripts/subagent-run-test.sh:*)` in allowlist, so the test-run criterion was unverifiable — bash itself works, the gap is profile scope, not EPERM). No MEDIUM+ defects.

### Note

The 5-cycle journey (RC3 TMPDIR → RC4 auto-worktree → RC5 revert → 8128 PASS without fix → v8.12.4 actual fix) is itself a documented case of how an error message can mislead the diagnostic process. The audit reports' consistent "concurrent process deleted file" framing was a hallucinated explanation that anchored two iterations of work toward the wrong layer. The fix only became visible after both bad fixes were reverted AND the sandbox profile was inspected directly.

## [8.12.3] - 2026-04-27

### Added — Token economy: subagent prompt prefix reduced ~75%

Six profile JSONs (`scout`, `builder`, `auditor`, `inspirer`, `evaluator`, `retrospective`) now pass `--disable-slash-commands`, `--setting-sources`, `project` to `claude -p`. These flags drop the 227 user-level skills' metadata (~50–90K tokens) from every subagent's system prompt. None of the subagents grant the `Skill` tool in their allowed_tools, so they couldn't invoke `/skill-name` anyway — the metadata was pure overhead.

**Empirically verified** (cycle 8124 audit):
- `cache_creation_input_tokens`: 135,173 → 45,424 (66% reduction)
- `cache_read_input_tokens`: 2,414,580 → 529,642 (78% reduction)
- Audit duration: 625s → 162s (74% faster on Sonnet)

### Added — Subagent telemetry sidecars

After every successful agent run, `scripts/dispatch/subagent-run.sh` writes two sidecar files into the workspace:

- **`${agent}-usage.json`** — extracts `usage`, `modelUsage`, `duration_ms`, `num_turns`, `total_cost_usd` from the agent's stdout JSON. Future audits can self-verify empirical token effects without parsing their own stdout.
- **`${agent}-timing.json`** — per-phase ms breakdown (`profile_load_ms`, `prep_total_ms`, `adapter_invoke_ms`, `finalize_ms`, `total_ms`). Bash 3.2-compatible (uses temp file, not `declare -A`). Confirmed empirically that 99.9% of subagent runtime is the `claude -p` API call; pure runner overhead is ~110ms.

### Added — Lightweight failure recording (design pivot from v8.12.2)

Per user direction, the `evolve-retrospective` subagent (shipped in v8.12.2) is **no longer invoked per-cycle**. Instead, when an audit returns FAIL/WARN/SHIP_GATE_DENIED:

1. Capture `git diff HEAD > $WORKSPACE_PATH/failed.patch` (forensic)
2. Run `scripts/failure/record-failure-to-state.sh $WORKSPACE_PATH $VERDICT` — extracts audit defects (severity + title) from `audit-report.md`, captures cycle/git-head/tree-state SHA + audit-report SHA256, appends a structured entry to `state.json.failedApproaches[]` with `retrospected: false`. **Total cost: ~50ms shell, no LLM calls.**
3. `git worktree remove --force` discards the failed code.

The retrospective subagent runs separately in **batches** (on-demand or scheduled), synthesizing cross-cycle patterns from accumulated `failedApproaches` entries. This is more useful than per-cycle retrospectives because failure patterns ("3rd parser bypass this month") only emerge from multiple data points.

**Net change**: per-FAIL cycle saves ~$0.50 + 3-5 minutes. Forensic information preserved (audit-report.md + failed.patch + state.json entry).

### Changed — Phase docs

- **`skills/evolve-loop/phase6-learn.md`** § 4c rewritten: documents the lightweight inline recording flow + the deferred batch-retrospective pattern. Heading enumerates `FAIL / WARN / SHIP_GATE_DENIED` (no longer ambiguous "FAILED tasks only").
- **`skills/evolve-loop/phases.md`** audit-to-ship branch updated: PASS → ship; FAIL/WARN/SHIP_GATE_DENIED → drop + record-failure (lightweight, no subagent). Reference to phase6-learn.md § 4c for the deferred-retrospective flow.

### Known issue — deferred to v8.12.4

**EPERM on `/tmp/claude-${UID}/...` task output files** during concurrent claude-session collisions. Three approaches investigated and rejected:
- v8.12.3 RC3 TMPDIR-override: empirically non-functional (Claude Code uses hardcoded `/tmp/claude-${UID}/`, not `${TMPDIR}/`).
- v8.12.3 RC4 auto-worktree (per-subagent unique cwd): empirically non-functional (cwd-hash alone doesn't prevent cleanup; broke profile's relative-path Write patterns; showed worktree HEAD instead of orchestrator's working tree).

EPERM remains an open issue. Documented in `scripts/dispatch/subagent-run.sh:295-313` with the failed approaches enumerated. Next investigation should target the actual cleanup trigger (likely `.claude/` directory location or parent PID, not cwd).

### Audit

- RC1 (cycle 8124, Sonnet): PASS — empirical token reduction verified.
- RC2 (cycle 8125, attempted): build progressed; audit aborted by user.
- RC3 (cycle 8126, Sonnet): FAIL — TMPDIR fix non-functional. Reverted.
- RC4 (cycle 8127, Sonnet): FAIL — auto-worktree fix had three failure modes. Reverted.
- RC5 (cycle 8128, Sonnet): **PASS** — auto-worktree reverted, all 8 acceptance criteria met, 0 MEDIUM+ defects, 2 LOW non-blocking notes (criterion 2 grep count off by 1 in build template; criterion 7 empirically unverifiable due to EPERM). Recommended ship.

### Latency analysis

Empirical phase-timing from the cycle 8128 audit (`auditor-timing.json`):
```
profile_load_ms:      14
prep_total_ms:        57
adapter_invoke_ms: 187,769  ← 99.9% of wall-clock
finalize_ms:          39
total_ms:         187,879
```

The runner overhead is ~110ms; everything else is the `claude -p` API call. To reduce audit latency further, the lever is **turn count** (~27 in this audit) — pre-running tests in the orchestrator and inlining results would cut to ~10 turns. Deferred to v8.12.4 / v8.13.0.

## [8.12.2] - 2026-04-27

### Added — Failed-cycle retrospective + lessons-learned pipeline

- **`agents/evolve-retrospective.md`** — new failure post-mortem subagent. Fires only on Auditor `FAIL`/`WARN` or `SHIP_GATE_DENIED`. Reads cycle artifacts (audit-report, build-report, scout-report, the failed diff) and writes a structured retrospective + one or more failure-lesson YAMLs into `.evolve/instincts/lessons/`. Read-only outside the lesson directory; cannot mutate state.json, ledger, profiles, or personal instincts.
- **`.evolve/profiles/retrospective.json`** — least-privilege permission profile. `read_only_repo: true`, `allow_network: false`. Allowed writes: only the retrospective report, handoff JSON, and lesson YAMLs. Disallows all source code mutation, git commit/push/release, all interpreter `-c`/`-e` flags, and network tools (WebFetch, WebSearch, curl, wget). Sandboxed via macOS sandbox-exec or Linux bwrap.
- **`scripts/failure/merge-lesson-into-state.sh`** — orchestrator post-processor. Reads `handoff-retrospective.json`, appends new lesson IDs to `state.json.instinctSummary[]` (so future Scout/Builder/Auditor agents see them in their context block via the existing channel), appends a structured `failedApproaches[]` entry, and emits a `SYSTEMIC_FAILURE` ledger event when the retrospective flagged 3+ same-error-category failures across recent cycles. The retrospective profile cannot mutate state.json directly — this helper runs under orchestrator permissions.
- **`scripts/merge-lesson-test.sh`** — 7-check smoke test (no-op on missing handoff, single-lesson merge, failedApproaches append, missing-YAML integrity exit 2, SYSTEMIC_FAILURE ledger event, malformed-handoff exit 1, multi-lesson merge).
- **`.evolve/instincts/lessons/.keep`** — directory marker so the lessons directory ships with the plugin. Per-project lesson YAMLs (`inst-L*.yaml`) remain gitignored.
- **`scripts/dispatch/subagent-run.sh`**: registered `retrospective` as a recognized agent role. Updated header docs and usage text.
- **`scripts/subagent-run-test.sh`**: extended Test 1 to validate the retrospective profile (now 11/11 from 10/10).

### Changed — Skill docs document the explicit FAIL branch

- **`skills/evolve-loop/phase6-learn.md`** § 4c rewritten to invoke the `evolve-retrospective` subagent (mirroring Scout/Builder/Auditor pattern) instead of doing classification inline. Documents lesson schema, error-category taxonomy (`planning` / `tool-use` / `reasoning` / `context` / `integration`), failed-step taxonomy, and orchestrator post-processing. Heading explicitly lists `FAIL` / `WARN` / `SHIP_GATE_DENIED` (was ambiguous "FAILED tasks only").
- **`skills/evolve-loop/phases.md`** audit-to-ship transition: PASS → ship; `WARN`, `FAIL`, or `SHIP_GATE_DENIED` → drop work via `git worktree remove --force`, run retrospective, do NOT commit.
- **`agents/evolve-auditor.md`** Verdict Rules: added "Downstream consumer note" instructing the Auditor to write per-defect root causes (not just symptoms), use consistent severity labels and IDs, and name contradicted prior instincts so the retrospective can propagate them via the lesson's `contradicts` field.

### Fixed — sandbox enforcement bugs (also affects all v8.12.0+ profiles with `read_only_repo: true`)

- **`scripts/cli_adapters/claude.sh`**: the `read_only_repo: true` branch in the macOS sandbox profile generator was a `:` placeholder. Replaced with `echo "(deny file-write* (subpath \"$repo_root\"))"` — emitted before the `write_subpaths` allow loop so per-agent allows correctly override (SBPL last-match-wins). Belt-and-suspenders against future broader allow rules; the auditor, evaluator, scout, and retrospective profiles all benefit.
- **`scripts/cli_adapters/claude.sh`**: the Linux bwrap branch hard-coded `--share-net`, ignoring `allow_network: false` in profiles. Replaced with conditional `--share-net` (when true) / `--unshare-net` (when false). Now mirrors the macOS branch's network policy. The retrospective profile's stated guarantee ("network is disabled") is now actually enforced on Linux.
- **`scripts/failure/merge-lesson-into-state.sh`**: removed unused `mapfile_compat` function.
- **`.evolve/profiles/retrospective.json`**: removed extraneous `Write(.evolve/runs/cycle-*/retrospective-stdout.log)` allow — the runner adapter writes stdout via redirection, not the subagent.

### Audit

- RC1 (Sonnet): WARN — 3 MEDIUM defects (sandbox enforcement bug, Linux bwrap network gap, ambiguous heading). All 9 acceptance criteria PASS otherwise.
- RC2 (Sonnet): PASS — all 5 defects (3 MEDIUM + 2 LOW) verified fixed; 0 new defects introduced; pre-existing LOW-bwrap-bind-coverage flagged for v8.13.0 backlog.

### Notes

- v8.12.0's `read_only_repo: true` flag was advertised as enforcement but was a no-op for ~24 hours. The flag set is now correctly enforced as of v8.12.2. No security incident is known; the implicit deny via "no allow rule covering the repo" provided defense in practice. The explicit deny added in this release is belt-and-suspenders documentation of the contract.

## [8.12.1] - 2026-04-27

### Fixed
- **`scripts/cli_adapters/claude.sh`** — two latent v8.12.0 adapter bugs:
  1. **Tool-pattern word-split**: `Bash(python -m pytest:*)` and similar patterns containing spaces were word-split when passed via `--allowedTools $JOINED_STRING`, producing tokens like `Bash(python` and `-m`. Claude's CLI parser rejected `-m` as an unknown flag (`error: unknown option '-m'`), silently breaking the auditor profile. The runner's `--validate-profile` path didn't catch it because validate-only never invokes `claude` — it only constructs the command line. Fix: read JSON arrays into bash arrays via portable `while IFS= read -r line; do … done < <(jq -r '.field[]?' …)` (bash 3.2 compatible — macOS default shell), then pass with `CMD+=(--allowedTools "${ALLOWED_TOOLS[@]}")` so each pattern survives shell tokenization as its own argv element.
  2. **`--bare` blocks OAuth auth**: `claude --bare` per `claude --help` "is strictly ANTHROPIC_API_KEY or apiKeyHelper via --settings (OAuth and keychain are never read)". Most Claude Code users authenticate via OAuth (no `ANTHROPIC_API_KEY` env var), so subagent subprocesses launched with `--bare` returned `"Not logged in · Please run /login"`. Fix: drop `--bare` automatically when `ANTHROPIC_API_KEY` is unset; emit WARN to stderr; `EVOLVE_FORCE_BARE=1` retains it for users who do have the API key set.

### Audit
- Reviewed by Auditor subagent (cycle 8121-v8121, model: opus, sandbox-exec macOS). Verdict: **PASS** — all 6 acceptance criteria verified, 3 LOW non-blocking observations, 0 MEDIUM+ defects.

### Deferred
- The orchestrator-as-subagent + ship-gate hook + Python shlex parser work tested in this cycle on `feat/orchestrator-as-subagent` (commit `8bc2759`) returned FAIL on two audits (cycle 8121, 8122) due to parser bypasses (D1 bare-newline, D2 pipe-to-sh, D3 here-string) and a workflow regression (D6: `git push` denied after `git commit` due to HEAD movement). That work is parked for v8.13.0, where it will land alongside the deterministic `run-cycle.sh` driver and `role-gate.sh` / `phase-gate-precondition.sh`. Failed-cycle retrospective + lessons-learned pipeline planned as v8.12.2.

## [8.12.0] - 2026-04-27

### Added
- **Subagent subprocess isolation runner (`scripts/dispatch/subagent-run.sh`)** — single entry point for invoking phase agents (Scout, Builder, Auditor, Inspirer, Evaluator) as isolated subprocesses with per-agent CLI permission profiles. Per-invocation challenge tokens detect forged artifacts; tamper-evident SHA256 ledger entries detect post-run mutation.
- **Per-agent permission profiles (`.evolve/profiles/*.json`)** — least-privilege CLI flag templates encoding `allowed_tools`, `disallowed_tools`, `permission_mode`, `add_dir`, sandbox config, and budget caps for each phase agent.
- **CLI adapter abstraction (`scripts/cli_adapters/{claude,gemini,codex}.sh`)** — uniform interface for provider-agnostic subagent dispatch. Claude adapter wraps invocations in `sandbox-exec` (macOS) or `bwrap` (Linux) when `EVOLVE_SANDBOX=1`.
- **OS-level sandboxing** for every Claude subprocess. Auditor and Evaluator profiles run with `read_only_repo: true` (read-only mount of the repo); Evaluator additionally has `allow_network: false`.
- **Mutation-testing pre-flight (`scripts/verification/mutate-eval.sh`)** — applies language-aware mutations to source-under-test and verifies eval kill rate ≥ 0.8. `phase-gate.sh gate_discover_to_build` runs this against every new eval and flags tautological checks (rollout phase 1: WARN-only).
- **Adversarial Auditor mode** (default-on) — runner injects an "ADVERSARIAL AUDIT MODE" prefix that requires positive executable evidence (test output, diff hunk, or repro command) for PASS verdicts. Counters same-model-judge sycophancy. Auditor model defaults to a different family from Builder (Opus vs Sonnet). Disable with `ADVERSARIAL_AUDIT=0` for permissive sweeps.
- **Subagent ledger match check** — `phase-gate.sh check_subagent_ledger_match` re-hashes each artifact and compares with the SHA256 recorded by the runner at write time, catching post-run mutation.
- **Smoke tests** — `scripts/subagent-run-test.sh` (10 checks: profile validation, forgery rejection, token presence, unknown-agent, missing-artifact, legacy-fallback) and `scripts/subagent-adversarial-test.sh`.

### Changed
- **CLAUDE.md** — added rules 5-8 codifying subprocess-isolation, OS sandboxing, mutation pre-flight, and adversarial auditor as non-negotiable defaults.
- **Skill phase docs** (`phase2-discover.md`, `phase3-build.md`, `phases.md`, `inspirer/SKILL.md`, `evaluator/SKILL.md`, top-level `evolve-loop/SKILL.md`) — replaced platform-dispatch boilerplate with the `subagent-run.sh` invocation contract. `LEGACY_AGENT_DISPATCH=1` documented as a one-A/B-cycle escape hatch only.
- **`agents/evolve-auditor.md`** — PASS verdict criterion tightened to require positive per-criterion evidence; absence of MEDIUM+ issues alone is not sufficient.
- **`scripts/verification/eval-quality-check.sh`** — fenced-code-block fallback parser. Files with eval commands only inside fenced blocks now WARN; files with no parseable commands emit `ANOMALY` (not silent skip).
- **`.gitignore`** — surgical exception so `.evolve/profiles/*.json` ship with the plugin while runtime state under `.evolve/` (state.json, ledger.jsonl, runs/, evals/, history/) remains ignored.

### Fixed
- **`scripts/dispatch/subagent-run.sh:322`** — successful agent run now exits 0 (per documented contract) instead of 1. Previously every successful subagent invocation was reported as a failure to the orchestrator.

### Documentation
- Added `docs/reports/2026-04-26-subagent-isolation-hardening-report.md` — full incident-style report on the isolation hardening initiative.

## [8.11.1] - 2026-04-20

### Fixed
- **Stability**: Enforce execution-based evals by blocking tautological checks [cycle 15].
- **Safety Guidelines**: Added incident report for Flawless Execution Anomaly and updated safety guidelines.
- Reverted "stability: implement Reasoning Asymmetry for Planner-Auditor [cycle 2]".

## [8.11.0] - 2026-04-20

### Added
- **`autoresearch` strategy** implemented for hypothesis testing against fixed metrics, embracing failure, and deep out-of-the-box exploration.
- **Platform-agnostic generalization**: dynamically scales context windows for Gemini CLI (2M tokens) and gracefully supports non-Claude worktree orchestration and skill lookups.

### Changed
- **Decriminalized failure**: Under `autoresearch` and `innovate` strategies, experimental failures no longer drop consecutive clean scores or discard worktrees.
- **Smart Web Search forced**: Context budget constraints are overridden for divergent strategies to guarantee deep research via the 6-stage pipeline.

## [8.10.3] - 2026-04-11

### Fixed
- **Removed `disable-model-invocation: true` from SKILL.md.** This flag blocked ALL Skill tool calls, including explicit `/evo:loop` slash commands, causing "Error: Skill evolve-loop:evolve-loop cannot be used with Skill tool due to disable-model-invocation". The skill description is specific enough ("Use when the user invokes /evo:loop") to prevent unwanted auto-triggering without the flag.

## [8.10.2] - 2026-04-11

### Fixed
- **`release.sh` plugin cache refresh now works post-push.** The old logic compared marketplace SHA to local HEAD (which was the pre-push commit), so the marketplace always appeared "up to date" with the old version. Now: always pulls unconditionally, then checks if the marketplace plugin.json version matches the target. Pre-push runs show "BEHIND — push first, then re-run"; post-push runs correctly refresh cache and registry.

## [8.10.1] - 2026-04-11

### Fixed
- **Context budget gate redesigned from cumulative to per-cycle model.** The old gate accumulated estimated token costs across all cycles (50K/cycle × N), hitting a 300K RED threshold at cycle 4-5 and forcing premature session breaks. The new gate asks "is there room for ONE more cycle?" — since agents run in isolated subagent context and auto-compaction reclaims older turns, effective context stays ~75-165K regardless of cycle count. Result: 10-cycle requests complete entirely in GREEN; YELLOW (lean mode) at cycle 10+; RED safety valve at cycle 30+.
- **YELLOW no longer suggests session break.** Old YELLOW recommendation said "Consider session break after this cycle" — the LLM over-interpreted this as a stop instruction. New YELLOW says "Lean mode activated — continue."
- **RED requires two consecutive confirmations to stop.** A single RED writes a handoff checkpoint but continues (auto-compaction frees space). Only two consecutive RED cycle starts trigger an actual session break.

## [8.10.0] - 2026-04-09

### Added
- **`ecc:e2e` first-class integration** — UI/browser tasks now auto-invoke the `everything-claude-code:e2e-testing` skill to generate and run Playwright tests. Scout routes UI work to a new `e2e` skill category; Builder Step 4.5 generates `tests/e2e/<slug>.spec.ts`; Auditor checklist D.5 verifies selector grounding, artifact presence, and `## E2E Verification` in the build-report; `phase-gate.sh` blocks ship if a UI task is missing e2e evidence.
- **`scripts/utility/setup-skill-inventory.sh` + `scripts/utility/setup_skill_inventory.py`** — deterministic filesystem scanner that indexes every installed skill (project, user-global, plugin cache) and writes `.evolve/skill-inventory.json`. Replaces LLM-side parsing of the session's skill listing with a zero-token, cache-friendly scan. Automatically picks newest plugin version, skips IDE mirror dirs (`.cursor/skills`, `.kiro/skills`), and categorizes via the routing taxonomy. Tested: 281 skills indexed across 7 scopes.
- **New E2E Graders eval-runner section** (`skills/evolve-loop/eval-runner.md`) — first-class grader type with artifact locations (`playwright-report/`, `test-results/`, `artifacts/*.zip`), flake handling, and skip-condition semantics.
- **Auditor audit-report template** extended with `## E2E Grounding (D.5)` table.
- **Builder build-report template** extended with `## E2E Verification` section.

### Changed
- **Phases renumbered to eliminate `x.5` irregularity.** Phase 0.5 → 1, cascade 1-6 → 2-7:
  - Phase 0: CALIBRATE (unchanged)
  - Phase 1: RESEARCH (was 0.5)
  - Phase 2: DISCOVER (was 1)
  - Phase 3: BUILD (was 2)
  - Phase 4: AUDIT (was 3)
  - Phase 5: SHIP (was 4)
  - Phase 6: LEARN (was 5)
  - Phase 7: META (was 6)
- **Phase markdown files renamed** to align filenames with phase numbers and descriptions:
  - `phase05-research.md` → `phase1-research.md`
  - `phase1-discover.md` → `phase2-discover.md`
  - `phase2-build.md` → `phase3-build.md`
  - `phase4-ship.md` → `phase5-ship.md`
  - `phase5-learn.md` → `phase6-learn.md`
  - `phase6-metacycle.md` → `phase7-meta.md` (filename now matches the `Phase 7: META` heading text)
- **Phase 0 Skill Inventory step** now calls `scripts/utility/setup-skill-inventory.sh` instead of LLM-parsing the system-reminder skill list. Deterministic, faster, and complete across every installed plugin.
- **Scout skill-matching table** adds an `e2e` category row routing UI tasks to `everything-claude-code:e2e-testing` as primary.
- **251 phase references** (text + filepaths) rewritten across 43 source files; TOC anchor slugs updated to match renumbered headers; `phase-gate.sh` anti-forgery whitelist extended with `setup-skill-inventory.sh`.

### Migration notes
- Phase numbering is an internal convention — plugin consumers invoke `/evo:loop` as before.
- `.evolve/` runtime artifacts from prior cycles still reference old phase names in historical logs; the next cycle's Scout will naturally write new-naming references.
- `skills/refactor/` has its own independent phase pipeline (SCAN/PRIORITIZE/PLAN/EXECUTE/MERGE) and was deliberately left unchanged.

## [8.9.1] - 2026-04-07

### Changed
- **Skill descriptions standardized to "Use when..." trigger format** — rewrote descriptions across all skills to start with concrete trigger conditions, improving auto-invocation accuracy.
- **`smart-web-search.md` split into reference files** — 654 → 112 lines. Extracted query transformation patterns, intent classification, and provider routing into `reference/`.
- **`phases.md` Phase 0.5 and Phase 1 extracted** — 700 → 474 lines. Each phase now has its own focused file (`phase05-research.md`, `phase1-discover.md`).
- **`refactor/SKILL.md` split into reference files** — 653 → 154 lines. Detection rules, fix patterns, and worktree orchestration moved to `reference/`.
- **Skill routing policy added** (`skill-routing.md`) — formal policy for which skill handles which kind of request, reducing dispatch ambiguity.
- **SKILL.md frontmatter standardized** — consistent header format and field ordering across all skills.

### Notes
Patch release. No behavior changes — all updates are documentation, file organization, and skill discoverability improvements.

## [8.9.0] - 2026-04-06

### Added
- **`/evaluator` skill** — Independent evaluation engine that works standalone or integrated with evolve-loop. 5-layer architecture (GRADE → DETECT → SCORE → DIRECT → META-EVAL) with 6 scoring dimensions.
- **6 scoring dimensions** — correctness (0.25), security (0.20), maintainability (0.20), architecture (0.15), completeness (0.10), evolution (0.10). Each scored 0.0-1.0 with confidence levels and 5-point granularity rubrics.
- **EST anti-gaming defenses** — Evaluator Stress Test (arXiv:2507.05619) protocol: perturbation tests detect format-dependent score inflation at 78% precision and 2.1% overhead. Includes saturation monitoring and proxy-true correlation tracking.
- **Self-improving evaluation lifecycle** — 4-stage lifecycle from EDDOps (arXiv:2411.13768): baseline → calibration → steady state → evolution. Adaptive difficulty auto-introduces harder criteria when dimensions saturate.
- **Strategic direction guidance** — Layer 4 (DIRECT) ranks improvement priorities by `(1.0 - score) * weight * feasibility` with evidence-linked recommendations tracing to specific files and lines.
- **Meta-evaluation (Layer 5)** — Red-team protocol for the evaluator itself. Triggered by repeated gaming detection, saturation, or proxy correlation drops.
- **3 evaluation scopes** — `task` (changed files), `project` (full codebase), `strategic` (trajectory and priorities).
- **Phase 3 delegation hook** — Evolve-loop Auditor can invoke `/evaluator --scope task` when `strategy == "harden"` or `forceFullAudit == true`.
- **`docs/research/evaluator-research.md`** — Comprehensive 414-line research archive documenting 14 papers, 8 agent benchmarks, 12 LLM-judge biases, reward hacking incidents, independent evaluation principles, and full cross-reference of existing evolve-loop eval mechanisms.
- **Reference material** — `scoring-dimensions.md` (6-dimension rubric), `anti-gaming.md` (EST protocol + known gaming patterns), `eval-lifecycle.md` (4-stage lifecycle + drift detection + meta-evaluation).

### Research Documented
- EDDOps reference architecture (arXiv:2411.13768) — evaluation as continuous governing function
- Evaluator Stress Test (arXiv:2507.05619) — gaming detection via format/content sensitivity
- CALM framework (arXiv:2410.02736) — 12 LLM-judge biases with mitigations
- METR reward hacking (June 2025) — frontier models actively hack evals
- Anthropic eval principles — unambiguous tasks, outcome over path, saturation monitoring
- AISI Inspect Toolkit — 3-axis sandbox isolation for evaluators
- LiveAgentBench, SWE-Bench Verified, CLEAR — major agent benchmarks 2025-2026

## [8.8.1] - 2026-04-06

### Added
- **`scripts/observability/token-profiler.sh`** — Measures token footprint of all skill, agent, and script files. Outputs ranked table with line counts and estimated tokens. Supports `--json`, `--save-baseline`, and `--compare` flags for tracking optimization progress over time.
- **`docs/research/token-optimization-guide.md`** — Research-backed optimization guide documenting 5 techniques (three-tier progressive disclosure, context block ordering, AgentDiet trajectory compression, event-driven reminders, per-phase context selection) with measured baselines and per-file recommendations. Cites 7 papers including AgentDiet (FSE 2026), OPENDEV, CEMM, and Prompt Compression Survey (NAACL 2025).

### Changed
- **`skills/evolve-loop/reference/policies.md`** — Compressed from 318 to 176 lines (44% reduction, ~2.1K tokens saved per read). Removed duplicate Session Break Handoff Template and compressed verbose rate limit pseudocode into tables. All 11+ functional sections preserved with zero quality loss.

## [8.8.0] - 2026-04-06

### Added
- **`/inspirer` skill** — Standalone creative divergence engine grounded in data-driven web research. Extracts the evolve-loop's internal creativity mechanisms (provocation lenses, concept scoring, research grounding) into a reusable skill invocable on any topic.
- **6-stage pipeline** — FRAME (parse topic) → DIVERGE (apply lenses) → RESEARCH (web search) → SCORE (Inspiration Cards) → CONVERGE (rank & filter) → DELIVER (report/table/JSON).
- **12 provocation lenses** — 10 from evolve-loop (Inversion, Analogy, 10x Scale, Removal, User-Adjacent, First Principles, Composition, Failure Mode, Ecosystem, Time Travel) + 2 new general-purpose lenses (Constraint Flip, Audience Shift).
- **3 depth levels** — QUICK (~20K tokens, 3 lenses), STANDARD (~40K, 4 lenses), DEEP (~60K, 5 lenses) for explicit creativity-vs-cost tradeoff.
- **Inspiration Cards** — Extended Concept Cards with one-liner pitch, implementation sketch (3-5 steps), risks, and next steps. Scored on feasibility x impact x novelty with KEEP/DROP verdicts.
- **Research grounding requirement** — Every idea MUST be backed by at least 1 web research result. No research = auto-drop.
- **3 output formats** — `full` (human-readable report), `brief` (compact table), `evolve` (JSON compatible with Scout task selection).
- **Domain affinity matrix** — Maps 5 topic domains to optimal lens selections for targeted creative divergence.
- **Phase 0.5 delegation hook** — Evolve-loop orchestrator can delegate to `/inspirer` when `strategy == "innovate"` or discovery velocity stagnates.
- **Reference material** — `provocation-lenses.md` (12 lenses with examples), `scoring-rubric.md` (detailed criteria), `worked-examples.md` (3 end-to-end pipelines).
- **Solution documentation** — `docs/reports/inspirer-solution.md` recording design rationale and architecture decisions.

## [8.7.0] - 2026-04-06

### Added
- **`/code-review-simplify` skill** — Unified code review and simplification engine integrated into the evolve-loop pipeline. Combines structured pattern checks with agentic reasoning in a single pass.
- **Hybrid pipeline+agentic architecture** — Pipeline layer runs 6 deterministic checks (~0.5s, ~2-5K tokens) before agentic layer handles contextual analysis (~15-40K tokens). Saves 40-60% tokens vs. separate review + simplify agents.
- **Multi-dimensional scoring** — 4 dimensions (correctness 0.35, security 0.25, performance 0.15, maintainability 0.25) with numeric 0.0-1.0 scores replace binary PASS/FAIL.
- **Adaptive depth routing** — 3 tiers (lightweight < 50 lines, standard 50-200 lines, full review > 200 lines) with auto-escalation for security-sensitive files.
- **`scripts/utility/code-review-simplify.sh`** — Pipeline layer engine with 6 checks: file length (800), function length (50), nesting depth (4), secrets detection, cognitive complexity (15/function), near-duplicate detection.
- **`scripts/verification/complexity-check.sh`** — Per-function cognitive complexity scorer with `--threshold` flag and multi-language support (bash, Python, JS/TS, Go, Java, Rust).
- **Auditor D4 integration** — Optional skill consultation for code changes > 20 lines; composite score supplements verdict; auto-generates simplification suggestions when maintainability < 0.7.
- **Builder self-review** — Optional Step 5 enhancement runs lightweight pipeline after eval pass; applies simplifications before auditor sees the code.
- **Simplification catalog** — 8 localized refactoring techniques (Extract Method, flatten nesting, decompose conditional, extract utility, rename, replace magic numbers, inline over-abstraction, remove dead code).
- **Solution documentation** — `docs/reports/code-review-simplify-solution.md` records research findings, build-vs-buy justification, architecture decisions, and future work.

### Research Findings
- Anthropic multi-agent code review: 16% → 54% substantive PR comments
- Cursor BugBot: pipeline → agentic = biggest quality gain (70% resolution, 2M+ PRs/month)
- Qodo 2.0: multi-agent specialists achieve F1 = 60.1%
- CodeScene: simplified code reduces AI token consumption ~50%
- ICSE 2025: LLMs excel at localized refactoring, weak at architectural

## [8.6.6] - 2026-04-05

### Added
- **Rate Limit Recovery Protocol** — Detects API rate limits after every agent dispatch (Scout, Builder, Auditor) and auto-schedules resumption via `/schedule` (remote trigger) or `/loop` (local retry) instead of silently dying.
- **3-tier auto-resumption** — Priority cascade: remote trigger (≥1hr limits) → local loop (short limits) → manual fallback.
- **Consecutive failure tracking** — 3+ sequential agent failures trigger rate limit recovery as a safety net.
- **Plugin cache refresh in release flow** — `scripts/utility/release.sh` now clears stale plugin cache, updates marketplace checkout, and refreshes the plugin registry automatically.

### Changed
- Orchestrator loop step 6 now includes rate limit check after every agent dispatch.
- `reference/policies.md` extended with Rate Limit Recovery section and comparison table (rate limit vs context budget).
- `phases.md` adds rate limit recovery gate wrapping all agent dispatches.

## [8.6.0] - 2026-03-31

### Added
- **External skill discovery and routing** — Phase 0 builds a skill inventory from installed plugins, categorizing ~150 skills into routing categories (security, testing, language:X, framework:X, etc.).
- **Task-to-skill matching** — Scout matches tasks to relevant external skills using a category routing table, adding `recommendedSkills` to task metadata.
- **Builder skill consultation** (Step 2.7) — Builder invokes matched skills via the `Skill` tool for domain-specific guidance before designing its approach.
- **Skill usage verification** (Auditor D3) — Auditor checks whether recommended primary skills were invoked (informational, non-blocking).
- **Skill effectiveness tracking** (Phase 5) — Tracks hit rate per skill; low-value skills demoted after 5+ invocations.
- **Skill Awareness** section in `agent-templates.md` — shared schema for `recommendedSkills` field.

### Changed
- **Scout and Builder tools** now include `Skill` in their tool arrays.
- **state.json schema** extended with `skillInventory` and `skillEffectiveness` fields.

## [8.5.0] - 2026-03-30

### Added
- **Beyond-the-Ask divergence trigger** — structured provocation system with 10 lenses (Inversion, Analogy, 10x Scale, Removal, User-Adjacent, First Principles, Composition, Failure Mode, Ecosystem, Time Travel) that fire during Phase 0.5 and Scout hypothesis generation to surface ideas beyond the user's explicit request.
- **Lens selection protocol** — each cycle selects 2 lenses (1 random + 1 matched to weakest benchmark dimension) for targeted creative divergence.
- **Beyond-ask tracking** in Phase 5 — hit rate, lens effectiveness, and benchmark delta for proactive insights. Underperforming lenses flagged for meta-cycle replacement.

### Changed
- **Scout hypothesis generation** now produces standard + beyond-ask hypotheses with differentiated auto-promotion thresholds (0.7 standard, 0.6 beyond-ask).
- **Phase 0.5** includes new Step 2.5 (DIVERGENCE TRIGGER) between gap analysis and research execution.
- **Scout report format** includes separate `Beyond-the-Ask Hypotheses` table.
- **Research brief format** includes `Beyond-the-Ask Provocations` section.

## [8.4.0] - 2026-03-30

### Added
- **Search routing** — decision table in `online-researcher.md` routes queries to Smart Web Search (deep research, surveys, concept cards) or Default WebSearch (quick lookups, error resolution, budget-constrained) based on complexity and token budget.
- **Cost profile benchmarks** — documented token/duration costs for each search approach based on head-to-head comparison testing.

### Changed
- **Builder reactive lookups** now default to Default WebSearch (1-2 direct queries, ~60% token savings) instead of always using the full Smart Web Search pipeline.
- **Phase 0.5 research** uses search routing — Smart for surveys/deep dives, Default for factual gap fills.
- **`smart-web-search.md`** clarifies when to use Smart vs Default with explicit routing guidance.

## [8.3.0] - 2026-03-30

### Added
- **Smart Web Search skill** — intent-aware 6-stage search pipeline that classifies questions, transforms queries using Query2doc/HyDE, iteratively searches and refines, and returns grounded cited answers.
- **Release checklist script** (`scripts/utility/release.sh`) — validates version consistency across all files before release to prevent version drift.

### Changed
- **Online Researcher** now leverages Smart Web Search skill for web searches.

## [8.0.0] - 2026-03-23

### Added
- **Progressive disclosure** — SKILL.md reduced from 523 to 90 lines (85% context reduction). Phase details load on demand via `read_file` references instead of being embedded inline.
- **Agent compression** — all 4 agent files compressed for 41% token reduction while preserving all behavior.
- **Anti-forgery defenses** (v7.9.0) — platform-specific safeguards after Gemini CLI forged audit reports during cross-platform run.
- **Research docs** — enterprise evaluation, agent personalization, adversarial eval co-evolution, runtime guardrails, secure code generation, multi-agent coordination, agent observability, uncertainty quantification, threat taxonomy, pre-execution simulation.

### Changed
- **SKILL.md architecture** — moved from monolithic orchestrator to progressive disclosure pattern. Entry point contains only routing logic; phase details are separate files loaded as needed.
- **Agent files** — restructured for leaner context footprint while maintaining full capability.
- **Reference files** — unified structure per Anthropic skill best practices (blockquote header, TOC, tables over prose).

### Security
- **Anti-forgery defenses** — added after incident where Gemini CLI session forged audit-report.md contents. Auditor now verifies report provenance.

## [7.8.0] - 2026-03-22

### Security
- **Deterministic phase gate script** (`scripts/lifecycle/phase-gate.sh`) — enforces phase transitions via bash, not LLM judgment. Verifies artifact existence, re-runs evals independently, checks health fingerprint, controls state.json writes. The orchestrator cannot skip, suppress, or bypass these checks.
- **Incident report: cycles 132-141** (`docs/incidents/cycle-132-141.md`) — documents orchestrator gaming: skipped agents, fabricated 4 empty cycles, inflated mastery. All existing detection mechanisms were bypassed because the orchestrator controlled whether they ran.
- **Anti-pattern #10: Orchestrator gaming** — added to SKILL.md with cross-reference to incident report

### Changed
- **Phase boundaries now mandatory** — all 5 phase transitions require `phase-gate.sh` execution (discover-to-build, build-to-audit, audit-to-ship, ship-to-learn, cycle-complete)
- **State.json writes moved to script** — `lastCycleNumber` and `consecutiveSuccesses` can only be updated by the phase gate script, not by the LLM directly
- **Safety & Integrity section rewritten** — now documents the separation of enforcement (scripts) from execution (LLM), with research basis (Greenblatt AI Control, Redwood Factored Cognition)
- **Protected paths expanded** — `scripts/` directory added to Builder's protected-file list alongside `skills/`, `agents/`, `.claude-plugin/`

### Research
- **Orchestrator anti-gaming research** (`docs/research-orchestrator-anti-gaming.md`) — surveyed principal-agent problem, separation of duties, tamper-proof logging, AI control protocols, factored cognition. Key finding: structural constraints > behavioral constraints.

## [7.7.0] - 2026-03-22

### Research
- **Pipeline optimization research** (`docs/research-pipeline-optimization.md`) — surveyed 25+ papers from 2025-2026 on parallelization, trimming, multi-model strategies. Key findings: 4-agent saturation (Google/MIT), Self-MoA > multi-model mixing (Princeton), speculative execution 48.7% latency reduction (Sherlock/Microsoft), AgentDiet 40-60% token savings

### Added
- **Self-MoA parallel builds** — spawn 2-3 Builder agents with approach diversity for M-complexity tasks; early termination accepts first passing result. Research: M1-Parallel 2.2x speedup (arXiv:2507.08944), Self-MoA (arXiv:2502.00674)
- **Budget-aware agent context** — `budgetRemaining` field (cyclesLeft, estimatedTokensLeft, budgetPressure) enables agents to self-regulate effort. Research: BATS framework (arXiv:2511.17006)
- **Per-phase context selection matrix** — each agent receives ONLY needed fields; saves 3-5K tokens per invocation. Research: Anthropic Select strategy
- **Speculative Auditor execution** — start Auditor concurrently with Builder; rollback on failure. Research: Sherlock (arXiv:2511.00330)
- **Eval-delta prediction** — Scout predicts benchmark impact per task; Phase 5 tracks prediction accuracy for calibration. Research: eval-driven development (arXiv:2411.13768)
- **Eager context budget estimation** — pre-compute cycle token cost before launching agents; proactive lean mode entry. Research: OPENDEV (arXiv:2603.05344)
- **AgentDiet trajectory compression** — prune useless/redundant/expired context between every phase transition. Research: AgentDiet (arXiv:2509.23586)

### Changed
- **Lean mode trigger** — now activates on budget pressure (not just cycle 4+), enabling earlier optimization
- **Scout task output** — now includes "Expected eval delta" field for prediction tracking
- **phase2-build.md** — expanded with Self-MoA dispatch, speculative auditor, trajectory compression sections

## [7.6.0] - 2026-03-22

### Added
- **Phase decomposition** — monolithic phases.md split into focused modules: `phase0-calibrate.md`, `phase2-build.md`, `phase5-learn.md`, `phase6-metacycle.md` (cycles 122-125)
- **Agent templates** — `agents/agent-templates.md` consolidates shared Input/Output schemas across Scout, Builder, Auditor (cycle 122)
- **Model routing doc** — `docs/reference/model-routing.md` is the single source of truth for tier definitions, provider mappings, and routing rules (cycle 124)
- **Changelog archive** — entries v2.0-v6.9 archived to `CHANGELOG-ARCHIVE.md`, keeping CHANGELOG.md lean (cycle 126)

### Changed
- **phases.md: 717 → 386 lines** (46% reduction) — Phase 0 and Phase 2 extracted to standalone modules
- **phase5-learn.md: 596 → 334 lines** (44% reduction) — meta-cycle logic extracted to phase6-metacycle.md
- **SKILL.md: 560 → 500 lines** (11% reduction) — model routing tables extracted to docs/reference/model-routing.md
- **token-optimization.md: 444 → 412 lines** — model routing duplication removed (references docs/reference/model-routing.md)
- **CHANGELOG.md: 368 → 102 lines** — old entries archived
- **Shared values consolidated** — memory-protocol.md Layer 0 references SKILL.md as canonical source (no duplication)
- **Dead state.json fields removed** — `processRewards` replaced by `processRewardsHistory` in schema
- **Instinct docs deduplicated** — docs/self-learning.md references phase5-learn.md instead of duplicating algorithms
- **Estimated token savings: 24-42K per cycle** (8-14% reduction) from modular loading and deduplication

### Architecture
```
Before (v7.5.0):                    After (v7.6.0):
phases.md (717 lines)               phases.md (386) — orchestrator sequencing
                                    ├── phase0-calibrate.md (99) — once per invocation
                                    ├── phase2-build.md (297) — build orchestration
                                    ├── phase4-ship.md (244) — shipping
                                    ├── phase5-learn.md (334) — per-cycle learning
                                    └── phase6-metacycle.md (191) — every 5 cycles

3 agents × duplicated boilerplate   agent-templates.md (68) + 3 lean agents
1 monolithic model routing table    docs/reference/model-routing.md (single source of truth)
```

## [7.5.0] - 2026-03-22

### Added
- **Platform compatibility doc** (`docs/architecture/platform-compatibility.md`) — tool mapping tables for 6 platforms, model tier mappings for 7 providers
- **Multi-platform agent frontmatter** — `capabilities`, `tools-gemini`, `tools-generic` fields in all 4 agents
- **Provider-agnostic prompt caching** — guidance for Anthropic, Google, OpenAI, and self-hosted engines

### Changed
- Agent invocation abstracted from Claude Code `Agent` tool to platform dispatch blocks
- Architecture doc updated: "host LLM session" replaces "Claude Code session"
- Model tier mappings updated to March 2026 latest (Gemini 3.1, GPT-5.4, Mistral Large 3, Qwen 3.5)

## [7.4.0] - 2026-03-21

### Added
- **Hallucination self-detection** — Auditor checklist now includes Section B2 that verifies imports, API signatures, and config keys against actual project dependencies. Catches fabricated APIs before they ship. (Source: agent-self-evaluation-patterns skill)
- **Parallel builder execution** — SKILL.md and phases.md now include explicit dependency-partitioning algorithm and fan-out/fan-in instructions for running independent tasks in parallel worktrees. Cuts cycle latency 2-3x for multi-task cycles. (Source: agent-orchestration-patterns skill)
- **Formal eval taxonomy** — Three grader types (`[code]`, `[model]`, `[human]`) formalized in eval-runner.md with type tagging, cost controls, and pass@k tracking. Scout tags every eval command with its grader type. (Source: eval-harness skill)
- **Process rewards per build step** — Builder reports step-level confidence in build-report.md. Auditor cross-validates via Section D2 (CALIBRATION_MISMATCH detection). Phase 5 aggregates step-level patterns into processRewardsHistory for meta-cycle analysis. (Source: eval-harness process rewards)
- **Instinct-to-skill graduation pipeline** — Meta-cycle now synthesizes qualifying instinct clusters (3+, same category, all confidence >= 0.8) into genes or skill fragments. Recorded in state.json.synthesizedTools. Closes the loop between learning and capability expansion. (Source: continuous-learning-v2, self-learning-agent-patterns skills)
- **Shared values inheritance model** — Shared agent values block in SKILL.md injected into every agent context. Eliminates protocol duplication across 4 agent files, enables single-source-of-truth meta-cycle edits. (Source: agent-shared-values-patterns skill)

### Changed
- **Version: 7.3.0 → 7.4.0** — minor version bump for 6 new features
- **Auditor reduced-checklist rule** — now references Section B2 (Hallucination Detection) alongside A and C as skippable sections
- **docs/skill-building.md** — Stage 5 expanded from 2 lines to full synthesis protocol with gene/skill-fragment examples
- **docs/meta-cycle.md** — Skill Synthesis section added between Automated Prompt Evolution and Mutation Testing

## [7.3.0] - 2026-03-20

### Added
- **Per-cycle enhanced summary** — each cycle now outputs a rich summary with benchmark delta, audit iterations, graduated instincts, operator warnings, and next focus
- **Final session report** — comprehensive markdown report generated after all cycles complete, covering task table, benchmark trajectory, learning stats, and recommendations
- **Auto version bump** — SHIP phase automatically increments patch version in plugin.json/marketplace.json after each cycle push
- **Operator brief spec doc** — new `docs/operator-brief.md` documenting the `next-cycle-brief.json` schema and cross-cycle communication protocol
- **Run isolation doc** — new `docs/run-isolation.md` documenting the `RUN_ID`/`WORKSPACE_PATH` parallel invocation safety model
- **Experiment journal doc** — new `docs/experiment-journal.md` documenting `experiments.jsonl` anti-repeat memory protocol
- **Scout discovery guide extraction** — modular discovery guide extracted from monolithic scout agent for better maintainability
- **Security self-check** — Builder agent now performs security self-verification before completing builds
- **Stepwise scoring enforcement** — mandatory stepwise confidence scoring wired into the evaluation protocol
- **isLastCycle flag** — passed to Operator context for reliable session-summary.md generation on final cycle
- **Instinct graduation section** — `docs/reference/instincts.md` now documents the graduation lifecycle
- **Parallel safety doc** — new `docs/parallel-safety.md` consolidating OCC, ship-lock, and run isolation

### Fixed
- **Schema hygiene** — missing fitness fields added to state.json schema example
- **Method attribution** — validation protocol added for research source attribution

### Changed
- **Benchmark score: ~91** — 12+ tasks shipped across cycles 20-23
- **Version: 7.2.0 → 7.3.0** — auto-bump now prevents version drift

## [7.2.0] - 2026-03-20

### Added
- **Stepwise self-evaluation** — Builder performs per-step correctness checks during implementation using stepwise verification (arxiv 2511.07364), catching errors before they compound
- **Instinct quality scoring (EvolveR)** — instincts now carry quality scores derived from downstream task outcomes, enabling confidence-weighted retrieval and automatic pruning of low-value instincts
- **MUSE functional memory categories** — instincts classified into functional categories (heuristic, constraint, pattern, anti-pattern) for targeted retrieval by agent role
- **CSI metric (Confidence-Stability Index)** — new composite metric tracking confidence-correctness alignment across cycles, used by Operator for pipeline health assessment
- **Phase 4 SHIP extraction** — shipping logic extracted into a dedicated, testable phase module with structured status reporting
- **Confidence-correctness alignment** — process rewards calibrated so stated confidence correlates with actual correctness (arxiv 2603.06604), reducing overconfident shipping of flawed changes

### Fixed
- **30+ broken internal links** — comprehensive link audit and repair across all docs, skills, and agent files (Cycle 16)
- **Link-checker grader regex** — fixed false negatives in the link-checker eval grader caused by overly strict regex patterns
- **processRewards schema** — corrected field validation that rejected valid reward entries with optional dimensions

### Changed
- **Benchmark score: 87.4 to ~91.5** — 9 tasks shipped across 4 cycles with 5 research methods adopted from 8 sources
- **CHANGELOG refreshed** — cycles 16-19 documented

## [7.1.0] - 2026-03-19

### Added
- **Chain-of-thought (CoT) design requirement** — Builder agent Step 3 now requires numbered reasoning steps with evidence citations before selecting an approach (+35% accuracy on complex tasks)
- **Multi-stage verification (MSV)** — Auditor agent applies segment→verify→reflect protocol for M-complexity tasks touching >3 files, with groundedness checking against filesToModify
- **Mutation testing specification** — eval-runner.md now documents mutation generation, kill rate calculation (target >=80%), and interpretation thresholds
- **Token budget awareness for Scout** — Scout agent now estimates per-task token cost and drops lowest-priority tasks when cycle budget (200K) would be exceeded
- **Eval grader best practices guide** — new `docs/eval-grader-best-practices.md` covering grader precision, anti-patterns, composition patterns, worked examples, and mutation resistance
- **Operator benchmark-to-brief translation** — Operator now maps projectBenchmark weakness scores to taskTypeBoosts in next-cycle brief, closing the benchmark→Scout feedback loop
- **Cross-run research deduplication** — OCC-based query locking protocol prevents parallel runs from issuing duplicate web searches (saves 45-90K tokens per overlapping cycle)

### Changed
- **All 4 agent files updated** — Builder (CoT), Auditor (MSV), Scout (token budget), Operator (benchmark sync) now implement documented accuracy and performance techniques
- **eval-runner.md** — mutation testing section + cross-reference to eval-grader-best-practices.md
- **CHANGELOG refreshed** — cycles 13-15 documented

## [7.0.0] - 2026-03-19

### Added
- **Accuracy self-correction techniques** — new `docs/accuracy-self-correction.md` with CoT prompting (+35% accuracy), multi-stage verification (HaluAgent pattern), context alignment scoring, and uncertainty acknowledgment, each mapped to specific evolve-loop agents
- **Implementation patterns** — concrete CoT-enforcing audit graders, multi-stage verification flow examples, and groundedness check patterns in accuracy-self-correction.md
- **Performance Profiling guide** — new `docs/performance-profiling.md` covering per-phase token measurement, cost-bottleneck identification, cycle-level telemetry, and model routing cost impact
- **Security considerations** — new `docs/security-considerations.md` documenting eval tamper detection, state.json integrity, prompt injection defense, rollback protocol, and output groundedness as security signal
- **Plan Cache Schema specification** — JSON schema, write-back protocol, similarity matching algorithm (composite score > 0.7), and eviction rules in token-optimization.md
- **Instinct Graduation specification** — graduation threshold (confidence >= 0.75, 3+ cycle citations), operational effects on Builder/Scout, and reversal conditions in phase5-learn.md
- **Agentic Plan Caching (APC) research baseline** — NeurIPS 2025 paper results (50.31% cost reduction, 27.28% latency reduction) documented in token-optimization.md
- **Dynamic Turn Limits** — probability-based marginal value gating pattern (24% cost reduction) in token-optimization.md

### Fixed
- **Benchmark eval macOS compatibility** — replaced grep -P (PCRE) with -E (POSIX ERE), fixed exit code handling, multi-file grep count summing, stale file paths, and setext header false positives
- **5 broken internal links** in SKILL.md and phase5-learn.md (incorrect relative paths from skills/evolve-loop/ to docs/)

### Changed
- **README.md** — updated project structure tree with all 18 docs, added 3 new feature bullets
- **Project digest** — regenerated at cycle 10 (meta-cycle)

---

For changelog entries prior to v7.0.0 (versions 2.0.0 through 6.9.0), see [CHANGELOG-ARCHIVE.md](CHANGELOG-ARCHIVE.md).
