# 2026-11-19: agent run deleted the reverted patch of two builder seats

Related documents: the fence is `go/internal/treefence/fence.go`, the run is `go/internal/cli/agentcmd/run.go`, and the procedure is the [issue-record](../../skills/issue-record/SKILL.md) skill.

## Summary

- **Impact:** two console-builder runs lost their code changes. The console ran each build a second time.
- **Root cause 1:** the console gave `--writable-dir` for directories of existing files. That flag allows only new files, so the fence reverted each change to an existing file.
- **Root cause 2:** the fence saved the reverted change to `fence-reverted.patch` in the run workspace and printed that path. `evolve agent run` then removed the workspace on exit, so the patch was gone.
- **Fix:** the console gives one `--writable` for each existing file. The inbox item `agent-run-deletes-the-fence-reverted-patch` keeps the patch past the run.
- **Lesson:** a WARN that names a recovery file must name a file that still exists when the command returns.

## Terms used throughout

- **Fence:** the worktree fence of a seat. After the seat ends, it reverts each write outside the paths that the run may write.
- **Run workspace:** `.evolve/ephemeral/agent-runs/<run label>/` under the project root of the run.

## Timeline

| Time (UTC, 2026-11-19) | Event | Evidence |
|---|---|---|
| 12:10 | The console starts the F5b builder with `--writable-dir` for the touched packages. | run `agent-console-builder-20261119T121043Z-5d75` |
| 12:12 | The console starts the reportdoc builder with `--writable-dir go/internal/reportdoc`. | run `agent-console-builder-20261119T121245Z-05ff` |
| 12:1x | Each run ends. The fence reverts the changes to existing files and prints a patch path. Each run exits with code 12. | the stderr of each run |
| 12:18 | `git apply` of the printed path fails: the file does not exist. A search of the disk finds no patch. | `can't open patch` |
| 12:20 | The console files the inbox item and runs each build again with one `--writable` for each file. | `2026-11-19T12-20-17Z-agent-run-deletes-the-fence-reverted-patch.json` |

Key log of run `agent-console-builder-20261119T121245Z-05ff` (paths shortened):

```text
[runner] WARN worktree fence: read-only phase console-builder wrote 3 path(s) into its worktree; restored to the dispatched tree: go/internal/cli/agentcmd/agent_verdict_test.go, go/internal/reportdoc/severity_counts.go, go/internal/reportdoc/severity_counts_test.go (reverted patch: runtime/.evolve/ephemeral/agent-runs/agent-console-builder-20261119T121245Z-05ff/fence-reverted.patch)
error: can't open patch '.../agent-console-builder-20261119T121245Z-05ff/fence-reverted.patch': No such file or directory
```

## Root cause

**F1.1** The usage text of `evolve agent run` says that `--writable-dir` names "each directory that it can add new files to". The console read it as a grant for the whole directory.
*Implication:* each change to an existing file under that directory was outside the grant.

**F2.1** `Outcome.savePatch` writes the patch to `filepath.Join(o.workspace, patchFileName)` (`go/internal/treefence/fence.go:531`).
*Implication:* the patch lives in the run workspace.

**F2.2** `evolve agent run` defers `removeWorkspace(r.workspace, stderr)` (`go/internal/cli/agentcmd/run.go:206`).
*Implication:* the workspace and the patch are removed after the fence prints the path. The WARN names a file that does not exist.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| The patch stays at the path that the WARN prints | inbox item `agent-run-deletes-the-fence-reverted-patch` | open |
| The usage text says that `--writable-dir` allows only new files, and that each existing file needs `--writable` | inbox item `agent-run-deletes-the-fence-reverted-patch` (its fix names the usage text) | open |

## Limits

- The times, the run ids and the `git apply` failure come from the console session of 2026-11-19. They are not in git. The inbox item is in the runtime inbox, not in git.
- The end time of each first run is not recorded here. The stderr files of the console hold no time stamp.
- The second runs of both builds are still running when this record is written.

## What we learned

- A recovery path in a WARN must outlive the command that prints it.
- Give a builder seat one `--writable` for each existing file that it changes, and `--writable-dir` only for new files.
