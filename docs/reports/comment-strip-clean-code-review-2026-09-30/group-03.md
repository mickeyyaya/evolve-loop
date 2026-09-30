## go/internal/failuregrade/failuregrade.go
verdict: MINOR
readability: 4
removed: REDUNDANT=4 RECOVERABLE=4 INVARIANT=6 DESIGN=2 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 1 | ch4-DESIGN | The package doc now says only "ADR-0048 Slice A classifier". Three things are gone: why a failure is graded instead of a binary abort, that the vocabulary is closed (only 3 evidence-anchored classes grade below Abort), and why substring matching is safe (the input is the orchestrator's own reason codes, never pane text). | Move to `docs/architecture/packages/internal-failuregrade.md` (new file). |
| 9 | ch4-INVARIANT | `TierAbort = iota` makes the zero `Tier` the fail-closed floor. The code no longer says so, and reordering the const block would silently make the zero value non-aborting. | Add `TestTier_ZeroValueIsAbort` (`var t Tier; t == TierAbort`). |
| 10 | N1 / ch4-RECOVERABLE | `TierCorrect` reads as the adjective ("nothing wrong") rather than "re-dispatch the phase with a correction directive". | Rename `TierCorrect` → `TierRedispatchWithCorrection` and keep `String()` = "correct". |
| 12 | ch4-DESIGN | Where `TierRepair` routes is no longer visible: the typed ship repair ladder, i.e. the TOFU SELF_SHA re-pin. | Record it in `internal-failuregrade.md`. |
| 30 | N4 / ch4-RECOVERABLE | `RebuildVerified` does not say what was verified: a reproducible rebuild that came out byte-identical twice. That is what separates "repair" from "real tampering". | Rename `RebuildVerified` → `RebuildIsByteIdentical`. |
| 34-36 | G5 / ch4-RECOVERABLE | All three signatures copy codes owned by other packages. The removed trailing comments were the only link to those owners, so renaming an owner code would silently turn Correct/Repair into Abort. | Use `sigSelfSHATampered = string(shiperr.CodeSelfSHATampered)` (shiperr has no path back to core, so no cycle). Extract `treediff.GuardErrPrefix = "tree-diff guard"` and use it here and in `guards/treediff/treediff.go:38,40` and `core/cyclerun_postreview.go:80`. `deliverable` imports core (a cycle), so pin that one with `TestGrade_RecognizesOwningPackagesCodes` in `package failuregrade_test`, which feeds `deliverable.CodeMissingChallengeToken` and `shiperr.CodeSelfSHATampered` into `Grade`. |
| 39-55 | ch4-INVARIANT (pinned) | The fail-closed rules (unknown reason → Abort, class missing its evidence → Abort, evidence ignored for the missing token) are now implicit. | Already pinned by `TestGrade` (unknown/empty/prose → abort; NOT benign, unverified → abort; token + evidence irrelevant → correct) and `TestTierString` (`Tier(99)` → "abort"). No action. |

## go/internal/modelcatalog/refresh.go
verdict: MINOR
readability: 4
removed: REDUNDANT=4 RECOVERABLE=4 INVARIANT=1 DESIGN=2 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 5 | ch4-DESIGN | Lost: "top" is the frontier default tier, and "high" is an input alias of "deep" (`bridge.translateV1TierKey`) rather than a canonical tier. A reader may add "high" here. | Record it in `docs/architecture/packages/internal-modelcatalog.md`. The canonical set is already pinned by `TestCanonicalTiersIncludesTop` and `TestBuildFromSnapshots_AllCanonicalTiersKeptNonCanonicalDropped`. |
| 7 | ch4-DESIGN | Why `CLISnapshot` exists instead of `setup.CLIStatus` is lost: it keeps modelcatalog a leaf with no setup→bridge import chain, and the command layer adapts. | Record it in `internal-modelcatalog.md`. |
| 9 | N1 / ch4-RECOVERABLE | `Ready` has lost its meaning: installed AND authed, the only CLIs that get cataloged. | Rename `Ready` → `IsUsable`. |
| 12 | N1 | `Efforts` is ambiguous; it means the reasoning-effort ladder. | Rename `Efforts` → `ReasoningEfforts` (in the snapshot, and in `CLIEntry` if the JSON tag is kept). |
| 32-35 | ch4-INVARIANT | An empty `Source` defaults to `SourceDetect` (unknown provenance counts as non-authoritative), and no test pins it. | Add `TestBuildFromSnapshots_EmptySourceDefaultsToDetect` and extract `func (s CLISnapshot) sourceOrDetect() string`. |
| 17-39 | F1 / G34 / ch4-RECOVERABLE | The inclusion rule (ready, named, at least one canonical-tier model with a non-empty id) now appears only as scattered `continue`s mixed with tier projection and source defaulting. The note that this is the bootstrap producer and a live `/model` source will produce the same shape is also lost. | Extract `func (s CLISnapshot) canonicalTierModels() map[string]string` and `func (s CLISnapshot) isCatalogable() bool` so the loop reads include → project → store. Put the producer note in `internal-modelcatalog.md`. |
| 41-56 | N7 / G20 / ch4-RECOVERABLE | `MergeFallbacks` does not say the direction (prior → next), that it only fills gaps, that it never resurrects a CLI absent from next, or that it deep-copies chains. The body nests 4 levels (for/if/if/for). | Rename `MergeFallbacks` → `CarryForwardTierFallbacks(prior, next Catalog) Catalog` and extract `func cloneFallbackChains(chains map[string][]string) map[string][]string`. The behavior is already pinned by `TestMergeFallbacks_PreservesTierFallbacks` and `TestMergeFallbacks_FreshChainWins`. |

## go/internal/subagent/recursion.go
verdict: NEEDS-REFACTOR
readability: 3
removed: REDUNDANT=1 RECOVERABLE=5 INVARIANT=2 DESIGN=2 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 11-14 | G16 (strip defect) | The kept directive lines are now sentence fragments. L12 reads "// scan (SSOT §IPC-protocol-allowed) — same pattern as FanoutWorkerTokenEnv." and L14 reads "// scan (SSOT §IPC-protocol-allowed).". The stripper keeps every line matching `.*IPC-protocol-allowed` (`commentaudit/equivalence.go:84`) but deleted the middle line of the same sentence. | Collapse each to one trailing marker, `// SSOT IPC-protocol-allowed`, as `ipcenv.go` does. Fix `commentaudit strip` to keep or drop a directive's whole comment group, and add `TestStrip_DirectiveGroupKeptWhole`. |
| 13, 15 | G16 / ch4-RECOVERABLE | `"EVOLVE_" + "DISPATCH_DEPTH"` is now an unexplained split literal. It existed only to dodge the flagreaders standalone-literal scan. | Move both keys to the IPC single source of truth: `ipcenv.DispatchDepthKey = "EVOLVE_DISPATCH_DEPTH" // SSOT IPC-protocol-allowed` and `ipcenv.FanoutWorkerTokenKey`, unsplit, like `ipcenv.FleetKey`. This also replaces the literal "EVOLVE_FANOUT_WORKER_TOKEN" repeated at `dispatchparallel.go:176` (G5). |
| 15 | ch4-DESIGN | Lost: the worker token carries the parent-dictated challenge token into `RunRequest.ChallengeTokenOverride`, which is the per-worker provenance boundary. | Record it in `docs/architecture/packages/internal-subagent.md` (exists). |
| 16 | ch4-DESIGN | Lost: why the cap is 3. Normal fan-out → worker nesting is depth 1, and 3 is a backstop against loops. | Record it in `internal-subagent.md`. |
| 25-26 | G25 / ch4-RECOVERABLE | Returning `0` for absent, invalid or negative input means "top-level", but a bare 0 does not say so. | Add `const topLevelDispatchDepth = 0` and return it. |
| 31, 38 | N7 / G20 / ch4-RECOVERABLE | `enforceDispatchDepth` and `enforceChildDispatchDepth` do not say which fence each is: the Run() entry, or DispatchParallel's fail-fast before it spawns doomed workers. | Rename to `rejectRunBeyondDepthCap(depth)` and `rejectFanoutBeyondDepthCap(parentDepth)`. Fail-fast is pinned by `TestDispatchParallel_RecursionDepthCap`. |
| 42-44 | ch4-INVARIANT | Nothing tests the POSIX `'\''` escape. The existing quoting test covers spaces only, never an embedded single quote. | Add `TestShellQuote_EscapesEmbeddedSingleQuote` (a path like `/tmp/it's` round-trips through `sh -c`) and rename `shellQuote` → `posixSingleQuote`. |
| 46 | F1 (8 params) / ch4-RECOVERABLE | 8 positional parameters, 5 of them adjacent strings. The removed parameter glossary was the only guard against swapping workspace, promptPath and workerToken. | Introduce `type workerRecursionSpec struct{ Bin, ParentAgent, Subtask string; Cycle, ChildDepth int; Workspace, PromptPath, WorkerToken string }` and `func (s workerRecursionSpec) command() string`. |
| 48 | G25 / G16 | `CLAUDECODE_TYPE=` is a bare fragment. Its job is to clear the host marker so the child is detected as nested and gets no inner sandbox wrap. | Add `const clearHostMarkerAssignment = "CLAUDECODE_TYPE="`. The behavior is pinned by `TestBuildWorkerRecursionCommand` and `TestRecursionStaysNested_NoInnerWrap`. |
| 50 | ch7 / ch4-INVARIANT (false claim) | The removed comment said role and subtask are "regex-constrained, safe unquoted". Role is (`agentRolePattern`, `run.go:107`). Subtask is not: `extractParallelSubtasks` accepts any `"name"` value except a quote (`dispatchparallel.go:266`), so a profile subtask name containing `;`, `$(` or a space is interpolated unquoted into `sh -c`. | Validate in `extractParallelSubtasks` with `subtaskNamePattern = regexp.MustCompile("^[a-z0-9][a-z0-9-]*$")`, or quote the `<role>-worker-<subtask>` argument. Add `TestDispatchParallel_RejectsSubtaskNameWithShellMetachars`. |

## go/internal/budgethistory/budgethistory.go
verdict: MINOR
readability: 4
removed: REDUNDANT=5 RECOVERABLE=5 INVARIANT=3 DESIGN=2 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 1 | ch4-DESIGN | Lost: why duration and not dollars (subscription CLIs report $0, the reason the old --budget-usd cap was removed); the read-only, degrade-gracefully contract; that a zero `Throughput` means "pace unknown, fall back to the policy floor"; and the leaf-dependency rule. | Record it in `docs/architecture/packages/internal-budgethistory.md` (new file). |
| 16-23 | G16 / ch4-RECOVERABLE | Three zero sentinels are now unexplained. `MedianCostUSD` = 0 with `CostSampleCount` = 0 means "no data", not $0. `MedianTokensPerCycle` = 0 means unknown. `SampleCount` = 0 means pace unknown. Also lost: cost is display-only, and `CyclesPerHour` is per lane. | Add query methods `IsPaceKnown() bool` (`SampleCount > 0`), `HasCostData() bool` (`CostSampleCount > 0`) and `HasTokenData() bool` (`MedianTokensPerCycle > 0`). Rename `CyclesPerHour` → `CyclesPerHourPerLane` and `MedianTokensPerCycle` → `MedianGrossTokensPerCycle`. Already pinned by `TestCollect_CostSampleCountDistinguishesNoDataFromZero`, `TestCollect_LegacyTimingWithoutTokensYieldsZeroMedian` and `TestCollect_AllMissingIsZeroValue`. |
| 30 | G5 | The inline `.evolve/runs/cycle-%d` join duplicates `paths.RunWorkspace`, which calls itself the one home of that layout (ADR-0103 unit 09). `paths` is a leaf that imports only ipcenv, so there is no cycle. | `ws := paths.RunWorkspace(projectRoot, c)`. |
| 25-60 | F1 / G34 | `Collect` (36 lines) mixes path building, three evidence reads with different skip rules, and statistics. | Extract `func readCycleEvidence(ws string, cycle int) (cycleEvidence, bool)` and `func summarize(durations []int64, costs []float64, tokens []int64) Throughput`. |
| 31-34 | ch7 / ch4-RECOVERABLE | `if err != nil { continue }` now reads as a swallowed error. It is deliberate: an unreadable timing log counts as absent evidence. | The extracted `readCycleEvidence` returns `ok=false`, so the name carries the intent. |
| 40-42 | ch4-INVARIANT | A zero-gross cycle is skipped so one legacy log cannot drag a mixed cohort's median down. Only the all-legacy case is tested. | Add `TestCollect_MixedCohortSkipsZeroTokenCycles` (gross 0/1000/2000 → median 1500, not 1000). |
| 43-45 | ch7 / N-shadow | `sum` is shadowed (timing rollup, then cost summary). Every `SummarizeCycle` error (ErrNoWorkspace, glob failure) is swallowed, although only `cyclecost.ErrNoLogs` means "no cost data". | Rename the inner variable to `cost` and skip only on `errors.Is(err, cyclecost.ErrNoLogs)`. Surface other errors, for example with a `Throughput.CostReadErrors int` counter. |
| 62 | ch4-INVARIANT (pinned) | GROSS deliberately counts cache read and write, because cache tokens are billed quota. | Already pinned by `TestCollect_MedianTokensPerCycle` ("220 would mean cache tokens were dropped"). Optionally rename `grossTokens` → `grossBilledTokens`. |
| 78-79 | ch4-INVARIANT | The midpoint form `lo + (hi-lo)/2` avoids the int64 overflow of `(lo+hi)/2`. Without the comment a simplifier would revert it. | Add `TestMedian_EvenCountNearMaxInt64DoesNotOverflow` (`{MaxInt64-2, MaxInt64}` → `MaxInt64-1`). |

## go/internal/paths/paths.go
verdict: MINOR
readability: 3
removed: REDUNDANT=10 RECOVERABLE=3 INVARIANT=2 DESIGN=8 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 1 | ch4-DESIGN | Lost: the per-field resolution order (override env var → PluginRoot- or ProjectRoot-derived path → cwd) and the rule that CapabilityDir never follows the adapters override (it mirrors bash REAL_ADAPTERS_DIR). | Record it in `docs/architecture/packages/internal-paths.md` (new file). |
| 14-32 | G16 | Every field is followed by a blank line, left over from the removed field comments, and the struct no longer shows its three groups. | Regroup into three blank-separated blocks: roots (ProjectRoot, PluginRoot, EvolveDir), `.evolve` files (StateFile, CycleStateFile, LedgerFile), plugin dirs (ProfilesDir, AdaptersDir, CapabilityDir). |
| 17 | ch4-DESIGN | Lost: PluginRoot is the install location. It equals ProjectRoot in a checkout and differs for an installed plugin. | Record it in `internal-paths.md`. |
| 21, 23 | N1 / ch4-RECOVERABLE | `StateFile` vs `CycleStateFile`: lost that the first is persistent batch-scope state and the second is transient per-cycle state. | Rename `StateFile` → `BatchStateFile`. |
| 39-64 | G25 / G5 / G34 | Five env keys are inline literals while the sixth uses `ipcenv.CycleStateFileKey`. The `v := lookupEnv(k); if v == "" { v = fallback }` shape repeats 5 times, and `Resolve` is 45 lines. | Add named consts `envProjectRoot`, `envPluginRoot`, `envProfilesDirOverride`, `envAdaptersDirOverride`, `envLedgerOverride`, and extract `func envOr(lookupEnv func(string) string, key, fallback string) string`. |
| 55-59 | G16 / ch4-RECOVERABLE | `capabilityDir := filepath.Join(pluginRoot, "adapters")` is identical to the adapters default two lines up and reads as a copy-paste bug. The reason it ignores the override is gone. | Add an explanatory variable `installedAdaptersDir := filepath.Join(pluginRoot, "adapters")`; set `adaptersDir := envOr(lookupEnv, envAdaptersDirOverride, installedAdaptersDir)` and `CapabilityDir: installedAdaptersDir`. Already pinned by `TestResolve_CapabilityDirAlwaysPluginRoot`. |
| 34 | G31 / ch4-DESIGN | A temporal coupling is now invisible: relative fields are bound to the cwd at Resolve time, so an `os.Chdir` before a write moves the write. Also lost: Resolve is total and does no validation. | Record it in `internal-paths.md`. |
| 81-95 | ch4-INVARIANT (pinned) | Lost why the lane override applies only inside its own evolve dir (spawned processes, test binaries included, inherit it) and that a relative override counts as outside. `isInside` also rejects the dir itself. | Already pinned by `TestCycleStateFileFor_TheOverrideGovernsOnlyItsOwnEvolveDir` (it includes the relative and dir-itself cases). Rename `isInside` → `isStrictlyBelow`. |
| 98 | ch7 / ch4-RECOVERABLE | `cwd, _ := os.Getwd()` now reads as a swallowed error. The policy is deliberate: an empty cwd, and callers that care call `Resolve`. | Extract `func cwdOrEmpty() string` so the name states the policy. |
| 104 | G20 / ch4-DESIGN | The name promises an absolute path, but `AbsoluteRoot` returns `p` unchanged after a warning. The reason is lost too: relative roots diverge between the worktree agent's cwd and the bridge's cwd (the cycle-119 ExitArtifactTimeout). | Rename → `AbsoluteRootOrWarn` and record the reason in `internal-paths.md`. Already pinned by `TestAbsoluteRoot_WarnsAndReturnsInputOnError`. |
| 121 | ch4-DESIGN (stale claim) | The removed comment called `RunWorkspace` "the ONE home" of `.evolve/runs/cycle-<n>`, but at least 9 inline copies remain: `budgethistory.go:30`, `contextfillcorrelate.go:241`, `acssuite.go:355`, `core/reset.go:84`, `cmd_loop_control.go:216`, `cmd_loop_blockerbreaker.go:45`, `cmd_acs.go:19,35`, `cmd_swarm.go:64`. | Add `RunWorkspaceIn(evolveDir, cycle)` for the evolveDir-based callers and migrate all of them. Guard with `TestRunWorkspaceLayout_HasOneHome`, an AST scan for a `"runs"` + `"cycle-"` join outside paths. |

## go/internal/routingtest/spec.go
verdict: NEEDS-REFACTOR
readability: 2
removed: REDUNDANT=9 RECOVERABLE=31 INVARIANT=3 DESIGN=3 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 1 | ch4-DESIGN | Lost: a scenario is pure data assembled from Bricks and Matrix, which the engine interprets against a Surface. Also lost: why this is a non-`_test` package (router_test and core_test import it; no production code does, so there is no cycle). | Record it in `docs/architecture/packages/internal-routingtest.md` (new file). |
| 13-14 | N1 / ch4-RECOVERABLE | `PureKernel` and `FullOrchestrator` no longer say what runs: one `router.Route()` decision, or an end-to-end `core.Orchestrator.RunCycle`. | Rename → `SurfaceRouteDecision` and `SurfaceRunCycle`. |
| 19-45 | G34 / G16 / ch4-RECOVERABLE | `ScenarioSpec` is a flat 20-field bag. The removed section markers were the only statement that Current/Verdict/Completed apply to PureKernel only and Verdicts/FailedAt/LastCycle to FullOrchestrator only, so a spec can silently set fields its Surface ignores. | Split into nested structs: `Routing RoutingConfigSpec`, `World WorldSpec{Signals, Env, StrictAudit}`, `Kernel KernelInput{Current, Verdict, Completed}`, `Cycle CycleInput{VerdictByPhase, FailedAt, LastCycle}`. Add `TestEngine_RejectsFieldsForTheOtherSurface`. |
| 19 | ch4-INVARIANT | Lost: a zero-value spec resolves to safe defaults in `engine.go` (Stage Off, default spine, tester trigger on acs_red>0). | Add `TestBuildConfig_ZeroSpecResolvesSafeDefaults`. |
| 29 | G25 | "0 → default 4" is gone here, and `engine.go:67-68` holds a bare `4`. | Add `const defaultMaxInsertions = 4` in `engine.go`. |
| 33 | N1 / ch4-RECOVERABLE | `Strict` does not say which strictness. It is policy.json `workflow.strict_audit`, threaded into `RouteInput.Strict`. | Rename → `StrictAudit`. |
| 39 | N1 / ch4-RECOVERABLE | The map key and value are unstated: phase name → fakeRunner verdict. | Rename `Verdicts` → `VerdictByPhase`. |
| 47-67 | N1 / N4 / ch4-RECOVERABLE | The signal fields lost which phase produces them. Scout produces CycleSize, GoalType and DeliverableKind; triage produces TriageSize and TriageDeliverableKind, and triage is authoritative (ADR-0099). The value set of SeverityMax (LOW, MEDIUM, HIGH, CRITICAL) is also gone. | Rename `CycleSize`→`ScoutCycleSize`, `TriageSize`→`TriageCycleSize`, `GoalType`→`ScoutGoalType`, `DeliverableKind`→`ScoutDeliverableKind`. Give SeverityMax a `Severity` type with consts `SeverityLow`…`SeverityCritical`. Dual rendering is already pinned by `TestSignalSpec_DualRenderingAgree`. |
| 70-73 | N1 / ch4-RECOVERABLE | Lost: both maps are keyed by `RouteInput.Current`, and `PlanError` forces the planner to fail (the degrade-to-static-spine path). | Rename → `ProposalByCurrentPhase`, `FailOnCurrentPhase`, `ForcePlannerError`. |
| 76, 80 | G20 / ch4-RECOVERABLE | `active()` really means "use the LLMProposal strategy". `hasPlan()` is true when there is no plan but PlanError is set. | Rename → `usesLLMProposer()` and `exercisesPlanner()`. |
| 87-104 | G16 / ch4-INVARIANT | "Empty fields are not asserted" is gone, so a reader cannot tell that a zero value means "skip". | Add `TestEngine_EmptyExpectFieldsAreNotAsserted` and split into `KernelExpect` and `CycleExpect`. |
| 91, 93 | N4 / ch4-RECOVERABLE | `Clamps` holds clamp rule names, and `Justification` is a substring match (`engine.go:108`). | Rename → `ClampRuleNames` and `JustificationContains`. |
| 95, 97-99, 101 | N4 / ch4-RECOVERABLE | The match semantics are gone. PhaseSequence is an exact match, skipped when nil. DecisionInserts and DecisionClamps mean "some logged decision". RoutingLedgerMin is a lower bound. ProposeAt is an exact set compare (`sameSet`). | Rename → `ExactPhasesRun`, `SomeDecisionInserts`, `SomeDecisionClamps`, `MinRoutingDecisionEntries`, `ProposerPhaseSet`. |
| 96 | ch4-RECOVERABLE (warning) | The PhasesAbsent caveat is lost. Since ADR-0044 C1, PhasesRun includes aborted-but-dispatched phases, so the check is valid only for phases that were never dispatched. | Rename → `PhasesNeverDispatched`. |
| 100 | G20 (name lies) | `engine.go:184` checks `RetroPrefix` with `strings.Contains`, not `HasPrefix`, so the name and the old comment ("has this prefix") were both wrong. | Rename → `RetroDecisionContains`, or switch the engine to `strings.HasPrefix`. |
| 103 | G25 | `Invariants []string` holds names looked up in `invariants.go`. A typo fails only at run time (`t.Fatalf` "unknown invariant"). | Add `type InvariantName string` with one const per registered invariant. |

## go/internal/bridge/clicontrol/usage.go
verdict: MINOR
readability: 4
removed: REDUNDANT=7 RECOVERABLE=2 INVARIANT=5 DESIGN=3 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 10 | ch4-DESIGN | The file-level design note is gone, and the package has no doc line. It said: the command is resolved from the manifest `controls` table (`/status` for codex, `/usage` for claude and agy); each CLI reports in its own direction (codex remaining, claude consumed); adding a CLI means a parser plus a manifest entry. Also: no production code calls `QueryUsage`, `ParseUsage` or `MinPercentLeft` yet, so this note was the only statement of purpose. | Record it in `docs/architecture/packages/internal-bridge-clicontrol.md` (new file). |
| 14 | ch4-DESIGN | Lost: ResetHint is kept as raw text on purpose, because formats and timezones vary per CLI and locale and a mis-parsed time is worse than an unparsed one. | Record it in `internal-bridge-clicontrol.md`. |
| 21 | N1 / ch4-RECOVERABLE | `Raw` does not say what it holds: the captured pane, the only evidence left on a parse miss. | Rename → `RawPane`. |
| 26 | G25 / ch4-RECOVERABLE | `-1` is a bare sentinel meaning "unknown", deliberately distinct from 0 ("exhausted"). | Add `const UnknownPercentLeft = -1`. Already pinned by `TestUsage_MinPercentLeftPicksTheTightestWindow`. |
| 28 | N-shadow | The local `min` shadows the Go 1.21 builtin (go.mod is 1.23). | Rename → `tightest`. |
| 39-42 | ch4-DESIGN | agy's absence is deliberate (no live `/usage` capture yet, and a guessed parser would fabricate numbers), but now looks like an oversight. | Record it in `internal-bridge-clicontrol.md`. An agy pane staying unparsed is already pinned by `TestParseUsage_UnrecognisedPaneIsHonestlyUnparsed`. |
| 44, 54-56 | G16 / G25 / ch4-INVARIANT (pinned) | The dense regex lost its sample row, and `m[1]`, `m[2]`, `m[3]` are positional magic indices. | Use named groups `(?P<window>…)`, `(?P<pct>…)`, `(?P<reset>…)` read through `codexUsageLine.SubexpIndex("pct")`. The percent-required rule is already pinned by `TestParseCodexUsage_BodilessHeadingYieldsNoWindow`. |
| 49-51, 69-71, 75 | G5 / G25 | The 0..100 validation is duplicated, and the literal 100 appears three times. | Extract `func parsePercent(s string) (int, bool)` with `const fullPercent = 100`, and `func percentLeftFromUsed(used int) int`. The inversion is already pinned by `TestParseUsage_Claude_InvertsConsumedToRemaining`. |
| 80, 102 | G5 / ch7 | `ParseUsage` returns an `ok` that duplicates `u.Parsed`, and `QueryUsage` discards it (`u, _ :=`), which now reads like a swallowed result. | Return only `Usage` from `ParseUsage`, since callers branch on `.Parsed`. "A nil error with Parsed=false is not a failure" is already pinned by `TestQueryUsage_UnreadablePaneIsNotAnError`. |
| 96 | ch7 | `fmt.Errorf` has no verb and no sentinel a caller can match. | Add `var ErrNilController = errors.New("clicontrol: QueryUsage requires a Controller")`. |
| 100 | ch7 / ch4-INVARIANT (pinned) | The controller error goes back with no context. The removed note justified leaving it unwrapped so that `errors.Is(err, ErrUnsupported)` works, but `%w` keeps that working. | `fmt.Errorf("clicontrol: query usage for %s: %w", family, err)`. `TestQueryUsage_PropagatesUnsupportedForFamiliesWithoutTheControl` keeps the `errors.Is` contract honest. |

## go/internal/ciparity/composedgates.go
verdict: MINOR
readability: 4
removed: REDUNDANT=1 RECOVERABLE=1 INVARIANT=1 DESIGN=2 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 1 | ch4-DESIGN | The file header is gone. It said these are the composed-tree gates for the trivial-rebase audit carry-forward (merge ladder RUNG 0), that gates bind to the TREE, and that they re-run through the ADR-0069 CI-parity runners rather than a new gate implementation. "Composed" is now undefined in the file. | Record it in `docs/architecture/packages/internal-ciparity.md` (new file). |
| 3 | ch4-DESIGN / G16 | `RequiredComposedGates` is an exported mutable slice, so any importer can append to or overwrite the gate floor the ship fast path trusts. Its role as the full native gate set that must record "pass" is gone. | Make it `func RequiredComposedGates() []string` returning a fresh copy, and update the callers at `cmd_composition_wiring.go:57-58`. |
| 8 | G25 / G5 | The producer and consumer spell one contract twice: `"pass"` here, and `"pass"`/`"fail"` at `cmd/evolve/cmd_composition_wiring.go:82,85`. | Add `const GateStatusPass = "pass"` and `GateStatusFail = "fail"` in ciparity, or constructors `PassedGate()` and `FailedGate(tail string) GateOutcome`. |
| 5-13 | ch4-INVARIANT (pinned) | A nil result, not an empty slice, means fully green, and callers test `missing != nil` (`core/identity_carry_forward.go:47`, `adapters/ledger/composition.go:94`, `phases/ship/composition.go:33`). | Already pinned by `TestMissingComposedGates_FullNativeGateSet` ("must return nil"). No action. |
| 17 | N4 / ch4-RECOVERABLE | `Tail` is set only on failure; it holds the gate's last output lines, which a decline quotes. | Rename → `FailureTail`. |
