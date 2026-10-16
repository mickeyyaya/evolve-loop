# internal/pruneephemeral

## Purpose

`internal/pruneephemeral` ports `legacy/scripts/observability/prune-ephemeral.sh`. It applies TTL retention to the ephemeral data of the phase tracker.

## Design

- **Safe to repeat.** The prune is idempotent. It is safe to run while a cycle is in progress, because the mtime filter protects recently modified files.
- **Retention policy.** The defaults can be changed through the environment.

| Path | Retention |
|---|---|
| `.evolve/runs/cycle-*/.ephemeral/` | 7 days (`EVOLVE_TRACKER_TTL_DAYS`) |
| `.evolve/dispatch-logs/batch-*.log` | 30 days (`EVOLVE_DISPATCH_LOG_TTL_DAYS`) |
| `.evolve/runs/cycle-*/*.md` | never pruned |
| `.evolve/runs/cycle-*/*.json` | never pruned |
| `.evolve/ledger.jsonl` | never pruned (append-only and tamper-evident) |

- **Options.** `ProjectRoot` is required. `RunsDir` and `DispatchLogsDir` are optional overrides. They default to `<ProjectRoot>/.evolve/runs` and `<ProjectRoot>/.evolve/dispatch-logs`. `TrackerTTL` defaults to 7 days and `DispatchLogTTL` to 30 days.
- **Dry run.** `Result` counts the entries pruned, or, in a dry run, the entries that it prunes if it runs for real. `EphemeralPaths` is filled in a dry run for audit.
- **Exit codes.** The cmd layer maps them: 0 for success (also when nothing was pruned) and 10 for bad arguments.

## Invariants

- Only the two paths in the table are ever removed. Pinned by `pruneephemeral_test.go`.
- A file newer than its TTL stays. Pinned by `pruneephemeral_test.go`.

## Findings

- None open.
