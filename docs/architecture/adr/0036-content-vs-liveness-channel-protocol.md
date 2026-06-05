# ADR 0036 — Content vs liveness channel protocol (the agent ⇄ LLM-CLI-driver read contract)

Status: PROPOSED (2026-06-04) · Closes the loop ADR-0020 opened (logfilter's residual machine role; phaseobserver's raw read) · No code in this ADR — design target + per-CLI intake recommendation

> **TL;DR.** An agent or the orchestrator must never consume *raw* LLM-CLI stdout as
> **content**. Raw stdout carries two noise profiles — stream-json `stream_event` redraw
> deltas (~86% of a headless log) and tmux-scrollback rendering (spinners, box borders,
> `Incubating… (12m · ↑54k tokens)` live redraws). The fix is **two channels, not "filter
> everything"**: a fully-filtered **content** surface (`<phase>-events.ndjson`) that every
> content consumer reads, and an intentionally **raw liveness** surface (stdout byte-growth +
> tmux pane-animation hash) used *only* by stall detection. The content filter already exists
> (`internal/phasestream`) and is broadly consumed; this ADR records the protocol, names the
> three residual raw-read leaks, and sets the consolidation target.

## Context

ADR-0020 ("Unified phase event stream") established `internal/phasestream` as the single
normalizer that turns a phase's raw `<phase>-stdout.log` into `<phase>-events.ndjson`, the
unified envelope stream. It works and is broadly consumed (orchestrator, observer,
cycleclassify, cyclecost, phasewatchdog). `phasestream/classify.go` already strips **both**
noise profiles:

- **stream-json deltas** → dropped as noise and coalesced into one progress tick
  (`classify.go:66`, `classify.go:134` `case "stream_event"`, `FlushProgress` at
  `classify.go:112`).
- **tmux-scrollback plaintext** → spinner/border/blank-line detection ported verbatim from
  the older `logfilter` (`classify.go:349` "noise detection (ported from
  logfilter/plaintext.go)", `isSpinnerRune` at `classify.go:371`; truncation helpers at
  `classify.go:389`).

Wired post-phase at `go/internal/phases/runner/runner.go:171` (`phasestream.Produce`).

But ADR-0020 left the picture **half-built**, and its own Context table flagged the two open
ends this ADR now closes:

1. **Two filters, not one.** The older `internal/logfilter` still runs in parallel
   (`runner.go:166`, default `logfilter.Process`) and writes a *second* rendering,
   `<phase>-stdout.clean.txt` — gated by `EVOLVE_STDOUT_FILTER` (`runner.go:489`). ADR-0020
   said `clean.txt` is "read by **nobody** machine"; that is now false in exactly one place
   (leak #1 below), which makes it a load-bearing duplicate of `events.ndjson`.
2. **phaseobserver reads raw.** ADR-0020 assumed the observer was dormant in the Go path; the
   ADR-0030 autospawn (cycle-122) re-armed it. It is live again and tails raw stdout (leak #2).

### The orchestrator is already mostly correct

The deliverable is extracted from the **artifact file**, not from stdout
(`orchestrator.go:394` `os.ReadFile(artPath)`; SHA-bound at `:417`). Completion is detected by
the artifact file appearing / pane-idle / git-trailer (the ADR-0027 Strategy contracts) — the
driver does **not** parse stdout for a content sentinel. So the worst failure mode (scraping
the answer out of a rendered terminal) is already avoided on the main path.

### The three residual raw-read leaks (the real gap)

| # | Reader | Reads | Where | Channel it *should* use |
|---|---|---|---|---|
| 1 | `backfill.TryExtract` | orphan `<phase>-stdout.clean.txt` (2nd filter) | `go/internal/backfill/backfill.go:44` | content → `events.ndjson` |
| 2 | `phaseobserver.tail()` | raw `<agent>-stdout.log` (3rd ad-hoc stream-json parser) | `go/internal/phaseobserver/phaseobserver.go:197,241,331` | content-signal → `events.ndjson`; keep raw byte-delta as liveness floor |
| 3 | tmux stop-review | raw pane `lastLines(curPane,40)` → `StdoutTail` → reviewer + escalation report | `go/internal/bridge/driver_tmux_repl.go:306,338`; pane written ANSI-stripped-only to `stdout.log` at `:346` | **liveness** (legitimately raw — see below) |

Leaks #1 and #2 are genuine content reads of an unfiltered/duplicate surface. Leak #3 is
*correctly* raw — it is a liveness read — but it is currently undistinguished from a content
read, which is what lets a future change "helpfully" filter it and break things.

### The critical tension — why "filter `thinking…` everywhere" is the wrong frame

The `Incubating… (12m · ↑54k tokens)` spinner and the per-second pane redraw are pure
**noise for content**. But during a long extended-thinking turn they are the **only liveness
signal that exists**: a `claude-tmux`/`codex-tmux`/`agy-tmux` agent in one long "Incubating"
turn commits no scrollback lines to `stdout.log` and writes no workspace artifact until the
turn ends — both filesystem signals stay flat for minutes, indistinguishable from a hang. The
fix in `knowledge-base/research/observer-false-stall-tmux-liveness-2026-06-02.md` added a tmux
`capture-pane` animation-hash probe **precisely because** the spinner advancing each second is
the proof-of-life. Filtering the spinner out of the **liveness** path would reintroduce that
false-stall bug.

Therefore the protocol is not "one filter applied everywhere." It is **two channels with
opposite requirements**: content must be maximally filtered; liveness must stay raw.

## Decision — the two-channel protocol

### Channel A — Content (`<phase>-events.ndjson`) — the single canonical content surface

- **Rule:** anything that needs *what the agent said or produced* reads `events.ndjson`.
  Never raw `<phase>-stdout.log`. Never `<phase>-stdout.clean.txt`.
- Fully noise-stripped by `phasestream`: stream-json `stream_event` deltas dropped + coalesced
  into a single progress tick; tmux spinners/borders/blank-lines/dedup dropped;
  `tool_result`/large payloads middle-truncated; thinking-signature blobs stripped.
  Interactive actions (`AskUserQuestion`, `ExitPlanMode`, permission prompts) survive at full
  fidelity — they are signal, never noise (carried over from ADR-0020).
- The phase **deliverable** stays a file (`scout-report.md`, …) read directly and SHA-bound —
  `events.ndjson` is the *secondary* content surface used when the file is missing
  (backfill), for classification, and for cost.

### Channel B — Liveness — intentionally raw, stall-detection only

- **Rule:** stall/liveness detection reads the raw signal — `stdout.log` byte-growth and the
  tmux pane-animation hash — and **must not be filtered**. The spinner redraw *is* the signal.
- Consumers: `phaseobserver` (byte-growth + pane-hash liveness floor), the tmux stop-reviewer
  (`PaneHasSubstantiveChange` / `StdoutTail`).
- This channel is **never** used to derive content. Its outputs are liveness verdicts
  (extend / pause / stall-kill), not the agent's answer.

### Consumer → channel map (the contract)

| Consumer | Needs | Channel | Reads | Today | Target |
|---|---|---|---|---|---|
| orchestrator (deliverable) | content | file | artifact file | ✅ correct | unchanged |
| orchestrator (backfill fallback) | content | A | `clean.txt` ❌ | leak #1 | `events.ndjson` |
| cyclecost | content | A | `events.ndjson` | ✅ | unchanged |
| cycleclassify | content | A | `events.ndjson` | ✅ | unchanged |
| phaseobserver (content counts) | content | A | raw `stdout.log` ❌ | leak #2 | `events.ndjson` |
| phaseobserver (liveness floor) | liveness | B | raw byte-growth + pane-hash | ✅ correct | unchanged (stays raw) |
| tmux stop-review | liveness | B | raw pane | ✅ correct | unchanged (stays raw, *labelled* liveness) |

## Single-content-surface consolidation (design target — sequenced follow-ups, not this ADR)

Each is a separate future PR with its own tests. Listed in dependency order; risk noted.

1. **Backfill → `events.ndjson`.** Reconstruct the missing artifact from the `assistant` /
   `result` envelopes in `events.ndjson` instead of grepping `clean.txt` for a phase header
   (`backfill.go:38-65`). Removes the orphan filter's only machine consumer. *Risk: low* —
   one function, well-tested; the header-marker heuristic maps cleanly onto envelope `type`.
2. **Retire `logfilter` as a separate filter.** Once #1 lands, `clean.txt` has no machine
   reader. Either drop it, or re-render it as a thin **human** projection *from*
   `events.ndjson` (one filter, two renderings). Default `runner.go:166` flips from
   `logfilter.Process` to the projection (or no-op). *Risk: low–medium* — operators who read
   `clean.txt` by hand must get an equivalent human view; keep `EVOLVE_STDOUT_FILTER` as the
   on/off dial.
3. **phaseobserver content-signal from `events.ndjson`.** Take `tool_use` ticks, `result`
   cost/cache, rate-limit markers, and loop-SHA from the normalized stream; keep raw
   byte-delta + pane-hash strictly as the Channel-B liveness floor. Removes the third ad-hoc
   stream-json parser (`phaseobserver.go:331`). *Risk: medium* — the observer is live and
   load-bearing for stall-kill; the liveness floor must be provably untouched (the
   false-stall fix is the regression to guard).

Non-goal of all three: do **not** filter Channel B.

## Per-CLI intake recommendation — prefer the native structured channel over TUI scraping

A tmux pane is a *rendered TUI*; scraping it is inherently lossy (spinners, borders, redraws,
the think-then-dump gap). Treat pane capture as **liveness-only**. For **content**, prefer the
CLI's native machine channel, in this order of cleanliness:

- **claude** — headless already emits `--output-format stream-json --include-partial-messages
  --verbose` (`driver_claudep.go:46`), which `phasestream` normalizes. For phases that need
  only the *final* result (verdict/terminal phases), `--output-format json` yields a single
  clean result blob, and `--json-schema` adds a validated `structured_output` field — the
  cleanest possible extraction (no streaming noise to filter at all). **Recommend:** keep
  `stream-json` where live progress / liveness matters; offer `--output-format json` for
  result-only phases. (`--output-format json` requires `--print`/`-p`; `stream-json` requires
  `--verbose`.)
- **codex** — `codex exec` already writes the final message to a file via
  `--output-last-message <file>` (clean, no scraping). It also offers `codex exec --json`
  (JSONL events: `thread.started`, `turn.started/completed/failed`, `item.*`, `error`) —
  structured like claude's stream-json and normalizable by the same pipeline. **Recommend:**
  prefer the headless `codex exec --json` (or `--output-last-message`) content channel over
  `codex-tmux` pane scraping wherever the phase allows interactive features to be dropped.
- **agy (antigravity)** — weakest documented structured output; pane scraping may be the only
  channel. **Recommend:** where no structured channel exists, the captured pane MUST flow
  through `phasestream` before any content consumer — already true via `Produce`, *provided*
  the leak-fixes above land so nothing reads `stdout.log` raw for content.

The general rule: **structured channel for content; TUI pane for liveness; never the reverse.**

## Consequences

- **One content filter, not two.** `phasestream`/`events.ndjson` becomes the sole content
  surface; `logfilter`/`clean.txt` is demoted to a human projection or retired.
- **One content surface every consumer trusts.** No more split across raw `stdout.log`,
  orphan `clean.txt`, and `events.ndjson`.
- **Liveness explicitly carved out and protected.** Channel B is documented as raw-by-design,
  so a future "let's filter the noisy stdout" change cannot silently regress the false-stall
  fix.
- **A clear per-CLI intake preference** that pushes content toward each CLI's cleanest
  machine channel and confines TUI scraping to liveness.
- **Non-goals:** no driver invocation changes, no observer rewrite, no code in this ADR. The
  three consolidation workstreams are sequenced follow-ups, each separately reviewed and
  tested.

## References

- [ADR-0020 — Unified phase event stream](0020-unified-phase-event-stream.md) — predecessor;
  this ADR closes its two open ends (logfilter's machine role, phaseobserver's raw read).
- [ADR-0023 — Live injection and launch rules](0023-live-injection-and-launch-rules.md) —
  the tmux 2-second poll loop and launch sequencing.
- [ADR-0030 — Phase-observer autospawn in `evolve loop`](0030-phase-observer-autospawn-in-evolve-loop.md)
  — why the observer is live again in the Go path.
- [ADR-0027 — Completion contracts] (artifact / stdout / git Strategy) — the completion
  protocol the drivers use (see `go/internal/bridge/completion.go`).
- `knowledge-base/research/stdout-noise-profile-2026-05-26.md` — measured 7.6% retention
  (200 KB → 15 KB), the `stream_event` 86%-of-lines finding.
- `knowledge-base/research/observer-false-stall-tmux-liveness-2026-06-02.md` — the
  think-then-dump false-stall lesson; why Channel B must stay raw.
- `knowledge-base/research/tmux-repl-cli-behavior-2026-05-26.md` — boot markers, no
  alt-screen, capture timing.
- `docs/architecture/phase-observer.md`, `docs/architecture/observer-severity.md` — observer
  envelope + severity vocabulary.
- External (per-CLI structured output): claude `--output-format json|stream-json` +
  `--json-schema` (Anthropic Claude Code docs; anthropics/claude-code issues #24594, #24596);
  codex `exec --json` / `--output-last-message` (developers.openai.com/codex — CLI reference,
  non-interactive mode).
