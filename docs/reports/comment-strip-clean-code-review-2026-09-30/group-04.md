## go/internal/shiperr/shiperr.go
verdict: MINOR
readability: 4
removed: REDUNDANT=10 RECOVERABLE=4 INVARIANT=7 DESIGN=5 HISTORY=2
| line | smell | finding | refactor |
|---|---|---|---|
| 10 | ch4-DESIGN | The ShipError doc said why this type exists: ship is a pure executor that cannot reject a cycle, the orchestrator decides (debugger, re-run or block), and the type sits in a zero-dependency leaf so ship and core can share it without an import cycle (core re-exports it through aliases). doc.go keeps only the first clause. No `docs/architecture/packages/internal-shiperr.md` exists. | Create `docs/architecture/packages/internal-shiperr.md` covering the executor-versus-orchestrator split, the leaf-package placement, the core alias re-export, and ADR-0064 (a cycle may not edit the gate that grades it, so it ships `--class manual`). |
| 20-25 | ch4-INVARIANT / ch4-DESIGN | The class meanings are gone: Transient means a retry may succeed, Precondition means the state can be re-established, and the debugger defaults an Integrity error to BLOCK. `SignalSeverity` (signalcodes.go) keeps only the fact that integrity is an incident. | Add a `classDocs map[ShipErrorClass]string` beside `codeDocs`, register it the same way, and pin it with `TestClassDocs_CoverEveryDeclaredClass`. Pin the BLOCK default in core with `TestShipRecoveryDebugger_IntegrityClassDefaultsToBlock`. |
| 92 | F1 / G28 | `NewShipError(code, class, stage, message, kv...)` asks every call site to choose a class, yet production pairs each code with a single class. The one exception is `CodeGitStageFailed` (Transient ×2, Precondition ×1). The removed per-code comments were the only statement of the intended pairs: FleetRebaseNeeded is transient, FleetRebaseConflict is integrity, RepoContractInfra and ManifestGate are precondition. | Turn `codeDocs` into `codeSpecs map[ShipErrorCode]codeSpec{Stage ShipStage; Class ShipErrorClass; Doc string}` and add `NewShipErrorFor(code, message, kv...)` that looks up class and stage. Pin the pairs with `TestCodeSpecs_ClassPerCode` (RebaseNeeded=transient, RebaseConflict=integrity, RepoContractInfra=precondition). |
| 40-89 | ch4-RECOVERABLE / G5 | The seven stage section headers (`// verify-class — audit binding`, `// atomic-ship — git`, …) are gone. The code-to-stage mapping now survives only as a prose prefix ("verify-class:") inside the `codeDocs` strings. | Store the stage as `codeSpec.Stage` (see the row for line 92) and drop the string prefix. Pin it with `TestCodeSpecs_EveryCodeHasAStage`. |
| 81-82 | ch4-INVARIANT | Nothing in the file now says why `CodeRepoContractInfra` deliberately stays separate from `CodeRepoContractGate`: a toolchain death means "safe to re-dispatch", not "fix your code". The vocab tests pin only the wire strings. | Keep `TestRepoContractGate_PersistentAmbiguityIsInfraClassedExactlyTwoRuns` (phases/ship) as the guard. Add `TestCodeSpecs_RepoContractInfraIsNotAnAliasOfGate` once `codeSpecs` exists. |
| 93-100 | G19 | The odd-trailing-key rule (a key with no value is kept as `""`, not dropped) sits in an inline loop inside a constructor. `TestNewShipError_OddKV` pins it, but the loop does not name it. | Extract `func debugPairs(kv []string) map[string]string` and call it as `Debug: debugPairs(debugKV)`. |

## go/internal/llmcalls/record.go
verdict: MINOR
readability: 3
removed: REDUNDANT=15 RECOVERABLE=8 INVARIANT=3 DESIGN=3 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 73 | N1 / ch4-RECOVERABLE | `FreshSessionRetry` reads as "this row is the retry". The removed doc and `bridge/engine.go:336` (`markFreshSessionRetry` on the first attempt) show that it marks the dead dispatch that a fresh session followed. Without the comment the name misleads. | Rename the Go field to `SupersededByFreshSession` and keep `json:"fresh_session_retry"`. Pin it with `TestEngine_FreshSessionRetryMarksTheDeadDispatchNotTheRetry` in bridge. |
| 59 | N2 / ch4-RECOVERABLE | `Model` is the legacy requested selector, but it sits beside `RequestedModel`/`DispatchedModel` with no sign that new code must not read it. | Rename the Go field to `LegacyRequestedModel` and keep `json:"model"`. |
| 20, 66 | N1 / ch4-RECOVERABLE | `Source`/`SourceUnknown` do not say what the source is of (token evidence). aggregate.go already calls the same value `MeasurementSource`. | Rename them `MeasurementSource` / `MeasurementSourceUnknown`, keeping `json:"source"`. |
| 18-46 | G11 (inconsistent) | `UsageStatus` is a typed string enum, while the dispatch, timing-scope and outcome vocabularies are untyped string constants and `Record.DispatchSource` is a plain `string`. The removed per-constant docs were the only thing grouping each vocabulary. | Add `type DispatchSource string`, `type TimingScope string` and `type Outcome string`, and type the `Record` fields with them. |
| 42-46 | ch4-RECOVERABLE | The rule "success means exit 0, failure means non-zero, unknown means no exit code" now lives only in aggregate.go `outcomeOf`, away from the constants. | Move it here as `func (r Record) Outcome() Outcome` and have aggregate call it. |
| 83 | G28 / G19 | `r.SchemaVersion == 0 && r.CallID == ""` is the legacy-row test. The removed SchemaVersion doc ("legacy records have no version") was what explained it. | Extract `func (r Record) isLegacy() bool`. |
| 96-104 | N1 / G19 | `callSequence` is used only when `crypto/rand` fails, and the removed doc was the only explanation of the fallback (timestamp-pid-sequence) path. | Rename it `fallbackCallSequence` and extract `func fallbackCallID(now time.Time) string`. |
| 18, 75 | ch4-DESIGN | Two definitions left with the comments: what `bridge_dispatch` timing covers (launch through driver completion, excluding token enrichment and the ledger append), and why first-output timing is optional (no source means the same thing on headless and REPL drivers). The package boundary (no provider or orchestration logic) went too. No package doc exists. | Create `docs/architecture/packages/internal-llmcalls.md` with the timing-scope definitions, the first-output rationale and the producer/reader boundary. |

## go/internal/phases/ship/manifest.go
verdict: NEEDS-REFACTOR
readability: 3
removed: REDUNDANT=3 RECOVERABLE=1 INVARIANT=1 DESIGN=2 HISTORY=1
| line | smell | finding | refactor |
|---|---|---|---|
| 29-40 | ch7 (silent fail-open) | Both git reads drop their errors (`if …; err == nil`). When `status` and `diff` both fail, `changedSet` is empty and the gate logs "OK — all 0 bound path(s) covered", so enforce passes with nothing checked. The removed doc called this gate FAIL-CLOSED. | Extract `func boundPaths(ctx, opts, target bindTarget) ([]string, error)` that returns the git error. Under enforce, turn that error into a `CodeGitIO` ShipError; under shadow, log it. Pin it with `TestReconcileManifest_GitReadFailureIsNotReportedOK`. |
| 34-39 | G5 / correctness | The `diff --name-only` lines are neither read through `shipmanifest.RawPathRead` nor decoded with `shipmanifest.UnquoteGitPath`, although `detectColliders` (gitops.go:100-105) does both. A non-ASCII path on the cycle branch arrives as `"caf\303\251.txt"`, so `OutOfManifest` flags it: a false block under enforce, and a duplicate of the decoded porcelain entry. | Inside `boundPaths`, run the read through `RawPathRead(...)` and pass each line through `UnquoteGitPath`. Pin it with `TestReconcileManifest_NonASCIIBranchPathIsCovered`. |
| 19 | F1 / F2 | The function takes six parameters, and `res *RunResult` is an output argument used only to append log lines. | Pass a `bindTarget{worktree, branch, cycleBranch string}` and return `(logLine string, err error)`; the caller appends the line. |
| 28-45 | G34 / G5 | Low-level set building and sorting (a copy of `shipmanifest.sortedKeys`) sits beside the gate decision. The removed doc named the input set ("porcelain dirt plus cycle-branch commits, the same inputs detectColliders binds"), and nothing in the code says so now. | Move that set building into `boundPaths` (see the row for lines 29-40) and share it with `detectColliders`, or export `shipmanifest.SortedKeys`. |
| 1-13 | ch4-DESIGN | The file header explained why the gate exists: ship binds the whole `git diff HEAD`, so an undeclared path, typically a sibling lane's untracked leak, would ship. It also said shadow is the default per the "new gates default shadow" precedent. internal-phases-ship.md:23 names `reconcileManifest` but not this reasoning. | Add the rationale paragraph to `docs/architecture/packages/internal-phases-ship.md`. `TestReconcileManifest_OnlyExactEnforceLiteralBlocks` already pins "anything other than enforce is shadow". |

## go/internal/phasetiming/phasetiming.go
verdict: MINOR
readability: 4
removed: REDUNDANT=9 RECOVERABLE=6 INVARIANT=3 DESIGN=6 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 101-107 | G5 | `addTokens` is a verbatim copy of `llmcalls/aggregate.go:200`. | Add `func (u TokenUsage) Plus(o TokenUsage) TokenUsage` to cyclestate and delete both copies. |
| 136 | G25 / ch4-RECOVERABLE | The literals `"evaluate"` and `"audit"` hide the rule the removed comments stated: audit is excluded by name because it is the serial anchor that branches on the verdict. | Add named constants `evaluateArchetype = "evaluate"` and `serialAnchorPhase = "audit"`, and extract `func isParallelizableCheck(e Entry) bool`. |
| 80-89 | G25 / G19 | The literals `"unknown"` (archetype of a legacy entry) and `"FAIL"` (tokens counted as wasted) carry meaning the removed docs stated. | Add named constants `unknownArchetype` and `verdictFail`, extract `archetypeOrUnknown(e Entry) string`, and extract `cacheHitRatio(t TokenUsage) float64` for lines 95-97. |
| 39-42 | ch7 / ch4-INVARIANT | The read error is returned unwrapped while the parse error gets context. The removed doc explained why: callers use `os.IsNotExist`, which does not unwrap. The resulting inconsistency looks accidental. | Wrap it as `fmt.Errorf("read %s: %w", FileName, err)`, move callers to `errors.Is(err, fs.ErrNotExist)`, and update `TestRead_MissingFileIsNotExist` to `errors.Is`. |
| 20, 30 | N1 / ch4-RECOVERABLE | `BootMS` is the cold-REPL-boot slice of `DurationMS` (0 on warm and headless paths). `Tokens` is the terminal attempt only, with per-attempt detail in llm-calls.ndjson. Neither name says this. | Rename the Go fields `ColdBootMS` and `TerminalAttemptTokens` and keep the JSON tags. |
| 31 | ch4-INVARIANT | The removed doc claimed that `omitempty` lets a reader "tell unknown fill from a genuine 0.0". With a `float64` field a genuine 0.0 is omitted as well, so the claim was false. | If the distinction matters, change the field to `*float64`. Otherwise keep `TestEntry_ContextFillOmittedWhenUnknown` as the contract and record the limit in the package doc. |
| 1, 17, 124 | ch4-DESIGN | Three kinds of design knowledge were lost: who writes and who reads the file (core `recordPhaseOutcome` is the sole writer; the dossier and `evolve cycle timing` read it), the leaf-package constraint, and the shadow status of `ParallelProjection` with its independence assumption (a lower bound). The Diagnostics consumers went too (seal `backfillFailReasons`, cyclehealth). | Create `docs/architecture/packages/internal-phasetiming.md`. |

## go/internal/phaseregistrar/registrar.go
verdict: NEEDS-REFACTOR
readability: 3
removed: REDUNDANT=4 RECOVERABLE=3 INVARIANT=8 DESIGN=3 HISTORY=1
| line | smell | finding | refactor |
|---|---|---|---|
| 37-114 | F1 / G34 | `Register` is 78 lines and runs ten steps at mixed levels: normalize ×3, validate spec, resolve CLI, check profile name, reject driverless, clamp tier, register mint, persist, build runner. The removed comments were the only step headings. | Extract `normalizeMint(cfg) phaseconfig.PhaseConfig`, split into `forceOptionalSandboxedWriter`, `defaultStubSelectMetadata` and `stampOnDemandUnlessPersonaResolves`. Also extract `resolveDispatchCLI(cfg) (phaseconfig.PhaseConfig, error)`, `validateMintedDispatch(spec, cfg, prof) error` (lines 76-91) and `r.registerThenPersist(spec, prof) error` (lines 93-105). |
| 39-44 | hidden side effect | `cfg` is a value copy, but `Dispatch.Sandbox` is a pointer. When the caller passes one, `Enabled` and `ReadOnlyRepo` are written into the caller's `SandboxConfig`. | Copy before writing: `sb := profiles.SandboxConfig{}; if cfg.Dispatch.Sandbox != nil { sb = *cfg.Dispatch.Sandbox }; sb.Enabled, sb.ReadOnlyRepo = true, false; cfg.Dispatch.Sandbox = &sb`. Pin it with `TestRegister_SourceWriterDoesNotMutateCallerSandbox`. |
| 53-60 | G19 | The flag variable `resolvable` hides the question being asked ("does the derived persona exist?"). | Extract `func (r Registrar) personaResolves(cfg phaseconfig.PhaseConfig) bool`. |
| 81 | G28 / ch4-INVARIANT | The comment on why the driverless check applies only when `ProfilesDir != ""` is gone. With persistence off, a dispatch-less config is the checked-in shape used by the campaign study path. | Add an explanatory method `func (r Registrar) persistsProfiles() bool`. Pinned by `TestRegister_DriverlessCLI_AllowedWithoutPersistence`. |
| 86 | G25 / G28 | `policy.TierRank(tier) == 0` encodes "unclassifiable tier" as a magic zero. The removed comment explained that `ValidatePin` exempts rank 0 but a mint must not. | Add `policy.IsClassifiedTier(tier) bool`, or a constant `policy.TierUnclassified = 0`. |
| 93-105 | G31 | Registering before persisting is required so every lane's tree-diff guard knows the name before the files appear. This is temporal coupling expressed only by statement order. | Put the ordering inside one method, `registerThenPersist`. It is pinned by `TestRegister_RegistryAppendFails_RejectsBeforePersist`. |
| 94-97 | G5 | The `NowFn` fallback is written inline. | Extract `func (r Registrar) now() time.Time`. |
| 143 | G5 / N1 | `nameRE` duplicates the regex at `phasespec/validate.go:10`. The removed doc said it is "the same rule phasespec applies". | Export `phasespec.IsKebabName(s string) bool` and use it here. |
| 68-74 | ch4-INVARIANT | The rule is to resolve a bare CLI family ("claude") to its driver before `ToProfile` and to reject an unresolvable one at mint time. Only the ephemeral `acs/cycle1325` predicates pin it; this package has no test. | Add `TestRegister_BareCLIFamilyPersistsConcreteDriver` and `TestRegister_UnresolvableCLIFamilyRejected`. |
| 1, 138-141 | ch4-DESIGN | The SRP note went: `Register` is a pure factory and the caller splices the runner, routing and catalog. The #404 precedent for the stub SELECT metadata went too ("steer the router away; never pad `metadataAllowlist`"). | Create `docs/architecture/packages/internal-phaseregistrar.md`. |

## go/internal/config/routing_config.go
verdict: MINOR
readability: 3
removed: REDUNDANT=5 RECOVERABLE=10 INVARIANT=3 DESIGN=4 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 29 | G20 / N1 | Without the `RubricHint` comment, `TriggerIsTheWholeRule` is a riddle. It holds when `insert_when` is set and there is no rubric hint, so the advisor cannot admit the phase another way. | Rename it `AdmitsOnlyByTrigger()`. internal-config.md:39 already carries the reasoning. |
| 38-39, 44-45 | N1 / ch4-RECOVERABLE | The removed trailing comments carried the key and value meaning ("phase -> conditional-mandatory rule", "phase -> enablement source", …). The bare names `Mandatory`, `Conditional`, `PhaseEnable` and `Triggers` lost it. | Rename them `MandatoryPhases`, `ConditionalMandatoryByPhase`, `EnableByPhase` and `TriggersByPhase`. |
| 49 | G25 / ch4-RECOVERABLE | `AuditFailRoutesTo string` accepts only "retrospective" or "memo" (from policy's `failure_floor`), and the type does not show this. | Add `type AuditFailRoute string` with constants `AuditFailToRetrospective` and `AuditFailToMemo`. |
| 51-54 | N7 / ch4-RECOVERABLE | The names of the bools and the cap do not say what they do. | Rename `RoutingJudge` to `ScoreRouteQuality`, `ReconDigest` to `InjectReconDigest`, `CompactPrompts` to `StripOnDemandPromptSections`, and `RePlanMaxDepth` to `MaxRePlansBeforeDebugger`. |
| 3-10 | ch4-RECOVERABLE | Nothing in the struct now says that the path fields are relative to `Root`. | Add `func (s DeliverableKindSpec) Path(rel string) string { return filepath.Join(s.Root, rel) }` and use it at the call sites. |
| 46-48 | ch4-INVARIANT | The empty-value fallbacks are documented in internal-config.md:36, but no test with a matching name pins them: an empty `Order` falls back to the router's canonical order, an empty `SpineOrder` to the kernel's spine literal, and an empty `LegalSuccessors` to the literal graph. | Add `TestRoute_EmptyOrderFallsBackToCanonicalOrder`, `TestKernel_EmptySpineOrderUsesLiteralSpine` and `TestKernel_EmptyLegalSuccessorsUsesLiteralGraph`. |

## go/internal/shipmanifest/porcelain.go
verdict: MINOR
readability: 3
removed: REDUNDANT=1 RECOVERABLE=2 INVARIANT=4 DESIGN=1 HISTORY=1
| line | smell | finding | refactor |
|---|---|---|---|
| 18-37 | G16 / G25 | `splitPorcelainRename` advances the outer loop index inside a nested loop to skip a quoted segment, and `i+4` hides `len(" -> ")`. | Extract `func skipQuoted(path string, open int) int`, add a constant `renameArrow = " -> "`, and write `path[i+len(renameArrow):]`. |
| 42-64 | G25 / G5 | The magic `3` is the width of the porcelain XY status plus a space, and `line[0]` is the index column. `detectColliders` (phases/ship/gitops.go:110-120) re-parses porcelain by hand, calling `TrimSpace` before `line[3:]`, so ` M go/x.go` yields `o/x.go`: the duplication carries a live bug. | Add a constant `porcelainPathOffset = 3` and `func indexStatus(line string) byte`, and route `detectColliders` through `ChangedPaths`. |
| 54 | N1 / G20 / ch4-RECOVERABLE | `gonePaths` does not say it returns the paths a `git add` pathspec must not name (naming one is rc=128 for the whole add). | Rename it `unstageablePathspecEntries`. |
| 61-66 | G28 / ch4-INVARIANT | `HasPrefix(line, "D ")` against `line[0] == 'R'` is a deliberate asymmetry: "DD" (a merge conflict), " D" (an unstaged deletion) and a copy source must stay named. Without the comment it reads as an inconsistency. DD is pinned in stageable_test.go:23; " D" and the copy source are not pinned. | Extract `isStagedDeletion(line)` and `isStagedRename(line)`, and add `TestGonePaths_UnstagedDeletionAndCopySourceStayNamed`. |

## go/internal/sysexec/sysexec.go
verdict: MINOR
readability: 4
removed: REDUNDANT=3 RECOVERABLE=1 INVARIANT=2 DESIGN=2 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 50-58 | G20 / ch7 | `CombinedOutput` is named after `exec.Cmd.CombinedOutput` but drops the exit code (`_`), so a non-zero exit returns a nil error, which the stdlib method would not. It has no production callers, while about ten sites call `exec.Command(...).CombinedOutput()` directly (for example modelquery/exec.go:16 and core/task_contract.go:192). | Either delete it and its tests (YAGNI), or return `(transcript string, exitCode int, err error)`, rename it `CombinedTranscript`, and migrate the direct callers. |
| 28 | G25 / ch4-RECOVERABLE | The `-1` that goes with an unrecoverable error was part of the removed `RunFunc` contract. Callers now compare against a bare literal. | Add an exported constant `ExitCodeNotRun = -1`. |
| 12-13 | F1 | `RunFunc` takes eight parameters. | Add `type Command struct{ Name, Dir string; Args, Env []string; Stdin io.Reader; Stdout, Stderr io.Writer }` and change the signature to `RunFunc func(ctx context.Context, c Command) (int, error)`. There are 29 importers, so land it as its own refactor commit. |
| 12 | ch4-INVARIANT | The contract "a non-nil env replaces the parent environment, nil inherits it" is unpinned. The exit-code and error contract is already pinned by `TestDefaultRunner_NonZeroExit_IsCodeNotError` and `TestDefaultRunner_BinaryNotFound_IsUnrecoverableError`. | Add `TestDefaultRunner_NonNilEnvReplacesParentEnvironment`. |
| 34 | N1 | `o, e` read as "output, error". | Rename them `stdoutBuf, stderrBuf`. |
| 12, 39 | ch4-DESIGN | Two design notes went: `RunFunc` is the one seam that every legacy runner signature reduces to, and the usage guide (use `Output` when any non-zero exit is a failure, `Capture` when the caller branches on the rc, as with `git diff --quiet` rc=1). | Create `docs/architecture/packages/internal-sysexec.md`. |
