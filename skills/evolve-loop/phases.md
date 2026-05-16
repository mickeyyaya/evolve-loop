> Read this file when orchestrating the full evolve-loop cycle. Covers phase gate verification, cycle setup, and all phase transitions (DISCOVER through LEARN).

## Contents
- [Mandatory Phase Gate Verification](#mandatory-phase-gate-verification) — deterministic trust boundary at every transition
- [Phase 0: CALIBRATE](#phase-0-calibrate-once-per-invocation) — project benchmark baseline
- [Cycle Setup](#for-each-cycle-while-remainingcycles--0) — context budget, lean mode, integrity
- [Phase 1: RESEARCH](#phase-1-research-every-cycle) — see phase1-research.md
- [Phase 2: DISCOVER](#phase-2-discover) — Scout launch, task claiming, stagnation detection
- [Context Quality & Handoffs](#context-quality-checklist-cemm) — CEMM checklist, per-phase matrix, handoff format
- [Phase 3: BUILD](#phase-3-build-loop-per-task) — Builder dispatch (see phase3-build.md)
- [Phase 4: AUDIT](#phase-4-audit-parallel) — Auditor launch, eval checksum verification
- [Benchmark Delta & Pre-Ship Gate](#benchmark-delta-check--pre-ship-integrity-gate) — regression detection, health check
- [Phase 5: SHIP](#phase-5-ship) — see phase5-ship.md
- [Phase 6: LEARN](#phase-6-learn) — see phase6-learn.md

# Evolve Loop — Phase Instructions

**Important:** Every agent context block must include `goal` (string or null) and `strategy` (one of: `balanced`, `innovate`, `harden`, `repair`, `ultrathink`, `autoresearch`).

## Mandatory Phase Gate Verification

Run `scripts/lifecycle/phase-gate.sh` at every phase transition. This deterministic bash script verifies artifacts exist, agents ran, and integrity checks pass. The orchestrator cannot skip it — it is the trust boundary between LLM judgment and structural enforcement.

```bash
bash scripts/lifecycle/phase-gate.sh <gate> <cycle> <workspace_path>
# Gates: discover-to-build, build-to-audit, audit-to-ship, ship-to-learn, cycle-complete
# Exit 0 = proceed, Exit 1 = block, Exit 2 = halt (present to human)
```

| Enforcement | Detail |
|-------------|--------|
| Workspace artifacts | scout/build/audit reports must exist and be fresh (<10 min old) |
| Eval re-execution | `verify-eval.sh` runs independently (not trusting Auditor claims) |
| Cycle health | `cycle-health-check.sh` runs 11 integrity signals |
| State updates | `state.json` cycle number and mastery updated ONLY by the script |
| Tamper detection | Eval checksums verified |

v8.13.1 kernel hooks: **role-gate.sh** (denies writes outside phase allowlist), **phase-gate-precondition.sh** (denies out-of-order subagent calls), **ship-gate.sh** (denies direct git commit/push). For lifecycle integration code and bypass flags see [reference/phases-detail.md](reference/phases-detail.md#kernel-hooks).

---

## Phase 0: CALIBRATE (once per invocation)

Runs **once per `/evolve-loop` invocation**, not per cycle. Establishes a project-level benchmark baseline.

### Session Resume Check (before calibration)

```bash
HANDOFF_FILE=".evolve/workspace/handoff.md"
if [ -f "$HANDOFF_FILE" ] && grep -q "Session Break Handoff" "$HANDOFF_FILE"; then
  # Parse: remaining cycles, strategy, goal, carry-forward context
  echo "Resuming from session break. Reading handoff..."
fi
```

Read the handoff to recover: remaining cycles, goal, carry-forward notes, task queue snapshot, recent verdicts.

### Calibration

For detailed calibration instructions, see [phase0-calibrate.md](phase0-calibrate.md).

| Step | Action |
|------|--------|
| Skip check | Skip if `projectBenchmark.lastCalibrated` < 24 hours ago |
| Automated scoring | Run checks from `benchmark-eval.md` for all 8 dimensions |
| Composite | `composite = automated` (automated scores only) |
| Store | Write to `state.json.projectBenchmark`, pass `benchmarkWeaknesses` to Scout |
| Tamper detection | Checksum `benchmark-eval.md` for verification in Phase 4-5 |

### Skill Inventory (once per session)

Run `bash scripts/utility/setup-skill-inventory.sh` (1h cache) to build `.evolve/skill-inventory.json`. Pass `categoryIndex` subset matching project language/framework/task-types to Scout as `skillCategories`. For routing categories table, JSON schema, scope precedence, and fallback behavior see [reference/phases-detail.md](reference/phases-detail.md#skill-inventory).

---

## FOR each cycle (while remainingCycles > 0):

### Rate Limit Recovery (wraps every agent dispatch)

```
if agent_result contains "rate limit|quota|overloaded|429|too many requests":
  → execute Rate Limit Recovery Protocol
if CONSECUTIVE_FAILURES >= 3:
  → execute Rate Limit Recovery Protocol
```

On detection: complete current phase, write handoff, auto-schedule resume (`/schedule` → `/loop 5m` → manual), then **STOP**. For full detection code block and 4-step protocol see [reference/phases-detail.md](reference/phases-detail.md#rate-limit-recovery).

### Context Window Budget Gate (MANDATORY — runs before anything else)

Run `scripts/verification/context-budget.sh` before each cycle (bash block in [phases-detail.md](reference/phases-detail.md#context-budget-gate)).

| Exit | Status | Action |
|------|--------|--------|
| 0 | **GREEN** | Continue — full budget |
| 1 | **YELLOW** | Force lean mode ON. **YELLOW is NOT a stop signal.** |
| 2 | **RED** | Write handoff checkpoint. Only STOP if RED on **two consecutive** starts. |

### Phase 1: RESEARCH (every cycle)

Proactive research loop — full 6-step protocol in [phase1-research.md](phase1-research.md). **Token budget:** 25K max. Phase gate: `bash scripts/lifecycle/phase-gate.sh research-to-discover $CYCLE $WORKSPACE_PATH`

### Lean Mode (cycle 10+ OR YELLOW from budget gate)

| Optimization | Detail |
|-------------|--------|
| State.json | Read ONCE at cycle start; use in-memory copy thereafter |
| Agent results | Read summary only; read full file only if summary indicates a problem |
| Scout report | Extract task list from agent return value, not by reading scout-report.md |
| Eval checksums | Compute once, verify from memory |
| Benchmark delta | Skip for cycles where all tasks are S-complexity docs-only |

### AgentDiet Trajectory Compression

Between phase transitions, compress accumulated context per AgentDiet [arXiv:2509.23586].

| After Phase | Keep | Remove |
|------------|------|--------|
| DISCOVER | Task list and eval definitions | Full Scout analysis |
| BUILD | build-report.md summary and worktree branch/SHA | Full Builder output |
| AUDIT | Verdict (PASS/WARN/FAIL) and issue list | Full Auditor analysis |
| SHIP | Commit SHA and benchmark delta | Commit details |

**Never prune:** challenge token, eval checksums, failed approach details, stagnation patterns.

### Cycle Integrity Setup

1. **Challenge token:** `openssl rand -hex 8`; embed in every agent context block and ledger entry
2. **Canary files:** plant `.integrity-canary` + `.integrity-canary-expected` in workspace; any access = anomaly
3. **Adaptive strictness decay:** halve all `consecutiveClean` values in `auditorProfile` at each new invocation
4. **Random full-audit flag:** `RANDOM_CHECK=$((RANDOM % 5))`; trigger `FORCE_FULL_AUDIT=true` when 0
5. **Hash chain:** compute `PREV_HASH` from last ledger entry; every new entry must include `"prevHash":"<$PREV_HASH>"`

For full code blocks see [reference/phases-detail.md](reference/phases-detail.md#cycle-integrity-setup).

### Atomic Cycle Number Allocation

1. Read `state.json`, note `version = V` and `lastCycleNumber`
2. `claimedCycle = lastCycleNumber + 1`
3. Write `state.json` with `lastCycleNumber = claimedCycle`, `version = V + 1`
4. Re-read `state.json` and verify `version == V + 1`
5. If version mismatch → re-read, re-claim next available number
6. Use `claimedCycle` as this iteration's cycle number `N`
7. Decrement `remainingCycles`

---

### Phase 2: DISCOVER

For the full protocol — convergence short-circuit, context pre-compute, prompt caching, Scout launch, task claiming, eval quality check, stagnation handling — see [phase2-discover.md](phase2-discover.md).

**Convergence short-circuit:** If `nothingToDoCount >= 2` and `discoveryVelocity.rolling3 == 0`, skip Scout and jump to Phase 5.

**Skill routing context:** Pass `skillCategories` to Scout. See [reference/skill-routing.md](reference/skill-routing.md) for precedence and conflict resolution.

### Context Quality Checklist (CEMM)

Based on the Context Engineering Maturity Model (arXiv:2603.09619), apply five criteria to every context block before passing to Scout:

| Criterion | Guiding Question |
|-----------|-----------------|
| **Relevance** | Is this context needed by the current phase? |
| **Sufficiency** | Does the context have enough info for the agent to complete its task? |
| **Isolation** | Is this phase's context independent from other phases' internal state? |
| **Economy** | Are we sending the minimum tokens needed? |
| **Provenance** | Can we trace where each piece of context came from? |


### Per-Phase Context Selection Matrix

Each agent receives ONLY the fields it needs (Anthropic's **Select** strategy — saves ~3-5K tokens/invocation).

| Field | Scout | Builder | Auditor | Operator |
|-------|:-----:|:-------:|:-------:|:--------:|
| `cycle` | Y | Y | Y | Y |
| `strategy` | Y | Y | Y | — |
| `challengeToken` | Y | Y | Y | — |
| `budgetRemaining` | Y | Y | — | Y |
| `instinctSummary` | Y | Y | — | — |
| `stateJson` (full) | Y | — | — | Y |
| `projectContext` | Y | — | — | — |
| `projectDigest` | Y | — | — | — |
| `changedFiles` | Y | — | — | — |
| `recentNotes` | Y | — | — | Y |
| `benchmarkWeaknesses` | Y | — | — | — |
| `task` (from scout-report) | — | Y | — | — |
| `buildReport` | — | — | Y | — |
| `auditorProfile` | — | — | Y | — |
| `recentLedger` | Y | — | Y | Y |

### Inter-Phase Handoff Format

Each phase writes `$WORKSPACE_PATH/handoff-<phase>.json` for the next phase. For full JSON schema see [reference/phases-detail.md](reference/phases-detail.md#inter-phase-handoff).

| File | Written by | Read by | Key fields |
|------|-----------|---------|------------|
| `handoff-scout.json` | Scout | Builder | `findings` (rationale), `files_modified`, `next_phase_context.taskSlug` |
| `handoff-builder.json` | Builder | Auditor | `findings` (approach), `files_modified`, `decisions`, `next_phase_context.worktreeBranch` |
| `handoff-auditor.json` | Auditor | Orchestrator (Ship) | `findings` (verdict), `decisions` (issues), `next_phase_context.verdict` |

If handoff file absent or malformed, fall back to full report (e.g., `scout-report.md`, `build-report.md`).

---

### Phase Boundary: DISCOVER -> BUILD

```bash
bash scripts/lifecycle/phase-gate.sh discover-to-build $CYCLE $WORKSPACE_PATH
# Exit 0 -> proceed to BUILD. Any other exit -> HALT.
```

### Phase 3: BUILD (loop per task)

For detailed Phase 3 implementation, see [phase3-build.md](phase3-build.md).

| Step | Detail |
|------|--------|
| Inline S-tasks | Execute first (per inst-007), commit sequentially |
| Independent worktree tasks | Launch IN PARALLEL — one Agent call per task with `isolation: "worktree"`. See phase3-build.md Dependency Partitioning Algorithm. |
| Dependent tasks | Build sequentially within groups per phase3-build.md conflict graph |
| Isolation | Builder MUST use worktree isolation for coding projects |
| Retries | Max 3 attempts per task; failures logged to state.json |

---

### Phase Boundary: BUILD -> AUDIT

```bash
bash scripts/lifecycle/phase-gate.sh build-to-audit $CYCLE $WORKSPACE_PATH
# Exit 0 -> proceed to AUDIT. Any other exit -> HALT.
```

### Phase 4: AUDIT (Parallel)

The Auditor reviews Builder changes **in the worktree**. Worktree tasks built in parallel during Phase 3 MUST also be audited in parallel.

**Inline task audit:** Run eval graders before committing. If any grader fails, revert and retry inline or escalate to Builder agent. Do NOT skip audit for inline tasks.

**Eval checksum verification:** `sha256sum -c $WORKSPACE_PATH/eval-checksums.json` — if any checksum fails → HALT: "Eval tamper detected."

**Launch Auditor Agent** via `scripts/dispatch/subagent-run.sh auditor "$CYCLE" "$WORKSPACE_PATH"`. For full context JSON and model-tier selection logic see [reference/phases-detail.md](reference/phases-detail.md#phase-4-audit).

**Enhanced evaluation via `/evaluator`** (optional, `strategy == "harden"` or `forceFullAudit == true`): merge scores into `## Evaluator Scores`. Skip when lean mode or budget YELLOW/RED.

After Auditor completes: PASS → Phase 5 Ship; WARN/FAIL/SHIP_GATE_DENIED → drop work, record failure, discard worktree.

---

### Benchmark Delta Check & Pre-Ship Integrity Gate

| Check | Detail |
|-------|--------|
| Exemptions | Skip for `repair` strategy, first 3 cycles, or `meta`/`infrastructure` tasks |
| Regression | Any dimension -3 or more blocks shipping; 1 retry, then drop |
| Pre-ship gate | `verify-eval.sh` + `cycle-health-check.sh` run independently of all agents |
| Proceed | Only if both health check and eval verification pass |

```bash
bash scripts/lifecycle/phase-gate.sh audit-to-ship $CYCLE $WORKSPACE_PATH
# Exit 0 -> proceed to SHIP. Exit 1 -> block. Exit 2 -> HALT for human.
```

### Phase 5: SHIP

For detailed instructions, see [phase5-ship.md](phase5-ship.md).

```bash
bash scripts/lifecycle/phase-gate.sh ship-to-learn $CYCLE $WORKSPACE_PATH
```
Verifies git is clean and updates `state.json.lastCycleNumber` (the SCRIPT does this, not the LLM).

---

### Phase 6: LEARN

For detailed instructions, see [phase6-learn.md](phase6-learn.md).

```bash
bash scripts/lifecycle/phase-gate.sh cycle-complete $CYCLE $WORKSPACE_PATH
```
Archives workspace, verifies all 3 reports exist, updates mastery ONLY if audit-report.md contains a genuine PASS verdict.

---

> For detailed phase-gate code blocks, Skill Inventory schema, Cycle Integrity Setup code, Rate Limit Recovery protocol, and Inter-Phase Handoff JSON schema, see [reference/phases-detail.md](reference/phases-detail.md).
