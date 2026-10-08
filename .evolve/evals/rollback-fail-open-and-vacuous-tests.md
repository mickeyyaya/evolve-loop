---
score_cap:
  - criterion: "A failing git ls-remote yields tag_delete failed, not not-present"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1839_001_' ./acs/cycle1839"
  - criterion: "gh release view failures other than not-found yield failed; not-found stays not-present"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1839_002_' ./acs/cycle1839"
  - criterion: "The default gh release step runs in RepoRoot"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1839_003_' ./acs/cycle1839"
  - criterion: "Rollback unit tests assert their claims hermetically"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/rollback"
  - criterion: "The rollback integration tier encodes fail-closed lookups and passes"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1839_005_' ./acs/cycle1839"
---

# Eval: rollback fails closed and its tests assert

> Pins the fail-closed contract of internal/rollback: lookup failures (ls-remote, gh release view) are `failed`, only a confirmed absence is `not-present`, gh runs in RepoRoot, and the nil-steps and mkdir unit tests assert instead of passing vacuously. Source: inbox item rollback-fail-open-and-vacuous-tests, cycle 1839.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| fail-closed-tag | ls-remote error is failed | 4/10 | `TestC1839_001_` |
| fail-closed-release | non-404 gh error is failed | 5/10 | `TestC1839_002_` |
| repo-root | gh cwd is RepoRoot | 5/10 | `TestC1839_003_` |
| hermetic-tests | unit tests assert | 6/10 | `go test ./internal/rollback` |
| integration-tier | integration tests encode fail-closed and pass | 5/10 | `TestC1839_005_` |
