# ADR-0044: Unified Phase Recovery Protocol — deterministic-first, single-owner recovery

> Status: **Implemented** (designed 2026-06-09; all six slices shipped 2026-06-10 — record in
> §Implementation; rollout dial `EVOLVE_PHASE_RECOVERY` default **shadow**, flip to `enforce` after soak).
> Design-first: records the analysis and chosen direction; implementation is
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

- **C1 amendment — 2026-09-13 (cycles 1634/1636, `docs/incidents/2026-09-13-phase-own-fail-reason-invisible.md`).**
  The envelope and the timing record gain the phase's OWN `Diagnostics` (severity + message, relayed unfiltered
  by `phaseOutcomeFrom`, additive `diagnostics` omitempty on `phase-timing.json`; the usage sidecar stays a usage
  record). A FAIL the phase itself reasoned about was previously sealed as "phase-infra class" because only floor
  phases had a side channel for their diagnostics. `cyclestate.ErrorMessages` (beside the severity vocabulary) is the ONE
  projection from diagnostics to reasons and `verdictFailReason` the ONE rendering of "why FAIL"
  (`floorVerdictError`, the chokepoint log line and the seal's backfill project it); `cyclehealth` reads the
  record through `phasetiming.Entry` — no hand-typed mirror — and projects the same function for its
  FAILED_EXPLAINED detail. Tests: `phase_diagnostics_seal_test.go`, `failreasons_backfill_test.go`,
  `phasetiming/diagnostics_test.go`, `cyclehealth/outcome_diagnostics_test.go`.

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

- **Slice 3 / C5 (+D4 config) — shipped 2026-06-10.** `looppreflight.checkCLIVersionFreeze`
  (`internal/looppreflight/freeze.go`), the fifth readiness check: the Specification
  *risky(bin) ∧ tmuxDriven(bin) ⇒ pinned(bin)*. "Risky" = host evidence of a self-updater via the
  `SelfUpdateEvidence` seam (default registry: codex → `~/.codex/version.json`, the updater-state file from
  the incident; the design sketch's "probe `codex doctor`" was dropped after host-grounding showed the doctor
  subcommand does not reliably emit update info). "Pinned" via the `PinnedLister` seam (default
  `brew list --pinned` — verified live: codex is a pinned formula). Confirmed risk + no pin → **Halt** with
  the exact convergent action (`brew pin codex` + the deliberate-update procedure); pin state unverifiable
  (brew absent/exec error) → **Warn**, never a false Halt (eval-gate fail-open posture); scope is *-tmux
  drivers only (headless `codex exec` does not run the updater). Read-only probes ⇒ idempotent by
  construction; the pin is the operator's one-time convergent state (per-cycle pin/unpin rejected in
  §Alternatives). **D4 config half:** `retrospective.json` gains `cli_fallback: ["codex-tmux"]` — the
  cycle-262 retro had NO recovery path at all (`cli_fallback:null`); with the runner's exit-81 trigger chain
  this works immediately, independent of the C2 stage. Broader meta-phase fallback-coverage audit rides C3.
  Tests: `looppreflight/freeze_test.go` (unpinned-halts w/ guidance, pinned-passes, no-evidence-passes,
  headless-only-skipped, pin-probe-error-warns, idempotent).

- **Slice 4 / C4 — shipped 2026-06-10.** `recovery.StallPolicy` (Strategy; `recovery/stallpolicy.go`):
  typed `StallEvent` → `extend | kill_retry | escalate` + justification. The observer's two INCIDENT sites
  (`phaseobserver.go` stuck_no_output / stuck_no_progress) now funnel through one `handleStallIncident`
  closure: **nil policy (the default) is byte-identical legacy** (Enforce→SIGTERM branch, unenriched
  envelope — pinned by `TestRun_StallPolicyNil_EnvelopeUnenriched` + the pre-existing enforce/no-enforce
  tests); with a policy injected, its verdict outranks `Enforce` (extend/escalate suppress the kill;
  kill_retry kills even without Enforce) and the decision lands INSIDE the INCIDENT envelope
  (`action` / `action_reason` — every recovery decision justified). Detection and action are now
  independently testable (SRP); no caller injects a policy yet — the C3 composition slice wires the real,
  stage-gated implementation. Tests: `phaseobserver/stallpolicy_test.go` (extend-no-kill,
  kill-retry-without-enforce, escalate-no-kill, nil-unenriched).

- **Slice 5 / LLMFailureAdvisor + promotion — shipped 2026-06-10.** The AI escalation tail.
  `core.FailureAdvisor` (`failure_advisor.go`, built exactly like `PhaseAdvisor`: bridge-dispatched,
  functional options, persona-injectable, strict-JSON-parsed, fail-safe — nil bridge / launch error /
  malformed output / vocabulary violation all return errors so the caller ESCALATES, never acts on
  garbage): `Advise(FailureAdviseInput) → *recovery.FailureAdvice` reads one CauseUnknown pane and returns
  {cause, pane_substr, justification}; validation at the parse site is the trust boundary (cause must be in
  the typed vocabulary; empty justification rejected). Persona `agents/evolve-failure-advisor.md` + profile
  `.evolve/profiles/failure-advisor.json` (deep tier, read-only tools + Write scoped to
  `failure-advice.json`, $0.5 budget — judgment work, off the hot loop). **Promotion loop**
  (`recovery/promote.go`, Reflexion-style): `Detect`or`.Promote` (in-memory, AFTER seeds — promotions can
  never shadow vetted seeds) + `PromoteSignature` (durable absent-only `<id>.yaml` under
  `.evolve/instincts/fatal-signatures/`, deterministic content-hash id ⇒ idempotent, confidence 0.5,
  zero-dep fixed-key YAML written AND parsed in the leaf) + `SeedDetectorWithPromotions(dir)` (replay at
  boot; corrupt files skipped — a bad promotion never bricks boot) + `PromoteAdvice` (the validating
  chokepoint: out-of-vocabulary cause or <12-char substring REJECTED — short substrings are false-positive
  bombs). The tmux driver now boots its detector via `SeedDetectorWithPromotions(<root>/.evolve/instincts/
  fatal-signatures)`, so a signature classified once is caught deterministically forever. No production
  caller invokes Advise yet — C3 wires the CoR's escalate→advise→promote path. Tests:
  `core/failure_advisor_test.go` (parse+dispatch contract, malformed/unknown-cause/nil-bridge/bridge-error
  all fail safe), `recovery/promote_test.go` (in-memory immediate catch, seed precedence, durable
  absent-only + idempotent id, replay, corrupt-file safety, PromoteAdvice validation).

- **Slice 6 / C3 — shipped 2026-06-10 (the composing slice; ADR → Implemented).** Four parts.
  (a) **The chain** (`recovery/handler.go`, modeled on `router/recovery.go`): ordered
  `integrity-escalate → busy-extend → known-fatal-kill → stall-budget-extend → unknown-advise(terminal)`,
  pure `Recover(RecoverInput) → Decision{Action, Handler, Reason}`; order pinned by tests (integrity
  outranks busy outranks known-fatal; known causes NEVER reach the LLM; only the unknown residue advises).
  `NewChainStallPolicy` adapts the chain to the C4 StallPolicy port (advise degrades to escalate — the
  observer subprocess cannot dispatch an advisor). (b) **The dial's orchestrator view**:
  `config.RolloutStages.PhaseRecovery` (EVOLVE_PHASE_RECOVERY via `parseEvidenceStage`, default
  **StageShadow**, typo→off) — the bridge/observer subprocesses keep reading env directly (the
  CommitEvidence/SandboxMode precedent). The observer subprocess (`cmd_phase_observer.go`) injects the
  chain-backed policy ONLY at explicit `enforce`. (c) **resume.go unified through the C1 chokepoint**
  (the deferred debt): `RunCycleFromPhase` now records every terminal disposition via
  `recordPhaseOutcome` + the shared `writePhaseTimings` writer — resumed phases are no longer invisible
  in phase-timing.json. (d) **The escalate→advise→promote hook** (`core/failure_hook.go`):
  `WithFailureAdviser` injects the Slice-5 tail behind the `FailureAdviser` port; at StageEnforce only,
  for artifact-timeout aborts only, the hook reads the bridge's escalation report (`final_pane`), checks
  the deterministic registry FIRST (known panes never pay for an LLM call), then Advise→PromoteAdvice —
  strictly best-effort (WARNs, never alters the abort flow). Tests:
  `recovery/handler_test.go` (order-is-load-bearing, known-fatal-never-hits-LLM, unknown-advises,
  integrity-always-escalates, observer adaptation), `core/failure_hook_test.go`
  (shadow/off-never-consults [the plan's ShadowDefault_NoCorrectiveAction], enforce-advises-and-promotes
  + second-occurrence-deterministic, known-pane-skips-advisor, advisor-error-best-effort),
  `config.TestLoad_PhaseRecoveryStage` (default shadow, trichotomy, typo→off+warn).
  **Not wired anywhere by default**: the composition root gains the dial but no caller passes
  `WithFailureAdviser` yet — flipping enforce + wiring `NewFailureAdvisor` at cmd-level is the
  post-soak step, deliberately separate from this slice.

### Amendment — F27 (2026-09-26): the C2 fatal-pane fast-fail gets its OWN dial

**Problem.** The one-dial discipline above left C2 unable to act on its own soak evidence:
`recovery.phase_recovery` also arms the live channel (`channel.Enabled`, ADR-0045 I6), the ask-broker and
the failure-adviser promotion, so flipping it to let the fast-fail act would have armed three unsoaked
subsystems. The fast-fail therefore stayed at shadow while dead panes burned the backstop.

**Evidence (the soak).** Every `fatal_pane_shadow` record in the live ledger — 3 records, 2 cycles, both
CLI families, trigger `dead_shell` (`: command not found`) — was a dead pane: cycle 1595 build (codex-tmux)
then idled 1200 s to `stop-review → pause: no output`; cycle 1687 triage (claude-tmux) recorded
`would_fast_fail` twice, idled 900 s, then exhausted its fallback chain. No record sits on a phase that later
completed on the same pane.

**Decision.** Split the dial exactly as R8.5 split `SpineFloor`: `recovery.fatal_pane` (policy-only, no env
var) gates ONE behavior — whether a persisted, non-Busy fatal-pane match at the stop-review checkpoint
preempts the reviewer. Default **enforce**; `"shadow"` is the no-recompile escape hatch; `"off"` skips the
detector. `recovery.phase_recovery` stays **shadow** and keeps the channel, ask-broker, transient-dwell,
the failure adviser and the observer's stall policy.

**Wiring (one path, pinned).** `policy.RecoveryConfig().FatalPane` → `config.PolicyStages.FatalPane` →
`Loader.ApplyPolicyStages` (`recovery.fatal_pane`, typo → off + `CONFIG_UNKNOWN_VALUE`) →
`config.RolloutStages.FatalPane` → `wireBridgeStages` (the root's ONE forwarding of the bridge dials,
`cmd_cycle_config.go`) → `Adapter.SetFatalPaneStage` → `productionEngineDeps` (the builder BOTH `Launch`
branches share — `RecoveryStage` moved there too, closing a branch-local assignment that could drift) →
`Deps.FatalPaneStage` → `fatalPaneStageOf` (the same `channel.ResolveStage` normalizer: unset → shadow,
typo → off, and `"0"` — `config.StageOff.String()` — accepted explicitly as off) → the wait state's
detector seeding and the checkpoint's persistence gate. The production roots that build engine `Deps`
without the cycle root's setters — `adapters/bridge.NewDefault` (the per-phase registry factories,
`cmd_campaign.go`) and the `evolve subagent run` root (`internal/subagent.execAdapterDeps`, which before
this amendment set neither dial) — seed both from `policy.json` through ONE accessor,
`policy.Policy.BridgeRecoveryStages`, which parses each word with `config.GateStage` (the Loader's own
trichotomy — no root-specific case-folding), so every root turns the same policy word into the same stage;
the cycle root overrides with the Loader-validated values (`wireBridgeStages`, pinned as called by
`cmd_cycle.go`).

**Routing is unchanged, only timing.** A fatal preemption (`ReviewStop`) and the reviewer's old
`ReviewPause` close out identically — escalation report, `ExitArtifactTimeout` (81) — so the fast-failed
phase enters the fallback chain at the same rung as before, 900–1200 s sooner. Whether that rung can
SAVE a `dead_shell` phase (a fresh session of the same CLI family) is a separate, cause-keyed follow-up —
with codex quota-walled, the chain's later rungs could not in cycle 1687.

**Tests.** `TestRunTmuxREPL_FatalPaneDialIsIndependentOfPhaseRecovery` (the two dials crossed both ways;
red against the mutant that re-points the wait state at the program dial), `TestFatalPaneStageOf`,
`TestWireBridgeStages_ForwardsEachDialOnItsOwnSetter` + `TestWireBridgeStages_IsTheRootsOnlyStageForwarding`,
`TestProductionEngineDeps_CarriesBothRecoveryDials`, `TestNewDefault_SeedsRecoveryDialsFromPolicy`,
`TestSetFatalPaneStage_WiresField`, `TestExecAdapterDeps_CarriesThePolicyRecoveryDials` (the subagent root),
`TestBridgeRecoveryStages_ParsesThroughTheLoaderTrichotomy` (one parser), `TestLoad_FatalPaneStage` (no env
vocabulary), the policy/config resolution tables, the defaults goldens and
`TestDefaults_GateDialsMatchPolicyCompiledDefaults`.

**Next (F31, below).** Fast-failing a dead pane saves the wait but not the phase when the chain has nowhere
to go; F31 gives a session-recoverable cause one fresh session of the same CLI first.

### Amendment — F31 (2026-09-26): a session-recoverable fatal cause gets ONE fresh session of the same CLI

**Problem.** F27 made the C2 fast-fail act. A dead pane now leaves the wait within one interval instead of 900 s, but the launch still exits 81 and the caller's chain walks to the next CLI family. With codex quota-walled and ollama unable to write source, cycle 1687's triage had nowhere to go and aborted. A `dead_shell` means the REPL *process* is gone, not the CLI or the account, so a fresh session of the same family would very likely have finished the phase.

**Decision.** The bridge engine owns sessions, so it owns this retry. Every caller (phases, retro, advisor, judge) shares it without a second implementation in the runner's or `bridgechain`'s walks, and because the retry runs before classify and the boot-strike step, breakers and walkers only ever see the final outcome.
- **Which causes.** `recovery.TerminalCause.SessionRecoverable` (next to the cause vocabulary) is true for `dead_shell` and `cli_self_updated`. It is false for `model_invalid`, `unknown` and the zero value, because a fresh session of the same configuration fails the same way. (`cli_self_updated` relaunches immediately; if the updater has not finished writing the binary, the relaunch can hit a boot timeout, which the one-retry limit bounds.)
- **How the cause travels.** A preempting fatal verdict (enforce stage, non-Busy pane) carries its typed `Cause` on `ReviewVerdict`. The checkpoint reports a `fatalPaneObservation` through the call-local, package-private `onFatalPane` hook (the `onModelDispatch` pattern). The observation carries the typed cause, whether the session is named, and the wait interval the driver actually used, and `runScoped` captures it on the `launchRun`. The gate reads these facts from the driver instead of re-deriving them. Shadow and off never report a cause, nor do the quota-wall failover, headless drivers or dry-run.
- **The gate** (`Engine.freshSessionAllowed`, from the architecture review):
  - The cause is session-recoverable and the run ended on the fast-fail's exit, `ExitArtifactTimeout`. A run that delivered is never run twice.
  - The session is **ephemeral**. A named session (resume, the swarm's workers) is kept alive for resume, so re-running would reattach to the dead pane, skip boot and the sandbox, and paste the prompt into a bare shell. Its lifecycle belongs to its owner. Named-ness is the driver's one resolution (`resolveSession`: the request's session name, the `BRIDGE_SESSION_NAME` env or the profile), carried on the observation and never re-derived from the request.
  - The launch is live, and the caller's deadline leaves room for one more of the waits the driver actually used (the observation's interval). Otherwise the chain gets a clean exit 81 while it still has time to walk.
- **The retry.** `Engine.freshSessionRetry` records the dead dispatch on the ledger, marked `fresh_session_retry` (the two rows share `Attempt`). It clears the boot strike the dead run earned by booting, emits `BRIDGE_FRESH_SESSION_RETRY` (fields `call_id` of the dead dispatch, `cli`, `agent`), and runs the launch ONCE more as a new tmux session. There is **at most one retry**: the second run's cause is never read, and a second death returns exit 81 to the chain as before. The worst case for one chain attempt is the time to fast-fail plus one full wait budget, bounded by the caller's context.

**Known limits.**
- The dead run's escalation report and scrollback file are overwritten by the fresh run; its stderr survives on its ledger row.
- A REPL that dies before the prompt is accepted closes as `submit_wedged`, not `dead_shell`, and is not retried.
- If the runner also re-dispatches the same CLI on a transient exit, one rung can hold more than two same-CLI sessions; unverified.

**Evidence and pins.** The real `Engine.Launch` → `runScoped` → claude-tmux → checkpoint path is driven by:
- `TestEngineLaunch_DeadShellGetsOneFreshSessionAndSucceeds`: the dead session, then OK; two rows, the first marked.
- `TestEngineLaunch_ASecondDeathReturnsExit81ToTheChain`
- `TestEngineLaunch_NoFreshSessionWithoutEnforceOrForANamedSession`

These go red if `Launch` bypasses the retry, `runScoped` drops the cause, or the named-session guard goes (mutation-checked). Unit pins:
- `TestFreshSessionRetry_TheGate`: every refusal, including a delivered run and a tight deadline.
- `TestFreshSessionRetry_OneFreshSessionForASessionRecoverableCause`
- `TestRunTmuxREPL_FatalCheckpointReportsItsTypedCause`
- `TestFatalPaneVerdict_EnforceCarriesTheTypedCause`
- `TestTerminalCause_SessionRecoverable`

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
