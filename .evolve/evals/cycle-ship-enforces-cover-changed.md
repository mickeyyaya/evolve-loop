---
score_cap:
  - criterion: "The compiled default of cover_changed.stage is enforce; an explicit shadow policy is kept"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1940_001_CoverChangedCompiledDefaultIsEnforce$' ./acs/cycle1940"
  - criterion: "With no cover_changed policy block, the production build floor (core.DefaultBuildFloorChecks) refuses a cycle diff that adds an uncovered branch, and names the function"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1940_002_BuildFloorRefusesUncoveredChangedFunctionByName$' ./acs/cycle1940"
  - criterion: "A fully covered change, an empty diff, a function that only the base changed, and an explicit shadow policy are not refused"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1940_00[3-6]_' ./acs/cycle1940"
  - criterion: "The existing cover-changed floor contract stays green"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run CoverChanged ./internal/core ./internal/policy"
  - criterion: "Under the enforce default a changed package whose tests fail is refused once, and a package that fails only under -tags integration is still refused"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1940_00[78]_' ./acs/cycle1940"
  - criterion: "The integration-tier floor test and the floor arg tests are green inside a fleet lane env"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1940_0(09|10)_' ./acs/cycle1940"
---

# Eval: Cycles refuse changed functions below 100% coverage

> Pins the enforce default of the build floor's cover-changed check. Loop cycles 1869 and 1870
> shipped `internal/cyclestate/read.go` (ReadLastCycleNumber 90.9%, ReadState 88.9%) and
> `internal/redteamcheck` (NoBatchCycleJump 75.0%) below 100%. The floor only warned in shadow,
> so console PRs #883 and #887 went red in CI on functions they did not touch (2026-10-13).
> Source: inbox item cycle-ship-enforces-cover-changed, cycle 1934. The recovery rung is the
> build handoff floor itself: a refusal goes back to Build as a floor failure.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| compiled-default | stage defaults to enforce, shadow override kept | 8/10 | `TestC1940_001` |
| refusal-by-name | uncovered changed branch refused, `p.F` named | 9/10 | `TestC1940_002` |
| preservation | covered, empty, base-only and shadow cases pass | 7/10 | `TestC1940_003..006` |
| regression | existing CoverChanged unit tests green | 6/10 | `go test -run CoverChanged ./internal/core ./internal/policy` |
| single-refusal | a failing changed package is refused once; integration-only failure still refused | 8/10 | `TestC1940_007..008` |
| integration-tier | audit offenders green in the lane env (cycle 1940 audit repair) | 7/10 | `TestC1940_009..010` |
