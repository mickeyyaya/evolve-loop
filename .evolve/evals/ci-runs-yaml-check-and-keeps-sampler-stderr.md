---
score_cap:
  - criterion: "required.yml ste-lint job runs evolve yaml check once, on the binary the job already builds, after that build"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2647_001_' ./acs/cycle2647"
  - criterion: "the yaml check step passes on the real tree and fails on a workflow without on/jobs"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2647_00[23]_' ./acs/cycle2647"
  - criterion: "floorCIParity has a yaml row for required.yml ste-lint and the parity test passes it"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2647_00[45]_' ./acs/cycle2647"
  - criterion: "every go.yml job that starts hostsampler keeps its stderr in a file that an always() upload-artifact step uploads"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2647_006_' ./acs/cycle2647"
  - criterion: "no macOS spawn shim enters the change"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2647_007_' ./acs/cycle2647"
---

# Eval: required CI runs the yaml check and keeps the host sampler stderr

> Pins the CI wiring of inbox item ci-runs-yaml-check-and-keeps-sampler-stderr in cycle 2647. The yaml floor tier had no CI step, so a malformed workflow could reach main. The host sampler wrote its stderr to /dev/null, so a sampler failure left no evidence. The macOS spawn shim (C13 part B) is out of scope.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| ste-lint-yaml-step | one yaml check step after the existing evolve build | 6/10 | `go test -tags acs -run TestC2647_001_` |
| yaml-step-gates | the step exits 0 on the tree and non-zero on a broken workflow | 7/10 | `go test -tags acs -run TestC2647_00[23]_` |
| parity-row | floorCIParity names the yaml row and the parity test passes | 6/10 | `go test -tags acs -run TestC2647_00[45]_` |
| sampler-stderr | stderr lands in a file the always() upload carries | 7/10 | `go test -tags acs -run TestC2647_006_` |
| no-shim | the change adds no spawn shim | 5/10 | `go test -tags acs -run TestC2647_007_` |
