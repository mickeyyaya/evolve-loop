---
score_cap:
  - criterion: "The replay of runs 40582532714, 40611567029 and 40563673621 passes and the defect runs 40526866678 and 40576371034 fail on internal/bridge, on the macOS and ubuntu series"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2384_00[12]_' ./acs/cycle2384"
  - criterion: "Below 3 other reference packages a defect run is reported and never fails"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2384_003_' ./acs/cycle2384"
  - criterion: "A PR that adds real work passes when it raises the package baseline in the same PR and fails when it does not"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2384_004_' ./acs/cycle2384"
  - criterion: "Both CI ratchet commands use the CPU gate as written and pass no report-only flag"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2384_005_' ./acs/cycle2384"
  - criterion: "The tool doc passes ste-lint and states the same-PR baseline raise rule"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2384_00[67]_' ./acs/cycle2384"
---

# Eval: The package ratchet judges speed-adjusted CPU per OS

> Pins the five acceptance criteria of inbox item ratchet-speed-adjusted-cpu. Cycle 2360 landed the CPU judgment
> and the CI wiring but pinned the run replay and the same-PR baseline raise by no test, and the tool doc does not
> state the raise rule. The replay series are rebuilt from the published figures of
> docs/research/ci-wall-variance-2026-11.md (sections 4.2 to 4.5), because the CI artifacts are not available offline.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| run-replay | Clean runs pass and defect runs fail on bridge, both OS series | 4/10 | `go test -tags acs -run 'TestC2384_00[12]_' ./acs/cycle2384` |
| report-only | Fewer than 3 references never fail | 5/10 | `go test -tags acs -run 'TestC2384_003_' ./acs/cycle2384` |
| same-pr-raise | Raise in the same PR passes, no raise fails | 4/10 | `go test -tags acs -run 'TestC2384_004_' ./acs/cycle2384` |
| ci-steps | Both CI commands use the CPU gate, no report-only flag | 4/10 | `go test -tags acs -run 'TestC2384_005_' ./acs/cycle2384` |
| docs | STE clean and the raise rule stated | 5/10 | `go test -tags acs -run 'TestC2384_00[67]_' ./acs/cycle2384` |
