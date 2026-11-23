# Build Explanation — Cycle 2576

## Build Binding
- Cycle: 2576
- Base SHA: 63f36a9357898ca756cd09e7f945ceb628905b29

## Summary
A salvaged cycle now keeps its branch on `refs/archive/cycles/<leaf>`, and its leaf has a hashed manifest. The new verb `evolve archive restore <cycle>` rebuilds the branch, the patch and the untracked files.

## Rationale
The salvage leaf held files with no proof of their content, and the branch stayed as a plain `cycle-*` branch that other sweeps can delete. A manifest hash and a compare-and-delete give one verified home for the failed cycle.

## Changed Areas
- `go/internal/gc/archive.go` — new: manifest writer, archive ref, compare-and-delete, leaf verification and `RestoreArchive`.
- `go/internal/gc/salvage.go` — a salvage-remove refuses a branch that moved since the plan, writes the manifest, and archives the branch after the worktree goes.
- `go/internal/gc/worktrees.go` — the plan records the scanned branch tip in `Head` of a salvage-remove item.
- `go/internal/gc/worktrees_salvage_test.go` — the round trip test expects the archive ref and restores through `RestoreArchive`.
- `go/internal/gc/salvage_backfill_f1_test.go` — a clean leaf now also holds the two manifest files.
- `go/internal/gittest/fixture.go` — `Repo.Git` now trims only trailing space. A status line keeps its first column. The old full trim broke the status checks of the round trip predicates.
- `go/cmd/evolve/cmd_archive.go` — new: the `archive restore` verb.
- `go/cmd/evolve/registry.go` — registers the verb.
- `docs/architecture/packages/internal-gc.md` — documents the archive ref and the manifest.
- `docs/architecture/packages/cmd-evolve.md` — documents the verb.

## Design Decisions
The manifest is written after the salvage files, from the files on disk, so it cannot list a file that is absent. The dev archive keeps its branch and writes no manifest. The compare-and-delete uses `git update-ref -d <ref> <tip>`, so git itself refuses a moved ref. Restore takes the commits from the archive ref, the object store or the bundle, in that order.

## Verification
The cycle predicates `TestC2576_001` to `TestC2576_009` pass after the `Repo.Git` trim fix (`go test -count=1 -tags acs ./acs/cycle2576`).

## Compatibility
A leaf from an earlier version has no manifest, and restore refuses it by name. A plan file with no `Head` keeps the branch, as before.

## Limitations
A cycle with several leaves is refused, not guessed. Restore needs the cycle number, not the leaf name.
