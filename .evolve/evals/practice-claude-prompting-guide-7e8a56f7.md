---
score_cap:
  - criterion: "Section 9 of the Claude prompting guide note reads the source at its link and treats the excerpt as untrusted data"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2416_001|TestC2416_002|TestC2416_003' ./acs/cycle2416"
  - criterion: "Section 9 states under ADR-0133 that no persona changes, with a reason, and no persona or model-notes file changes"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2416_004|TestC2416_005' ./acs/cycle2416"
  - criterion: "The note keeps its earlier sections and passes the STE lint with zero findings"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2416_006|TestC2416_007' ./acs/cycle2416"
---

# Eval: Document practice-watch item 7e8a56f7 under ADR-0133

> Pins the fourth report of the Claude prompting guide in section 9 of
> `docs/research/practice-claude-prompting-guide-2026-11.md`. The stored
> excerpt holds an icon glyph and the word Console, which are page chrome and
> not a prompting rule. Source incident: cycle 2416 inbox item 7e8a56f7.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| source-read-as-data | Link, snapshot and untrusted-data statement present | 6/10 | `go test -tags acs -run TestC2416_001` |
| no-persona-change | ADR-0133 verdict with reason, no agents/ diff | 7/10 | `go test -tags acs -run TestC2416_004` |
| ste-clean | Whole note passes the STE lint | 6/10 | `go test -tags acs -run TestC2416_007` |
