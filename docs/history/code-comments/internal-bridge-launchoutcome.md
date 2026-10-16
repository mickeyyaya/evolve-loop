# Comment history: `internal/bridge/launchoutcome`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero D4

### `go/internal/bridge/launchoutcome/cause.go:45` — above `func firstDiagnosticLine(stderr string) string {`

```text
// firstDiagnosticLine picks the one-line cause threaded into the launch
// error chain. Validate-gauntlet failures put the cause FIRST and prefix it
// "[bridge]"; driver failures accumulate launch chatter first and end with
// the causal line (cycle-286: a timeout's first line was a stream_output
// NOTE while "FAIL: completion never signalled" sat last). So: the first
// "[bridge]"-prefixed line wins; otherwise the LAST non-empty line.
```

### `go/internal/bridge/launchoutcome/cause_test.go:15` — above `func TestFirstDiagnosticLine_PrefersCausalLine(t *testing.T) {`

```text
// TestFirstDiagnosticLine_PrefersCausalLine — cycle-286 field evidence: a
// driver-timeout stderr starts with chatter ("[claude-tmux] NOTE: …") and
// ends with the causal line; the gauntlet failures put the cause first
// ("[bridge] …"). The picker must prefer a [bridge]-prefixed line, else the
// LAST non-empty line — never leading chatter.
```

### `go/internal/bridge/launchoutcome/exit.go:1` — above `package launchoutcome`

```text
// Package launchoutcome is unit 10 of the component breakdown (ADR-0103):
// what one bridge launch exit MEANS. It owns the exit vocabulary as ONE table —
// the class name, the registered BRIDGE_EXIT_* signal code, the attempt
// ledger's cause code and the port sentinel each exit wraps — and the
// cause-line miners (the first diagnostic line, the artifact-timeout summary,
// the typed sub-cause). Classify projects the table onto ONE Outcome — the
// error chain the orchestrator parses, the BRIDGE_EXIT_* signal code and the
// ledger cause; CauseCode is the attempt ledger's own projection of the same
// row (llm-calls.ndjson). Pure: no clock, no filesystem, no Center — the host
// (bridge.Engine.Launch) owns the emit and the request gauntlet. Design:
// docs/architecture/decomposition/10-bridgeengine.md.
```

### `go/internal/bridge/launchoutcome/exit.go:40` — above `const ExitSignalDeath = -1`

```text
// ExitSignalDeath is what Go's ExitError.ExitCode() reports for a signal
// death or a failed Start — most commonly exec.CommandContext SIGKILLing the
// driver on our own cancellation. Transient only under a cancelled context
// (the deliverable-authority door, cycle-859); a -1 with a live context is a
// start failure that stays plain.
```

### `go/internal/bridge/launchoutcome/importgraph_test.go:3` — above `import (`

```text
// importgraph_test.go — the package is a leaf under bridge (ADR-0103 unit 10
// §2): stdlib plus internal/core (the request type and the two port
// sentinels) and internal/signalcenter, never internal/bridge itself (the
// compiler is the cycle guard; this is the leaf-ness declaration —
// core/failurelearning/importgraph_test.go idiom).
```

### `go/internal/bridge/launchoutcome/limits_test.go:3` — above `import (`

```text
// limits_test.go — the clean-code limits the design promises (ADR-0103 unit
// 10 §4), enforced by a test rather than by review: every function < 50
// lines, nesting depth ≤ 4, every file < 800 lines (signalcenter/limits_test.go
// idiom; comments inside a function count, its doc comment does not).
```
