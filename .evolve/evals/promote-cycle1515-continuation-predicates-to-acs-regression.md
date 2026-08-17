---
score_cap:
  - criterion: "The eight cycle-1515 continuation predicates are present and GREEN on the durable path go/acs/regression/cycle1515 (not merely present)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -v ./acs/regression/cycle1515 | grep -c -- '--- PASS: TestC1515_' | grep -qx 8"
  - criterion: "The promoted package falls inside the exact pattern the acs-durable CI job sweeps (./acs/regression/...)"
    max_if_missing: 9
    evidence: "cd go && go list -tags acs ./acs/regression/... | grep -qx 'github.com/mickeyyaya/evolve-loop/go/acs/regression/cycle1515'"
  - criterion: "The promoted predicates still drive the production seams they pin (internal/continuation, internal/inboxmover) rather than being renamed stubs"
    max_if_missing: 7
    evidence: "cd go && go list -tags acs -f '{{join .TestImports \"\\n\"}}' ./acs/regression/cycle1515 | grep -q 'go/internal/inboxmover'"
  - criterion: "The negative predicate (unknown scope id must not read as a successful release) survives the promotion and passes"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -v -run '^TestC1515_007_ContinuationReleaseRejectsUnknownScope$' ./acs/regression/cycle1515 | grep -q -- '--- PASS: TestC1515_007'"
  - criterion: "The acs-durable CI job keeps running the recursive regression sweep that makes the promotion durable"
    max_if_missing: 6
    evidence: "grep -q 'go test -count=1 -tags acs ./acs/regression/\\.\\.\\.' .github/workflows/ci.yml"
---

# Eval: promote the cycle-1515 continuation predicates into the durable ACS gate

> `registry-release-on-park-consume` and `continuation-operator-cli` shipped (cycle-1507
> and the cycle-1515 landing) and are protected by eight predicates in
> `go/acs/cycle1515/predicates_test.go`. That file lives in a PER-CYCLE ACS package,
> and `.github/workflows/ci.yml`'s `acs-durable` job runs only
> `go test -count=1 -tags acs ./acs/regression/...` — so the shipped continuation CLI
> and the park/consume registry-release binding had ZERO standing CI protection: a
> refactor of `inboxmover.ReleaseContinuationBinding`, `continuation.ListRegistryEntries`
> or the `registry.go` command table could silently break either behaviour with CI
> green. This eval pins the promotion permanently: the predicates must live where the
> durable gate sweeps, must actually pass there, and must keep driving the real
> production seams instead of degrading into same-named stubs. Source incidents:
> cycles 1487/1497 (the `park-consume-releases-continuation-binding` burns) and the
> standing `warnship_apicover_ci_gap` / ADR-0069 class of "per-cycle ACS ≠ standing CI".

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| promoted-green | All 8 `TestC1515_*` predicates PASS on the durable path | 8/10 | `go test -tags acs ./acs/regression/cycle1515` |
| swept-by-gate | Package appears in the `./acs/regression/...` expansion CI runs | 9/10 | `go list -tags acs ./acs/regression/...` |
| seams-intact | Test imports still reach inboxmover/continuation (anti-stub) | 7/10 | `go list -f TestImports` |
| negative-survives | Unknown-scope release still fails loudly | 7/10 | `-run TestC1515_007` |
| gate-still-recursive | CI keeps the recursive regression sweep | 6/10 | `grep ci.yml` |
