# Phase Tracker & Latency-Cost Observability

> Canonical design for real-time subagent visibility, per-phase latency/cost analysis, and ephemeral-data lifecycle. Reference for all `scripts/observability/` tooling that produces or consumes per-cycle metrics. Layered on top of existing `*-stdout.log`, `*-usage.json`, `*-timing.json` sidecars (see `scripts/dispatch/subagent-run.sh:880-950`) — does **not** replace them.

## Table of contents

- [Pain points addressed](#pain-points-addressed)
- [Data sources](#data-sources)
- [Folder structure](#folder-structure)
- [Event schema](#event-schema)
- [Trace.md format](#tracemd-format)
- [Cycle-metrics rollup](#cycle-metrics-rollup)
- [Phase-report integration](#phase-report-integration)
- [TTL policy](#ttl-policy)
- [Two-phase rollout](#two-phase-rollout)
- [File index](#file-index)

## Pain points addressed

| Pain | Pre-tracker | Post-tracker |
|---|---|---|
| Operator can't see subagent activity until phase report drops (5–15 min blackout) | `--output-format json` returns one final blob | `--output-format stream-json` → tracker writes per-event NDJSON + tailable `trace.md` in real time |
| No per-tool-call latency breakdown — can't tell if scout slow due to WebSearch or thinking | Only final cumulative cost/turns | Per-event `latency_ms` in NDJSON; rollup computes `tool_calls.by_latency[]` |
| No cycle-over-cycle baseline — "is this slow vs typical?" unanswerable | Per-phase sidecars exist but no aggregation across cycles | `append-phase-perf.sh` reads last 5 cycles' `metrics.json` for delta columns |
| Logs accumulate forever (or never exist) | No structured logs at all | 7-day TTL on `.ephemeral/`; permanent reports unaffected |
| `phase-watchdog` false-positives on WebSearch-heavy scouts (cycle-36 stalled at 240s threshold) | mtime-based stall detection is naive | NDJSON event arrivals provide a *real* liveness signal; future watchdog can subscribe |

## Data sources

The tracker reuses the existing capture path. The pipeline (`scripts/dispatch/subagent-run.sh:880-950`) already writes three artifacts per subagent invocation:

| Artifact | Producer | Today | Phase B |
|---|---|---|---|
| `<agent>-stdout.log` | `claude -p` stdout | 1 line (final result JSON) | N lines (NDJSON stream) |
| `<agent>-usage.json` | extracted from final result | input/output/cache tokens, total_cost_usd, modelUsage[] | (unchanged) |
| `<agent>-timing.json` | runner-internal phase markers | profile_load, prep, adapter_invoke, finalize | (unchanged) |

Phase A tooling consumes these directly. Phase B switches `claude.sh`'s `--output-format` flag so `stdout.log` becomes the per-event stream.

## Folder structure

```
.evolve/runs/cycle-{N}/                               # cycle workspace (existing)
├── orchestrator-prompt.md                            # PERMANENT
├── orchestrator-report.md                            # PERMANENT
├── {phase}-report.md                                 # PERMANENT (scout, audit, etc.)
├── {phase}-stdout.log                                # PERMANENT (today: single blob; Phase B: NDJSON)
├── {phase}-usage.json                                # PERMANENT
├── {phase}-timing.json                               # PERMANENT
├── context-monitor.json                              # PERMANENT
├── cycle-digest.json                                 # PERMANENT
└── .ephemeral/                                       # 7-DAY TTL — new
    ├── trackers/                                     # raw NDJSON per invocation
    │   └── {phase}-{ISO8601}.ndjson
    ├── metrics/                                      # rollup snapshots
    │   ├── {phase}.json
    │   └── cycle-metrics.json
    └── trace.md                                      # human-readable bullet log (tailable)
```

The `.ephemeral/` naming serves two purposes: (1) the leading dot hides it from `ls` defaults; (2) it acts as a policy marker — any tool can trivially classify "safe to prune / safe to compress / safe to skip in git" by directory name alone.

## Event schema

One NDJSON line per Anthropic API event. Common envelope:

```json
{
  "ts": "2026-05-13T11:45:01.234Z",
  "cycle": 36,
  "phase": "scout",
  "invocation_id": "scout-20260513T114456Z",
  "turn": 1,
  "type": "tool_use|tool_result|message|system|result|error"
}
```

Per-type fields:

| `type` | Extra fields |
|---|---|
| `system` | `subtype` (`init`, `compact`), `session_id`, `model` |
| `message` | `role`, `text` (truncated to 200 chars), `text_full_bytes` |
| `tool_use` | `name`, `input` (jq-compacted), `tool_use_id` |
| `tool_result` | `tool_use_id`, `latency_ms` (from previous `tool_use`), `size_bytes`, `is_error` |
| `result` | `subtype`, `duration_ms`, `cost_usd`, `usage{}`, `num_turns`, `stop_reason` |
| `error` | `message`, `details` |

`latency_ms` on `tool_result` is computed by the tracker (Δ between the result's `ts` and the matching `tool_use`'s `ts`). The Anthropic CLI does not provide it natively.

## Trace.md format

One line per event. Format:

```
[HH:MM:SS] cycle-N phase   tN  <kind>  <one-line-summary>  [<latency>] [<size>]
```

Sample:

```
[11:43:14] cycle-36 orchestrator t1 START model=sonnet prompt=310 lines
[11:43:18] cycle-36 orchestrator t1 msg   "I'll start by reading inbox and state..."
[11:43:21] cycle-36 orchestrator t1 tool  Bash(cycle-state.sh get phase) → "calibrate"          [45ms]
[11:43:23] cycle-36 orchestrator t2 tool  Bash(phase-gate.sh gate_calibrate_to_research) → rc=0  [210ms]
[11:44:56] cycle-36 scout        t1 START model=sonnet prompt=128 lines (RESUME)
[11:45:04] cycle-36 scout        t2 tool  WebSearch "self-correcting AI pipelines 2026"
[11:49:16] cycle-36 scout        t2 ←     [4m12s] 18234 bytes
…
[11:58:24] cycle-36 scout           END   latency=7m33s cost=$1.14 turns=44 outcome=success
```

Properties:
- Tailable: `tail -F .evolve/runs/cycle-N/.ephemeral/trace.md`
- Greppable: `grep WebSearch trace.md` shows all web searches across all phases
- Time-aligned: per-line ISO timestamps sortable

## Cycle-metrics rollup

Generated by `rollup-cycle-metrics.sh <cycle>` from per-phase sidecars. Schema:

```json
{
  "cycle": 36,
  "wall_clock_start": "2026-05-13T11:43:14Z",
  "wall_clock_end":   "2026-05-13T12:13:34Z",
  "total_wall_ms":    1820000,
  "total_cost_usd":   4.21,
  "phases": [
    {"phase": "calibrate", "latency_ms": 12000,  "cost_usd": 0.02, "turns": 0,  "verdict": "ok"},
    {"phase": "scout",     "latency_ms": 244000, "cost_usd": 1.14, "turns": 44, "verdict": "ok"},
    {"phase": "triage",    "latency_ms": 89000,  "cost_usd": 0.18, "turns": 12, "verdict": "ok"}
  ],
  "models_used": ["sonnet-4-6", "haiku-4-5"],
  "model_cost_split": {"sonnet-4-6": 1.139, "haiku-4-5": 0.008},
  "cache_hit_rate": 0.96,
  "hot_spots": [
    "scout: 244s (60% of cycle wall time)",
    "scout/web_search_requests: 1 call"
  ]
}
```

## Phase-report integration

`append-phase-perf.sh <cycle> <phase>` appends a `## Performance & Cost` section to the phase report. Idempotent (re-running replaces an existing section, never duplicates). Section format:

```markdown
## Performance & Cost

| Metric | This cycle | vs cycle-{N-1} same phase | vs 5-cycle baseline |
|---|---|---|---|
| Wall time | 7m 33s | +47% (was 5m 08s) | +28% |
| Cost | $1.14 | -38% (was $1.84) | -22% |
| Turns | 44 | -10% (was 49) | +5% |
| Cache hit rate | 96% | +2pp | +8pp |
| Models | sonnet+haiku | (same) | (same) |
```

Tool-call breakdown table is only populated in Phase B (after `--output-format stream-json` is wired). In Phase A, the section shows just the top-level metrics + baseline comparison.

## TTL policy

| Path | Retention | Pruner |
|---|---|---|
| `.evolve/runs/cycle-*/.ephemeral/` | **7 days** | `prune-ephemeral.sh` |
| `.evolve/runs/cycle-*/*.md` (reports) | permanent | never |
| `.evolve/runs/cycle-*/*.json` (sidecars) | permanent | never |
| `.evolve/dispatch-logs/*.log` | **30 days** | same pruner |
| `.evolve/ledger.jsonl` | permanent (append-only) | never |

Pruner uses `find -mtime +N` for bash 3.2 compat. Idempotent. `--dry-run` flag for safe inspection.

## Two-phase rollout

| Phase | Status | Surface | Lives In |
|---|---|---|---|
| **A — additive** | shipped v10.4.0 | All new scripts; works on existing data via replay mode | `scripts/observability/*.sh`, `scripts/tests/tracker-writer-test.sh`, this doc |
| **B — wire live** | shipped v10.5.0 (opt-in via `EVOLVE_TRACKER_ENABLED=1`) | `subagent-run.sh` post-phase replays NDJSON through `tracker-writer` + `append-phase-perf`; `run-cycle.sh` calls `rollup-cycle-metrics` at cycle exit; `.claude/settings.json:hooks.Stop` runs `prune-ephemeral` | `scripts/dispatch/subagent-run.sh`, `scripts/dispatch/run-cycle.sh`, `.claude/settings.json` |

> **Note** — `claude.sh` already emits `--output-format stream-json --verbose` by default (since v9.2.0, gated by `EVOLVE_STREAM_JSON`, default ON). The original Phase-B plan called for flipping that flag; turned out it had already been flipped two minor versions earlier. v10.5.0 just wires the consumer side.

**Promotion path** (v8.55.0 verify→default-on ladder):

| Version | `EVOLVE_TRACKER_ENABLED` default | Criterion |
|---|---|---|
| v10.5.0 | `0` (opt-in) | Operators validate the replay model under real cycles |
| v10.6+ candidate | `1` (default on) | One verification cycle confirms no WARN spam + no per-cycle overhead > 1¢ |

## File index

| File | Role |
|---|---|
| `scripts/observability/tracker-writer.sh` | stdin NDJSON → `.ephemeral/trackers/*.ndjson` + `.ephemeral/trace.md` + tally |
| `scripts/observability/rollup-cycle-metrics.sh` | per-phase sidecars → `.ephemeral/metrics/cycle-metrics.json` |
| `scripts/observability/append-phase-perf.sh` | phase report + sidecars + baseline → "Performance & Cost" appendix |
| `scripts/observability/show-trace.sh` | `trace.md` pretty-printer with `--watch`, `--cycle`, `--phase` flags |
| `scripts/observability/prune-ephemeral.sh` | 7-day TTL pruner |
| `scripts/tests/tracker-writer-test.sh` | synthetic NDJSON fixtures + assertions |
| `docs/architecture/phase-tracker.md` | this doc |

## Related infrastructure

| Existing | What it does | Relationship to tracker |
|---|---|---|
| `scripts/observability/show-cycle-cost.sh` | Per-phase cost table from `*-stdout.log` | Complementary — same data source, different view |
| `scripts/observability/show-context-monitor.sh` | Context window utilization tracking | Different concern (memory pressure, not time/cost) |
| `scripts/observability/verify-ledger-chain.sh` | Tamper-evident integrity check | Different concern (integrity, not performance) |
| `scripts/dispatch/subagent-run.sh:880-950` | Writes `*-stdout.log`, `*-usage.json`, `*-timing.json` | Source data for everything in this design |

## Bash 3.2 compatibility

All scripts follow the project's bash 3.2 contract:

- No `declare -A` — use parallel indexed arrays or temp files
- No `mapfile` / `readarray` — use the `mapfile_compat` pattern from `show-cycle-cost.sh`
- No GNU-only sed flags — write to `.tmp.$$` and `mv`
- No GNU-only `date -d` — fallback chain `gdate || date -d || date -j -f`

Atomic writes via `mv-of-temp`. Append-style NDJSON writes use single `>>` (atomic for ≤PIPE_BUF lines on POSIX).
