---
score_cap:
  - criterion: "A console claim or console route between two reads returns superseded with the claim"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2105_00[12]_' ./acs/cycle2105"
  - criterion: "An untouched item returns none"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2105_00[34]_' ./acs/cycle2105"
  - criterion: "The check reads once per ask and a failed read never supersedes"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2105_00[56]_' ./acs/cycle2105"
  - criterion: "workflow.supersede.stage is strict off, shadow, enforce with default shadow"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2105_00[789]_' ./acs/cycle2105"
  - criterion: "The new package is enrolled in the apicover gate and names its API"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2105_010_' ./acs/cycle2105"
---

# Eval: A pure check finds that the console took the item of a live cycle

> Pins the contract of `internal/supersede` (ADR-0158 decision 13): `Check` reads the inbox state of a claimed item on demand and returns none or superseded with the console claim, and `workflow.supersede.stage` is a strict stage key that defaults to shadow. Source incident: cycle 2035, where the cycle spent about 41 LLM minutes after the console took its item. The call site in `internal/core` is out of scope and belongs to the console item `wire-cost-aware-repair-call-sites`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| claim-and-route | superseded with the claim after a console claim or route | 5/10 | `go test -tags acs -run 'TestC2105_00[12]_' ./acs/cycle2105` |
| untouched | none for an untouched item and for non-console states | 6/10 | `go test -tags acs -run 'TestC2105_00[34]_' ./acs/cycle2105` |
| no-poll | one read per ask, a read failure is an error | 6/10 | `go test -tags acs -run 'TestC2105_00[56]_' ./acs/cycle2105` |
| strict-stage | default shadow, strict off/shadow/enforce | 6/10 | `go test -tags acs -run 'TestC2105_00[789]_' ./acs/cycle2105` |
| api-cover | enrolled and named | 7/10 | `go test -tags acs -run 'TestC2105_010_' ./acs/cycle2105` |
