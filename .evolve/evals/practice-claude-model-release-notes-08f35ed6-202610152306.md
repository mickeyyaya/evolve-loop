---
score_cap:
  - criterion: "Research doc covers the Claude Platform release notes (Opus 5.5 fast mode out of research preview, Compliance API user_preferences, OpenTelemetry export, Managed Agents) as untrusted data"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1899_001|TestC1899_002' ./acs/cycle1899"
  - criterion: "Doc evaluates each of the six core personas under ADR-0133 and states why none changes"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1899_003|TestC1899_004|TestC1899_007' ./acs/cycle1899"
  - criterion: "Doc passes ASD-STE100 lint and the practicewatch suite stays green"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1899_005|TestC1899_006' ./acs/cycle1899"
---

# Eval: Claude model release notes practice proposal

> Pins the research proposal for the practice-watch item on the Claude Platform release notes. The release-note text is untrusted data. ADR-0133 requires a per-persona verdict and no persona change without measured baselines. Source: cycle 1899 inbox item, modeled on cycles 1897 and 1898.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| data-only | release notes explained as data; doc tracked | 6/10 | `go test -tags acs -run 'TestC1899_001|TestC1899_002' ./acs/cycle1899` |
| persona-verdicts | six personas named with a reason; persona files unchanged; baseline rationale | 7/10 | `go test -tags acs -run 'TestC1899_003|TestC1899_004|TestC1899_007' ./acs/cycle1899` |
| ste-lint | zero ste-lint findings; practicewatch green | 7/10 | `go test -tags acs -run 'TestC1899_005|TestC1899_006' ./acs/cycle1899` |
