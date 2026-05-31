# Self-Healing Gaps & Hardening (cycle-164 / multi-CLI run)

Status: living analysis. Source: the multi-CLI validation run (cycles 154-164) + a
full trace of the recovery control-flow in `go/internal/core/orchestrator.go`.

## Why this exists

The pipeline already has recovery machinery — audit-FAIL → `retro` → failure-adapter
(PROCEED/RETRY/BLOCK); ship-error → a 4-link recovery chain (`router/recovery.go`) →
`debugger`/re-audit/retry-ship. But a `RunCycle` hard error **stops the whole batch**
(this is why a single phase failure aborted runs 154-162). So every place RunCycle
returns an error instead of routing to recovery is a potential pipeline-blocker.

Research alignment (2025 self-healing / self-debugging agent literature): the resilient
pattern is *analyze context → decide retry / reroute / escalate*, adaptively modifying
the workflow in-flight rather than aborting; corrective feedback is fed back to condition
the retry; outcomes update the knowledge base (lessons). The caveat: an imprecise
evaluator/retry can reinforce bad patterns and **burn tokens** — so retries must be
narrowly scoped to genuinely transient failures, never a model's real FAIL.

## Ranked hard-abort gaps (where a blocker escapes self-healing)

| # | Site (orchestrator.go unless noted) | Trigger | Recoverable? | Fix posture |
|---|---|---|---|---|
| 1 | attempt loop `return ... "phase %s: %w"` | any non-ArtifactTimeout bridge error on a mandatory phase | yes (transient class) | **DONE** — cycle-173: transient bridge failures (exits 80/85/86) are classified and retried up to phaseMaxAttempts |
| 9 | `retro.go` bridge-fail returns error | retro's OWN bridge dies | yes | **DONE** — return FAIL verdict + nil error → routes via `decideAfterRetro` |
| 5 | `return ... "non-canonical verdict"` | runner returns verdict ∉ {PASS,FAIL,WARN,SKIPPED} (parse blip) | yes | **DONE** — cycle-173: non-canonical verdicts are classified and retried up to phaseMaxAttempts |
| 3 | `router/recovery.go` integrity-block | integrity-class ShipError (e.g. INTEGRITY_TREE_DRIFT false positive) | sometimes | route through `debugger` deep-dive before BLOCK |
| 2 | `phaseMaxAttempts=2`, no backoff | repeated ArtifactTimeout | partly | optional: backoff/third attempt |
| 4 | `maxRecoveryDepth=2` then abort | persistent ship blocker | by design | keep cap; escalate to operator notice |
| 6 | tree-diff guard / `recoverBuildLeak` false | unrecoverable leak | correctness guard | keep (bugs #5/#6 already hardened recoverBuildLeak) |
| 7 | reviewer reject (noop default) | future reviewer trips | n/a today | add retry budget before any real reviewer ships |
| 8 | state-machine transition error | unknown verdict edge | programmer error | keep (guards a bug, not runtime) |

## Principle for fixes

A failure in the **failure-handler** (retro, debugger) must never be fatal (GAP 9).
A **transient** infra/bridge failure should retry-or-reroute, bounded (GAP 1/5). A
**genuine** model FAIL or correctness-guard breach must still stop — recovery is for
transient/infra faults, not for masking real failures (token-optimization + the
"imprecise-evaluator" caveat).

## Multi-CLI note

Per-phase CLI is resolved via `EVOLVE_<AGENT>_CLI` > `EVOLVE_CLI` > profile.cli >
`claude-tmux`, with a `cli_fallback` chain on trigger exits `[80 81 124 127]`
(`runner/cli_chain.go`). A non-Claude CLI on any phase functions as long as (a) its
manifest/flags are correct (bugs at 154), (b) its writes are confined or recovered
(bugs #4/#5/#6 — `recoverBuildLeak`), and (c) the fallback chain is populated.
