# Build Explanation — Cycle 2451

## Build Binding
- Cycle: 2451
- Base SHA: ced71cdd2d1033fc8937a6fd47629c596babee4d

## Summary
`evolve agent run` now reports a fence revert as a hard warning. A writer seat that changed a file outside its grant gets a WARN line on stderr and exit code 12.

## Rationale
Before this change the fence removed the stray file but stderr stayed silent, and exit 1 was shared with every FAIL. A console script cannot tell a grant error from a real failure. A code of its own lets the console re-run with a corrected grant.

## Changed Areas
- `go/internal/cli/agentcmd/run.go` — `exitCode` and `finish` take the whole outcome and return the fence-revert code.
- `go/internal/cli/agentcmd/verdict.go` — `fenceReverted` finds the revert message and `warnFenceRevert` prints the WARN line with the next step.
- `go/internal/cli/agentcmd/agent.go` — adds `exitFenceRevert` and documents exit 12 in the usage.
- `go/internal/treefence/fence.go` — exports `RestoredMarker`, so the matched text has one home.
- `go/internal/treefence/apicover_named_test.go` — names the new export in a real assertion.
- `go/internal/cli/agentcmd/agent_fence_grant_test.go` — the created-file journey now expects exit 12, not exit 1.
- `go/internal/cli/agentcmd/agent_fence_warn_test.go` — new tests for the WARN line, the distinct code, the re-run and the usage.
- `docs/architecture/packages/internal-cli-agentcmd.md` — records the fence-revert contract.

## Design Decisions
The WARN line reuses the fence diagnostic that already lists every reverted path. A second path parser duplicates it. The envelope verdict is unchanged, because an existing test pins PASS for an existing-file revert.

## Verification
`go test -count=1` passes for `internal/cli/agentcmd` and `internal/treefence`. The four ACS predicates of cycle 2451 pass.

## Compatibility
Exit 12 is new. A created-file revert changes from exit 1 to exit 12. The JSON envelope is unchanged.

## Limitations
The WARN line is as long as the fence message, which includes the reverted patch path. A read-only seat is not covered.
