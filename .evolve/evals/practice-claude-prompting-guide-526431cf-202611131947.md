---
score_cap:
  - criterion: "The note section for item 526431cf reads the source as untrusted data and explains each quoted guide line in an H row"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2372_00[123]' ./acs/cycle2372"
  - criterion: "The proposal gives each core persona a verdict, a true marker count and a reason under ADR-0133, one persona at a time"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2372_00[456]' ./acs/cycle2372"
  - criterion: "The note keeps earlier rows, passes the STE lint and stays linked from the research index"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2372_00[789]' ./acs/cycle2372"
---

# Eval: practice-claude-prompting-guide-526431cf-202611131947

> Pins the third report of the claude prompting guide change. The stored excerpt of the item has no added line, so the note must record that fact and must not guess a change. Source incident: the same empty-excerpt shape as item 8d950f1e in the same note.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| source-as-data | Source link, snapshot, untrusted-data statement, H rows with short quotes | 6/10 | `go test -run TestC2372_00[123]` |
| adr-0133-verdicts | Six persona verdict rows with true marker counts, one change at a time, agents unchanged | 7/10 | `go test -run TestC2372_00[456]` |
| hygiene | Earlier rows kept, STE lint 0, research-index link | 5/10 | `go test -run TestC2372_00[789]` |
