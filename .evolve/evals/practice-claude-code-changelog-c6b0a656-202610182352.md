---
score_cap:
  - criterion: "Research doc cites the source link and the c6b0a656 snapshot, explains the 2.1.303 maintenance line as untrusted data without inventing an identifier, and keeps the older sourced items"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1959_001|TestC1959_002|TestC1959_003|TestC1959_004|TestC1959_005' ./acs/cycle1959"
  - criterion: "Each of the six core personas gets a reasoned no-change verdict that weighs 2.1.303, and no persona file changes"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1959_006|TestC1959_007' ./acs/cycle1959"
  - criterion: "The index row names 2.1.303, the conclusion ties the verdict to the ADR-0133 baseline rule, and no emphatic capital rule is added"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1959_008|TestC1959_009|TestC1959_010' ./acs/cycle1959"
  - criterion: "Doc and index pass strict ASD-STE100 lint and the practicewatch suite stays green"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1959_011|TestC1959_012' ./acs/cycle1959"
---

# Eval: Claude Code changelog practice proposal for 2.1.303 (cycle 1959)

> Pins the research proposal for the fourth practice-watch item on the Claude Code changelog (snapshot hash c6b0a656). Release 2.1.303 carries one line, "Bug fixes and reliability improvements", so the doc must explain it as untrusted data and must cite no identifier that the line lacks. The doc keeps the 2.1.299 to 2.1.302 items. ADR-0133 requires a per-persona verdict and no persona change without a measured baseline. Source incident: cycle 1959 inbox item practice-claude-code-changelog-c6b0a656-202610182352.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| data-only | source link, snapshot hash, maintenance line explained as data, older items kept | 6/10 | `go test -tags acs -run 'TestC1959_001\|TestC1959_002\|TestC1959_003\|TestC1959_004\|TestC1959_005' ./acs/cycle1959` |
| persona-verdicts | six personas weigh 2.1.303; persona files unchanged | 7/10 | `go test -tags acs -run 'TestC1959_006\|TestC1959_007' ./acs/cycle1959` |
| index-and-baseline | index row, conclusion, no emphatic rule | 6/10 | `go test -tags acs -run 'TestC1959_008\|TestC1959_009\|TestC1959_010' ./acs/cycle1959` |
| ste-lint | zero lint findings; practicewatch green | 7/10 | `go test -tags acs -run 'TestC1959_011\|TestC1959_012' ./acs/cycle1959` |
