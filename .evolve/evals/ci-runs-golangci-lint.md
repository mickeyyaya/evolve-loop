---
score_cap:
  - criterion: "The required CI workflow has one golangci-lint job that fails on a swallowed error and that the CI required gate waits on"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2324_00[123]' ./acs/cycle2324"
  - criterion: "CI reads the golangci-lint version from the one pin file and the gate's binary reports that version"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2324_00[45]' ./acs/cycle2324"
  - criterion: "Main passes the workflow lint command, so the new job is green on merge"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2324_006 ./acs/cycle2324"
---

# Eval: CI runs golangci-lint

> Pins the CI lint job of cycle 2324. Cycles landed errcheck and ineffassign issues on main because CI ran
> go vet, gofmt and tests but not golangci-lint, and the local commit gate then refused a console merge
> that touched the same packages. The job, its config, its version and the five issues that main still holds
> in `cmd/vulnfilter` are pinned here.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| ci-blocks-lint-debt | the required gate fails when the lint job fails, and the job's command rejects an unchecked error | 4/10 | `go test -tags acs -run TestC2324_00[123]` |
| one-version | the pin file is the only home of the version, and the host binary matches it | 6/10 | `go test -tags acs -run TestC2324_00[45]` |
| main-clean | the workflow command exits 0 on main | 5/10 | `go test -tags acs -run TestC2324_006` |
