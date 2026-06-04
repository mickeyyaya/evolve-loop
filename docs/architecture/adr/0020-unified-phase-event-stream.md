# ADR 0020 — Unified phase event stream (live normalizer)

Status: ACCEPTED (2026-05-26) · Supersedes the post-phase-only machine role of `internal/logfilter` (`.clean.txt` human render retained) · Superseded-by: [ADR-0036](0036-content-vs-liveness-channel-protocol.md) (content-vs-liveness channel protocol — closes the logfilter-duplicate + phaseobserver-raw-read ends left open here)

> **Implemented** on branch `worktree-phasestream-normalizer` (tasks 1–5, commits e43873a → 34f162a). Go-pipeline notes vs the original proposal below:
> - **Post-phase, not live tail.** The Go `bridge.Launch` is synchronous and both machine consumers (cyclecost, cycleclassify) read strictly post-phase, so the producer is `phasestream.Produce` (batch, called from `phases/runner` after `bridge.Launch`), not a live-tailing goroutine. `Normalizer.Poll`/`Run` remain for a future live call site.
> - **Observer merge is a no-op in Go.** The live observer (stall→pgid-kill) was never wired in the Go pipeline — `phaseobserver` is bash-spawned (`subagent-run.sh`), `adapters/observer` + `core.Observer` are dormant scaffolding. So "retire the separate observer spawn" required no Go change; the dormant packages are left untouched (the bash path still exec's `evolve phase-observer`).
> - **Producer runs before the bridge-error guard** so a phase that fails on a timeout/429/529 still emits events for cycleclassify to classify the infra failure.

## Context

Today **four independent passes** parse the same raw `<agent>-stdout.log`, each
re-deriving event structure with partial fidelity:

| Pass | Where | Live? | Extracts |
|---|---|---|---|
| `logfilter.Process` | `internal/logfilter` | post-phase | full event taxonomy → human `.clean.txt` (read by **nobody** machine) |
| cyclecost | `internal/cyclecost/cyclecost.go:142` | post-phase | last `result` → cost + 4 token counts |
| phaseobserver | `internal/phaseobserver/phaseobserver.go:298` | **live (5 s poll)** | liveness, tool/error counts, cost, rate-limit, loop-SHA |
| cycleclassify | `internal/cycleclassify/classify.go:57` | post-cycle | infra markers (EPERM, rate-limit, 429/529, ETIMEDOUT) from stdout **+ stderr** |

Problems: duplicated parse logic; each consumer wades through `stream_event`
redraw deltas (the dominant noise); the one component that already classifies
*everything* (`logfilter`) feeds no machine consumer; and `phaseobserver`
already implements the exact unified envelope we want (`phaseobserver.go:368`:
`{schema_version, ts, trace_id, source{}, type, severity, data}` with
`INFO/WARN/INCIDENT`) — but only for its own observations.

There are also **two** observer implementations (`phaseobserver` rich /
separate-process, `adapters/observer` minimal / in-process) plus
`dispatchevents` (cycle boundary). The collapse below unifies the per-phase pair.

### The signal each consumer needs (the "clean information")

- **cyclecost** — last `result`: `total_cost_usd`, `usage.{cache_read,cache_creation,output,input}_tokens`. Output `Summary` JSON shape is **frozen** (byte-compatible with `show-cycle-cost.sh --json`); input may change freely.
- **phaseobserver** — liveness tick (freshness), `tool_use` count, `tool_result.is_error`, `result` cost/cache, `rate_limit_event`, tool-input SHA (loop detection).
- **cycleclassify** — typed infra markers from stdout **and** stderr.
- **NEW — interactive actions** — `AskUserQuestion` prompts (question + options + recommendation), `ExitPlanMode` plans, permission prompts, and their resolution. These are **signal, never noise**, and must survive at **full fidelity** (exempt from the generic 200-byte `tool_use` truncation).

## Locked design parameters

| Decision | Choice | Rationale |
|---|---|---|
| Cutover | **Full collapse now** | One normalizer; raw parsers in cyclecost/cycleclassify retired; no runtime fallback → parity tests gate the merge |
| Stream model | **Single unified `<agent>-events.ndjson`** | One severity-tagged envelope file per phase; normalizer events + observer incidents both land here |
| Liveness | **Coalesced `progress` tick** | `stream_event` payloads dropped; one tick per poll-batch carries cumulative tokens + fresh ts; stall granularity = poll (~5 s, status quo) |
| Infra markers | **Normalizer owns the vocabulary** | One marker enum in the shared package; emits typed `infra_failure` envelopes from stdout structured events + stderr text; cycleclassify just filters |
| Producer topology | **Merge observer tail+rules into the normalizer** | Honors single-writer discipline (no two-process append race), removes the double tail, resolves the duplicate-observer split |

## Architecture (4-layer)

```
L1 OBSERVE    <agent>-stdout.log   <agent>-stderr.log        [raw · forensic · BYTE-FOR-BYTE UNCHANGED]
                       │  single live tail (Seek-offset, reused from phaseobserver.tail)
                       ▼
L2 DIGEST     ┌───────────────── normalizer (always-on, per phase) ─────────────────┐
              │ classify once (phasestream taxonomy, ported from logfilter)          │
              │   • stream_event delta        → coalesce  → progress (liveness)       │
              │   • tool_use AskUserQuestion/ExitPlanMode → interaction (FULL fidelity)│
              │   • stderr text + structured 429/529/EPERM → infra_failure (typed)    │
              │   • result · rate_limit · tool_use · tool_result · assistant · …      │
              │   • DROP: stream_event payloads, system:init, blank, spinner/border   │
              └───────────────────────────────┬─────────────────────────────────────┘
                                               │ single-writer sink (one goroutine owns the fd)
L3 TRANSLATE                                   ▼
              <agent>-events.ndjson   ◄── observer rules (stall/loop/error-spike/cost) append incidents here
                 │                 │                       │
                 ▼                 ▼                       ▼
             cyclecost        observer-rules          cycleclassify
          (last kind=result) (latest-ts freshness)   (kind==infra_failure)
                 │                 │                       │
L4 EXECUTE   cost meter +     pgid kill on            RETRY / STOP
             checkpoint       INCIDENT                (failure-adapter)
```

`.clean.txt` survives as an **optional human render** of the unified ndjson
(same single classification pass, two outputs). Raw `.log` is never touched —
cyclecost's frozen-output contract and forensic replay both depend on it.

## Unified envelope (schema 2.0)

Extends phaseobserver's 1.0 envelope; adds `seq` (gap-free per-file ordering)
and `kind` (replaces ad-hoc `type`).

```json
{
  "schema_version": "2.0",
  "seq": 1234,
  "ts": "2026-05-26T08:41:09Z",
  "trace_id": "cycle-12-build-1748246469",
  "source": {"producer": "normalizer", "cli": "claude-tmux", "cycle": 12, "phase": "build", "agent": "build"},
  "kind": "result",
  "severity": "INFO",
  "data": {"cost_usd": 0.42, "tokens": {"in": 1200, "out": 850, "cache_r": 30000, "cache_c": 1800}, "num_turns": 7, "is_error": false}
}
```

### Event taxonomy

| kind | producer | severity | key `data` | consumed by |
|---|---|---|---|---|
| `result` | normalizer | INFO | cost_usd, tokens{in,out,cache_r,cache_c}, num_turns, is_error | **cyclecost** (last/file), observer (cost) |
| `progress` | normalizer | INFO | cum_output_tokens, delta_count, since_ms | **observer** (liveness/stall) |
| `tool_use` | normalizer | INFO | name, id, input_excerpt (200 B) | observer (count, loop-SHA) |
| `tool_result` | normalizer | INFO/WARN | tool_use_id, is_error, excerpt (200/100) | observer (error_count) |
| `interaction` | normalizer | INFO | mode(`ask_user_question`\|`exit_plan_mode`\|`permission`), question, options[]{label,description,recommended}, plan, resolved_choice, auto_resolved | **human render + audit trail** — never truncated, never dropped |
| `assistant_text` / `thinking` | normalizer | INFO | text | human render |
| `rate_limit` | normalizer | WARN | retry_after, raw | observer, cycleclassify |
| `infra_failure` | normalizer | INCIDENT | marker(`eperm`\|`rate_limit`\|`api_429`\|`api_529`\|`timeout`\|`conn_refused`), source(stdout\|stderr), excerpt | **cycleclassify** (filter) |
| `system_hook` | normalizer | INFO | hook_name, event, exit_code, outcome | human render |
| `error` | normalizer | WARN | excerpt | observer, cycleclassify |
| `unknown` | normalizer | INFO | raw_excerpt (500 B) | never silently dropped |
| `stall_no_output` | observer | INCIDENT | idle_s, threshold_s | execute (pgid kill) |
| `loop_detected` / `error_spike` / `cost_anomaly` | observer | WARN/INCIDENT | rule-specific | execute |
| `observer_started` / `heartbeat` / `observer_shutdown` | observer | INFO | counters | report |

**Dropped (never emitted):** raw `stream_event` deltas (→ `progress`),
`system:init`, blank lines, pure spinner/border lines (tmux). Everything else is
preserved; only `tool_use`/`tool_result`/`unknown` get truncated.
`interaction`, `result`, `infra_failure` are full-fidelity.

### Interactive-action capture (new requirement)

In stream-json, `AskUserQuestion` / `ExitPlanMode` arrive as `tool_use` blocks.
The normalizer special-cases them **by tool name** before the generic truncation:

- `AskUserQuestion` → parse input fully → `interaction{mode:ask_user_question, question, options[], recommended}`.
- `ExitPlanMode` → `interaction{mode:exit_plan_mode, plan}`.
- The matching `tool_result` (correlated by `tool_use_id`) sets `resolved_choice` + `auto_resolved` (true when `EVOLVE_INTERACTIVE_POLICY` self-resolved it — ties into v12.1 bridge policy injection).

tmux fidelity is best-effort (rendered boxes); stream-json is the high-fidelity path.

## Detection rules (observer, L3) — unchanged thresholds, new input

Each keys off the unified stream instead of raw. Env vars unchanged.

| Rule | Fires on | Threshold env |
|---|---|---|
| stall_no_output | now − latest-envelope-ts ≥ stall | `EVOLVE_OBSERVER_STALL_S=600` |
| loop_detected | same tool-input SHA × N in window | `LoopN=6` / `LoopWindowS=120` |
| error_spike | error_count / tool_result_count ≥ rate | `ErrorRate=0.3` |
| cost_anomaly | result cost > σ over rolling mean | `CostSigma=2` |

## Lifecycle

- Spawned per phase, **before** `bridge.Launch` returns content (so it tails live); dies on EOF-grace or phase end. Always-on — **not** gated by `EVOLVE_OBSERVER_ENFORCE` (that flag now controls only whether INCIDENT → pgid kill, not whether events exist).
- Wired in `runner.go` in place of the post-phase `logfilter.Process` call (`runner.go:309`) and the separate observer spawn.

## Cutover sequence (full collapse — one PR, gated by parity tests)

1. **[DONE @ e43873a]** New `internal/phasestream` — envelope types + taxonomy + marker vocabulary + classifier (ports `logfilter/streamjson.go` + `plaintext.go` to emit envelopes, not text). 11 unit + 2 live tests.
2. **[DONE @ 5777b16]** Live normalizer `Poll` core — single tail of stdout+stderr (reuse `phaseobserver.tail`), classify → single-writer sink, stall rule. (`Run` ticker-loop deferred — no live call site in the synchronous-bridge Go runner.)
3. **[DONE @ 8502654]** Repoint **cyclecost** → glob `*-events.ndjson`, last `kind==result`. `Summary` JSON shape FROZEN.
4. **[DONE @ a25ff10]** Repoint **cycleclassify** → glob `*-events.ndjson` filter `kind==infra_failure`; keep `orchestrator-report.md` scan for ship-gate/audit/build. (Also added symmetric stdout infra detection to `classifyPlain` so stdout-borne markers — cycle-61 — survive the cutover.)
5. **[DONE @ 34f162a]** Wire `phasestream.Produce` (post-phase batch) into `runner.go` via the `EventsProducer` seam; retire the post-phase logfilter machine role (keep optional `.clean.txt` render). Runs before the bridge-error guard.
6. **[DONE]** Raw-parse paths deleted in cyclecost/cycleclassify (tasks 3/4 — events-only, no fallback). `<agent>-observer-events.ndjson` retirement is N/A in the Go pipeline (observer dormant — bash-only).
7. **[DONE @ 34f162a]** **Parity tests** (gate, no runtime fallback): `TestSummarizeCycle_ParityViaProduce` (cost identical) + `TestClassify_ParityViaProduce` (infra verdict identical) drive a real raw log through the actual `Produce` path. (Observer-stall parity N/A — dormant in Go.)

## Out of scope

- Raw `.log` format or the bridge's `--stdout-log` CLI plumbing (unchanged).
- `dispatchevents` (cycle-boundary, separate audience) and `phasewatchdog` (already deprecated by observer).
- High-fidelity tmux interaction capture (stream-json is the supported path).
- Cross-phase aggregation beyond what cyclecost/cycleclassify already glob.

## Consequences

- 4 parsers → 1 normalizer + 3 thin reducers; noise (`stream_event`) dropped at the source for every consumer.
- Single severity vocabulary + marker enum, defined once in `phasestream`.
- Interactive decisions become a first-class, auditable event class.
- Risk: full collapse touches billing/stall/classification with no fallback — mitigated only by the parity-test gate; a normalizer parse regression is a billing/integrity regression. This is the explicit cost of "full collapse now."
