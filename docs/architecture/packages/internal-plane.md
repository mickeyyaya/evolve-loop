# internal/plane

## Purpose

`internal/plane` classifies which worktree plane a project root occupies (ADR-0080, runtime and console separation). The loop runs in a dedicated linked worktree. The primary checkout is the console of the operator. The boot-time classification is the standing tripwire against relaunching the loop into the tree of the operator.

## Design

- **Filesystem only.** `.git` is a directory in the primary checkout and a `gitdir:` pointer file in a linked worktree. HEAD is a one-line ref file in both. No git subprocess is used, so the package is hermetic, has no dependencies, and is callable at boot before any bridge or exec seam exists.
- **`Info`.** `IsLinkedWorktree` is true when `.git` is a pointer file (the runtime plane). `Branch` is empty on a detached HEAD. `GitDir` is the resolved git directory: the `worktrees/<name>` directory of the hub for a linked worktree, `<root>/.git` for the primary.
- **`headBranch`.** It parses `ref: refs/heads/<name>`. A detached HEAD or an unreadable HEAD gives `""`. For classification, no branch is data and not an error.
- **`CommonGitDir`.** It resolves the shared git directory (the hub): the `.git` of the primary, or the gitdir of a linked worktree walked through its `commondir` file. The hub sits outside every worktree. The console lease lives there so that no lane phase can author or shadow it from inside a checkout (ADR-0080 S4).
- **`ConsoleLeaseFileName`** is the one name that the lease writer (`cli/opscmd`) and the reader (`internal/core`) must agree on. With two hardcoded copies, a rename silently disarms every lease.
- **`BootLine`** is the one-line report that the loop prints at launch. A linked worktree is informational. The primary checkout carries the ADR-0080 warning.

## Invariants

- **The `gitdir:` prefix is required.** A `.git` file without the prefix makes `Classify` return an error (`TestClassify_ForeignDotGitFileErrors`). Trimming without a check made any non-empty `.git` file read as a linked worktree. The tripwire then failed open in the primary checkout, which is the one direction that matters.
- **A relative pointer resolves against the checkout.** Git 2.48 `worktree.useRelativePaths` and submodule checkouts write relative pointers. Resolved against the process working directory, `Branch` went blank and the S3 sync became a permanent no-op.

## Findings

- Batch 15 lost four lanes (1149 to 1152) to operator activity in a shared checkout. The boot classification and the warning in `BootLine` are the response.
