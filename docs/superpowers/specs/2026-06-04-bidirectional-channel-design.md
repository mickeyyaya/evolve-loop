# Design — Bidirectional communication channel for long-running LLM-CLI phases

Date: 2026-06-04 · Status: DESIGN (pre-implementation) · Builds on: ADR-0020 (event stream), ADR-0023 (live injection), ADR-0030 (observer autospawn), ADR-0036 (content-vs-liveness channels)

## Problem

For long-running phases we want to **continuously monitor** the running LLM CLI and
**ask it follow-up questions mid-task**, getting useful, correlated feedback back — without
waiting for the phase to finish. Today:

- **Outbound (host→agent) exists** — `evolve bridge send --kind=command|interrupt|nudge|system_rule|keystroke`
  appends to an NDJSON inbox the `*-tmux` driver drains every 2s (ADR-0023).
- **Inbound (agent→host) is batch-only** — `<phase>-events.ndjson` is produced *post-phase*
  (`phasestream.Produce`, `go/internal/phases/runner/runner.go:171`). The live
  `phasestream.Normalizer.Poll` exists but is **dormant** (ADR-0020 note).
- There is **no correlation** between an injected question and the agent's answer.

So "keep getting useful feedback *during* a long task" and "ask and get *that* answer back"
are the missing pieces.

## Decisions (locked during brainstorming)

| # | Decision | Choice |
|---|---|---|
| 1 | Who drives the loop | **Programmatic supervisor/agent** is the real consumer; the human side is a **read-only debug tail**, not a control surface |
| 2 | Transport | **tmux REPL** (`claude-tmux`/`codex-tmux`/`agy-tmux`) — reuse the ADR-0023 inbox for outbound; wake the dormant live `Normalizer` for inbound |
| 3 | Ask↔answer | **Correlated request/response** via **driver-owned bracketing** (no agent-side echo) |
| 4 | Attach surface | **File-based, no server**; `evolve bridge watch` is a read-only human debug tail; the supervisor uses a Go API + the existing inbox |
| 5 | Producer structure | **Approach A** — the existing concurrent **observer** becomes the channel producer (single writer of the feed) |
| 6 | Scope | **Channel plumbing** + a **minimal default supervisor policy**; a smart/LLM supervisor brain is a deferred follow-up |

## Architecture

### Single-writer ownership map

Every component writes only what it already owns — so no file ever has two writers (honors
the sequential-write-discipline invariant).

| Component | Role | Sole writer of | Reads |
|---|---|---|---|
| Live feed `<agent>-channel.ndjson` | inbound surface | — | — |
| **Observer** (extended) | channel **producer** | the feed | raw `<agent>-stdout.log` (content) + `<agent>-stderr.log` (driver breadcrumbs) |
| **tmux driver** (extended) | transport | its own `stderr.log` breadcrumbs | the inbox (drains — existing) |
| **Supervisor** (new) | the "asker" | the inbox (`inbox.Append` — existing) | the feed |
| **`evolve bridge watch`** | human debug | — (read-only) | the feed |

### Why a new feed file (`<agent>-channel.ndjson`) rather than reusing `events.ndjson`

`events.ndjson` is produced post-phase on a frozen timing contract for `cyclecost` /
`cycleclassify`. The live channel uses the already-built `Normalizer.Poll` to write a
**separate** live file with the **same envelope schema** — zero risk to post-phase consumers,
and it finally activates the live normalizer ADR-0020 built. The post-phase `events.ndjson`
path is unchanged.

## Components

### 1. Live feed — `<workspace>/<agent>-channel.ndjson`

Append-only NDJSON, same `phasestream.Envelope` schema as `events.ndjson` (ADR-0020) plus one
new envelope kind:

```
kind: "correlation"
  sub: "request"            data: { corr_id, at_seq }
  sub: "response_complete"  data: { corr_id, span: [start_seq, end_seq] }
  sub: "response_timeout"   data: { corr_id, waited_s }
```

Content + liveness envelopes keep the existing schema. The feed is the **content channel**
(filtered per ADR-0036) — never the raw liveness signal.

### 2. Observer (extended) — the channel producer

The observer already runs concurrently (ADR-0030 autospawn) and tails `<agent>-stdout.log`.
Changes:
- Instantiate a `phasestream.Normalizer{ StdoutPath: <agent>-stdout.log, StderrPath:
  <agent>-stderr.log, Sink: <append-only feed handle> }` and call `Poll()` each tick. This
  emits filtered content + the coalesced progress tick + stall envelopes into the feed (the
  Normalizer already does all of this).
- A new classifier rule recognizes the driver's structured breadcrumb lines in the stderr
  stream and emits `correlation` envelopes (`request` on `inject_applied`,
  `response_complete` on `idle_reached`, span = the content seq-range between them).
- Arms a per-`corr_id` timeout; emits `response_timeout` if the bracket never closes.
- The observer's **raw byte-growth / pane-hash liveness floor is unchanged** (ADR-0036).

### 3. tmux driver (extended) — breadcrumbs + CorrID

- `inbox.Envelope` gains one optional field `CorrID string`.
- In `injectEnvelope` (`go/internal/bridge/driver_tmux_repl.go`), when an ask envelope with a
  `CorrID` is delivered, print one structured breadcrumb to **stderr** (the driver already
  owns its log): `{"evolve_channel":"inject_applied","corr_id":"…"}`.
- The poll loop already detects idle (prompt marker visible). When, after an `inject_applied`,
  the marker returns (busy→idle), print `{"evolve_channel":"idle_reached","corr_id":"…"}`.
- Everything else on the send/inject path is unchanged.

### 4. Supervisor (new Go API + thin default policy)

- Interface:
  ```
  type Supervisor interface {
      // Ask injects a correlated question and returns the agent's answer span content.
      Ask(ctx, question string) (Answer, error)   // ErrTransportNoInject for headless
      Feed() <-chan Envelope                       // live tail of the channel feed
  }
  ```
- `Ask` = `inbox.Append(ws, agent, Envelope{Kind: command, Body: question, CorrID: newID()})`
  (reuses the existing send path), then waits on the feed for `response_complete{corr_id}` (or
  `response_timeout`) and returns the spanned content envelopes as the answer.
- **Default policy** (minimal, pluggable): on an observer `stall`/liveness gap of N seconds,
  inject "Summarize progress so far and any blockers in 3 bullets." This is an example
  consumer; a smarter policy plugs into the same interface later.
- Attach is opt-in: orchestrator wires a supervisor when `EVOLVE_CHANNEL_SUPERVISOR=1`.

### 5. `evolve bridge watch` — read-only human debug tail

`evolve bridge watch --workspace DIR --agent NAME [--raw]` tails `<agent>-channel.ndjson` and
pretty-prints the filtered feed (correlation markers inlined). Read-only; symmetric with
`evolve bridge send`. For debugging only — humans do not control the CLI through it.

## Data flow (correlation lifecycle)

```
supervisor.Ask("summarize progress + blockers")
  └─ inbox.Append(ws, agent, {kind:command, body, corr_id:C1})        # existing send path
driver poll loop drains inbox → injectEnvelope(C1)
  ├─ idle-gate (wait for prompt marker), paste body + Enter           # existing
  └─ stderr breadcrumb {evolve_channel:inject_applied, corr_id:C1}    # NEW
observer/normalizer (SOLE writer of feed):
  ├─ tails stdout → filtered content envelopes seq N, N+1 …           # Normalizer.Poll
  ├─ sees inject_applied → feed {correlation/request, corr_id:C1, at_seq:N}
  ├─ agent answers → content envelopes seq N+1 … M
  └─ driver prints idle_reached{C1} → feed {correlation/response_complete, corr_id:C1, span:[N+1,M]}
supervisor tails feed → matches response_complete{C1} → reads content[N+1..M] = answer → react/ask again
human: evolve bridge watch → sees the whole live stream, read-only
```

## Error handling / edge cases

- **Agent never idles after an ask**: delivery bounded by existing `maxInjectDefer`; reply
  bracket bounded by the per-`corr_id` timeout → `response_timeout` (supervisor never blocks).
  Phase-level stall→kill stays independent.
- **Headless drivers** (`claude -p`): no mid-turn input → read-only monitoring; `Ask` returns
  `ErrTransportNoInject`. The live content feed still works via the normalizer.
- **ADR-0036 liveness**: the raw liveness floor is untouched; the content feed is added
  alongside and never filters the liveness path (guards the false-stall fix).
- **Crash safety / single-writer**: feed append-only, observer sole writer, valid prefix on
  crash; supervisor resumes by seq cursor. No file has two writers.
- **Teardown**: no new process; normalizer is extra work in the observer's existing goroutine
  (lifecycle already tied to the phase). Feed cleaned with the workspace.
- **Opt-in**: `EVOLVE_CHANNEL=1` gates the whole feature (off → byte-identical to today);
  `EVOLVE_CHANNEL_SUPERVISOR=1` gates supervisor attach.

## Testing

**Coverage gate: ≥95% statement coverage on every new/changed package**, verified per package
with `go test -cover ./internal/bridge/channel/... ./internal/phasestream/... …` and enforced
in CI for the touched packages (not just the repo-wide 80% floor). Each PR in the build order
reports `coverage: N% (≥95% required)` in its description; a package under 95% blocks merge.
TDD per the repo norm (RED→GREEN→cover). Reuses existing fakes (`deps.Tmux`, `deps.Runner`,
injected `Now`) so every branch is deterministically reachable.

What the 95% must actually exercise (behavioral, not line-padding):
- **Unit**: breadcrumb line → `correlation` envelope + span computation; driver emits
  `inject_applied`/`idle_reached` for a `CorrID` envelope (fake tmux toggling marker
  visibility); `response_timeout` on the injected clock; `CorrID` round-trips through the
  inbox.
- **Branch/edge** (the lines coverage tools miss without intent): headless `Ask` →
  `ErrTransportNoInject`; agent-never-idles → `response_timeout`; inbox re-queue hits
  `maxInjectDefer`; feed crash-prefix resume by seq cursor; `EVOLVE_CHANNEL=0` →
  byte-identical-to-today (producer is a no-op); malformed/duplicate breadcrumb tolerated.
- **Integration**: `supervisor.Ask` → inbox → fake driver applies+idles → observer writes
  feed → supervisor reads the response span. Deterministic.
- **Golden**: `evolve bridge watch` renders a fixture feed to expected lines.
- **Invariant guard**: a test asserting the observer's raw liveness floor is unchanged when
  the channel is on (no regression of the ADR-0036 false-stall fix) — single-writer asserted
  by a concurrent-append race test (`go test -race`).

## Files (anticipated)

- New: `go/internal/bridge/channel/` (supervisor + Ask/Feed), `go/cmd/evolve/cmd_bridge_watch.go`
- Edit: `go/internal/bridge/inbox/…` (add `CorrID`), `go/internal/bridge/driver_tmux_repl.go`
  (breadcrumbs), the **autospawn observer** (`go/internal/adapters/observer/` — the in-process
  producer wired by ADR-0030's `core.WithObserver`; `phaseobserver/` is the separate-process
  variant and is out of scope), `go/internal/phasestream/classify.go` (breadcrumb rule)
- Graduates to **ADR-0037** during implementation.

## Non-goals

- No headless duplex (`claude --input-format stream-json`) — deferred transport.
- No smart/LLM supervisor brain — deferred against the `Supervisor` interface.
- No streaming server/SSE endpoint — file-based only.
- No change to the post-phase `events.ndjson` path or the observer's liveness floor.
