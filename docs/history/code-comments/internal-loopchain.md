# Comment history: `internal/loopchain`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/loopchain/defaults.go:81` — above `func FleetLaneActive(evolveDir string) (active bool, err error) {`

```text
// FleetLaneActive reports whether a SIBLING fleet lane holds a live run under
// evolveDir. Discovery is gc.Discover, the lease-aware run-dir scan the
// retention engine owns; the lease of each Live dir is then re-read here for
// the two questions retention never asks — whose is it, and is the owner
// alive? A run dir is a sibling when its lease is not this process's own
// (runlease.Lease.OwnerPID == os.Getpid(): the whole chain runs in one process
// and every lease it writes carries its pid, while a different lane is by
// construction a different process — cycle 1364) AND its owner is live
// (runlease.OwnerLive: a sealed lane's lease outlives its process and stays
// fresh for a TTL — 2026-09-15, twice, with no lane running). Retention keeps
// its TTL-only Live on purpose (it errs toward keeping dirs). A dir Live
// through gc's other liveness source (the current workspace, no fresh lease)
// has no pid to compare and is NOT excluded — the fail-safe posture. A
// discovery error is unverifiable safety state.
```
