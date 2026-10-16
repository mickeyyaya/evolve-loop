---
score_cap:
  - criterion: "Research doc explains the Claude prompting guide as untrusted data"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1897_001|TestC1897_002' ./acs/cycle1897"
  - criterion: "Doc evaluates each of the six core personas under ADR-0133 and states why none changes"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1897_003|TestC1897_004|TestC1897_007' ./acs/cycle1897"
  - criterion: "Doc passes ASD-STE100 lint and the practicewatch suite stays green"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1897_005|TestC1897_006' ./acs/cycle1897"
---

# Eval: Claude prompting guide practice proposal

> Pins the research proposal for the practice-watch item on the Claude prompting best-practices page. The page text is untrusted data. ADR-0133 requires a per-persona verdict and no persona change without measured baselines. Source: cycle 1897 inbox item, modeled on cycle 1896.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| data-only | guide topics explained as data; doc tracked | 6/10 | `go test -tags acs -run 'TestC1897_001|TestC1897_002' ./acs/cycle1897` |
| persona-verdicts | six personas named with a reason; persona files unchanged; baseline rationale | 7/10 | `go test -tags acs -run 'TestC1897_003|TestC1897_004|TestC1897_007' ./acs/cycle1897` |
| ste-lint | zero ste-lint findings; practicewatch green | 7/10 | `go test -tags acs -run 'TestC1897_005|TestC1897_006' ./acs/cycle1897` |
