## go/internal/bridge/inbox/inbox.go
verdict: MINOR
readability: 4
removed: REDUNDANT=5 RECOVERABLE=4 INVARIANT=1 DESIGN=3 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 26 | G9/ch4-RECOVERABLE | `Seq` has no Go writer or reader: writer.go never sets it and the cursor never reads it. The removed note "best-effort writer hint; the reader's cursor is authoritative" was the only sign that the field is inert. | Delete `Seq`. The wire key `seq` is omitempty, so on-disk lines don't change. |
| 27 | N1/ch4-RECOVERABLE | The abbreviation `TS` hides that this is the RFC3339 UTC mint time (writer.go:15). | Rename `TS` → `MintedAt`, keeping `json:"ts"`. |
| 30 | G25/ch4-RECOVERABLE | Source's vocabulary (`"cli" \| "observer" \| custom`) lived only in the comment. The literals are spelled out at cmd/evolve/cmd_bridge.go:164 and internal/phaseobserver/phaseobserver.go:146. | Add `const SourceCLI = "cli"` and `const SourceObserver = "observer"` in inbox, and use them at both call sites. |
| 32 | N1/ch4-DESIGN | `CorrID` is an abbreviation, and nothing now says who echoes it (the driver's inject_applied/idle_reached breadcrumbs, the producer's answer-span bracketing, ADR-0037). | Rename to `CorrelationID` (`json:"corr_id"` unchanged). Move the echo contract to docs/architecture/packages/internal-bridge-inbox.md. |
| 34 | ch4-INVARIANT | The bound on `DeferCount` lives in another package (`maxInjectDefer = 10`, bridge/tmux_inject.go:50). Only the first re-queue is tested (TestInjectEnvelope_CommandMidTurn_Defers). | Write `TestInjectEnvelope_CommandMidTurn_DropsAtMaxInjectDefer`. |
| 41 | G5/G25 | The default agent name `"agent"` is duplicated at internal/bridge/engine.go:289. The removed "mirrors engine.go" was the only link between the two copies. | Export `const DefaultAgent = "agent"` from inbox and use it at engine.go:289. |
| 6 | ch4-DESIGN | The per-Kind injection semantics (idle-gated vs ESC-first vs raw send-keys, the "## Rules" prefix for system_rule, the keystroke operator hatch) are no longer stated anywhere near the type. | Move the Kind semantics table to docs/architecture/packages/internal-bridge-inbox.md. |

## go/internal/bridge/channel/enablement.go
verdict: MINOR
readability: 3
removed: REDUNDANT=1 RECOVERABLE=0 INVARIANT=1 DESIGN=1 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 5 | N4/G20 | `channel.ResolveStage` also normalizes the fatal-pane dial (bridge/fatalpane.go:29), which is not a channel stage. Without the comment, nothing says this is the shared recovery-stage normalizer. | Rename to `ResolveRecoveryStage`. |
| 7-19 | G25 | The stage words `"shadow"`, `"off"` and `"enforce"` are bare literals repeated 6 times. | Add `const StageOff = "off"`, `StageShadow = "shadow"`, `StageEnforce = "enforce"`. |
| 9 | G25/ch4-RECOVERABLE | `case "0"` makes no sense without the lost note that `config.StageOff.String()` returns "0", the word the composition root forwards. | Add `const configStageOffWord = "0"`. Write `TestResolveStage_AcceptsEveryConfigStageString`, which loops over the config.Stage values. |
| 13 | ch4-INVARIANT | The fail-closed rule "a typo never enables a kill path" is kept by TestResolveStage ("typo", "yes" and "1" all give off). But `config.StageAdvisory.String()` returns "advisory", which also falls into this branch and silently becomes "off". | Decide the intended result and pin it with `TestResolveStage_AdvisoryResolvesOff`, or add an explicit case. |
| 1 | ch4-DESIGN | The file comment is gone: the channel has no dial of its own and rides EVOLVE_PHASE_RECOVERY (ADR-0045's single dial), so enforce means on. | Record this in docs/architecture/packages/internal-bridge-channel.md. |

## go/internal/treestate/treestate.go
verdict: MINOR
readability: 4
removed: REDUNDANT=3 RECOVERABLE=1 INVARIANT=3 DESIGN=1 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 21 | G25/ch4-RECOVERABLE | In `exitCode > 1`, the lost inline note said rc=1 means "differences exist" and is normal. | Add `const gitDiffDifferencesExit = 1` and write `if exitCode > gitDiffDifferencesExit`. |
| 24-26 | G16/ch7 | `_, _ = h.Write(...)` reads like a swallowed error. | Replace with `sum := sha256.Sum256([]byte(buf.String())); return hex.EncodeToString(sum[:]), nil`. |
| 15 | ch4-INVARIANT | The exit semantics: {0,1} are hashed, >1 gives a RunError with Err nil, a runner failure gives Err set. These are kept by TestSHA_TableSeam (differences-exit1-hashed, fatal-exit128, runner-error) and TestRunError_MessagesAndUnwrap. | None; the tests exist. |
| 1 | ch4-DESIGN | Lost: SHA is the single fingerprint that both the commit-gate attestation reader and the audit-binding verifier (phases/ship) hash byte-identically. | Record this in docs/architecture/packages/internal-treestate.md. |

## go/internal/phaseconfig/phaseconfig.go
verdict: MINOR
readability: 4
removed: REDUNDANT=5 RECOVERABLE=4 INVARIANT=1 DESIGN=4 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 32 | G9/ch4-DESIGN | `SwarmWorkers` has no production reader: `.SwarmWorkers` appears nowhere outside tests. The removed contract ("0 = EVOLVE_SWARM_CONCURRENCY default, >1 fans out, writers need disjoint ownership") described behavior no code implements. | Either wire it into the swarm harness with `TestSwarmWorkers_ZeroUsesGlobalDefault`, or delete the field. |
| 33, 63 | N7/G9/ch4-RECOVERABLE | `Prompt` and `PromptBody` don't say "inline; empty means load agents/<agent>.md". The runner's interface calls the same shape `InlinePromptBody()` (runner.go:59). `PromptBody` has no production caller, since registrar.go:111 reads `cfg.Prompt` directly. | Rename the field `Prompt` → `InlinePrompt` (`json:"prompt"`). Rename `PromptBody` → `InlinePromptBody` and call it from registrar.go:111, or delete it. |
| 39 | G5/G25/ch4-RECOVERABLE | The "evolve-" prefix is duplicated: phasespec.go:127 adds it, and phaseconfig.go:39, phasespec/clamp.go:19 and runner/preparation.go:95 each strip it. | Add `const AgentNamePrefix = "evolve-"` and `func (s PhaseSpec) ProfileName() string` in phasespec. Delete `PhaseConfig.ProfileName`, which then comes through the embedding. |
| 42 | N7/ch4-INVARIANT | ToProfile is deliberately partial: MaxTurns, ParallelEligible and OutputArtifact stay zero, and the Registrar overlays them. The name promises a complete Profile. | Rename to `DispatchProfile()`. Write `TestToProfile_UnmodeledFieldsStayZero`, asserting ParallelEligible=false (the single-writer invariant). |
| 1 | ch4-DESIGN | Lost: the option-B decomposition (Spec, ToProfile and PromptBody feed the unchanged loaders) and the layering rule "MUST NOT import core, runner, llmroute". | Record in docs/architecture/packages/internal-phaseconfig.md. Pin the layering with `TestImportGraph_PhaseconfigImportsOnlyPhasespecProfiles` (same pattern as launchoutcome/importgraph_test.go). |
| 14, 26 | ch4-DESIGN | Dispatch deliberately leaves out the fields PhaseSpec already carries. SystemPrompt is the persona carried in-band, so there is no system_prompt_file. | Record in the same package doc. |

## go/internal/bridge/launchoutcome/exit.go
verdict: MINOR
readability: 3
removed: REDUNDANT=9 RECOVERABLE=4 INVARIANT=3 DESIGN=2 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 25-32 | N1/ch4-RECOVERABLE | `name` and `causeCode` hold identical strings in 10 of 11 rows. Nothing now says one is the class name and the other is the attempt ledger's (llm-calls.ndjson) cause. `sentinel` doesn't say it is the port error the Outcome wraps. | Rename `name`→`className`, `causeCode`→`ledgerCause`, `sentinel`→`wrappedSentinel`, `sentinelOnCancel`→`wrapsOnlyWhenCtxCancelled`. |
| 50 | G31/N7/ch4-RECOVERABLE | `classOf(ExitOK)` silently returns `driverErrorClass`. Both callers (classify.go:21 and :73) must check for ExitOK first, and that precondition was stated only in the removed doc. | Rename to `failureClassOf`, or return `(exitClass, bool)` and move the ExitOK check inside. |
| 12 | N4/ch4-RECOVERABLE | `ExitCostLeak` doesn't say "forbidden env-var leak (ANTHROPIC_API_KEY…)". | Rename to `ExitForbiddenEnvLeak` here and in bridge/exitcodes.go:7. The pin is TestExitCodes_HostAliasesAreTheLeafValues. |
| 18 | N4/ch4-RECOVERABLE | `ExitRequireFullUnmet` has a different name from its row's class name "required_tier_unavailable" (line 42), so one exit has two names. | Rename to `ExitRequiredTierUnavailable` here and in bridge/exitcodes.go:13. |
| 9-21 | G5/ch4-INVARIANT | The exit numbers are spelled twice on purpose: here and in bridge/exitcodes.go, which acs/cycle1580 regex-scans. A reader now sees plain duplication. | Kept by TestExitCodes_HostAliasesAreTheLeafValues (bridge/launch_outcome_seam_test.go:13) and TestExitCodes_NumericContractUnchanged. Record why in docs/architecture/packages/internal-bridge-launchoutcome.md. |
| 23, 45 | N4/ch4-INVARIANT | -1 is also what Go reports for a failed Start, not only for a signal death, and it is transient only under a cancelled ctx. | Rename to `ExitSignalOrStartFailure`. Kept by TestClassify_SignalDeath_TransientOnlyWhenCtxCancelled. |
| 43-44 | ch4-INVARIANT | Why 124 is transient but 127 deliberately fails loud: a missing CLI is an environment defect, and the family fallback sees the raw 127. | Kept by TestClassify_TransientSet_IsExactly80_85_86_124. Record the reason in the package doc above. |

## go/internal/bridge/driver.go
verdict: MINOR
readability: 3
removed: REDUNDANT=6 RECOVERABLE=1 INVARIANT=1 DESIGN=3 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 13 | ch4-RECOVERABLE | `(int, error)` hides the two failure channels: a CLI that ran and failed returns non-zero with err == nil, and err is only for harness failures. | Name the results in the interface: `Launch(ctx, cfg, deps) (exitCode int, harnessErr error)`. |
| 16 | ch4-INVARIANT | CLIPreflight is optional (type-asserted) and best-effort: launch.go:168-171 logs the error and carries on. No test pins "a failed preflight still launches". | Write `TestLaunch_PreflightErrorLogsAndStillLaunches`. |
| 51-52 | G9/G16 | The "codex" and "agy" rows are dead. Both are registered drivers, so DriverFor's pass-through wins (TestDriverFor_BareAndDriverNames pins codex→codex and agy→agy). A reader will believe codex maps to codex-tmux. | Delete the two rows. |
| 50 | ch4-DESIGN | gemini→claude-tmux looks like a bug without the lost reason (gemini.sh's HYBRID mode delegated to claude.sh). | Pinned by TestDriverFor_BareAndDriverNames. Record the reason in docs/architecture/packages/internal-bridge.md. |
| 55 | G20/N7 | `DriverFor` returns a driver name, not a Driver; compare `LookupDriver`, which does return a Driver. | Rename to `DriverNameFor` (5 call sites). |
| 10 | ch4-DESIGN | The Strategy + Registry split: the Engine owns validate→resolve→preflight→report, and a Driver owns argv, dispatch and the artifact wait. | Record in docs/architecture/packages/internal-bridge.md. |

## go/internal/modelquery/lineage.go
verdict: MINOR
readability: 3
removed: REDUNDANT=1 RECOVERABLE=0 INVARIANT=3 DESIGN=1 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 10 | G16/ch4-INVARIANT | The regex's year anchor (19xx/20xx) is what keeps it off ":8b", "32b" and "4.6", but nothing tests withoutDate directly. A 4-digit capability token such as "-2048" does match. | Rename `dateRun` → `yearAnchoredDate`. Write `TestWithoutDate_StripsOnlyYearAnchoredRuns`: "gpt-4o-2024-08-06" is stripped, "qwen:32b" and "opus-4.6" are untouched, and the test decides what "ctx-2048" should do. |
| 20-23 | G34/ch4-RECOVERABLE | LineageKey mixes a named helper (`withoutDate`) with inline index slicing on `versionToken`, which is declared in another file (newestwins.go:9). The reader can't see that it is the same token NewestInLineage compares. | Extract `func withoutVersion(id string) string` next to `versionToken`, and `func collapseSeparators(s string) string`. The body becomes `collapseSeparators(withoutVersion(withoutDate(strings.ToLower(id))))`. |
| 19 | ch4-INVARIANT | Different keys must never be substituted: Flash never replaces Pro, and -mini never stands in for the base model. | Kept by TestLineageKey_SeparatesCapabilityClasses. |
| 28 | ch4-INVARIANT | Order inside a bucket must follow input order, because NewestInLineage's tie-break keeps the first-listed id. | Kept by TestGroupByLineage_OrderPreservingBuckets. |
| 12 | ch4-DESIGN | withoutDate is the one date strip shared with NewestInLineage (newestwins.go:25), so the bucket key and the compared version agree on what counts as a date. | Record in docs/architecture/packages/internal-modelquery.md. |

## go/internal/runscope/runscope.go
verdict: MINOR
readability: 4
removed: REDUNDANT=11 RECOVERABLE=2 INVARIANT=4 DESIGN=1 HISTORY=1
| line | smell | finding | refactor |
|---|---|---|---|
| 13 | G16/ch4-RECOVERABLE | `"EVOLVE_" + "LANE"` looks like a typo. The lost note said the split keeps the name out of the flagreaders guard (the bootstrap-locator pattern; --lane is primary, the env var is a script fallback). | Add `const bootstrapLocatorEnvPrefix = "EVOLVE_"` and write `EnvLane = bootstrapLocatorEnvPrefix + "LANE"`. Update the ACS pin TestC49B_002_EnvLane_IsSplitConst, which string-matches the literal form. |
| 65-67 | G5/G9/ch4-INVARIANT | `WorkspacePath` has no production caller and re-implements paths.RunWorkspace (paths.go:121, which core.RunWorkspacePath uses). "Must not embed the lane" is kept by TestRunScope_WorkspacePathStableNoLane. | Delete the method and its test, or delegate: `return paths.RunWorkspace(root, s.cycle)`. |
| 50, 66 | G25 | The `"cycle-"` prefix is spelled out twice. | Add `const cycleLeafPrefix = "cycle-"`. |
| 80-85 | ch7/N7/ch4-RECOVERABLE | absClean silently drops `filepath.Abs`'s error and hashes the relative path, which gives a different lane. The lost doc justified this: it only happens when the cwd is unreadable, and it matches the hotfix token. | Rename to `absCleanBestEffort`. The behavior is pinned by TestLaneFromRoot_EqualsHotfixToken. |
| 96 | G28/ch4-INVARIANT | The rune filter deliberately drops '.' so an override can't form "..", ".lock" or a leading ".". Now it is an unexplained 5-way condition. | Extract `func isRefSafeLaneRune(r rune) bool`. The invariant is kept by the "a..b"/"a.lock"/".hidden" loop in TestResolveLane_OverridePrecedence. |
| 15 | ch4-INVARIANT | Lane: distinct roots give distinct lanes, and one root keeps its lane across resume. | Kept by TestResolveLane_DistinctRootsDistinctLanes and TestResolveLane_StableAcrossResume. |
| 1 | ch4-DESIGN | Lost: runscope composes projecthash (the lane) and sessionrecord (the run token), and RunID is deliberately kept out of every path (resume and the warm worktree depend on it). | Record in docs/architecture/packages/internal-runscope.md. |
