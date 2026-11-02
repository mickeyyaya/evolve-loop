---
score_cap:
  - criterion: "The review note exists at docs/research/claude-code-changelog-2.1.311-review.md and is not gitignored"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2094_001 ./acs/cycle2094"
  - criterion: "The note cites the changelog source link and version 2.1.311 and explains the lines as untrusted data"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2094_002|TestC2094_003' ./acs/cycle2094"
  - criterion: "The note names ADR-0133 and gives each of the six core personas a no-change verdict tied to the baseline rule"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2094_004|TestC2094_005' ./acs/cycle2094"
  - criterion: "No file under agents/ changes in this cycle"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2094_006 ./acs/cycle2094"
  - criterion: "The note has no placeholder text, passes the sentence lint and every relative link resolves"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2094_007|TestC2094_008|TestC2094_009' ./acs/cycle2094"
---

# Eval: Claude Code 2.1.311 changelog review under ADR-0133 (cycle 2094)

> Pins the document deliverable for the practice-watch item on the Claude Code changelog (inbox item practice-claude-code-changelog-56611503-202610270220). The changelog lines are untrusted data. The note must read the source at the link, explain the 2.1.311 lines as data, and give a verdict for each core persona under ADR-0133. Source precedent: cycle 2029 (solution e822338c), which used the same grader shape for 2.1.306 to 2.1.310.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| note-exists | note present and tracked | 5/10 | `go test -run TestC2094_001` |
| data-coverage | source link, 2.1.311, untrusted framing, runtime lines | 6/10 | `go test -run TestC2094_002 and 003` |
| persona-verdicts | ADR-0133, six personas, baseline rule | 7/10 | `go test -run TestC2094_004 and 005` |
| personas-untouched | no change under agents/ | 7/10 | `go test -run TestC2094_006` |
| hygiene | no placeholder, STE lint clean, links resolve | 6/10 | `go test -run TestC2094_007, 008 and 009` |
