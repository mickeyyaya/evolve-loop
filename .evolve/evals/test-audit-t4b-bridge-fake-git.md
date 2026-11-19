---
score_cap:
  - criterion: "the git contract table passes on the fake and on real git"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestGitContract$' ./internal/bridge/"
  - criterion: "a failed git is an exit code with a nil error on the fake and on real git"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestGitContract/(fake|real)/(rev_list_unknown_base_exits_128|log_unknown_rev_exits_128)$' ./internal/bridge/"
  - criterion: "a fake that fails through err fails the contract on every non-zero case"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestGitContract_ErrStyleFakeFailsEveryNonZeroCase$' ./internal/bridge/"
---

# Eval: bridge fake git contract against real git (T4)

> Pins the bridge git part of T4 (D8). `git_contract_test.go` holds one table of cases that runs through the same `gitCmd` path on a model fake and on real git from `internal/gittest`. A fake that reports a failure through the error value, as the old fakes did, fails the table. Source incident: cycle 2490 found that the bridge read a non-zero git exit as data because no fake modeled the real `execRunner`, which returns the exit code with a nil error.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| contract-green | whole table passes on both backends | 4/10 | `go test -run '^TestGitContract$' ./internal/bridge/` |
| exit-not-err | failed git is an exit code with a nil error | 6/10 | `TestGitContract/fake/rev_list_unknown_base_exits_128` |
| err-style-fails | the old err-style fake fails the table | 7/10 | `TestGitContract_ErrStyleFakeFailsEveryNonZeroCase` |
