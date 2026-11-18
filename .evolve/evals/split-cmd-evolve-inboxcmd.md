---
score_cap:
  - criterion: "TestInboxCmdRun passes in internal/cli/inboxcmd and calls inboxcmd.Run"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestInboxCmdRun$' ./internal/cli/inboxcmd"
  - criterion: "only the shim remains among the non-test cmd_inbox*.go files of go/cmd/evolve"
    max_if_missing: 5
    evidence: "cd go && test \"$(ls cmd/evolve/cmd_inbox*.go | grep -v _test.go | tr '\\n' ' ')\" = 'cmd/evolve/cmd_inbox.go '"
  - criterion: "the inbox verb group behaves the same through the production binary"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2503_003' ./acs/cycle2503"
  - criterion: "the new package is enrolled in the apicover gate and names every export"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2503_004' ./acs/cycle2503"
  - criterion: "the test package of cmd/evolve compiles once the curator tests that need inboxcmd internals have moved"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2503_006' ./acs/cycle2503"
  - criterion: "the six curator tests of the moved fixture run in internal/cli/inboxcmd and no longer sit in cmd/evolve"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2503_007' ./acs/cycle2503"
---

# Eval: Move the inbox verb group to internal/cli/inboxcmd

> Pins step 1 of docs/plans/split-cmd-evolve-2026-11.md: the inbox verb group moves out of
> package main behind a one-file shim and a Deps value. Source incident: cycle 2466 (build floor failed) and cycle 2503 (the snapshot left two curator test files in package main that no longer compile), fleet
> item split-cmd-evolve-inboxcmd (the admission cell of go/cmd/evolve holds every lane).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| red-test | TestInboxCmdRun passes in the new package | 4/10 | `go test -run '^TestInboxCmdRun$' ./internal/cli/inboxcmd` |
| shim-only | one non-test cmd_inbox*.go file remains | 5/10 | `ls cmd/evolve/cmd_inbox*.go` |
| behavior | exit codes and output of the verbs are unchanged | 5/10 | `go test -tags acs -run TestC2503_003 ./acs/cycle2503` |
| compile | go vet -tags integration ./cmd/evolve is clean | 4/10 | `go test -tags acs -run TestC2503_006 ./acs/cycle2503` |
| moved-tests | the six curator tests run in inboxcmd, their files left cmd/evolve | 5/10 | `go test -tags acs -run TestC2503_007 ./acs/cycle2503` |
| apicover | enrolled and every export named | 6/10 | `go test -tags acs -run TestC2503_004 ./acs/cycle2503` |
