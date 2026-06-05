# Bidirectional Channel — Root-Cause Analysis & Corrected Architecture

Date: 2026-06-04 · Supersedes the transport assumption in [the design](2026-06-04-bidirectional-channel-design.md) and ADR-0037 · Status: ANALYSIS (awaiting go-ahead)

> The components shipped on `feat/bidirectional-channel-design` are individually correct and ≥95%-covered, but the **tmux ask↔answer loop is non-functional on the real path**. This document is the verified root cause and the corrected architecture. The fix concentrates in the driver; the channel package (Producer/Supervisor/correlator/watch/policy) is reused as-is.

## The break (verified against code)

A live data-flow map across every launch path (in-process engine, `evolve bridge` CLI, `evolve loop` orchestrator, `evolve subagent run`) confirms **two independent breaks, one root mistake**.

| Sink | Path | HEADLESS (claude-p) | TMUX (claude-tmux) | Carries breadcrumbs? |
|---|---|---|---|---|
| `cfg.StdoutLog` (`<phase>-stdout.log`) | inner-CLI fd / at-exit dump | **LIVE** (stream-json streams to the fd) | **AT-EXIT ONLY** (`os.WriteFile` overwrite of a `capture-pane` dump, `driver_tmux_repl.go:390-391`) | No |
| `cfg.StderrLog` (`<phase>-stderr.log`) | same | LIVE | AT-EXIT ONLY | No |
| `deps.Stderr` (where breadcrumbs go) | driver diagnostic stream | in-mem `bytes.Buffer`→`BridgeResponse.Stderr` (`engine.go:311`) / `os.Stderr` / `io.Discard` | same | **Yes — but never on disk** |
| Producer reads | `<phase>-stdout.log` + `<phase>-stderr.log` (`producer.go:61-62`) | sees stdout live ✓ | **sees nothing until exit ✗** | **sees none ✗** |

- **Break 1 — breadcrumbs target a non-persisted sink.** `emitChannelBreadcrumb` writes to `deps.Stderr` (`driver_tmux_repl.go:309,486`), which `engine.go:58` documents as the driver's *diagnostic* stream — "NOT the log files." In every production path it is an in-memory buffer, the terminal, or discard. It never reaches `<phase>-stderr.log`, which is what the Producer's correlator tails. **The write side and read side are different sinks.**
- **Break 2 — tmux content is at-exit only.** For headless drivers the inner CLI writes its fd continuously, so `<phase>-stdout.log` is live (stream-json). For tmux drivers the logs are written **once at clean exit** as a `capture-pane` scrollback dump. During the run they are empty, so the Producer live-tailing them sees nothing until the phase is over — too late for a mid-task ask.

**The rigged e2e test masked both:** `e2e_test.go:85,94` hand-writes breadcrumbs + content directly into the files the Producer reads — a wiring no real driver performs. Green, but it never exercised the driver→file seam.

## Why (the architectural mistake)

The design generalized the **headless** model ("the CLI streams to a log file; tail it") to **tmux**, where it does not hold: during a tmux run the agent's live output exists **only inside the tmux pane** (a rendered viewport in tmux's memory), materialized to a file only at teardown. The driver is the *sole* component with a handle to that pane. The "Producer reads a live log file" model is simply not connected to that pane for tmux — and breadcrumbs were sent to a diagnostic stream rather than a persisted file. (My own ADR-0036 investigation had already recorded "tmux writes stdout.log only at completion" — I failed to carry it into the channel design.)

## The corrected architecture — `tmux pipe-pane` as the live source

`tmux pipe-pane -o 'cat >> <file>'` is tmux's purpose-built primitive: it streams **all** pane output to a file **as it is produced** — no polling, no fragile viewport-delta extraction. Validated viable here: all three REPL CLIs render to the **normal pane (no alt-screen)** (`knowledge-base/research/tmux-repl-cli-behavior-2026-05-26.md:33-35`), so the stream linearizes into a transcript (the same content the at-exit `capture-pane` dump already produces successfully for `phasestream.Produce`).

```
tmux pane (live)
  └─ pipe-pane -o 'cat >> <phase>-pane.live'         ← NEW: driver streams pane live
driver poll loop:
  └─ append breadcrumbs to <phase>-breadcrumbs.live  ← FIX: breadcrumbs to a FILE, not deps.Stderr
channel.Producer (UNCHANGED logic; pointed at the live files):
  └─ Normalizer tails pane.live (StdoutPath) + breadcrumbs.live (StderrPath)
       ├─ NEW pre-pass: CR-collapse + ANSI-strip on raw-pane mode  ← the real engineering
       ├─ plaintext noise filter (spinners/borders/dedup — already exists)
       └─ correlator brackets request/response_complete spans (already exists)
  → <agent>-channel.ndjson feed
Supervisor.Ask / bridge watch (UNCHANGED)
```

### Changes (scoped; most of the built feature is reused)

1. **`TmuxController.PipePane(ctx, session, shellCmd)`** — new interface method + `execTmux` impl (drop-in, mirrors `KillSession`, `tmux.go`) + a no-op in the test fake.
2. **Driver wiring** (`driver_tmux_repl.go`): after the boot prompt marker is confirmed (gate skips TUI boot chrome), start `pipe-pane` → a NEW dedicated `<phase>-pane.live` file; `defer` pipe-pane-off at exit. Write breadcrumbs to a NEW `<phase>-breadcrumbs.live` file (append) instead of `deps.Stderr`. **Leave the at-exit `os.WriteFile` dump untouched** (separate file → no truncation clobber, single-writer preserved).
3. **Producer source paths** (`producer.go` / the `core_adapter` spawn site): thread the live-file paths in. Transport-aware: tmux → the `.live` pair; headless → keep `<phase>-stdout.log` (already live). Producer logic otherwise unchanged.
4. **Raw-pane filter pre-pass** (the substantive work, in `phasestream`): a `RawPane` tailing mode that, before `classifyPlain`, (a) splits on `\r` keeping the last segment (terminal last-write-wins → an in-place spinner collapses to its final frame instead of one ever-growing line), and (b) `stripANSI`s each segment (reuse `tmux.go:103`, extend for bare control chars). **The stream-json / JSON path stays byte-identical** — only raw-pane mode gets this. This is ADR-0036's content-channel filtering applied at the live source.
5. **`collectSpan` 64KB fix** (`supervisor.go:248`): the default `bufio.Scanner` silently drops answer lines >64KB; size the buffer (like `produce.go:25` 16MB) or use `ReadBytes`.
6. **Real e2e** replacing the rigged one: drive the actual driver via the `PipePane` seam (fake tmux emitting pane output + breadcrumbs through the real code path) → live files → Producer → Supervisor. Assert the answer is recovered without hand-writing the Producer's inputs.
7. **ADR-0037 honesty**: downgrade its status to reflect the defect + this correction until 1–6 land and the real e2e is green.

### Risks / assumptions to validate during implementation
- **No alt-screen** holds for current CLI versions (verified, but version-sensitive — the research notes stale "alt-screen" comments). If a future CLI uses alt-screen, `capture-pane`-scrollback already breaks too, so this isn't a new fragility.
- **Live spinner volume**: the live stream has MORE transient frames than the at-exit snapshot (which only captures each line's final state). The CR-collapse pre-pass is what tames this; validate retention on a real capture.
- **pipe-pane availability**: standard in tmux ≥1.6; gate gracefully (best-effort, like `KillSession`) so a tmux without it degrades to no-live-feed rather than failing the phase.

### Consistency
- **ADR-0036 two-channel model:** the pane→pre-pass→Normalizer path IS the content channel (filtered); the observer's raw byte-growth/pane-hash **liveness** floor is untouched. Consistent.
- **Single-writer:** the driver owns the new `.live` files; the Producer is sole writer of the feed; supervisor/`bridge send` write only the inbox; `watch` is read-only. No file has two writers.

### Alternative considered (and why pipe-pane wins)
- **Driver-brackets-the-answer:** since the driver knows the inject→idle window and CapturePane's each tick, it could diff baseline-vs-idle panes and write the answer directly. Decouples the ask path from continuous streaming, but duplicates content-extraction in the driver and diverges from the single-filter Normalizer model — and gives no continuous monitor feed. `pipe-pane` serves both monitor + ask uniformly through the existing filter. Rejected as primary; viable fallback if pipe-pane linearization disappoints.
- **Poll-and-delta the pane** (driver appends `capture-pane` deltas each tick): fragile viewport-delta extraction on a scrolling buffer; `pipe-pane` is the native, delta-free stream. Rejected.

## Pre-existing land mine to flag (separate)
`subagent/run.go:273-274` uses the **dot** form `<agent>.stdout.log` while the engine/Producer use the **hyphen** form `<agent>-stdout.log`. Not on the channel's active path, but any new live-file convention must use the hyphen form, and the dot/hyphen split is worth a separate cleanup.
