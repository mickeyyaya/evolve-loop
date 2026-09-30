## go/internal/bridge/recipe/recipe.go
verdict: MINOR
readability: 4
removed: REDUNDANT=14 RECOVERABLE=0 INVARIANT=4 DESIGN=6 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 1 | ch4-DESIGN | The kept package doc says what the package drives but no longer says how it is built. It lost two things: the package owns its `SessionDriver`/`Clock` ports (executor.go) so the import arrow stays bridge → recipe and the engine stays a pure pane-in/keys-out state machine, and the pattern map (Repository `LoadRecipe`, Template Method `Engine.Run`, Strategy await kinds, Command steps). Nothing in the tree now stops a reader from importing `bridge` here. | Create `docs/architecture/packages/internal-bridge-recipe.md` with the sections "Owned ports and dependency direction" and "Patterns". |
| 11-19 | ch4-DESIGN | The sentinel set lost two facts. Callers branch with `errors.Is` and the CLI maps each error to an exit code. `ErrAutoRespondEscalation` fires on policy=escalate or a loop-guard trip. | In the same doc, add a "Sentinel errors → exit codes" table giving each `Err*` and the condition that returns it. |
| 24-25 | ch4-DESIGN / N1 | `KindCommand` vs `KindKeys` no longer shows how each one sends. `command` pastes the body through the paste buffer and then presses Enter, which survives newlines. `keys` sends raw tmux key tokens with no trailing Enter. The routing is visible only at executor.go:137 (`SendKeys` vs `SendCommand`). The per-CLI rationale on `Recipe` is gone too: Claude uses `/plugin` one-liners, Codex uses a menu TUI. | Document the `send.kind` and `per_cli` JSON fields in internal-bridge-recipe.md. Rename the Go identifiers `KindCommand`→`KindPasteCommand` and `KindKeys`→`KindRawKeys`; the wire values "command" and "keys" stay the same. |
| 31 | G28 / ch4-INVARIANT | An empty `on_timeout` aborts only because executor.go:124 tests `== OnTimeoutContinue`. The executor never reads `OnTimeoutAbort`. The removed comment credited a function `abortIfEmpty` that exists nowhere in go/, so it was already stale. | Add `func (o OnTimeout) continuesOnTimeout() bool { return o == OnTimeoutContinue }` and use it at executor.go:124. Add `TestEngineRun_ZeroValueOnTimeoutAborts`; today this is covered only indirectly, because `cmdStep` leaves the field empty in `TestEngineRun_StepTimeoutAbort`. |
| 73 | G20 / ch4-INVARIANT | The name `mergeParams` hides that the function DROPS undeclared caller keys. That is a security property: a stray `--param` cannot slip an unvalidated token into a step body. | Rename `mergeParams`→`resolveDeclaredParams`. `TestMergeParams/"undeclared caller key ignored"` already pins the behavior. |

## go/internal/phases/registry/registry.go
verdict: MINOR
readability: 4
removed: REDUNDANT=3 RECOVERABLE=1 INVARIANT=1 DESIGN=2 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 1 | ch4-DESIGN | The package doc shrank to "implements the phase Factory Method". It lost four things: each `internal/phases/<name>` package self-registers in `init()`; the dispatch sites (cmd_phase.go, cmd_compose.go) look phases up by name, so a new phase needs no switch edit; the concurrency model (writes at init time, concurrent reads under the RWMutex); and the `Register` usage snippet. | Create `docs/architecture/packages/internal-phases-registry.md` with the sections "Self-registration convention" (including the `init()` snippet), "Lookup sites" and "Thread safety". |
| 15 | N1 | `mu` guards only `factories`, but its name does not say so. | Rename `mu`→`factoriesMu`. |
| 52, 58 | N7 / naming consistency | `ResetForTesting` and `SnapshotForTest` use different suffixes for the same test-only role. The signature `SnapshotForTest() func()` hides that the returned func RESTORES the snapshot. | Rename `SnapshotForTest`→`SnapshotForTesting` and give it a named result: `func SnapshotForTesting() (restore func())`. |
| 52, 58 | ch4-INVARIANT | The rule that production code must not call these hooks is no longer stated anywhere, and nothing enforces it. | Add `TestTestingHooks_HaveNoProductionCallers`. It walks the non-`_test.go` files under go/ and fails on any reference to `registry.ResetForTesting` or `registry.SnapshotForTest(ing)`. |
| 60-63, 68-71 | G5 | The same map-copy loop is written twice, once for the snapshot and once for the restore. | Extract `func cloneFactories(src map[string]Factory) map[string]Factory` and call it in both places. |

## go/internal/verdictcache/verdictcache.go
verdict: MINOR
readability: 3
removed: REDUNDANT=10 RECOVERABLE=4 INVARIANT=3 DESIGN=2 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 1 | ch4-DESIGN | The kept doc says what is stored but not how the cache degrades. The cache is advisory and invalidates itself: a lost, stale or corrupt file costs a full tdd/build/audit run, never correctness (the same contract as clihealth). The key also deliberately leaves out the ADR's `inputs_digest` until the enforce stage. That contract is the only reason `Load` swallows read and parse failures. | Create `docs/architecture/packages/internal-verdictcache.md` with the sections "Degradation contract" and "Key choice: tree SHA only (inputs_digest deferred to enforce)". |
| 22-23 | N1 / ch4-RECOVERABLE | `ArtifactSHA256` and `ArtifactPath` do not say which artifact they mean: the audit-report.md the verdict was bound to. | Rename the fields to `AuditReportSHA256` and `AuditReportPath`; the json tags stay the same. |
| 16, 41 | G25 | The file name `".evolve", "verdict-cache.json"` is written inline. The `schemaVersion` constant no longer says which file it versions. | Add `const cacheFileName = "verdict-cache.json"` and rename `schemaVersion`→`cacheFileSchemaVersion`. |
| 44, 85, 94 | G20 / ch7 | `Load` returns an `error`, but every path returns nil: read failures and corrupt files degrade to an empty map with a WARN. `Put` and `Lookup` discard that error with `_`, which reads like a swallowed error. | Change the signature to `func (s *Store) Load() map[string]Entry` (or rename it `LoadOrEmpty`) so the degrade-to-empty contract is in the signature. `TestLoad_CorruptFile_DegradesToEmpty` and `TestLoad_ReadError_DegradesToEmpty` keep the behavior pinned. |
| 68 | G19 / G29 | `baseTreeSHA == "" \|\| candidateTreeSHA != baseTreeSHA` hides the guard against fresh-base collisions. A candidate equal to a resolved base is an untouched worktree that every sibling lane shares. A base that cannot be resolved is not evidence that the worktree is fresh. | Use explanatory variables: `baseUnresolved := baseTreeSHA == ""` and `worktreeChanged := candidateTreeSHA != baseTreeSHA`, then `return baseUnresolved \|\| worktreeChanged`. `TestProbeEligible_FreshBaseGuard` keeps the semantics pinned. |
| 76-78, 91-93 | ch7 | `Put` returns nil silently when TreeSHA is empty, and `Lookup` misses. Both are intended (a verdict with no content identity cannot be addressed by content), but without the comment they look like bugs. | Add `func hasContentIdentity(sha string) bool { return sha != "" }` and use it in `Put`, `Lookup` and `ProbeEligible`. `TestPut_EmptyTreeSHA_NoOp` and `TestPut_EmptyTreeSHAStaysNoOp` stay as they are. |
| 99-114 | G5 / ch7 | This is one of several hand-rolled temp-file-plus-rename writers; the others are core/phase_advisor.go:206 `writeArtifactAtomically`, adapters/storage/statejson.go:172 `writeJSONAtomic` and observerengine/report.go:44 `writeAtomic`. When `Rename` fails, the `.tmp.<pid>` file is left behind. | Extract one shared `atomicfile.Write(path string, data []byte, perm os.FileMode) error` that removes the temp file on failure, and call it here. |

## go/internal/phaseio/output.go
verdict: MINOR
readability: 3
removed: REDUNDANT=6 RECOVERABLE=6 INVARIANT=1 DESIGN=4 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 6-9 | G5 / ch4-RECOVERABLE | The removed doc said this vocabulary must match the EGPS gate (`core.Verdict*` = `cyclestate.Verdict*`). The literals are typed again here, so nothing keeps the two sets in step. `cyclestate` is a leaf package with no internal imports. | Derive the constants: `VerdictPASS Verdict = cyclestate.VerdictPASS`, and the same for FAIL, WARN and SKIPPED. If phaseio must stay free of that import, add `TestVerdict_MatchesCyclestateVocabulary` instead. |
| 21 | ch4-RECOVERABLE | `Severity string` lost its value set ("LOW".."CRITICAL"). | Add `type Severity string` with `SeverityLow`, `SeverityMedium`, `SeverityHigh` and `SeverityCritical`, and make the field `Severity Severity`. |
| 27 | ch4-RECOVERABLE | `Class` holds a failure-class slug such as `build_error` or `gate_fail`, but neither its name nor its type says so. | Add `type FailureClass string` and make the field `Class FailureClass`, with named constants for the classes already emitted. |
| 32-41 | ch4-DESIGN | The contract for `PhaseOutput` is gone. It is the typed message on the pipe (P4). Raw telemetry (tokens, cost, duration) deliberately stays on `core.PhaseResponse`. The zero value is a valid empty output. `Failure` becomes mandatory on non-PASS at the enforce stage (Phase 3.8). `Signals` keys are namespaced as `<phase>.<key>`. | Create `docs/architecture/packages/internal-phaseio.md` with a "PhaseOutput contract" field table. |
| 37 | N1 | `NextPhase` reads like a command, but it is an advisory routing hint that the kernel may ignore. | Rename `NextPhase`→`NextPhaseHint`. |
| 38 | N1 | `CommitSHA` does not say which commit it holds: the one anchoring this phase's deliverable (ADR-0027), or empty when there is none. | Rename `CommitSHA`→`DeliverableCommitSHA`. |
| 40 | N1 / G16 | `Reconciled` is opaque. It means the bridge reported a process failure but the on-disk deliverable was trusted anyway. It mirrors `core.PhaseResponse.Reconciled` (core/phase.go:119, set at phases/runner/verdict/classify.go:84) and `coherence.Reconciled`. | In one refactor commit, rename the field to `ProcessFailureOverridden` in `phaseio`, `core` and `coherence`; the json tag stays the same. |

## go/internal/phaseoutputs/phaseoutputs.go
verdict: MINOR
readability: 4
removed: REDUNDANT=6 RECOVERABLE=3 INVARIANT=2 DESIGN=3 HISTORY=1
| line | smell | finding | refactor |
|---|---|---|---|
| 1 | ch4-DESIGN / HISTORY | The package doc kept the "what" but lost the purity boundary: the input is the completed-phase list plus a name→size listing, and the `evolve cycle outputs` CLI and the wave monitor do the I/O. It also lost the file-naming rules the candidate lists encode: the report name comes from the resolved contract, usage files are named after the phase (C1 chokepoint), and prompt and events files are named after the phase, falling back to the agent name for retro. The operator-question and weeks-of-stubs narrative is history. | Create `docs/architecture/packages/internal-phaseoutputs.md` with the sections "Purity boundary" and "Artifact naming per phase kind" (as a table). |
| 17 | ch4-DESIGN | `NotOwed` is set on the row rather than just skipped in `Gaps`, so a consumer reading rows cannot re-derive a legitimate absence as a gap. Nothing says that now. | Add a section "NotOwed is data, not a filter" to the same doc. `TestRowArtifactResult_AreTheWireShape` already pins the field. |
| 33 | N1 / ch4-RECOVERABLE | `nativePhases` does not say what "native" means here: no agent was dispatched, so no prompt and no events are owed, but usage still is. | Rename `nativePhases`→`undispatchedPhases` and `TestNativePhases_RegisterIsExactlyShip`→`TestUndispatchedPhases_RegisterIsExactlyShip`. `TestSurvey_NativeShipOwesOnlyUsage` keeps the usage rule. |
| 35-73 | F1 / G34 | `Survey` is 38 lines doing four jobs: removing duplicate phases, resolving agent and report names from the contract, observing four artifacts, and marking which ones are owed. | Extract `func surveyPhase(phase string, listing map[string]int64, resolver phasecontract.Resolver) Row`. Inside it, call `func resolveNames(phase string, contract phasecontract.Contract, known bool) (agent, report string)` and `func markNotOwed(row Row, phase string, contract phasecontract.Contract, known bool) Row`. `Survey` keeps only the dedupe loop. |
| 46 | N5 | The one-letter name `c` has to be tracked across 20 lines of a long function. | Rename `c`→`contract`. |
| 59-61 | G5 / ch4-RECOVERABLE | Three calls repeat the same pattern: try the phase-named file first, then fall back to the agent-named one. Why the fallback exists (retro's prompt is named after the agent) is gone. | Extract `func observePhaseOrAgent(listing map[string]int64, phase, agent, suffix string) Artifact` and add the constants `promptSuffix = "-prompt.txt"`, `eventsSuffix = "-events.ndjson"` and `usageSuffix = "-usage.json"`. |
| 89, 107 | G5 | `Gaps` and `SummaryLine` each list the four artifacts by hand. `surveyed` only counts `len(r.Rows)`. | Add `func (r Row) artifacts() []Artifact` and `func (r Row) complete() bool`. In `SummaryLine`, use `surveyed := len(r.Rows)`. |

## go/internal/phasestream/mask.go
verdict: MINOR
readability: 4
removed: REDUNDANT=2 RECOVERABLE=2 INVARIANT=1 DESIGN=0 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 5 | ch4-RECOVERABLE | The reason only tool_use and tool_result can be evicted is gone: they carry the bulky file-read, test and build-log payloads, while verdict, error and thinking envelopes are never evicted. | Rename `isEvictable`→`isBulkyToolObservation`. |
| 9, 15 | N1 | `windowTurns` counts evictable envelopes, not turns. `MaskStaleObservations(in, 1)` keeps only Seq 4 and masks the Seq 3 tool_result (`TestMaskStaleObservations_MasksOldEvictablePreservesRest`). The removed comment was the only thing that explained the newest→oldest walk. | Rename the parameter `windowTurns`→`keepNewestObservations` and the variable `kept`→`keptObservations`. |
| 9 | ch4-INVARIANT | No test covers the promise that masking keeps the identity keys (`name`, `tool_use_id`) so the chain of actions still reads. Purity and the ≤0 pass-through are already pinned (`TestMaskStaleObservations_MasksOldEvictablePreservesRest`, `TestMaskStaleObservations_WindowLEZeroPassthrough`). | Add `TestMaskStaleObservations_PreservesIdentityKeys`. |
| 9 | ch4-DESIGN (same group) / dead code | The research basis (observation masking, arXiv 2508.21433) and the "≤0 means the feature is off" framing are gone. The function has no production caller: only mask_test.go uses it, and docs/research/code-audit-2026-07/deadcode-2026-07-08.txt lists it. Nothing tells a reader it is a staged lever that has not been wired in yet. | Add a section "Observation masking (not wired)" to `docs/architecture/packages/internal-phasestream.md`, or delete the function until something calls it. |
| 31-34 | N1 / G19 | `nd` is a two-letter name for the cloned data map. | Extract `func cloneData(d map[string]any) map[string]any` and write `maskedData := cloneData(e.Data)`. |
| 38, 41, 43 | G5 / G25 / G16 | The placeholder format string appears twice. The Data keys are bare string literals, also used in classify.go:256 and :295. The tool_use branch says "[output of …]" even though it replaces `input_excerpt`, which is an input. | Add `func maskedPlaceholder(kind, id string, seq int) string` and pass "input" or "output". Add the constants `dataKeyInputExcerpt`, `dataKeyExcerpt` and `dataKeyMasked`, shared with classify.go. |

## go/internal/phaseintegrity/repin_ifdrifted.go
verdict: MINOR
readability: 4
removed: REDUNDANT=2 RECOVERABLE=2 INVARIANT=3 DESIGN=0 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 10 | F1 (arguments) | The function takes five parameters (`statePath, binPath, runningCommit, pluginVer, prov`), and four of them are strings that can be swapped without any error. | Add `type RepinRequest struct { StatePath, BinPath, RunningCommit, PluginVersion string; Provenance ProvenanceVerified }` and change the signature to `func RepinIfDrifted(req RepinRequest) (RepinResult, error)`. |
| 12, 15-18 | ch7 | Every read or parse failure of state.json, and every `ReadFile` failure on the binary (permission denied or EIO, not just a missing file), becomes a silent `RepinResult{}, nil`. The removed comments were the only sign that failing open is deliberate, and only for an absent or torn file. | Narrow both no-op branches to `errors.Is(err, fs.ErrNotExist)`, plus a JSON decode error for a torn state file. Return every other error with context, e.g. `fmt.Errorf("phaseintegrity: read binary %s: %w", binPath, err)`. Add `TestRepinIfDrifted_UnreadableBinary_ReturnsError`. The branch table stays pinned by the four existing `TestRepinIfDrifted_*` tests. |
| 19-20 | G5 / ch4-RECOVERABLE | This inline sha256-of-file has to match `core.ShipSHAMismatch` (core/boot_preflight.go:73), and the removed trailing comment was the only link between them. The same helper is written again in cyclehealth.go:501, phases/ship/verify.go:20, subagentrun/verify.go:119 and cyclesimulator.go:334. | Extract one `fileSHA256Hex(path string) (string, error)` into a shared leaf package, and call it here and in `ShipSHAMismatch`. |
| 24 | F3 / ch4-RECOVERABLE | The bare `false` flag argument does not show its meaning: this path is not operator-authorized and only ever runs unattended. | Wrap the call once as `func repinUnattended(statePath, sha, runningCommit, pluginVer string, prov ProvenanceVerified) (RepinResult, error)`, which passes `false`. Alternatively, split `RepinShipSHA` into `RepinShipSHAUnattended` and `RepinShipSHAByOperator`. |
| 36 | G25 | `"expected_ship_sha"` is a bare literal here, in repin.go:46-47 and in releasepipeline.go:471-474. | Add `const expectedShipSHAKey = "expected_ship_sha"` to phaseintegrity and use it in both files. |

## go/examples/secure-orchestrator/main.go
verdict: NEEDS-REFACTOR
readability: 3
removed: REDUNDANT=16 RECOVERABLE=9 INVARIANT=0 DESIGN=2 HISTORY=1
| line | smell | finding | refactor |
|---|---|---|---|
| 1 | ch4-DESIGN | Everything the example taught is gone: the four defenses (deterministic orchestration, a worktree with hooks disabled, a SHA-256 lock on the eval file, and stripping comments before the audit), the note that this is the Go port of examples/secure-orchestrator.py, and the run command. The run command was already stale: it said `go run examples/secure-orchestrator.go`, but the real invocation is `go run ./examples/secure-orchestrator` from go/. | Create `docs/architecture/packages/examples-secure-orchestrator.md` describing the four defenses, the omitted check for a poisoned environment (package.json/Makefile), and the correct run command. |
| 15-69 | ch4-RECOVERABLE | The four `DEFENSE n` banners that grouped the functions by threat are gone, and one file now mixes all four defenses. | Split the code into `defense_argv.go`, `defense_worktree.go`, `defense_evallock.go` and `defense_auditstrip.go`. `main.go` keeps only the mock loop. |
| 15, 25 | N1 | "Safe" does not say what makes these functions safe: they run argv directly with no shell, so an argument written by an LLM (for example the commit message at line 108) stays literal. | Rename `runSafeCommand`→`runArgvNoShell` and `safeGitCommit`→`commitWithLiteralMessage`. |
| 26-27, 33, 35 | ch7 | Every git call throws away its error, including the security step itself (`core.hooksPath=/dev/null`). `main` still prints "hook-disabled worktree" (line 94) and "Orchestrator security held." (line 111) even when that step failed. | Return errors: `setupSecureWorktree(...) (string, error)` and `safeGitCommit(...) error`, and halt the cycle when either fails. |
| 30, 35 | G20 / ch4-RECOVERABLE | `setupSecureWorktree` hides that all of its security is one line: turning off local git hooks so a Builder cannot take over through a pre-commit hook. | Rename it `createHooklessWorktree` and extract `func disableGitHooks(wtDir string) error`. |
| 40-56 | ch7 / command–query separation | `hashFile` returns "" on any read error. If the eval file cannot be read both when it is locked and when it is audited, the two empty strings compare equal and the check "passes". `verifyTestIntegrity` is a query, but it also prints. | Change to `hashFile(path string) (string, error)`. Rename `verifyTestIntegrity`→`evalFileUnchanged(evalFile, lockedHash string) (bool, error)` and move the BREACH message into `main`. |
| 71-112 | F1 / G34 | `main` is 41 lines that mix phase narration, file I/O and mock data. The removed step "(Imagine the LLM edits files in wtDir here)" is no longer visible anywhere. | Extract `runScout(evalFile string) (string, error)`, `runBuilder(cycle int) (string, error)` with a named no-op `simulateBuilderEdits(wtDir string)`, `runAuditor(evalFile, lockedHash string) (bool, error)`, and `runShip(wtDir string) error`. |
