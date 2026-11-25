---
score_cap:
  - criterion: "The guard reports an os.MkdirTemp pattern that has no registered temp shape"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2631_001|TestC2631_002|TestC2631_003' ./acs/cycle2631"
  - criterion: "A temp dir past the horizon with a live socket stays"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2631_004|TestC2631_005' ./acs/cycle2631"
  - criterion: "An evcodexhome dir past the horizon with no holder is reaped, a held or fresh one stays"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2631_006|TestC2631_007|TestC2631_008' ./acs/cycle2631"
---

# Eval: A temp shape registry replaces pipelineTempRe

> Pins the acceptance of recycle-x7-temp-shape-registry (plan PR #1585, check K8, component X7). On 2026-11-20 the temp dir held 131,402 entries older than 24 hours (evcodexhome, evghconfig, evcodeusage, buildfloor-cover) that pipelineTempRe matched none of. One registry in gc replaces the regexp, and a guard fails on an os.MkdirTemp pattern with no registered shape.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| guard | an unregistered MkdirTemp pattern is reported | 7/10 | `go test -tags acs -run 'TestC2631_00[123]' ./acs/cycle2631` |
| live-socket | a dir with a listening socket stays | 7/10 | `go test -tags acs -run 'TestC2631_00[45]' ./acs/cycle2631` |
| evcodexhome | an evcodexhome dir past the horizon with no holder goes | 7/10 | `go test -tags acs -run 'TestC2631_00[678]' ./acs/cycle2631` |
