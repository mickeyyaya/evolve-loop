# The research loop: operating rules for the lead and the seats

> The skill entry: [SKILL.md](../SKILL.md). The spec of record: [docs/architecture/research-loop.md](../../../docs/architecture/research-loop.md). The templates: [ledger.md](ledger.md). The decision: [ADR-0171](../../../docs/architecture/adr/0171-research-is-an-llm-steered-loop-inside-a-code-envelope-gated-in-depth-blocks-by-evidence-coverage.md).
>
> The lead is the console main session. A seat is a researcher that the console starts through `evolve agent run`. No phase agent starts a seat.
>
> **Shadow rule (D14).** The `evolve research` verbs (C2) are planned and do not exist. Until they exist, the lead does the work of code by the written rules. The lead sets each seat status (see the status rules below) and counts the independent sources. The lead also computes the band and forces STOP-INCOMPLETE at a cap. The lead records each result in the depth log.

## The rules of the lead

1. **Claims first (D1).** Split the question into the decisions that it must support. Write the claims that each decision requires. Mark each claim that can change the leading option as driving. Fill the ledger before the first query.
2. **Two axes (D2).** Angles split the topic. Lenses split the stance. Read 3–5 overview sources first and find the perspectives of the subject. Add each perspective that the floor does not name.
3. **The lens floor (D2).** Each request has these lenses:
   - builder: primary sources, such as code, specs and official docs;
   - skeptic: failures, issues and postmortems;
   - measurer: papers and benchmarks.
   A neighbour lens (an adjacent field) is optional.
4. **The contrarian (D2).** After block 1, start one seat. It reads the ledger and makes the strongest case against the leading option.
5. **Independence (D3).** Give each seat its brief only. Do not show a seat the notes of another seat before a barrier. Run no debate rounds. When the console has a second model family, give it the skeptic or the contrarian seat.
6. **The tier (D4).** Set the tier before the first query. A tier only rises. The numbers are design choices, not measured values.

| Tier | Use | Seats | Blocks | Calls for each seat in a block |
|---|---|---|---|---|
| T0 | A two-way door, one small decision | 1–2 | 1 | 10 or fewer |
| T1 | A plan or a reversible design | 3 | 2 or fewer | 15 or fewer |
| T2 | An ADR, a protected surface or a one-way door | 4–5 | 4 or fewer | 15 or fewer, a hard stop at 20 |

7. **The block (D5).** A block is up to three levels of follow-up for each seat. One level is one hop from a source that the seat read. The block size is configuration. Tell each seat its remaining calls after each call.
8. **The barrier (D7).** Collect one report from each seat. Use the template in [ledger.md](ledger.md). Merge the ledger deltas. Count the independent sources.
9. **The band (D8).** Compute the band from the ledger by the rules below (code does this once C2 exists).
10. **The choice (D9).** Choose one verb and cite the ledger ids. Re-read the evidence that the seats gathered before you order more search.
11. **The report (D10).** Give the main session the fixed short form at each barrier.
12. **Shadow (D14).** Record the band, the choice and the stop reason in the depth log and the dossier.

## The seat status

Until C2 exists, the lead sets the status of each seat. The inputs are the calls used, the clock, the exit of the seat and the count of new claims. A seat never sets its own status.

| Status | Condition |
|---|---|
| `covered` | The seat ended inside its caps, with no error, and each claim of its brief is `supported`, `contradicted` or `unverifiable`, each with sources. |
| `budget-capped` | The calls used reached the cap of the tier while a claim of the brief is still `open`. |
| `stalled` | The last level of the seat added no new supported or contradicted claim, and a claim of the brief is still `open`. |
| `timed-out` | The clock cap ended the seat before it finished. |
| `errored` | A tool error or a run error ended the seat. |

`errored`, `stalled`, `budget-capped` and `timed-out` never count as `covered`. When two conditions hold, take the first of this order: `errored`, `timed-out`, `budget-capped`, `stalled`.

## The band

The thresholds are first values. Tune them in shadow. Three values set the band: coverage, stability and verification. Progress does not set the band. It is a guard on the verb: a stalled run cannot take DEEPEN.

| Input | Definition |
|---|---|
| Coverage | The share of required claims in the state `supported`. |
| Covered | A driving claim with state `supported` and two independent sources, and one is not the vendor. |
| Stability | The leading option of each decision is the same as at the last barrier. In block 1, the option at level 3 equals the option at level 2. |
| Verification | The share of driving claims that a seat which did not write them checked. |
| Progress | New supported claims for each call. From block 2, divide it by the same value of the last block. Block 1 has no baseline: show the count of new supported claims, and call it stalled only at zero. Below 0.5 is falling. Zero new claims in the block is stalled. |

| Band | Rule |
|---|---|
| HIGH | Coverage 90% or more. Every driving claim is covered. No driving claim is contradicted. Stability held. Verification 100%. |
| MEDIUM | Coverage 70% or more. No driving claim is contradicted. Stability held, or verification 50% or more. |
| LOW | Any other state. |

## The verbs

| Verb | Use it when |
|---|---|
| STOP | The band is at or above the band of the tier. |
| DEEPEN | A lead names a claim that it can change. Name the leads in the order. |
| WIDEN | A required claim has no lead, a driving claim is contradicted, or the run stalled. Name the new angle or lens. |
| STOP-INCOMPLETE | A cap is reached. You must choose it at a cap (code forces it once C2 exists). You can also choose it before a cap. Write each open claim as a gap. |

The lowest band for STOP: T0 MEDIUM. T1 MEDIUM, with every driving claim covered. T2 HIGH.

You can always continue inside the caps. You cannot STOP below the band of the tier. A stalled run cannot take DEEPEN. Name a falling progress in your reason. A T2 run that ends in STOP-INCOMPLETE goes to the operator.

## The report to the main session

```
Block <n> of <max> · tier <T> · band <HIGH|MEDIUM|LOW>
Inputs: coverage <x%> · stability <held|not held> · verification <x%> · progress <ratio>
Decisions: DEC1 leads with <option>; DEC2 leads with <option>
Choice: <STOP|DEEPEN|WIDEN|STOP-INCOMPLETE> because <reason with ledger ids>
Budget left: <blocks> blocks, <calls> calls for each seat
```

## The rules of a seat

1. Read your brief. It names your lens, your angle and your claim ids.
2. Work alone. Do not read the notes of another seat.
3. Follow a lead one level deeper only when you can name the claim that the lead can change. Drop a lead that every option predicts in the same way. It has no value.
4. Read the primary source. Write the span that supports the claim. Label the claim: measured, maker claim, search summary or synthesis.
5. Check your remaining calls after each call. Stop at your cap.
6. Write a ledger delta. Do not set the state of a claim that you wrote to `supported` without a source span.
7. Write each open lead with the claim id that it can change. Write each exhausted query.
8. Write your verbal confidence. It is a log field. It does not decide anything.
9. Do not write your own status. The lead sets it by the status rules (code sets it once C2 exists).
10. When you are a verifier, check only if the source supports the claim. Do not search for new claims.

## Integration with the other documents

1. The dossier ends with one Decision, Rationale and Alternatives entry for each decision. Each entry cites finding ids.
2. Give the ADR writer the dossier and the 3–5 most recent related ADRs.
3. Each ADR option and driver cites a finding id. The ADR names the check that confirms the decision.
4. Each plan component cites decision ids.
5. A writer marks a missing fact `NEEDS-EVIDENCE: <claim>`. Run one block for each marker, at the tier of its decision.
6. Handle at most five markers for each document in a pass. Rank them by impact times uncertainty.
7. Send a marker that is still open after two runs to the operator.
8. A document with an open marker does not ship. Ask the `docs-review` seat to look for the marker.
