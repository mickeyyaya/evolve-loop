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
- Is unit-tested via `scripts/failure-adapter-test.sh`.

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
   so the next cycle benefits. Operator opt-out:
   `EVOLVE_DISABLE_AUTO_RETROSPECTIVE=1` reverts to pre-v8.45 record-only.
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
phase_list=$(EVOLVE_USE_PHASE_REGISTRY=1 bash scripts/dispatch/list-phase-order.sh)

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

1. **Read state**: `cycle-state.sh get cycle_id`, `cycle-state.sh get phase`, `cycle-state.sh resume-phase`.
2. **Skip completed**: For each phase in `EVOLVE_RESUME_COMPLETED_PHASES`, trust existing artifacts.
3. **Clear checkpoint**: `cycle-state.sh clear-checkpoint`.
4. **Pick up**: Invoke `EVOLVE_RESUME_PHASE` subagent normally.
5. **Re-pause**: If `quota-likely` or `batch-cap-near`, write new checkpoint.

---

## Section: failure-adaptation

Loaded for the Failure Adaptation Kernel (v8.22.0+).

| `action` field | What you do |
|---|---|
| `PROCEED` | Run standard phase sequence. |
| `RETRY-WITH-FALLBACK` | Run standard sequence with exported `set_env`. |
| `BLOCK-CODE` | Do NOT spawn Scout/Builder. Write report with `verdict_for_block`. |
| `BLOCK-OPERATOR-ACTION` | Infrastructure block. Write report with `BLOCKED-SYSTEMIC`. |
