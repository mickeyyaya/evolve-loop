---
score_cap:
  - criterion: "A real gh in a test sees no login after gittest.IsolateHostCredentials"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestIsolateHostCredentials_ARealGhInATestSeesNoLogin$' ./internal/gittest"
  - criterion: "The helper sets the hermetic git and gh values, restores them and reports failures"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestIsolateHostCredentials_' ./internal/gittest"
  - criterion: "The cmd/evolve TestMain calls the helper so every test of the package is hermetic"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestMain_IsolatesHostCredentialsForEveryTestInThePackage$' ./cmd/evolve"
---

# Eval: gittest.IsolateHostCredentials

> Pins the AD1-4 component of the 2026-11-03 architecture decisions plan. One helper in `internal/gittest` sets GH_CONFIG_DIR, GIT_CONFIG_GLOBAL, GIT_CONFIG_NOSYSTEM, GIT_TERMINAL_PROMPT and GIT_SSH_COMMAND, drops GH_TOKEN and GITHUB_TOKEN, and returns a restore function. The cmd/evolve TestMain uses it. Source: cycle 2138, inbox item arch-decisions-ad1-4-a-hermetic-helper-gittest.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| gh-no-login | A real gh sees no login | 4/10 | `go test -run TestIsolateHostCredentials_ARealGhInATestSeesNoLogin ./internal/gittest` |
| helper-contract | Values, restore and failure paths | 6/10 | `go test -run TestIsolateHostCredentials_ ./internal/gittest` |
| testmain-wiring | TestMain reaches the helper | 6/10 | `go test -run TestMain_IsolatesHostCredentialsForEveryTestInThePackage ./cmd/evolve` |
