# internal/cli/cloudcmd

> The spec of record is [llm-isolation.md](../llm-isolation.md) §5.2. The verb is a core verb of [cli-contract.md](../cli-contract.md) §5.

## Purpose

`internal/cli/cloudcmd` holds `evolve cloud list`. The verb prints each cloud session that a cloud seat launched. It reads the launch records and starts nothing.

## Design

- `Run(args, stdout, stderr)` dispatches the subcommand. `list` is the only one. `help`, `-h` and `--help` print the usage.
- `list` reads `launchrecord.List` under the evolve directory of the project root. It keeps each record that has a `cloud` block, in time order.
- The text form has one line for each session: the id, the URL, the title and the time, separated by tabs.
- `--json` prints one JSON array of `cloudlaunch.Session`. The dispatcher puts it in the `data` of the envelope `evolve.cloud.list/1`. No record gives `[]`.
- The project root is `--project-root`, else `EVOLVE_PROJECT_ROOT`, else the current directory, as for `evolve agent`.
- Exit 0 listed, 1 a launch record that does not read, 10 usage.

There is no launch verb in this package. A cloud seat starts only through the bridge, when a profile or a `cli_routing` rule names `claude-cloud`. Thus the I-1 rule holds: only the bridge starts an LLM.

## Invariants

- The list is in time order, and it shows only the records with a cloud session (`TestRun_ListPrintsEachCloudSessionInTimeOrder`).
- A bad launch record fails the list with exit 1, and the error names the run (`TestRun_ABadRecordFailsTheList`).
- The agent class of `cloud list` is A (`internal/agentclass/table.go`), because it only reads.
