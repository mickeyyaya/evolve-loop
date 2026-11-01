# internal/agentclass

> Decision: [ADR-0156](../adr/0156-the-cli-is-the-service-contract-and-the-pipeline-reads-no-ambient-control-flags.md) rules 17 to 22. Spec of record: [cli-contract.md](../cli-contract.md) §4.1 to §4.5. Plan: [llm-isolation-2026-10](../../plans/llm-isolation-2026-10.md) component X17e, item `cli-agent-tool-surface`.

## Purpose

`internal/agentclass` holds the agent class of each `evolve` verb. The operator said on 2026-10-26: "make sure that LLM cli is able to justify how to interact with the evo loop pipeline as it can decide how to leverage evo loop cli to help it to complete its tasks."

An agent in a cell reads the classes from `evolve agent capabilities --json`. The class gate of `evolve` reads the same table, so the list that the agent sees is the rule that the gate applies.

| Class | In a cell |
|---|---|
| `A` | The verb runs. It needs no credential. |
| `R` | The verb refuses with exit 3. The agent writes a request of the kind in the table. |
| `D` | The verb refuses with exit 3. No request can ask for it. |

## Design

- **The table** (`table.go`). One `Verb` row for each verb path: `Path`, `Class`, `Kind` (only for `R`), `OnlyWith` and `DeniedWith`. `Verbs` returns a copy.
- **The match** (`Of`). It reads two words first. Then it reads the slot `<name>` of `phase <name>`, if the second word is not a flag. Then it reads one word.
  - A verb that the table does not name is `D`, with the first word as its name.
  - An `A` row with `OnlyWith` is `D` when the flag is absent or false. An `A` row with `DeniedWith` is `D` when the flag is set.
  - The last instance of the flag wins. A value that is not a boolean counts as set for `DeniedWith` and as not set for `OnlyWith`.
  - For `OnlyWith`, an instance right after another flag with no `=` does not count, because it can be the value of that flag.
  - A flag after `--` does not count.
- **The request names** (`requestname.go`). `RequestsDir`, `KindDispatch`, `RequestFile`, `ChildRequest` and `ValidRequestName` are the one grammar of a request file and of a request name. `RequestPath`, the request channel and the check of a judgement read it ([llm-isolation.md](../llm-isolation.md) §7.2.1).
- **The refusal text** (`Match.Refusal`). An `R` verb names `RequestPath` of its kind. A `D` verb names no path.
- **The surface** (`Surface`). It gives the document of [cli-contract.md](../cli-contract.md) §4.2. An `A` row has `input` `args/1` and the `output` that the caller gives, else `text/1`. An `R` row has its request. A `D` row has only the verb and the class.
- **The brief** (`Brief`). It is the short section that the bridge adds to the prompt of a phase cell. It points to the verb and names the request directory. It names no tool syntax of a family.

## Invariants

- The package imports the standard library only. It reads no file and no environment.
- `TestAgentClasses_MatchTheContract` (`go/cmd/evolve`) compares the table with cli-contract.md §4.1 and §5. It fails on a row that differs, on a verb of the contract with no row, and on a core verb with no row.
- Each `R` row has a request kind, and no other row has one (`TestVerbs_IsACopyOfTheTableWithOneRowForEachPath`).

## Limits

- The match reads words, not the flags of the verb. A flag before the subverb, for example `evolve inbox --project-root P list`, gives the one-word name, which is `D`. The error is to the safe side.
- The table has no record of the cell, so `Surface` gets the cell from its caller. The verb passes none.
