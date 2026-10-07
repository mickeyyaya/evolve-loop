---
score_cap:
  - criterion: "a repo-contract pack cancelled by its context leaves no test binary running"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1829_001_' ./acs/cycle1829"
  - criterion: "a cancelled pack still names no test: RunRepoContractPack returns no reds and the build floor no finding"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1829_002_' ./acs/cycle1829"
---

# Eval: a cancelled repo-contract pack kills its test binaries

> runGoTestJSON ran `go test -json` under exec.CommandContext, which kills only the `go` process: a pack cancelled at the build floor's 120s deadline (or by ship) left its test binary running with PPID 1 until the binary's own 20m -test.timeout, one orphan per handoff and per correction (inbox 2026-09-29, measured with a throwaway probe). Pins that the build floor's production pack (core.RepoContractFloorChecks over ship.RunRepoContractPack), cancelled while a lane tree's test binary runs, leaves that binary dead, and that the cancel still classifies as naming nothing. Source incident: cycle 1829 RED run, survivor pid recorded by the fixture after the floor returned.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| no-orphan | the test binary is gone after the cancel | 8/10 | `go test -tags acs -run '^TestC1829_001_' ./acs/cycle1829` |
| cancel-names-nothing | no reds, no floor finding, exit error kept | 7/10 | `go test -tags acs -run '^TestC1829_002_' ./acs/cycle1829` |
