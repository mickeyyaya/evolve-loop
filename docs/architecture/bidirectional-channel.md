# Bidirectional channel — full design & implementation guide

> Companion to the decision record [ADR-0037](adr/0037-bidirectional-channel.md). The ADR
> records *what was decided*; this guide records the *request, the requirements, every
> approach considered (including the two that were tried and reverted), and the shipped
> solution end-to-end* — so a future reader can follow the whole arc, not just the verdict.
>
> Status: shipped on `feat/bidirectional-channel-design` (RT0–RT6 + a post-review hardening
> commit), opt-in via `EVOLVE_CHANNEL=1`, not yet merged to `main`.

---

## 1. The request / requirement

> *"I want to build a bidirectional communication channel so we can continue to monitor and
> ask the LLM CLI for longer tasks and keep getting the useful feedback."* — and, on the
> protocol: *"the agent and orchestrator should not read directly from stdout output without
> a filter applied (to filter out the noise and useless redraw)."*

Concretely, two unmet needs for phases that run for minutes under a tmux-REPL driver
(`claude-tmux`, `codex-tmux`, `agy-tmux`, `ollama-tmux`):

1. **Continuous, filtered monitoring** — a live, noise-filtered view of what the agent is
   producing *while the phase is still running* (not just the post-phase `events.ndjson`).
2. **Correlated mid-task asks** — the host can inject a question into a running REPL
   (`evolve bridge send` already existed) but had no way to know *which* agent output is the
   *answer* to *which* injected question. We need request→response bracketing.

### Requirements / invariants

- **No noise.** The feed must be the filtered content stream, never the raw redraw/spinner
  churn. (Drives the per-CLI delta extraction + the `phasestream` classifier.)
- **No agent cooperation.** Bracketing must be driver-owned — no changes to agent prompts or
  personas (they must not have to emit special markers).
- **Single-writer discipline.** Each file has exactly one writer (sequential-write invariant).
- **Opt-in, byte-identical off.** `EVOLVE_CHANNEL=1` enables everything; off must be
  byte-for-byte identical to before (no files, no goroutine, no extra `capture-pane`).
- **Headless degrades gracefully.** `claude -p` cannot receive mid-turn input → read-only
  monitoring; `Supervisor.Ask` returns `ErrTransportNoInject`.
- **TDD + ≥95% coverage per changed package; spec + quality review per task.**

---

## 2. Approaches considered (including the dead ends)

The headline lesson: **two plausible designs were disproven by capturing real terminal
output**, and the real fixtures caught bugs a fabricated test never would. The captures are
committed at `knowledge-base/research/tmux-live-capture-2026-06-04/` (and mirrored as test
fixtures under `go/internal/bridge/panestream/testdata/<cli>/{thinking,answer,final}.txt`).

### 2.1 Live content source — three candidates

| Approach | Verdict | Why |
|---|---|---|
| **Tail `<phase>-stdout.log`** (the original ADR assumption) | ✗ rejected | A tmux driver streams to the tmux *pane*; its `stdout.log` is **empty until the at-exit dump**. Tailing it yields nothing live. |
| **`tmux pipe-pane`** (first fix attempt, FT1) | ✗ tried & reverted (RT0) | A real capture proved `pipe-pane`'s raw stream is 2D cursor motion (`\e[2C\e[3A…`, absolute column moves, CRs, non-UTF8). Only a full terminal emulator can linearize it; a `stripANSI`+CR-collapse filter jams lines into garbage (`Accessing\e[12Gworkspace:` → `Accessingworkspace:`). |
| **Poll `tmux capture-pane` + emit stabilized deltas** | ✓ **chosen** | `capture-pane -p` renders the pane to **clean text** (tmux *is* the emulator). The driver already polls it every tick. We diff successive rendered snapshots and emit only newly-stable content above the volatile bottom UI. No emulator, no raw-ANSI filter, reuses tmux's rendering. |

### 2.2 Delta extraction — the two bugs real frames exposed

A naive "emit lines below a fixed cursor index" extractor fails on real frames:

- **Volatile-zone-same-height bug:** between the echoed prompt and the empty input box sits a
  volatile zone (spinner + separators) whose height is identical in the thinking and answer
  frames, so an index cursor primed on the thinking frame lands on the spinner and **skips the
  answer**. → Fix: trim a trailing run of volatile rows (`isVolatileTailRow`) so the stable
  region ends at the last real content line in *both* frames.
- **Top-of-buffer-shift bug:** when older scrollback/preludes prepend, every content line
  keeps its text but moves index — a positional cursor re-emits the last bullet. → Fix: a
  **content-anchored** cursor that re-locates the last-emitted line by text each frame.

Both are encoded in `panestream.PaneDelta` and table-tested against all four CLIs' real frames.

### 2.3 Busy/idle detection — marker-presence vs the real signal

The original correlation bracketed the answer span on a **busy→idle transition**, detecting
"busy" as **the prompt marker being absent**. The real frames disproved this:

- The input-box marker (`❯`, `›`, `>`) **persists during generation** for claude/agy (and
  ollama echoes `>>>` on the prompt line), so "idle = marker present" is *always* idle →
  `idle_reached` **never fired** → correlation never functioned.
- The actual busy signal is **per-CLI and bimodal**:
  - claude/agy: the interrupt/cancel affordance (`esc to interrupt` / `esc to cancel`) shown
    for the whole interruptible turn. (The spinner words `Inferring`/`Generating` were
    considered but rejected — redundant with the affordance, and as bare words they could
    false-match answer prose.)
  - ollama: no affordance; its `Thinking…` header *persists into the answer*, so that is not a
    signal either. The real distinction is the idle input **placeholder** `Send a message`,
    which is absent mid-turn. → `PaneProfile.IdlePlaceholder`.
  - codex: the captured frames carry *no* busy affordance and no placeholder → **documented
    weak-signal degradation** (monitoring works; its span cannot be bracketed).

→ `panestream.PaneBusy(rendered, profile)`: busy = affordance present **OR** idle placeholder
absent. Validated against every `testdata/<cli>/{thinking,answer,final}.txt`.

### 2.4 Breadcrumb sink — stderr vs a file

The original wrote correlation breadcrumbs to `deps.Stderr` — an **in-memory stream the
Producer never read**, so correlation was dead on arrival. → Breadcrumbs are appended to a
`<agent>-breadcrumbs.live` file (O_APPEND) that the Producer tails as its `StderrPath`.

### 2.5 Producer source selection — transport-aware

Because tmux content lives in `pane.live` (stdout.log empty until exit) but headless content
lives in `stdout.log` (no pane.live exists), pointing the Producer at the wrong source
*silently* empties the feed. → `CoreAdapter.channelSourcePaths` resolves the per-phase CLI
family and selects the `.live` pair (tmux) or the legacy logs (headless). Empty paths on
`ProducerConfig` fall back to the legacy defaults, keeping headless byte-identical.

---

## 3. The shipped solution — end-to-end data path

```
Supervisor.Ask(question)                     [go/internal/bridge/channel/supervisor.go]
  └─ inbox.Append({CorrID})                  [go/internal/bridge/inbox]
       └─ driver drains inbox, idle-gated inject  [driver_tmux_repl.go: injectEnvelope]
            ├─ writes "inject_applied" → <agent>-breadcrumbs.live
            ├─ per tick: capture-pane → PaneDelta.Next(rendered, profile)
            │            → append new content → <agent>-pane.live
            └─ busy→idle via PaneBusy(pane, profile)
                 └─ writes "idle_reached" → <agent>-breadcrumbs.live
  Producer (observer goroutine)              [channel/producer.go + adapters/observer/core_adapter.go]
    └─ Normalizer tails StdoutPath=<agent>-pane.live (content via classifier.Line)
                     + StderrPath=<agent>-breadcrumbs.live (correlation via classifier.Stderr)
       └─ writes filtered envelopes → <agent>-channel.ndjson  (the feed; sole writer)
  Supervisor.awaitReply
    └─ sees response_complete{start,end} → collectSpan(feed, start, end) → Answer
```

### Per-CLI `PaneProfile` (in `panestream.Profiles`)

| CLI | BoundaryMarker | BoundaryExact | IdlePlaceholder | busy signal |
|---|---|---|---|---|
| claude | `❯` | – | – | `esc to interrupt` |
| codex | `›` | – | – | *(none captured → weak-signal degradation)* |
| agy | `>` | ✓ (blockquote `>` vs empty box) | – | `esc to cancel` |
| ollama | `>>>` | – | `Send a message` | idle placeholder absent |

### Files written (when `EVOLVE_CHANNEL=1`)

| File | Writer | Contents |
|---|---|---|
| `<agent>-pane.live` | tmux driver | newly-stable rendered content (PaneDelta deltas) |
| `<agent>-breadcrumbs.live` | tmux driver | `{"evolve_channel":"inject_applied"\|"idle_reached","corr_id":…}` |
| `<agent>-channel.ndjson` | `channel.Producer` (sole writer) | filtered content + `KindCorrelation` envelopes |

Off (`EVOLVE_CHANNEL` unset): none of these exist; no producer goroutine; no extra capture.

### Resilience

- **RT4 — large-answer recovery:** `collectSpan`'s `bufio.Scanner` uses a 1 MB/10 MB buffer
  (project convention) so a >64 KB answer line is not silently dropped; `scanner.Err()` is
  surfaced as a WARN.
- **Silent-empty-feed guard (post-review):** if the content source file never appears after
  `sourceMissThreshold` (20) polls (~40 s at the 2 s prod poll), the Producer emits a one-time
  WARN — converting the two "silent empty feed" failure modes (agent≠phase name, mis-resolved
  CLI family) into a loud signal.

---

## 4. Operations

| Env var | Default | Effect |
|---|---|---|
| `EVOLVE_CHANNEL` | `0` | `1` enables the producer + live feed + `.live` files. |
| `EVOLVE_CHANNEL_SUPERVISOR` | `0` | Reserved: auto-attach a `Supervisor` on phase launch (manual wiring only today). |
| `EVOLVE_<AGENT>_CLI` / `EVOLVE_CLI` | unset → `claude-tmux` | Resolves the per-phase CLI family for transport-aware source selection. |

- **Human debug tail:** `evolve bridge watch --workspace DIR --agent NAME [--follow]` (read-only).
- **Inject:** `evolve bridge send …` (existing, ADR-0023) — the uncorrelated counterpart.

---

## 5. Testing

- `panestream`: `PaneDelta` + `PaneBusy` table-tested against the four CLIs' real frames (100% cov).
- `bridge`: driver writes `pane.live`/`breadcrumbs.live`, byte-identical off, WARN on open error.
- `channel`: producer source-path override, >64 KB span recovery, silent-feed WARN.
- **Full real-path e2e** (`bridge/channel_e2e_test.go`): `Supervisor.Ask` → inbox → driver →
  real claude frames through `PaneDelta` → `.live` files → real `Producer` → `Supervisor`
  recovers the bracketed answer span. Self-paced fake tmux keyed on `breadcrumbs.live`; passes
  `-race -count=20`.
- Coverage sweep ≥95% all changed packages (panestream 100%).

---

## 6. Known limitations & follow-ups (non-blocking)

1. **codex weak-signal:** no capturable busy affordance in the fixtures → codex spans are not
   bracketed (monitoring still works). Revisit if codex exposes an interrupt affordance.
2. **profile.cli-pin family mis-read:** `phaseCLI` resolves family from env only; a CLI pinned
   in `profile.json` (not env) is mis-read as tmux. Now *loud* (silent-feed WARN fires) but not
   *correct* — full fix is to surface the resolved CLI to the observer spawn site.
3. **double `capture-pane`/tick** when a correlation span is open (delta capture + idle-bracket
   capture). Channel-on only; reuse the delta `rendered` for the idle check to save one IPC.
4. **e2e boot-sync** uses a fixed 20 ms sleep before the supervisor injects; a `driver_ready`
   breadcrumb would remove the timing assumption.
5. **`sourceMissThreshold`** is a constant (20 polls); expose on `ProducerConfig` if a phase
   has an unusually slow boot.

---

## 7. References

- [ADR-0037](adr/0037-bidirectional-channel.md) — the decision record (with the RT0–RT5
  implementation-correction addendum).
- [ADR-0036](adr/0036-content-vs-liveness-channel-protocol.md) — content vs liveness channels.
- [ADR-0023](adr/0023-live-injection-and-launch-rules.md) — the inbox / `evolve bridge send`.
- [ADR-0030](adr/0030-phase-observer-autospawn-in-evolve-loop.md) — the observer the producer rides on.
- `knowledge-base/research/tmux-live-capture-2026-06-04/NOTES.md` — the real-capture evidence
  that drove the `pipe-pane`→`capture-pane` pivot and the busy/idle redesign.
- Design/FIX specs: `docs/superpowers/specs/2026-06-04-bidirectional-channel-{design,FIX}.md`.
