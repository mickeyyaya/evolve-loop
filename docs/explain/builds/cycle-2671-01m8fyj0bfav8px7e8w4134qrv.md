# Build Explanation — Cycle 2671

## Build Binding
- Cycle: 2671
- Base SHA: d1e5d509cef378dd5911e5391418c9bceb5e8cad

## Summary
`evolve gc` now reads `.evolve/dispatch-trees` once at the start of `dispatchProcesses` and shares that snapshot between the process pass and the tree pass. The `only the tag proof applies` warning prints at most once per dispatch ID per run across all passes. `gcRun.recordedTree` also caches direct lookups and deduplicates its warning per dispatch ID so `reapDispatches` shares the same per-run guarantee.

## Rationale
Before this change, `recordedTree` called `proctree.LoadTree` on each `StaleDispatch` evaluation across planning, unreaped reporting, reaping, and stale-tree selection. `treeFiles` then called `proctree.ListTrees` again. A tree file written after the process pass can be removed without a fresh process evaluation. A corrupt tree file emitted the tag-proof warning up to three times per run.

## Changed Areas
- `go/cmd/evolve/cmd_gc_dispatch.go` — adds `dispatchTreeSnapshot`, `loadDispatchTrees`, `recordedTree`, `warnOnce`, and `warningFor` to load `.evolve/dispatch-trees` once in `dispatchProcesses`. `treeFiles` reuses that snapshot and `warnOnce` prints the tag-proof warning once per dispatch ID. `gcRun.recordedTree` also caches direct lookups and warnings per ID.
- `go/cmd/evolve/cmd_gc_dispatch_test.go` — adds `TestGCDispatchTreeSnapshot_WarnsOncePerDispatchOnUnreadableDirAndCachesDirectLookup` to verify single-warning and cached-lookup behavior on unreadable tree directories and repeated direct `recordedTree` lookups.

## Design Decisions
Keeping the snapshot and warning deduplication inside `go/cmd/evolve/cmd_gc_dispatch.go` avoids changes to `proctree.ListTrees` and keeps the diff local. Matching `ListTrees` warnings by `proctree.TreeFile` prefixes preserves the `only the tag proof applies` warning on corrupt files while absent files stay silent.

## Verification
`gofmt -l`, `go vet ./cmd/evolve`, targeted `cmd/evolve` tests (`TestGCDispatch*`, `TestGCRecordedTree*`, `TestReapDispatches*`, `TestLaneReap*`), and `evolve acs suite --cycle 2671` pass (`green=265 red=0 skip=53 total=318`), including `TestC2671_001_GCReadsTheDispatchTreeRecordsOncePerRun` and `TestC2671_002_TheTagProofWarningPrintsOncePerTaggedProcessPerRun`.

## Compatibility
`gcRun.recordedTree`, `gcRun.reapDispatches`, and `proctree.ListTrees` keep their existing signatures and fallback behavior. Tagged processes without a tree file still use the tag proof with no warning. Corrupt tree files still stay on disk with one file-stays warning and one tag-proof warning.

## Limitations
`warningFor` relies on the error message prefixes emitted by `proctree.readTreeRecord` to associate a `ListTrees` warning with a specific dispatch tree file path.
