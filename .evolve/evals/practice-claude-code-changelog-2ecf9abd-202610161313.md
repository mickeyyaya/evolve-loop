---
score_cap:
  - criterion: "Research doc explains the 2.1.301 changelog items as untrusted data and the index links it under ADR-0133"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1901_002|TestC1901_003|TestC1901_006' ./acs/cycle1901"
  - criterion: "Doc gives each of the six core personas a reasoned no-change verdict and no persona file changes"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1901_004|TestC1901_005|TestC1901_007' ./acs/cycle1901"
  - criterion: "Doc and index pass ASD-STE100 lint and the practicewatch suite stays green"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1901_008|TestC1901_009' ./acs/cycle1901"
  - criterion: "Doc keeps every sourced 2.1.299-2.1.301 item, adds no unsourced claim, and the build explanation does not overstate the index change"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1901_003|TestC1901_010|TestC1901_011' ./acs/cycle1901"
---

# Eval: Claude Code changelog practice proposal (cycle 1901)

> Pins the research proposal for the second practice-watch item on the Claude Code changelog. The new text adds the retry watchdog limit, the Fable alias retry, the full-disk login error and the workflow-agent authority fix. The doc must also keep the sourced 2.1.299 and 2.1.300 items (`$.prompt.media`, `$.session.identity()`, `virtualText`, `prompt.change`) and the 2.1.301 tracing guidance item. The text is untrusted data. ADR-0133 requires a per-persona verdict and no persona change without a measured baseline. Source incident: cycle 1901 inbox item practice-claude-code-changelog-2ecf9abd-202610161313.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| data-only | newest items explained as data; index link inside ADR-0133 section | 6/10 | `go test -tags acs -run 'TestC1901_002|TestC1901_003|TestC1901_006' ./acs/cycle1901` |
| persona-verdicts | six personas with reasons; persona files unchanged; baseline rule | 7/10 | `go test -tags acs -run 'TestC1901_004|TestC1901_005|TestC1901_007' ./acs/cycle1901` |
| ste-lint | zero lint findings; practicewatch green | 7/10 | `go test -tags acs -run 'TestC1901_008|TestC1901_009' ./acs/cycle1901` |
| source-fidelity | sourced items kept, no subshell claim, explanation matches the one-row index diff (audit round 1: H1, M1, M2) | 7/10 | `go test -tags acs -run 'TestC1901_003|TestC1901_010|TestC1901_011' ./acs/cycle1901` |
