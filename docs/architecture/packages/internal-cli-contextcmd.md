# internal/cli/contextcmd

> The design of record is [cell-context-index](../cell-context-index.md) §7 and §8. Plan: [cell-context-index-2026-10](../../plans/cell-context-index-2026-10.md) components CX2 and CX3. The index format is in [internal-inputindex](internal-inputindex.md).

## Purpose

`internal/cli/contextcmd` holds `evolve context`. An agent of any family reads the cell index `inputs/index.json` with the same verbs, so no persona names the file tool of one CLI.

## Design

- **`New(Deps)`** returns the standard subcommand function. `Deps.Env` reads the pipeline markers. cmd/evolve passes `os.Getenv`.
- **`index [--layer L1|L2] [--required] [--json] [--workspace W]`** prints the rows of the index. `--layer` keeps one layer, and `--required` keeps the required rows. The data of `--json` is the index with the field `listed`, the count of the rows before the filter.
- **`open <id> [--json] [--workspace W]`** prints the body of one row through `inputindex.Index.Open`. A flag can come before or after the id.
- **`stats [--phase P] [--json] [--project-root R]`** prints a `inputindex.KindStat` for each phase and kind from the access log. Its agent class is `D`, because it reads the log of other cycles.
- **The cell.** `--workspace W` names the cell. `W` must be a run directory `<root>/.evolve/runs/<dir>`, and the root is three levels up. With no flag, `EVOLVE_PROJECT_ROOT` and the cycle of `EVOLVE_DISPATCH_ID` name the cell. The prompt names the workspace in its `workspace` line, so an agent can always pass the flag.
- **Envelope.** With `--json`, each verb prints one envelope (`schema`, `ok`, `data`, `error`, `warnings`) on each exit, also on a usage error. The schemas are `SchemaIndex`, `SchemaOpen` and `SchemaStats`.

| Exit | Kind | When |
|---|---|---|
| 0 | ok | the rows, the body or the stats printed |
| 1 | failed | the body differs from the `sha256` of the index |
| 2 | io | no index, a missing body or an unreadable access log |
| 10 | usage | no verb, an unknown verb or flag, a bad layer, no id for `open`, or no cell |
| 11 | input | the index does not decode, an unknown id, or a path outside the project root |

## Invariants

- The verbs write nothing. The access record of an open comes from the sandbox report of the read (cell-context-index.md §8), so a cell cannot forge it with the verb.
- `open` reads only a listed path inside the root, and only the body that the index hashed (`TestContextOpen_RefusesAnUnknownIDAForeignPathAndAChangedBody`).
- Each exit prints one envelope with `--json` (`TestContext_UsageAndCellErrorsPrintAnEnvelopeOnEachExit`).
