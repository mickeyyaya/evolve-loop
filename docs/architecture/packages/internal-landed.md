# internal/landed

## Purpose

`internal/landed` answers one question: is every change in a worktree since a merge-base already in `origin/main`? `evolve worktree cleanup --dev` removes a console dev tree only on this proof, or on two older ones: head in main, and a merged PR at the head. The procedure and the flags are in [workspace-layout.md](../../operations/workspace-layout.md). The package is a leaf: it depends only on `internal/gitexec`, `internal/sysexec` and `internal/plane` (for `plane.OriginMainRef`, the one spelling of the ref every judgement compares against; `plane` imports only the standard library). The checkpoint prune can therefore call the same proof instead of keeping a copy.

## API

| Export | What it does |
|---|---|
| `Changes(ctx, worktree, base) (Verdict, error)` | judges every tracked path `git diff --raw <base>` lists in the worktree |
| `ListUntracked(ctx, worktree) ([]UntrackedFile, error)` | names and `hash-object` hashes of the untracked, non-ignored files, in name order |
| `Untracked(ctx, worktree, files) (Verdict, error)` | each file must exist in `origin/main` with the same hash |
| `ParseCatFileBatch(specs, out) (map[string]Blob, error)` | reads one `git cat-file --batch` answer per spec |
| `Verdict`, `Blob`, `UntrackedFile` | the result (landed, or the reason it is not), a blob that may be absent, and an untracked file's name and hash |

## Design

- **Per path, not per patch.** `Changes` lists the changed paths with `git diff --raw -z --no-renames <base>`. It reads every merge-base and main blob in one `cat-file --batch`, and reads the worktree's own bytes from disk (`Lstat`, then `Readlink` for a symlink). `judgePath` then decides each path in this order:
  1. an unmerged path or a submodule (mode `160000`) is refused, because the proof does not judge it;
  2. a deletion counts only when main deleted the file too;
  3. a path main lacks is not landed;
  4. a mode change needs main's mode, read with `git --literal-pathspecs ls-tree`;
  5. equal bytes are landed;
  6. a binary path, a symlink, or a path the merge-base lacks counts only when equal;
  7. text needs a clean `git merge-file -p main base tree` whose output is main's bytes, exactly.
- **Why not `git apply --reverse --check`.** git apply searches for a hunk at shifted line offsets anywhere in the file. An edit whose text already exists in another, identical block of main's copy therefore passed as landed. The first build of `cleanup --dev` used it, and a dirty tree was force-removed in a test. The three-way merge aligns lines through diffs against the merge-base, so it never searches elsewhere in the file. Pinned by `TestChanges_AnEditWhoseTextExistsInAnIdenticalBlockElsewhereIsNotLanded`.
- **Later edits elsewhere are tolerated, overlapping ones are not.** A lane change that main carries, beside a later main edit in the same file, merges back to main's bytes. A later main edit on or next to the lane's lines conflicts, and the path is named. Pinned by `TestChanges_AChangeMainCarriesIsLandedEvenAfterMainEditedTheFileElsewhere` and `TestChanges_AnExtraUnlandedHunkIsNotLanded`.
- **Normalization.** Tracked bytes are compared raw, and untracked files are hashed through the clean filters. Under `core.autocrlf` or Git LFS, a tracked path is therefore refused even when it is landed. The error is always in the direction of keeping the tree.
- **Read-only.** The proof writes only a temporary directory for `merge-file`'s three inputs, and removes it. It writes no objects and no index. The caller supplies the git runner: `cleanup --dev` sets `GIT_OPTIONAL_LOCKS=0`, so judging a tree never refreshes its index.
- **Names git cannot express on one line are refused.** A changed or untracked path holding a newline cannot go through `cat-file --batch` or `hash-object --stdin-paths`, so it is an error, never a pass.

## Invariants

- **No false landed verdict for a deletion, a mode change, a binary, a symlink, a submodule or an unmerged path.** Pinned by `TestChanges_ADeletionIsLandedOnlyWhenMainDeletedTheFileToo`, `TestChanges_AModeChangeMainLacksIsNotLanded`, `TestChanges_ABinaryOrSymlinkChangeIsLandedOnlyWhenEqualToMain` and `TestJudgePath_AGitlinkOrAnUnmergedPathIsRefusedEvenWhenEqualToMain`.
- **An untracked file is landed only when main holds it with the same hash.** Pinned by `TestUntracked_AFileMissingFromOrDifferentInMainIsNotLanded` and `TestListUntracked_HashesEveryUntrackedFileInNameOrderAndSkipsIgnored`.
- **A malformed `cat-file` answer is an error, never a panic or a pass.** Pinned by `TestParseCatFileBatch_ReadsBlobsAndRefusesAMalformedAnswer`.
- **Mutation record (2026-10-06).** Each of these mutants is killed:
  - the per-path verdict is ignored;
  - the mode check is removed;
  - a deletion is treated as landed;
  - binaries and symlinks are merged instead of compared;
  - the submodule and unmerged refusal is removed.

## Follow-ups

- **The checkpoint prune converges onto this proof.** The checkpoint verb and this package are both on main, and the ref has one spelling, `plane.OriginMainRef`. `evolve checkpoint prune --landed` still uses `wtcheckpoint.hasLanded`, an equality-only check of a checkpoint's changed paths against main. Two steps remain, each a separately reviewed component:
  1. **A commit-tree mode.** A checkpoint is a full-tree commit, not a worktree. That mode was removed from this first version because nothing called it, and it comes back with its tests.
  2. **The prune calls the proof.** `wtcheckpoint.hasLanded` calls `Changes`. Whether the prune may loosen from exact equality to the merge proof is reviewed on its own, since a prune deletes a ref.

  Filed as the inbox item `checkpoint-prune-converges-on-landed-proof`.
