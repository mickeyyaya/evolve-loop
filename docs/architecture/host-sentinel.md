# The host sentinel

> The host sentinel samples the host during a wave. It reads the disk, the Go build cache, the tmux sessions, the orphan processes and the load. It signals each reading that is over its threshold. At stage `enforce` it runs the gc scopes that are safe during a wave. Code: [go/internal/hostsentinel](../../go/internal/hostsentinel) and [go/cmd/evolve/cmd_host_sentinel.go](../../go/cmd/evolve/cmd_host_sentinel.go). Inbox item: `host-sentinel-inside-the-loop`.

## 1. The problem

The gc runs only at a boundary: boundary step 10, the loop pre-batch hook and the batch-end hook. A wave runs for 9 to 13 hours. During a wave, nothing reads the host.

The evidence of 2026-11-04:

- The host had 273 MB of free disk.
- The Go build cache had 138 GB.
- 324 orphan processes ran. They were 3 to 6 hours old.

The operator decision of 2026-11-04: while a loop, a wave or a cycle runs, the loop must read the system and the tmux status.

## 2. The rule of the operator

The sentinel and the gc recycle only a target that is complete or orphaned. They never recycle a running tmux session or a running worktree. When the liveness of a target is not proven, the target stays.

## 3. Approaches that we studied

| Approach | What it does | Why we did or did not select it |
|---|---|---|
| A. Gc at the boundary only (the old design) | The gc runs between waves. | A wave of 13 hours fills the disk before the next boundary. Not enough. |
| B. An external cron job or a systemd timer that runs `evolve gc` | A host timer runs the full gc on a schedule. | It is not related to the loop life. It can run the full sweep while lanes run, and it stays after the loop stops. It adds a second process to manage. Rejected. |
| C. A watchdog agent (an LLM) that reads the host and decides | An agent reads the readings and selects the cleanup. | AGENTS.md does not let an LLM direct the pipeline, and a decision about a disk does not need judgment. Rejected. |
| D. A sentinel inside the loop process (selected) | A goroutine of the loop samples the host at an interval. It compares each reading with a threshold, signals, and runs only the gc scopes that are safe during a wave. | It starts and stops with the loop. It uses the ownership proofs that the gc has. It has a shadow stage before the enforce stage. Selected. |

### What other systems do (live research, 2026-11-04)

- **Gas Town.** A Go daemon is the base of a watchdog chain of three tiers. Its heartbeat tick runs each 3 minutes. The daemon is mechanical, and it does not reason about health. It gives the triage to an agent (Boot), and it restarts a dead Deacon session.
  - The Deacon writes a heartbeat file with counts of the healthy and the unhealthy agents. Sources: [Daemon/Boot/Deacon watchdog chain](https://docs.gastownhall.ai/design/watchdog-chain/), [Heartbeats](https://gastown.dev/docs/concepts/heartbeats), [Dog infrastructure](https://gastown.dev/docs/design/dog-infrastructure). We use the same interval as the default, and the same split: the sentinel is mechanical, and it makes no judgment.
- **OpenAI Symphony.** On each poll tick, the orchestrator first reconciles the running work against the tracker. Then it fetches the candidates and dispatches the work. When a preflight fails, the tick does not dispatch, but the reconciliation continues. Sources: [Symphony orchestration spec](https://agentpatterns.ai/standards/symphony-orchestration-spec/), [opensymphony spec alignment](https://docs.rs/crate/opensymphony/1.8.0/source/docs/symphony-spec-alignment.md). We use the same rule for the floor: below the floor no new lane starts, and the running lanes continue.
- **Kubernetes kubelet.** The kubelet compares eviction signals (`nodefs.available`, `imagefs.available`) with thresholds. Image garbage collection starts at a high threshold of disk use and collects down to a low threshold. Eviction comes only after the image gc. Sources: [Node-pressure eviction](https://kubernetes.io/ko/docs/concepts/scheduling-eviction/node-pressure-eviction), [kubernetes issue 124868](https://github.com/kubernetes/kubernetes/issues/124868). We use the same order: first the cache trim (the image gc of this host), and only then the floor that holds new lanes.

## 4. The design

### 4.1 The tick

The loop starts the sentinel after the batch preparation, and it stops the sentinel when the loop returns. The first sample comes after one interval. Each tick does these steps:

1. Sample each metric. A metric that fails is a failed reading. It is not a zero value.
2. Compare each reading with its threshold. A failed reading has no comparison.
3. For each breach, find its gc scope. Ask the gc for that scope: a preview at `shadow`, an apply at `enforce`.
4. Emit one `HOST_*` signal for each breach, with the value and the threshold.
5. Compare the disk free with the floor. At `enforce`, below the floor, hold new lanes.
6. Write the status to `.evolve/host-sentinel.json`.

The sentinel emits each code one time in each interval. A breach that continues past the interval signals again. A failed sample writes a `WARN` line and `HOST_SAMPLE_FAILED`. It never stops the loop.

### 4.2 The metrics

| Metric | Reading | Threshold | Code | Gc scope |
|---|---|---|---|---|
| `disk_free_bytes` | The free disk of the project root | `host_sentinel.disk_low_gib` (below) | `HOST_DISK_LOW` | `cache` |
| `go_cache_bytes` | The sum of the entries of `go env GOCACHE` | `gc.go_cache_max_gb` (above) | `HOST_CACHE_HIGH` | `cache` |
| `tmux_sessions` | The count of tmux bridge sessions | `host_sentinel.tmux_sessions_max` (above) | `HOST_TMUX_HIGH` | none |
| `orphans` | The count of finished-cycle orphan processes | `host_sentinel.orphans_max` (above) | `HOST_ORPHANS_HIGH` | `orphans` |
| `load_per_cpu` | The one-minute load of `/proc/loadavg` divided by the CPU count | `host_sentinel.load_per_cpu_max` (above) | `HOST_LOAD_HIGH` | none |

The cache threshold is the gc cap. When `gc.cache_trim_free_gb` is set, it is also the disk-low threshold, because the cache scope trims only under that value. The sentinel does not keep a second copy of these settings.

### 4.3 The gc scopes that are safe during a wave

`evolve gc --only <scope>` runs one scope and no other sweep. Each scope is safe while lanes run:

- `cache`: the LRU trim of the Go build cache and the sweep of stale pipeline temp. This scope touches no worktree, branch, session or process.
- `orphans`: the finished-cycle orphan processes. The gc proves the ownership of each target. Its parent pid is 1. Its cwd is in a closed or removed cycle tree. No live lane holds that tree. This scope touches no worktree, session or socket.

These stay at the boundary only: the run dirs, the worktrees, the dispatch processes, and the sessions and sockets of a live lease. The boundary gc does not change.

The sentinel runs a scope through the same entry as the operator: `evolve gc --only <scope>`. When the boundary holds the gc lease, the scope refuses at once (exit 3). The sentinel records `lease_held` and does not wait.

### 4.4 The floor

When disk free is below `host_sentinel.floor_gib` at `enforce`, a new lane launch returns `deferred` (exit 5). The lane does not start. A lane that runs continues. When disk free is above the floor again, new lanes start again. At `shadow`, `HOST_FLOOR_HOLD` has `held=false`: it names the hold that it can make.

### 4.5 The policy block

```json
{
  "host_sentinel": {
    "stage": "shadow",
    "interval_seconds": 180,
    "disk_low_gib": 20,
    "floor_gib": 5,
    "tmux_sessions_max": 40,
    "orphans_max": 20,
    "load_per_cpu_max": 2
  }
}
```

The values above are the defaults. The stage is `off`, `shadow` (the default) or `enforce`. An unknown stage writes a `WARN` line, and the sentinel does not run. Use `shadow` first. Read the signals and the previews of a few waves, then set `enforce`.

### 4.6 The status

`evolve wave status` shows the last sample on one line, for example:

```
host:    sampled 2026-11-04T09:00:00Z, stage shadow: disk free 0.25 GiB, go cache 138.00 GiB, tmux 11, orphans 324, load per cpu 3.50; breaches HOST_DISK_LOW,HOST_CACHE_HIGH,HOST_ORPHANS_HIGH
```

The JSON has the full status in `host`.

## 5. Tests

- The journey test drives the production sentinel from a real `policy.json`. It has a Go cache in the layout of the go command, and a lane with a live lease. At `enforce` a low disk trims the stale cache entry through `evolve gc --only cache`. The fresh entry and the live worktree stay. The sentinel signals `HOST_DISK_LOW` with its threshold, and `evolve wave status` shows the sample. At `shadow` the same host releases nothing.
  - Test: `TestHostSentinelJourney_AtEnforceALowDiskMidWaveTrimsTheStaleCacheSignalsAndSparesTheLiveLane`.
- `TestRunGC_OnlyOrphansSparesAProcessAndTheWorktreeOfALiveLane` proves that the orphans scope keeps the process and the worktree of a live lane.
- `TestHostSentinel_AGCLeaseHeldByTheBoundaryMakesTheReleaseSkipNotWait` proves the skip on a held lease.
- `TestHostGatedLaunch_BelowTheFloorNoNewLaneStartsAndARunningLaneContinues` proves the floor.
