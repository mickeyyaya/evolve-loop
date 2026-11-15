# internal/launchrecord

> The design of record is [agent-run.md](../agent-run.md) §7, for component X11 of [llm-isolation.md](../llm-isolation.md) §13 (ADR-0154). The writer is [internal-cli-agentcmd](internal-cli-agentcmd.md), and the reader is [internal-commitgate](internal-commitgate.md).

## Purpose

`internal/launchrecord` holds the bridge launch record of one `evolve agent run`. The record names the profile, the driver, the family, the model, the verdict and the tree that the run saw. The commit gate reads it to attest a review run.

## Design

| Symbol | Meaning |
|---|---|
| `Subdir` | `agent-runs`, the directory of the records under the evolve directory |
| `Record` | one record. Its JSON keys are the fields of the table in [agent-run.md](../agent-run.md) §7. |
| `Record.BoundTo(tree)` | true only when the tree is not empty and both tree states equal it |
| `Path(evolveDir, runID)` | the file of a run. It refuses a run id that is not one name of letters, digits, `.`, `_` and `-`, with a letter or a digit first. |
| `Write(evolveDir, r)` | creates the record file (`O_EXCL`) and returns its path. It refuses to replace a record (`fs.ErrExist`). |
| `Read(evolveDir, runID)` | reads the record. It refuses a file that is not JSON and a file that holds another run id. |
| `Record.Cloud` | the `cloudlaunch.Session` of a run on a cloud seat, else nil (JSON key `cloud`) |
| `List(evolveDir)` | each record of `Subdir`, in the order of `ts`. No directory gives no record. A bad record fails the list, and the error names the run. |

## Invariants

- A run id never names a path outside `Subdir` (`TestPath_RefusesARunIDThatIsNotOneName`).
- A record that saw a change of the tree binds to no tree (`TestBoundTo_HoldsOnlyWhenTheTreeDidNotChangeAndMatches`).
- A renamed record does not attest another run (`TestRead_ARecordOfAnotherRunIDFails`).
- A record is never replaced (`TestWrite_RefusesToReplaceARecord`).
- A record can hold `Violations`, the contract violations of a failed verification. A failing deliverable is kept next to the record by `KeepDeliverable`, and the call keeps only the newest files (`TestKeepDeliverable_WritesAtomicallyAndKeepsTheNewestAndNeverTouchesRecords`).
- No cell can write a record: the review profiles write only their run workspace, and the build seat denies `.evolve/agent-runs`. The guard is `confinedToWorktree` in `internal/cli/agentcmd`.
