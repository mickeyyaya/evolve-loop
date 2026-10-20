# internal/builddiff

> Spec: [posthoc-schema.md](../posthoc-schema.md) ("The diff metrics are host-derived"). Decision: [ADR-0106](../adr/0106-logic-first-delivery.md) H3. Caller: `core.writeBuildDiffSummary` (`go/internal/core/build_diff_summary.go`). Inbox item: `posthoc-diff-anchor-no-op-cycle`.

## Purpose

`internal/builddiff` derives the diff of a cycle from its recorded base, and writes the diff into `build-report.md`. Before it, the report took its diff numbers from the tip commit. After the post-build soft reset that commit is the cycle base, so a no-op cycle showed the numbers of the prior ship commit (cycle 1479).

## Design

- **The anchor is the base of the run state.** `Derive(ctx, git, base)` runs `git diff --numstat --no-renames <base>` and `git diff --name-status --no-renames <base>` in the worktree. It adds each untracked file as an `ADD` with its line count. A binary, an unreadable or a non-regular untracked file counts 0 lines. A symlink or a file over `untrackedReadLimit` (1 MiB) is not read, and git does not quote a path (`core.quotepath=off`). Thus a cycle with no commit gives 0, one commit gives its numbers, and more commits give the sum, and an empty base is `ErrNoBase`.
- **The report rewrite.** `Apply(report, summary, base, deriveErr)` changes three parts. A POSTHOC row whose name is `files_changed`, `lines_added` or `lines_removed` (in any spelling with a space or an underscore) gets the value. The `- **Files changed:**` line gets the count. The first table of `## Changes` becomes the diff, with the description of the build for each path. Other rows and the prose of the build stay.
- **Each pass rewrites the host rows.** A row is found by the POSTHOC marker of the build or by the host marker of an earlier pass. Thus a build correction round or a remediation round writes the new numbers, and a failed later diff writes `absent`. Pinned by `TestApply_ASecondPassRewritesTheHostRowsAndAFailedSecondDiffStatesAbsent`.
- **A disclosed difference.** A path in the table of the build that is not in the diff is named in the line `Declared by the build but not in the diff`. A path in the diff with no row of the build gets `(not described by the build)`.
- **A fault is loud.** With `deriveErr`, each diff row reads `absent` and the cause, and `Apply` returns one warning. The table of the build stays.
- **Limit.** Only the heading `## Changes` is rewritten. A report that uses another accepted heading gets a second, host-derived `## Changes` section.

## Invariants

- **The base, not the tip.** Pinned by `TestDerive_AnchorsOnTheCycleBaseNotOnTheTipCommit`. With the anchor `HEAD^`, its no-commit case gives the 2 files of the prior commit.
- **A no-op cycle states zero.** Pinned by `TestApply_ANoOpCycleStatesZeroAndNeverThePriorCommitsStats`.
- **Never `pending`.** Pinned by `TestApply_ADeriveFaultWritesAbsentAndALoudWarningNeverPending`.
