# Retrospective — Dynamic Advisor First Live Run (cycles 215–224)

**Date:** 2026-06-05
**Operator session:** micro-phase catalog delivery + first `EVOLVE_DYNAMIC_ROUTING=advisory` cycles
**Verdict:** Wave 1 shipped and advisor-validated; three systemic pipeline defects found and fixed (one operator-committed, two bridged + queued); the user-phase dispatch chain needs the queued root fix before advisory mode is reliable unattended.

---

## 1. Timeline

| Cycle | Mode | Outcome | Notes |
|---|---|---|---|
| 215 | static | audit **FAIL** (red_count=6) | 6 stale regression predicates — pre-existing, outside the wave-1 diff |
| 216 | static | sealed at scout (operator) | predicates re-baselined → `fix(acs)` commit `5cdb864`; suite 62/68→**68/68 PASS** |
| 217 | static | **SHIPPED** `a354d85`, CI green | **Wave 1 complete**: 7 phase dirs (phase.json + agent.md), router recipe table, `max_optional_insertions` 4→6, operator `.gitignore` fix rode along |
| 218 | static | **SHIPPED** `120c805` | wave-1 carryover closeout + eval docs |
| 219 | static | sealed at intent (operator) | boundary stop for the routing switch (user request) |
| 220 | **advisory** | crash at `test-amplification` dispatch | gap (a): persona not at `agents/evolve-<name>.md` |
| 221 | advisory | crash at `spec-verify` dispatch | gap (b): runner registration is boot-time; mid-run persona invisible |
| 222 | advisory | crash at `spec-verify` launch | gap (c): permission profile missing (phase-name vs role-name: `spec-verify` ≠ `spec-verifier.json`) |
| 223 | advisory | full 9-phase run; audit **FAIL** (correctly) | first end-to-end advisor-composed pipeline; auditor caught real H1/H2 integrity defects |
| 224 | advisory | adversarial-review ran; crash at `mutation-gate` dispatch | gap (a) recurrence on a phase created mid-incident; sealed per user stop request |

## 2. Defects found and their dispositions

### 2.1 Stale regression predicates (FIXED — committed `5cdb864`)
`cycle-49/006`, `cycle-89/003`, `cycle-89/004` asserted content in CLAUDE.md that the intentional docs split (`d8ac721`) moved to `docs/operations/runtime-reference.md`; `cycle-84/002` asserted `carryoverTodos == []`, contradicting the sanctioned queue workflow; `cycle-91/005` cascaded. Re-baselined intent-preserving (accept either canonical doc; schema-validity instead of emptiness). Reviewed (code-simplifier + code-reviewer; HIGH jq-precedence finding `(.carryoverTodos // []) | .[]` applied). Lesson: **manual/docs ships bypass the ACS suite, so doc restructures can silently break cycle ships one cycle later.**

### 2.2 `.gitignore` root-`.evolve` shadow (FIXED — shipped inside `a354d85`)
v8.21.0's `**/.evolve/` matched the ROOT `.evolve/` directory, dead-lettering every whitelist below it — new `phase.json`/`agent.md`/profile files could only be staged with `git add -f` (explains historical force-add workarounds). Ship uses plain `git add -A`, so cycle 217's deliverables would have been **silently dropped at ship**. Fix: `*/**/.evolve/` (nested-only) + `!.evolve/phases/*/agent.md`, applied in the cycle's worktree so it shipped with the cycle commit (no mid-cycle main commit → ff-merge preserved). Proof: `git add --dry-run` stages phase files, still ignores runtime state.

### 2.3 User-phase dispatch chain (BRIDGED — root fix queued `user-phase-persona-resolution`)
Advisor-selectable phases need **three** disk artifacts, resolved at three different times:
1. **Persona** `agents/evolve-<name>.md` — read lazily at dispatch (`load agent` crash, cycles 220/224). The documented ADR-0035 authoring path `.evolve/phases/<name>/agent.md` is **never read** — docs and runtime disagree.
2. **Runner** — registered at boot from persona presence (`no runner registered`, cycle 221). Mid-run persona additions are invisible.
3. **Permission profile** `.evolve/profiles/<phase-name>.json` — bridge launch `exit=10` (cycle 222). Built-ins ship ROLE-named profiles (`spec-verifier.json`); resolution is phase-NAME-based; user phases had none.

Operator bridges now on disk (committed this session): 11 personas (7 wave-1 + spec-verify + security-scan + dependency-audit + mutation-gate), 14 profiles (template: architecture-design.json, tier sonnet). **Root fix requirements** (queued, HIGH): phase-dir persona resolution with agents/ fallback; name→role→archetype-default profile fallback; and a **plan-time dispatchability clamp** — the advisor plan must be validated against (persona ∧ runner ∧ profile) like `ClampPlanToFloor`, never crashing mid-pipeline. Bridge copies become removable once the loader reads phase dirs.

### 2.4 Plan-vs-trigger reconciliation gap (OBSERVED — not yet queued)
Cycle 224's advisor selected `mutation-gate` `run:true` ("dogfood the shipped artifact"), but the phase's own `insert_when` (`amplify.tests_added > 0`) couldn't fire (test-amplification was advisor-skipped), and content-routing dropped it… until the dispatch attempt crashed on the persona. Open design question: does an explicit advisor `run:true` override `insert_when`, or AND with it? Currently ambiguous; should be defined in the root-fix ADR.

## 3. What the advisor did well (evidence for the catalog design)

- **Goal-conditioned composition** (cycle 224 plan evaluated all 19 catalog phases): selected adversarial-review *because the prior audit failed* (escalation), rejected fault-localization/reproduce-bug with "faults already localized / deterministic artifact issue, nothing to reproduce" — correct bugfix-recipe discrimination.
- **Justified skips**: architecture-design "design settled in catalog doc"; tester "no acs_red signal; evaluate budget covered".
- **Recipe adherence**: feature-shaped cycles got spec-verify → tdd → build → test-amplification → tester (the design-doc feature recipe).
- **Kernel clamp worked**: advisor twice voted scout `run:false`; the pinned spine ran it anyway. Model proposes, kernel disposes — zero incidents.
- **Phase value proven on first run**: cycle 223's `test-amplification` (132-line report) caught a real `gte`-vs-`gt` routing-condition bug the tdd contract missed — the AgentCoder anti-bias hypothesis (test designer that hasn't seen the implementation) validated in production.
- **spec-verify** produced a contract-conforming WARN verdict consumed by tdd on its first dispatch.

## 4. Trust-kernel observations

- Cycle 223's audit FAIL was **correct**: stale challenge token in build-report (H1, cribbed from sealed cycle-220 artifacts) + grep-only window-dressing predicate (H2) — both genuine integrity defects; ship blocked, retro recorded.
- All operator interventions used sanctioned paths: `evolve cycle reset` (×4, reasons in ledger), gated `/commit` (predicates), in-worktree config fix that shipped through the cycle's own audited commit.
- `[contract-gate] ship: no contract registered for phase "ship"` fail-open WARN — benign (ship has no deliverable), but noisy; candidate for an explicit empty contract.

## 5. State at session end

- **main**: `120c805` (+ bridge commit, see below); CI green throughout; suite 68/68 → 77/77 with cycle-223's predicates (in its sealed worktree only).
- **carryoverTodos**: `micro-phase-wave-2` (mutation-gate built twice, never shipped — remains), `micro-phase-wave-3`, `user-phase-persona-resolution` (HIGH; three modes documented).
- **Sealed runs**: cycle-216/219/220/221/222/224 reset archives under `.evolve/runs/` (incident archaeology).
- **Wave-1 catalog phases**: live on main, advisor-selectable, 3 of 7 + 2 legacy executed successfully at least once (spec-verify, test-amplification, adversarial-review).

### 5.1 Known inert gate (review finding)
`mutation-gate/phase.json` declares `fail_if_signal: {"mutation.score": "<60"}`, but `specrunner.go:122-127` stubs `fail_if_signal` evaluation ("requires Stage 3 signal bus") — the gate is a documented no-op until the signal bus lands. Same applies to every wave-1 `fail_if_signal` rule. Operators must not rely on these thresholds yet; classify `require_sections`/`verdict_on_pass` ARE enforced.

## 6. Recommendations (priority order)

1. Implement `user-phase-persona-resolution` (queued, HIGH) — includes the plan-time dispatchability clamp; advisory mode is not unattended-safe until then.
2. Define plan-vs-trigger semantics (§2.4) in the same ADR.
3. Re-ship mutation-gate through a clean cycle (wave-2 todo) — its two prior builds validated the spec but never landed.
4. Add `evolve loop --stop-after-cycle` (graceful batch boundary) — both operator interventions this session required kill+reset at a phase boundary; a sentinel check between cycles would make boundary switches loss-free.
5. Keep `EVOLVE_DYNAMIC_ROUTING=advisory` for supervised runs only until (1) lands; static mode remains the unattended default.
