<!-- Provenance: UI/observability pattern survey produced 2026-09-02 by a research subagent (WebSearch/WebFetch) for the `evolve dashboard` design (ADR-0095); every claim carries the URL that was fetched, UNVERIFIED items are labelled. -->

# Agent/pipeline observability UIs — what to borrow for the evolve-loop dashboard

Research date: 2026-09-02. Scope: open-source agent-trace UIs, coding-agent UIs, workflow/CI UIs, Go single-binary live dashboards, incident/failure presentation. ~45 primary pages fetched; every claim below carries its URL. Items I could not confirm are marked **UNVERIFIED**.

Cost scale used throughout: **S** < 1 day, **M** 1–3 days, **L** > 3 days (one engineer, Go + vanilla JS).

---

## 1. Executive summary

1. **A cycle is a trace; a task is a session.** Every mature tool separates the two: Langfuse *traces → observations*, with *sessions* grouping traces of one interaction ([data model](https://langfuse.com/docs/observability/data-model)); LangSmith *runs → trace → thread* ([concepts](https://docs.langchain.com/langsmith/observability-concepts)); Helicone `Helicone-Session-Id` + `Helicone-Session-Path` ([sessions](https://docs.helicone.ai/features/sessions)); Phoenix `session.id` ([setup](https://arize.com/docs/phoenix/tracing/how-to-tracing/setup-tracing/setup-sessions)). For us: `run.json`/cycle dir = trace, the inbox task id = session (groups retries, continuations, salvage cycles of one task).
2. **Lanes, not trees, for a ~10-phase linear pipeline.** Langfuse had to redesign its tree into a zoomable timeline to fit "twelve hundred" spans ([changelog](https://langfuse.com/changelog/2025-03-19-new-trace-view)); Airflow's Grid (rows = tasks, columns = runs, colored cells) is the debugging home ([UI docs](https://airflow.apache.org/docs/apache-airflow/stable/ui.html)). Our board should be a phase × cycle grid plus a per-cycle horizontal phase timeline from `phase-timing.json`. Trees only inside a phase (`*-interactions.ndjson`).
3. **Retries are immutable, numbered attempts — never overwrite.** Buildkite: "Previously, if you retried a job, you lost the history… now a new job is created" ([blog](https://buildkite.com/blog/job-retries)); GitHub `github.run_attempt` "begins at 1… increments with each re-run" ([contexts](https://docs.github.com/en/actions/reference/workflows-and-actions/contexts)); Temporal exposes `Attempt` + last failure on pending activities ([retry policies](https://docs.temporal.io/encyclopedia/retry-policies)). Our `audit-report.md.round1/.round2` archives are exactly this; show a round selector and a findings diff between rounds.
4. **"What went wrong" = Sentry issue header, not a log.** Group by fingerprint; show first seen / last seen / count / regressed ([grouping](https://docs.sentry.io/concepts/data-management/event-grouping/), [issues list](https://docs.sentry.io/product/issues/)). `disposition.json.fingerprint` joined over `ledger.jsonl` gives us this for free; add Dagger-style breadcrumbs from error back to root ([Dagger traces](https://dagger.io/blog/introducing-dagger-traces)).
5. **Separate state *type* from state *name*.** Prefect: "State types drive orchestration logic, whereas state names provide visual bookkeeping" (Retrying is name, RUNNING is type) ([states](https://docs.prefect.io/v3/concepts/states)); OpenHands has a closed AgentState enum ([backend](https://docs.openhands.dev/usage/architecture/backend)). Derive a 6-value state type from `cycle-state.json` + `acs-verdict.json` + `failure-decision.json`; keep phase names as labels.
6. **Severity levels on events, with a filter.** Langfuse `level` ∈ {DEBUG, DEFAULT, WARNING, ERROR} + `statusMessage`, and "you can filter the observations by log level" ([log levels](https://langfuse.com/docs/observability/features/log-levels)). Apply to `*-events.ndjson`.
7. **Reports first, transcript second.** Devin's Progress tab exists so users don't "parse full chat transcripts" ([fast.io](https://fast.io/resources/devin-session-tools-guide/)); Copilot links every commit to its session log ([sessions](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/cloud-agent/track-copilot-sessions)). Render `*-report.md` above the fold; `*-interactions.ndjson` behind a step viewer with j/k keys (SWE-agent inspector, OpenHands trajectory-visualizer).
8. **Aggregate counts above every list.** Prefect users had to "apply each state filter and keep track of the counts" for 1000s of task runs ([issue #12102](https://github.com/PrefectHQ/prefect/issues/12102)). Header tiles: running / queued / pass / fail / halt, ship-rate sparkline.
9. **SSE, one stream, named events, ids, keepalive.** ntfy (`/sse`, `since=`, `poll=1`) ([API](https://docs.ntfy.sh/subscribe/api/)), Dozzle (`event:`/`data:`/`:ping`, `X-Accel-Buffering: no`) ([sse.go](https://raw.githubusercontent.com/amir20/dozzle/master/internal/support/web/sse.go)), PocketBase realtime ("implemented via Server-Sent Events") ([docs](https://pocketbase.io/docs/api-realtime/)). WebSocket buys nothing for a one-way local feed.
10. **Read artifacts atomically; watch directories.** fsnotify: watching individual files is "generally not recommended as many programs… update files atomically" ([fsnotify](https://github.com/fsnotify/fsnotify)); renameio guarantees a concurrent open sees "either the old file or the just written file" ([renameio](https://pkg.go.dev/github.com/google/renameio/v2)); tail with `CompleteLines` for NDJSON ([nxadm/tail](https://pkg.go.dev/github.com/nxadm/tail)).

---

## 2. Pattern table

| Pattern | Seen in (project + URL) | What it shows the human | Maps to OUR artifacts | Cost |
|---|---|---|---|---|
| Trace → observations tree with tree/timeline toggle, color by type, in-trace search | Langfuse [data model](https://langfuse.com/docs/observability/data-model), [new trace view](https://langfuse.com/changelog/2025-03-19-new-trace-view), [timeline](https://langfuse.com/changelog/2024-06-12-timeline-view); MLflow span tree + Gantt ([tracing](https://mlflow.org/docs/latest/genai/tracing/)) | Where time/tokens went; parallelism; nesting | `phase-timing.json` → phase bars; `routing-decision-N.json` labels each bar (CLI/model/effort); `*-usage.json` sizes/annotates bars | S |
| Sessions group traces of one interaction (path-based hierarchy) | Langfuse sessions; Helicone `Helicone-Session-Id/-Path/-Name` ([sessions](https://docs.helicone.ai/features/sessions)); LangSmith threads via `thread_id` metadata ([concepts](https://docs.langchain.com/langsmith/observability-concepts)); Phoenix `session.id` ([setup](https://arize.com/docs/phoenix/tracing/how-to-tracing/setup-tracing/setup-sessions)) | All attempts at one task in order | Task id from `inbox/*.json` (consumed) + `run.json` → group cycles per task; salvage/continuation chain visible | S–M (needs stable task id in `run.json`) |
| Observation `level` + `statusMessage`, filter by level | Langfuse [log levels](https://langfuse.com/docs/observability/features/log-levels) | Only WARNING/ERROR when triaging | `*-events.ndjson` (add `level` if absent; default DEFAULT) | S |
| Span-kind taxonomy with color coding | OpenInference `openinference.span.kind` ∈ LLM, CHAIN, TOOL, AGENT, RETRIEVER, EVALUATOR… ([spec](https://github.com/Arize-ai/openinference/blob/main/spec/semantic_conventions.md)); Phoenix ([traces](https://arize.com/docs/phoenix/tracing/llm-traces)) | Kind of step at a glance | Phase archetype (scout / tdd / build / audit / ship / retro) → fixed palette | S |
| Standard usage attribute names | OTel GenAI `gen_ai.operation.name`, `gen_ai.provider.name`, `gen_ai.request.model`, `gen_ai.usage.input_tokens/output_tokens`, `gen_ai.agent.name`, `gen_ai.conversation.id`, `error.type` ([registry](https://opentelemetry.io/docs/specs/semconv/registry/attributes/gen-ai/), [agent spans](https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/gen-ai-agent-spans.md), status *Development*) | Consistent token/cost columns | Normalize `*-usage.json` + `routing-decision-N.json` into these field names in the dashboard's JSON API (no OTel export — YAGNI) | S |
| Closed agent-state enum | OpenHands `LOADING, RUNNING, AWAITING_USER_INPUT, FINISHED, ERROR, PAUSED, STOPPED, RATE_LIMITED` ([backend](https://docs.openhands.dev/usage/architecture/backend)); Prefect type vs name ([states](https://docs.prefect.io/v3/concepts/states)) | Unambiguous badge color | Derive `QUEUED / RUNNING / PASS / WARN / FAIL / HALTED` from `cycle-state.json.current_phase`, `acs-verdict.json`, `failure-decision.json.action` | S |
| Grid: rows = tasks, columns = runs, colored cells, tooltip, click → logs | Airflow Grid view ([UI](https://airflow.apache.org/docs/apache-airflow/stable/ui.html)) | Failed/retried phases across recent cycles at a glance | rows = phases, columns = last N cycles; cell = phase status from `cycle-state.json.completed_phases` + timing | M |
| Immutable retry attempts with attempt selector | Buildkite "retry selector… indicator for steps with retries" ([build page](https://buildkite.com/docs/pipelines/build-page), [blog](https://buildkite.com/blog/job-retries)); GitHub "Latest" attempt dropdown ([re-run](https://docs.github.com/en/actions/managing-workflow-runs-and-deployments/managing-workflow-runs/re-running-workflows-and-jobs)), `github.run_attempt` ([contexts](https://docs.github.com/en/actions/reference/workflows-and-actions/contexts)); Temporal `Attempt` + last failure ([retry](https://docs.temporal.io/encyclopedia/retry-policies)) | Did the fix loop converge? | `audit-report.md.round1`, `.round2`, `audit-report.md` (+ `cycle-state.json` round counter) → round tabs + findings diff | S |
| Sidebar "group by state" (failed / waiting first) | Buildkite ([build page](https://buildkite.com/docs/pipelines/build-page)) | Attention goes to what needs it | Cycle list sorted HALTED → FAIL → RUNNING → PASS | S |
| Issue = fingerprint group; first seen / last seen / count / regressed / escalating | Sentry [grouping](https://docs.sentry.io/concepts/data-management/event-grouping/), [fingerprint rules](https://docs.sentry.io/concepts/data-management/event-grouping/fingerprint-rules/), [issues](https://docs.sentry.io/product/issues/), [issue details](https://docs.sentry.io/product/issues/issue-details/) | Is this new, recurring, or a regression? | `disposition.json.fingerprint` × `ledger.jsonl` → Failure Classes page; "regressed" = fingerprint reappears after a PASS ship that cited it | M |
| Breadcrumbs from error to root; cached/pending/cancelled indicators | Dagger Traces ([blog](https://dagger.io/blog/introducing-dagger-traces), [observability](https://docs.dagger.io/features/observability/)) | Where in the hierarchy it broke | Failure panel: phase → round → finding → file:line; phase states skipped/cached shown dimmed | S |
| Session log = reasoning + tools + commit traceability | Copilot coding agent ([sessions](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/cloud-agent/track-copilot-sessions)); Devin Progress tab ([docs](https://docs.devin.ai/get-started/first-run), [fast.io](https://fast.io/resources/devin-session-tools-guide/)) | What the agent did and why, linkable from the commit | `*-interactions.ndjson` viewer; ship commit SHA in `run.json` ↔ cycle page deep link | M |
| Step-by-step trajectory viewer, keyboard nav | SWE-agent `sweagent I` (`.traj`, h/l steps, v verbose toggle) ([inspector](https://swe-agent.com/latest/usage/inspector/)); OpenHands trajectory-visualizer (actions blue / observations gray-red, arrow keys) ([repo](https://github.com/OpenHands/trajectory-visualizer)) | Replay one phase's tool calls | `*-interactions.ndjson` → step list; j/k; collapse tool output | S |
| Todo checklist with `pending / in_progress / completed` | Claude Code task tools ([todo tracking](https://code.claude.com/docs/en/agent-sdk/todo-tracking)) | Progress as a checklist | Phase stepper from `cycle-state.json.completed_phases`; queue from `inbox/*.json` | S |
| Counts by state above the list | Prefect [#12102](https://github.com/PrefectHQ/prefect/issues/12102) (the gap); Sentry event/user counts | Triage without filtering | Header tiles from the in-memory index | S |
| Raw history download / open raw JSON | Temporal Event History JSON view/download ([web UI](https://docs.temporal.io/web-ui)); AgentOps session replay ([repo](https://github.com/AgentOps-AI/agentops)) | Escape hatch when the UI is wrong | "Raw artifacts" list linking every file in the cycle dir | S |
| Named SSE events + keepalive + ids + catch-up | ntfy `/sse`, `since=`, `poll=1`, `keepalive` ([API](https://docs.ntfy.sh/subscribe/api/), [server.go](https://raw.githubusercontent.com/binwiederhier/ntfy/main/server/server.go)); Dozzle `sseWriter.Event("container-event", …)`, `Ping()` every 5 s ([logs.go](https://raw.githubusercontent.com/amir20/dozzle/master/internal/web/logs.go), [sse.go](https://raw.githubusercontent.com/amir20/dozzle/master/internal/support/web/sse.go)); go-sse `FiniteReplayer`/`ValidReplayer` on `Last-Event-ID` ([go-sse](https://github.com/tmaxmax/go-sse)) | Live board without refresh; survives reconnect | `/api/events` stream; `id:` = `ledger.jsonl` seq; `event: cycle|queue|ledger` | M |
| `embed.FS` + `fs.Sub` + dev-time live FS override | Dozzle `//go:embed all:dist`, `fs.Sub(content,"dist")`, `LIVE_FS` env ([main.go](https://raw.githubusercontent.com/amir20/dozzle/master/main.go)); statsviz `http.FileServerFS(static.Assets())` ([statsviz.go](https://raw.githubusercontent.com/arl/statsviz/master/statsviz.go)); ntfy `//go:embed site` ([server.go](https://raw.githubusercontent.com/binwiederhier/ntfy/main/server/server.go)); Go [embed](https://pkg.go.dev/embed) | Single binary, no build step | `go/internal/dashboard/static/{index.html,app.js,app.css}` embedded | S |
| Web dashboard for a Ralph-style loop (iterations, status, tokens, logs) | ralph-orchestrator `ralph web` ([repo](https://github.com/mikeyobrien/ralph-orchestrator)) — alpha; Wiggum CLI TUI ([search](https://wiggum.app/)) | Prior art is thin: iteration counts + token totals + log tail | Confirms the gap; nothing to copy beyond counters | — |

---

## 3. Recommended information architecture (opinionated, minimal)

Three pages. No settings page, no auth, no DB. Everything derives from files on disk through one in-memory index rebuilt on directory events.

### 3.1 `/` — Board (the only page most of the time)

```
┌ header ─────────────────────────────────────────────────────────────────────┐
│ RUNNING 2 · QUEUED 7 · PASS 24h 5 · FAIL 24h 2 · HALT 0 │ ship-rate 7d ▁▃▅▆▇█ │
├ left: QUEUE (inbox/*.json) ─────────┬ center: ACTIVE CYCLES ────────────────┤
│ 0.93 gate-diag …        [pending]   │ #1604  task…  ●●●●◐○○○  audit r2  12m │
│ 0.90 disposition-preseed…[pending]  │        claude/opus/xhigh   PASS-so-far│
│ 0.88 carryover-retire… [→ #1603]    │ #1605  task…  ●●○○○○○○  build     3m │
├ bottom: RECENT CYCLES GRID (Airflow-style) ──────────────────────────────────┤
│ phase \ cycle   1596 1597 1598 1599 1600 1601 1602 1603                      │
│ scout            ■    ■    ■    ■    ■    ■    ■    ■                        │
│ build            ■    ■    ■    ■    ■    ■    ■    ■                        │
│ audit            ■    ■r2  ■    ■r3  ■    ■    ■    ■r2   (r = rounds)      │
│ ship             ■    ■    ·    ■    ·    ■    ■    ■                        │
│ verdict          P    P    F    P    H    P    P    F                        │
└──────────────────────────────────────────────────────────────────────────────┘
```

- Above the fold: header tiles + active cycle cards. Cards show a **phase stepper** (`cycle-state.json.completed_phases`, current phase pulsing, audit **round badge**), elapsed, and the current phase's CLI/model/effort (`routing-decision-N.json`). This is the Claude Code checklist + Buildkite "group by state" + Prefect state-type badge in one card.
- Queue rows link to the consuming cycle once consumed (task = session).
- Grid cells: click → `/cycle/{id}#phase`. Tooltip: duration, CLI/model, verdict. Rounds shown as a small `rN` glyph, not as extra columns (keeps the grid dense; Airflow encodes retries by color + tooltip).

### 3.2 `/cycle/{id}` — Cycle detail

Order top-to-bottom is the triage order:

1. **Header**: verdict pill (state type), task title, cycle id, ship commit SHA (link), total tokens, wall time.
2. **What went wrong** panel — only when verdict ≠ PASS. Fixed-height, no scrolling; must let a human act in < 30 s:
   - Row 1 — pills: `failure-decision.json` **category · level · action** (e.g. `verdict-incoherence · P0 · HALT`).
   - Row 2 — **fingerprint** (mono, short hash, copy button) · `seen 4× · first #1571 · last #1603` · **REGRESSED** badge if it reappears after a PASS ship whose cycle cited the same fingerprint (`ledger.jsonl`).
   - Row 3 — **root_cause** (one paragraph from `disposition.json.root_cause`) with **legitimacy** badge (`legit / false-fail / infra`).
   - Row 4 — **cited findings**: top 3 from the final `audit-report.md`, each `severity · file:line · found in round N · still open?`. Click → the finding's text; second click → source file context.
   - Row 5 — **repair-round history**: `round1 FAIL (7 findings) → round2 FAIL (3: 5 resolved, 1 new) → final FAIL (2)`. Findings diffed by normalized title (Buildkite immutable attempts; GitHub attempt dropdown).
   - Row 6 — **salvage pointer** (`disposition.json`): branch / worktree path, with a copy-able `git` command; links to `retrospective-report.md`, `failure-dossier.json`, `acs-verdict.json` raw.
3. **Phase timeline**: horizontal bars from `phase-timing.json`, one lane per phase, audit rounds as consecutive bars in the audit lane; bar label = `cli/model/effort`; bar right edge = tokens (from `*-usage.json`). This is the Langfuse/MLflow Gantt reduced to a flat linear pipeline.
4. **Phase tabs** (one per phase, in order): **Report** (`*-report.md`, rendered) · **Interactions** (`*-interactions.ndjson` step viewer; j/k; actions vs observations colored like OpenHands' visualizer; tool output collapsed) · **Events** (`*-events.ndjson`, level filter defaulting to WARNING+ when verdict ≠ PASS) · **Prompt** (`*-prompt.txt`, collapsed) · **Usage** (`*-usage.json`). Audit tab additionally has the **round selector**.
5. **Raw artifacts**: every file in the cycle dir, size, mtime, link (Temporal "JSON" view as escape hatch).

### 3.3 `/failures` — Failure classes and trends

- Top: two charts, 30-day window, vanilla SVG (no chart lib): **ship rate per day** (stacked PASS/WARN/FAIL/HALT counts from `ledger.jsonl`) and **failure-category distribution** (`failure-decision.json.category`, stacked per week).
- Table grouped by `disposition.json.fingerprint` (Sentry issue stream): **count · first seen · last seen · category · level · legitimacy · latest root_cause (truncated) · salvage · state** (`new` = first seen < 24 h, `ongoing`, `regressed`). Sort default: last seen. Click → the cycles carrying that fingerprint, newest first.
- No per-class detail page (YAGNI): the cycle page already holds the detail.

### 3.4 Data model (in-memory index, rebuilt incrementally)

```go
type CycleSummary struct {
    ID, TaskID      string
    State           StateType   // QUEUED|RUNNING|PASS|WARN|FAIL|HALTED  (Prefect "type")
    CurrentPhase    string      // cycle-state.json               (Prefect "name")
    Phases          []PhaseRun  // name, status, start, end, cli, model, effort, inTok, outTok, round
    AuditRounds     int
    Verdict         *ACSVerdict // acs-verdict.json
    Failure         *Failure    // failure-decision.json + disposition.json (+ dossier path)
    ShipSHA         string      // run.json
    UpdatedAt       time.Time
}
type Failure struct{ Category, Level, Action, Fingerprint, Legitimacy, RootCause, Salvage string; Findings []Finding; Rounds []RoundDelta }
```

- Index is keyed by cycle id; `ledger.jsonl` line number is the global sequence used as the SSE `id:`.
- Fingerprint stats (`count/first/last/regressed`) are a second map built from the same index — no second pass over disk.
- `*-interactions.ndjson` and `*-prompt.txt` are **not** indexed; they are read on request (they are the bulk of the ~100 files).

---

## 4. Live update for a Go single binary

**Recommendation: SSE, one stream per tab, polling as fallback. Not WebSocket.**

Why SSE: the feed is one-directional; EventSource reconnects automatically and resends `Last-Event-ID` ([MDN](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events)); the wire format is trivial to emit from `net/http`. Single-binary Go tools that chose it: ntfy `/sse` ([API](https://docs.ntfy.sh/subscribe/api/)), Dozzle ([logs.go](https://raw.githubusercontent.com/amir20/dozzle/master/internal/web/logs.go)), PocketBase/Beszel realtime ([PocketBase](https://pocketbase.io/docs/api-realtime/)). statsviz chose WebSocket (gorilla, 1 s ticks) ([statsviz.go](https://raw.githubusercontent.com/arl/statsviz/master/statsviz.go)) and GoTTY needs it for bidirectional terminal I/O ([gotty](https://github.com/sorenisanerd/gotty)) — neither applies to us.

### 4.1 Server side (`GET /api/events?since=<seq>`)

Borrowed verbatim from the three reference implementations:

| Concern | Do this | Reference |
|---|---|---|
| Headers | `Content-Type: text/event-stream`, `Cache-Control: no-cache, no-transform`, `Connection: keep-alive`, `X-Accel-Buffering: no` | Dozzle [sse.go](https://raw.githubusercontent.com/amir20/dozzle/master/internal/support/web/sse.go) |
| Framing | `event: cycle\nid: 118301\ndata: {json}\n\n`; comment `: ping` every 15 s (Dozzle uses 5 s) | Dozzle [logs.go](https://raw.githubusercontent.com/amir20/dozzle/master/internal/web/logs.go); ntfy `fmt.Sprintf("event: %s\ndata: %s\n", …)` [server.go](https://raw.githubusercontent.com/binwiederhier/ntfy/main/server/server.go) |
| Flush | `http.NewResponseController(w).Flush()` after every event (Go ≥ 1.20); fall back to `http.Flusher` | [ResponseController](https://pkg.go.dev/net/http#ResponseController) |
| Disconnect | `select { case <-r.Context().Done(): return … }` | ntfy, Dozzle (same idiom) |
| Timeouts | Do **not** set `Server.WriteTimeout` (it "does not let Handlers make decisions on a per-request basis" and will kill the stream); leave it 0 and, if you want protection on other handlers, use `ResponseController.SetWriteDeadline` per request | [net/http Server](https://pkg.go.dev/net/http#Server) |
| Catch-up | `?since=<seq>` or `Last-Event-ID` → replay from a ring buffer of the last N events; if `since` is older than the ring, send one `event: snapshot` with the full index, then live events | ntfy `since=` / `since=latest` ([API](https://docs.ntfy.sh/subscribe/api/)); go-sse `FiniteReplayer` ([go-sse](https://github.com/tmaxmax/go-sse)) |
| Polling fallback | `GET /api/state?since=<seq>` returns the same events and closes | ntfy `poll=1` ([API](https://docs.ntfy.sh/subscribe/api/)) |
| Event types | `snapshot` (full index), `cycle` (one `CycleSummary`, replace-by-id), `queue` (inbox delta), `ledger` (one ledger line) | Dozzle's `container-event` / `logs-backfill` / `search-status` split |

Multiplex everything onto **one** EventSource per tab: browsers cap SSE at 6 connections per origin without HTTP/2 ([MDN](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events)).

### 4.2 Client side

Vanilla JS: `const es = new EventSource('/api/events?since='+lastSeq); es.addEventListener('cycle', e => upsertCard(JSON.parse(e.data)))`. If you prefer declarative swaps, the htmx SSE extension gives `sse-connect`, `sse-swap="cycle"`, `hx-trigger="sse:cycle"` and adds exponential back-off "on top of the browser's automatic reconnection" ([htmx sse](https://htmx.org/extensions/sse/)). Either is fine; do not pull in a framework.

### 4.3 Producing events from the filesystem

1. **Watch directories, not files.** fsnotify FAQ: atomic writers "write to a temporary file which is then moved to a destination"; "watch the parent directory and use `Event.Name` to filter" ([fsnotify](https://github.com/fsnotify/fsnotify)). Watch the runs root + each active cycle dir + `inbox/`. Recursive watching is not supported, so add a watch when a new cycle dir appears. Ignore `Chmod`. macOS kqueue holds one fd per watched path — that is another reason to watch dirs, not ~100 files per cycle.
2. **Debounce** a directory's events (~200 ms) and re-read only the JSON files that changed; recompute that cycle's `CycleSummary`; emit `event: cycle`.
3. **Atomic-write awareness.** Ask the harness to write every `*.json`/`*.md` artifact via temp-file-in-same-dir + rename (renameio: a concurrent open sees "either the file previously located at the destination path… or the just written file, but the file will always be present" ([renameio](https://pkg.go.dev/github.com/google/renameio/v2), [Stapelberg](https://michael.stapelberg.ch/posts/2017-01-28-golang_atomically_writing/))). Until that lands, the reader must treat `json.Unmarshal` failure as *transient*: keep the previous summary, mark `stale: true`, retry on the next event — never render a red "corrupt" state for a half-written file.
4. **Tailing NDJSON that is still being appended.** Keep a per-file offset; read from offset; only hand off lines terminated by `\n` (nxadm/tail `CompleteLines`: "Only return complete lines" ([tail](https://pkg.go.dev/github.com/nxadm/tail))); buffer the partial tail; if `size < offset` the file was truncated/replaced → reset to 0 (tail's "truncation/move detection", `ReOpen`) ([nxadm/tail](https://github.com/nxadm/tail)). `ledger.jsonl` is tailed this way and its line number becomes the SSE `id`. Do not tail `*-interactions.ndjson` continuously — read on demand from the cycle page.
5. **Startup**: full scan builds the index (a few hundred JSON files — sub-second); then fsnotify. If fsnotify fails (e.g. fd limits) degrade to a 2 s directory-mtime poll and say so in the header ("polling").

---

## 5. Anti-patterns to avoid (each seen in the wild)

1. **Overwriting retry history.** Buildkite: "if you retried a job, you lost the history of the job that was retried" — fixed by making retries new jobs ([blog](https://buildkite.com/blog/job-retries)). Never show only the latest `audit-report.md`; the `.roundN` archives are the product.
2. **Long-lived SSE through a buffering proxy without keepalives.** Argo UI's `/api/v1/workflow-events` stream died every 10–20 s with `ERR_INCOMPLETE_CHUNKED_ENCODING` behind an OpenShift route ([issue #5006](https://github.com/argoproj/argo-workflows/issues/5006)). Local binary avoids most of it; still send `: ping` and `X-Accel-Buffering: no` (Dozzle) so `ssh -L` / reverse proxies behave.
3. **Server-wide `WriteTimeout` on a streaming handler** — it is reset only "whenever a new request's header is read" ([net/http](https://pkg.go.dev/net/http#Server)); it silently kills SSE.
4. **One EventSource per widget** → the 6-connection browser cap ([MDN](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events)). One stream, named events.
5. **Watching files instead of their directory** — the rename in an atomic write orphans the watch ([fsnotify](https://github.com/fsnotify/fsnotify)).
6. **Lists without counts.** Prefect flow-run page: to know how many failed you had to "apply each state filter and keep track of the counts" ([#12102](https://github.com/PrefectHQ/prefect/issues/12102)).
7. **Deep span trees for a shallow pipeline.** Langfuse's tree needed a zoomable timeline redesign at scale ([changelog](https://langfuse.com/changelog/2025-03-19-new-trace-view)). Ten phases want lanes; reserve trees for within-phase interactions.
8. **Transcript as the primary view.** Devin added a Progress tab so users stop parsing chat transcripts ([fast.io](https://fast.io/resources/devin-session-tools-guide/)); Copilot shows "internal monologue" *inside* a session log you reach from the PR, not as the landing page ([sessions](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/cloud-agent/track-copilot-sessions)). Reports and the failure panel first.
9. **Implicit grouping.** Sentry needed fingerprint rules (`error.type:ConnectionError -> connection-error`) because default stack-trace grouping mis-buckets ([fingerprint rules](https://docs.sentry.io/concepts/data-management/event-grouping/fingerprint-rules/)). Our `disposition.json.fingerprint` is explicit — display it and never re-derive a second one in the UI.
10. **Bidirectional transport for a one-way feed.** statsviz's gorilla WebSocket ([statsviz.go](https://raw.githubusercontent.com/arl/statsviz/master/statsviz.go)) is fine for a 1 Hz metrics push but forgoes free reconnection + `Last-Event-ID`.
11. **TUI-only monitoring** (Wiggum CLI dashboard, `sweagent inspect` pager) — not linkable from a commit or a retro. Keep the TUI; the web page adds deep links (`/cycle/{id}#audit-r2`).
12. **Storing dashboard state in a DB.** Dozzle "doesn't store any log files" ([README](https://github.com/amir20/dozzle)); our history already lives on disk in the cycle dirs and `ledger.jsonl`. An in-memory index is enough; a DB is YAGNI.

---

## 6. Sources (fetched unless marked)

Agent observability
- Langfuse data model — https://langfuse.com/docs/observability/data-model — fetched
- Langfuse log levels — https://langfuse.com/docs/observability/features/log-levels — fetched
- Langfuse timeline view (2024-06-12) — https://langfuse.com/changelog/2024-06-12-timeline-view — fetched
- Langfuse new trace view (2025-03-19) — https://langfuse.com/changelog/2025-03-19-new-trace-view — fetched
- Arize Phoenix LLM traces — https://arize.com/docs/phoenix/tracing/llm-traces — fetched
- Arize Phoenix sessions setup — https://arize.com/docs/phoenix/tracing/how-to-tracing/setup-tracing/setup-sessions — fetched
- OpenInference semantic conventions — https://github.com/Arize-ai/openinference/blob/main/spec/semantic_conventions.md — fetched
- AgentOps README (session replay, "Event Graphs" waterfall) — https://github.com/AgentOps-AI/agentops — fetched (README only; dashboard screenshots not inspected)
- OpenLLMetry README — https://github.com/traceloop/openllmetry — fetched; annotations (@workflow/@task/@agent/@tool) — https://www.traceloop.com/docs/openllmetry/tracing/annotations — fetched
- Helicone sessions — https://docs.helicone.ai/features/sessions — fetched
- LangSmith observability concepts — https://docs.langchain.com/langsmith/observability-concepts — fetched
- W&B Weave tracing (ops/calls/traces/threads) — https://docs.wandb.ai/weave/guides/tracking/tracing — fetched; UI feature names **UNVERIFIED** (page describes model, not UI)
- MLflow tracing — https://mlflow.org/docs/latest/genai/tracing/ — fetched
- OTel GenAI attribute registry — https://opentelemetry.io/docs/specs/semconv/registry/attributes/gen-ai/ — fetched; GenAI semconv repo — https://github.com/open-telemetry/semantic-conventions-genai — fetched; agent spans — https://github.com/open-telemetry/semantic-conventions-genai/blob/main/docs/gen-ai/gen-ai-agent-spans.md — fetched

Coding-agent and workflow UIs
- OpenHands backend architecture (EventStream, AgentState) — https://docs.openhands.dev/usage/architecture/backend — fetched
- OpenHands trajectory-visualizer — https://github.com/OpenHands/trajectory-visualizer — fetched
- SWE-agent trajectory inspector — https://swe-agent.com/latest/usage/inspector/ — fetched
- Aider history options — https://aider.chat/docs/config/options.html — fetched
- Devin first session (Ask/Agent modes) — https://docs.devin.ai/get-started/first-run — fetched; Progress/Shell/IDE/Desktop tabs — https://fast.io/resources/devin-session-tools-guide/ — fetched, **secondary source**
- Claude Code todo tracking — https://code.claude.com/docs/en/agent-sdk/todo-tracking — fetched
- GitHub Actions re-running — https://docs.github.com/en/actions/managing-workflow-runs-and-deployments/managing-workflow-runs/re-running-workflows-and-jobs — fetched; `github.run_attempt` — https://docs.github.com/en/actions/reference/workflows-and-actions/contexts — fetched
- Copilot coding agent sessions — https://docs.github.com/en/copilot/how-tos/use-copilot-agents/cloud-agent/track-copilot-sessions — fetched
- Temporal Web UI — https://docs.temporal.io/web-ui — fetched; retry policies — https://docs.temporal.io/encyclopedia/retry-policies — fetched
- Argo Server — https://argo-workflows.readthedocs.io/en/latest/argo-server/ — fetched (UI features not on that page); `argo watch` — https://argo-workflows.readthedocs.io/en/latest/cli/argo_watch/ — fetched; SSE watch failure — https://github.com/argoproj/argo-workflows/issues/5006 — fetched
- Airflow UI (Grid/Graph/Gantt) — https://airflow.apache.org/docs/apache-airflow/stable/ui.html — fetched
- Prefect 3 states — https://docs.prefect.io/v3/concepts/states — fetched; state-count gap — https://github.com/PrefectHQ/prefect/issues/12102 — fetched
- Buildkite build page — https://buildkite.com/docs/pipelines/build-page — fetched; job retries blog — https://buildkite.com/blog/job-retries — fetched
- Dagger observability — https://docs.dagger.io/features/observability/ — fetched; Dagger Traces blog — https://dagger.io/blog/introducing-dagger-traces — from search snippet
- GitLab pipeline graphs — https://docs.gitlab.com/ci/pipelines/pipeline_graphs/ — **UNVERIFIED** (auth redirect on docs.gitlab.com and 404 on raw mirrors; no claims made from it)
- ralph-orchestrator (`ralph web`) — https://github.com/mikeyobrien/ralph-orchestrator — fetched; Wiggum CLI — https://wiggum.app/ — search snippet only

Go single-binary live dashboards and mechanics
- Dozzle README — https://github.com/amir20/dozzle — fetched; main.go (embed) — https://raw.githubusercontent.com/amir20/dozzle/master/main.go — fetched; logs.go (SSE loop) — https://raw.githubusercontent.com/amir20/dozzle/master/internal/web/logs.go — fetched; sse.go (headers/flush) — https://raw.githubusercontent.com/amir20/dozzle/master/internal/support/web/sse.go — fetched
- statsviz — https://github.com/arl/statsviz — fetched; statsviz.go — https://raw.githubusercontent.com/arl/statsviz/master/statsviz.go — fetched
- ntfy subscribe API — https://docs.ntfy.sh/subscribe/api/ — fetched; server.go — https://raw.githubusercontent.com/binwiederhier/ntfy/main/server/server.go — fetched
- pprof web UI routes — https://raw.githubusercontent.com/google/pprof/main/internal/driver/webui.go — fetched
- GoTTY (WebSocket + xterm.js) — https://github.com/sorenisanerd/gotty — fetched
- Beszel — https://github.com/henrygd/beszel — search only; its hub realtime is PocketBase SSE — https://pocketbase.io/docs/api-realtime/ — fetched
- MDN Server-sent events — https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events — fetched
- htmx SSE extension — https://htmx.org/extensions/sse/ — fetched
- go-sse (Replayer, Last-Event-ID) — https://github.com/tmaxmax/go-sse — fetched
- Go `embed` — https://pkg.go.dev/embed — fetched; `http.ResponseController` — https://pkg.go.dev/net/http#ResponseController — fetched; `http.Server` timeouts — https://pkg.go.dev/net/http#Server — fetched
- fsnotify — https://github.com/fsnotify/fsnotify — fetched
- nxadm/tail — https://github.com/nxadm/tail and https://pkg.go.dev/github.com/nxadm/tail — fetched
- renameio — https://pkg.go.dev/github.com/google/renameio/v2 — fetched; Stapelberg atomic writes — https://michael.stapelberg.ch/posts/2017-01-28-golang_atomically_writing/ — fetched

Failure presentation
- Sentry event grouping — https://docs.sentry.io/concepts/data-management/event-grouping/ — fetched; fingerprint rules — https://docs.sentry.io/concepts/data-management/event-grouping/fingerprint-rules/ — fetched; Issues list — https://docs.sentry.io/product/issues/ — fetched; Issue details — https://docs.sentry.io/product/issues/issue-details/ — fetched

Not applicable / not fetched: `gops` (CLI, no web UI), `expvarmon` and `lazygit` (TUIs), `miniflux` (no live stream), `uptime-kuma` (Node). No claims made about them.
