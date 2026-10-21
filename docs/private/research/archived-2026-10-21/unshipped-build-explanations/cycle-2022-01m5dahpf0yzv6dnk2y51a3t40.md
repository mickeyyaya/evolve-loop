# Build Explanation — Cycle 2022

## Build Binding
- Cycle: 2022
- Base SHA: f7b28ac26df8076e89a0f6d8d557fb8fcbf4327f

## Summary
`evolve ship --class manual` takes a new `--paths a,b` flag that stages and commits only the named paths. `--dry-run` prints the staged set and no longer stages anything.

## Rationale
The manual ship ran `git add -A` twice: once in the verify stage and once in `stageExplicitPaths`. Both calls take the whole tree. The flag therefore limits both: the verify stage adds only the named pathspec, and `stageExplicitPaths` keeps only the changed files inside the named paths. The attestation hash is `git diff HEAD` of the index, so an attestation for a different set stays stale and is refused as before. A runner wrapper that rewrites `git add` arguments was rejected: it would hide the change from the code that owns it.

## Changed Areas
- `go/internal/phases/ship/paths.go` — new `verifyManualStage`, `previewStagedSet`, `pathspec`, `stageArgs` and `withinPaths`: a dry run lists the changed paths inside the named set and leaves the index alone.
- `go/internal/phases/ship/verify.go` — the manual class calls `verifyManualStage`, and `verifyManualConfirm` stages through `stageArgs`.
- `go/internal/phases/ship/gitops.go` — `stageExplicitPaths` keeps only the changed paths inside `--paths`; when none match it stages nothing, because `git add -A --` with no pathspec would take the whole tree.
- `go/internal/phases/ship/native.go` — new `Options.Paths` field.
- `go/cmd/evolve/cmd_ship.go` — new `--paths` flag and `splitShipPaths`, which feeds `Options.Paths`.
- `go/internal/phases/ship/paths_test.go` — tests for the dry-run preview, for `withinPaths` and for the path-limited preview.
- `go/cmd/evolve/cmd_ship_paths_test.go` — table test of `splitShipPaths`.

## Design Decisions
The dry-run branch sits in a wrapper, not in `verifyManualConfirm`, because that function is at its size-ratchet ceiling. A named directory matches by whole path segment, so `pkg` does not match `pkg2/c.go`.

## Verification
`TestC2022_001` to `007` pass against a built binary in a real git repository. The ship package and command tests pass with `-count=1`.

## Compatibility
A ship without `--paths` behaves as before. A dry run no longer reaches the commit-prefix gate, because nothing is staged.

## Limitations
`--paths` takes comma-separated exact files or directories; globs are not matched.
