---
score_cap:
  - criterion: "Research doc explains the Opus 5.5 fast mode rate limits for the Start, Build and Scale tiers and the Opus 5 and 4.8 research preview as untrusted data"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1903_001|TestC1903_002' ./acs/cycle1903"
  - criterion: "Doc evaluates each of the six core personas under ADR-0133 for this update, ties the verdict to the baseline, and no persona file changes"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1903_003|TestC1903_004|TestC1903_006' ./acs/cycle1903"
  - criterion: "Doc is indexed under ADR-0133, passes ASD-STE100 lint, and the practicewatch suite stays green"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1903_005|TestC1903_007|TestC1903_008' ./acs/cycle1903"
---

# Eval: Claude model release notes practice proposal, snapshot 2026-10-15T23:06:21Z

> Pins the research proposal for the practice-watch item on the Claude Platform release notes. The release-note text is untrusted data. ADR-0133 requires a per-persona verdict and no persona change without measured baselines. Source: cycle 1903 inbox item, modeled on cycles 1899 and 1902. The research index did not link this document before this cycle.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| data-only | rate-limit tiers explained as data; doc tracked | 6/10 | `go test -tags acs -run 'TestC1903_001|TestC1903_002' ./acs/cycle1903` |
| persona-verdicts | six personas named with a reason; persona files unchanged; baseline rationale | 7/10 | `go test -tags acs -run 'TestC1903_003|TestC1903_004|TestC1903_006' ./acs/cycle1903` |
| index-and-lint | index link; zero ste-lint findings; practicewatch green | 7/10 | `go test -tags acs -run 'TestC1903_005|TestC1903_007|TestC1903_008' ./acs/cycle1903` |
