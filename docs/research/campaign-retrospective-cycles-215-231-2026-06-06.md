# Campaign Retrospective — Cycles 215–231: Every Defect, Its Root Cause, and the Architecture That Prevents the Next Ones

**Date:** 2026-06-06
**Scope:** wave-1 micro-phase catalog delivery (215–218) → dynamic-advisor first run (220–224) → v16.6.0 release → 15-cycle architecture campaign (225–231, paused for this retrospective)
**Verdict in one line:** the trust kernel was never breached and the advisor composed intelligently — but **12 distinct defects** (6 of them one disease) cost ~9 operator interventions and 5 lost cycle-runs, and they share a single architectural root: *replicated beliefs without a coherence protocol*.

Companion docs: [dynamic-advisor-first-run-retrospective-2026-06-05.md](dynamic-advisor-first-run-retrospective-2026-06-05.md) (cycles 220–224 detail) · [micro-phase-catalog-research-2026-06-05.md](micro-phase-catalog-research-2026-06-05.md) · `docs/architecture/micro-phase-catalog.md`

---

## 1. Incident catalog

Format per entry: **what happened → evidence → why (instance-level) → immediate fix → status**.

### I-1 Stale regression predicates (cycle 215)
Six regression predicates red on a clean tree: three asserted content lives in CLAUDE.md after the intentional docs split (`d8ac721`) moved it to `docs/operations/runtime-reference.md`; one (`cycle-84/002`) asserted `carryoverTodos == []`, contradicting the sanctioned queue workflow; one meta-predicate cascaded; one was context-dependent.
**Evidence:** `.evolve/runs/cycle-215/acs-verdict.json` (red_count=6); audit-report Issues H1.
**Why:** assertions encoded a *location* of truth, not the truth itself; manual/docs ships bypass the ACS suite so the break surfaced one cycle late.
**Fix:** intent-preserving re-baselines, reviewer chain, `5cdb864`. **Status: closed.**

### I-2 Gitignore root-`.evolve/` shadow (latent for months; defused before cycle 217's ship)
`**/.evolve/` (v8.21.0, meant for *nested* worktree state) also matched the ROOT `.evolve/`, dead-lettering every whitelist beneath it — new `phase.json`/`agent.md`/profile files were unstageable without `git add -f` (the historical force-add workarounds were the symptom). Ship's plain `git add -A` would have **silently dropped cycle 217's primary deliverables**.
**Evidence:** `git check-ignore -v` traces; `git add --dry-run` proof; fix shipped inside `a354d85` (`*/**/.evolve/` + `!.evolve/phases/*/agent.md`).
**Why:** two declarative layers (ignore rules, ship staging) composed without any check that the builder's outputs were stageable.
**Status: closed** (fix validated by `a354d85` itself shipping 7 agent.md files).

### I-3 → I-8 The six phase-identity drift modes (cycles 220, 221, 222, 224, 225, 226–227)
One disease, six expressions — a phase's identity is hand-authored on ≥6 surfaces that can disagree:

| Mode | Cycle | Surface pair that disagreed | Crash signature |
|---|---|---|---|
| (a) persona path | 220, 224 | loader reads `agents/evolve-<name>.md` only; documented authoring path `.evolve/phases/<name>/agent.md` is never read | `load agent: no such file` |
| (b) runner binding | 221 | runner registry built at BOOT from persona presence; runtime catalog mutable | `no runner registered for phase` |
| (c) profile name | 222 | resolution by phase-NAME; built-ins ship ROLE-named profiles (`spec-verify` ≠ `spec-verifier.json`); user phases had none | `bridge launch exit=10` |
| (d) artifact name | 225 | persona instructed `plan-review.md`; contract polls `plan-review-report.md` | `artifact timeout exit=81` ×2 → batch-fatal |
| (e) suite root | 226–227 | auditor LLM improvises `acs suite -root`; evaluated MAIN while changes lived in the worktree | false `red_count=8`, two correct builds FAILed |
| (f) registry path | 226 | predicate expected `.evolve/phase-registry.json`; registry lives at `docs/architecture/` | predicate red |

**Evidence:** failure diags in `.evolve/runs/cycle-{220,221,222,225}*`; `cycle-227` audit-report Eval Results (`-root /Users/.../evolve-loop` with all 8 ACs red on missing symbols); carryover todo `user-phase-persona-resolution` (all modes documented as they were found).
**Why (instance):** each surface was added at a different epoch (agents/ dir predates ADR-0028; profiles predate user phases; contracts unified in ADR-0035 but resolution never was).
**Fixes:** operator bridges `306c4f0` (11 personas + 14 profiles), `9218262` (artifact name), `48f8ff7` (C0 suite-root pin); **the systemic fix was then implemented BY the loop and triple-audit-PASSED — branch `cycle-230` @ `201f7cb`** (`AgentForPhase` two-path resolution, `clampDispatchable` plan-time clamp, `Resolve(name,role)` fallback, `PersonaArtifactDrift` check, `userPhaseNameRE` lint, `LedgerEntry.Source`, `ResolveRegistryPath` seam) — unshipped only because of I-10. **Status: bridged everywhere; root fix audited, awaiting landing.**

### I-9 Batch-fatal failure domain + uncheckpointed crashes (cycles 225, 230, 231)
A single phase bridge error kills the entire N-cycle batch (`rc=2`), while *content* failures correctly fail only the cycle — inverted severity. Hard crashes write no checkpoint, so `--resume` refused every time ("no live checkpoint"); cycle 231 ended with the dispatcher dying **silently** mid-audit (no exit summary, orphaned codex pane) — incident #13 in its own right.
**Evidence:** c225/c230 dispatcher tails (`stop_reason: error`); three `resume: no live checkpoint` attempts; c231 reset seal `cycle-231.reset-20260606T011627`.
**Why:** failure handling grew ad-hoc per path; only the quota wall got checkpointing; no supervision hierarchy.
**Status: open** (Invariant 3).

### I-10 Workspace-vs-worktree write collision (cycle 226 leak; cycle 230 ship abort)
Phases with `writes_source=false` (tdd predicates, bug-reproduction, test proposals) write files into the **main repo tree** (their workspace); when the worktree ships the same paths, `ff-merge` aborts on "untracked would be overwritten" — and the orchestrator recovery-loops audit↔ship with no budget: cycle 230 recorded **3 PASS audits and 0 ships**. Cycle 226's variant: builder content appearing on main (stashed: `cycle-226 FAILED-audit builder leak`).
**Evidence:** c230 dispatcher tail (`GIT_FF_MERGE_DIVERGED (precondition) → recovery routes to audit`, PhasesRun `[...audit,audit,audit]`); colliders salvaged to `/tmp/evolve-c230-salvage/`.
**Why:** three trees (main workspace, run dir, cycle worktree) with phase-dependent write targets and **no ownership model**; ship discovers the conflict last and has no pre-flight.
**Status: open** (Invariant 2; also queued as drift mode 6 in the todo).

### I-11 agy weak-signal stalls (cycles 227, 229, 230 — 3/3 builder attempts)
The agy CLI's interactive rating dialog (`[1] Good … [0] Skip`) or idle-without-artifact ends consume the agent's run; bridge artifact-poller times out (`exit=81`). Cross-CLI fallback to claude-tmux rescued every instance at a **~10 min/cycle latency tax** — the campaign's single largest latency loss (user's P1).
**Evidence:** `fallback 2/2: trying cli=claude-tmux (previous=agy-tmux exit=81)` ×3; operator keystroke unblock at 03:57.
**Why:** the bidirectional channel's per-CLI profiles (PR #59) deliberately deferred "weak-signal" handling (§6 follow-ups).
**Status: open** (migration step 6).

### I-12 Self-inflicted gate trip (cycle 229)
The operator's C0 suite-root block pushed `agents/evolve-auditor.md` to 319 lines; regression predicate `cycle-77/001-auditor-stage8-cold-move` enforces ≤300 (persona token budget). red_count dropped 8→1 — the last red was the fix for the previous red.
**Evidence:** c229 audit Critique quoting "319 lines, above the 300-line ceiling".
**Why:** gates exist only in the cycle-audit path; nothing checks persona edits at authoring/commit time.
**Fix:** cycle 230's builder slimmed the persona in-worktree (in `201f7cb`). **Status: instance closed; class open (Practice 5).**

### I-13 Positive control — the gates that fired correctly
Not defects, included as evidence the kernel held: c223 audit caught a stale challenge token (builder cribbed sealed c220 artifacts) + a grep-only window-dressing predicate; c226 caught ungrounded "75/75 PASS" build narratives; ship-gate blocked every unsound merge; `ClampPlanToFloor` silently overrode every unsafe advisor vote (scout-skip ×2). **Zero trust-kernel breaches in 17 cycles.**

### Minor ledger (one line each)
Contract-gate "no contract registered, failing open" WARNs for ship/plan-review/tester/bug-reproduction/mutation-gate (fail-open ambiguity — Practice 5); tracked `go/evolve` binary leak (twice restored); `.evolve/inbox/processing/` unignored (fixed in `306c4f0`); plan-vs-trigger precedence undefined (advisor `run:true` on mutation-gate dropped by unfired `insert_when`, c224); `fail_if_signal` silently inert pre-Stage-3 (every wave-1 threshold gate is decorative); "spine not satisfied … proceeding fail-open" WARNs under advisory routing.

---

## 2. Root-cause classes (instance → class)

| Class | Incidents | One-line cause |
|---|---|---|
| A. SSOT violation | I-3…I-8 | phase identity hand-authored on ≥6 surfaces; every disagreeing pair = a latent crash |
| B. LLM computes what the kernel knows | I-7(d), I-8(e) | load-bearing values (roots, artifact names) re-derived by agents instead of injected |
| C. Topology without ownership | I-2, I-10 | three trees, phase-dependent write targets, no enforced map |
| D. Failure not a domain | I-9, I-10 loop, I-11 | ad-hoc per-path handling; inverted severity; no budgets; no checkpoints |
| E. Boot-time binding, runtime-mutable catalog | I-4(b) | runners bind once; catalog changes any time |
| F. Gates outside the authoring path | I-12, fail_if_signal | enforcement happens cycles later than the mistake |

## 3. The unifying diagnosis

The pipeline is a **distributed system masquerading as a config-driven monolith**: LLM agents (3 CLI families) × Go kernel × three filesystem trees × git × declarative configs are independent actors holding **replicated, independently-authored beliefs** about shared entities — a phase, a cycle's topology, a gate's semantics, "what ships". There is no coherence protocol. Every incident above is a consistency failure between two replicas of belief; the six drift modes are the same bug six times. Patching instances (which this campaign did nine times, by hand) converges only when the *classes* become unrepresentable.

## 4. Architecture: three invariants + two practices

> Design test for every future change: *"could two surfaces disagree about this?"* If yes, one of them must become derived, scoped, or verified.

### INVARIANT 1 — Single Derivation Root (author once; everything else is a projection)
Canonical models: `PhaseDescriptor` (registry ⊕ phase-dir merge), `CycleTopology` (trees/branch/roots for cycle N), `GateSpec`. Persona path, profile, contract, artifact name, prompt-injected paths, suite invocation, runner registration, and the human-facing tables all become **derived projections** through one Resolver façade — never parallel hand-authored files.
- **Retires:** drift modes a–f.
- **Prevents (pre-mortem, several already latent):** persona `tools:` ↔ profile `allowed_tools` contradictions (live instances found by reviewers this session, not yet fired); persona model-tier ↔ profile tier; `gate_in/gate_out` refs ↔ state-machine edges; descriptor schema ↔ registry drift; skill-docs ↔ runtime drift (the `/publish` skill still teaches the legacy pipeline); rename fan-out misses (the `bug-reproduction` rename hand-chased 7 files; under projections: 1 edit).
- **Meta-gate:** CI predicate — no literal artifact filenames inside `agents/*.md`; profiles carry `generated-from` provenance or fail.
- **Seed:** `201f7cb` is phase 1, already triple-audited. Land it first.

### INVARIANT 2 — Capability-scoped effects (handles, not paths)
Actors act through scope-encoding handles: `WorkspaceHandle` (run dir — all evaluate-phase proposals), `WorktreeHandle` (builder), `KernelHandle` (ship/main). Writes, git ops, and suite invocations flow through the handle; **the handle, not the prompt, resolves roots**. Ship stages what the worktree handle tracked and pre-flights main-side colliders with an actionable error.
- **Retires:** I-2 class, I-8(e), I-10.
- **Prevents:** swarm-stage parallel-writer races (`EVOLVE_SWARM_STAGE=enforce` is on the roadmap); concurrent batches sharing main; audits reading half-merged trees; operator↔cycle simultaneous-edit hazards (created twice this session — the ff-merge fragility exists only because effects aren't scoped).
- **Meta-gate:** role-gate denies any mutation resolving outside the actor's granted root — kernel denial, not audit finding.

### INVARIANT 3 — Failure is a supervised domain (OTP-style supervision tree)
attempt (CLI retry → cross-CLI fallback) → phase (correction budget 2) → **cycle** (fail → retro → batch continues) → batch (pause + checkpoint, always resumable). Checkpoint at **every** phase boundary; every recovery edge carries a traversal budget (c230's audit↔ship loop becomes impossible); classification by failing layer, not stderr string-matching; supervised events in the ledger (extends `LedgerEntry.Source` from `201f7cb`).
- **Retires:** I-9 (incl. the silent c231 dispatcher death leaving no trace), unbounded recovery loops, resume-impossible.
- **Prevents:** quota wall mid-ship; dispatcher OOM with stale locks; observer false-kills during long mutation runs; every future novel failure landing in a defined layer.
- **Meta-gate:** test asserting every orchestrator recovery edge declares a budget.

### PRACTICE 4 — Verified handoffs (generalize the challenge token)
Every artifact carries `{cycle, phase, tree_sha, inputs_digest}`; consumers verify before use; report claims must cite digests gates independently recompute. Makes the "ungrounded narrative" class (c223/c226) and stale-artifact reuse (c224) structurally detectable instead of auditor-vigilance-dependent.

### PRACTICE 5 — Declared semantics are load-bearing or rejected
A spec field either has a kernel implementation or validation rejects it loudly (`fail_if_signal` → "requires Stage-3 signal bus"). Invariant-1 projections also run in the commit gate (persona line budget, contract-name echo, frontmatter↔profile coherence) so edits can't trip cycle gates post-hoc (I-12 class).

## 5. Migration path (each step independently shippable)

| # | Step | Lands | Retires |
|---|---|---|---|
| 1 | Land branch `cycle-230` @ `201f7cb` (triple-audited) — via one clean cycle or gated manual ship | resolution fallbacks, dispatchability clamp, naming lint, `Source` field | drift modes a–f recurrence |
| 2 | Kernel-resolved suite root + run-dir proposals + ship collider pre-flight | Invariant 2 minimal | I-8(e) deterministically, I-10 |
| 3 | Phase-boundary checkpoints + recovery budgets + cycle-level bridge-failure classification | Invariant 3 minimal | I-9 |
| 4 | Profile/prompt projection generation + meta-gates | Invariant 1 full | latent drift family |
| 5 | Provenance headers + authoring-path gates | Practices 4, 5 | I-12/I-13 classes |
| 6 | Bridge dialog auto-respond profiles (per-CLI weak-signal table) | — | I-11 (~10 min/cycle) |

`dynamic_routing: advisory` becomes the persistent default **only after steps 1–3**; until then advisory stays supervised-run-only.

## 6. What worked (keep, and lean on)
Trust kernel (zero breaches; every unsound merge blocked); advisor composition (all-19-phase justified plans; failure-adaptive recipes — inserted spec-verify after a grounding failure, adversarial-review after an audit FAIL; correct bugfix-recipe discrimination); cross-CLI fallback (3/3 rescues); correction retries; the new catalog phases produced real findings on first runs (test-amplification's `gte`-vs-`gt` catch); the campaign's *failures themselves* were the highest-value architecture review — every defect it surfaced was in the user's stated P1 scope.

## 7. Scorecard

| Metric | Value |
|---|---|
| Cycles run (215–231, incl. seals) | 17 started, 12 full pipelines |
| Shipped | 4 (215-era predicates fix → `5cdb864`*, cycles 217 `a354d85`, 218 `120c805`; release `897880a`) + 4 operator commits (`306c4f0`, `da3e772`, `0149d81`, `9218262`, `48f8ff7`) |
| False-FAILed correct builds | 2 (c226, c227 — topology bug) |
| Triple-audited-but-unshipped | 1 (`201f7cb` — awaiting step 1) |
| Operator interventions | 9 (4 cycle resets, 2 batch stops, persona/profile bridges, collider salvage, dialog keystroke) |
| Defects found / closed / open | 12+ / 7 / 5 (mapped to invariants) |
| Cost | $0.00 metered (subscription); ~9 h wall-clock |

\* operator-driven between cycles.

## 8. Open decision (user)
Resume the remaining campaign cycles now, or land migration steps 1–3 first? **Recommendation: land step 1 (`201f7cb`) + step 2 in one or two supervised cycles, then resume the campaign** — the remaining 10 cycles then run on rails that none of this report's incidents can derail.

## Addendum (post-pause, found while landing this report)

**I-14 — Cache-masked assertion drift:** `TestLoad_RealRegistry` expected `MaxInsertions=4` and has been broken since cycle 217 intentionally raised the registry to 6 — masked locally by Go's test cache (the release's "146/146 ok" verification was partially cache-grounded; CI's uncached race run exposed it). Same class as I-1 (assertion encodes a moved truth) plus a verification lesson: release preflight and operator test runs must use `-count=1`. Fixed: expectation updated to 6 with provenance comment.

**I-15 — `ship --class manual` stages `-A`:** the docs-only retro commit (`faafbab`) silently swept cycle-231 residue (5 unaudited predicates + a RED Go test) and a locally-rebuilt `go/evolve` binary into the repo; removed in `342a202`. Fresh I-10/Practice-4 evidence: ship-by-belief ("the tree is exactly my commit") instead of ship-by-manifest. The Invariant-2 handle model (ship stages what the handle *tracked*) retires this class too.

**I-12 closure:** the persona line budget is now met on main (auditor persona compressed to exactly 300 by merging the duplicated C0/C1 worktree blocks — DRY fixed the budget), `cycle-77` predicate + Go twin both green.

## Appendix — artifact index
Reset seals: `.evolve/runs/cycle-{216,219,220,221,222,224,225,228,230,231}.reset-*` · Salvage: git stash "cycle-226 FAILED-audit builder leak", `/tmp/evolve-c230-salvage/`, branch `cycle-230`@`201f7cb` · Carryover todos: `phases-quality-gates`, `phases-release-and-memory`, `user-phase-persona-resolution` (6 modes; superseded by §4–5 of this report)
