---
score_cap:
  - criterion: "evolve rebuild builds the project and prints both binary SHAs"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestRunRebuild_SuccessPrintsSHAs$' ./cmd/evolve"
  - criterion: "a live run lease exits 1 and a failing build exits 2 with the output tail"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestRunRebuild_LiveRunLeaseExits1|TestRunRebuild_BuildFailureExits2WithTail)$' ./cmd/evolve"
  - criterion: "the boot refresh, the chain boundary and the verb call one build function"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestMakeGoBuild_TheCommandHasExactlyOneCallSite$' ./cmd/evolve"
  - criterion: "the chain build failure keeps the make output tail in its error"
    max_if_missing: 6
    evidence: "cd go && go test -tags integration -count=1 -run '^TestDefaultChainRebuild_SucceedsQuietlyOrCarriesTheBuildOutput$' ./cmd/evolve"
---

# Eval: evolve rebuild console verb with one build function

> The verb shipped in cycle 1963. Cycle 2087 closes acceptance 3: the chain boundary
> adapter `defaultChainRebuild` was a second copy of the `make -C go build` call that
> `defaultBootRefreshRebuild` already holds for the boot refresh and the verb.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| verb-prints-shas | both SHAs printed | 4/10 | `go test -run TestRunRebuild_SuccessPrintsSHAs ./cmd/evolve` |
| exit-codes | lease exits 1, build failure exits 2 with tail | 5/10 | `go test -run TestRunRebuild_ ./cmd/evolve` |
| one-build-home | one non-test `make -C go build` call site | 6/10 | `go test -run TestMakeGoBuild_TheCommandHasExactlyOneCallSite ./cmd/evolve` |
| chain-tail-kept | chain error keeps the output tail | 6/10 | `go test -tags integration -run TestDefaultChainRebuild_ ./cmd/evolve` |
