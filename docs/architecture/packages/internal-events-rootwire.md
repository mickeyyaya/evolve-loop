# internal/events/rootwire

> Spec of record: [event-channels.md](../event-channels.md) §1, §5 and §6. Plan: [event-notification-protocol-2026-10.md](../../plans/event-notification-protocol-2026-10.md), component E10. Decision: [ADR-0127](../adr/0127-push-only-event-channels.md).

## Purpose

`internal/events/rootwire` subscribes one Publisher to the Signal Center of a root process. It returns the closer that the root defers. It is component E10 of ADR-0127.

A root is a process entry that builds its own Center. The roots are `cycle`, `loop`, `loop-chain`, `ship`, `subagent`, `simulate` and `phase-observer`. Each Center of a process has its own role, because the reader keys gaps and the duplicate window on (`source`, `pid`).

The package uses three other packages:

- `internal/events/publisher` (E6) routes and appends the events ([internal-events-publisher.md](internal-events-publisher.md)).
- `internal/policy` holds the compiled defaults: `EventsDefaults()` returns the channels, the queue keys and the lock deadline.
- `internal/ipcenv` names the environment key `EVOLVE_DISPATCH_ID`.

## API

| Export | What it does |
|---|---|
| `Attach(c, role, evolveDir, stderr) func()` | builds the Publisher config, subscribes `p.Listen` to the Center `c`, and returns the closer |

The closer runs `c.Flush()`, then `p.Close(deadline)`. When `Close` returns more than 0, it writes one stderr line. A root can call the closer more than once, and each call flushes and closes again. The stderr line goes out once. A root calls the closer before a re-exec and defers it for its exit.

## Design

- **One Publisher for each Center.** The role is part of the Publisher config, so two Centers need two Publishers. The loop process holds the chain Publisher and one batch Publisher at a time. Spec §Limits accepts the two waiting lock calls.
- **The config.**
  - The channel root is `paths.EventChannelsRoot(evolveDir)`, which is `<evolveDir>/events/ch`.
  - `Dispatch` is `EVOLVE_DISPATCH_ID`. An unset variable gives no `dispatch` key.
  - The channels, the segment size, the queue keys and the lock deadline come from `policy.EventsDefaults()`. A channel with a segment size of 0 uses the default size.
- **The deadline.** The closer gives `Close` the channel count times the lock deadline. The default is 7 × 250 ms = 1.75 s. `Close` can pass its deadline by that amount, because each channel can wait for one lock call (spec §5).
- **Degrade, never fail.** A Publisher that cannot write counts the loss. The closer then writes one `[events] WARN` line. A refused config writes one WARN at attach, and the closer only flushes. The Center, its other listeners and the root keep working.
- **An evolve directory that is not absolute is refused.** `newPublisher` checks `filepath.IsAbs` before it builds the config. An empty or relative path (for example `.evolve`, from an unset `EVOLVE_PROJECT_ROOT`) can put the channel logs under the working directory. The check refuses such a path. The role then publishes no events, writes one WARN, and creates no file. Every root gets this check, because every root calls `Attach`.
- **The closer is not a once-function.** A re-exec ends the process image, so a root closes its Centers just before it. If the exec fails, the loop goes on, and the Publisher still takes events. A once-function can drop the later events with no gap record. Each call of the closer flushes and closes again, and `Close` writes the gap records of the events that came after the call before.
- **No timer.** The package makes no timer. The deadline is a number that `Close` turns into one context. The no-poll guard scans this directory (`TestNoPollTimerInTheEventChannelSources`).

## Invariants

- **The kinds of the oversight loop reach the channels that the orchestrator spec names.** Pinned by `TestAttach_RoutesTheOrchestratorKindsToTheirChannels` and `TestAttach_AFailedCycleSealedReachesTheErrorsChannelToo`.
  - `cycle` holds `cycle.sealed`, `ship.landed`, `phase.dispatched` and `phase.outcome`.
  - `ship` holds `ship.landed`, and `loop` holds each event of the `loop` module.
  - `errors` holds each WARN and INCIDENT event, and `signals` holds every event.
- **A record carries the role and the dispatch stamp.** Pinned by `TestAttach_StampsTheRoleAndTheDispatchIDOfTheProcess` and `TestAttach_AProcessWithNoDispatchIDWritesNoDispatchKey`.
- **The closer flushes first, closes with the deadline, and a second call flushes and closes again.** Pinned by `TestCloser_FlushesThenClosesThePublisherWithTheDeadline`, `TestCloser_ASecondCallFlushesAndClosesAgainButWarnsOnce`, `TestAttach_ALossAfterAnEarlyCloseGetsItsGapAtTheFinalClose` and `TestDeadlineOf_IsTheChannelCountTimesTheLockDeadline`.
- **A best-effort channel is on disk when the closer returns.** Pinned by `TestAttach_TheCloserMakesTheBestEffortChannelsDurable`.
- **A loss gives one stderr line, and a failing Publisher never stops the Center.** Pinned by `TestCloser_WritesOneStderrLineWhenCloseCountsALoss`, `TestAttach_AnUnwritableEventsDirectoryDegradesToOneStderrWarn`, `TestAttach_ARefusedPublisherConfigWarnsOnceAndTheCenterStillWorks` and `TestAttach_AnEmptyOrRelativeEvolveDirPublishesNothingAndWarnsOnce`.
- **The policy maps to the Publisher config.** Pinned by `TestConfigOf_MapsThePolicyToThePublisherConfig`.
- **Every root subscribes the Publisher and defers its closer.** Pinned in `cmd/evolve` by `TestRootSignalCenter_EveryRootSubscribesThePublisher`, `TestRootSignalCenter_EveryRootDefersItsCloser` and `TestRootSignalCenter_NoRootDropsItsCloserAndEveryDepsLiteralCarriesIt`.
  - **The roots are derived, not listed.** A function that returns a closer (or an `orchDeps`) relays it, and a function that calls a relay and returns no closer is a root. A seam variable such as `wireOrchestratorDepsFn` counts as its default. A new root is thus checked at once. A root must have a top-level `defer` of `closeSignals` before any `return` that follows the call that obtains the closer. A defer in a branch, in a closure, or after an early return fails the guard.
  - **The guard has its own tests.** `TestDeriveCloserFlow_*` and `TestCloserDeferProblem_NamesWhatLetsARootSkipItsCloser` pin the derivation and each way that a root can skip its closer.

## Findings

- **E7 is not built.** The compiled defaults live in `go/internal/policy/policy_events.go`. E7 adds the `policy.json` keys and the gc categories on top of them.
- **No `ci` channel in the defaults.** The route `kind=ci.*` matches no registered kind until E11 adds `ci.completed`. The filter grammar refuses such a route, and then `publisher.New` refuses every config.
- **Observer reports are not on `cycle`.** The wave orchestrator spec §9 lists "observer reports (`cycle`)". The `cycle` route of spec §2 holds no observer kind. An `observer.warning` reaches `errors` and `signals`. The oversight loop must read the heartbeat of a lane from `phase.dispatched` and `phase.outcome`, or the route must change.
- **Fleet lanes share one set of channels.** `fleetLaneEvolveDirOK` requires `--evolve-dir` to be `<projectRoot>/.evolve`. Every lane process thus appends to the same channels under the same lock.
- **A re-exec retires the Centers first.** The boot refresh, the chain boundary refresh and the wave boundary refresh call `syscall.Exec`. Each root now closes its Centers just before the call, so a best-effort queue is on disk. In a chain, the batch Center closes first and the chain Center second (`loopConfig.retiring` and `loopConfig.beforeReExec` in `cmd/evolve/signal_loop.go`). A goroutine that emits between the close and the exec can still lose its events with no gap record.
- **`Close` does not bound a hung `write`.** The closer passes a deadline to `Close`. The deadline bounds the lock waits. `Close` waits for each writer goroutine with no timeout (`awaitWriter` in `go/internal/events/publisher/queue.go`). A `write` that hangs in the kernel thus holds the exit of the root.
- **Tests do not create channel files by default.** `attachRootEventsFn` in `cmd/evolve` is `rootwire.Attach`. The `TestMain` of that package replaces it with a flush. Without that, each of the 47 test call sites of the wiring functions leaves a writer goroutine that can race the `TempDir` cleanup.
