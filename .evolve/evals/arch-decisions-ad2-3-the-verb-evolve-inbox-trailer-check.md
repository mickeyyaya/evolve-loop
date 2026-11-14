---
score_cap:
  - criterion: "evolve inbox trailer-check refuses a range with no inbox trailer and an id that is not open, and accepts Closes-Inbox none"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^(TestCmd_InboxTrailerCheck_RefusesARangeWithNoTrailer|TestCmd_InboxTrailerCheck_RefusesAnIdThatIsNotOpen|TestCmd_InboxTrailerCheck_AcceptsNone)$' ./cmd/evolve"
  - criterion: "the stage key inbox.curator.trailer.stage defaults to shadow, accepts shadow and enforce, and refuses any other word"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestInboxTrailerStage' ./internal/policy"
  - criterion: "the built evolve binary reaches the verb from the inbox dispatcher in both stages"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2382_002' ./acs/cycle2382"
  - criterion: "every function of the verb file has 100 percent statement coverage and the changed files carry no comment"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2382_004|TestC2382_005|TestC2382_006' ./acs/cycle2382"
  - criterion: "the CLI contract row, the policy spec and the package notes state the verb and the key and pass ste-lint"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2382_007' ./acs/cycle2382"
---

# Eval: The inbox trailer-check verb

> Pins component AD2-3 of the 2026-11-03 architecture decisions plan. The verb `evolve inbox trailer-check --range`
> reads the commit messages of a range and checks the Closes-Inbox, Refs-Inbox and `Closes-Inbox: none` trailers.
> The key `inbox.curator.trailer.stage` sets shadow (report, exit 0) or enforce (refuse, exit 1). Source: cycle 2382,
> plan docs/plans/architecture-decisions-2026-11.md.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| verb-behaviour | three named refusals and acceptances | 4/10 | `go test -run TestCmd_InboxTrailerCheck_* ./cmd/evolve` |
| stage-key | default shadow, strict values | 5/10 | `go test -run TestInboxTrailerStage ./internal/policy` |
| wiring | the binary reaches the verb | 5/10 | `TestC2382_002` |
| floor | 100% coverage, zero comments | 6/10 | `TestC2382_004`, `005`, `006` |
| docs | contract row, spec, package notes | 5/10 | `TestC2382_007` |
