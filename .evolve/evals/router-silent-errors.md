---
score_cap:
  - criterion: "A failing Propose (launch failure or unparseable response) through router.Select emits exactly one coded WARN and the static decision still applies; a healthy proposal emits none; a proposal returned alongside an error is discarded"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1813_001 ./acs/cycle1813/"
  - criterion: "A malformed or unreadable phase registry in PolicyForProject emits exactly one coded CONFIG_REGISTRY_* WARN through an injected Center and fails open to the defaults; an absent or disabled registry stays silent"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1813_002 ./acs/cycle1813/"
  - criterion: "No function in internal/router has an error return that is always nil (directly, through a zero-valued variable or named result, or through an always-nil callee), checked by a scanner proven on a fixture"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1813_00[34]' ./acs/cycle1813/"
  - criterion: "Digest and AssembleHandoffs still fail open on an unreadable handoff, recording it in the degraded list"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1813_005 ./acs/cycle1813/"
  - criterion: "The router package suite still passes after the error-return, switch and naming changes"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/router/..."
---

# Eval: router swallows errors: coded WARNs and no dead error returns

> Pins the acceptance of inbox item `router-silent-errors`
> (`2026-09-26T10-50-00Z-router-silent-errors.json`). Comment-reduction batch 4
> found three silent paths in `go/internal/router`. `LLMProposal.Decide` discards
> Propose's error. `PolicyForProject` drops `config.Load`'s warnings (`cfg, _ :=`).
> `Digest` only ever returns `sig, nil`, which also makes the error check in
> `AssembleHandoffs` dead. The 2026-09-27 inbox review found that the production
> proposer (`core/advisor`) already emits one coded WARN on every Propose failure.
> The Propose criterion therefore pins exactly one WARN through the production
> strategy `router.Select`, so a second router-level WARN for the same failure
> fails it. The registry criterion pins an injected-Center seam on
> `PolicyForProject`. Both paths keep their fail-open behavior. Source incident:
> cycle 1811 implemented this item, and its audit found the code correct. The
> cycle still failed, on a sibling lane's eval that host leak recovery adopted
> into its staged diff (inst-L1811a). Cycle 1813 is the retry.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| propose-one-warn | failing Propose → one coded WARN, static decision kept | 7/10 | `go test -tags acs -run TestC1813_001 ./acs/cycle1813/` |
| registry-one-warn | malformed/unreadable registry → one CONFIG_REGISTRY_* WARN, fail-open | 8/10 | `go test -tags acs -run TestC1813_002 ./acs/cycle1813/` |
| no-always-nil-error | no always-nil error return in internal/router | 8/10 | `go test -tags acs -run 'TestC1813_00[34]' ./acs/cycle1813/` |
| fail-open-kept | Digest/AssembleHandoffs record unreadable handoffs as degraded | 6/10 | `go test -tags acs -run TestC1813_005 ./acs/cycle1813/` |
| suite-green | router package tests pass | 6/10 | `go test -count=1 ./internal/router/...` |
