---
score_cap:
  - criterion: "Every §7 component row that a consumed inbox record cites (design doc §7.N <Id>) reads shipped/merged and cites the cycle whose PASS dossier lists that record: W8 → cycle 1724, G2 → cycle 1726, G3 → cycle 1729"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1746_001_ConsumedComponentRowsReadShippedWithTheirCycle ./acs/cycle1746/..."
  - criterion: "No §7 row whose inbox record is still pending (A1, A2, G1, Q5, W4, W5, W6, W7 at the base) claims shipped or merged — a pass that stamps every row shipped fails"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1746_002_PendingComponentRowsDoNotClaimShipped ./acs/cycle1746/..."
  - criterion: "G2's corrected status keeps the fact the row already records: an allowance is a ceiling (§5.13)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1746_003_G2KeepsItsCeilingNote ./acs/cycle1746/..."
  - criterion: "§7.1 P3 matches its merge commit abc9ced0 (ADR-0106 P3, Q1, Q2 … (#661)): the status cites #661 and says nothing is pending"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1746_004_P3RowMatchesItsMergeCommit ./acs/cycle1746/..."
  - criterion: "No duplicated bullet remains anywhere in the document: no two bullet lines are identical and no bullet list repeats a bold lead label (the header's two **Status:** bullets and §12's two walled-dispatch bullets at the base)"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1746_005_NoBulletIsDuplicated ./acs/cycle1746/..."
  - criterion: "Deduplication keeps exactly one copy: one **Status:** bullet naming the living document in the header, one **One rule for a walled dispatch** bullet naming isQuotaWall in §12"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1746_006_DeduplicatedBulletsKeepOneCopy ./acs/cycle1746/..."
  - criterion: "Against base cc16fb89, every heading, every §7 row (with its component and files cells) and every header/§12 bullet label is still present — only status cells and duplicate bullets change"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1746_007_StatusPassKeepsTheRestOfTheDocument ./acs/cycle1746/..."
---

# Eval: Design-doc status pass — every component row matches its consumed record or merge commit

> Pins the inbox item `design-doc-status-pass` (the 2026-09-28 inbox architecture review, N6) for
> `docs/architecture/logic-first-delivery-design.md`. §7's legend defines **shipped** as a commit on a
> branch with its PR open or merged, and **built** as green in a worktree but not shipped. At base
> `cc16fb89`, three rows contradicted their consumed inbox records: W8 (`designed`, shipped by cycle 1724),
> G2 (`built`, shipped by cycle 1726 as `5cbc3e56` on main) and G3 (`designed`, shipped by cycle 1729).
> P3 still read "PR pending" although #661 had merged (`abc9ced0`). The document also carried two
> duplicated bullets: the header's two `**Status:**` lines (15:14 and a stale 14:14) and §12's
> walled-dispatch bullet, repeated verbatim. Source incident: cycle 1746. The predicates compute each
> row's expected status from the inbox records, the PASS dossiers and git history. No expected string is
> hand-written into them.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| consumed-rows-shipped | a consumed record's row reads shipped and cites its PASS cycle | 9/10 | `go test -tags acs -run TestC1746_001 ./acs/cycle1746` |
| pending-rows-negative | a pending record's row never claims shipped | 8/10 | `go test -tags acs -run TestC1746_002 ./acs/cycle1746` |
| g2-fact-kept | G2 keeps its §5.13 ceiling note | 6/10 | `go test -tags acs -run TestC1746_003 ./acs/cycle1746` |
| p3-merge-commit | P3 cites #661 and drops "pending" | 7/10 | `go test -tags acs -run TestC1746_004 ./acs/cycle1746` |
| no-duplicate-bullet | no identical bullet and no repeated bold label in a list | 9/10 | `go test -tags acs -run TestC1746_005 ./acs/cycle1746` |
| one-copy-kept | each deduplicated bullet survives exactly once | 8/10 | `go test -tags acs -run TestC1746_006 ./acs/cycle1746` |
| rest-preserved | headings, §7 rows and bullet labels of the base survive | 7/10 | `go test -tags acs -run TestC1746_007 ./acs/cycle1746` |
