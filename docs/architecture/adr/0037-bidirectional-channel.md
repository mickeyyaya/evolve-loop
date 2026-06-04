# ADR 0037 — Bidirectional channel for long-running tmux-REPL phases

Status: ACCEPTED (2026-06-04), AMENDED (2026-06-04, RT0–RT5) · Builds on: ADR-0020 (event stream), ADR-0023 (live injection), ADR-0030 (observer autospawn), ADR-0036 (content-vs-liveness channel protocol)

> **⚠ Implementation correction (RT0–RT5).** A holistic review + a **real live capture**
> ([`knowledge-base/research/tmux-live-capture-2026-06-04/`](../../../knowledge-base/research/tmux-live-capture-2026-06-04/NOTES.md))
> disproved three assumptions in the original decision below. The shipped channel differs:
>
> 1. **Live content source is `<agent>-pane.live`, not `<agent>-stdout.log`.** A tmux driver's
>    stdout.log is empty until the at-exit dump (it streams to the tmux *pane*). The driver
>    now polls `capture-pane` each tick and appends newly-stabilized rendered lines (extracted
>    per-CLI by `panestream.PaneDelta`) to `<agent>-pane.live`. (`tmux pipe-pane` was tried
>    first and reverted — its raw stream is 2D cursor motion that needs a terminal emulator;
>    `capture-pane` is already rendered. See the capture NOTES.)
> 2. **Breadcrumbs go to `<agent>-breadcrumbs.live`, not stderr.** The original `deps.Stderr`
>    sink was an in-memory stream the Producer never read, so correlation never functioned.
>    The driver now appends `inject_applied` / `idle_reached` to a `<agent>-breadcrumbs.live`
>    file (O_APPEND), which the Producer tails as its `StderrPath` (RT3 transport-aware spawn:
>    tmux → the `.live` pair, headless → the legacy `<phase>-stdout/-stderr.log`).
> 3. **busy→idle is detected by `panestream.PaneBusy`, not the prompt marker.** The input-box
>    marker PERSISTS during generation for claude/agy (and ollama echoes it on the prompt
>    line), so the original "idle = marker visible" check was always-idle and `idle_reached`
>    never fired. `PaneBusy` keys on the real per-CLI signal: the interrupt/cancel affordance
>    (claude `esc to interrupt`, agy `esc to cancel`) OR an absent idle placeholder (ollama
>    `Send a message`). codex has no capturable signal in the fixtures → documented
>    weak-signal degradation (monitoring works; its span cannot be bracketed).
>
> Sections 3, 4, and 7 below are corrected inline. The Supervisor/Watch/Gating decisions are
> unchanged. RT4 also sized `collectSpan`'s scanner buffer (1 MB/10 MB) so a >64 KB answer
> line is not silently dropped from the recovered span.

> **TL;DR.** Long-running tmux-REPL phases now have a live, filtered content feed
> (`<agent>-channel.ndjson`) and a Go API for correlated mid-task asks
> (`channel.Supervisor.Ask`). The observer (ADR-0030) is extended to double as the channel
> producer — the sole writer of the feed. Correlation (request/response bracketing) is
> driver-owned via stderr breadcrumbs; no agent-side cooperation is required. Headless
> `claude -p` phases degrade gracefully to read-only monitoring. A read-only human debug tail
> (`evolve bridge watch`) is symmetric with the existing `evolve bridge send`. Gated behind
> `EVOLVE_CHANNEL=1`; off is byte-identical to before.

## Context

For phases that run for minutes or longer under a tmux REPL driver, two needs are unmet:

1. **Continuous monitoring** — `<phase>-events.ndjson` (ADR-0020) is produced *post-phase*.
   The live `phasestream.Normalizer.Poll` was built (ADR-0020) but never wired to a live
   output file; it has been dormant since landing.
2. **Correlated mid-task asks** — `evolve bridge send` (ADR-0023) lets the host inject
   questions into a running tmux REPL, but there is no mechanism to capture *which* agent
   output is the answer to *which* injected question.

The two-channel protocol from ADR-0036 — content channel (`events.ndjson`) vs. raw liveness
floor (byte-growth + pane-hash) — is the prerequisite: this ADR adds a **live** content
channel alongside the existing post-phase one, while explicitly preserving the raw liveness
floor.

The design was specified in
[`docs/superpowers/specs/2026-06-04-bidirectional-channel-design.md`](../../superpowers/specs/2026-06-04-bidirectional-channel-design.md)
and all implementation listed in the Files section below is merged on the
`feat/bidirectional-channel-design` branch.

## Decision

### 1. Transport — tmux REPL only; headless degrades to read-only

The bidirectional channel is scoped to tmux-REPL phases (`claude-tmux`, `codex-tmux`,
`agy-tmux`). These already drain the ADR-0023 inbox for outbound injection. Headless
`claude -p` cannot receive mid-turn input; for headless phases `channel.Supervisor.Ask`
returns `ErrTransportNoInject` and the feed operates in read-only monitoring mode.

### 2. Live feed — `<workspace>/<agent>-channel.ndjson`

A new append-only NDJSON file produced during the phase (not post-phase). It uses the same
`phasestream.Envelope` schema as `events.ndjson` (ADR-0020) plus one new envelope kind:

```
kind: "correlation"
  sub: "request"            data: { corr_id, at_seq }
  sub: "response_complete"  data: { corr_id, span: [start_seq, end_seq] }
  sub: "response_timeout"   data: { corr_id, waited_s }
```

Content and liveness envelopes keep the existing schema. The post-phase `events.ndjson`
path is unchanged.

### 3. Producer — the autospawn observer doubles as the channel producer

The observer (ADR-0030) already runs concurrently beside every phase. When
`EVOLVE_CHANNEL=1`, a `channel.Producer` goroutine is spawned alongside the observer's
stall-detection loop. The Producer instantiates a `phasestream.Normalizer` with
`StdoutPath`, `StderrPath`, and an append-only sink pointing at the feed file, then calls
`Poll()` each tick to emit filtered content + correlation envelopes into the feed.

> **Corrected (RT3).** The Producer's source paths are now **transport-aware**
> (`CoreAdapter.channelSourcePaths`): a tmux-family phase reads `StdoutPath =
> <agent>-pane.live` + `StderrPath = <agent>-breadcrumbs.live`; a headless phase keeps the
> legacy `<phase>-stdout.log` / `<phase>-stderr.log` (empty `Stdout/StderrPath` on
> `ProducerConfig` falls back to those defaults, so headless is byte-identical). Family is
> resolved best-effort from the per-phase CLI env (`EVOLVE_<AGENT>_CLI` > `EVOLVE_CLI` >
> default `claude-tmux`); a profile-pinned CLI not surfaced in env is mis-read as tmux —
> a contained degradation (the Normalizer WARNs on a missing file, never blocks).

The Producer is the **sole writer** of the feed file, honoring the sequential-write-discipline
invariant. The observer's raw byte-growth and pane-hash liveness floor (ADR-0036 Channel B)
is untouched.

### 4. Correlation — driver-owned bracketing, no agent cooperation

`inbox.Envelope` gains one optional field `CorrID string`. When the tmux driver delivers a
`CorrID`-bearing idle-gated ask, it appends a structured breadcrumb to
`<agent>-breadcrumbs.live` (the file the Producer tails as `StderrPath` — **corrected from
the original stderr sink, which the Producer never read**):

```json
{"evolve_channel":"inject_applied","corr_id":"<id>"}
```

When the REPL returns to idle after that injection (busy→idle), the driver appends:

```json
{"evolve_channel":"idle_reached","corr_id":"<id>"}
```

> **Corrected (RT2/RT5).** "Busy" is **not** the prompt marker being absent — the input box
> persists during generation for claude/agy. The driver brackets on
> `panestream.PaneBusy(pane, profile)`: busy = an interrupt/cancel affordance present
> (`esc to interrupt` / `esc to cancel`) OR the profile's idle placeholder absent (ollama
> `Send a message`). `idle_reached` fires exactly once on the busy→idle transition. codex
> has no capturable busy signal → its span is not bracketed (weak-signal degradation).

The Producer's classifier turns these breadcrumbs into `KindCorrelation` envelopes:
`request{corr_id,at_seq}` on `inject_applied` and `response_complete{corr_id,start_seq,end_seq}`
on `idle_reached`. The content envelopes in `[start_seq, end_seq]` are the agent's answer.
A per-`corr_id` timeout emits `response_timeout` if the bracket never closes.

### 5. Supervisor — Go API for correlated asks

`channel.Supervisor` exposes:

```go
type Supervisor interface {
    Ask(ctx context.Context, question string) (Answer, error) // ErrTransportNoInject for headless
    Feed() <-chan Envelope                                     // live tail of the channel feed
}
```

`Ask` appends to the inbox (reusing the existing ADR-0023 send path with a fresh `CorrID`),
then blocks on the feed until the matching `response_complete` arrives — honoring context
cancellation and returning `ErrResponseTimeout` on timeout.

`channel.StallPolicy` is the minimal default consumer: on a `stall` envelope from the
observer it injects "Summarize progress so far and any blockers in 3 bullets." A
smart/LLM policy is a deferred follow-up against the same `Policy` interface. The
supervisor auto-attaches when `EVOLVE_CHANNEL_SUPERVISOR=1` (reserved; not yet wired to
the orchestrator).

### 6. Human debug tail — `evolve bridge watch`

`evolve bridge watch --workspace DIR --agent NAME [--follow]` tails the feed read-only and
pretty-prints the filtered stream with correlation markers inlined. It never writes the
inbox or the feed. Symmetric with `evolve bridge send` (ADR-0023).

### 7. Single-writer ownership map

| Component | Sole writer of | Reads |
|---|---|---|
| `channel.Producer` (observer goroutine) | `<agent>-channel.ndjson` feed | tmux: `<agent>-pane.live` (content) + `<agent>-breadcrumbs.live` (breadcrumbs); headless: `<phase>-stdout.log` + `<phase>-stderr.log` |
| tmux driver | `<agent>-pane.live` (capture-pane delta) + `<agent>-breadcrumbs.live` (correlation) | inbox (drains — existing), `capture-pane` (read-only) |
| `channel.Supervisor` / `evolve bridge send` | inbox (`inbox.Append`) | feed |
| `evolve bridge watch` | — (read-only) | feed |
| observer liveness floor | unchanged | raw `stdout.log` byte-growth + pane-hash (Channel B, ADR-0036) |

No file has two writers.

### 8. Gating

`EVOLVE_CHANNEL=1` enables the producer + feed. Off (the default) is byte-identical to
before: no producer goroutine is spawned, no feed file is created. `EVOLVE_CHANNEL_SUPERVISOR`
is reserved to gate supervisor auto-attach; currently opt-in manual wiring only.

## Consequences

- **Live content feed activated.** The dormant `phasestream.Normalizer.Poll` (ADR-0020) is
  finally wired to a live output file — activating the live normalizer path ADR-0020 built
  but never used.
- **Correlated ask↔answer without agent cooperation.** Bracketing is driver-owned via
  `<agent>-breadcrumbs.live` breadcrumbs + `panestream.PaneBusy`; no changes to agent prompts
  or personas required.
- **ADR-0036 liveness floor preserved.** The raw byte-growth + pane-hash stall floor is
  explicitly untouched; the content feed is added alongside and never filters the liveness
  path. The false-stall fix (observer tmux liveness probe) has no regression risk.
- **Post-phase `events.ndjson` unchanged.** The live feed is a separate file; all existing
  consumers of `events.ndjson` (cyclecost, cycleclassify, backfill, orchestrator) are
  unaffected.
- **Headless phases degrade gracefully.** `Ask` returns `ErrTransportNoInject`; the live
  feed still operates read-only via the Normalizer (content without the injection half).
- **Smart supervisor deferred.** A policy that reasons over the feed content and formulates
  follow-up questions is a future addition against the `Policy` interface; this ADR ships
  only the plumbing and a minimal stall-ask default.
- **Coverage:** all changed internal packages ≥95% (post-RT5 sweep) — `internal/bridge/inbox`
  96.6%, `internal/phasestream` 96.3%, `internal/bridge/channel` 96.5%,
  `internal/bridge/panestream` 100%, `internal/adapters/observer` 97.8%.

## Files (shipped)

- **New:** `go/internal/bridge/channel/` — `feed.go`, `producer.go`, `supervisor.go`,
  `policy.go`; `go/internal/bridge/panestream/panedelta.go` (capture-pane delta extractor +
  per-CLI `Profiles` + `PaneBusy`); `go/cmd/evolve/cmd_bridge_watch.go`
- **Edited:** `go/internal/bridge/inbox/` (CorrID field); `go/internal/bridge/driver_tmux_repl.go`
  (RT2 per-tick `PaneDelta` → `<agent>-pane.live`, breadcrumbs → `<agent>-breadcrumbs.live`,
  RT5 `PaneBusy` bracket); `go/internal/bridge/channel/producer.go` (RT3 `StdoutPath`/`StderrPath`),
  `go/internal/bridge/channel/supervisor.go` (RT4 sized scan buffer); `go/internal/phasestream/`
  (`KindCorrelation` + `correlation.go` + classifier wiring); `go/internal/adapters/observer/core_adapter.go`
  (RT3 transport-aware `channelSourcePaths` + spawn Producer)

## References

- [ADR-0020 — Unified phase event stream](0020-unified-phase-event-stream.md) — established
  `phasestream` + `events.ndjson`; the dormant live `Normalizer.Poll` this ADR activates.
- [ADR-0023 — Live injection and launch rules](0023-live-injection-and-launch-rules.md) —
  the ADR-0023 inbox the supervisor reuses for outbound asks; `evolve bridge send`.
- [ADR-0030 — Phase-observer autospawn in `evolve loop`](0030-phase-observer-autospawn-in-evolve-loop.md)
  — the observer this ADR extends to become the channel producer.
- [ADR-0036 — Content vs liveness channel protocol](0036-content-vs-liveness-channel-protocol.md)
  — the two-channel protocol this ADR implements: live content feed (Channel A) added
  alongside the raw liveness floor (Channel B, untouched).
- [Design spec](../../superpowers/specs/2026-06-04-bidirectional-channel-design.md) —
  pre-implementation design doc this ADR graduates.
