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
The operator asked three questions, and the evidence below answers each one. The operator then asked for a solution from an
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
nudged a bare `zsh` shell (`command not found: Please`). This is interactive-TUI-only behavior. The headless
`codex exec` path (`driver_codex.go`) does not trigger it, and the loop's builder uses the `codex-tmux` REPL path.

### 2b. retro was dispatched with an invalid model (D3)
The retro pane captured the fatal state: `"There's an issue with the selected model (auto). It may not exist
or you may not have access to it. Run /model to pick a different model."` — `Brewed for 0s`.

Per-phase routing gave scout/tdd concrete models (`claude --model sonnet`, `claude --model opus`), and they worked.
**Retro never received a concrete model**, so it fell through to the literal `auto`. The `claude-tmux` driver passed
`--model auto` verbatim. `auto` is a *settings* concept, **not a valid `claude --model` CLI value**. The codex
driver guards exactly this (`driver_codex.go:48` — `case resolved=="" || resolved=="auto": omit`). The
`claude-tmux` driver has **no equivalent guard**.

### 2c. the successful build was never recorded — so the cycle "failed" (D1, the critical one)
The codex build try hit `exit 81` (artifact timeout). The configured CLI fallback to `claude-tmux`
**succeeded** (`dispatch chain: codex-tmux=81 -> claude-tmux=0`). It wrote a valid `build-report.md` (`**Status:**
PASS`) and **actually accomplished the goal** (`chore(build)` is present in the manifest). Yet:
- there is **no `build-usage.json`**,
- **no `build` entry in `phase-timing.json`** (only `scout`, `tdd`, `retro`),
- **no recorded build verdict**.

So the orchestration spine saw `build` as not-completed. It **skipped audit** (`ship ⇒ build ∧ audit` was unable to
advance). It routed to retro as a failed cycle. The failure-learning logged `cycle-262-failed-build`. **The
work was done; only the recording was missing.**

**Mechanism located (2026-06-10, Slice 1 locate-the-fork). This supersedes the implicit guess of the paragraph above
that the fallback was not reconciled.** The loop log proves that the fallback chain of the runner worked end-to-end
(`dispatch chain: codex-tmux=81 -> claude-tmux=0`; Classify ran; the runner returned PASS with nil error).

What erased the record: the claude fallback builder wrote the tracked config
`.evolve/commit-prefix-scope.json` into the **main tree** and not into its worktree. `recoverBuildLeak`
deliberately never relocates `.evolve/` paths (they are normally runtime state, orchestrator.go:622). The
post-phase **tree-diff guard therefore correctly aborted the cycle** (`leaked paths: [.evolve/commit-prefix-scope.json]`
is the final error of the cycle).

That abort path returned **without recording the outcome**. So did *every* abort path between the return of `runner.Run` and
the single happy-path recording site of the orchestrator.
There was no timing entry, no usage sidecar and no `PhasesRun` membership. About 9 paths are affected:

- review-gate reject;
- correction failures;
- ship-error recovery;
- leak-recovery failure;
- tree-guard;
- ledger/state persistence failures.

The guard was right. Erasing the evidence was the defect.

Adjacent finding, not D1: the guard system has a blind spot
for builders whose *legitimate deliverable* is a tracked `.evolve/` config. Relocation skips it by design,
so such a build can only ever abort. Tracked as a C3-era question.

### Cost of the two slow-fails
codex-dead (2a) burned ~20 min and retro-model-error (2b) burned ~20 min. Each ran to the `maxExtends`
backstop (`tmuxArtifactTimeoutS=300` × ~4), because **nothing recognizes a self-describing fatal pane state**.
The cycle spent ~40 min of ~52 min waiting on failures that were deterministically detectable on sight.

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
modules with no single owner**. Each module assumes that another layer catches the failure:

- `bridge/driver_tmux_repl.go` — artifact-wait + `maxExtends` backstop (but no fatal-state classification)
- `phases/runner/runner.go` — `cli_fallback` dispatch (but the success of the fallback is not reconciled)
- `core/orchestrator.go` — verdict recording, spine advance, failure-adapter (but only for the primary path)
- `core/observer.go` + `phaseobserver/` — stall *detection* (but no corrective action)
- `phases/ship/` repair ladder — typed repairs (but only for ship error codes)

No component owns "given a dispatch result, did this phase succeed, and is that success recorded?".
The answer can therefore diverge from reality. cycle-262 is the proof: **the process succeeded and the record said
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
4. **Self-learning tail.** When a *novel* pane state appears, the LLM classifies it once.
   The system **promotes that classification into the deterministic signature registry**.
   Next time it is a fast deterministic catch.
   Judgment at the frontier, determinism in the core — and the frontier shrinks over time.

## 6. Solution architecture

Introduce **one owner**: a *Phase Recovery Pipeline* whose single responsibility is *"given a dispatch result,
produce a reconciled `PhaseOutcome` or a typed terminal failure."* It is assembled from standard patterns.

### C1 — Single-source outcome recording (DRY chokepoint) → fixes D1 — ✅ SHIPPED 2026-06-10 (as-located)
The design sketch below predates locate-the-fork. The shipped shape moved the chokepoint to where the fork
actually is: the **orchestrator**. Its phase iteration had ONE recording site on the happy path and ~9
abort returns before it. The runner already reconciles every dispatch route correctly, including
fallback-success and reconcile-on-timeout:

```
BEFORE (recording is happy-path-only)                AFTER (one chokepoint, every terminal disposition)
  happy advance        ─► record verdict/usage/…      happy advance      ─┐
  review-gate reject   ─► (nothing) ──► ✗ LOST        exhausted retries  ─┤
  tree-guard abort     ─► (nothing) ──► ✗ LOST        review reject      ─┼─► recordPhaseOutcome(PhaseOutcome)
  leak-recovery fail   ─► (nothing) ──► ✗ LOST        ship-err recovery  ─┤      └─► PhasesRun + phase-timing.json
  ledger/state fail    ─► (nothing) ──► ✗ LOST        guard/persist abort─┘          + <phase>-usage.json (+abort_reason)
```

As built:

- The `go/internal/recovery/` leaf package owns the `PhaseOutcome` envelope.
- `core.phaseOutcomeFrom(phase, resp, attempts, abortReason)` owns the reconciliation rule. The rule records a canonical agent verdict as-is. Anything else synthesizes FAIL — **never PASS**.
- The verdict stays the agent's own on abort paths. The abort is recorded as additive `abort_reason` (omitempty) in both artifacts.

Since 2026-09-13 (cycles 1634/1636) the timing record also carries the OWN `diagnostics` of the phase (severity + message, as the
`PhaseResponse` reported them, omitempty). A phase that returns FAIL by its own Classify seals with the reason it
gave. The seal (`backfillFailReasons`), the chokepoint log line and `cyclehealth` all read that one field.
The synthesized "phase-infra class" marker is now reserved for a FAIL that recorded nothing.

Aborted-but-dispatched phases now appear in `PhasesRun`. **This makes the 262-class divergence structurally impossible.**
The faithful 262 replay (test `TestPhaseOutcome_TreeGuardAbort_RecordsBuildOutcome`) still fails the cycle on
the genuine leak (the guard is right). But it records the PASS + cost + timing of build, so a salvage needs no forensic
reconstruction. Deferred to the C3 slice: `resume.go` is a second, simpler recording boundary (it writes no
timings/sidecars at all). The slice unifies it through the same chokepoint.

### C2 — Terminal-state classification (Template Method hook + Strategy) → fixes D2, D3 — ✅ SHIPPED 2026-06-10 (as-built below)
The tmux driver already *is* a Template Method (`driver_tmux_repl.go`). The design sketch:

```
ClassifyTerminal(pane, exitCode, artifactPresent) → TerminalCause
   { Success | FatalConfig("model auto invalid") | NeedsRestart("codex self-update") | DeadShell | Stalled | RecoverableTimeout }
```

Known-fatal causes → **fast-fail with a typed cause** (no `maxExtends` backstop). Model-flag normalization
(`auto` → omit or concrete) becomes a per-driver **`ModelFlagPolicy`** (Strategy): codex already has it,
claude-tmux gets the same. Adding a CLI ⇒ a new policy, zero edits elsewhere (Open/Closed).

**As built:**

- The typed-cause registry is `recovery.FatalPaneDetector` (`internal/recovery/detector.go`). It is the single recovery owner, not a per-driver hook. It is seeded with the three cycle-262 signatures.
- `bridge.fatalPaneVerdict` consults it at the stop-review checkpoint **before** the reviewer. This ordering is load-bearing. The dead panes counted as "progressed" (the nudge of the bridge echoed into them), so a check after the reviewer never fires.
- `EVOLVE_PHASE_RECOVERY` gates the stage (off | **shadow default** | enforce; typo→off). Classification is always-on above off. Only the kill is staged.
- The detector never preempts a Busy pane.
- The one-shot nudge is now gated on `ReviewPause`, so a fatal `ReviewStop` exits immediately (exit 81 → the fallback chain of the runner takes over). This is the rescue of cycle-262, ~20 min sooner.
- `ModelFlagPolicy` landed at the **realizer chokepoint** (`realizeScalar`: post-resolution `auto` emits no model param on any channel). This is one matrix-wide guard instead of N per-driver copies. The headless codex driver keeps its exec-path guard.
- The per-driver `ClassifyTerminal` hook was deliberately NOT added yet, because it has no consumer. It rides with C3 if the chain needs it.

### C3 — Chain of Responsibility for recovery → composes D1–D4 — ✅ SHIPPED 2026-06-10 (as-built below)
The dispatch result flows through an ordered chain; each handler recovers, passes, or escalates. The design
sketch named the pre-dispatch guards as links:

```
ModelFlagPolicy → FatalPaneDetector → FallbackStrategy(cli/model) → VerdictReconciler → LLMFailureAdvisor (escalate)
```

**As built** (`recovery/handler.go`, modeled on `router/recovery.go`): the always-on correctness pieces stay where the slices put them. They are not runtime *decisions*. The pieces are:

- ModelFlagPolicy at the realizer;
- VerdictReconciler = the C1 chokepoint;
- FallbackStrategy = the exit-81 chain of the runner.

The chain owns the DECISIONS for a classified terminal state:

```
integrity-escalate → busy-extend → known-fatal-kill → stall-budget-extend → unknown-advise (terminal)
```

`Recover(RecoverInput) → Decision{Action, Handler, Reason}` — pure, deterministic, every decision justified.
Consumers:

- The chain-backed StallPolicy of the observer (`NewChainStallPolicy`). The observer subprocess injects it ONLY at `EVOLVE_PHASE_RECOVERY=enforce`. Advise degrades to escalate there.
- The escalate→advise→promote hook of the orchestrator (`core/failure_hook.go`, `WithFailureAdviser`, enforce-only, deterministic-registry-first, best-effort).

`config.RolloutStages.PhaseRecovery` is the view of the orchestrator on the one dial (default shadow). The same slice unified `resume.go` through the C1 chokepoint.
To add a recovery behavior, add a handler. The chain *is* the protocol that was missing.

### C4 — Observer + StallPolicy (Single Responsibility) → fixes D5 — ✅ SHIPPED 2026-06-10 (seam default-nil; real policy wired in C3)
Keep the Observer as pure detection. It is already a clean Observer pattern that emits `stall_no_output` and other events.
Inject a **`StallPolicy`** that maps events → actions (`extend | kill+retry | escalate`). Detection and action
become independently testable. The observer stops being a passive logger.

### C5 — CLI-version-freeze Specification (preflight) → fixes D6 — ✅ SHIPPED 2026-06-10
Extend `looppreflight` (ADR readiness gate) with a predicate. Any `-tmux` CLI that reports `startup update
check = true` (for example, with `codex doctor`) must be pinned (converge the state), or the batch Halts with guidance.
The predicate is idempotent and crash-safe. Pinning is a steady state, **not** a per-cycle toggle.

A pin/unpin-on-exit is fragile: a hard kill never runs the unpin, which silently freezes the CLI of the host. Also, `brew pin` is global +
persistent, not session-scoped. See §8 alternatives.

**As built** (`internal/looppreflight/freeze.go`, the 5th check of the gate): host-grounding showed that `codex doctor`
does not reliably emit update info. The check therefore detects "risky" with the `SelfUpdateEvidence` seam (default
registry: codex → `~/.codex/version.json`, the updater-state file from the incident). It detects "pinned" with
`PinnedLister` (default `brew list --pinned`). The outcomes are:

- Confirmed risk without a pin → Halt with the exact `brew pin codex` guidance.
- Unverifiable pin state → Warn (fail-open on ambiguity).
- Headless-only usage is out of scope (the updater fires on the TUI launch path).

### Where AI genuinely drives (the escalation tail) — ✅ COMPONENTS SHIPPED 2026-06-10 (CoR wiring in C3)
`LLMFailureAdvisor` is the **last** link, reached only for *unclassified* terminal states. It reasons about a
never-seen failure and proposes a recovery. The system **writes back its classification of a novel pane signature
into the deterministic registry** that `FatalPaneDetector` reads. Each failure class gets cheaper and faster to
handle over time. The LLM is never on the hot path for a known failure.

**As built:**

- `core.FailureAdvisor` (PhaseAdvisor blueprint: bridge + persona `agents/evolve-failure-advisor.md` + profile `failure-advisor.json`, strict-JSON, every failure mode errors → caller escalates) returns `recovery.FailureAdvice{cause, pane_substr, justification}`.
- `recovery.PromoteAdvice` is the validating trust boundary (typed-vocabulary cause, ≥12-char substring). It promotes in-memory (after seeds — promotions never shadow vetted entries) AND durably (`.evolve/instincts/fatal-signatures/<content-hash-id>.yaml`, absent-only, confidence 0.5, zero-dep YAML).
- The tmux driver boots with `SeedDetectorWithPromotions`, so the system pays for each novel state exactly once.

## 7. Implementation order (risk-ranked, test-gated)

| Order | Change | Why first | Size |
|------|--------|-----------|------|
| 1 | ✅ **C1 outcome-recording chokepoint** (shipped 2026-06-10 as-located in the orchestrator; see §6 C1) | Highest impact — makes 262-class record divergence impossible | M |
| 2 | ✅ **C2 `ModelFlagPolicy`** (shipped 2026-06-10 at the realizer chokepoint — matrix-wide omit-on-auto) | Tiny; stops the exact retro break | S |
| 3 | ✅ **C2 `FatalPaneDetector`** (shipped 2026-06-10; seeded with the 3 cycle-262 signatures, `EVOLVE_PHASE_RECOVERY`-staged, shadow default) | Eliminates the ~20-min slow-fails (in enforce) | M |
| 4 | ✅ **C5 freeze preflight** + **D4 retro fallback** (shipped 2026-06-10: `looppreflight` 5th check — self-update evidence ∧ tmux-driven ⇒ brew-pinned or Halt; `retrospective.json` gains `cli_fallback:["codex-tmux"]`; broader meta-phase coverage rides C3) | Cheap; prevents recurrence | S |
| 5 | ✅ **C4 observer `StallPolicy`** (shipped 2026-06-10: `recovery.StallPolicy` Strategy at both INCIDENT sites, default-nil byte-identical; verdict + justification recorded in the envelope) | Corrective detection | M |
| 6 | ✅ **C3 Chain refactor** (shipped 2026-06-10: `recovery.Recover` CoR + chain-backed StallPolicy [enforce-only] + `config.PhaseRecovery` dial [shadow default] + resume.go→C1 chokepoint + the enforce-only advise→promote hook) | Unifies into the protocol | L |

**Program status (2026-06-10): all six steps shipped.** The dial ships at `shadow`; flip
`EVOLVE_PHASE_RECOVERY=enforce` (and wire `WithFailureAdviser(core.NewFailureAdvisor(bridge, …))` at the
composition root) after a soak batch shows clean shadow logs. The cycle-262 replay tests pin every D1–D6
fix; D7 (typed recovery beyond ship) is the chain itself.

Each step ships behind the gates of the project (TDD red→green, adversarial audit, EGPS red_count==0). Each step is measured and
behavior-neutral where possible. It is never a blind change to the hot dispatch loop.

## 8. Alternatives considered

- **Per-cycle `brew pin` / `unpin`-on-exit (first instinct of the operator).** Rejected as the primary mechanism.
  The unpin depends on a clean exit, but the loop is kill-prone (SIGKILL/OOM/reboot never run the unpin).
  The CLI of the host is then silently frozen with no signal. `brew pin` is also global + persistent, so concurrent
  cycles race. C5 replaces it (pin is a convergent steady state; updates are an explicit operator action).
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
