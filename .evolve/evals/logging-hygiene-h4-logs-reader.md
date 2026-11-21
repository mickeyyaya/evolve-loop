---
score_cap:
  - criterion: "evolve logs digest gives 40 lines or fewer for one cycle, one line for each phase outcome, with a pointer to D1 or D2 for a failure"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2584_00[123]_' ./acs/cycle2584"
  - criterion: "evolve logs events, show and failures stop at their line budget and end with a truncation line that names a narrower query"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2584_00[4567]_' ./acs/cycle2584"
  - criterion: "No evolve logs verb returns a whole file: a file larger than the budget, and one huge line, stay bounded"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2584_00[89]_|TestC2584_016_' ./acs/cycle2584"
  - criterion: "phase.json accepts log_access, and in shadow mode a read outside the declared access records LOG_ACCESS_SHADOW and does not block"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2584_01[0123]_' ./acs/cycle2584"
  - criterion: "The prompt builder inlines only the D0 digest and one instruction to use evolve logs, and leaves a phase without log_access unchanged"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2584_01[45]_' ./acs/cycle2584"
---

# Eval: evolve logs reader and log_access in shadow (logging plan H4)

> Pins the reader side of docs/plans/logging-and-process-hygiene-2026-10.md sections 6.2 to 6.4:
> the five evolve logs verbs with a hard line budget (D0 40, D1 and D2 200, D3 150), the log_access
> key of phase.json in shadow mode, and the prompt block that inlines D0 and one instruction. Source
> incident: agents read whole log files and filled their context window (plan section 2, G7 to G9).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| digest-budget | One bounded line for each phase outcome, a pointer for a failure | 4/10 | `TestC2584_001` to `003` |
| truncation-line | events, show and failures end with a narrower-query line | 4/10 | `TestC2584_004` to `007` |
| never-whole-file | A file larger than the budget, and one huge line, stay bounded | 5/10 | `TestC2584_008`, `009`, `016` |
| log-access-shadow | phase.json schema, shadow signal, no block | 5/10 | `TestC2584_010` to `013` |
| prompt-d0-only | D0 and one instruction only; unchanged without log_access | 5/10 | `TestC2584_014`, `015` |
