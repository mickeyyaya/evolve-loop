---
score_cap:
  - criterion: "evolve failures list/reset/prune behave per the api contract"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1800_00[4-9]|TestC1800_01[0-1]|TestC1800_01[56]' ./acs/cycle1800"
  - criterion: "loop --reset and failures reset still acknowledge --fingerprint when state.json is unparseable, and still report a committed prune under the base prefixes when the ack fails"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1800_01[7-9]|TestC1800_02[0-2]' ./acs/cycle1800"
---

# Eval: evolve failures command group

> Pins the new `evolve failures` list/reset/prune verbs (cycles 1798 and 1800):
> read-only list with JSON output, mutating verbs requiring --project-root,
> reset sharing loop --reset's classes and fingerprint ack, prune removing only
> expired entries. Cycle 1800's round-2 audit (M1) found that extracting the
> shared reset made a prune error skip the fingerprint ack and made an ack error
> hide the committed prune and change the `[loop] --reset --fingerprint:` prefix;
> the second row pins the base behavior on both error paths through the real
> `evolve loop --reset` and `evolve failures reset` binaries.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| failures-cli | list/reset/prune contract | 4/10 | `go test -tags acs -run 'TestC1800_00[4-9]|TestC1800_01[0-1]|TestC1800_01[56]' ./acs/cycle1800` |
| reset-error-paths | ack survives a prune error; prune report and base prefix survive an ack error | 5/10 | `go test -tags acs -run 'TestC1800_01[7-9]|TestC1800_02[0-2]' ./acs/cycle1800` |
