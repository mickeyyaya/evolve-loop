# Comment history: `internal/usageprobe`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d10

### `go/internal/usageprobe/cancel_test.go:13` — above `func TestProber_Run_ReturnsPromptlyOnCancelEvenWhenAProbeIgnoresIt(t *testing.T) {`

```text
// 2026-09-15 wave-4 boundary: the loop ignored SIGINT ×2 and SIGTERM for
// over a minute while `[loop] usage-probe: checking …` ran — the probe was
// handed context.Background() and its WaitGroup waited on a bridge session
// that does not return on cancel. The loop's interrupt must win: a cancelled
// context skips the probe entirely, and a cancel that lands mid-probe returns
// promptly even when a probe ignores it, naming what was abandoned.
```

### `go/internal/usageprobe/usageprobe.go:82` — above `done := make(chan struct{})`

```text
// A probe over the bridge may not return on cancel (a pane that never
// answers); the loop's interrupt must not wait for it — 2026-09-15, the
// wave-4 boundary needed SIGKILL after SIGINT ×2 and SIGTERM sat here.
```
