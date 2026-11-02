---
score_cap:
  - criterion: "phaseidentity passes its own tests and has a passing line-limit guard test"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC2119_001 ./acs/cycle2119"
  - criterion: "a test fails when a line of 1024 bytes or more is injected into Authority, Block or WrapPasted"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run TestC2119_002 ./acs/cycle2119"
  - criterion: "the guard accepts a line of 1023 bytes in each injected block"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2119_003 ./acs/cycle2119"
  - criterion: "the guard failure names the block and the 1024-byte limit"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2119_004 ./acs/cycle2119"
---

# Eval: Line-length guard covers every injected block

> The line-length guard of `go/internal/bridge/phaseidentity` covered only `Authority()`. `Block(Facts)` and `WrapPasted` also put lines into a pasted prompt, and a canonical-mode TTY drops the rest of a line of 1024 bytes or more. The predicates inject a padding line into each block with a `go test -overlay` mutant of `identity.go` and require a guard test to fail. Source: plan `docs/plans/opus55-effort-config-2026-10.md` Later section; cycle 2119.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| guard-discoverable | a passing test with `tty`, `limit` or `length` in its name | 5/10 | `TestC2119_001` |
| mutant-killed | 1024-byte line in each of the three blocks fails the guard | 4/10 | `TestC2119_002` |
| boundary | 1023-byte line passes the guard | 6/10 | `TestC2119_003` |
| diagnostic | failure text has the block name and 1024 | 7/10 | `TestC2119_004` |
