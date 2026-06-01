# ADR-0026: Self-Healing Review Layer (review-before-stop)

> A pipeline stop condition (timeout, error, non-zero exit) becomes an **observation that triggers a review**, not a silent kill. A reviewer examines the evidence and justifies the next move: extend, pause-for-investigation, or stop. Shipped Stage 0 (artifact-timeout path, deterministic reviewer); Stage 1 extends the same seam to every stop point + an LLM/orchestrator reviewer.

- **Status:** Accepted — Stage 0 shipped; Stage 1 backlog (loop-built)
- **Date:** 2026-05-27
- **Supersedes/relates:** extends the auto-respond `extend_timeout` primitive (ADR-0023) and the phase-observer stall detector; complements `failure-adapter` (PROCEED/RETRY/BLOCK).

## Context

The `*-tmux` artifact wait used a hard wall-clock deadline (`tmuxArtifactTimeoutS = 300`): if a phase did not produce its artifact within 300s **total**, the bridge returned `ExitArtifactTimeout` (exit 81) and the cycle died.

This conflates two different states:

| State | Old behavior | Correct behavior |
|---|---|---|
| Agent still producing output, just slow | **Killed at 300s** ❌ | Keep waiting |
| Agent genuinely stuck (no output) | Killed at 300s ✓ | Surface for investigation |

Incident (cycle 109, 2026-05-27): a research-heavy ultrathink Scout streamed output continuously (`Deliberating… 5m 2s · ↓ 10.0k tokens`) and was killed at exactly 300s mid-synthesis — `exit=81`, `total_cost_usd: 0` wasted, whole batch dead. A pipeline that dies on the first slow-but-productive phase cannot "run for long hours."

## Decision

Stop conditions flow through one review layer modelled in four steps:

```
Observe   → StopEvent  (evidence envelope: kind, phase, elapsed, interval, attempt, progressed, stdoutTail)
Review    → StopReviewer.Review  (deterministic now; LLM/orchestrator later)
Translate → ReviewVerdict  {extend | pause | stop}
Execute   → caller applies + logs the justification to the self-healing trail
```

The reviewer is a seam (`StopReviewer` interface), so the loop wiring never changes as reviewers grow smarter.

## Stage 0 — shipped (this slice)

- **`go/internal/bridge/stopreview.go`** — `StopKind`, `StopEvent`, `ReviewAction`, `ReviewVerdict`, `StopReviewer` interface, `deterministicReviewer`, `envInt`.
- **Artifact wait converted to interval-review** (`driver_tmux_repl.go`): every `interval` seconds without the artifact, capture the pane, set `Progressed = pane changed since the interval baseline`, call `reviewer.Review(StopEvent{…})`. `extend` → reset baseline + advance `attempt` + keep waiting; otherwise → `ExitArtifactTimeout`.
- **Deterministic reviewer:** extend while output progresses, up to `maxExtends` (default 6); else pause. Never kills a producing agent within the backstop.
- **Knobs:** `EVOLVE_ARTIFACT_TIMEOUT_S` (review interval, default 300), `EVOLVE_ARTIFACT_MAX_EXTENDS` (backstop, default 6), `cfg.ArtifactTimeoutS` (per-launch override). Backstop ≈ `maxExtends × interval` (~30 min default).
- **Hardening:** loop honours `ctx.Done()` (cancellation pre-empts the extend budget); `reviewer` self-defaults when nil (direct callers safe).

### Known Stage-0 limitations (Stage 1 closes these)

- **Spinner = progress.** `Progressed` is a full-pane diff, so an animated spinner/clock reads as progress; only `maxExtends` bounds a spinner-stuck agent. Stage 1's reviewer inspects `StdoutTail` (and line-diff) to disambiguate.
- **Single stop kind.** Only `StopArtifactTimeout` is wired. Non-zero exit, launch error (e.g. exit 81 vs REPL-boot timeout), and audit-block still bypass the review layer.
- **Deterministic only.** No LLM judgment; `pause` and `stop` collapse to the same `ExitArtifactTimeout`.

## Stage 1 — backlog (loop extends the seam, TDD)

1. Route **all** pipeline stop conditions through `StopEvent`/`StopReviewer` (non-zero exit, launch error, audit block, quota wall) — one review layer, no per-kind mechanism.
2. **LLM/orchestrator reviewer** behind `StopReviewer`: judge "stuck vs working" from `StdoutTail` + partial artifacts (Rule 5 — deterministic fast-path first, AI judgment for the ambiguous case).
3. Distinct **`pause` semantics**: checkpoint + write an investigation report rather than reusing `ExitArtifactTimeout`; let the orchestrator decide retry/adapt/abandon.
4. **SHIPPED (cycle-186)**: Sharper progress signal (strip volatile spinner lines) so spinner animation no longer reads as progress. Implementation: `PaneHasSubstantiveChange` in `stopreview.go`, wired into the driver's wait loop.
5. **SHIPPED (cycle-188)**: Emit the verdict + justification to the ledger as an auditable self-healing trail. Implementation: `Deps.OnStopReview` callback in `engine.go`; driver calls it (nil-safe) at each review checkpoint; orchestrator wires it to `ledger.Append(LedgerEntry{Kind:"stop_review", Action:action, Message:reason})`; `checkSelfHealEvents` in `cyclehealth.go` flags `action=pause` as `SeverityWarn`.

## Consequences

- **+** A slow-but-productive phase is no longer killed; the pipeline survives long phases → "run for long hours."
- **+** Genuine stalls are still caught (deterministic pause within one idle interval; backstop on noisy-stuck agents).
- **+** The seam is the single extension point for all future self-healing.
- **−** A spinner-stuck agent now takes up to `maxExtends × interval` (~30 min) to surface vs. instant under the old wall-clock — accepted for Stage 0, closed by Stage 1 #4.
- **−** Two new env knobs; documented in CLAUDE.md current-behavior table.
