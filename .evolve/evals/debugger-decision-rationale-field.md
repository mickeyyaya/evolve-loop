---
score_cap:
  - criterion: "Parser reads rationale and the legacy reasoning field into one signal"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run TestDecisionRationale ./internal/phases/debugger"
  - criterion: "The persona output contract example is parsed into the rationale signal"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2666_002 ./acs/cycle2666"
  - criterion: "The build phase failed-approach entry names root_cause and not reasoning"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2666_003 ./acs/cycle2666"
---

# Eval: Debugger decision field reasoning renamed rationale

> Pins the rename of the debugger decision field `reasoning` to `rationale`, with one release of legacy read support, and the matching `root_cause` name in the build phase failed-approach entry. Source: inbox item persona-diet-c1-debugger-reasoning-field, cycle 2666. A persona-only edit breaks the parser contract, so the persona example is fed through `Classify`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| parser-both-fields | rationale, legacy reasoning, both, neither | 5/10 | `go test -run TestDecisionRationale ./internal/phases/debugger` |
| persona-parser-agree | persona example yields the rationale signal | 6/10 | `go test -tags acs -run TestC2666_002 ./acs/cycle2666` |
| build-text-root-cause | failed-approach entry has root_cause | 6/10 | `go test -tags acs -run TestC2666_003 ./acs/cycle2666` |
