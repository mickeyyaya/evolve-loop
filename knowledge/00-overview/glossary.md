# Glossary

> Every load-bearing term in Evolve Loop, defined once. Cross-links use relative
> markdown so you can walk the concept graph. For the big picture, start at
> [system-in-one-page.md](system-in-one-page.md).

---

### ACS (Acceptance Criteria Suite)

The on-disk collection of `EGPS` predicate scripts under `acs/`. Per-cycle
predicates live in `acs/cycle-N/`; shipped ones promote to `acs/regression-suite/`.
`evolve acs suite --cycle N` runs the suite and writes `acs-verdict.json`. The ACS
*is* the cycle's evidence — the prose audit merely narrates it.
See [EGPS](#egps-eval-gated-promotion-system), [acs-verdict.json](#acs-verdictjson).

### acs-verdict.json

The deterministic verdict file produced by running the ACS predicate suite. Its
`red_count` field is the ship gate: `red_count == 0` ships, anything else fails the
cycle. Contains `verdict`, `green_count`, `red_count`, `total_predicates`.
See [red_count](#red_count), [ship-gate](#ship-gate).

### Adapter (CLI adapter)

A thin translation layer that maps the framework's neutral phase request onto a
specific CLI's invocation (`claude -p`, `gemini -p`, `codex`). Each adapter accepts
the same env-var contract and emits the same usage envelope, so the orchestrator and
`ledger` never need to know which CLI actually ran.
See [bridge](#bridge), [pluggability](../../docs/concepts/pluggability.md).

### Adversarial audit

The default audit posture: the `Auditor` prompt is prefixed with "ADVERSARIAL AUDIT
MODE" framing that demands *positive evidence* for a PASS, and the Auditor runs on a
different model family from the `Builder`. Breaks same-model-judge sycophancy.
Disable only with `ADVERSARIAL_AUDIT=0`.

### Audit (phase)

The adversarial cross-check phase. Reads everything upstream plus `git diff HEAD`,
runs the predicate suite, and emits `audit-report.md` + `acs-verdict.json`.
See [Auditor](#auditor), [audit-binding](#audit-binding).

### audit-binding

The integrity rule that binds the `Audit` verdict to a specific tree state. The
auditor records the `git_head` and `tree_state_sha` it examined; `Ship` refuses
unless the current tree still matches. You cannot audit one tree and ship another.
See [ship-gate](#ship-gate), [ledger](#ledger).

### Auditor

The persona (`agents/evolve-auditor.md`) that runs the `Audit` phase. Read-only
(`read_only_repo` sandbox), different model family from Builder by default.

### Bridge

The CLI-agnostic dispatch layer in the Go binary (`go/internal/adapters/bridge/`)
that launches a phase on whichever CLI is routed for it, injects the interactive
policy + system-prompt prefix, and drives the chosen `adapter`. The seam that makes
"any CLI × any phase × any model" work.
See [adapter](#adapter), [recipe engine](#recipe-engine).

### Build (phase)

The single-writer implementation phase. The `Builder` writes the minimum code to
turn RED predicates GREEN, inside an isolated git `worktree`.

### Builder

The persona (`agents/evolve-builder.md`) that runs `Build`. Single-writer:
structurally excluded from parallel fan-out to preserve write discipline.

### Capability catalog

The declarative description of what each CLI/driver supports (budget caps,
permission scoping, native sandbox, non-interactive prompt, model flag, etc.), in
`*.capabilities.json`. The `bridge` reads it to choose native vs. driven (tmux)
execution and to refuse unsupported flags gracefully. Introduced with ADR-0031.
See [recipe engine](#recipe-engine).

### Checkpoint-resume

The heavy error-recovery layer: when a cycle is about to fail mid-flight (cost
spike, quota signature) the full worktree + `cycle-state.json` are checkpointed and
the cleanup trap is skipped, so `evolve loop --resume` continues from the paused
phase. Triggered at `EVOLVE_CHECKPOINT_AT_PCT` (default 95%) or by a quota signature.
See [error-recovery](../../docs/concepts/error-recovery.md), [worktree](#worktree).

### Commit-gate

The single chokepoint for interactive (non-cycle) commits. `evolve ship --class
manual` requires a fresh review attestation (`.commit-gate/attestation.json`) whose
`tree_state_sha` equals `sha256(git diff HEAD)`. Produced by the `/commit` skill.
Missing/stale → ship refuses. Bypass with `EVOLVE_BYPASS_COMMIT_GATE=1` (a policy
violation in routine use).
See [ship-gate](#ship-gate), [ship classes](#ship-classes).

### Cycle

One full pass through the phase sequence — the unit of work. Numbered sequentially;
each number must correspond to real Scout → Build → Audit → Ship → Learn execution.
State lives in `cycle-state.json`; history archives to `.evolve/history/cycle-N/`.
See [phase](#phase), [system-in-one-page.md](system-in-one-page.md#the-cycle-lifecycle).

### Dynamic routing

The Go routing kernel (`go/internal/router`) that lets the `PhaseAdvisor` compute a
whole-cycle plan and drive run/skip decisions instead of a fixed static sequence.
Rolled out in stages via `EVOLVE_DYNAMIC_ROUTING` (`off`/`shadow`/`advisory`/
`enforce`). Always clamped by the integrity floor. See [PhaseAdvisor](#phaseadvisor),
[integrity floor](#integrity-floor).

### EGPS (Eval-Gated Promotion System)

The discipline that makes verdicts deterministic: every acceptance criterion is an
executable predicate script whose **exit code is the verdict** (GREEN=0, RED≠0). No
LLM judges the outcome. Banned patterns (e.g. grep-only "AC-by-grep" assertions) are
rejected by `validate-predicate.sh`. The cycle ships only when all predicates are
GREEN. See [ACS](#acs-acceptance-criteria-suite), [red_count](#red_count).

### failedApproaches[]

The cheap, always-on error-recovery layer: a single-loop raw record appended to
`state.json` on every audit FAIL/WARN (`cycle`, `verdict`, `errorCategory`,
`expiresAt`). Drives the `failure-adapter`'s PROCEED/RETRY/BLOCK decision and ages
out after 30 days unless marked `systemic`.
See [instinct](#instinct--lesson), [error-recovery](../../docs/concepts/error-recovery.md).

### Fan-out

Running a read-only phase (Scout, Auditor, Retrospective, Plan-reviewer) as K
parallel sub-personas to diversify perspective. Single-writer roles are excluded by
`parallel_eligible: false`. Opt-in via `EVOLVE_FANOUT_ENABLED=1`.

### Instinct / Lesson

A `lesson` is a durable, evidence-bound YAML file
(`.evolve/instincts/lessons/<id>.yaml`) extracted by the Retrospective on FAIL/WARN,
carrying a `pattern` (for matching) and a `prevention` (imperative rule). Once
merged into `state.json:instinctSummary[]` it becomes an **instinct** that the next
cycle's `Scout` reads verbatim. This is the double-loop half of self-evolution.
See [self-evolution](../../docs/concepts/self-evolution.md), [Retro](#retro--memo).

### Integrity floor

The non-configurable routing constraint: `ship ⇒ build ∧ audit ∧ (tdd unless
trivial)`, forced into every plan even when an operator shrinks the configurable
mandatory set. The `PhaseAdvisor` cannot reach ship without a real build and audit.
See [dynamic routing](#dynamic-routing).

### Intent (phase)

The pre-Scout phase that structures a vague operator goal into 8 fields (goal,
non-goals, constraints, interfaces, acceptance, challenged premises, risk) plus an
Ask-when-Needed classifier. Output: `intent.md`. Opt-in via `EVOLVE_REQUIRE_INTENT`.

### Ledger

The append-only, SHA-chained audit trail (`.evolve/ledger.jsonl`). Each entry
records role, model, exit code, artifact SHA256, `git_head`, `tree_state_sha`,
`entry_seq`, and `prev_hash`. Tampering with any past entry invalidates every later
`prev_hash`. Verify with `evolve ledger` / `verify-ledger-chain.sh`. The system's
source of truth for "what actually ran."
See [hash-chain](#hash-chain), [audit-binding](#audit-binding).

### hash-chain

The `prev_hash` linkage between consecutive `ledger` entries that makes the ledger
tamper-evident. See [ledger](#ledger).

### Memo

See [Retro / Memo](#retro--memo).

### Phase

One stage of a `cycle` (Intent, Scout, Triage, Build, Audit, Ship, …). Each phase =
one persona, one perspective, one output artifact. Phase order is enforced at the OS
layer by `phase-gate`. See [cycle](#cycle), [persona](#persona).

### phase-gate

The Tier-1 PreToolUse hook (`phase-gate-precondition.sh`) that denies calling a
phase out of order or before its precondition state exists. Reads `cycle-state.json`
+ on-disk artifacts + ledger SHA — never the model's claim of progress.
See [trust kernel](#trust-kernel), [role-gate](#role-gate).

### PhaseAdvisor

The Go component (`core.PhaseAdvisor`) that, under `dynamic routing`, proposes the
cycle's run/skip plan. Its plan is clamped by the `integrity floor` and re-validated
by `CanTransition` before it can override the static successor.
See [dynamic routing](#dynamic-routing).

### Persona

A role definition in `agents/<role>.md`: prompt, single-purpose perspective, output
format, and tool allowlist (via `.evolve/profiles/<role>.json`). One of the three
pluggability axes. Personas never invoke each other — the orchestrator sequences them.
See [skill](#skill), [pluggability](../../docs/concepts/pluggability.md).

### Plan-Review (phase)

Opt-in four-lens review (CEO / Eng / Design / Security fan-out) of the plan between
Scout and Build. Output: `plan-review.md`. Gated by `EVOLVE_PLAN_REVIEW`.

### Recipe engine

The tmux-driven execution layer (ADR-0031) that lets the `bridge` drive a real
terminal session for CLIs lacking a clean non-interactive mode — sending keystrokes
per a `keyspec` recipe and reading scrollback. Backed by the `capability catalog`.
See [bridge](#bridge), [capability catalog](#capability-catalog).

### red_count

The number of RED (failing) predicates in `acs-verdict.json`. `red_count == 0` is
the hard ship condition (WARN was removed in EGPS v10). See [EGPS](#egps-eval-gated-promotion-system).

### regression-suite

The promoted set of predicates (`acs/regression-suite/`) that a shipped cycle
contributes. Every future cycle's audit must keep them GREEN — the cumulative,
self-reinforcing safety net. See [ACS](#acs-acceptance-criteria-suite).

### Retro / Memo

The post-ship learning phase. **PASS → Memo**: captures `carryover-todos.json` for
the next cycle. **FAIL/WARN → Retrospective**: extracts `lesson` YAMLs and a
`retrospective-report.md`. Auto-fires on FAIL/WARN (`EVOLVE_DISABLE_AUTO_RETROSPECTIVE`
to opt out). The engine of self-evolution.
See [instinct / lesson](#instinct--lesson).

### role-gate

The Tier-1 PreToolUse hook (`role-gate.sh`) on every Edit/Write that denies writes
outside the active phase's allowlist, outside the active `worktree` (for write-bound
roles), or to dangerous paths. Verdict from `.evolve/profiles/<role>.json` +
`cycle-state.json`. See [trust kernel](#trust-kernel).

### Sandbox

OS-level isolation (Tier 2): `sandbox-exec` on macOS, `bwrap` on Linux, wrapping
each `claude -p` subprocess to its role's allowed write paths. Auditor/Evaluator run
`read_only_repo`. Falls back gracefully when nested-Claude is detected — Tier 1 still
enforces. See [worktree](#worktree), [trust kernel](#trust-kernel).

### Scout (phase)

The discovery + planning phase. Reads the codebase, `state.json:carryoverTodos[]`,
and `instinctSummary[]`; cites research; proposes tasks. Output: `scout-report.md`.
The phase where prior cycles' lessons re-enter the loop.
See [instinct / lesson](#instinct--lesson).

### Ship (phase)

The phase that commits + pushes through `evolve ship` if `red_count == 0` and
`audit-binding` holds. Native Go implementation (`go/internal/phases/ship/`):
self-SHA TOFU, audit-binding, EGPS gate, atomic commit + ff-merge + push.
See [ship-gate](#ship-gate), [ship classes](#ship-classes).

### Ship classes

The verification profile selected by `evolve ship --class <X>`: **`cycle`** (default;
full audit-binding), **`manual`** (operator commits; skips audit-binding but requires
a `commit-gate` attestation), **`release`** (release pipeline; skips audit because
version-bump mutates files post-audit). See [commit-gate](#commit-gate).

### ship-gate

The Tier-1 hook (`ship-gate.sh`) that denies any git commit/push not routed through
`evolve ship`, plus force-pushes and out-of-worktree branch ops. The single chokepoint
that makes "merge without re-reading every diff" safe.
See [audit-binding](#audit-binding), [trust kernel](#trust-kernel).

### Skill

The imperative workflow recipe inside a persona (`skills/<name>/SKILL.md`): steps,
exit criteria, checklists. One of the three pluggability axes; multiple personas can
share a skill. See [persona](#persona), [pluggability](../../docs/concepts/pluggability.md).

### TDD (phase)

The predicate-first phase (default-on): a TDD-engineer writes RED predicates *before*
the Builder writes code, separating the predicate author from the implementer so the
Builder cannot weaken its own test. Gated by `EVOLVE_TEST_PHASE_ENABLED`.
See [EGPS](#egps-eval-gated-promotion-system).

### Triage (phase)

The cycle-scope bouncer that bounds Scout's backlog to `top_n` / deferred / dropped,
preventing scope blob. Default-on (`EVOLVE_TRIAGE_DISABLE` to opt out). Output:
`triage-decision.md`.

### Trust kernel

The composed enforcement system (Tier 1 structural integrity + Tier 2 OS isolation +
Tier 3 workflow defaults) that prevents the LLM from gaming the pipeline. Tier 1 is
non-negotiable; Tier 2 adapts; Tier 3 is operator-tunable.
See [trust-architecture](../../docs/concepts/trust-architecture.md),
[phase-gate](#phase-gate), [role-gate](#role-gate), [ship-gate](#ship-gate).

### Worktree

The fresh per-cycle git worktree (branched from `main`) that isolates a cycle's
Builder edits. Lives on a temporary `evolve/cycle-N` branch, deleted post-ship.
Preserved (not deleted) under `checkpoint-resume` so in-flight work survives a crash.
See [sandbox](#sandbox), [checkpoint-resume](#checkpoint-resume).
