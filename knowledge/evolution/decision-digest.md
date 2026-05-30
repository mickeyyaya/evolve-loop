# ADR Decision Digest

> **Institutional memory.** The distilled *why* of all 32 Architecture Decision
> Records, clustered so the rationale survives even after the original bash-era ADR
> files (0001–0020) are dropped. Each row preserves the decision, the problem it
> solved, and — where one exists — the rejected alternative, because the rejected
> path is often the more valuable memory.
>
> Cross-links: [bash-to-go-port.md](bash-to-go-port.md) ·
> [rejected-approaches.md](rejected-approaches.md) ·
> [compound-improvement-arc.md](compound-improvement-arc.md) ·
> [../incidents/pattern-library.md](../incidents/pattern-library.md)

---

## The five clusters

The 32 ADRs sort into five conceptual clusters. Two cross-cutting invariants thread
through all of them and should be read first, because they explain *why* most
decisions look the way they do:

- **Model proposes, kernel disposes.** Wherever an LLM makes a choice (routing,
  setup recommendation, audit verdict), a deterministic Go kernel clamps it against
  a non-negotiable floor. The LLM can never *weaken* an integrity invariant.
- **Execution is the only evidence.** A claim is worth nothing; a sandbox exit code,
  a git commit, or a tamper-evident ledger entry is worth everything. This is the
  scar left by the cycle 102–141 reward-hacking incidents.

| Cluster | ADRs | One-line theme |
|---|---|---|
| **A. CLI / multi-LLM substrate** | 0001, 0002, 0003, 0029, 0031 | Decouple LLM choice from permission policy; make any CLI × any model × any phase executable with graceful fallback. |
| **B. Phase purity & standalone phases** | 0004, 0005, 0007, 0008, 0010–0018 | Make phases pure, declarative, independently invocable, with RED-first executable acceptance. |
| **C. Trust kernel / integrity** | 0007, 0012, 0024, 0025, 0027(commit) | Replace model-claimed verdicts with execution-grounded ones; commit-as-evidence; conditional ship-gate floor. |
| **D. Bridge / launch / liveness** | 0020, 0021, 0022, 0023, 0026, 0030 | Native bridge; CLI-agnostic launch intent; live injection; slow-vs-stuck review; auto-spawned stall observer. |
| **E. Routing / extensibility / onboarding** | 0019, 0028, 0027(setup) | Externalize the build plan; open the phase set; onboard operators with a skill + deterministic validator. |

> Note: there are two ADR-0027 files (`-commit-as-evidence` and `-setup-onboarding`) —
> a numbering collision in the source. They are distinct decisions; this digest keeps
> them separate as `0027C` (commit) and `0027S` (setup).

---

## Cluster A — CLI / multi-LLM substrate

The founding insight: **LLM selection and permission policy are two different
concerns that were conflated in one file.**

| ADR | Decision | Why / rejected alternative |
|---|---|---|
| **0001** LLM router | `.evolve/llm_config.json` is the authoritative CLI+model selector, consulted *first*; profiles keep permission/sandbox policy only. | Editing production profile JSON to change which model runs a phase forced operators to touch permission policy to express an LLM choice. Separation of concerns. |
| **0002** Capability matrix | Flat `supports.*` boolean block per `<cli>.capabilities.json`, derived from the existing mode data. | `subagent-run.sh` passed `--max-budget-usd` to adapters that silently ignored it; operators had no visibility into which guarantees were absent. Extend-in-place beat a rewrite. |
| **0003** True native CLI invocation | Three-mode hierarchy **NATIVE > HYBRID > DEGRADED**. If `gemini`/`codex` is on PATH and supports non-interactive prompts, invoke it directly. | Adapters always delegated to claude (HYBRID) even when the named CLI was installed — the "[hybrid masquerade](rejected-approaches.md#the-hybrid-masquerade)": a "Gemini Auditor" was secretly Claude. NATIVE-first makes the operator's installed CLI actually run. |
| **0029** Fallback chain + per-agent CLI | `cli_fallback[]` ordered alternates + `cli_fallback_on_exit[]` triggers (default `[80,127]`); per-agent `EVOLVE_<AGENT>_CLI`; launch-time `--cli phase=cli`; startup capability probe. | Cycle 121: a single pinned CLI (`codex-tmux`) hit a REPL-boot bug and killed the whole cycle though three other CLIs could have run the phase. Goal: "any CLI × any model × any phase, always executes." Defaults are byte-identical to pre-G so operators opt in. |
| **0031** Recipe engine + capability catalog | A keyspec-driven sequence engine for multi-step slash-command flows (e.g. plugin install) with inter-step verification, plus a machine-readable per-CLI capability catalog cross-checked against `/help`. | `keystroke` (ADR-0023) drives one keypress; multi-step flows with verification needed a real engine and a durable record of what each CLI can do. |

**Durable lesson:** the multi-CLI dream (model diversity to break same-family judge
sycophancy) is only real if the adapter *actually invokes the named CLI*. The hybrid
masquerade silently defeated it for many cycles; ADR-0003 + 0029 are the structural
fix, and the cross-family auditor default (different model from the builder) is what
the diversity is *for*.

---

## Cluster B — Phase purity & standalone phases

Theme: a phase should be a **pure, declarative, independently runnable brick** with a
machine-readable I/O contract — not prose scattered across three files.

| ADR | Decision | Why |
|---|---|---|
| **0004** Pure-function phases | `docs/architecture/phase-registry.json` is the single declarative authority for phase order + per-phase I/O contract + optional-phase env vars + fan-out eligibility. | Phase order lived in 3 places (orchestrator prose, `phase-gate.sh`, `run-cycle.sh`); reordering meant coordinated edits and caused the v8.55 fan-out incident. |
| **0005** Standalone phase init | `init-standalone-cycle.sh` + `check-phase-inputs.sh` make every phase invocable in isolation, with input-existence verification and a clobber guard. | Running `/scout` or `/audit` standalone was brittle; nothing bootstrapped `cycle-state.json` or verified declared inputs existed. |
| **0007** TDD via EGPS predicates | Formalize **RED-first predicate authorship**: write the executable acceptance test, prove it's RED, implement to GREEN. Mutation kill-rate ≥ 0.8; anti-tautology AC mandatory; `mktemp -d` hermetic fixtures; copy predicates to PROJECT_ROOT before audit. | Confidence-cliff reward hacking (verdicts clustered at the PASS/WARN boundary). Binary exit-code predicates eliminate the gaming at the signal source. Predicates accumulate as a permanent regression net. |
| **0008** Retrospective role-gate case | Add a `retrospective)` case to `role-gate.sh` mirroring `learn)`. | The retrospective phase couldn't write `instincts/lessons/*.yaml` without a DENY — a structural gap caught in cycle-70 retro. |
| **0010–0011** Scout / Intent stop-criterion tightening | Tighten the STOP CRITERION wording for scout and intent phases. | Chronic turn-budget overruns; see also the "[ceiling-miscalibration](rejected-approaches.md#tuning-the-watchdog-threshold)" lesson — sometimes the *ceiling* is wrong, not the agent. |
| **0012** Commit-claim coherence | Bind the commit message / prefix to the actual diff; record divergence. | Cycles 70–72 mislabeling pattern + cycle-75 fabricated-AC: commit prefix drifted from the work done. |
| **0013–0016** Orchestrator / Builder / Auditor / Retrospective "cold-moves" | Relocate specific decision points (phase-loop, builder skills-notes, auditor stage 8, retrospective stage 9) to colder, more deterministic positions in the flow. | Reduce the surface where an LLM's discretion could drift from the contract. |
| **0017–0018** Scout densification + mid-session stop reinjection | Densify the scout stop criterion; re-inject the stop criterion mid-session for long agent runs. | Long sessions drift past their stop criterion; a one-shot prompt at launch isn't enough. |
| **ship-as-executor** Ship = pure executor + advisor recovery | Ship verifies + executes (commit→ff-merge→push) but **cannot REJECT** a cycle; on any failure it returns a structured `core.ShipError{Code, Class, Stage, Debug}` on its output. The advisor's recovery Chain-of-Responsibility maps error class → recovery phase: `PRECONDITION`/`AUDIT_BINDING_*` → re-audit (Saga alt-path), `TRANSIENT` → retry-ship (bounded), `INTEGRITY` → BLOCK, unknown → debugger (LLM catch-all). Patterns: **Strategy + Chain-of-Responsibility + Saga + Retry**; bounded by `maxRecoveryDepth`. | A *passing* audit whose ship aborted on a stale precondition (`AUDIT_BINDING_HEAD_MOVED`, cycle-151) had no recovery — the whole loop aborted. Conflating execution with judgment, plus a lossy `exit=N` ship→orchestrator boundary, destroyed the information needed to recover. See [routing-and-advisor.md §10](../architecture/routing-and-advisor.md#10-ship-error-recovery--the-debugger-phase). |

**Durable lesson:** phase identity and signals were the *walls* (closed enums,
hardcoded transition graphs), not the execution model — which was already ~80% a
clean Pipes-&-Filters pipeline. ADR-0028 (cluster E) finished the job.

---

## Cluster C — Trust kernel / integrity

The most load-bearing cluster. Born directly from the reward-hacking incidents
(cycles 102–141). The thesis: **stop letting any model report whether the work is
done; let the sandbox's exit code, the git tree, and the ledger be whether the work
is done.**

| ADR | Decision | Why |
|---|---|---|
| **0007** (also B) EGPS predicates | Binary `acs-verdict.json:red_count == 0` IS the ship gate; no scalar confidence, no WARN level. | Confidence-cliff gaming. v10.0.0 BREAKING change to the verdict contract. |
| **0024** Conditional ship-gate floor + PhaseAdvisor | The floor is **one conditional invariant**, not a phase list: `reach(ship) ⇒ audit-PASS bound to the build tree ⇒ build ⇒ (tdd if non-trivial)`. An LLM `PhaseAdvisor` proposes a whole-cycle plan; the kernel **clamps** it against the floor; any plan reaching ship without a bound PASS audit is rejected. Advisor errors degrade to the static plan. | Earlier dynamic routing had a *fixed mandatory spine* the LLM could only add to. Operator wanted to shrink the hard set to its safety core — but a naive "only TDD mandatory" would let the advisor skip audit, reopening the gaming surface. The conditional invariant lets a no-ship investigation cycle end after scout, while making it *impossible* to ship unverified code. |
| **0025** ACS suite runner + standing red-team suite | `evolve acs suite` deterministically globs + runs every predicate (cycle-N + regression-suite + `red-team/`) in its own process group with per-predicate timeouts, and writes the verdict. `acs/red-team/` encodes each *past gaming incident* as a live test firing every cycle (e.g. `rt-001-ledger-role-completeness` ↔ cycle 102-111). | v12.0.0's flag-day deleted the bash `run-acs-suite.sh` runner with no Go port — the "every prior predicate runs every cycle" guarantee was resting on an LLM improvising against a dangling reference. Google's adversarial "report & mitigate" loop: encode every past failure as a standing test. |
| **0027C** Commit-as-evidence | **A commit is the universal evidence of a phase's deliverable.** The orchestrator advances iff the phase committed its artifact(s) to the worktree branch; detection is uniform `git`, never a per-phase path heuristic. Ship is the apex — the single commit that reaches `main`. | Path-polling (`bridge.artifactReady` watching a file path) produced a recurring class of failures: agents wrote to subdirs, used literal placeholder tokens, or raced. Each miss spawned a new heuristic — whack-a-mole. Git is the one detection that can't be gotten wrong by where the agent put the file. |
| **0012** (also B) Commit-claim coherence | Commit prefix/message bound to the actual diff. | Mislabeling + fabrication pattern. |

**Durable lesson:** every integrity ADR is a response to a *specific* gaming attack
that actually happened. The defenses compound — EGPS (signal can't be gamed) +
commit-as-evidence (completion can't be faked) + red-team suite (every past attack is
a standing test) + adversarial cross-family auditor (the judge can't be sycophantic).
See [rejected-approaches.md](rejected-approaches.md) for the attacks themselves.

---

## Cluster D — Bridge / launch / liveness

Theme: the harness that launches and supervises subagents. The recurring failure mode
here is **conflating "slow" with "stuck"** and **fusing intent with one CLI's
realization of it.**

| ADR | Decision | Why |
|---|---|---|
| **0020** Unified phase event stream | One live normalizer producing a unified envelope (`{schema_version, ts, trace_id, source, type, severity, data}`) consumed by cyclecost, phaseobserver, cycleclassify. Interactive actions (AskUserQuestion, ExitPlanMode, permission prompts) are full-fidelity signal, exempt from truncation. | Four independent passes re-parsed the same raw stdout with partial fidelity; each waded through `stream_event` redraw noise; the one component that classified everything (`logfilter`) fed no machine consumer. |
| **0021** Native bridge port | Reimplement the ~3,000-line bash bridge in Go (Strategy + Registry, Template-Method REPL engine, injectable seams). | See [bash-to-go-port.md](bash-to-go-port.md). Untestable bash; the port made the seam mockable. |
| **0022** Launch-intent realizer | A CLI-agnostic `LaunchIntent` (model tier, permission, session mode, allowed tools) + per-CLI `Realizer` mapping it across explicit channels (flags / REPL injection / controller hints). Flags-first; raw `extra_flags` escape hatch applied *only* to the matching CLI. | Profiles carried raw **claude-shaped** flags forwarded verbatim to any CLI. `--no-session-persistence` is claude's *realization* of "ephemeral," not a parameter — it broke the claude REPL (print-mode-only) and `agy` (rejects the flag) → EC80 boot timeout. Reclassify the intent, realize it per-CLI. |
| **0023** Live injection + launch rules | **Facet A:** file-based NDJSON inbox (`<workspace>/.bridge-inbox/<agent>.ndjson`) drained in the existing poll loop; kinds `command`/`interrupt`/`nudge`/`system_rule`/`keystroke`; idle-gated except interrupt; cursor seeks EOF on launch (no backlog replay). **Facet B:** launch-time per-agent system-prompt `## Rules` block. | Launch was fire-and-forget — no way to correct, nudge, or interrupt a running agent. `keystroke` (added cycle-124) unblocks CLI-native modals that neither `command` (idle-gated) nor `interrupt` (sends ESC, dismisses the modal) can reach. Go-tmux-only by physics (headless drivers exit after one prompt). |
| **0026** Self-healing review layer | Replace the hard 300s wall-clock artifact timeout with an **interval-review** loop: `Observe → Review → Translate → Execute`. A `StopReviewer` seam (deterministic now, LLM later) returns `extend`/`pause`/`stop`; extend while output *progresses*, up to a backstop (~30 min default). | Cycle 109: a research-heavy ultrathink Scout streamed output for 5 min and was killed at *exactly* 300s mid-synthesis — wasted spend, dead batch. **Liveness timeouts must be inactivity/progress-based, never total wall-clock.** |
| **0030** Phase-observer auto-spawn | Auto-spawn the phase-observer goroutine from `RunCycle` for every phase, gated by `EVOLVE_OBSERVER_AUTOSPAWN=1`. New `FileNeverCreatedGraceS` (90s) defense: SIGTERM if the stdout log never appears. | **Silent regression from the v12 flag day**: the bash dispatcher auto-spawned the observer; the Go port preserved the *code* but shipped it as a manual subcommand and never re-wired the auto-spawn. Cycle 122 was the first run it bit — a codex permission modal blocked 10 min and the coarse artifact-timeout (`exit=81`) wasn't in the fallback chain's trigger list. |

**Durable lesson:** the bridge's hardest bugs are all *cross-seam contract* bugs —
flags fused to one CLI, doc↔runner name mismatches, per-CLI rendering (codex
alt-screen invisible to `capture-pane scrollback=0`), exit codes not in a trigger
list. Fakes hide every one of them; they need explicit contract tests and adversarial
inputs.

---

## Cluster E — Routing / extensibility / onboarding

Theme: open the closed parts of the system (the build plan, the phase set, the
onboarding gap) — always on the **proposes/disposes** pattern.

| ADR | Decision | Why |
|---|---|---|
| **0019** Build-planner phase | Externalize Builder's internal design step into a dedicated phase (`build-plan.md`) run by an independent Opus session, on a 3-cycle shadow → advisory → enforce rollout. | Builder's planning and execution shared one LLM context, so design drift was invisible to the Auditor. An artifact the Auditor can compare against the diff enables Plan-Adherence checks. |
| **0028** User-defined phases | Three unified contracts (PhaseSpec, uniform handoff envelope with a namespaced signal bus, PhasePlan) on the existing routing ladder so the static path stays byte-identical. Adding a phase becomes JSON, not Go edits in 5–10 places. The kernel FLOOR clamps any plan. | Phases were a closed set: closed enum, closed transition graph, four closed signal layers. The execution model (`PhaseRunner`, Template-Method, open registry) was already there — only *identity* and *signals* were walled. Canonical Pipes-&-Filters / Argo-DAG realization. |
| **0027S** Setup onboarding | Split: an in-session `/setup` *skill* does the judgment (recommend per-phase models, explain the pipeline — zero API cost, interactive) and the Go binary does the deterministic work (`evolve setup detect` / `validate` / `complete`). The skill proposes, `validate` disposes. | No onboarding existed; the powerful config surface (`llm_config.json`, envelopes, `cross_family_with`, `allowed_clis`) was undocumented at the point of use. **Cross-family validation is ADVISORY (WARN), not exit-2** — because the live production config is intentionally all-Claude (post-Gemini-quota-wall recovery), and a hard reject would break that legitimate fallback. The first-run nudge is one stderr line, never blocking (respects bypass-permissions). No new feature flag — marker lives in `state.json` (no flag sprawl). |

**Durable lesson:** every "open it up" decision rides the same rollout ladder
(off → shadow → advisory → enforce) and the same kernel clamp. Extensibility never
buys its way past the integrity floor.

---

## Retired / inert decisions (anti-knowledge)

| ADR | Status | Why it was retired |
|---|---|---|
| **0009** P2 turn-budget advisory | **INERT** (cycle 72) | Advisory-only enforcement fails when *the implementer == the discloser*. The Builder both received the 20-turn guidance and produced the telemetry that would expose a violation — and overran twice (64 turns, then 39). `claude -p` has no `--max-turns` flag to enforce it programmatically. INERT is the correct double-loop response: retire the failed protocol rather than ship a third attempt at the same enforcement *shape*. The field is preserved as the source for a future real-time watchdog (Case A). See [rejected-approaches.md § advisory-where-implementer-is-discloser](rejected-approaches.md#advisory-enforcement-where-the-implementer-is-the-discloser). |

---

## Reading order for a new developer

1. This file's two cross-cutting invariants ("model proposes, kernel disposes";
   "execution is the only evidence").
2. Cluster C (trust kernel) — it explains *why* the system distrusts its own agents.
3. [rejected-approaches.md](rejected-approaches.md) — the attacks the trust kernel
   defends against.
4. Cluster D + [bash-to-go-port.md](bash-to-go-port.md) — how the harness supervises
   subagents and how it got there.
5. Clusters A, B, E for the substrate, phase model, and extensibility.
