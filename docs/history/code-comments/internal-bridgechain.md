# Comment history: `internal/bridgechain`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d9

### `go/internal/bridgechain/bridgechain.go:1` — above `package bridgechain`

```text
// Package bridgechain makes the CLI/tier fallback chain a property of the
// bridge HANDLE, not of one caller.
//
// The shared phase runner has walked a chain since WS-G1 (llmroute.Dispatch)
// and WS-876 (DispatchTiered): on a trigger exit — 80 REPL boot timeout, 81
// artifact timeout, 124 command timeout, 127 missing binary — it advances to
// the next CLI; when every CLI at a tier exits 85 (a quota wall) it steps down
// a tier; a classified wall benches the family (clihealth). Every launcher that
// called the bridge itself had none of it: the retro, the failure advisor, the
// phase judge, the retry adjudicator, the swarm launcher. Lanes 1676 and 1677
// (2026-09-14) paid for that: the retro launched on codex, codex timed out
// after 1800 s, the retro emitted FAIL with no disposition and the cycle sealed
// abnormally — one CLI's timeout costing the whole cycle's work.
//
// Walking is a Decorator (GoF) over core.Bridge. Wrapped ONCE at the
// composition root (cmd/evolve wireOrchestratorDeps) it gives every Launch the
// walk by construction; a caller that walks its own chain (the runner, the
// advisor) marks each attempt with BridgeRequest.ChainAttempt and is passed
// straight through, so no launch is walked twice. CLIHealthEnabled,
// ApplyCLIHealthBench and BenchOnEscalation are the runner's cli-health
// projections hoisted here so both walks share one implementation.
```

### `go/internal/bridgechain/bridgechain_test.go:59` — above `func TestWalking_ArtifactTimeoutAdvancesToTheNextCLI(t *testing.T) {`

```text
// TestWalking_ArtifactTimeoutAdvancesToTheNextCLI reproduces lanes 1676/1677
// (2026-09-14): the retro launched the bridge itself, codex exited 81 after the
// artifact window, and the cycle sealed with no disposition. Through the walk
// the next CLI in the chain runs, every attempt is marked, and the fallback is
// one log line — the runner's own contract, now the bridge handle's.
```
