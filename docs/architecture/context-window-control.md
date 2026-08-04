# Context-Window Control (v9.1.0+)

> Canonical reference for evolve-loop's context-size enforcement. Paired
> capability to `checkpoint-resume.md` — same hazard (resource exhaustion),
> different dimension (context vs. cost).

## Why this exists

Cost budgets (`--budget-usd N`) and context budgets are two independent
dimensions of resource consumption. Pre-v9.1.0:

- **Cost budget**: tracked at the dispatcher level. Cycles ran until
  cumulative spend exceeded the cap. Hard tripwire at 100%, no graceful
  pause. (v8.58 Layer B improved this; v9.1.0 Cycle 2 adds 80/95% pre-
  emptive thresholds.)
- **Context budget**: WARN-only stderr line per phase. Operator had no
  way to ask "how much context has this cycle burned?" without grepping
  scrollback. No cumulative tracking across phases. No autotrim.

v9.1.0 Cycle 6 closes the gap with three deliverables.

## What v9.1.0 Cycle 6 does

### 1. Per-phase autotrim (opt-in)

When `EVOLVE_CONTEXT_AUTOTRIM=1` AND the assembled prompt exceeds
`EVOLVE_PROMPT_MAX_TOKENS` (default 30000), `subagent-run.sh` trims the
prompt aggressively:

- Preserve **60% from the head** (instructions, role context, intent).
- Preserve **35% from the tail** (current task, most recent activity).
- Drop the middle (typically low-priority ledger entries, instinct
  summaries, older artifacts).
- Insert an explicit marker between head and tail so the LLM knows
  content was dropped.

The trim is non-destructive — Builder's edits, Scout's research, Auditor's
verdict are all in artifact files; the prompt is just an assembly *view*
of those artifacts. The next phase reassembles its own prompt from the
canonical files.

### 2. Per-cycle monitor JSON

`subagent-run.sh` writes `.evolve/runs/cycle-N/context-monitor.json` per
phase invocation:

```json
{
  "cycle": 14,
  "lastUpdated": "2026-05-11T16:42:00Z",
  "phases": {
    "intent":  {"input_tokens": 7340, "cap_tokens": 30000, "cap_pct": 24, "measuredAt": "..."},
    "scout":   {"input_tokens": 11820, "cap_tokens": 30000, "cap_pct": 39, "measuredAt": "..."},
    "builder": {"input_tokens": 24500, "cap_tokens": 30000, "cap_pct": 81, "measuredAt": "..."}
  },
  "cumulative_input_tokens": 43660,
  "cumulative_cap": 120000,
  "cumulative_pct": 36
}
```

- `cap_tokens` is the per-phase cap (`EVOLVE_PROMPT_MAX_TOKENS`, default 30k).
- `cumulative_cap` is `cap_tokens * 4` (~4 expensive phases per cycle).
- `cumulative_pct` is the cycle-level usage indicator.

### 3. Operator observability

```bash
$ bash legacy/scripts/observability/show-context-monitor.sh 14

Cycle 14 context-monitor:
  last updated: 2026-05-11T16:42:00Z

  phase                input_tokens   cap_tokens cap_pct
  -------------------- ------------ ------------ --------
  intent                       7340        30000       24
  scout                       11820        30000       39
  builder                     24500        30000       81

  CUMULATIVE: 43660 / 120000 tokens (36%)
```

Variants:

| Flag | Purpose |
|---|---|
| (no flag, with `<cycle>`) | Tabular render of the specified cycle |
| (no args) | Tabular render of the most recent cycle |
| `--watch` | Live-tail mode (3s refresh) of latest cycle |
| `--json <cycle>` | Emit raw JSON for scripting |

Threshold annotations:

- `>>> WARN: cumulative >= 80%` — emitted at `EVOLVE_CHECKPOINT_WARN_AT_PCT`
- `>>> CRITICAL: cumulative >= 95% — next phase will signal checkpoint`
  — emitted at `EVOLVE_CHECKPOINT_AT_PCT`

The thresholds intentionally share env vars with `checkpoint-resume.md` —
context exhaustion and cost exhaustion produce the same operator action
(graceful pause + resume), so they share the signal channel.

## Env-var reference

| Var | Default | Role |
|---|---|---|
| `EVOLVE_PROMPT_MAX_TOKENS` | `30000` | Per-phase cap. Unchanged from v8.56.0; v9.1.0 only adds autotrim enforcement |
| `EVOLVE_CONTEXT_AUTOTRIM` | `0` (opt-in) | Set `1` to enable head/tail-preserving autotrim when over cap. v9.2 candidate flips default-on once empirical data shows it's non-harmful to verdict quality |
| `EVOLVE_PROMPT_BUDGET_ENFORCE` | `0` | Pre-v9.1.0 hard-fail mode. Set `1` to fail-fast when over cap instead of WARN. Mutually exclusive with autotrim (autotrim runs first; if it can't get under cap, the cap-enforce check applies) |

## When to enable autotrim

| Workload | Recommendation |
|---|---|
| Multi-cycle `/loop` runs (>10 cycles) | Enable. Cumulative context grows; autotrim caps the per-phase burn |
| Single experimental cycle | Skip. The WARN line tells you what happened; you can iterate |
| Retrospective-heavy cycles | Skip for retrospective specifically; it's the synthesizer and needs the full picture. Per-role override not yet implemented (Cycle 6 wires global enable only) |
| Tight subscription quota | Enable. Smaller prompts = lower output tokens = slower quota burn |

## Interaction with checkpoint-resume

The two systems integrate at the threshold layer:

1. **Cumulative context cap** — `subagent-run.sh` records cumulative usage.
2. **show-context-monitor.sh** emits `WARN`/`CRITICAL` annotations based
   on the same `EVOLVE_CHECKPOINT_WARN_AT_PCT` / `EVOLVE_CHECKPOINT_AT_PCT`
   thresholds used by the cost-side dispatcher logic.
3. **Implicit checkpoint** — when context is in the danger zone, the
   same `EVOLVE_CHECKPOINT_REQUEST=1` signal is set as for cost.

This means an operator running `/evo:loop --budget-usd 5 "<goal>"` who
hits the context wall before the cost wall sees the same graceful pause
behavior as if cost had hit first.

## What v9.1.0 Cycle 6 does NOT do

- **Mid-phase trim.** The autotrim runs at prompt-assembly time, not
  during an in-flight subagent's reasoning. The subagent's own context
  is its concern (the kernel can't see it).
- **Selective phase exemption.** A future v9.2+ change could allow
  `EVOLVE_CONTEXT_AUTOTRIM_EXCLUDE=retrospective` to skip autotrim for
  the synthesizer phase. Not yet implemented.
- **Token-accurate measurement.** The 1-token≈4-bytes heuristic is an
  upper bound for English text. The cap_pct measurement is conservative
  — actual token counts are typically 80-95% of the byte-based estimate.

## Context-fill ratio — `internal/contextfill` (cycle-1269)

The autotrim/monitor machinery above measures the **assembled prompt** against an
operator-set byte-heuristic cap (`EVOLVE_PROMPT_MAX_TOKENS`, default 30000).
That is a *pre-dispatch* view: it says nothing about how much of the **model's
real context window** a phase ended up occupying once its own output, cache reads
and cache writes are counted.

`go/internal/contextfill` closes that gap on the measurement side. It is a pure
stdlib+`cyclestate` leaf — a derivation over data the loop **already** persists
(`cyclestate.TokenUsage` per phase, rolled up into `<workspace>/phase-timing.json`
by `internal/phasetiming`), so it needs no new instrumentation of the tmux or
headless drivers:

```go
const HotThreshold = 0.85                 // inclusive: >= 0.85 of the window is "hot"
var  ErrInvalidWindow error               // a window we cannot identify is an error, not a guess
func FillRatio(tokens cyclestate.TokenUsage, windowSize int) (float64, error)
func IsHot(ratio float64) bool
func WindowSizeForTier(tier string) int   // "fast"/"balanced"/"deep"/"top"; unknown => 0
```

Two design points are load-bearing:

- **Occupancy is the whole token record.** `Input + Output + CacheRead + CacheWrite`
  all consume window space; summing only `Input`+`Output` under-reports fill by
  more than half on cache-heavy phases — exactly the phases under pressure.
- **The ratio is never clamped at 1.0, and an unknown window is never defaulted.**
  Telemetry that saturates at "full" cannot distinguish a phase that just fit from
  one that overran by 50%, which is the entire diagnostic value. Likewise an
  unrecognised tier reports `0` from `WindowSizeForTier`, which flows into
  `FillRatio` as `ErrInvalidWindow` rather than a fabricated fill number.

`WindowSizeForTier` is deliberately a flat per-**tier** stub (every Claude tier the
loop routes to today shares a 200k-token window). The upgrade path, when that stops
holding, is a per-model registry keyed off the resolved model id (mirroring
`internal/modelcatalog`'s tables) with this function kept as the tier-level fallback.

### Deliberately deferred (nothing imports this package yet)

The package ships as measurement-only; three follow-ups are queued, in dependency
order, and are **not** wired in cycle-1269:

1. `wire-context-fill-stage` — the `Off`/`Advisory`/`Enforce` dial from
   `.evolve/policy.json` (the `internal/cyclebudget` Stage precedent), plus a
   `ContextFill` field persisted on `phasetiming.Entry`. Default-off, byte-identical
   to today when the policy key is absent.
2. `context-fill-hint-prompt-injection` — an advisory `ContextBudgetHint` line beside
   the existing `TurnBudgetHint` injection (`internal/phases/runner/runner.go`).
3. Enforce-stage behavior (interrupting an in-flight phase at the high-water mark)
   — needs its own design pass on safely halting a live tmux REPL session.

When (1) lands it should **reconcile with, not duplicate,** `context-monitor.json`
above: same hazard, two measurement points (pre-dispatch prompt bytes vs. realized
window occupancy).

## See also

- `docs/architecture/checkpoint-resume.md` — paired capability for cost
  exhaustion.
- `docs/architecture/token-economics-2026.md` — per-phase cost forensics
  and ROI-ordered optimization roadmap.
- `docs/architecture/token-floor-history.md` — campaign-by-campaign
  static-context floor measurements (see **Campaign E — clean-boot, 2026-07-17**).
- `knowledge-base/research/token-optimization-2026/part4-per-phase-boot-context.md`
  + `part5-campaign-implementation-2026-07-17.md` — the **clean-boot** campaign: cut the
  per-turn boot base ~64K→~19–32K via config-injected launch flags (`extra_flags_by_cli`),
  measured **−39% cache_read/cycle**. Complementary lever to autotrim: autotrim trims the
  *assembled prompt*; clean-boot trims the *fixed boot + tool-schema base* re-read every
  turn. Measurement prerequisite (token telemetry attribution fix): part5 §1 +
  `docs/architecture/adr/0071-token-telemetry-attribution-and-clean-boot.md`.
- `docs/adr/0002-disable-slash-commands-semantics.md` — the defense-in-depth semantics
  the clean-boot skill flags follow (master-off + `Skill(<name>)` allowlist).
- `legacy/scripts/observability/show-context-monitor.sh` — operator-facing tool.
- `legacy/scripts/tests/context-window-control-test.sh` — 22-assertion test
  suite covering autotrim algorithm, monitor JSON, and operator tool.
