---
score_cap:
  - criterion: "a ref that starts with a dash reaches git after --end-of-options and is read as a revision"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2241_00[123]' ./acs/cycle2241"
  - criterion: "a SHA argument that is not 7 to 64 hex characters is refused before git runs"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2241_00[4567]' ./acs/cycle2241"
  - criterion: "ValidateSHA, GitRefOutput and GitSHAOutput have 100 percent statement coverage"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2241_008 ./acs/cycle2241"
  - criterion: "the acsassert sources carry no comment except the package doc and build directives"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2241_009 ./acs/cycle2241"
---

# Eval: acsassert passes --end-of-options to git and checks a SHA before it runs git

> Pins the contract of `GitRefOutput`, `GitSHAOutput` and `ValidateSHA` in `go/pkg/acsassert`. A ref that starts with a dash must not reach git as an option. A SHA that is not 7 to 64 hex characters must never reach git. Source: inbox item `acsassert-end-of-options-and-sha` (cycle 2241), from operator decision 2026-11-03 (AD2-0 / AD3-4).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| end-of-options | A dash-leading ref is a revision, never an option | 4/10 | `go test -tags acs -run 'TestC2241_00[123]' ./acs/cycle2241` |
| sha-gate | A non-hex or wrong-length SHA is refused before git is spawned | 4/10 | `go test -tags acs -run 'TestC2241_00[4567]' ./acs/cycle2241` |
| coverage | The three new functions are fully covered | 6/10 | `go test -tags acs -run TestC2241_008 ./acs/cycle2241` |
| zero-comments | No comment in the acsassert sources | 6/10 | `go test -tags acs -run TestC2241_009 ./acs/cycle2241` |
