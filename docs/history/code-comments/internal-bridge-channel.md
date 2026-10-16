# Comment history: `internal/bridge/channel`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero h2

### `go/internal/bridge/channel/apicover_named_test.go:13` — above `func TestPolicy_StallPolicySatisfiesAndDecides(t *testing.T) {`

```text
// TestPolicy_StallPolicySatisfiesAndDecides binds both the Policy interface and
// the *AskAction return type to their real producer StallPolicy.OnEvent.
//   - var _ Policy = StallPolicy{} pins that StallPolicy implements Policy
//     (the seam a future LLM-driven policy must also satisfy — ADR-0037).
//   - On a "stall" envelope OnEvent must return a non-nil *AskAction whose
//     Question field is exactly the configured prompt (full-field assertion).
//   - On any non-stall envelope it must return nil (no spurious injection).
```

### `go/internal/bridge/channel/enablement.go:3` — above `import "strings"`

```text
// enablement.go — ADR-0045 I6: ONE rollout dial. The live bidirectional
// channel (ADR-0037) rides EVOLVE_PHASE_RECOVERY: enforce implies the channel
// on; off/shadow → byte-identical (no .live files, no per-tick delta capture).
// The deprecated explicit flag was retired in v19.x.
//
// The single source for "is the channel on" both the bridge driver and the
// observer adapter call.
```

### `go/internal/bridge/channel/enablement.go:13` — above `func ResolveStage(raw string) string {`

```text
// ResolveStage normalizes a raw recovery stage word to the canonical stage
// vocabulary, the SINGLE home of that rule for BOTH ADR-0044 recovery dials —
// the program dial (the bridge's recoveryStageFromEnv and the observer adapter)
// and, since F27, the fatal-pane dial (fatalPaneStageOf) — so the
// "unset → shadow, typo → off" policy can never drift between readers.
// Unset/empty → "shadow" (an unwired stage observes); off|shadow|enforce →
// as-is, plus "0" → "off" (config.StageOff.String(), the word the composition
// root forwards — accepted explicitly, not by the typo rule); anything else →
// "off" (a typo must never silently enable a kill-path).
```

### `go/internal/bridge/channel/enablement_test.go:5` — above `func TestResolveStage(t *testing.T) {`

```text
// TestResolveStage covers all branches of ResolveStage (ADR-0045 I6).
```

### `go/internal/bridge/channel/enablement_test.go:15` — above `{"0", "off"},`

```text
// config.StageOff.String(): the word the cycle root forwards for off (F27)
```

### `go/internal/bridge/channel/enablement_test.go:35` — above `func TestEnabled_UsesStageOnly(t *testing.T) {`

```text
// TestEnabled_UsesStageOnly — ADR-0045 I6 (EVOLVE_CHANNEL removed in v19.x):
// the channel is implied solely by the EVOLVE_PHASE_RECOVERY stage.
// shadow/off → byte-identical (channel off); enforce → on.
```

### `go/internal/bridge/channel/policy.go:3` — above `type Policy interface {`

```text
// Policy decides whether to ask, from one feed envelope (decoded as a map). A
// smarter (LLM-driven) policy implements the same interface later (ADR-0037
// non-goal for v1).
```

### `go/internal/bridge/channel/producer.go:21` — above `StdoutPath string`

```text
// StdoutPath/StderrPath are the source files the Normalizer tails. Empty
// (the headless default) falls back to <phase>-stdout.log / -stderr.log, so
// headless phases are byte-identical to pre-RT3. A tmux-family phase sets
// these to the driver's live pair (<agent>-pane.live / -breadcrumbs.live),
// whose <phase>-stdout.log stays empty until the at-exit dump (ADR-0037 RT3).
```

### `go/internal/bridge/channel/producer_test.go:96` — above `deadline := time.Now().Add(10 * time.Second)`

```text
// POLL for the WARN instead of sleeping a fixed 60ms: a loaded runner (the
// macOS CI job, 2026-07-30) can starve the 1ms poll loop of the iterations
// the miss threshold needs, and the old fixed sleep then read an empty
// buffer — a wall-clock flake of exactly the async-wait class that
// dominates the flaky-test taxonomy. Generous ceiling, fast exit.
```

### `go/internal/bridge/channel/supervisor.go:74` — above `type Supervisor struct{ cfg SupervisorConfig }`

```text
// Supervisor injects a correlated question via the ADR-0023 inbox and blocks
// until the agent's reply span appears in the feed, then returns the bracketed
// Answer. It is safe for concurrent use across separate Ask calls (each call
// mints its own correlation ID).
```

### `go/internal/bridge/channel/feed.go:1` — above `package channel`

```text
// Package channel is the bidirectional communication channel for long-running
// tmux-REPL phases (ADR-0037): a live filtered inbound feed produced beside the
// observer, plus correlated outbound asks over the ADR-0023 inbox.
```
