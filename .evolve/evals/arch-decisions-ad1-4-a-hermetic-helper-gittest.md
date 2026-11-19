---
score_cap:
  - criterion: "The named red test of the helper stays green: a gh in a test sees no login"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestIsolateHostCredentials_ARealGhInATestSeesNoLogin$' ./internal/gittest"
  - criterion: "The test binaries of changedpkgs, dossier and core isolate host credentials through TestMain"
    max_if_missing: 4
    evidence: "cd go && GH_TOKEN=x GIT_CONFIG_GLOBAL=/host go test -count=1 -run '^TestMain_IsolatesHostCredentialsForEveryTestIn' ./internal/changedpkgs ./internal/dossier"
  - criterion: "No hand-written GIT_CONFIG line remains in the adopting test files"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2543_005' ./acs/cycle2543"
  - criterion: "The package notes of the changed packages state the helper in STE"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2543_009' ./acs/cycle2543"
---

# Eval: Adopt gittest.IsolateHostCredentials in the test binaries (AD1-4)

> Pins the adoption of `gittest.IsolateHostCredentials()` by the test binaries of changedpkgs,
> dossier, core and cmd/evolve, and the removal of their hand-written `GIT_CONFIG_GLOBAL` lines.
> Source: docs/plans/architecture-decisions-2026-11.md AD1-4, cycle 2543.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| helper-green | The named red test stays green | 5/10 | `go test -run TestIsolateHostCredentials_ARealGhInATestSeesNoLogin ./internal/gittest` |
| testmain-adoption | TestMain isolates credentials | 4/10 | `go test -run TestMain_IsolatesHostCredentialsForEveryTestIn ./internal/...` |
| no-hand-lines | No hand-written config line | 5/10 | `go test -tags acs -run TestC2543_005 ./acs/cycle2543` |
| ste-notes | Package notes state the change | 6/10 | `go test -tags acs -run TestC2543_009 ./acs/cycle2543` |
