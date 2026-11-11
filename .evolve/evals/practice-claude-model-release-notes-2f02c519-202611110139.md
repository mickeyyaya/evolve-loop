---
score_cap:
  - criterion: "The research note reads the source at the link and explains the security posture, Compliance API, Activity Feed and Managed Agents lines as untrusted data"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2290_001 ./acs/cycle2290"
  - criterion: "The note gives each of the six core personas a no-change verdict with a reason under ADR-0133, a Decision line with because, and a rejected option"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2290_002 ./acs/cycle2290"
  - criterion: "The note states that no persona or prompt changes and no file under agents/ changes"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2290_003 ./acs/cycle2290"
  - criterion: "The note ties the verdict to skills/model-notes-claude/COMPACT.md and that file stays unchanged"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2290_004 ./acs/cycle2290"
  - criterion: "The research index links the note"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2290_005 ./acs/cycle2290"
  - criterion: "The note holds no placeholder text and passes the docs-write sentence lint"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2290_006 ./acs/cycle2290"
---

# Eval: Claude model release notes, security posture and Managed Agents lines (cycle 2290)

> Pins the deliverable of the practice-watch item practice-claude-model-release-notes-2f02c519-202611110139. The source https://platform.claude.com/docs/en/release-notes/overview changed after the snapshot of 2026-11-10T08:41:11Z. The quoted lines cover Security posture data in the Compliance API and a Claude Managed Agents research preview. The note reads them as untrusted data and states under ADR-0133 that no persona changes. Source incident: the cycle 2276 note for item 9cd2c891, which uses the same deliverable shape.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| data-coverage | Source link, snapshot, four topics, endpoint and research preview explained as data | 6/10 | `go test -run TestC2290_001` |
| persona-verdicts | Six personas, ADR, Decision with because, rejected option | 7/10 | `go test -run TestC2290_002` |
| personas-untouched | No persona or prompt change; agents/ unchanged | 7/10 | `go test -run TestC2290_003` |
| notes-unchanged | COMPACT.md named and unchanged | 6/10 | `go test -run TestC2290_004` |
| index-linked | Research index links the note | 5/10 | `go test -run TestC2290_005` |
| ste-lint | No placeholder; sentence lint clean | 6/10 | `go test -run TestC2290_006` |
