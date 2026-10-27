# internal/cli/agentcmd

> The design of record is [agent-run](../agent-run.md), decided in [ADR-0152](../adr/0152-one-phase-agent-runs-from-the-command-line-with-no-cycle-through-the-phase-runner.md). This page keeps the package detail.

## Purpose

`internal/cli/agentcmd` holds `evolve agent run <profile>`, `evolve agent list` and `evolve agent capabilities`. A person or a program runs one phase agent with an input on stdin and gets the report on stdout, with no cycle. A review profile is read-only. The build seat `console-builder` writes only the `--writable` files of a linked worktree ([agent-run.md](../agent-run.md) §8).

## Design

- **`New(Deps)`** returns the standard subcommand function. `Deps` has seven seams:

| Seam | Default |
|---|---|
| `InstallRouter` | none; cmd/evolve passes `rootRouterInstaller` |
| `Bridge(root, signals)` | `defaultBridge`: the Center-less `phasecmd.DefaultBridge`; cmd/evolve passes `agentRunBridge`, which is `bridge.NewDefault` with the Center of the run |
| `Signals(root, stderr)` | `noSignals`: no Center; cmd/evolve passes `agentRunSignals`, which is `newRoleSignalCenter` with the role `agent` |
| `TreeState(ctx, dir)` | `defaultTreeState`: `treestate.SHA` through `sysexec.DefaultRunner` |
| `Getenv` | `os.Getenv` |
| `Now` | `time.Now` |
| `VerbSchema(path)` | no schema, so each `A` verb shows `text/1`; cmd/evolve passes `coreVerbSchema`, the envelope schema of each core verb |

  Tests inject a fake bridge and a fake tree state, so no test launches a real CLI or reads a real tree.
- **One dispatch path.** The run builds its runner with `phasecmd.NewSpecRunner`, the constructor that `evolve phase` uses. The routing table, the fallback walk, the sandbox and the verdict all come from the shared phase runner.
- **Admission** (`admit`) runs before the input is read. Each profile check fails closed.
  - A dispatch depth above 0 (`subagent.ReadDispatchDepth`) is refused first, so a phase agent cannot start a nested run.
  - A built-in phase is refused (exit 3; exit 2 before X17b), because each one reads or writes cycle state. A spec that `phasecmd.SpecRunnable` rejects is unknown (exit 10).
  - `checkWorktree` refuses a relative `--worktree` (exit 10) and a directory that is not a linked worktree (exit 3).
  - A spec is a writer when `core.PhaseWritesSource` marks it, or when its agent profile is not confined to the run workspace (`writesSource`, `confinedToWorkspace`).
  - `admitWriter` refuses a writer (exit 3) unless `confinedToWorktree` holds. The grants must be under `{worktree_path}` or the run workspace, and the sandbox must deny `.evolve/agent-runs` and `.commit-gate`.
  - An admitted writer needs `--worktree` and at least one clean `--writable` path (exit 10). A reader with `--writable` is exit 10.
- **Input** (`composeInput`). `--input FILE` wins over stdin, and `-` means stdin. Each `--file PATH` appends `### <path>` and a fenced block. The fence is longer than each backtick run in the file (`fence`). Each source stops at 8 MiB (`readLimited`), and so does the composed input. Empty or oversized input is exit 11.
- **A terminal on stdin** is not read. The check is for a character device, so `/dev/null` counts as a terminal. With `--file` the files are the input, else it is a usage error.
- **The run workspace** is `.evolve/ephemeral/agent-runs/<label>`. `os.Mkdir` creates the label directory, so a label collision fails (exit 21) and never shares the workspace of another run. The workspace is removed at the end unless `--keep`. A failed removal is named on stderr, and the exit code stays the code of the run.
- **The request** has cycle 0, `RunID` = the label, and the input under `specrunner.AgentInputKey`. With no worktree the request has no fence, and the bridge grants no Git writes. With `--worktree`, the request has the worktree and a fence: an empty fence for a reader, the `--writable` paths for a writer.
- **The report** is `<workspace>/specrunner.ArtifactName(spec)`, the name that the runner judges.
- **The ledger entry** is appended after every dispatch, also after a dispatch error. It has cycle 0, `cycle_label` and `run_id` = the label, kind `agent_run` and source `evolve agent run`. It also holds the model, the exit code, the duration and the report hash. A failed append is exit 21 (`exitDispatch`, exit 3 before X17b), after the output is printed.
- **The launch record** (`writeLaunchRecord`) is written after every dispatch through `launchrecord.Write`. `launchSpy` wraps the bridge and keeps the driver and the model of the last launch. It also returns the Signal Center of the run, so the verdict engine emits to it. The tree state is read before and after the dispatch in the worktree, else in the project root.
  - A tree-state error is named on stderr and leaves the tree empty.
  - After a dispatch error the record has no verdict, and a writer run is marked `writes_source`.
  - A failed write is exit 21. `stageWorkspace` refuses a run label whose record exists, before the dispatch (exit 21).
- **Output.** stdout carries the report text. With `--json` it carries the `Envelope` struct, which is the `data` of the envelope of [cli-contract.md](../cli-contract.md) §2. The envelope also names the driver, the family, the launch record and the worktree. A dispatch error goes to stderr and into `Envelope.Error`. A ledger error and a launch record error go there too.
- **Signals.** SIGINT and SIGTERM cancel the dispatch context (`signal.NotifyContext`), so the ledger append and the workspace removal still run.
- **Capabilities** (`capabilities.go`, X17e, [cli-contract.md](../cli-contract.md) §4.2). The verb prints `agentclass.Surface` with no cell. With `--json` the document is the `data` of the envelope `evolve.agent.capabilities/1`. The text form has one line for each verb and ends with `default D`. A stray word or an unknown flag is exit 10.
  - `--project-root` is accepted for the contract of §5, and the table is the same for each root.
  - Pinned by `TestAgentCapabilities_JSONListsEachVerbOfTheTableWithDefaultClassD` and `TestAgentCapabilities_TextNamesEachClassAndAStrayWordIsUsage`.

## Invariants

- The run writes nothing outside its run workspace except the ledger files, the launch record and the event channels. Pinned by `TestAgentRun_AdversarialReviewDiffOnStdinPrintsTheReportAndLeavesNoCycleFootprint`, which snapshots the whole project before and after.
- Each run leaves one launch record (`TestAgentRun_EachRunWritesABridgeLaunchRecordThatNamesTheDriverFamilyModelAndTree`), and a cell signal reaches the `isolation.violation` channel (`TestAgentRun_AnIsolationSignalOfTheBridgeReachesTheIsolationChannel`).
- A refused run dispatches nothing and writes nothing (`TestAgentRun_Refusals`, `TestAgentRun_RefusedRunWritesNothing`).
- Each listed profile grants `Write` on `<workspace>/specrunner.ArtifactName(spec)`, and no `disallowed_tools` rule denies it ([agent-run.md](../agent-run.md) §6). `TestEveryAgentRunProfileGrantsWriteOnItsDeliverable` reads the shipped profiles and pins this rule. `TestDeliverableWriteProblem` proves that a blanket or a scoped deny fails the check.
  - The grant is profile data, and the launcher does not derive it. The rule has a glob for the run label, so it is the same for each run. The cycle seats also keep their `Write` grants as data, for example `plan-reviewer.json`.
  - A rename of a report or a move of the run workspace thus changes each profile. The guard test then names each profile that has no grant, so no copy can drift without a failure.
- `evolve agent list` and `evolve agent run` read one catalog loader, `phasecmd.LoadCatalog`, and one rule: `phasecmd.SpecRunnable`, and a reader or a writer that `worktreeWriter` admits. The list marks a writer with `writes_source`.
