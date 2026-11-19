---
score_cap:
  - criterion: "The note reads the guide at its link, names the item, source, snapshot and review date, and calls the guide untrusted data"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2527_001 ./acs/cycle2527"
  - criterion: "An F row explains the quoted lines as search shortcut chrome that is not a rule, and F rows cover points of the guide"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2527_002|TestC2527_003' ./acs/cycle2527"
  - criterion: "Each core persona has one verdict row with a reason, and the Decision and rejected option state their reasons"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2527_004|TestC2527_005' ./acs/cycle2527"
  - criterion: "The COMPACT.md decision and the persona verdicts match the file diff, and at most one persona changes"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2527_006|TestC2527_007' ./acs/cycle2527"
  - criterion: "The note and COMPACT.md pass strict STE lint, and the research index links the note"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2527_008|TestC2527_009' ./acs/cycle2527"
  - criterion: "No Go source, regression predicate or other family notes change"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2527_010 ./acs/cycle2527"
---

# Eval: Claude prompting guide review, item 2c5f9a4d (cycle 2527)

> Pins the deliverable of the practice-watch item practice-claude-prompting-guide-2c5f9a4d-202611190048. The guide at https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices changed after the snapshot of 2026-11-15T11:10:47Z. The stored lines hold only a search shortcut symbol and a key letter, which are page chrome. The note reads the guide as untrusted data and names each persona to change under ADR-0133, or states that none changes and why. Source incident: the cycle 2416 and cycle 2526 notes, which use the same deliverable shape.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| source-read-as-data | Link, snapshot, date and untrusted-data statement present | 6/10 | `go test -tags acs -run TestC2527_001` |
| quoted-lines-explained | Chrome lines explained as not a rule; guide points covered | 6/10 | `go test -tags acs -run 'TestC2527_002|TestC2527_003'` |
| persona-verdicts | Six verdict rows, Decision with because, rejected option | 7/10 | `go test -tags acs -run 'TestC2527_004|TestC2527_005'` |
| diff-matches-note | COMPACT.md and agents/ diffs match the note; one persona at most | 7/10 | `go test -tags acs -run 'TestC2527_006|TestC2527_007'` |
| ste-and-index | Strict STE lint clean; research index links the note | 5/10 | `go test -tags acs -run 'TestC2527_008|TestC2527_009'` |
| scope-fence | No Go source, regression or other family notes change | 5/10 | `go test -tags acs -run TestC2527_010` |
