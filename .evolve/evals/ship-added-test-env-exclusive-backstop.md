---
score_cap:
  - criterion: "A require-tmux/env-exclusive newly added test does not block the ship (it is honestly unrunnable here)"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestRepoContractGate_AddedEnvExclusiveTestBackstopRecorded$' ./internal/phases/ship"
  - criterion: "The scan log records an explicit, durable exclusion for the candidate — never a silent drop or a false green claim"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestRepoContractGate_AddedEnvExclusiveTestBackstopRecorded$' ./internal/phases/ship"
---

# Eval: Honest backstop for env-exclusive added tests

> Pins T1 AC3 of the red-first-deliverable-reds-main lane's
> `ship-added-red-test-guard` task (live inbox record). Cycle-1555 already
> closed AC1/AC2 (a genuinely added red test blocks ship; selection stays
> bounded to newly added `_test.go` files) and T2's AC1/AC2 (production-path
> wiring proof; an honest `t.Skip` reproducer stays non-blocking) — those
> tests are pre-existing in `go/internal/phases/ship/repocontract_test.go`
> and remain RED because the green implementation has not landed yet.
>
> This eval covers the one acceptance criterion that had no test until
> cycle-1559: a candidate added test file that carries the
> `requires_tmux`-style build-constraint convention (scout's "env-exclusive"
> shape) cannot be executed on a quiet host. Left unhandled, this is a
> dishonest signal in EITHER direction — a lone-file package excluded by its
> own `//go:build` constraint reports "build constraints exclude all Go
> files", which a naive classifier reads as a genuine compile break (false
> RED blocking an honest ship), while silently skipping the package with no
> record at all launders the gap as "nothing to see here" (false green). The
> fix must instead leave a durable, explicit backstop entry in the ship-time
> scan log (`ship-repocontract-scan.log`) naming the excluded file and its
> exclusion reason, and must not fail the ship over it.
>
> Source incident: live inbox `red-first-deliverable-reds-main`, scout's
> explicit callout of "honest handling for require-tmux-style tests" as part
> of the acceptance contract.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| non-blocking | An env-exclusive added test never fails the ship | 7/10 | `go test -run '^TestRepoContractGate_AddedEnvExclusiveTestBackstopRecorded$'` |
| explicit-backstop | Scan log names the file + exclusion reason, no silent drop | 8/10 | `go test -run '^TestRepoContractGate_AddedEnvExclusiveTestBackstopRecorded$'` |
