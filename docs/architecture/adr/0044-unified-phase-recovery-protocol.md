# ADR-0044: Unified Phase Recovery Protocol — deterministic-first, single-owner recovery

> Status: **Accepted — C1 implemented** (designed 2026-06-09; implementation started 2026-06-10, slice record in
> §Implementation). Design-first: records the analysis and chosen direction; implementation is
> risk-ranked and gate-tested (no blind change to the hot dispatch loop). Full evidence + design:
> [phase-recovery.md](../phase-recovery.md). Builds on [ADR-0026](0026-self-healing-review-layer.md) (stop-reviewer),
> [ADR-0029](0029-cli-fallback-chain-and-per-agent-overrides.md) (CLI fallback), [ADR-0039](0039-failure-floor-and-failure-signal-contract.md) (ship repair ladder + failure floor).

## Context

Post-mortem of cycle-262 (2026-06-09): a cycle whose work was **done correctly** recorded itself as a
**failure**. Three failures stacked — codex self-upgraded its binary mid-phase (D6), retro was dispatched with
an invalid `claude --model auto` (D3), and the build's **successful** CLI fallback (`codex exit 81 → claude
exit 0`, valid `build-report.md` PASS, goal achieved) left **no record at all** (no `build-usage.json`, no
`phase-timing` entry, no verdict, not even `PhasesRun` membership) → audit skipped → cycle FAIL (D1). ~40 min
of a ~52 min cycle was burned waiting out the `maxExtends` backstop on two **self-describing fatal pane
states** that nothing detects (D2). Full taxonomy (D1–D7) in the design doc.

**D1 mechanism located (2026-06-10, Slice 1 locate-the-fork — supersedes the post-mortem's first guess):**
the runner's fallback chain DID reconcile correctly (Classify ran on claude's deliverable, the runner returned
PASS with a nil error). The record was lost downstream: the fallback builder wrote the tracked config
`.evolve/commit-prefix-scope.json` into the **main tree** (and `recoverBuildLeak` deliberately never relocates
`.evolve/` paths), so the post-phase **tree-diff guard correctly aborted** the cycle — and that abort path,
like *every* abort path between `runner.Run` returning and the orchestrator's single happy-path recording
site (~9 paths: review-gate reject, correction failures, ship-error recovery, leak-recovery failure,
tree-guard, ledger/state persistence failures), returned **without recording the phase outcome**. The guard
was right; erasing the evidence was the defect.

**Root cause:** recovery is an **un-owned cross-cutting concern**, smeared across `bridge/driver_tmux_repl.go`,
`phases/runner`, `core/orchestrator.go`, `core/observer.go`, and `phases/ship` (repair ladder). Each layer
assumes another catches the failure; none owns *"did this phase succeed, and is that success recorded?"* — so
the record can diverge from reality.

## Decision

Adopt a **Unified Phase Recovery Protocol** with one owner (a *Phase Recovery Pipeline*), governed by
**deterministic-first, LLM-last** (Core Agent Rule 5) and **outcome-orientation** (reconcile to a recorded
verdict, never "process exited 0"). Built from standard patterns:

| Ref | Mechanism | Pattern | Fixes |
|---|---|---|---|
| **C1** | Single-source outcome recording — every terminal disposition of a dispatched phase (happy advance AND each abort return) funnels through one `recordPhaseOutcome` chokepoint producing a `recovery.PhaseOutcome` (verdict / cost / duration / attempts / abort_reason) | DRY chokepoint in the orchestrator (as-located; not the runner) | **D1** |
| **C2** | `ClassifyTerminal(pane, exit, artifact) → TerminalCause`; known-fatal causes fast-fail; `ModelFlagPolicy` normalizes `auto` per driver | Template Method hook + Strategy | **D2, D3** |
| **C3** | Recovery handlers in an ordered chain: `ModelFlagPolicy → FatalPaneDetector → Fallback → VerdictReconciler → LLMFailureAdvisor` | Chain of Responsibility | composes **D1–D4** |
| **C4** | Observer stays detection-only; a `StallPolicy` maps events → `extend\|kill+retry\|escalate` | Observer + Strategy/Policy (SRP) | **D5** |
| **C5** | Preflight predicate: any `-tmux` CLI with `startup update check = true` must be pinned (converge) or Halt | Specification | **D6** |

The **LLM failure-advisor is the escalation tail only** (unclassified states); its classification of a novel
pane signature is **promoted into the deterministic registry** `FatalPaneDetector` reads — the known-failure
set grows, the LLM never sits on the hot path for a known failure.

Implementation is risk-ranked (design doc §7): **C1 first** (alone it makes 262-class record divergence
structurally impossible — note the faithful 262 replay still *fails the cycle*, correctly, on the genuine
worktree leak; what C1 guarantees is that the build's PASS verdict, cost, and timing survive into the record
so the salvage needs no forensic reconstruction), then C2 (model-flag + fatal-detector), then C5+D4 (config),
C4, and finally the C3 chain refactor that composes them.

## Implementation record (per slice)

- **Slice 1 / C1 — shipped 2026-06-10.** New leaf package `go/internal/recovery/` (`PhaseOutcome`, the
  single-source outcome envelope; leaf constraints mirror `internal/router` — never imports core/bridge,
  no verdict constants). `core.phaseOutcomeFrom` owns the reconciliation rule (canonical agent verdict
  recorded as-is; anything else synthesizes FAIL — a synthesized PASS is structurally impossible);
  `core.(*Orchestrator).recordPhaseOutcome` is the chokepoint, called exactly once on every terminal path of
  `RunCycle`'s phase iteration: happy advance, exhausted bridge retries, non-canonical verdict, both
  correction-loop failures, both review-gate rejects, ship-error recovery, worktree-leak recovery failure,
  tree-diff guard abort (the cycle-262 path), ledger-append failure, post-phase state-write failure, plus the
  failure-learning retro record (deduplicated through the same chokepoint). `phase-timing.json` and
  `<phase>-usage.json` gain an additive `abort_reason` (omitempty — happy-path artifacts byte-identical);
  aborted-but-dispatched phases now appear in `PhasesRun` (generalizing the existing failure-learning
  precedent that already appended retro). Tests: `orchestrator_phaseoutcome_test.go` (faithful 262 replay
  incl. tracked-`.evolve/`-leak + guard abort; abort-path table; never-invents-PASS; single-chokepoint pin)
  + `TestRun_FallbackOnArtifactTimeout_CarriesVerdictCostDuration` (runner-level baseline pin).
  **Known deferred debt:** `resume.go` is a second, simpler recording boundary (no timings/sidecars at all);
  unify it through the chokepoint in the C3 slice. `PhasesRun` consumers audited: printing/telemetry +
  routingtest only; no gate reads it.

- **Slice 2 / C2 — shipped 2026-06-10.** Two halves. (a) `recovery.FatalPaneDetector` (the deterministic
  registry, `internal/recovery/detector.go`): typed `TerminalCause` vocabulary
  (model_invalid / cli_self_updated / dead_shell / unknown), ordered first-match-wins substring signatures
  seeded from the real cycle-262 pane text; consulted by the bridge's stop-review checkpoint via the
  `fatalPaneVerdict` seam (`bridge/fatalpane.go`) BEFORE the reviewer — because the dead panes read as
  "progressed" (the bridge's own nudge echoed into them), the check must preempt the
  extend-while-progressing flow. Stage-gated by `EVOLVE_PHASE_RECOVERY` (off | shadow [DEFAULT] | enforce;
  unknown→off; bridge reads env directly, subprocess pattern): shadow logs the would-be fast-fail only;
  enforce returns `ReviewStop` → the wait exits in ONE interval and exit-81 hands the phase to the runner's
  fallback chain (cycle-262's ~20-min burns become ~5 min each). A Busy pane is never preempted (prime
  directive: never kill a working agent); the nudge is now gated on `ReviewPause` (behavior-identical for
  the legacy reviewer, which never emits stop) so a fatal verdict is never nudged; Stop also writes the
  escalation report. (b) `ModelFlagPolicy` at the realizer chokepoint (`realizer.go` `realizeScalar`):
  post-resolution `model_tier == "auto"` emits NO model param on ANY channel (flag or repl) — matrix-wide,
  one guard for claude-tmux/codex-tmux/every future manifest (the headless codex driver keeps its own
  exec-path guard). Always-on correctness, not stage-gated: `--model auto` is fatal for every CLI.
  **Deviation from the design sketch:** the optional runner-`Hooks` `ClassifyTerminal` extension was NOT
  added — nothing consumes it yet (the bridge checkpoint + realizer policy are the load-bearing pieces);
  it rides with C3 when the chain needs it. Tests: `recovery/detector_test.go` (real-262-fixture seeds,
  first-match-wins, healthy/empty-pane negatives), `bridge/fatalpane_test.go` (enforce-preempts /
  shadow-neutral-logs / busy-never / off-skips / stage parse), `bridge/realizer_modelpolicy_test.go`
  (auto-omits on both channels, concrete tiers + raw names unaffected).

## Consequences

- **Positive:** a successful recovery is *structurally* recorded (D1 can't recur); self-describing fatal states
  fast-fail instead of burning the `maxExtends` backstop (D2); recovery gains a single testable owner; the
  deterministic catch-set self-expands via the LLM tail.
- **Constraint (non-negotiable):** phase isolation — no cross-phase REPL/context reuse; builder≠auditor
  cross-family floor unchanged.
- **Constraint:** reconciliation only ever upgrades a synthesized FAIL toward the agent's real verdict; it never
  invents a PASS. A valid deliverable with red EGPS predicates still fails the gate.
- **Discipline:** no blind change to the hot dispatch loop — each step is TDD'd, adversarially audited, and
  behavior-neutral where possible.

## Alternatives considered

- **Per-cycle `brew pin`/`unpin`-on-exit.** Rejected as primary: unpin depends on a clean exit (a kill/OOM/reboot
  never runs it → host CLI silently frozen); `brew pin` is global + persistent, so concurrent cycles race.
  Superseded by C5 (pin as convergent steady state; updates are an explicit operator action).
- **Raise `maxExtends` / lower the artifact timeout.** Treats the symptom, not the untyped-terminal-state cause.
- **Route all failures through the LLM failure-adapter.** Violates deterministic-first; the LLM retro itself
  failed in cycle-262.
- **Add CLI fallbacks everywhere and stop.** Insufficient without C1 — an unrecorded *successful* fallback still
  fails the cycle.
