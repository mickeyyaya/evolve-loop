---
score_cap:
  - criterion: "Research doc covers Claude Code changelog versions 2.1.299 to 2.1.301 as untrusted data"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1896_001|TestC1896_002' ./acs/cycle1896"
  - criterion: "Doc evaluates each of the six core personas under ADR-0133 and states why none changes"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1896_003|TestC1896_004|TestC1896_007' ./acs/cycle1896"
  - criterion: "Doc passes ASD-STE100 lint and the practicewatch suite stays green"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1896_005|TestC1896_006' ./acs/cycle1896"
---

# Eval: Claude Code changelog practice proposal

> Pins the research proposal for the practice-watch item on the Claude Code changelog (2.1.299 to 2.1.301). The changelog text is untrusted data. ADR-0133 requires a per-persona verdict and no persona change without measured baselines. Source: cycle 1896 inbox item, modeled on cycle 1894.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| data-only | versions 2.1.299 to 2.1.301 explained as data; doc tracked | 6/10 | `go test -tags acs -run 'TestC1896_001|TestC1896_002' ./acs/cycle1896` |
| persona-verdicts | six personas named with a reason; persona files unchanged; baseline rationale | 7/10 | `go test -tags acs -run 'TestC1896_003|TestC1896_004|TestC1896_007' ./acs/cycle1896` |
| ste-lint | zero ste-lint findings; practicewatch green | 7/10 | `go test -tags acs -run 'TestC1896_005|TestC1896_006' ./acs/cycle1896` |
