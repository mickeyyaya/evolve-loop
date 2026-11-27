# The claim ledger, the barrier report and the depth log

> The skill entry: [SKILL.md](../SKILL.md). The rules: [loop.md](loop.md). The spec of record: [docs/architecture/research-loop.md](../../../docs/architecture/research-loop.md).
>
> Copy a template into the notes file in `.evolve/evidence/<topic>/`. A seat fills its part. The lead sets the status by the status rules of [loop.md](loop.md) (code sets it once C2 exists).

## The claim ledger

The lead writes one row for each required claim before the first query. A seat adds rows only with a decision id.

| id | decision | driving | state | sources | independent | label | verified_by |
|---|---|---|---|---|---|---|---|
| C1 | DEC1 | yes | open | | 0 | | |

| Field | Values |
|---|---|
| `id` | `C<n>`. Do not reuse an id. |
| `decision` | `DEC<n>` |
| `driving` | `yes` when the claim can change the leading option. Otherwise `no`. |
| `state` | `open`, `supported`, `contradicted` or `unverifiable` |
| `sources` | Links or `path:line`, each with the span that supports the claim. |
| `independent` | The count of independent sources. |
| `label` | `measured`, `maker claim`, `search summary` or `synthesis` |
| `verified_by` | The id of the seat that checked the claim. It is not the author. |

## The barrier report of a seat

One report for each seat and each block.

```
Seat: <id> · lens: <builder|skeptic|measurer|neighbour|contrarian|verifier> · angle: <angle>
Block: <n> · levels used: <0-3> · calls used: <n>
Status: <covered|budget-capped|stalled|timed-out|errored>   (set by the lead until C2, then by code)
```

| Part | Content |
|---|---|
| Ledger delta | The rows that the seat added or changed. Use the table above. |
| Leading option | One line for each decision: the option that the seat favors. |
| Open leads | Each lead with the claim id that it can change, and the reason. |
| Exhausted | The queries and the dead ends, one for each line. |
| Conflicts | Each pair of claim ids that disagree, with the two sources. |
| Verbal confidence | A number from 1 to 5. It is a log field. |

## The depth log

The lead writes one row at each barrier. The log goes to the method line of the dossier. C2 (planned) will keep it in code.

| Block | Tier | Coverage | Stability | Verification | Progress | Band | Choice | Cited ids | Stop reason |
|---|---|---|---|---|---|---|---|---|---|
| 1 | T2 | 50% | held | 0% | n/a, 3 new | LOW | DEEPEN and WIDEN | C4, C5 | not stopped |
| 2 | T2 | 100% | held | 100% | 1.0 | HIGH | STOP | C1, C2, C4, C5, C6 | band reached |

Progress in block 1 has no baseline, so it shows the count of new supported claims. The stop reason is one of: band reached, cap reached, stalled, or operator.
