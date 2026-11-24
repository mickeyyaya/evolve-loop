# internal/hostsentinel

## Purpose

`internal/hostsentinel` decides what a host reading means for a running wave. It holds the sample type, the pure decision function `Decide`, the signal codes and the publisher. The plan is [host-sentinel-inside-the-loop-2026-11](../../plans/host-sentinel-inside-the-loop-2026-11.md). The package is a leaf: it imports only `signalcenter`.

## Design

- **A pure core.** `Decide(sample, config, state, now)` returns a `Decision` and a new `State`. It never changes the state it receives. The shell in `cmd/evolve` ([cmd-evolve.md](cmd-evolve.md)) reads the host, calls `Decide`, and acts on the result.
- **Four readings.** A sample holds four values: the minimum free disk, the Go cache size, the owner-dead tmux servers and the orphan processes of finished cycles. The cache and the counts are slow to read. A sample marks them with `CacheRead` and `CountsRead`. An unread value crosses no threshold.
- **Findings and dedupe.** Each crossed threshold gives one finding with its value and its limit. The dedupe key is the code. A finding repeats after `repeat_s`. It repeats at once in a second band: twice the limit for a size or a count, and half the floor for free disk.
- **The hold.** Free disk under `gc.disk_floor_gb` sets the hold. Free disk at `gc.disk_warn_gb` clears it. The hold stays between the two values. A hold that lasts `hold_max_s` gives one stuck report, and the hold stays on. The pool reports a clear with `LOOP_ORCH_POOL_FLOOR_HOLD` and `held=false`.
- **The release.** Free disk under the warn line or a cache over its cap asks for the mid-wave release. A cooldown of `repeat_s` limits it. After a release that frees less than `no_progress_bytes`, the same reading starts no new run until it gets worse (`State.AfterRelease`). The backoff ends when the reading recovers: free disk at the warn line or more, or a cache read at its cap or less. The count findings never start a release.
- **Why stages.** `Announce` maps a decision to signals for two stages. At shadow it emits `HOST_WOULD_HOLD` and `HOST_WOULD_RELEASE` and does nothing else. At enforce the shell sets the hold and runs the release, and `Announce` emits `HOST_LANE_HOLD` and the stuck incident. The two stages are separate so that a false stop of the hold cannot block the release.

## Invariants

- **The decision never reads the environment, the clock or the host.** The caller passes the sample and the time. Pinned by the tests in `decide_test.go`, which run with fixed values.
- **The state is a value.** `Decide` copies the mark map before it writes. Pinned by `TestDecide_DoesNotMutateTheStateItReceives`.
- **The codes belong to the `host` module.** `HOST_DISK_LOW` stays with the cycle-start guard and keeps its fields. The sentinel has its own disk codes, because a second registration with other fields is a registry conflict. `HOST_LANE_HOLD` and `HOST_LANE_HOLD_STUCK` use the kind `loop.wave`, so `wave watch` prints them. The other codes use the new kind `host.sentinel`.
- **The seed closure is this package only.** `DecisionVersion` hashes the transitive closure of the stage seed. A leaf keeps the version stable across unrelated merges.

## Findings

- **2026-11-21, build at shadow.** The console built the sentinel at shadow only. The scoring of false stops and the evidence path of the stage flip are a follow-up item, `host-sentinel-stage-evidence`. Both stages stay at `shadow` until that item lands.
