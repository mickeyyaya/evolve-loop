# Build Explanation — Cycle 1936

## Build Binding
- Cycle: 1936
- Base SHA: 7cce1ae6cf82d4331ae6ef08803fbcf1693dac22

## Summary
Two lanes ship together. The walk-delta guard (`internal/treewalk`, `evolve treewalk`, `make test`) fails a test run that leaves any new file or empty directory under `go/`. Slice 1 of the knowledge-base graph (`internal/kbgraph`, `evolve kb graph neighbors|recurrences`) projects cycle dossiers and inbox items into a read-only in-memory graph.

## Rationale
`git status` cannot show an empty directory, so the empty `.evolve/runs/cycle-0` leaks went unseen. A before/after path walk shows them, and it needs no allowlist. The graph slice reuses `dossier.ReadCommitted`, so the dossier schema stays the single source. It keeps the caps at 2 hops and 40 nodes, and it reports a missing edge kind as ABSENT instead of inventing the edge.

## Changed Areas
- `go/internal/treewalk/treewalk.go` — `Take`, `Diff`, `Snapshot.Write` and `Load`: a sorted path snapshot and the paths a later walk adds.
- `go/cmd/evolve/cmd_treewalk.go` — the `evolve treewalk snapshot|check` verbs; `check` prints each `LEAK` path and exits 1.
- `go/Makefile` — the `test` target takes a snapshot, runs `go test`, runs the check, and keeps the `go test` status.
- `go/internal/kbgraph/kbgraph.go` — `Build`, `Graph.Neighbors` and `Graph.Recurrences` over `knowledge-base/cycles` and `.evolve/inbox`.
- `go/cmd/evolve/cmd_kb.go` — the `evolve kb graph neighbors|recurrences` verbs, with `--hops`, `--json` and `--project-root`.
- `go/cmd/evolve/registry.go` — registers `treewalk` and `kb`.
- `go/cmd/evolve/main.go` — lists both verbs in the usage text.
- `go/.apicover-enforce` — enrolls `internal/treewalk` and `internal/kbgraph`.
- `go/internal/treewalk/treewalk_test.go` — table test of the diff and the malformed-baseline cases.
- `go/internal/treewalk/apicover_named_test.go` — names every export of the package.
- `go/internal/kbgraph/kbgraph_test.go` — hop table, malformed inbox item and id-less item.
- `go/internal/kbgraph/apicover_named_test.go` — names every export of the package.
- `go/cmd/evolve/cmd_kb_treewalk_test.go` — verb-level tests of both commands, including bad invocations.
- `go/acs/cycle1936/predicates_test.go` — the TDD phase's predicates, unchanged by the build.

## Design Decisions
`Diff` reports additions only, because a leak is a new path; a removed path is not a leak. The graph stays in memory and writes nothing. Rejected: an allowlist of known leaks (it hides the next leak), and reading the lesson YAML files in slice 1 (the dossier already carries lessons; slices 2-3 stay queued).

## Verification
`go test -tags acs -count=1 ./acs/cycle1936` passes all predicates. `go test -count=1 ./internal/treewalk ./internal/kbgraph ./cmd/evolve` passes. `make test` now reports the two leaks that remain (see Limitations).

## Compatibility
No existing command, flag or schema changes. `make test` can now fail on a leak it did not fail on before.

## Limitations
The guard finds a real leak: `internal/loopwave/launcher.go:48` builds `inboxmover.Options` without `ProjectRoot`, so two loopwave tests and one `cmd/evolve` test write `.evolve/ledger.*` into the package directory. The control-plane role gate denied the one-line fix in this cycle; it needs `evolve ship --class manual`.
