---
score_cap:
  - criterion: "TestLandCmdRun passes in internal/cli/landcmd for the pr and ci verbs"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestLandCmdRun$' ./internal/cli/landcmd"
  - criterion: "Only the cmd_pr.go and cmd_ci.go shims remain among the non-test pr and ci files of go/cmd/evolve"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2564_002' ./acs/cycle2564"
  - criterion: "The registry verbs pr and ci reach landcmd through the shims with a wired Deps value"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2564_003' ./acs/cycle2564"
  - criterion: "pr and ci behave the same through the production binary after the move"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2564_004' ./acs/cycle2564"
  - criterion: "landcmd is enrolled in go/.apicover-enforce and names every exported symbol"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2564_005' ./acs/cycle2564"
  - criterion: "No dead copy of a moved verb stays in main and the flag parser has one home"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2564_006' ./acs/cycle2564"
---

# Eval: Move the pr and ci verbs into go/internal/cli/landcmd behind shims

> Pins step 2 of docs/plans/split-cmd-evolve-2026-11.md. The `pr` verb group
> (merge, window, boundary, refresh) and the `ci` verb group (classify, watch,
> telemetry, darwin-set) leave `package main` for `internal/cli/landcmd`. Two shim
> files stay in `go/cmd/evolve` and pass a `Deps` value for the helpers that
> `main` owns. The pattern is the one of cycle 2503 (inboxcmd).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| red-test | TestLandCmdRun passes in the new package | 4/10 | `go test -run '^TestLandCmdRun$' ./internal/cli/landcmd` |
| shim-only | only cmd_pr.go and cmd_ci.go remain | 5/10 | `TestC2564_002` |
| wiring | registry verbs reach landcmd with a wired Deps | 5/10 | `TestC2564_003` |
| behavior | same exit codes and messages through the binary | 6/10 | `TestC2564_004` |
| apicover | enrolled and every export named | 6/10 | `TestC2564_005` |
| no-dead-copy | no moved verb stays in main; one flag parser | 6/10 | `TestC2564_006` |
