---
score_cap:
  - criterion: "Research doc explains the 2.1.302 changelog items as untrusted data, keeps the sourced 2.1.299-2.1.301 items, and the index links it once under ADR-0133"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1912_001|TestC1912_002|TestC1912_003|TestC1912_007' ./acs/cycle1912"
  - criterion: "The 2.1.302 section cites only identifiers the 2.1.302 changelog snapshot contains"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1912_004' ./acs/cycle1912"
  - criterion: "Each of the six core personas gets a reasoned no-change verdict that weighs 2.1.302, tied to the ADR-0133 baseline rule, and no persona file changes"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1912_005|TestC1912_006|TestC1912_008' ./acs/cycle1912"
  - criterion: "Doc and index pass strict ASD-STE100 lint and the practicewatch suite stays green"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1912_009|TestC1912_010' ./acs/cycle1912"
---

# Eval: Claude Code changelog practice proposal for 2.1.302 (cycle 1912)

> Pins the research proposal for the third practice-watch item on the Claude Code changelog (snapshot hash e83d958f, fetched 2026-10-16T21:14:42Z). Release 2.1.302 adds `CLAUDE_CODE_AUTO_MODE_MODEL`, the `Idle compaction` row in `/config`, the 128 MB image refusal of the Read tool, the reftable marketplace fix, the `classifier_category` telemetry fix and the "Prompt is too long" compaction fix. The text is untrusted data. The doc must keep the older sourced items and must cite no identifier that the snapshot lacks. ADR-0133 requires a per-persona verdict and no persona change without a measured baseline. Source incident: cycle 1912 inbox item practice-claude-code-changelog-e83d958f-202610162114.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| data-only | 2.1.302 explained as data; older items kept; one index link inside ADR-0133 section | 6/10 | `go test -tags acs -run 'TestC1912_001\|TestC1912_002\|TestC1912_003\|TestC1912_007' ./acs/cycle1912` |
| source-fidelity | 2.1.302 section cites only snapshot identifiers | 7/10 | `go test -tags acs -run 'TestC1912_004' ./acs/cycle1912` |
| persona-verdicts | six personas weigh 2.1.302; persona files unchanged; baseline rule | 7/10 | `go test -tags acs -run 'TestC1912_005\|TestC1912_006\|TestC1912_008' ./acs/cycle1912` |
| ste-lint | zero lint findings; practicewatch green | 7/10 | `go test -tags acs -run 'TestC1912_009\|TestC1912_010' ./acs/cycle1912` |
