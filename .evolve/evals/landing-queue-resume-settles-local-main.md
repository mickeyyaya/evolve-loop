---
score_cap:
  - criterion: "The landing queue resume fast-forwards local main to origin before it clears the flag, when origin holds the intent commit"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1852_001 ./acs/cycle1852"
  - criterion: "The resume leaves local main unchanged and parks the record when origin lacks the intent commit"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1852_002 ./acs/cycle1852"
  - criterion: "Plan row Q17 names TestLandingQueueCLI_ResumeSettlesLocalMainToOrigin and keeps its seven existing tests"
    max_if_missing: 6
    evidence: "grep -q 'TestLandingQueueCLI_ResumeSettlesLocalMainToOrigin' docs/plans/concurrent-cycle-landing-2026-10.md"
  - criterion: "The resume section keeps the ship push invariant that makes the local fast-forward safe"
    max_if_missing: 5
    evidence: "grep -q 'Ship moves `main` only after its push lands' docs/architecture/fleet-landing-queue.md"
---

# Eval: The landing queue resume settles local main

> Pins the resume step of the landing queue spec (ADR-0128, `docs/architecture/fleet-landing-queue.md`).
> For a stranded head whose intent commit `origin` holds, the resume made the record `landed` and
> cleared the flag, but nothing moved local `main`. Each other candidate then failed its fast-forward
> check against a stale `main` and composed again. Source: PR #827 round-3 review, cycle 1852.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| settle | local main fast-forwards to origin before the flag clears | 8/10 | `TestC1852_001` |
| no-move | origin lacks the commit: main stays, record parks | 7/10 | `TestC1852_002` |
| plan-test | Q17 names the settle test | 6/10 | grep in the plan |
| invariant | ship push invariant kept | 5/10 | grep in the spec |
