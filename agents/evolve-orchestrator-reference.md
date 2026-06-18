# Orchestrator Reference (Layer 3 — on-demand)

This is the orchestrator's deep-reference file. Sections here are loaded only
when the orchestrator's primary flow encounters specific failure modes; in the
common PASS-cycle path, none of this content is needed. v8.64.0 Campaign D
Cycle D1 split.

The orchestrator's compact role-card (Layer 1, ~16 KB after split) lives at
`agents/evolve-orchestrator.md`. It includes a `## Reference Index` pointing
here. To use this file, the orchestrator invokes `Read` on the specific
section path when its decision branch demands it — never as a default.

---

## Section: operator-action-block-template

Loaded when `adaptiveFailureDecision.action` is `BLOCK-CODE` or
`BLOCK-OPERATOR-ACTION` (~10% of cycles in steady state).

When the failure adapter returns `BLOCK-CODE` or `BLOCK-OPERATOR-ACTION`, the
orchestrator-report.md MUST contain:

```markdown
## Operator Action Required

**Verdict**: <verdict_for_block from JSON>
**Reason**: <reason from JSON>

**Remediation**:
<remediation from JSON, verbatim>

**Forensic evidence**:
- non_expired_count: <evidence.non_expired_count>
- by_class: <evidence.by_class>
- consecutive_infra_transient_streak: <evidence.consecutive_infra_transient_streak>
```

This block lets the human operator know exactly what to do without reading
source code. **Do not paraphrase** — quote the JSON fields verbatim. The block
is what makes a BLOCK verdict actionable rather than mysterious.

---

## Section: failure-adapter-rationale

Loaded only when an operator/auditor questions why the orchestrator follows
the failure-adapter JSON without interpretation. Background reading; not
needed during normal operation.

The pre-v8.22 model gave the orchestrator a markdown table and asked it to
"decide." That was non-deterministic (interpretation could drift between
cycles) and conflated environmental issues (sandbox-eperm) with code-quality
issues (audit FAIL).

v8.22's adapter:
- Uses a typed classification taxonomy (7 distinct classes) with per-class
  age-out windows.
- Scores code and infrastructure failures separately (no "any-kind"
  conflation that obscures root cause).
- Returns the action JSON deterministically — same input → same output.
- Is unit-tested via `legacy/scripts/failure-adapter-test.sh`.

If you find yourself wanting to override the adapter's verdict, that's a sign
the decision rules need updating (file an issue) — NOT a sign to bypass the
kernel. Bypassing the adapter is exactly the failure mode the kernel exists
to prevent.

---

## Section: operating-principles

Loaded when the orchestrator wants to re-ground on the design intent
(e.g., feeling tempted to peek inside the diff or interpret a gate's
stderr in a creative way). Background reading; not needed during the
healthy phase loop.

1. **You are not the Builder.** Resist the urge to peek inside the diff
   and fix something yourself. If audit FAIL, record and exit; the next
   cycle handles it.
2. **Trust the gates.** Don't try to circumvent role-gate, ship-gate, or
   phase-gate-precondition. They exist because LLM judgment alone cannot
   enforce trust boundaries.
3. **Retrospect inline on FAIL/WARN (v8.45.0+).** Reverses the pre-v8.45
   "batched per v8.12.3" design. After `record-failure-to-state.sh`,
   advance to phase=retrospective and invoke `subagent-run.sh
   retrospective`. The retrospective subagent reads audit-report +
   build-report + scout-report + failure context, produces a structured
   lesson YAML at `.evolve/instincts/lessons/<id>.yaml`, then
   `merge-lesson-into-state.sh` updates `state.json:instinctSummary[]`
   so the next cycle benefits. Failure-learning routing is configured only
   through `.evolve/policy.json:failure_floor`.
   Cost: ~$0.30-0.50 per FAIL/WARN cycle (retrospective uses Sonnet by
   default — see `.evolve/profiles/retrospective.json`).
4. **Write the report once.** orchestrator-report.md is single-write. If
   you need to refine, do it in your editor before writing.
5. **Respect the budget.** If `budgetRemaining.budgetPressure` is `high`,
   prefer Haiku-tier reasoning; do not iterate excessively on borderline
   decisions.

---

## Section: failure-modes-recovery

Loaded when the orchestrator hits an unexpected stderr or non-zero exit from
a kernel script. In healthy cycles, none of these are encountered.

| Symptom | Recovery |
|---------|----------|
| subagent-run.sh exits non-zero | Read its stderr; usually a profile/CLI issue. Record failure and exit; the operator addresses tooling. |
| Auditor produces no audit-report.md | Treat as FAIL; record and exit. |
| ship.sh exits non-zero | Read stderr (often "audit verdict not PASS" or "tree state changed since audit"). Record. Exit. |
| role-gate denies an Edit | You shouldn't be editing — read the gate's stderr to understand what you mistakenly attempted. |
| phase-gate-precondition denies | Check cycle-state.json — you likely forgot to advance the phase before invoking the next agent. |

The system is designed so mistakes are loud and recoverable. Lean into the
constraints rather than fighting them.

---

## Section: registry-dispatch

Loaded when `EVOLVE_USE_PHASE_REGISTRY=1` (default). The phase sequence is driven by `docs/architecture/phase-registry.json`.

```bash
# Calibrate: read phase order from registry
phase_list=$(EVOLVE_USE_PHASE_REGISTRY=1 bash legacy/scripts/dispatch/list-phase-order.sh)

# Dispatch loop (registry-driven):
while IFS= read -r phase_name; do
    gate_in=$(jq -r --arg p "$phase_name" '.phases[] | select(.name==$p) | .gate_in // empty' docs/architecture/phase-registry.json 2>/dev/null || true)
    [ -n "$gate_in" ] && gate_run_by_name "$gate_in" "$CYCLE" "$WORKSPACE"
    cycle-state.sh advance "$phase_name" "$(jq -r --arg p "$phase_name" '.phases[] | select(.name==$p) | .role' docs/architecture/phase-registry.json)"
    subagent-run.sh "$(jq -r --arg p "$phase_name" '.phases[] | select(.name==$p) | .role' docs/architecture/phase-registry.json)" "$CYCLE" "$WORKSPACE"
    gate_out=$(jq -r --arg p "$phase_name" '.phases[] | select(.name==$p) | .gate_out // empty' docs/architecture/phase-registry.json 2>/dev/null || true)
    [ -n "$gate_out" ] && gate_run_by_name "$gate_out" "$CYCLE" "$WORKSPACE"
done <<EOF
$phase_list
EOF
```

---

## Section: resume-mode

Loaded when `EVOLVE_RESUME_MODE=1`. Protocol for picking up a paused cycle.

Three pause causes: `quota-likely`, `batch-cap-near`, `operator-requested` — the orchestrator emits the `CHECKPOINT-PAUSED` verdict for each.

### Resume protocol

1. **Read state**: `cycle-state.sh get cycle_id`, `cycle-state.sh get phase`, `cycle-state.sh resume-phase`.
2. **Skip completed**: For each phase in `EVOLVE_RESUME_COMPLETED_PHASES`, trust existing artifacts. Do not re-run completed phases — re-running burns tokens and may stomp valid output.
3. **Clear checkpoint**: `cycle-state.sh clear-checkpoint` once you resume forward progress.
4. **Pick up**: Invoke `EVOLVE_RESUME_PHASE` subagent normally.
5. **Re-pause**: If `quota-likely` or `batch-cap-near` or if `EVOLVE_CHECKPOINT_REQUEST=1` is set (intentional operator pause), write new checkpoint and emit `CHECKPOINT-PAUSED`.

---

## Section: failure-adaptation

Loaded for the Failure Adaptation Kernel (v8.22.0+).

| `action` field | What you do |
|---|---|
| `PROCEED` | Run standard phase sequence. |
| `RETRY-WITH-FALLBACK` | Run standard sequence with exported `set_env`. |
| `BLOCK-CODE` | Do NOT spawn Scout/Builder. Write report with `verdict_for_block`. |
| `BLOCK-OPERATOR-ACTION` | Infrastructure block. Write report with `BLOCKED-SYSTEMIC`. |

## Section: phase-observer

Loaded when phase-observer reporting is enabled by the orchestrator policy.

The observer is a sibling subagent that watches the active phase agent's stream-json output and writes two artifacts to the workspace before exiting:

- `{agent}-observer-events.ndjson` — live event stream (one observation envelope per line). Tailable for real-time inspection.
- `{agent}-observer-report.json` — phase-end summary with `summary.verdict` ∈ `{NORMAL, DEGRADED, INCIDENT}` and an `incidents[]` array.

**After each subagent returns, before reading `{agent}-report.md`, do this:**
1. If `{agent}-observer-report.json` exists, `Read` it.
2. If `summary.verdict == "INCIDENT"` OR `incidents[]` non-empty, the observer detected an abnormal condition (stuck, infinite loop, error spike, throttled, cost anomaly). Treat the first incident's `suggested_action.machine_readable` as a decision input alongside the subagent's own `{agent}-report.md`.
3. If `summary.verdict == "DEGRADED"`, mention the WARN observations in your final Notes section but continue normally.
4. If `summary.verdict == "NORMAL"` or the file is absent, proceed normally.

The observer is purely advisory; it never SIGTERMs the subagent (phase-watchdog still does that). Severity semantics: see `docs/architecture/observer-severity.md`.

---

## Section: egps-tester-phase

Loaded only when `EVOLVE_TEST_PHASE_ENABLED=1` (default 0).

After Builder completes (build-report.md + production code in worktree), spawn the Tester subagent before advancing to Audit:

```bash
cycle-state.sh advance test tester
subagent-run.sh tester "$CYCLE" "$WORKSPACE"
```

The Tester reads `build-report.md` and writes `acs/cycle-N/{NNN}-{slug}.sh` predicate scripts for each acceptance criterion, then produces `tester-report.md`. After Tester returns, advance to Audit normally.

Phase sequence (v10.3+ when enabled): `Scout → Triage → Builder → Tester → Auditor → Ship → (Retro)`

The Tester adds ~3-5 minutes wall time per cycle but breaks the AC-by-grep gaming pattern structurally (Builder cannot self-validate; Tester writes the predicates Builder's claims are checked against).

**Gate rationale:** `tester.json` profile and `agents/evolve-tester.md` are present but the phase is not yet default-on; forcing it previously caused 241s watchdog kills when subagent-run.sh's allowlist was missing `tester`.

```bash
# Orchestrator pattern (only when EVOLVE_TEST_PHASE_ENABLED=1):
if [ "${EVOLVE_TEST_PHASE_ENABLED:-0}" = "1" ]; then
    cycle-state.sh advance test tester
    subagent-run.sh tester "$CYCLE" "$WORKSPACE"
fi
# Otherwise: Builder writes its own acs/cycle-N/*.sh predicates (v10.1 fallback)
```

If Tester is unavailable (legacy profile, fallback mode), Builder writes its own predicates per v10.1 (backward-compat path).

---

## Section: legacy-phase-loop

*Moved from agents/evolve-orchestrator.md Phase Loop body — C75 token-opt patch (ADR-0013). The `## Phase Loop` heading and legend line remain in orchestrator.md to satisfy regression-suite/cycle-42 AC3.*

Execute phases strictly in this order. After each agent finishes, the runner does not auto-advance cycle-state — **you** advance it via `cycle-state.sh advance <new_phase> <agent>` before invoking the next agent.

```
1. Calibrate (read state, decide strategy)
   ↓ if cycle-state.intent_required==true: advance intent intent
1b. Intent (only when intent_required) → subagent-run.sh intent $CYCLE $WORKSPACE
   ↓ advance research scout
2. Research / Discover  →  subagent-run.sh scout $CYCLE $WORKSPACE
   ↓ unless EVOLVE_TRIAGE_DISABLE=1: advance triage triage
2b. Triage (default-on; opt-out via EVOLVE_TRIAGE_DISABLE=1)
       Before invoking the Triage subagent, check the 3 auto-skip conditions
       (Opt A, v10.19.0; default-on, opt-out via EVOLVE_TRIAGE_AUTO_SKIP_TRIVIAL=0):
         (i)   scout-report.md task count ≤ 1
         (ii)  state.json:carryoverTodos | length == 0
         (iii) predicate-dependency-check.sh returns exit 0 (no cross-cycle
               predicate-graph reachability via cycle-91 lesson)
       If ALL three hold AND EVOLVE_TRIAGE_AUTO_SKIP_TRIVIAL != 0, write a
       deterministic stub triage-report.md to $WORKSPACE with:
         - challenge_token (generate via openssl rand or `date +%s` fallback)
         - `cycle_size_estimate: trivial`
         - `auto_skip: true`, `reason: trivial-cycle-no-cross-deps`
         - top_n derived from scout's tasks verbatim; deferred=[]; dropped=[]
       and a JSON sidecar with the same fields. Skip the LLM call, advance
       phase state (`cycle-state.sh advance plan-review orchestrator` or the
       next-eligible phase per registry), and log `[triage-auto-skipped]`.
       Else (any condition fails): → subagent-run.sh triage $CYCLE $WORKSPACE
       Reads scout-report + state.json:carryoverTodos[]; emits triage-report.md
       with top_n[]/deferred[]/dropped[]/cycle_size_estimate. phase-gate
       (`triage-to-plan-review`) blocks on cycle_size_estimate=large (split required).
       phase-gate (`discover-to-build`) emits a soft WARN if Triage was skipped
       without explicit EVOLVE_TRIAGE_DISABLE=1 (first-rollout: WARN, not FAIL).
   ↓ if EVOLVE_PLAN_REVIEW=1: advance plan-review plan-reviewer (Sprint 2)
2c. Plan-review (opt-in) → see Sprint 2 docs
   ↓ advance build builder
   (if size == trivial: skip TDD)
2d. Build-planner (opt-in) →
                          if [ "${EVOLVE_BUILD_PLANNER:-1}" = "1" ]; then
                              advance build-planner build-planner
                              subagent-run.sh build-planner $CYCLE $WORKSPACE
                              phase-gate.sh build-planner-to-build $CYCLE $WORKSPACE
                          fi
   (EVOLVE_BUILD_PLANNER=1 by default since v10.20 — advisory mode. Produces build-plan.md; Builder reads it as a sanity check.
    Revert: EVOLVE_BUILD_PLANNER=0. Enforce mode in cycle-105. See ADR-0019 for 3-cycle rollout: shadow→advisory→enforce.)
3. Build                →  subagent-run.sh builder $CYCLE $WORKSPACE
   ↓ advance audit auditor
   (if size == trivial: skip Audit → jump to ship)
4. Audit                →  subagent-run.sh auditor $CYCLE $WORKSPACE
   (Fan-out: use `evolve subagent dispatch-parallel auditor $CYCLE $WORKSPACE` explicitly
    when profile.parallel_eligible=true. Configure via .evolve/policy.json "fanout" block.)
   ↓ verdict-driven branch:
5a. PASS         →  advance ship orchestrator
                    if [ "$size" = "trivial" ]; then
                        ship.sh --class trivial "<msg>"
                    else
                        ship.sh "<msg>"
                    fi
                    advance learn memo
                    subagent-run.sh memo $CYCLE $WORKSPACE  (PASS-cycle memo emits carryover-todos.json + memo.md cycle memo — see Layer-P Memo Phase Contract)
                    merge-lesson-into-state.sh $WORKSPACE; MERGE_RC=$?
                    if [ $MERGE_RC -eq 2 ]; then record-failure-to-state.sh "$WORKSPACE" lesson-merge-integrity-fail; exit 2; fi  # INTEGRITY_FAIL: lesson YAML missing
                    [ $MERGE_RC -ne 0 ] && log "WARN: merge-lesson-into-state exit $MERGE_RC"
                    reconcile-carryover-todos.sh --cycle $CYCLE --workspace $WORKSPACE --verdict PASS  (Layer D)
5b. WARN →  record-failure-to-state.sh $WORKSPACE WARN  (low-severity awareness)
                       advance ship orchestrator  →  ship.sh "<commit-msg>"
                       (ship.sh accepts WARN per v8.28.0 fluent-by-default policy)
                       advance retrospective retrospective
                       subagent-run.sh retrospective $CYCLE $WORKSPACE
                       merge-lesson-into-state.sh $WORKSPACE; MERGE_RC=$?
                       if [ $MERGE_RC -eq 2 ]; then record-failure-to-state.sh "$WORKSPACE" lesson-merge-integrity-fail; exit 2; fi  # INTEGRITY_FAIL: lesson YAML missing
                       [ $MERGE_RC -ne 0 ] && log "WARN: merge-lesson-into-state exit $MERGE_RC"
                       gate_retrospective_to_complete  (gate — verifies lesson YAML landed in instincts/)
                       reconcile-carryover-todos.sh --cycle $CYCLE --workspace $WORKSPACE --verdict WARN
5c. FAIL         →  record-failure-to-state.sh $WORKSPACE FAIL  (no ship)
                       advance retrospective retrospective
                       subagent-run.sh retrospective $CYCLE $WORKSPACE
                       merge-lesson-into-state.sh $WORKSPACE; MERGE_RC=$?
                       if [ $MERGE_RC -eq 2 ]; then record-failure-to-state.sh "$WORKSPACE" lesson-merge-integrity-fail; exit 2; fi  # INTEGRITY_FAIL: lesson YAML missing
                       [ $MERGE_RC -ne 0 ] && log "WARN: merge-lesson-into-state exit $MERGE_RC"
                       gate_retrospective_to_complete  (gate — verifies lesson YAML landed in instincts/)
                       reconcile-carryover-todos.sh --cycle $CYCLE --workspace $WORKSPACE --verdict FAIL
6. Write orchestrator-report.md → exit
```
