---
score_cap:
  - criterion: "A guard fails when a cmd/evolve test file walks the module outside the shared loader"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestModuleScanGuard$' ./cmd/evolve"
  - criterion: "The shared loader file exists, is git-tracked and holds the one module walk behind sync.Once"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2398_002' ./acs/cycle2398"
  - criterion: "The inbox-writers and ledger-roots tests run under 5 s and no longer walk the module themselves"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2398_003' ./acs/cycle2398"
  - criterion: "The cmd-evolve package note names the loader and the guard and each line about them passes STE"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2398_004' ./acs/cycle2398"
---

# Eval: cmd/evolve tests read the Go module once

> Pins the shared module loader and its guard in go/cmd/evolve. Before this work, 14 walk sites in 10 test
> files each walked and parsed the whole module, so under CI contention TestInboxWriters_EveryAutofiledItemCarriesAKnownClass
> and TestUnobservedLedgerRootsArePinned took 7.5 s to 92.7 s (docs/research/ci-wall-variance-2026-11.md, R6).
> Source incident: cycle 2398.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| guard | A direct module walk fails the guard | 6/10 | `go test -run '^TestModuleScanGuard$' ./cmd/evolve` |
| one-home | One walk in the loader, memoized | 6/10 | `go test -tags acs -run TestC2398_002 ./acs/cycle2398` |
| speed | The two named tests run under 5 s | 5/10 | `go test -tags acs -run TestC2398_003 ./acs/cycle2398` |
| docs | The package note passes STE | 5/10 | `go test -tags acs -run TestC2398_004 ./acs/cycle2398` |
