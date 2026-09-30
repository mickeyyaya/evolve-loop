## go/internal/adapters/flock/flock.go
verdict: MINOR
readability: 4
removed: REDUNDANT=1 RECOVERABLE=2 INVARIANT=1 DESIGN=1 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 1 | ch4-DESIGN | The kept package doc no longer says that `Lock` blocks on purpose (the sibling `TryLock` uses LOCK_NB). It also no longer says that flock(2) locks each open file description, so two `Lock` calls in the same process also contend. Neither fact is in any doc, and no test covers contention within one process. | Add `docs/architecture/packages/internal-adapters-flock.md` with both facts. Pin the second one with `TestLock_SecondLockInSameProcessBlocksUntilRelease`. |
| 14 | N4 / ch4-INVARIANT | Other code also takes `ShipLockPath`: the gc worktree pruning (gc/worktrees.go:371) and the dossier closeout commit (core/dossier_producer.go:29, cmd/evolve/cmd_loop_dossiers.go:21). The name hides that this is the one lock for every write to the shared `.git/index`. The removed warning said a second `"ship.lock"` literal reopens the index.lock race, and nothing enforces that now. | Rename to `GitIndexMutationLockPath`. Add `TestGitIndexMutationLockPath_IsTheOnlyShipLockLiteral`, which scans non-test `.go` files for `"ship.lock"` outside flock.go. |
| 32-36 | ch4-RECOVERABLE / ch7 | The rule "call release exactly once" is gone. A second call runs flock on a closed fd and closes the file twice, and both errors are dropped with `_ =`. | Wrap the returned closure in `sync.Once` (`releaseOnce`) so the code enforces the rule. |
| 26-27, 33-34 | G5 / ch4-RECOVERABLE | The `flockFn(int(f.Fd()), how)` + `runtime.KeepAlive(f)` pair appears twice. The comment explaining why KeepAlive is needed (f must outlive the raw-fd syscall) is gone. | Extract `func flockFile(f *os.File, how int) error`, which makes the syscall and calls KeepAlive in one place. |

## go/internal/fleetbudget/fleetbudget.go
verdict: NEEDS-REFACTOR
readability: 3
removed: REDUNDANT=9 RECOVERABLE=9 INVARIANT=3 DESIGN=2 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 13-17, 65-69 | N4 / ch4-RECOVERABLE | Three untyped string constants, and the comment saying what `DerivedFrom` records is gone. `FromFloor` misleads: that branch runs `cfg.Count` lanes, not `cfg.Floor`. `FromResetPace` suggests pacing, but that branch does no pacing. | Add `type DerivedFrom string` and use it for `BudgetPlan.DerivedFrom`. Rename `FromFloor` to `FromNoQuotaSignal` and `FromResetPace` to `FromResetUnsized`, keeping the wire values. |
| 19-24 | N1 / ch4-RECOVERABLE | The fields have no unit or range. `Count` is the maximum lane count and `Floor` the minimum. `CapacityCycles` is the number of cycles a 100% quota window affords. `Safety` must be in (0,1]. None of this can be read from the code any more. | Rename to `MaxLanes`, `MinLanes`, `CyclesPerFullQuota` and `SafetyFraction`. |
| 33-39 | N1 | `binding` / `rem` is jargon for "the tightest quota window, which is the binding constraint". | Rename the type to `tightestWindow` and the field `rem` to `remainingFraction`. |
| 42-47 | G31 / ch4-RECOVERABLE | `Plan` rewrites its own `cfg` parameter to fix the bounds (Floor ≥ 1, Count ≥ Floor). The reason, defence against bad operator input, is gone. | Extract `func (cfg Config) withSaneBounds() Config` and call `cfg = cfg.withSaneBounds()`. |
| 50 | G28 | The four-part condition for the budget branch is written inline. | Extract `func (b tightestWindow) canSizeBudget(tp budgethistory.Throughput, cfg Config) bool`. |
| 74 | G19 / ch4-RECOVERABLE | The removed comment said `tp.CyclesPerHour` is the rate for one lane, not the whole fleet, and that `affordable` is a lane count. Without it the formula cannot be checked. | Rename `affordable` to `affordableLanes` and add the explanatory variable `cyclesPerLaneUntilReset := tp.CyclesPerHour * dt.Hours()`. |
| 78-84 | G16 / F1 | The duty-cycle pacing cannot be read without its comment: `dutyGap := Floor/affordable - 1`, scaled by the median cycle time and capped at the reset horizon. | Extract `func floorForcedPaceDelay(affordableLanes float64, minLanes int, medianCycle, untilReset time.Duration) time.Duration`. The cap is already pinned by `TestPlan_PaceDelayCappedAtResetHorizon`. |
| 111-124 | ch4-INVARIANT | A probed state with no buckets must leave `found=false` so the plan falls back to the floor. No test covers this. | Add `TestPlan_ProbedStateWithoutBucketsFallsBackToFloor`. |
| 1, 90 | ch4-DESIGN | Lost design: sizing uses native units (remaining fraction plus reset), never dollars; `min_lanes` became `Floor` (#303); the default is safe to run in shadow; `QuotaJoin` is deliberately not a `BudgetPlan` field, so it cannot leak into the decision. No package page exists. | Create `docs/architecture/packages/internal-fleetbudget.md`. |

## go/internal/docsfloor/docsfloor.go
verdict: MINOR
readability: 3
removed: REDUNDANT=4 RECOVERABLE=5 INVARIANT=3 DESIGN=3 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 40-50, 82-92, 117-127 | G5 | The same "some changed file starts with some prefix" loop is written three times. | Extract `func firstPathUnder(changedFiles, prefixes []string) (string, bool)` and use it in `LabelArchitecture`, `HasDocsDelta` and `Evaluate`. |
| 29, 80, 82 | N4 / ch4-RECOVERABLE | `docPrefixes` (`docs/`) and `DocsRoots` (the architecture tree plus the runtime reference) read as synonyms. The removed comments said the difference is deliberate: the WARN floor accepts any change under `docs/`, while the blocking floor accepts only architecture docs. | Rename to `anyDocPrefixes` and `ArchitectureDocRoots`, and rename `HasDocsDelta` to `HasArchitectureDocsDelta`. |
| 31, 40, 52, 59 | N4 / ch4-RECOVERABLE | Two pairs of near-identical names hide the split between a WARN-grade check and a blocking-grade check: `architectureSurfaces` vs `strictArchitectureSurfaces`, and `LabelArchitecture` vs `IsArchitectureClass`. | Rename to `kernelSurfaces` / `blockingArchitectureSurfaces` and `TouchesKernelSurface` / `IsBlockingArchitectureChange`. |
| 40 | G9 | `LabelArchitecture` has no production caller. Only the acs/cycle1147 and cycle1151 tests use it; core/build_floor_reviewer.go:110 labels with `IsArchitectureClass`. | Delete it together with its acs predicates, or wire it in. Do not keep an exported function that only tests call. |
| 62, 73 | G28 / ch4-RECOVERABLE | Rule 1 (a test-only change never labels) and rule 3 (a phase spec is vocabulary) have lost their reasons. The inline conditions look arbitrary. | Extract `isTestFile(p string) bool` and `isPhaseSpec(p string) bool`, named after the rules. |
| 108 | G25 | The only stage literal in the code is `"off"`. `"shadow"`, `"enforce"` and "empty means enforce" appear only in tests and the removed comment. | Add named constants `StageOff`, `StageShadow` and `StageEnforce` in docsfloor, and compare against `StageOff`. |
| 132 | G16 | `strings.Join(docPrefixes, "/")` uses `/` as the list separator. With a second prefix the message would read `docs//x/`. | Use `strings.Join(docPrefixes, ", ")`. |
| 1 | ch4-DESIGN | The code no longer states that the floor warns and never rejects (because judging a doc's adequacy is editorial) or that the package imports only stdlib so every layer can use it. | The rationale is in docs/architecture/adr/0077-docs-floor-for-architecture-changes.md. Add the leaf-import rule to a new `docs/architecture/packages/internal-docsfloor.md`. |

## go/internal/shipmanifest/shipmanifest.go
verdict: MINOR
readability: 4
removed: REDUNDANT=6 RECOVERABLE=4 INVARIANT=5 DESIGN=2 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 43-48, 68-73 | G5 | Both functions re-implement `sortedKeys` (L105) line for line. | Replace each block with `return sortedKeys(seen)`. |
| 65 | ch7 | The error from `continuation.ReadManifest` is discarded. A corrupt continuation manifest silently drops the prior attempt's declarations, and enforce mode then fails a correctly resumed cycle without naming the cause. | Return the error (`Declared(...) ([]string, error)`), or at least emit a WARN that names the manifest path. |
| 55-57 | ch7 | Every `ReadFile` error is treated as "report absent", so a permission error reads as an empty manifest. | Skip only when `errors.Is(err, fs.ErrNotExist)` and surface any other error. |
| 64-67 | G34 / ch4-RECOVERABLE | The continuation union (ADR-0076 slice C, one hop only) is an unnamed sibling-directory computation inside `Declared`. | Extract `func priorAttemptWorkspace(workspacePath string) (string, bool)`. |
| 31, 34-40 | F1 / ch4-RECOVERABLE | `extractReportPaths` does six things: tokenize, check the left boundary, trim punctuation, strip `./`, filter escapes, and dedupe and sort. The reason for the L31 check is gone (RE2 has no lookbehind, so without it `.goreleaser.yml` would be cut to `goreleaser.yml`). | Extract `startsInsideLargerToken(md string, start int) bool` and `normalizeReportToken(tok string) (string, bool)`. The behaviour is already pinned by `TestExtractReportPaths_BareRootFilenames` and `TestExtractReportPaths_NeverAbsoluteOrParent`. |
| 81 | G16 | `c != ""` and `!strings.HasPrefix(c, "/")` can never be false after L77: `path.Clean` never returns `""` and keeps a relative path relative. | Reduce to `return c != "." && c != ".." && !strings.HasPrefix(c, "../")`. |

## go/internal/modelquery/latest.go
verdict: MINOR
readability: 4
removed: REDUNDANT=0 RECOVERABLE=2 INVARIANT=0 DESIGN=4 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 5 | N1 / ch4-RECOVERABLE | The note "ids the CLI resolves itself, in preference order" is gone. Order matters, because the first alias present wins (L14-17), but the name does not say so. | Rename `AliasIDs` to `AliasesByPreference`. |
| 9-19 | G34 | `Freshest` nests building a set and scanning aliases inside `if p.PreferAlias`. | Extract `func (p FreshnessPolicy) firstPresentAlias(lineage []string) (string, bool)`. `Freshest` then becomes one guard and a return. |
| 23, 39 | N1 / ch4-RECOVERABLE | `sel` means the classifier's tier-to-model map in `PromoteLatest` and the incumbent model id in `incumbentFirst`: one abbreviation for two types. The reason for `incumbentFirst` (a tie keeps the classifier's pick) now depends on the parameter name. | Rename `PromoteLatest`'s `sel` to `tierSelection` and `incumbentFirst`'s `sel` to `incumbent`. |
| 3, 8, 23 | ch4-DESIGN | The removed text covered: the zero value is the default for CLIs that list their models, the policy comes from the manifest's `model_freshness`, no CLI name appears in a conditional, and the alias rule sits in front of `NewestInLineage`. | Already in docs/architecture/packages/internal-modelquery.md:26-27. The tie, verbatim-selection and no-mutation invariants are pinned by `TestPromoteLatest_*`. No action needed. |

## go/internal/adapters/statemap/statemap.go
verdict: NEEDS-REFACTOR
readability: 3
removed: REDUNDANT=5 RECOVERABLE=3 INVARIANT=4 DESIGN=3 HISTORY=0
| line | smell | finding | refactor |
|---|---|---|---|
| 80 | ch7 (silent fail-open) | When the file on disk is malformed or unreadable, `ReadStateMap` errors, so `WriteStateMap` skips the revision CAS and then overwrites the file. This contradicts the removed `ReadStateMap` contract ("a malformed file returns an error so callers refuse to clobber"). Only `UpdateStateMap` aborts. core/reset.go:221 and phases/ship/statefile.go:18 call `WriteStateMap` directly. | On a read error, refuse: `return fmt.Errorf("refuse to overwrite unreadable %s: %w", path, err)`. Pin with `TestWriteStateMap_MalformedOnDiskRefusesToClobber`. |
| 78-125 | F1 / G34 | The 47-line function mixes symlink resolution, the revision CAS, the carryoverTodos tripwire, mkdir, marshal, and a temp-file write plus rename. | Extract `refuseStaleRevision(onDisk, m map[string]any, path string) error`, `warnOnCarryoverMassDrop(onDisk, m map[string]any, path string)` and `writeFileAtomic(path string, buf []byte) error`. |
| 78 | N7 / ch4-RECOVERABLE | The name hides that this is the unlocked primitive (callers must hold `flock.WithPathLock`, while `UpdateStateMap` is the locked path). It also hides that the function may refuse (CAS) and write to stderr. | Rename to `WriteStateMapUnlocked`. |
| 81, 157 | G25 / ch4-DESIGN | The literal `"stateRevision"` sits next to the constant `statemapRevisionKey`. The two-counter design is gone: `storage.UpdateState` owns `stateRevision`, statemap only compares it and bumps its own counter. | Add `const storageRevisionKey = "stateRevision"` and put the two-counter design in a new `docs/architecture/packages/internal-adapters-statemap.md`. |
| 71-76, 88-90 | G25 / G28 / ch4-INVARIANT | Four unexplained values: the `-1` sentinel, `20`, `/2` and `"carryoverTodos"`. The WARN goes straight to `os.Stderr` as a hidden side effect, and no test covers the tripwire. | Change to `todosCount(m) (int, bool)`, add `carryoverTodosKey` and `massDropMinTodos = 20`, and extract `isCarryoverMassDrop(oldN, newN int) bool`. Pin with `TestWriteStateMap_WarnsOnCarryoverMassDrop`. |
| 106-123 | G5 | `_ = os.Remove(tmpPath)` is repeated on four error paths. | Use one `defer` that removes the temp file unless a `renamed` flag is set. |
| 137 | N1 / ch4-INVARIANT | `mutate` runs under the cross-process flock and must not call `UpdateStateMap` on the same path, or the blocking flock deadlocks. Nothing says so now. | Rename the parameter to `mutateUnderLock` and record the deadlock rule in the package page. The resolved-path lock is already pinned by `TestUpdateStateMap_CrossTreeWritersSerializeOnCanonical`. |
| 145-149 | G16 | The if/else is equivalent to `rev, _ := counterOf(m, statemapRevisionKey); m[statemapRevisionKey] = rev + 1`, because `counterOf` returns 0 when the key is absent. | Collapse it to those two lines. Add `const lastUpdatedKey = "lastUpdated"` for L150. |
| 37 | N1 | `maxDepth` does not say what it bounds. | Rename to `maxSymlinkHops`. |

## go/internal/core/carryover/carryover.go
verdict: MINOR
readability: 4
removed: REDUNDANT=7 RECOVERABLE=1 INVARIANT=2 DESIGN=4 HISTORY=2
| line | smell | finding | refactor |
|---|---|---|---|
| 21-29 | G11 / G17 | One const block mixes the priority vocabulary (which itself mixes `P0/P1` and `high/medium`), a re-exported ledger prefix, and two rune caps. | Split it into a `type Priority string` block and a limits block. |
| 28 | N1 / ch4-RECOVERABLE | The removed comment said the cap is for a failure summary's message. core/carryover_lifecycle.go:65 aliases it as `maxFailureLearningSummaryChars`, which says chars where the constant counts runes. | Rename to `MaxFailureSummaryRunes`, and rename the core alias to match. |
| 62 | F (5 params) | `warn(origin, cycle, code, reason, fields)` takes five positional parameters. | Pass a `warning{Origin, Cycle, Code, Reason, Fields}` struct, or have the four call sites build the `signalcenter.Event` fields. |
| 63 | ch4-INVARIANT | `l.center().Emit(...)` reads like a nil dereference. It relies on `(*Center).Emit` treating a nil receiver as a no-op (the Null Object). | Already pinned by `TestWithSignals_NilIsTheNullObject`. Rename `center()` to `centerOrNil()` so the Null Object is visible at the call. |
| 37 | ch4-DESIGN | The code no longer explains why `WithSignals` takes an accessor function rather than a `*Center`: the orchestrator's Center is installed after construction. | Already in docs/architecture/decomposition/03-carryover-lifecycle.md:72. Rename the parameter `c` to `centerAccessor`. |

## go/internal/usageprobe/usageprobe.go
verdict: MINOR
readability: 4
removed: REDUNDANT=4 RECOVERABLE=4 INVARIANT=2 DESIGN=1 HISTORY=1
| line | smell | finding | refactor |
|---|---|---|---|
| 59, 69 | G5 / G16 / ch4-INVARIANT | `ctx.Err() != nil` is checked twice with no blocking call in between. Now that the comments are gone, the second check looks like a mistake. The intent, never write a bench from a pane read after the interrupt, is not tested. | Keep one guard, named `interruptedAfterProbe := ctx.Err() != nil`, right after `Probe`. Add `TestProber_probeOne_CappedPaneReadAfterCancelIsNotBenched`. |
| 22, 72 | N1 / ch4-RECOVERABLE | `Classify` returns a bool meaning "this pane shows the family is capped now". The name does not say what `true` means. | Rename the field to `IsCapped`. |
| 32 | N1 / ch4-RECOVERABLE | The inline note "snapshot: skip families already benched" is gone, and `active` does not say what it holds. | Rename to `alreadyBenched`. |
| 39 | G9 | `family := family` has been dead code since Go 1.22 made loop variables per-iteration, and go.mod says `go 1.23`. | Delete the line. |
| 17 | N1 / ch4-RECOVERABLE | The note that this label tells the proactive probe's benches apart from the reactive `rate_limit` benches is gone. | Rename to `proactiveProbeBenchReason`. |
| 29, 53, 66, 77, 80 | G5 | The `[usage-probe]` prefix is written out five times. | Add `func (p *Prober) logf(format string, args ...any)` with a `logPrefix` constant, and add the explanatory variable `inFlight := launched.Load() - completed.Load()` at L53. |
| 1, 27 | ch4-DESIGN | Lost design: the proactive bench complements the reactive one, the dispatcher skips benched families up front (`runner.applyBenchToPlan`), the tmux session GC reaps abandoned probe sessions, and the whole package fails open. No package page exists. | Create `docs/architecture/packages/internal-usageprobe.md`. The cancel contract is already pinned by `TestProber_Run_ReturnsPromptlyOnCancelEvenWhenAProbeIgnoresIt`. |
