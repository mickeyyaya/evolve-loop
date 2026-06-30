# ADR-0068: Bridge SignalCenter — Concurrency Model

Status: Accepted
Date: 2026-07-01
Relates to:
- [ADR-0047](0047-single-source-with-projection.md) — single-source-with-projection; DetectorFor is the fallback projection
- [ADR-0065](0065-per-phase-binary-integrity.md) — per-phase integrity; SignalCenter lives inside the liveness package boundary
- Campaign: signal-center S2 (cycle 430); S1 landed cycle 429

## Context

The signal-center campaign (S1–S5) unifies bridge liveness detection behind a
single `SignalCenter` Facade. S1 (cycle 429) unified the token extractor.
S2 (this ADR) introduces the `SignalCenter` type itself.

`SignalCenter` owns per-session, stateful `LivenessProbe` instances (keyed by
session string) and aggregates them to one `LivenessState`. The key concurrency
question is: **what lock model governs the `sessions` map and the handler
`registry`?**

Three models were considered:

### Option A: Channel-actor (goroutine owns the map; all access via channel)

- A single goroutine serialises all reads and writes.
- Eliminates the mutex but adds goroutine lifecycle and shutdown surface.
- Failure mode: goroutine leaks / stall on blocked send — exactly the wedge
  failure class that incidents 254, 262, 274–277, 286–288 tracked.
- **Rejected**: adds the very failure surface the campaign must not regress.

### Option B: `sync.Map` + per-entry atomic/lock

- `sync.Map` optimises disjoint-key reads (write-once pattern).
- Our access pattern is *write-every-interval* (Observe on every review tick),
  not write-once: `sync.Map` does not help and loses type safety.
- Stateful detectors (`stalls`, `primed`, peak-token) still need a per-entry
  lock to mutate safely — net complexity exceeds a plain RWMutex.
- **Rejected**: complexity without benefit for this access profile.

### Option C: `sync.RWMutex` over `map[string]*sessionSignals` (chosen)

- Idiomatic, type-safe, trivially provable under `-race`.
- Writers (`Observe`, `RegisterHandler`) hold the write lock for their full
  duration — stateful detector mutation requires exclusive access.
- Readers (`Aggregate`) hold a read lock — allows concurrent aggregation across
  evaluation phases.
- Per-session sharding is explicitly **deferred to S5** (after consumers migrate
  in S3/S4 and ParallelEvaluate contention can be measured).

## Decision

**Use `sync.RWMutex` over a `map[string]*sessionSignals` and a `map[string]func()
LivenessProbe` registry.** A single mutex guards both maps; writers take the
write lock, readers take the read lock.

### Aggregation rule (documented, not implicit)

`Aggregate()` applies a priority sweep over all active sessions:

1. Any `LivenessConverging` session ⇒ return `LivenessConverging`.
2. Else any `LivenessHung` ⇒ return `LivenessHung`.
3. Else any `LivenessBusyButStagnant` ⇒ return `LivenessBusyButStagnant`.
4. Else any `LivenessIdle` ⇒ return `LivenessIdle`.
5. No sessions (empty center) ⇒ return `0` (unset/undefined).

Rationale: if any session is actively producing output (`Converging`), the
overall signal is `Converging`. `Hung` takes priority over stagnancy because
fast-failing a hung session is more urgent than waiting for a stagnant one.

### Handler registry (OCP seam)

`RegisterHandler(profileName, factory)` lets callers add a CLI strategy without
editing the `DetectorFor` switch. On first session creation, `Observe` looks up
the registry; on miss it falls through to `DetectorFor` (backward-compatible
default). Add-a-CLI = `RegisterHandler` + profile-entry only.

## Consequences

- **Positive:** `-race` clean under ≥8 concurrent producers (verified, cycle 430).
- **Positive:** OCP — new CLIs register without a switch edit.
- **Positive:** No goroutine lifecycle; no shutdown path required.
- **Deferred (S5):** Per-session locking if ParallelEvaluate concurrency reveals
  contention on shared session keys.
- **Invariant (S3):** Consumers (`driver_tmux_repl.go`, `stopreview.go`) remain
  untouched this slice; they migrate in S3/S4.
