---
score_cap:
  - criterion: "Violation.Error names each package on the cycle exactly once, closing the loop at the referenced package"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1856_00[12]_' ./acs/cycle1856"
  - criterion: "extractPins carries no never-set error return, so ExtractFrozenPins and CheckFrozenPins hold no unreachable error branch, and an unreadable frozen file stays fail-open"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1856_00[34]_' ./acs/cycle1856"
  - criterion: "The regexp identifier goSourceLiteal is renamed goSourceLiteral"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run TestC1856_005_ ./acs/cycle1856"
  - criterion: "The reachabilityprobe package tests pass with no regression"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/reachabilityprobe"
---

# Eval: reachabilityprobe Violation.Error prints the pinning package twice, and the pin-extraction error path is dead

> Pins the diagnostic contract of `(*Violation).Error()` in `go/internal/reachabilityprobe`: the
> cycle chain runs from the referenced package to the pinning package, and the message closes the
> loop by naming the referenced package again (`storage -> core -> storage`), never by repeating the
> pinning package (`storage -> core -> core`). It also pins the removal of the dead `error` result of
> the private `extractPins` (it never returned a non-nil error, so the callers' branches could never
> fire) while preserving fail-open handling of an unreadable frozen file, and the `goSourceLiteral`
> spelling. Source incident: comment-reduction batch 29 (2026-09-26), implemented in cycle 1856.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| exact-message | Error closes the cycle at ReferencedPackage, direct and via CheckFrozenPins | 7/10 | `go test -tags acs -run 'TestC1856_00[12]_' ./acs/cycle1856` |
| dead-error-path | extractPins single result; callers bind no error; fail-open preserved | 5/10 | `go test -tags acs -run 'TestC1856_00[34]_' ./acs/cycle1856` |
| identifier-spelling | goSourceLiteral declared, goSourceLiteal gone | 3/10 | `go test -tags acs -run TestC1856_005_ ./acs/cycle1856` |
| no-regression | package unit tests green | 6/10 | `go test ./internal/reachabilityprobe` |
