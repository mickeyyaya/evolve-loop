# Phase Recovery — why "self-healing" let a successful cycle record itself as a failure

> Decision record: [ADR-0044](adr/0044-unified-phase-recovery-protocol.md).
> Companion incident: cycle-262 (2026-06-09), run dir `.evolve/runs/cycle-262/`.
> Related: [ADR-0026](adr/0026-self-healing-review-layer.md) (stop-reviewer), [ADR-0029](adr/0029-cli-fallback-chain-and-per-agent-overrides.md) (CLI fallback), [ADR-0039](adr/0039-failure-floor-and-failure-signal-contract.md) (failure floor + ship repair ladder), [pipeline-latency.md](pipeline-latency.md) (sibling: boot latency).

This document is the full design record behind the **Unified Phase Recovery Protocol**. It is grounded in
a real post-mortem (cycle-262), not a hypothetical: a cycle whose work was **done correctly** and then
**recorded as a failure**. The investigation exposed that the loop's "self-healing" is a set of disconnected
point-fixes, not a designed protocol — and shows precisely where the seams leak.

## 1. The request

While measuring pipeline latency (ADR-0043 A0 boot_ms), a measured cycle (262) was run with a trivial,
real goal: register a `chore(build)` entry in `.evolve/commit-prefix-scope.json`. The cycle **failed**.
The operator asked three questions, each answered by evidence below, and then asked for a solution from an
**AI-driven design** and a **clean-code / design-pattern architecture** perspective. This doc is that answer.

## 2. What actually happened (the cycle-262 incident)

Phase chain, reconstructed from `.evolve/runs/cycle-262/` artifacts + the orchestrator log:

```
scout(claude/sonnet) PASS ─► tdd(claude/opus) PASS ─► build[codex exit 81 → claude exit 0] ─► retro(claude/auto) FAIL
                                                                                  ▲                          ▲
                                                                          AUDIT SKIPPED              cycle FAIL
```

Three independent failures stacked into one cycle death:

### 2a. codex self-upgraded its own binary mid-phase (D6)
`codex doctor` reports `startup update check = true` and `update action = brew upgrade --cask codex`.
`~/.codex/version.json` shows `last_checked_at = 2026-06-09T06:24:42Z` (cycle start) and
`dismissed_version = 0.137.0`. Because the available `0.138.0 > 0.137.0`, the dismissal no longer applied,
so on the build-phase launch (14:33:06) codex auto-ran `brew upgrade --cask codex` (`/opt/homebrew/Caskroom/codex/0.138.0/`
created 14:33), printed `"🎉 Update ran successfully! Please restart Codex"`, and **exited**. The bridge then
nudged a bare `zsh` shell (`command not found: Please`). This is interactive-TUI-only behavior — the headless
`codex exec` path (`driver_codex.go`) would not trigger it; the loop's builder uses the `codex-tmux` REPL path.

### 2b. retro was dispatched with an invalid model (D3)
The retro pane captured the fatal state: `"There's an issue with the selected model (auto). It may not exist
or you may not have access to it. Run /model to pick a different model."` — `Brewed for 0s`. scout/tdd got
concrete models (`claude --model sonnet`, `claude --model opus`) from per-phase routing and worked; **retro was
never assigned a concrete model**, so it fell through to the literal `auto`, and the `claude-tmux` driver passed
`--model auto` verbatim. `auto` is a *settings* concept, **not a valid `claude --model` CLI value**. The codex
driver guards exactly this (`driver_codex.go:48` — `case resolved=="" || resolved=="auto": omit`); the
`claude-tmux` driver has **no equivalent guard**.

### 2c. the successful build was never recorded — so the cycle "failed" (D1, the critical one)
The codex build attempt hit `exit 81` (artifact timeout); the configured CLI fallback to `claude-tmux`
**succeeded** (`dispatch chain: codex-tmux=81 -> claude-tmux=0`), wrote a valid `build-report.md` (`**Status:**
PASS`), and **actually accomplished the goal** (`chore(build)` is present in the manifest). Yet:
- there is **no `build-usage.json`**,
- **no `build` entry in `phase-timing.json`** (only `scout`, `tdd`, `retro`),
- **no recorded build verdict**.

So the orchestration spine saw `build` as not-completed, **skipped audit** (`ship ⇒ build ∧ audit` could not
advance), routed to retro as a failed cycle, and the failure-learning logged `cycle-262-failed-build`. **The
work was done; only the recording was missing.**

**Mechanism located (2026-06-10, Slice 1 locate-the-fork — supersedes the paragraph above's implicit guess
that the fallback wasn't reconciled):** the loop log proves the runner's fallback chain worked end-to-end
(`dispatch chain: codex-tmux=81 -> claude-tmux=0`; Classify ran; the runner returned PASS with nil error).
What erased the record: the claude fallback builder wrote the tracked config
`.evolve/commit-prefix-scope.json` into the **main tree** instead of its worktree; `recoverBuildLeak`
deliberately never relocates `.evolve/` paths (they're normally runtime state, orchestrator.go:622), so the
post-phase **tree-diff guard correctly aborted the cycle** (`leaked paths: [.evolve/commit-prefix-scope.json]`
is the cycle's final error) — and that abort path, like *every* abort path between `runner.Run` returning and
the orchestrator's single happy-path recording site (~9 paths: review-gate reject, correction failures,
ship-error recovery, leak-recovery failure, tree-guard, ledger/state persistence failures), returned
**without recording the outcome** — no timing entry, no usage sidecar, no `PhasesRun` membership. The guard
was right; erasing the evidence was the defect. (Adjacent finding, not D1: the guard system has a blind spot
for builders whose *legitimate deliverable* is a tracked `.evolve/` config — relocation skips it by design,
so such a build can only ever abort. Tracked as a C3-era question.)

### Cost of the two slow-fails
codex-dead (2a) burned ~20 min and retro-model-error (2b) burned ~20 min — each ran to the `maxExtends`
backstop (`tmuxArtifactTimeoutS=300` × ~4) because **nothing recognizes a self-describing fatal pane state**.
~40 min of a ~52 min cycle was spent waiting on failures that were deterministically detectable on sight.

## 3. Defect taxonomy

| # | Defect | Severity | Layer | Root |
|---|--------|----------|-------|------|
| **D1** | Phase outcome recording is happy-path-only: every orchestrator abort path between dispatch return and the recording site erases the outcome (no verdict/usage/timing/PhasesRun) → record diverges from reality | 🔴 Critical | orchestrator (as-located 2026-06-10; runner reconciles correctly) | one recording site, ~9 abort returns before it |
| **D2** | No fatal-pane-state detection → full `maxExtends` wait on self-describing fatal states | 🔴 Critical | bridge driver | terminal state is untyped |
| **D3** | `auto` model leaks to `claude --model auto` (invalid); no omit-on-auto guard for claude-tmux | 🟠 High | bridge driver | model-flag policy not uniform across drivers |
| **D4** | Meta-phases (retro) have `cli_fallback:null, model_fallback:null` — no recovery path at all | 🟠 High | profiles | fallback coverage incomplete |
| **D5** | Observer detects stalls (`stall_no_output`) but takes no corrective action | 🟡 Medium | observer | detection and action conflated/absent |
| **D6** | Self-updating CLI mutates its own binary mid-cycle, killing the REPL | 🟠 High | host / preflight | CLI versions not frozen for a run |
| **D7** | Repair ladder (ADR-0039 §8) covers only ship errors, not phase-execution failures | 🟡 Medium | ship/recovery | recovery typed for one phase only |

## 4. Root cause: recovery is an un-owned cross-cutting concern

Every defect above is a symptom of one architectural fact: **recovery logic is smeared across five
modules with no single owner**, and each assumes another layer will catch the failure:

- `bridge/driver_tmux_repl.go` — artifact-wait + `maxExtends` backstop (but no fatal-state classification)
- `phases/runner/runner.go` — `cli_fallback` dispatch (but the fallback's success isn't reconciled)
- `core/orchestrator.go` — verdict recording, spine advance, failure-adapter (but only for the primary path)
- `core/observer.go` + `phaseobserver/` — stall *detection* (but no corrective action)
- `phases/ship/` repair ladder — typed repairs (but only for ship error codes)

Because no component owns "given a dispatch result, did this phase succeed, and is that success recorded?",
the answer can diverge from reality. cycle-262 is the proof: **the process succeeded and the record said
failure.** A correctly-architected recovery layer makes that divergence structurally impossible.

## 5. AI-driven design principles (the governing lens)

1. **Deterministic-first, LLM-last (Core Agent Rule 5).** Every cycle-262 failure was *deterministically
   classifiable* — a known error string, an exit code, a present-but-unrecorded artifact. Mechanical recovery
   (fallback selection, fatal-signature matching, verdict reconciliation, version pinning) belongs in **regular
   code**. The LLM is reserved for genuine judgment: *"is this novel failure recoverable, and how?"*
2. **Classify before you handle.** No failure is acted on without a *typed cause*. `exit 81` is not a cause;
   `claude booted into an inaccessible-model error` is. Untyped terminal states are why D1/D2 leaked.
3. **Outcome-orientation, not process-orientation.** Recovery reconciles to a **recorded verdict**, never to
   "the process exited 0." D1 is the canonical violation of this.
4. **Self-learning tail.** When a *novel* pane state appears, the LLM classifies it once; that classification is
   **promoted into the deterministic signature registry** so next time it is a fast deterministic catch.
   Judgment at the frontier, determinism in the core — and the frontier shrinks over time.

## 6. Solution architecture

Introduce **one owner**: a *Phase Recovery Pipeline* whose single responsibility is *"given a dispatch result,
produce a reconciled `PhaseOutcome` or a typed terminal failure."* It is assembled from standard patterns.

### C1 — Single-source outcome recording (DRY chokepoint) → fixes D1 — ✅ SHIPPED 2026-06-10 (as-located)
The design sketch below predates locate-the-fork; the shipped shape moved the chokepoint to where the fork
actually is — the **orchestrator**, whose phase iteration had ONE recording site on the happy path and ~9
abort returns before it (the runner already reconciles every dispatch route correctly, including
fallback-success and reconcile-on-timeout):

```
BEFORE (recording is happy-path-only)                AFTER (one chokepoint, every terminal disposition)
  happy advance        ─► record verdict/usage/…      happy advance      ─┐
  review-gate reject   ─► (nothing) ──► ✗ LOST        exhausted retries  ─┤
  tree-guard abort     ─► (nothing) ──► ✗ LOST        review reject      ─┼─► recordPhaseOutcome(PhaseOutcome)
  leak-recovery fail   ─► (nothing) ──► ✗ LOST        ship-err recovery  ─┤      └─► PhasesRun + phase-timing.json
  ledger/state fail    ─► (nothing) ──► ✗ LOST        guard/persist abort─┘          + <phase>-usage.json (+abort_reason)
```

As built: `go/internal/recovery/` leaf package owns the `PhaseOutcome` envelope;
`core.phaseOutcomeFrom(phase, resp, attempts, abortReason)` owns the reconciliation rule (canonical agent
verdict recorded as-is; anything else synthesizes FAIL — **never PASS**); the verdict stays the agent's own on
abort paths, with the abort recorded as additive `abort_reason` (omitempty) in both artifacts. Aborted-but-
dispatched phases now appear in `PhasesRun`. **This makes the 262-class divergence structurally impossible** —
the faithful 262 replay (test `TestPhaseOutcome_TreeGuardAbort_RecordsBuildOutcome`) still fails the cycle on
the genuine leak (the guard is right) but records build's PASS + cost + timing, so a salvage needs no forensic
reconstruction. Deferred to the C3 slice: `resume.go` is a second, simpler recording boundary (writes no
timings/sidecars at all) to be unified through the same chokepoint.

### C2 — Terminal-state classification (Template Method hook + Strategy) → fixes D2, D3
The tmux driver already *is* a Template Method (`driver_tmux_repl.go`). Add one hook every driver implements:

```
ClassifyTerminal(pane, exitCode, artifactPresent) → TerminalCause
   { Success | FatalConfig("model auto invalid") | NeedsRestart("codex self-update") | DeadShell | Stalled | RecoverableTimeout }
```

Known-fatal causes → **fast-fail with a typed cause** (no `maxExtends` backstop). Model-flag normalization
(`auto` → omit or concrete) becomes a per-driver **`ModelFlagPolicy`** (Strategy): codex already has it,
claude-tmux gets the same. Adding a CLI ⇒ a new policy, zero edits elsewhere (Open/Closed).

### C3 — Chain of Responsibility for recovery → composes D1–D4
The dispatch result flows through an ordered chain; each handler recovers, passes, or escalates:

```
ModelFlagPolicy → FatalPaneDetector → FallbackStrategy(cli/model) → VerdictReconciler → LLMFailureAdvisor (escalate)
```

Adding a recovery behavior = adding a handler. The chain *is* the protocol that is currently missing; it gives
recovery a single, testable owner and a single place to reason about ordering.

### C4 — Observer + StallPolicy (Single Responsibility) → fixes D5
Keep the Observer as pure detection (it is already a clean Observer pattern emitting `stall_no_output` etc.).
Inject a **`StallPolicy`** that maps events → actions (`extend | kill+retry | escalate`). Detection and action
become independently testable; the observer stops being a passive logger.

### C5 — CLI-version-freeze Specification (preflight) → fixes D6
Extend `looppreflight` (ADR readiness gate) with a predicate: any `-tmux` CLI reporting `startup update
check = true` (e.g. via `codex doctor`) must be pinned (converge the state) or the batch Halts with guidance.
Idempotent and crash-safe — pinning is a steady state, **not** a per-cycle toggle (a pin/unpin-on-exit is
fragile: a hard kill never runs the unpin, freezing the host's CLI silently; and `brew pin` is global +
persistent, not session-scoped). See §8 alternatives.

### Where AI genuinely drives (the escalation tail)
`LLMFailureAdvisor` is the **last** link, reached only for *unclassified* terminal states. It reasons about a
never-seen failure and proposes a recovery — and its classification of a novel pane signature is **written back
into the deterministic registry** that `FatalPaneDetector` reads. Each failure class gets cheaper and faster to
handle over time; the LLM is never on the hot path for a known failure.

## 7. Implementation order (risk-ranked, test-gated)

| Order | Change | Why first | Size |
|------|--------|-----------|------|
| 1 | ✅ **C1 outcome-recording chokepoint** (shipped 2026-06-10 as-located in the orchestrator; see §6 C1) | Highest leverage — makes 262-class record divergence impossible | M |
| 2 | **C2 `ModelFlagPolicy`** (claude omit-on-auto + ensure every phase gets a concrete model) | Tiny; stops the exact retro break | S |
| 3 | **C2 `FatalPaneDetector`** (seed with the 3 known signatures) | Eliminates the ~20-min slow-fails | M |
| 4 | **C5 freeze preflight** + **D4 meta-phase fallbacks** (config) | Cheap; prevents recurrence | S |
| 5 | **C4 observer `StallPolicy`** | Corrective detection | M |
| 6 | **C3 Chain refactor** (compose 1–4) + LLM advisor + promotion loop | Unifies into the protocol | L |

Each step ships behind the project's gates (TDD red→green, adversarial audit, EGPS red_count==0), measured and
behavior-neutral where possible — never a blind change to the hot dispatch loop.

## 8. Alternatives considered

- **Per-cycle `brew pin` / `unpin`-on-exit (operator's first instinct).** Rejected as the primary mechanism:
  the unpin depends on a clean exit, but the loop is kill-prone (SIGKILL/OOM/reboot never run the unpin) →
  the host's CLI is silently frozen with no signal; `brew pin` is also global + persistent, so concurrent
  cycles race. Replaced by C5 (pin is a convergent steady state; updates are an explicit operator action).
- **Just raise `maxExtends` / lower the artifact timeout.** Rejected: tuning the backstop treats the symptom
  (slow-fail) not the cause (untyped terminal state). C2 classification is the real fix.
- **Make the LLM failure-adapter handle everything.** Rejected: violates deterministic-first (Rule 5); cycle-262
  proved the LLM retro can itself fail. The LLM is the escalation tail, not the trunk.
- **Give every phase a CLI fallback and call it done.** Insufficient alone: without C1, even a *successful*
  fallback isn't recorded — fallback coverage (D4) without reconciliation (D1) still fails the cycle.

## 9. Risks & non-goals

- **Phase isolation stays sacred.** Recovery must not let one phase's REPL/context leak into another (no
  cross-phase session reuse; builder≠auditor cross-family floor unchanged).
- **Reconciliation only ever upgrades a synthesized FAIL toward the agent's real verdict** — it never invents a
  PASS. A valid deliverable with red predicates still fails the EGPS gate. (Mirrors the existing
  artifact-timeout reconciliation invariant.)
- **Not in scope:** test-suite latency (`go/docs/testing.md`); the boot_ms/pipeline-latency program (ADR-0043),
  which is a sibling workstream.
