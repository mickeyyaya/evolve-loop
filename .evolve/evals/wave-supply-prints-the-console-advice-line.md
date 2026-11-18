---
score_cap:
  - criterion: "one admissible item in a window of 8 prints console: feed"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2519_001' ./acs/cycle2519"
  - criterion: "8 admissible items and 11 open PRs print console: finish"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2519_002' ./acs/cycle2519"
  - criterion: "8 admissible items and 3 open PRs print console: build"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2519_003' ./acs/cycle2519"
  - criterion: "the PR limit is one batch of 10 and the starvation line is below the window"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2519_004' ./acs/cycle2519"
  - criterion: "the JSON supply carries the advice"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2519_005' ./acs/cycle2519"
---

# Eval: evolve wave supply prints the console advice line

> Pins plan K3 of console-build-or-route-2026-11: `evolve wave supply` prints one
> advice line, `console: build`, `console: feed` (D4) or `console: finish` (D6),
> in text and in JSON. The predicates run the built binary with a stub `gh` on
> PATH, so the production caller path (verb, census, PR port, printer) is reached.
> Source: inbox item wave-supply-prints-the-console-advice-line, cycle 2519.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| starved-supply | 1 of window 8 prints feed | 6/10 | `TestC2519_001` |
| open-pr-limit | 11 open PRs print finish | 6/10 | `TestC2519_002` |
| healthy | 3 open PRs print build | 6/10 | `TestC2519_003` |
| boundaries | 10 PRs build, 7 of 8 feed | 5/10 | `TestC2519_004` |
| json | data.advice in the JSON | 5/10 | `TestC2519_005` |
