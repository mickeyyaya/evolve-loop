# Recommendation — token-frontier-watch

Every number below cites `assumptions-and-evidence.md`. The dossier with the full re-survey and the
per-family verdicts is
[frontier-watch-2026-09-26.md](../../docs/research/token-optimization-2026/frontier-watch-2026-09-26.md)
(`docs/research/token-optimization-2026/frontier-watch-2026-09-26.md`).

## Per-family verdicts (identical to the dossier)

- **Family A, soft-prompt / KV compression on the self-hosted ollama lane: DEFER.** The lane had 0
  successful dispatches in 103 cycles (E1, E2). Ollama ships no KV import/export (E4). The persistence
  PR is open and deprioritized (E5). The counterfactual ceiling is ≈2.0 s per dispatch at 8B and ≈0.06%
  of fleet tokens, none of them billed (derived figures, A2, A6).
- **Family B, latent inter-agent channels: DEFER.** All latent work is white-box (E10). The 2026-07/08
  causal audits show conditional, partly spurious gains (E11, E12). Relayed KV cannot be inspected
  (E13). Vendor compaction is same-vendor and either text or opaque (E14).

No verdict is BUILD, so no inbox record is emitted.

## Options Compared

| Criterion | Option 1: DEFER on triggers | Option 2: BUILD ollama KV spike now | Option 3: retire and redirect |
|---|---|---|---|
| Tokens moved now | 0 (lane idle, E2) | 0 billed; ≤7,464 local prefill tokens per counterfactual dispatch (E7) | 0 |
| Latency moved now | 0 ms | ≈0.76–2.0 s per reload, only if routed (derived, A2, A6) | 0 ms |
| Cost | ≈1 small document cycle per quarter (A4) | 1–2 build cycles plus a driver change plus a fork or server swap (E3, E5, E6) | none |
| Source risk | none | fork of an unmerged PR; REPL→HTTP driver rewrite | none |
| Detects an applicability flip | yes, within one interval (T-A1/T-A2, T-B1–T-B3) | only for family A | no |
| Integrity floor (auditor-readable handoffs) | preserved | preserved | preserved |
| Fits this task's research-only scope (AC3) | yes | no, needs source changes | yes |

The deciding facts are E2 and E5. There is no traffic to optimize and no shipped serving surface to
optimize with, so Option 2 has an expected value of about zero at a real cost. Option 3 is cheaper than
Option 1 by only one small cycle per quarter (A4). It gives up flip detection in a field that produced
five relevant papers in three months (E10–E13) and has an open upstream PR (E5).

## Recommendation

**Adopt Option 1: DEFER both families and re-check on the observable triggers.**

- **Winner:** Option 1. It costs almost nothing and keeps flip detection, and each trigger is a probe
  that one command can answer (PR state, telemetry share, changelog scan).
- **Runner-up:** Option 3, if two consecutive re-checks find the measured ollama share (E1) and the
  PR #17953 state (E5) unchanged.
- **Evidence that would flip the choice to Option 2 (BUILD):** both T-A1 (an ollama release ships
  persistent KV or a public KV API; today E5 is open and unmerged) and T-A2 (at least 5% successful
  ollama-tmux dispatches over 20 cycles; today it is 0.08% with 0 successes, E1 and E2). For family B,
  any of T-B1 to T-B3 flips it: a fleet CLI imports another model's compressed or latent context, a
  cross-vendor protocol exists, or a homogeneous self-hosted pair has an auditor-readable payload (E14).
- **Honest limits:** A1 (ollama reuses the prefix across REPL sessions), A2 (linear prefill scaling from
  3B to 8B) and A6 (linear interpolation in prompt length between the two PR #17953 rows that bracket the
  triage prompt) are unmeasured. The literature and vendor check read abstracts and docs, and nothing was
  exercised. None of these limits changes the verdict. Even with 0 s restore overhead and the 8B scaling, the
  win stays under the ≈2.6 s cold prefill of one dispatch on a lane that carries no traffic.
