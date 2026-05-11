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
$ bash scripts/observability/show-context-monitor.sh 14

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

This means an operator running `/evolve-loop --budget-usd 5 "<goal>"` who
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

## See also

- `docs/architecture/checkpoint-resume.md` — paired capability for cost
  exhaustion.
- `docs/architecture/token-economics-2026.md` — per-phase cost forensics
  and ROI-ordered optimization roadmap.
- `docs/architecture/token-floor-history.md` — campaign-by-campaign
  static-context floor measurements.
- `scripts/observability/show-context-monitor.sh` — operator-facing tool.
- `scripts/tests/context-window-control-test.sh` — 22-assertion test
  suite covering autotrim algorithm, monitor JSON, and operator tool.
