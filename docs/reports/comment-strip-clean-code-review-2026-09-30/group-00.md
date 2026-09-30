## go/internal/gcpolicy/gcpolicy.go
verdict: MINOR
readability: 3
removed: REDUNDANT=2 RECOVERABLE=5 INVARIANT=5 DESIGN=3 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 5 | G25 / ch4-RECOVERABLE | `Mode string` hides its three values (off, shadow, enforce) and the rule that an empty mode means shadow. That default now lives only as raw literals in `cmd/evolve/cmd_loop_outcome.go:79-88`, not in this package's `WithDefaults`. | Add `type Mode string` with `ModeOff`, `ModeShadow`, `ModeEnforce` constants, plus `func (p Policy) EffectiveMode() Mode` (returns `ModeShadow` for ""). Switch the caller to use it. |
| 9 | N1 / ch4-RECOVERABLE | `TrackerTTLDays` does not say what it prunes: the `<run-dir>/.ephemeral` subtrees of runs that were kept. | Rename the Go field to `EphemeralTTLDays` and keep the JSON tag `tracker_ttl_days`. |
| 28-39 | G25 / ch4-RECOVERABLE | Magic numbers 10, 30, 30 and 7. The fact that 7 mirrors pruneephemeral is no longer visible. | Add named constants `DefaultKeepFullRuns = 10`, `DefaultSalvageTTLDays = 30`, `DefaultLogsTTLDays = 30` and `DefaultTrackerTTLDays = 7`. Share the 7 with `internal/pruneephemeral`. |
| 27 | ch4-INVARIANT | The warning that WithDefaults must never invent an Archive/Delete horizon is gone. `TestWithDefaults` already pins it (archive=0, delete=0). | No new test. Rename `TestWithDefaults` to `TestWithDefaults_NeverInventsArchiveOrDeleteHorizon` so the test name carries the why. |
| 17-18 | ch4-INVARIANT | "0 = never" and "delete wins over archive" are not visible from the field names. The delete-wins case is covered by `TestPlan_ArchiveThenDeleteLadder` (90-day run, both thresholds match). Zero-means-never is not pinned. | Add `TestPlan_ZeroArchiveAndDeleteDaysNeverTargetOldRuns` in `internal/gc`. |
| 23 | ch4-INVARIANT | `MinAgeMinutes` covers the race between creating a worktree and writing its lease. Nothing in the code says so. | Add `TestPlanWorktrees_CandidateYoungerThanMinAgeIsNeverTouched` in `internal/gc`. |
| 1, 12, 21 | ch4-DESIGN | The reason for splitting config from engine (the import cycle policy→gc→phasecontract→phasespec→policy), plus the S4 worktree-sweep grace, were dropped. | Move them into a new `docs/architecture/packages/internal-gcpolicy.md` under "Why a leaf" and "Worktree grace". |

## go/internal/core/phase.go
verdict: NEEDS-REFACTOR
readability: 2
removed: REDUNDANT=5 RECOVERABLE=8 INVARIANT=8 DESIGN=12 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 64-102 | G34 / ch10 | `PhaseRequest` has 32 fields. The blank-line groups used to have labels (build handoff, directives, routing overlay); now they are unexplained clusters. | Extract embedded structs so the JSON stays flat: `BuildHandoff{BuildPlan, BuildExplanation, BuildExplanationState, BuildExplanationError}`, `DispatchDirectives{CorrectionDirective, OperatorDirectives}` and `ModelRoutingOverlay{ModelRoutingCLI, ModelRoutingTier}`. |
| 67-69 | N4 / ch4-DESIGN+INVARIANT | `ProjectRoot`, `Workspace` and `Worktree` are three paths that look alike. Nothing now says which is the runtime-data root and which is the shipped tree, or that mixing them up points a predicate at the wrong tree. | Add a "PhaseRequest roots" section to `docs/architecture/packages/internal-core.md`. Add `TestDispatch_PredicatesRunAgainstWorktreeNotProjectRoot`. |
| 70 | ch4-INVARIANT | `WorktreeBaseSHA` is the host-owned base, and integrity diffs must use it rather than a mutable manifest field. That rule is now invisible. | Add `TestIntegrityDiff_UsesRequestWorktreeBaseSHANotManifest`. |
| 72 | ch4-INVARIANT | The read-only fence (agent writes are reported and undone; the phase never sets this itself) is invisible. | Pin it with the existing `internal/phases/runner/worktree_fence_verified_integration_test.go`, and rename its top test to `TestBaseRunner_ReadOnlyWorktreeUndoesAgentWrites`. |
| 74 | N1 / ch4-RECOVERABLE | `WorktreeVerified` does not say what was verified: the snapshot was restored after a read-only dispatch. It has 5 non-test references. | Rename to `WorktreeSnapshotRestored`. |
| 78, 97, 115 | N4 / ch4-RECOVERABLE | There are three `Signals`-like maps with different types and owners: selector inputs (`map[string]string`), the upstream bus, and response emissions. | Rename `PhaseRequest.Signals` to `DispatchSignals`, matching its builder `dispatchSignals()` (JSON tag unchanged). Add `type SignalBus map[string]any` with `func SignalKey(phase Phase, key string) string` for the `<phase>.<key>` namespace. |
| 82 | ch4-INVARIANT | `BuildPlan` is populated only at EVOLVE_PHASE_IO>=advisory with the planner enabled, so dispatch stays byte-identical below that. The gate is not visible. | Add `TestDispatch_BuildPlanEmptyBelowPhaseIOAdvisory`. |
| 86 | ch4-INVARIANT | `BuildExplanationError` is host-derived data and must never be treated as prompt instructions. This security rule is gone. | Add `TestRetroPrompt_QuotesBuildExplanationErrorAsData`. |
| 90 | N4 / ch4-RECOVERABLE | `BypassPolicy` reads as "skip all policy", but it only skips policy.json pin enforcement. | Rename to `BypassPolicyPins` here and in `CycleRequest` (JSON tag and CLI flag unchanged). |
| 91 | N1 / ch4-INVARIANT | `ComposePhases` is a bool that reads like a command. The kernel guard's downgrade from BLOCK to WARN is invisible, and no test references the field. | Rename to `ViaCompose`. Add `TestKernelGuard_ComposeRunDowngradesBlockToWarn`. |
| 99-100 | ch4-INVARIANT | The overlay applies only under ModelRouting=auto and is left empty under advisory/static. | Already pinned by `TestModelRouting_AdvisoryLogsNotApplies`. No change beyond the struct extraction above. |
| 101 | N1 / ch4-RECOVERABLE | `BudgetScale` does not say what it scales, or that 0 and 1 both mean unscaled (`bridge/budget_scale.go:6`, `scale <= 1`). | Rename to `ArtifactBudgetScale`. In the bridge, add `const unscaledBudget = 1.0` and test `scale <= unscaledBudget`. |
| 112 | N1 / ch4-RECOVERABLE | `BootMS` is the cold REPL-boot slice of `DurationMS`; 0 means no cold boot. | Rename to `ColdBootMS` (JSON tag unchanged). |
| 119 | N1 / ch4-RECOVERABLE | `Reconciled` does not say what was reconciled: the deliverable was trusted after the bridge reported ErrArtifactTimeout. | Rename to `ReconciledAfterTimeout` (JSON tag unchanged). |
| 121 | G25 / ch4-RECOVERABLE | `ModelSource` is a free string. Its only values ("profile", "pin", "advisor") are raw literals in `phases/runner/routing.go:61-66`. | Add `type ModelSource string` with `ModelSourceProfile`, `ModelSourcePin` and `ModelSourceAdvisor` in core, and use them in routing.go. |
| 130-132 | ch4-INVARIANT | `PersonaAvailable() error` has three meanings: nil = available; wraps ErrAgentDocMissing = exclude; any other error = report but keep the phase. Only the missing-persona case is tested. | Add `TestAdvisorPlanInput_PersonaProbeNonSentinelErrorKeepsPhase`. |
| 43, 45-53, 104, 125 | ch4-DESIGN | The reasons are gone: the wrapper-not-var re-export, the aliases that keep call sites unchanged, the JSON tags for the phaseproto subprocess path, and PhaseRunner's in-process/subprocess independence. | Record them under "Re-exports and the phase envelope" in `docs/architecture/packages/internal-core.md`. |

## go/internal/modelcatalog/catalog.go
verdict: MINOR
readability: 4
removed: REDUNDANT=9 RECOVERABLE=4 INVARIANT=2 DESIGN=4 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 38, 46 | G20 | The names `DispatchModel` and `Lookup` do not say how they differ. `DispatchModel` is the live-source-only gate that stops a detect-derived catalog from overriding the manifest. | Rename `DispatchModel` to `LiveModel`. The invariant is already pinned by `TestDispatchModelGatesOnLiveSource`. |
| 17, 34, 36 | G25 / ch4-RECOVERABLE | `Source` is an untyped string and the two constants are untyped, so provenance values are not enforced. | Add `type Source string`, declare `SourceLive Source = "live"` and `SourceDetect Source = "detect"`, and type the field as `Source`. |
| 38, 46 | N1 / ch4-RECOVERABLE | The precondition that `cli` is already a base name (no -tmux/-p suffix) is gone. | Rename the parameter to `baseCLI` in both methods. |
| 16 | N1 / ch4-RECOVERABLE | Empty `Efforts` means "not discovered", not "the CLI has no effort dial". A reader will now take the opposite meaning. | Rename the Go field to `DiscoveredEfforts` (JSON tag `efforts` unchanged). |
| 58 | ch4-INVARIANT | A future `FetchedAt` (clock skew) is treated as fresh. | Already pinned by the `TestIsStale` case "future fetch (clock skew) is fresh". No change. |
| 1, 6, 15, 19 | ch4-DESIGN | Lost: why the manifest fallback stays at the call site (importing bridge would create a cycle), cycle-boundary refresh for reproducibility, `Available` being audit-only, and how `CandidatesHash` lets a refresh skip the classifier call. | Put them in a new `docs/architecture/packages/internal-modelcatalog.md`. |

## go/internal/core/cycle_worktree_teardown.go
verdict: MINOR
readability: 3
removed: REDUNDANT=2 RECOVERABLE=1 INVARIANT=1 DESIGN=0 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 8 | F3 | Two bool flag parameters (`preserve`, `completedNormally`). Every caller passes both from closeout fields. | Replace them with one `type worktreeDisposition int` (`pruneSpent`, `preserveAudited`, `preserveUnadjudicated`), computed once in closeout. |
| 12 | G28 / G19 | `preserve \|\| !completedNormally` is the core rule and has no name. | Extract `func worktreeHoldsUnshippedWork(preserve, completedNormally bool) bool`. |
| 17 | G16 | The log says "cycle ended abnormally" even when `preserve` came from preserveOnVerdict on a cycle that completed normally. | Split the message by cause: `preserving worktree %s: verdict holds audited work` vs `...: cycle ended before adjudication`. |
| 20-22 | ch7 / G29 / ch4-RECOVERABLE | `if cerr != nil { return }` looks like a swallowed error. The reason (keep the path named so resume/reset can still reach it) is gone. The only error `gitWorktree.Cleanup` returns is the project-root refusal, which it already logs. | Make the condition positive: `if o.worktree.Cleanup(projectRoot, wtPath) == nil { o.clearActiveWorktree(wtPath) }`, extracted as `func (o *Orchestrator) pruneSpentWorktree(projectRoot, wtPath string)`. |
| 8 | ch4-INVARIANT | The asymmetry rule is gone: a wrong prune destroys audited work, so both no-prune conditions must stay, and resume must use this same rule. No test references `teardownCycleWorktree`. | Add `TestTeardownCycleWorktree_PrunesOnlyCompletedUnpreservedTrees` and `TestTeardownCycleWorktree_FailedPruneKeepsActiveWorktreeNamed`. |

## go/internal/contextfill/contextfill.go
verdict: MINOR
readability: 3
removed: REDUNDANT=2 RECOVERABLE=2 INVARIANT=1 DESIGN=2 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 24 | G16 (strip defect) | An orphaned half-sentence survived: `// minimal: this is a flat per-TIER stub, not a per-model registry. Every Claude`. The stripper seems to have read `minimal:` as a directive. | Delete the line, and narrow the commentaudit directive matcher to real tool prefixes (`//go:`, `//nolint`, `//lint:`). Move the stub note and upgrade path (key a registry off the resolved model id) to `docs/architecture/packages/internal-contextfill.md`. |
| 28 | G25 / ch4-RECOVERABLE | `200_000` is a magic number, and the fact that every routed Claude tier shares it is not visible. | Add `const claudeContextWindowTokens = 200_000`. |
| 30 | G25 / ch4-RECOVERABLE | `return 0` for an unknown tier is deliberate (FillRatio rejects it as ErrInvalidWindow), but reads like a lazy default. | Add `const unknownWindowSize = 0` and return that. Coverage of every canonical tier is already pinned by `TestWindowSizeForTier` (it iterates `modelcatalog.CanonicalTiers`). |
| 19 | ch4-INVARIANT | The rule that the ratio is not clamped at 1.0 (so overruns stay visible) is gone. | Already pinned by the `TestFillRatio` case "over window is not clamped". No change. |
| 1 | ch4-DESIGN | The rule that only the phasetiming/recordPhaseOutcome importers may use this package, and that the Stage dial is deferred, are lost. | Record them under "Importers" in `docs/architecture/packages/internal-contextfill.md`. |

## go/internal/bridge/clicontrol/clicontrol.go
verdict: MINOR
readability: 4
removed: REDUNDANT=4 RECOVERABLE=2 INVARIANT=1 DESIGN=2 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 14 | N1 / ch4-RECOVERABLE | `EventCleanCtx` is an abbreviation, and "clean" is the wrong verb: the event clears the conversation (/clear, /new). | Rename to `EventClearContext` (wire value `clean_ctx` unchanged, still pinned by `TestEventWireValues`). |
| 22 | N1 / ch4-RECOVERABLE | `Pane` does not say it is the REPL pane captured after the command settled. | Rename to `SettledPane`. |
| 25-27 | ch4-INVARIANT | Implementations must be safe for concurrent use, because the per-cycle prober fans `Do` out across all families. `TestControllerInterface` only checks that the type compiles. | Add `TestTmuxController_DoConcurrentFamilies` in the bridge package, run under `-race`. |
| 1, 9 | ch4-DESIGN | Lost: the Adapter + Strategy rationale (swapping a CLI's command surface is a manifest `controls` edit), the stdlib-leaf dependency direction, and how an Event resolves to a command. | Move them to a new `docs/architecture/packages/internal-bridge-clicontrol.md`. |

## go/internal/cyclestate/state.go
verdict: NEEDS-REFACTOR
readability: 2
removed: REDUNDANT=2 RECOVERABLE=8 INVARIANT=14 DESIGN=4 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 8, 78 | N1 / ch4-RECOVERABLE | `FailedAt` reads like a timestamp but is `[]FailedRecord`. Its wire key is `failedApproaches` in State and `failed_at` in CycleState. There are 22 non-test references. | Rename both Go fields to `FailedApproaches` (JSON tags unchanged). |
| 5, 14 | N4 / ch4-RECOVERABLE | `LastCycleNumber` and `LastAllocatedCycleNumber` differ only by "Allocated". The completed-vs-minted distinction, and the fact that a crashed run uses up its number, are gone. | Rename `LastCycleNumber` to `LastCompletedCycleNumber` (13 references, JSON tag unchanged). Add `TestUpdateState_CrashedRunBurnsAllocatedNumber`. |
| 3 | ch4-INVARIANT | `State` is a subset view, and WriteState drops every key it does not model. A new state.json key added elsewhere will silently disappear. | Add `TestWriteState_DropsUnmodeledKeys` so the loss is explicit and deliberate. |
| 10-11 | ch4-INVARIANT | The setup marker is written by a lossless raw merge, never by WriteState. | Add `TestSetupComplete_RawMergePreservesUnmodeledKeys`. |
| 13 | ch4-INVARIANT | `StateRevision` goes up exactly once per locked read-modify-write, so a gap exposes a writer that bypassed the lock. | Pin with the existing `internal/adapters/storage/updatestate_test.go`, and name its case `TestUpdateState_BumpsStateRevisionExactlyOnce`. |
| 19 | N1 / ch4-RECOVERABLE | `Floors int` is opaque. | Rename to `FloorsPassed`. |
| 22-24 | N4 / ch4-RECOVERABLE | `BatchAccrual` holds `CycleAccruedCostUSD` and is scoped per dispatcher invocation, so three scope words compete. | Rename the type to `DispatcherCostAccrual` (JSON tags unchanged). |
| 28, 32 | N4 | `TS` and `RecordedAt` are two unexplained timestamps on the same record. | Rename `TS` to `OutcomeAt` (or remove one) after checking which writer sets each. |
| 48 | ch4-RECOVERABLE | `CyclesUnpicked` is never incremented and was marked "add no new reads", yet `core/advisor/todos.go:50` reads it. | Keep a machine-read `// Deprecated: use the inbox item's failure_count` directive (staticcheck SA1019 enforces it), or rename it to `LegacyCyclesUnpicked`. |
| 49 | ch4-INVARIANT | An empty `ExpiresAt` means the todo is never auto-pruned. | Add `TestLoopStartPrune_KeepsTodoWithEmptyExpiresAt`. |
| 52 | ch4-INVARIANT | Fields added later are omitempty so that older checkpoints round-trip unchanged. | Extend `TestStateOmitempty` into `TestCycleState_LegacyCheckpointRoundTripsByteIdentical`. |
| 53, 56, 57 | ch4-INVARIANT | Three resume rules are gone: FinalVerdict survives post-audit resume (no PASS default); Shipped is never inferred from main HEAD (sibling lanes move it); PreCycleHEAD is never recaptured on resume. | Add `TestResume_PostAuditKeepsFinalVerdict`, `TestShipped_NotInferredFromMainHEAD` and `TestResume_DoesNotRecapturePreCycleHEAD`. |
| 69-74, 76 | G31 / G34 / ch4-INVARIANT | Five audit-round fields sit apart and their roles are invisible: attempts persist so a resume cannot grant unlimited retries; dispatches are counted before the checkpoint write; RepairActive is true only inside a granted round. | Extract an embedded `AuditRound{RepairAttempts, Dispatches, RepairActive, DeclineReason, FailReasons}` with JSON tags unchanged. Add `TestResume_AuditRepairAttemptsSurvive` and `TestAuditDispatch_BumpedBeforeCheckpointWrite`. |
| 72 | G28 | The presence of `AuditDeclineReason` is itself a flag. It is encapsulated only as the private `retroRouted()` in `core/repair_eligibility.go:55`. | Move it to a method: `func (cs CycleState) AuditRepairDeclined() bool`. |
| 75 | G25 / ch4-RECOVERABLE | A zero `ExplanationDocumentationVersion` grandfathers legacy checkpoints. | Add `const LegacyExplanationDocumentationVersion = 0`. |
| 77, 79 | ch4-INVARIANT | ShipFailReasons makes a ship rejection after a green audit read as a task failure, not a forged verdict. The bookkeeping regrade is bounded to once per cycle, even across a crash-resume. | Regrade is pinned by `core/bookkeeping_regrade_test.go`. Add `TestCoherenceFloor_ShipFailAfterGreenAuditIsTaskFailure`. |
| 12, 58, 68, 70 | ch4-DESIGN | Lost: the triagecap ownership of TriageThroughput, goal carry across quota pauses, the WorktreeBaseSHA normalize on crash-resume, and ShipRecoveryCode seeding the standing-findings brief. | Add a "CycleState field lifecycle" table to `docs/architecture/packages/internal-cyclestate.md`. |

## go/internal/gopkgpattern/gopkgpattern.go
verdict: MINOR
readability: 4
removed: REDUNDANT=1 RECOVERABLE=1 INVARIANT=1 DESIGN=3 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 1 | G16 (strip defect) | The package doc now ends with "…must never disagree about:" and promises a list that was stripped. | End the sentence at "…lints must never disagree about." and move the two-caller list (acssuite run time, evalqualitycheck authoring time) to `docs/architecture/packages/internal-gopkgpattern.md`. |
| 14-26 | G19 / ch4-INVARIANT | The check is deliberately conservative (a false positive demotes a real gate), but it reads as ad-hoc checks. | Extract `hasWhitespace(s)`, `looksLikeFilePath(s)` and `isRelativePackage(s)`. Already pinned by `TestIsPackagePattern`. Add a URL case (`https://…`) to it. |
| 23 | G25 | `len(s) > 2` is a magic length meaning "not the bare ./". | Replace with `s != "./"`. |
| 33 | N1 / ch4-RECOVERABLE | `Key` does not say what kind of key it returns. It also returns "" for both unrecognized input and the whole-module sweep. | Rename to `DirKey` and add `const NoDirKey = ""`, returned from both branches. |
| 11, 29 | ch4-DESIGN | The reason whole-module is special (acssuite maps it to a never-in-scope key) and why recursive sweeps are costly (`-run` does not bound build/load) are lost. | Record them in `docs/architecture/packages/internal-gopkgpattern.md`. |
