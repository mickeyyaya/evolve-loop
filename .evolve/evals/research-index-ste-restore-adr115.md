---
score_cap:
  - criterion: "The 16 lossy research-index rows from cycle 1900 restore their dropped facts"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1924_001 ./acs/cycle1924"
  - criterion: "The ADR-0115 row says docs/ is exempt from the added-comment rule, not from 'the rule'"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1924_002 ./acs/cycle1924"
  - criterion: "docs/research-index.md passes strict STE lint with 0 findings"
    max_if_missing: 6
    evidence: "cd go && go run ./cmd/evolve docs ste-lint --strict --project-root .. docs/research-index.md"
  - criterion: "The index keeps every row that origin/main links"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1924_004 ./acs/cycle1924"
---

# Eval: Restore the facts that the cycle 1900 STE rewrite dropped from the research index

> Cycle 1900 rewrote 20 lines of `docs/research-index.md` to pass a whole-file STE lint. The rewrite
> dropped facts from 16 table rows, and the ADR-0115 row then seemed to exempt docs/ from the
> comment-removal rule. Source incident: cycle 1900 audit M1 (carryover d70195091b8b12e95ada44dc64d70df30).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| restored-facts | Each lossy row carries its dropped facts again | 7/10 | `go test -tags acs -run TestC1924_001` |
| adr0115-semantics | The exemption names the added-comment rule | 8/10 | `go test -tags acs -run TestC1924_002` |
| ste-clean | Strict STE lint reports 0 findings | 6/10 | `evolve docs ste-lint --strict` |
| no-row-loss | No index row disappears | 5/10 | `go test -tags acs -run TestC1924_004` |
