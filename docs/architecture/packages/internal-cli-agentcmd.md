# internal/cli/agentcmd

> The design of record is [agent-run](../agent-run.md), decided in [ADR-0152](../adr/0152-one-phase-agent-runs-from-the-command-line-with-no-cycle-through-the-phase-runner.md). This page keeps the package detail.

## Purpose

`internal/cli/agentcmd` holds `evolve agent run <profile>` and `evolve agent list`. A person or a program runs one read-only phase agent with an input on stdin and gets the report on stdout, with no cycle.

## Design

- **`New(Deps)`** returns the standard subcommand function. `Deps` has four seams: `InstallRouter` (cmd/evolve passes `rootRouterInstaller`), `Bridge` (default `phasecmd.DefaultBridge`), `Getenv` (default `os.Getenv`) and `Now` (default `time.Now`). Tests inject a fake bridge, so no test launches a real CLI.
- **One dispatch path.** The run builds its runner with `phasecmd.NewSpecRunner`, the constructor that `evolve phase` uses. The routing table, the fallback walk, the sandbox and the verdict all come from the shared phase runner.
- **Admission** (`admit`) runs before the input is read. A built-in phase is refused (exit 2), because each one reads or writes cycle state. A spec that `phasecmd.SpecRunnable` rejects is unknown (exit 10). A spec that `core.PhaseWritesSource` marks is refused (exit 2). A dispatch depth above 0 (`subagent.ReadDispatchDepth`) is refused first, so a phase agent cannot start a nested run.
- **Input** (`composeInput`). `--input FILE` wins over stdin, and `-` means stdin. Each `--file PATH` appends `### <path>` and a fenced block. The fence is longer than each backtick run in the file (`fence`). Each source stops at 8 MiB (`readLimited`). Empty or oversized input is exit 11.
- **A terminal on stdin** is not read. The check is for a character device, so `/dev/null` counts as a terminal. With `--file` the files are the input, else it is a usage error.
- **The run workspace** is `.evolve/ephemeral/agent-runs/<label>`. `os.Mkdir` creates the label directory, so a label collision fails (exit 3) and never shares the workspace of another run. The workspace is removed at the end unless `--keep`. A failed removal is named on stderr, and the exit code stays the code of the run.
- **The request** has cycle 0, no worktree, `RunID` = the label, and the input under `specrunner.AgentInputKey`. With no worktree the request has no fence, and the bridge grants no Git writes.
- **The report** is `<workspace>/specrunner.ArtifactName(spec)`, the name that the runner judges.
- **The ledger entry** is appended after every dispatch, also after a dispatch error. It has cycle 0, `cycle_label` and `run_id` = the label, kind `agent_run` and source `evolve agent run`. It also holds the model, the exit code, the duration and the report hash. A failed append is exit 3, after the output is printed.
- **Output.** stdout carries the report text, or one `Envelope` with `--json`. A dispatch error goes to stderr and into `Envelope.Error`. A ledger error goes there too.
- **Signals.** SIGINT and SIGTERM cancel the dispatch context (`signal.NotifyContext`), so the ledger append and the workspace removal still run.

## Invariants

- The run writes nothing outside its run workspace except the ledger files. Pinned by `TestAgentRun_AdversarialReviewDiffOnStdinPrintsTheReportAndLeavesNoCycleFootprint`, which snapshots the whole project before and after.
- A refused run dispatches nothing and writes nothing (`TestAgentRun_Refusals`, `TestAgentRun_RefusedRunWritesNothing`).
- `evolve agent list` and `evolve agent run` read one catalog loader, `phasecmd.LoadCatalog`, and one rule: `phasecmd.SpecRunnable` and not `core.PhaseWritesSource`.
