---
score_cap:
  - criterion: "evolve rebuild builds the project to go/bin/evolve and prints the old and new binary SHAs"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1963_001|TestC1963_002' ./acs/cycle1963"
  - criterion: "a live run lease exits 1 without building; a stale lease does not block"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1963_005|TestC1963_006' ./acs/cycle1963"
  - criterion: "a failing build exits 2 and prints the tail of the build output"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1963_007 ./acs/cycle1963"
  - criterion: "the boot refresh and the rebuild verb reach one make build function"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1963_008 ./acs/cycle1963"
---

# Eval: evolve rebuild verb (cli-rebuild-lane)

> Pins the `evolve rebuild [--project-root P]` verb added in cycle 1963. The verb runs the
> same `make -C go build` as the boot refresh, prints the old and new binary SHAs, refuses
> on a live run lease (exit 1) and exits 2 with the output tail on a build failure.
> Source: inbox item cli-rebuild-lane (split of cli-rebuild).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| verb-builds | rebuild writes go/bin/evolve and prints both SHAs | 4/10 | `go test -tags acs -run 'TestC1963_001|TestC1963_002' ./acs/cycle1963` |
| lease-guard | live lease exits 1, stale lease passes | 5/10 | `go test -tags acs -run 'TestC1963_005|TestC1963_006' ./acs/cycle1963` |
| fail-tail | failing build exits 2 with tail | 5/10 | `go test -tags acs -run TestC1963_007 ./acs/cycle1963` |
| one-builder | boot refresh and verb share one build function | 6/10 | `go test -tags acs -run TestC1963_008 ./acs/cycle1963` |
