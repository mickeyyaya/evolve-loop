---
score_cap:
  - criterion: "The tool-using MaxNoProgress test fails when its events file is missing"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2556_001 ./acs/cycle2556"
  - criterion: "No phaseobserver test ignores a json.Unmarshal error"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2556_00[23]' ./acs/cycle2556"
  - criterion: "The phaseobserver package tests stay green"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 ./internal/phaseobserver/"
---

# Eval: phaseobserver test hygiene

> Pins the two acceptance criteria of inbox item phaseobserver-test-hygiene (found by comment-reduction batch 31). A test must not pass when its input file is missing, and no test may drop a json.Unmarshal error. Cycle 2556.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| mutation | Missing events file fails the test | 6/10 | `go test -tags acs -run TestC2556_001 ./acs/cycle2556` |
| ast-lint | No ignored Unmarshal error | 6/10 | `go test -tags acs -run 'TestC2556_00[23]' ./acs/cycle2556` |
| regression | Package tests stay green | 5/10 | `go test -count=1 ./internal/phaseobserver/` |
