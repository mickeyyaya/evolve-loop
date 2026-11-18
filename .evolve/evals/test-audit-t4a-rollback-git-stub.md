---
score_cap:
  - criterion: "the contract table runs the ls-remote shape case on the stub and on real git"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -v -tags integration -run '^TestGitStubContract$' ./internal/rollback | grep -q -- '--- PASS: TestGitStubContract/ls_remote_prints_sha_tab_ref/real'"
  - criterion: "the legacy bare-ref ls-remote output fails the shape case"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -v -tags integration -run '^TestGitStubContract_LegacyOutputFailsTheShapeCase$' ./internal/rollback | grep -q -- '--- PASS: TestGitStubContract_LegacyOutputFailsTheShapeCase'"
  - criterion: "a revert conflict reaches the failed path on the stub and on real git"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -v -tags integration -run '^TestGitStubContract_RevertConflictReachesFailedPath$' ./internal/rollback | grep -q -- '--- PASS: TestGitStubContract_RevertConflictReachesFailedPath/real'"
---

# Eval: rollback git stubs match real git

> Pins task test-audit-t4a-rollback-git-stub (T4 of the 2026-11 test-quality audit). The rollback test stubs printed `refs/tags/v9.9.9` for `git ls-remote`, but real git prints `<sha> TAB <ref>`, so the stubs hid a shape mismatch. A contract table now runs each case on the stub and on real git, and a revert conflict (exit 1, CONFLICT) reaches the failed path. Source incident: cycle 2512 inbox item.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| contract-table | ls-remote shape case runs on stub and real git | 6/10 | `go test -run '^TestGitStubContract$'` |
| legacy-rejected | old bare-ref output fails a case | 7/10 | `go test -run '^TestGitStubContract_LegacyOutputFailsTheShapeCase$'` |
| revert-conflict | conflict reaches the failed path on both backends | 7/10 | `go test -run '^TestGitStubContract_RevertConflictReachesFailedPath$'` |
