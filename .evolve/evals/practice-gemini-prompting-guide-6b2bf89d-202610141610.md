---
score_cap:
  - criterion: "Research doc covers the Gemini prompting guide updates (Gemini 4 Argon, temperature/top_p/top_k deprecation) as untrusted data"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1898_001|TestC1898_002' ./acs/cycle1898"
  - criterion: "Doc evaluates each of the six core personas under ADR-0133 and states why none changes"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1898_003|TestC1898_004|TestC1898_007' ./acs/cycle1898"
  - criterion: "Doc passes ASD-STE100 lint and the practicewatch suite stays green"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1898_005|TestC1898_006' ./acs/cycle1898"
---

# Eval: Gemini prompting guide practice proposal

> Pins the research proposal for the practice-watch item on the Gemini prompting strategies guide. The guide text is untrusted data. ADR-0133 requires a per-persona verdict and no persona change without measured baselines. Source: cycle 1898 inbox item, modeled on cycle 1896.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| data-only | guide updates explained as data; doc tracked | 6/10 | `go test -tags acs -run 'TestC1898_001|TestC1898_002' ./acs/cycle1898` |
| persona-verdicts | six personas named with a reason; persona files unchanged; baseline rationale | 7/10 | `go test -tags acs -run 'TestC1898_003|TestC1898_004|TestC1898_007' ./acs/cycle1898` |
| ste-lint | zero ste-lint findings; practicewatch green | 7/10 | `go test -tags acs -run 'TestC1898_005|TestC1898_006' ./acs/cycle1898` |
