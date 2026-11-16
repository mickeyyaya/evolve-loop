---
score_cap:
  - criterion: "evolve pr merge refuses a PR with no Closes-Inbox or Refs-Inbox trailer in the enforce stage"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run '^TestPRMerge_RefusesAPRWithoutATrailerInEnforce$' ./cmd/evolve"
  - criterion: "evolve pr merge accepts declared trailers and only warns in the shadow stage"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestPRMerge_(AcceptsADeclaredTrailerInEnforce|ShadowStageWarnsAndStillMerges|RefusesAnIdThatIsNotOpenInEnforce)$' ./cmd/evolve"
  - criterion: "newPRMerger carries the trailer stage of policy into the merger"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestNewPRMerger_CarriesTheTrailerStageOfPolicy$' ./cmd/evolve"
  - criterion: "required.yml runs the trailer check on every pull request and the required job waits on it"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2439_00[678]' ./acs/cycle2439"
---

# Eval: AD2-4 CI step in required.yml and trailer check in evolve pr merge

> Pins the AD2-4 contract of docs/plans/architecture-decisions-2026-11.md: the same
> Closes-Inbox or Refs-Inbox check runs in CI for every PR and in mergeOne of
> evolve pr merge, staged by inbox.curator.trailer.stage. Source incident: cycle 2439.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| enforce-refusal | PR without a trailer is refused and nothing merges | 3/10 | `go test -run TestPRMerge_RefusesAPRWithoutATrailerInEnforce ./cmd/evolve` |
| stage-semantics | declared trailers pass; shadow warns and merges | 5/10 | `go test -run TestPRMerge_ ./cmd/evolve` |
| policy-wiring | policy stage reaches the merger | 5/10 | `go test -run TestNewPRMerger_CarriesTheTrailerStageOfPolicy ./cmd/evolve` |
| ci-step | required.yml runs the check for all PRs | 6/10 | `go test -tags acs -run TestC2439_00 ./acs/cycle2439` |
