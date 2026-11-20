---
score_cap:
  - criterion: "The darwin set passes one fixed go test -p on a 3-core and a 5-core host"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2563_001|TestC2563_002' ./acs/cycle2563"
  - criterion: "Only the darwin set is fixed; the full-set target keeps the host default"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2563_003|TestC2563_004' ./acs/cycle2563"
  - criterion: "The research doc records the span ratio or names why not, passes STE, and marks R3 done"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2563_005|TestC2563_006' ./acs/cycle2563"
---

# Eval: the darwin test set runs with a fixed go test -p

> Pins item ci-macos-fixed-parallelism (R3 of docs/research/ci-wall-variance-2026-11.md).
> Source incident: cycle 2360 (re-pinned by cycle 2563 after cycle 2548 failed on harness reds). go test sets -p to the CPU count, so a 5-core macOS host and a
> 3-core host run different schedules for the same packages.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| same-p-both-hosts | Every go test of the darwin recipe has the same -p on 3 and 5 cores | 4/10 | `go test -tags acs -run 'TestC2563_001|TestC2563_002' ./acs/cycle2563` |
| scope-fence | test-integration keeps no -p; the darwin recipe keeps its flags | 6/10 | `go test -tags acs -run 'TestC2563_003|TestC2563_004' ./acs/cycle2563` |
| doc-outcome | Doc names the span ratio or why not, STE clean, R3 done | 6/10 | `go test -tags acs -run 'TestC2563_005|TestC2563_006' ./acs/cycle2563` |
