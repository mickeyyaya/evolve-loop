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
  in S3/S4 and ParallelEvaluate contention can be measured) — **resolved below.**

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

### S5 (resolved): per-session sharding, measured

**Measured evidence.** `BenchmarkSignalCenter_ParallelObserve`
(`signalcenter_bench_test.go`) drives `-P` goroutines, each repeatedly calling
`Observe` on its OWN distinct session key (the ParallelEvaluate shape: N
independent sessions, no key sharing), across `-cpu=1,2,4,8`:

| GOMAXPROCS | Before (single RWMutex across `Assess`) | After (per-session lock) |
|---|---|---|
| 1 | 7738 ns/op | 7763 ns/op |
| 2 | 8916 ns/op (worse) | 3923 ns/op |
| 4 | 9699 ns/op (worse) | 2029 ns/op |
| 8 | 9737 ns/op (worse) | 1265 ns/op |

Under the original single-`sync.RWMutex`-across-`Assess()` model, ns/op *rises*
with parallelism instead of falling — proof that `Observe` calls on disjoint
session keys were serializing on one process-global write lock (H1 confirmed:
material contention, not a theoretical concern). After sharding, throughput
scales ~6.1× from `P=1` to `P=8` (near-linear), because independent sessions no
longer contend on a shared lock at all.

**Decision: implement minimal per-session sharding.** Each `sessionSignals` now
carries its own `sync.Mutex` (`ss.mu`) guarding `probe`/`last`/`busy`/`clean`/
`changed`. `SignalCenter.mu` (the global `sync.RWMutex`) is demoted to a purely
**structural** lock: it guards only the `sessions` map's shape (insert, lookup)
and the `registry` map — never the stateful `probe.Assess()` call or any field
read on `sessionSignals`.

**Ownership.** `SignalCenter.mu` owns "does this session key exist in the map
yet" and the `registry` contents. `sessionSignals.mu` owns everything about
ONE session's observed state (`probe`, `last`, `busy`, `clean`, `changed`) — no
other lock may read or write those fields.

**Lock ordering (invariant, enforced by code shape).** The global structural
lock is always acquired and **fully released** before any per-session lock is
acquired:
1. `Observe` takes `sc.mu.Lock()`, does the map lookup/insert, then
   `sc.mu.Unlock()` — only THEN does it take `ss.mu.Lock()` to run `Assess` and
   mutate the session's fields.
2. `Aggregate` takes `sc.mu.RLock()`, copies the `*sessionSignals` pointers into
   a slice, then `sc.mu.RUnlock()` — only THEN does it lock each `ss.mu` in
   turn to read `ss.last`.
3. `Busy`/`Changed` follow the same shape: `sc.mu.RLock()` → lookup → `sc.mu.RUnlock()`
   → `ss.mu.Lock()` → read → `ss.mu.Unlock()`.

No code path holds `sc.mu` and any `ss.mu` at the same time, and no code path
acquires a per-session lock before the global lock. This eliminates lock-order
inversion by construction (there is only one direction).

**No torn reads.** `Aggregate`/`Busy`/`Changed` read `ss.last`/`ss.busy`/
`ss.changed` under the exact same `ss.mu` that `Observe` writes them under, so
no read can observe a partially-updated `sessionSignals` — verified by
`TestSignalCenter_ObserveAggregateSameKeyRaceClean` (same-key concurrent
Observe/Aggregate/Busy/Changed, `-race`-clean).

**No new exported surface.** `sessionSignals.mu` is an unexported field on an
already-unexported type — zero `apicover` delta.

## Consequences

- **Positive:** `-race` clean under ≥8 concurrent producers (verified, cycle 430)
  and under the ParallelEvaluate mixed-op stress harness (cycle 433, ≥16
  producers × ≥100 cycles + concurrent readers + concurrent registration).
- **Positive:** OCP — new CLIs register without a switch edit.
- **Positive:** No goroutine lifecycle; no shutdown path required.
- **Resolved (S5):** Per-session sharding implemented — measured ~6.1×
  throughput improvement at `GOMAXPROCS=8` (see benchmark table above);
  `Observe` no longer holds a process-global lock across `Assess()`. Lock
  ordering and ownership documented above; no torn reads (verified under
  `-race`).
- **Invariant (S3):** Consumers (`driver_tmux_repl.go`, `stopreview.go`) remain
  untouched this slice; they migrate in S3/S4.
