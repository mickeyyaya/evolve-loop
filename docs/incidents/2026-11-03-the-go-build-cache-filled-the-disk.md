# 2026-11-03: the Go build cache filled the disk and other tools failed in strange ways

Related documents: the policy [policy-config.md](../architecture/policy-config.md#free-disk-floor-gcdisk_floor_gb-gcdisk_warn_gb), the package note [internal-gc.md](../architecture/packages/internal-gc.md) and the operations guide [wave-operations.md](../operations/wave-operations.md).

## Summary

- **Impact:** the host had 273 MB free of 460 GB. Tools failed with errors that did not look related. The first symptom was "no space left on device" from `evolve ci classify` during an artifact download.
- **Root cause:** `~/Library/Caches/go-build` held 138 GB. `evolve gc` caps the cache at 20 GB, but it runs only at wave boundaries. Long waves with many lanes and console builds grow the cache faster than the boundary runs trim it.
- **Fix at the time:** `go clean -cache` freed 139 GB in 2.5 minutes. The cache is a pure cache, so only build time was lost.
- **Fix in the design:** the operator chose a host sentinel inside the loop, so that the trim does not wait for a boundary. The inbox item `host-sentinel-inside-the-loop` holds it.
- **Status:** the sentinel is not yet in this record as shipped. See "What changed".

## Timeline

| Time | Event | Evidence |
|---|---|---|
| 2026-11-03 | `evolve ci classify` fails with "no space left on device" during an artifact download. Other tools fail in ways that do not point to the disk. | the error text of the tool |
| 2026-11-03 | `df -h` shows 273 MB free of 460 GB. The Go build cache holds 138 GB. | `~/Library/Caches/go-build` |
| 2026-11-03 | `go clean -cache` frees 139 GB in 2.5 minutes. | the command output |
| 2026-11-04 | The operator picks the sentinel inside the loop. | inbox item `host-sentinel-inside-the-loop` (source field) |
| 2026-11-15 | A new reading shows 87 GiB free. The cache trim removes 1.08 GB and reports 27.45 GB left, over the 20 GB cap. | the output of `evolve gc` |

## Root cause

**F1.1** The cap for the Go build cache is 20 GB (`defaultGoCacheMaxGB` at `go/internal/gcpolicy/gcpolicy.go:66`).
*Implication:* a cache of 138 GB was far over the cap, so the cap did not act.

**F1.2** `evolve gc` runs only at wave boundaries: the boundary step, the loop pre-batch and the batch end (inbox item `host-sentinel-inside-the-loop`, source field). A wave runs 9 to 13 hours.
*Implication:* in a long wave, nothing trims the cache between boundaries.

**F1.3** A wave with many lanes and console builds adds cache entries faster than the boundary runs remove them.
*Implication:* the cache grows without a bound during a wave. At the same time, nothing watches the free disk.

**F1.4** The cap never trims an entry that is newer than `GoCacheInUseWindow`, two hours (`go/internal/gc/gocache.go:21`).
*Implication:* a cache that grows fast can stay over the cap after a trim. The reading of 2026-11-15 shows this: most entries were under two hours old, so 27.45 GB stayed over the 20 GB cap.

**F1.5** A full disk does not give one clear error. Each tool fails at the point where it first writes a file.
*Implication:* the visible symptom was far from the cause. The first check must be `df -h`.

## What changed

| Change | State | Test |
|---|---|---|
| `go clean -cache` by hand | done on 2026-11-03 | none: a one-time action |
| `evolve gc --only cache` as a scope (`gcScopeCache` at `go/cmd/evolve/cmd_gc_scope.go:10`). It touches no worktree, branch, session or process, so it is safe while lanes run. | in the code | `go/cmd/evolve/cmd_gc_scope_test.go` |
| Policy floors `gc.disk_floor_gb` (10 GB) and `gc.disk_warn_gb` (20 GB). Under a floor, `evolve gc` writes a `WARN` line. | in the code and in policy-config.md | `go/internal/gcpolicy/gcpolicy_test.go` |
| Host sentinel inside the loop: a background task. It samples the free disk and the cache size. It runs the scopes that are safe during a wave. | open in the inbox as `host-sentinel-inside-the-loop`. No pull request is known. | the acceptance of the item |

## What we learned

- When tools fail in strange ways, run `df -h` first. A full disk causes errors that do not name the disk.
- A cleanup that runs only at a boundary cannot protect a task of 9 to 13 hours. A guard must run inside the long task. The sentinel is that guard.
- A cap with a hold time can stay exceeded for a while. Read the free disk, not only the cap, to know the host state.
