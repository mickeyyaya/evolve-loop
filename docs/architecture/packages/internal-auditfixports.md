# internal/auditfixports

> Decision: [ADR-0158](../adr/0158-a-repair-round-runs-only-for-a-finding-the-lane-can-fix-and-the-auditor-applies-a-small-prescribed-fix.md) decisions 8 to 13. Design: [retry-ladder.md](../retry-ladder.md) §5. Plan: [cost-aware-repair-2026-10](../../plans/cost-aware-repair-2026-10.md) item 11. Handler: [internal/auditfix](internal-auditfix.md).

## Purpose

`internal/auditfixports` builds the four production ports of the `audit_fix` handler. The handler in `internal/auditfix` is pure: it decides, and the ports act. Before this package, no port had a production implementation, so a chosen `audit_fix` never ran.

The operator asked on 2026-10-27: "make sure audit will involve fix and trigger another review for its changes; balance cost and output".

## Design

| Port | Type | What it does |
|---|---|---|
| `Auditor` | `Fixer` | `Seat` routes the `console-builder` profile. `ApplyFix` runs `evolve agent run console-builder` on the cycle worktree. Each cited file is a `--writable` path, and `audit-report.md` is a `--file`. The input is a brief of the prescriptions and the bounds. The reason of each changed path is the first prescription that cites it. |
| `Reviewer` | `Reviewer` | `ReviewSeat` routes each candidate profile and returns the first one whose first family differs from the family of the fixer. `Review` writes the patch and the prescriptions into `audit-fix-review/` of the run directory and runs that profile on the two files only. |
| `Gates` | `Gates` | `Run` reads `run.json` of the run directory (`core.ReadRunCycleState`) and projects it with `core.ReviewInputFor` for the build phase. Then it runs the whole build handoff floor and the ACS of the cycle (`acssuite.Run`) on the worktree. |
| `Worktree` | `Worktree` | `Snapshot` writes the tree of the whole worktree through a copy of the index (`gitexec.TreeOfIndexCopy` with `add -A`). `Diff` compares that tree with a new snapshot. `Restore` puts back each changed path from the snapshot tree. |

- Each LLM call goes through `evolve agent run`, as the in-process `AgentRun` that the composition root gives (`runAgentSeat` of `cmd/evolve`). The package starts no process and imports no bridge.
- The review verdict is the typed `Verdict` field of the `agentcmd.Envelope`. `agentcmd` derives it from the declared `Verdict` section and the severity counts of the report. PASS and WARN pass. FAIL, or no verdict, gives one reason with the run label, the family and the CRITICAL and HIGH counts. A FAIL verdict exits non-zero. `Review` thus reads an error with a FAIL verdict and no envelope error as a verdict, not as a dispatch failure.
- `RouterRoute` turns a `cliroute` resolve into a `Route`: the family of the first candidate CLI and the tier of the plan. The composition root passes `cliRouter.Resolve`.
- `Snapshot` never writes the real index. The cycle worktree holds the uncommitted build, so the snapshot holds the build, and the diff holds only the change of the fixer.
- `Restore` removes a file that the fixer added. For each other path it runs `git restore --source=<snapshot> --worktree`, so the index of the worktree does not change.
- Each `git diff` runs with `core.quotePath=false`, so git does not quote a non-ASCII path. The cited-file check and the restore thus see the real path.
- A binary change counts 1 line for each side of the numstat, so a binary file never measures 0 lines against the bounds.

## Invariants

- **The diff is the change of the fixer only, and the restore returns the built tree.** `TestWorktree_DiffMeasuresOnlyTheFixAndRestoreReturnsTheBuiltTree` builds a committed base and an uncommitted build. Then it modifies, adds and deletes files. It checks the measured changes, and that the restore gives the built files and the same `git status`.
- **The review seat is of another family.** `TestReviewer_PicksTheFirstProfileOfAnotherFamily` and `TestReviewer_GivesNoSeatWhenEveryProfileIsOfTheFixerFamily`.
- **The fixer writes only the cited files.** `TestFixer_RunsTheConsoleBuilderOnlyOnTheCitedFilesOfTheWorktree` pins the exact `evolve agent run` arguments.
- **The gates read the binding of the cycle.** `TestGates_RunTheFloorAndTheCycleACSOnThePatchedWorktree`.
- **Each failure is loud.** `TestWorktree_EachGitFailureIsLoud`, `TestFixer_ErrorsAreLoud`, `TestReviewer_ADispatchErrorOrAnUnwritableWorkspaceIsLoud` and `TestGates_AMissingRunStateOrAnACSErrorIsLoud`.
- **The handler works on a real git worktree.** The integration tests (`-tags integration`) run `auditfix.Handler` with the real `Worktree` and fake seats. A passed patch stays. A failed review restores the tree and then stops. An uncited file is refused and removed.

## Limits

- The fixer is not the loaded audit session. Each phase dispatch is an ephemeral bridge session, and no phase profile names a session (`bridge/launch.go`, `launchIntentFor`). So the audit session ends with its dispatch. The closest supported continuation is a fresh writer seat with the audit report as its input. The only writer profile of `evolve agent run` is `console-builder`.
- `evolve agent run` takes no family and no tier. The review port predicts the family from the routing. The router can walk to another CLI at launch, so the family of the run can differ from the predicted family. The record keeps the predicted seat.
- The tier of the review is the tier of the routing of the profile. ADR-0158 decision 10 asks for the deep tier.
- `AgentRun` drops the context: `evolve agent run` installs its own signal context.
- The candidate review profiles are `code-reviewer`, `go-reviewer` and `adversarial-review`, in this order (`cmd/evolve/audit_fix_wiring.go`).
