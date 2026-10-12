# Patterns that show the reasoning

> A design document earns trust when a reader can follow each step from the evidence to the decision. The reader can also see where a step can be wrong. These patterns make the steps visible. Sources: [docs/research/docs-write-skill-2026-10.md](../../../docs/research/docs-write-skill-2026-10.md) §2.

## 1. Finding with its implication

```markdown
**F3.4** `TestDefaultExecAdapter_DispatchesViaBridge` took 479 s. A stack dump at 40 s shows `tokenusage.ScanConfigRoot` → `readLines` (`go/internal/tokenusage/scanner.go:82`).
*Implication:* the test reads host state, so its time grows with the use of the host.
```

The fact and the meaning are separate. A reader can accept the fact and still question the meaning.

## 2. Decision table

```markdown
| Id | Decision | Because | Rejected |
|---|---|---|---|
| D7 | Skip a transcript file whose time is before the window start. | F3.6: only lines inside the window count. | A size cap: it drops data that counts. |
```

## 3. Scored comparison of candidate designs

```markdown
| Design | Effect | Time saved | Cost | Risk | Score |
|---|---|---|---|---|---|
| A. Raise limits | 5 | 1 | 5 | 2 | 13 |
| B. Measure once | 5 | 5 | 3 | 4 | 17 |

Each criterion has a score from 1 (bad) to 5 (good). B wins because <the decisive difference>.
```

Name the criteria before you score. Say which criterion decided the result.

## 4. Worked example

Show one real case from end to end, with real values: the input, each step, and the output. A reader who follows one case can predict the next one.

## 5. Consequences, both ways

```markdown
## Consequences
- Good: <what gets better, with the measure>.
- Cost: <what gets worse or more expensive>.
- Risk: <what can go wrong, and the control>.
```

## 6. What changes our mind

```markdown
We revisit D3 if <a measurable condition>, for example if the profile percent differs from `go test -cover` on any package.
```

This line turns a decision into a testable claim.

## 7. Limits

A list of what the design does not do and what its tests cannot prove. It stops a reader from over-trusting the design.

## 8. Before and after

```markdown
| Measure | Before (<date>) | After |
|---|---|---|
| Local floor time | 23.4 min | <measured value> |
```

Give the date and the method of each number. Mark an estimate as an estimate.
