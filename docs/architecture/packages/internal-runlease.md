# internal/runlease

## Purpose

`internal/runlease` is the single source of truth for the per-run `.lease` heartbeat file. It is the one shared interface between two sides. The retention engine (`internal/gc`) reads leases to classify a run directory as live. The fleet scheduler writes and refreshes a lease for every run it supervises. The package is a leaf that imports only the standard library, so both sides import it without bending the import graph.

## Design

- **Contract.** The file is `<run-dir>/.lease` and holds one JSON object. The writer refreshes `HeartbeatAt` at least every `DefaultTTL/2`. A reader treats the run as live while `now - HeartbeatAt < ttl`. Writes are atomic (temporary file, then rename), so a reader never sees a torn file.
- **A missing lease is not an error; a corrupt one is.** `Read` returns `ok=false` and no error when the file is absent. When the file exists but is unreadable or is not valid JSON, `Read` returns an error, so the caller decides.
- **Callers that discard the error read a corrupt lease as no lease.** Liveness then falls back to the run-state signal (gc's `workspaceIsCurrent`). A parse error must never make a run more collectable than a missing file does. `gc` also logs the anomaly.
- **`Fresh`** uses `DefaultTTL` when `ttl <= 0`, and a timestamp that does not parse is never fresh.
- **`OwnerLive` adds a pid probe on top of `Fresh`.** A crashed owner can keep a fresh heartbeat for the common 2 to 6 minutes after the crash. Without the probe it reads as live and blocks a seal. The probe is a further way to prove death, and nothing more. A stale heartbeat is never live, whatever the pid state says (this guards against pid reuse).
- **`OwnerLive` falls back to freshness alone** and does not call the probe in two cases. One is a nil `alive`. The other is an `OwnerPID` of 0: a lease written before pid tracking, or by a caller that never sets it.
- **`OwnerPID` is informational** (forensics: which scheduler held the lease). A live pid alone is never proof of run liveness; the freshness of `HeartbeatAt` is the only liveness signal.
- **`HeartbeatAt` is RFC3339Nano UTC,** stamped by `Write`. Callers never set it.
- **`PIDAlive` is the one owner-liveness probe** that lease readers hand to `OwnerLive`; the swarm reaper delegates to it. On Unix it sends signal 0, which only checks for errors. Nil means alive and signalable, `EPERM` means alive but owned by another user, and `ESRCH` means gone. A pid of 0 or below is never probed (0 is the caller's own group, -1 is everything, negatives are groups) and reports dead. On other platforms it cannot probe: a pid above 0 reports alive, and 0 or below reports dead. A fresh lease then keeps its conservative meaning, and `OwnerLive` falls back to the heartbeat alone.
- **The write seams are per instance,** held by `writer`, not package variables. A test injects a failing `createTemp` or `rename` into its own writer without mutating shared state, so the write tests run in parallel. `newWriter` supplies the real operating-system seams, and `Write` is the convenience over a real-seam writer for the single production caller.

## Invariants

- **Write ordering.** A scheduler writes the lease before it moves its run to a non-terminal state. A gc pass that snapshots liveness then never races a run into existence without a lease. gc re-checks leases at apply time as defense in depth, but that re-check only narrows the window. The ordering closes it.
- A pid at or below 0 is never signalled.

## Findings

- Before the pid probe, liveness was freshness alone (a 10 minute lease TTL). A crashed owner with a fresh heartbeat blocked `SealCycle` and the loop guard, and forced `evolve cycle reset --force` at every batch boundary. The `TestOwnerLive_*` tests pin the dead-pid, live-pid, nil-probe and pid-0 cases.
