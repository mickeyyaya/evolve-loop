---
score_cap:
  - criterion: "the routed-source guard fails an unregistered emit site and a registered test name that does not exist"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2639_00[1-3]_' ./acs/cycle2639"
  - criterion: "a seal that carries routed items reaches ClassConflict through Seal.Outcome, and the real guard passes on the tree"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2639_004_' ./acs/cycle2639"
  - criterion: "a mutant that drops the routed items from Seal.Outcome fails the seal scorer test by assertion"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2639_005_' ./acs/cycle2639"
---

# Eval: The routed-source guard registers each stage-evidence emit site with a scorer test

> Pins the registry contract of the stage-evidence routed-source guard. The guard used to demand that Outcome.Routed stay empty, so the first change that fills it could never pass CI. It now maps each emit site (file and hit kind) to a named scorer test that reaches ClassConflict, and it fails for an unregistered site and for a registered name that is not a test function. Source incident: PR 1735 (shadow-contract K2 wiring) held out of the 2026-11-24 batch; cycle 2639 carries the inbox item.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| registry-fails-closed | Unregistered site and missing test name both fail | 7/10 | `go test -tags acs -run 'TestC2639_00[1-3]_' ./acs/cycle2639` |
| seal-reaches-conflict | Seal.Outcome carries routed items into the scorer | 7/10 | `go test -tags acs -run 'TestC2639_004_' ./acs/cycle2639` |
| mutant-killed | Dropping routed items from Seal.Outcome fails the scorer test | 8/10 | `go test -tags acs -run 'TestC2639_005_' ./acs/cycle2639` |
