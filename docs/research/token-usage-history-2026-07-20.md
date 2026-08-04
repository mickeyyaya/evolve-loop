# Token-usage history analysis — cycles ≥900 (2026-07-20)

Source: 719 per-phase `*-usage.json` artifacts under `.evolve/runs/cycle-9xx/` (of 7,041 total on disk),
aggregated 2026-07-20 after the v22.5.0 release. Native-Go phases (ship) correctly record zero.

## Where the context volume goes

| phase | runs | output tok | cache-read tok | avg min | retries | unmeasured |
|---|---|---|---|---|---|---|
| build | 70 | 1.52M | **195.2M** | 7.4 | 2 | 7 |
| tdd | 71 | 2.18M | **153.9M** | 7.6 | 0 | 0 |
| test-amplification | 15 | 0.96M | **81.2M** (5.4M/run!) | 13.5 | 0 | 0 |
| coverage-gate | 38 | 0.59M | 63.3M | 4.9 | 0 | 0 |
| scout | 80 | 0.57M | 54.9M | 2.5 | 0 | 3 |
| bug-reproduction | 35 | 0.44M | 44.5M | 3.5 | 0 | 1 |
| retro | 44 | 0.82M | 43.3M | 6.1 | 0 | 2 |
| fault-localization | 47 | 0.37M | 39.0M | 2.7 | 0 | 0 |
| audit | 52 | 0.29M | 23.0M | 4.0 | **12** | **22 (42%)** |
| adversarial-review | 31 | 0.20M | 11.3M | 3.1 | 0 | 6 |
| triage | 75 | 0.04M | 4.6M | 1.6 | 0 | **25 (33%)** |
| memo | 16 | 0.04M | 5.5M | 1.5 | 0 | 2 |

Totals: cache-read **768.3M**, output 8.5M. Cache read:write ratio 21:1 (healthy prompt-cache reuse).
Unmeasured: **173/719 artifacts (24%)**.

## Findings, ranked by leverage

1. **Blast-radius context scoping is the #1 lever.** build+tdd+scout+audit = ~55% of all cache-read
   volume, and all four re-read whole-subsystem context per run. The codegraph item
   (`codegraph-blast-radius-context-for-scout-audit-review`, design: `docs/architecture/code-graph-blast-radius.md`)
   directly targets this; the reference implementation claims ~82× median token reduction on the
   review path. Operator directive 2026-07-20: build it.
2. **test-amplification is the per-run outlier**: 5.4M cache-read/run, 13.5 min avg — 3.5× build's
   per-run context on an advisor-inserted optional phase. Scope its input to the diff's covering
   tests (a codegraph S3 consumer) or cap its corpus until then.
3. **The telemetry blind spot went LIVE** (was latent per the 2026-07-17 investigation): agy-tmux
   now completes real successful phases — triage 25/75 unmeasured, adversarial-review 6/31 — and the
   engine WARNs `token usage uncovered for driver "agy-tmux"` on live batches. Separately, audit is
   42% unmeasured **on claude-tmux** — the retry/relaunch path loses transcript attribution
   (12 retries across 52 runs). Tripwire item escalated; per-CLI collector + relaunch attribution now justified.
4. **Audit retry tax**: 12 relaunches × ~0.44M cache-read each ≈ 5M wasted context; ties to the
   queued audit-artifact-timeout item.
5. **What's already good**: memo replaces retro on PASS cycles at ~1/8th the context (0.35M vs 1.0M
   per run); cache reuse ratio is healthy; ship/native phases burn zero LLM tokens.

## Actions queued (2026-07-20)

- `codegraph-blast-radius-context-for-scout-audit-review` → 0.90 (operator build directive; measured target: >50% of context volume).
- `telemetry-coverage-tripwire-nonclaude-success` → 0.90 + live-gap evidence (premise update: collectors now justified, not just the tripwire).
- NEW `test-amplification-context-scope` → 0.84 (below the pipeline-integrity band by rule; first-in-line efficiency item).
