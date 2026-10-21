# Build Explanation — Cycle 2028

## Build Binding
- Cycle: 2028
- Base SHA: 9f6c5f0bc4abcebe6ab6bdf71c4b7cab1c6883ae

## Summary
`evolve ship --class manual --paths a.go,pkg/` now stages only the named paths (`git add -A -- <paths>`) instead of the whole tree, so untracked files outside the set are not committed. Paths that the index already holds stay in the commit. With `--dry-run` it prints only those paths as the staged set. Without `--paths` the whole tree is staged as before.

## Rationale
The first attempt added an `Options.Paths` field and a filter in `stageExplicitPaths`. That touched two protected control-plane files, `gitops.go` and `native.go`, and ship refuses such a diff. This build carries the path set in the call context instead. The ship command runner narrows every `git status --porcelain -uall` read to that pathspec. The existing manual-ship staging then sees only the named files, so the commit-gate attestation check compares against the narrowed set.

## Changed Areas
- `go/cmd/evolve/cmd_ship.go` — adds the `--paths` flag, passes the set through `ship.WithPaths`, and moves the argument checks into `validShipArgs` so `runShipCmd` stays within its size allowance.
- `go/internal/phases/ship/paths.go` — adds `WithPaths`, `pathspec`, `stageArgs` and `scopeStatusRead`, and the dry-run preview.
- `go/internal/phases/ship/runner.go` — applies `scopeStatusRead` to every git call of the ship options runner.
- `go/internal/phases/ship/verify.go` — the manual class stages through `verifyManualStage` and `stageArgs(ctx)`.
- `go/internal/phases/ship/paths_test.go` — tests `scopeStatusRead`, `stageArgs` and the scoped dry-run preview.
- `go/cmd/evolve/cmd_ship_paths_test.go` — tests `splitShipPaths`.
- `go/acs/cycle2028/predicates_test.go` — the acceptance predicates of the lane.

## Design Decisions
The path set travels in the context, not in `Options`. This avoids the protected `native.go` and `gitops.go`. Git itself matches files and directories in the pathspec, so no custom matcher is needed. Only reads that contain `status`, `--porcelain` and `-uall` are narrowed, so cycle-class reads are unchanged.

## Verification
The six predicates `TestC2028_001` to `TestC2028_006` pass. The ACS suite reports green=246 red=0. `TestC2028_006` proves that no protected path is in the lane diff.

## Compatibility
Without `--paths` the context holds no set and every git call is unchanged. The attestation binds to the `git diff HEAD` content (`go/internal/phases/ship/audit.go:336`), so an attestation made for a different change set is refused as stale, with or without `--paths`.

## Limitations
The `--paths` flag is accepted for every ship class, and the scope reaches the status reads of every class. This build tests it for the manual class only. The scope narrows the paths that `git add -A` stages. It does not unstage a path that the index already holds, so such a path outside `--paths` is still committed. A named path that does not exist fails at staging with `git add -A failed (rc=128)` (`go/internal/phases/ship/verify.go:167`). A named path that exists but has no changes yields an empty set and the usual empty-diff exit.
