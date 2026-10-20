# internal/treefence

> Related: [the incident record](../../incidents/2026-09-03-auditor-mutates-the-worktree.md) and [internal-phases-runner-verdict.md](internal-phases-runner-verdict.md). This page keeps the package-level detail that the code cannot say.

## Purpose

`internal/treefence` is a content fence around the worktree of a read-only phase. A phase that is not a declared source writer (registry `writes_source=false`: audit, adversarial-review, retrospective and others) must hand downstream the exact tree it was given.

## Design

- **Reading a snapshot entry.** `writeFromTree` runs `git ls-tree -z` and parses one entry shaped `<mode> <type> <object>\t<path>\0`. It takes the mode and object from the fields before the tab. Mode 160000 (a submodule) is refused, 120000 is written as a symlink and 100755 as executable.
- **Why a fence.** The agents behind those phases have a shell. Cycles 1603, 1604 and 1605 each failed the deterministic build-explanation gate. The mutation probes of the auditor rewrote the material files of the builder in place (`cp /tmp/digest.go.bak ...`, "wrote reverted tdd.go", probe tests dropped into the material packages). The sealed diff digest then no longer matched the tree. No LLM round can repair a defect that the auditor itself introduced.
- **A snapshot is a git tree object.** `Take` records tracked changes and untracked files, with `.gitignore` respected, through a throwaway index seeded from the real one. Only changed paths are re-hashed, and the real index is never touched.
- **Restore diffs a fresh tree against the snapshot** (`diff-tree -z`) and undoes every difference. Added paths are removed. Modified, deleted and retyped paths are written back from the snapshot with their mode. The directories that the phase created around its files are pruned.
- **Ignored paths are outside the fence by construction.** Build outputs and the `.evolve/` scratch of the worktree are in this set. It is the same set that the explanation digest ignores.
- **Two dispatch surfaces hold one `Fence`.** The shared phase runner and the retro phase's own runner both call the bridge. `Begin` runs before the agent and `End` runs after it, so the mechanism cannot be wired on one surface and forgotten on the other.
- **Retract keeps the evidence, then restores** ([ADR-0149](../adr/0149-a-test-writing-phase-that-states-fail-on-its-own-work-is-retracted-to-its-dispatch-tree.md)). `Snapshot.Retract` diffs a fresh tree against the snapshot, copies each path that still exists to the keep directory, and then calls `Restore`. A path that the phase deleted has no content to keep. The shared runner calls it for a test-writing phase that states FAIL on its own work.

## Invariants

- **No silent no-op.** `Take` fails on an empty path or a non-repository. The caller decides how loudly to fail open.
- **Every entry is attempted.** `Restore` continues after a failure, joins the failures and returns them, so the caller reports each by name. A tree that still matches is a no-op.
- **A malformed listing is an error.** `diff` never returns a silently shortened list.
- **Removals run first.** A path that the phase turned from a file into a directory (or the reverse) must be cleared before its other form is written back. The order in which git lists the two entries does not matter. Pinned by `TestRestore_UndoesTypeChangesInBothDirections`.
- **Pruning stops at a non-empty directory** and never removes the root. A path that is already gone, or whose parent was restored to a file, is nothing to do (`removeAdded`, `pruneEmptyParents`).
- **Only a real restore authenticates the tree.** `Outcome.Verified` is true only when a snapshot was restored successfully. An inert fence, a failed snapshot or a failed restore cannot authenticate the worktree. A temporary `.gitignore` change of the phase can hide additions, and restoration can reveal them. So `End` hashes the final tree again and does not trust the write success alone. Pinned by `TestFenceEnd_DistinguishesInertTakeErrFromGenuineEmptyRestore`.
- **The fence reports; the gates decide.** `Diagnostics` is silent for a clean phase. It warns with the restored paths for a writing phase. It warns with the failure when the fence failed to do its job. It is never an error severity. A source writer (`readOnly` false) or a dispatch without a worktree gets an inert fence.
- **Ambient git environment is ignored.** `GIT_DIR` and `GIT_WORK_TREE` of the parent process must never redirect the fence to another repository. Pinned by `TestTake_IgnoresAmbientGitEnvironment`.
- **Restore is idempotent.** A second restore over a restored tree changes nothing.
- **A retraction that cannot keep its evidence restores nothing.** The tree stays as the phase left it, so the gates judge the same tree as before ADR-0149. Pinned by `TestRetract_AnEvidenceFailureRestoresNothing`, with the restore itself pinned by `TestRetract_KeepsThePhaseWritesAsEvidenceAndRestoresTheDispatchTree`.

## Findings

- **Cycles 1603 to 1605**: the auditor's probes mutated the builder's files and the sealed digest no longer matched. The fence was added (2026-09-03).
