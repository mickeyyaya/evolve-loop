# Build Explanation — Cycle 2593

## Build Binding
- Cycle: 2593
- Base SHA: a1d94de45f161e7196b00f806015f048d8447aba

## Summary
A failed cycle now leaves a hashed archive leaf. Its branch moves to an archive ref with a compare-and-delete. The new verb `evolve archive restore` rebuilds the branch, the patch and the untracked files.

## Rationale
The salvage leaf already held the HEAD, the patch, the untracked files and a bundle. It had no integrity record and no way back. A manifest hash and a restore verb close that gap with the smallest new surface.

## Changed Areas
- `go/internal/gc/archive.go` — adds the leaf manifest and its hash, the archive ref move, the leaf verification and `RestoreArchive`.
- `go/internal/gc/salvage.go` — writes the manifest after the leaf and moves the branch only if its tip still equals the scanned tip.
- `go/internal/gc/worktrees.go` — records the scanned branch tip on each salvage item, so the delete has an expected value.
- `go/cmd/evolve/cmd_archive.go` — adds the `archive restore` command that calls `RestoreArchive`.
- `go/cmd/evolve/registry.go` — registers the `archive` verb.
- `go/internal/gittest/fixture.go` — keeps leading whitespace of git output in the fixture helper (`TrimRight` instead of `TrimSpace`), so output compares accurately.

## Design Decisions
The archive ref holds the commits. The bundle in the leaf is the fallback when the ref and its objects are gone. Restore never overwrites an existing branch. Restore refuses a leaf file or a manifest whose hash differs.

## Verification
The ACS predicates `TestC2593_001` to `TestC2593_009` verify the round trips, the refusals, the compare-and-delete and the hashes. Unit tests in `internal/gc`, `cmd/evolve` and `internal/gittest` pass.

## Compatibility
Active leaf files keep their names. The manifest and its hash are new files in the leaf.

## Limitations
Restore needs a plain parent path for the untracked files. It does not merge into an existing branch.
