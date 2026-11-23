# Build Explanation — Cycle 2613

## Build Binding
- Cycle: 2613
- Base SHA: a099157d14664c4483d0db2009b6551b6ded79d6

## Summary
`TrimGoCache` counts, trims, and reports executable `-d` directories in the Go build cache.
A live-holder probe spares any directory whose files an active process holds.

## Rationale
`listShard` previously skipped every directory entry, so cached executable trees escaped both the age trim and the size cap.
Reusing `TempOption` and `SparingHeld` lets `TrimGoCache` spare open cache trees in the same way `ReapPipelineTemp` does.

## Changed Areas
- `go/internal/gc/gocache.go` — walks `-d` directories to sum regular file sizes, tracks the newest inner file mtime, re-checks mtime on release, and honors `SparingHeld`.
- `go/cmd/evolve/cmd_gc.go` — passes `LiveHolderProbe` through `SparingHeld` and routes directory removals to `removeAll`.

## Design Decisions
A `-d` directory counts as one entry. Its size sums regular files, and its age comes from the newest inner file or directory mtime when empty.
`TrimGoCache` takes variadic `opts ...TempOption` so prior three-argument callers compile without change.
`selectOverCap` skips held entries instead of stopping at them, so unheld older entries still satisfy the byte cap.
`cmd_gc.go` dispatches directory paths to `removeAll` and file paths to `remove`, which keeps prior file-removal test stubs intact.

## Verification
All eight cycle-2613 ACS predicates pass, along with `internal/gc` and `cmd/evolve` Go build cache tests.

## Compatibility
Three-argument `TrimGoCache` callers and the `CacheTrimReport` JSON schema remain unchanged.

## Limitations
Liveness checks rely on one `lsof` snapshot per run; when `lsof` fails, `TrimGoCache` spares entries and records the error.
