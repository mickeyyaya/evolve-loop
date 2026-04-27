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

Run `scripts/phase-gate.sh` at every phase transition. This deterministic bash script verifies artifacts exist, agents ran, and integrity checks pass. The orchestrator cannot skip it — it is the trust boundary between LLM judgment and structural enforcement.

```bash
bash scripts/phase-gate.sh <gate> <cycle> <workspace_path>
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

The LLM retains all creative work — task selection, implementation, code review, instinct extraction. The script enforces *process*, the LLM does *substance*.

## Phase 0: CALIBRATE (once per invocation)

Runs **once per `/evolve-loop` invocation**, not per cycle. Establishes a project-level benchmark baseline.

### Session Resume Check (before calibration)

On every invocation, check for a prior session-break handoff:

```bash
HANDOFF_FILE=".evolve/workspace/handoff.md"
if [ -f "$HANDOFF_FILE" ] && grep -q "Session Break Handoff" "$HANDOFF_FILE"; then
  echo "Resuming from session break. Reading handoff..."
  # Parse: remaining cycles, strategy, goal, carry-forward context
  # The handoff contains orchestrator reasoning that only existed in session memory
  # Read the "Carry Forward" section — this is context not derivable from state.json
fi
```

Read the handoff to recover:
- **Remaining cycles** (only source — not in state.json)
- **Goal** (if user-provided, only in handoff)
- **Carry Forward notes** (orchestrator observations from prior session)
- **Task queue snapshot** (selected-but-incomplete tasks)
- **Recent verdicts** (quick context without reading full ledger)

After reading, the handoff has been consumed. The new session writes its own handoff at each cycle checkpoint.

### Calibration

For detailed calibration instructions, see [phase0-calibrate.md](skills/evolve-loop/phase0-calibrate.md).

| Step | Action |
|------|--------|
| Skip check | Skip if `projectBenchmark.lastCalibrated` < 24 hours ago |
| Automated scoring | Run checks from `benchmark-eval.md` for all 8 dimensions |
| Composite | `composite = automated` (automated scores only) |
| Store | Write to `state.json.projectBenchmark`, pass `benchmarkWeaknesses` to Scout |
| Tamper detection | Checksum `benchmark-eval.md` for verification in Phase 4-5 |

### Skill Inventory (once per session)

Build a compact, deterministic inventory of every installed skill — project-local, user-global (`~/.claude/skills/`), and plugin cache (`~/.claude/plugins/cache/*/skills/`). This is done by a filesystem-scan script, NOT by LLM parsing of the system-reminder list, because the scripted scan is cheaper (zero tokens), more complete (every installed skill regardless of context window), and cached across sessions.

| Step | Action |
|------|--------|
| Run script | `bash scripts/setup-skill-inventory.sh` — idempotent, freshness-gated (1h cache) |
| Skip check | Script exits early with cache-hit message if `.evolve/skill-inventory.json` is <1 hour old (pass `--force` to rebuild) |
| Read output | Orchestrator reads `.evolve/skill-inventory.json` — already categorized, already sorted |
| Pass to Scout | Extract subset of `categoryIndex` matching `projectContext.language`/`framework`/task types; pass as `skillCategories` in the Scout context block |

**Output file:** `.evolve/skill-inventory.json` (schema matches the `state.json.skillInventory` shape below, plus a `skills` map keyed by name with `path`, `origin`, `referenceFiles`, and `categories` fields).

**Scopes scanned** (precedence: project > user > plugin/extension — first-seen wins on name collision):

| Scope | Path | Typical count |
|---|---|---|
| `project` | `./skills/**/SKILL.md` | 5-20 |
| `user` | `~/.claude/skills/*/SKILL.md` or `~/.gemini/skills/*/SKILL.md` | 50-300 |
| `plugin/extension` | `~/.claude/plugins/.../SKILL.md` or `~/.gemini/extensions/*/skills/*/SKILL.md` | 10-100 per plugin |

IDE-specific mirror dirs (`.cursor/skills/`, `.kiro/skills/`, `.agents/skills/`) are **skipped** — only the canonical `skills/` directory of each plugin version is consumed.

**Routing Categories:**

| Category | Matches | Example Skills |
|----------|---------|---------------|
| `code-review` | code review, quality, patterns | `/code-review-simplify` (built-in), `code-review:code-review`, `pr-review-workflow` |
| `testing` | TDD, test generation, coverage | `everything-claude-code:tdd`, `testing-patterns` |
| `security` | security audit, vulnerability | `everything-claude-code:security-review`, `security-patterns-code-review` |
| `language:<lang>` | language-specific patterns | `python-review-patterns`, `go-review-patterns`, `typescript-review-patterns` |
| `framework:<fw>` | framework-specific | `everything-claude-code:django-patterns`, `everything-claude-code:springboot-patterns` |
| `architecture` | design patterns, DDD | `architectural-patterns`, `domain-driven-design-patterns` |
| `debugging` | debugging, investigation | `superpowers:systematic-debugging`, `gstack-investigate` |
| `performance` | profiling, caching, optimization | `performance-anti-patterns`, `caching-strategies` |
| `frontend` | UI, components, design | `frontend-design:frontend-design`, `everything-claude-code:frontend-patterns` |
| `database` | SQL, ORM, migrations | `database-review-patterns`, `everything-claude-code:postgres-patterns` |
| `agent-design` | agent patterns, orchestration | `agent-orchestration-patterns`, `agent-memory-patterns` |
| `docs` | documentation, API docs | `code-documentation-patterns`, `review-api-contract` |
| `infra` | CI/CD, containers, deployment | `cicd-pipeline-patterns`, `container-kubernetes-patterns` |
| `refactoring` | refactor, code smells | `/refactor` (built-in), `detect-code-smells`, `refactoring-decision-matrix` |

For skill precedence, conflict resolution, phase eligibility, and budget-aware depth routing, see [reference/skill-routing.md](reference/skill-routing.md).

**Inventory schema (`.evolve/skill-inventory.json`, written by `scripts/setup-skill-inventory.sh`):**

```json
{
  "lastBuilt": "<ISO-8601>",
  "totalSkills": 281,
  "scopes": {
    "project": 5,
    "user": 226,
    "plugin:ecc:ecc": 32,
    "plugin:claude-plugins-official:superpowers": 14
  },
  "categoryIndex": {
    "code-review": ["code-reviewer", "pr-review-workflow", ...],
    "security": ["security-review", "security-patterns-code-review", ...],
    "language:python": ["python-patterns", "python-review-patterns", "python-testing"],
    "e2e": ["e2e-testing", "ui-demo", ...]
  },
  "skills": {
    "e2e-testing": {
      "name": "e2e-testing",
      "description": "Playwright E2E testing patterns, Page Object...",
      "origin": "user",
      "path": "/Users/.../e2e-testing/SKILL.md",
      "referenceFiles": ["references/playwright-config.md", ...],
      "categories": ["testing", "e2e"]
    }
  }
}
```

The full file runs ~50-200KB depending on installed plugins. The orchestrator does NOT pass it whole to agents — it reads the file, extracts the `categoryIndex` subset matching `projectContext.language`/`framework`/task-type signals, and passes only that slice to Scout as `skillCategories`. Individual `skills[<name>]` entries (including `path` and `referenceFiles`) are looked up on demand when Builder wants to open a specific skill's reference material.

**Fallback behavior:** If `.evolve/skill-inventory.json` is missing (e.g., first-ever cycle, setup script not yet run), the orchestrator MUST invoke `bash scripts/setup-skill-inventory.sh` before launching Scout. If the script fails (e.g., `python3` unavailable), fall back to the legacy LLM-parsing path and log a WARN to the ledger so the next session can retry.

---

## FOR each cycle (while remainingCycles > 0):

### Rate Limit Recovery (wraps every agent dispatch)

Track consecutive agent failures across all phases. After every agent dispatch (Scout, Builder, Auditor), check the return value for rate limit signals. See [policies.md § Rate Limit Recovery Protocol](reference/policies.md#rate-limit-recovery-protocol) for full detection logic.

```
CONSECUTIVE_FAILURES=0

# After each agent dispatch:
check_rate_limit(agent_result):
  if agent_result contains "rate limit|quota|overloaded|429|too many requests":
    → execute Rate Limit Recovery Protocol
  if agent_failed:
    CONSECUTIVE_FAILURES += 1
    if CONSECUTIVE_FAILURES >= 3:
      → execute Rate Limit Recovery Protocol
  else:
    CONSECUTIVE_FAILURES = 0
```

**On rate limit detection:**
1. Complete current phase (never break mid-phase)
2. Write handoff using Session Break Handoff Template (cause: "API rate limit")
3. Auto-schedule resume: try `/schedule` first (remote trigger at next hour), fall back to `/loop 5m`, fall back to manual resume instructions
4. **STOP** — do not start next phase

### Context Window Budget Gate (MANDATORY — runs before anything else)

Run `scripts/context-budget.sh` before each cycle. This is a **per-cycle** gate — it checks "is there room for ONE more cycle?" not cumulative session usage. Each cycle is an independent plan-mode unit; auto-compaction reclaims context from older cycles between runs.

```bash
CYCLES_THIS_SESSION=${CYCLES_THIS_SESSION:-0}
BUDGET_JSON=$(bash scripts/context-budget.sh "$CYCLE_NUMBER" "$CYCLES_THIS_SESSION" "$WORKSPACE_PATH" 2>/dev/null)
BUDGET_EXIT=$?
BUDGET_STATUS=$(echo "$BUDGET_JSON" | grep -o '"status": *"[^"]*"' | cut -d'"' -f4)
REMAINING_ESTIMATE=$(echo "$BUDGET_JSON" | grep -o '"remainingCyclesEstimate": *[0-9]*' | grep -o '[0-9]*$')
```

| Exit Code | Status | Trigger | Action |
|-----------|--------|---------|--------|
| 0 | **GREEN** | Cycles 1-9 | Continue — full per-cycle budget available |
| 1 | **YELLOW** | Cycle 10+ or headroom tight | Force lean mode ON for this cycle. **Continue — YELLOW is NOT a stop signal.** |
| 2 | **RED** | Cycle 30+ or headroom < one lean cycle | Write handoff checkpoint. Continue if `remainingCycles > 0`. Only STOP if RED triggers on **two consecutive cycle starts** — this confirms context is genuinely exhausted. |

**On RED (first occurrence):** Write enriched `handoff.md` as a safety checkpoint. Then **continue immediately** — auto-compaction should free context. The handoff preserves state as insurance.

**On RED (second consecutive):** STOP. Output resume command: `/evolve-loop <remaining> <strategy> <goal>`.

**On YELLOW:** Set `leanMode = true` for this cycle only. Lean mode reduces agent prompt depth and file reads — it does NOT trigger a session break.

Increment after each cycle completes: `CYCLES_THIS_SESSION=$(( CYCLES_THIS_SESSION + 1 ))`

### Phase 1: RESEARCH (every cycle)

Proactive research loop. Runs inline by the orchestrator (no separate agent). Transforms evaluation signals into research questions, generates Concept Cards, and filters them through the Research Ledger.

For the full 6-step protocol (ORIENT → GAP ANALYSIS → DIVERGENCE → RESEARCH → CONCEPTUALIZE → EVALUATE → DECIDE), see [phase1-research.md](phase1-research.md).

**Token budget:** 25K max. Skip when lean mode + no weaknesses + empty agenda. Phase gate: `bash scripts/phase-gate.sh research-to-discover $CYCLE $WORKSPACE_PATH`

---

### Eager Context Budget Estimation

At cycle start, **after passing the context window gate**, estimate total context cost for this specific cycle. This enables proactive lean mode entry and Self-MoA decisions. (Research: OPENDEV [arXiv:2603.05344], Token Consumption Prediction [OpenReview:1bUeVB3fov].)

| Task Type | Estimated Tokens |
|-----------|-----------------|
| S-task inline | ~5-10K |
| S-task worktree | ~20-40K |
| M-task worktree | ~40-80K |
| M-task Self-MoA | ~80-160K (N x single) |
| Orchestrator overhead | ~10-15K |

If `estimatedCycleTokens > tokenBudget.perCycle * 0.8`: drop lowest-priority tasks, switch M-tasks to single-Builder, or activate lean mode immediately.

### Lean Mode (cycle 10+ OR YELLOW from budget gate)

Activated when `context-budget.sh` returns YELLOW (cycle 10+ or headroom tight) OR when estimated cycle tokens exceed 80% of per-cycle budget:

| Optimization | Detail |
|-------------|--------|
| State.json | Read ONCE at cycle start; use in-memory copy thereafter |
| Agent results | Read summary only; read full file only if summary indicates a problem |
| Scout report | Extract task list from agent return value, not by reading scout-report.md |
| Eval checksums | Compute once, verify from memory |
| Benchmark delta | Skip for cycles where all tasks are S-complexity docs-only |

Saves ~10-15K tokens per cycle (from ~35K to ~20K).

### AgentDiet Trajectory Compression

Between phase transitions, compress accumulated context. (Research: AgentDiet [arXiv:2509.23586] — 40-60% input token savings.)

| After Phase | Keep | Remove |
|------------|------|--------|
| DISCOVER | Task list and eval definitions | Full Scout analysis |
| BUILD | build-report.md summary and worktree branch/SHA | Full Builder output |
| AUDIT | Verdict (PASS/WARN/FAIL) and issue list | Full Auditor analysis |
| SHIP | Commit SHA and benchmark delta | Commit details |

**Never prune:** challenge token, eval checksums, failed approach details, stagnation patterns.

### Cycle Integrity Setup

At cycle start, before any agent invocation, establish the integrity layer:

1. **Generate challenge token:**
   ```bash
   CHALLENGE=$(openssl rand -hex 8 2>/dev/null || head -c 8 /dev/urandom | xxd -p)
   ```
   Pass to every agent in their context block. Each agent embeds it in workspace output and ledger entries. `scripts/cycle-health-check.sh` verifies consistency.

2. **Plant canary files:**
   ```bash
   CANARY_VALUE=$(openssl rand -hex 16 2>/dev/null || head -c 16 /dev/urandom | xxd -p)
   echo "$CANARY_VALUE" > "$WORKSPACE_PATH/.integrity-canary"
   echo "$CANARY_VALUE" > "$WORKSPACE_PATH/.integrity-canary-expected"

   if [ ! -f ".evolve/evals/_canary-eval.md" ]; then
     cat > ".evolve/evals/_canary-eval.md" << 'CANARY'
   # Canary Eval (DO NOT REFERENCE)
   This file monitors for eval system probing. Any access = anomaly.
   ## Graders
   - `echo "canary-sentinel-$(date +%s)"`
   CANARY
   fi
   ```

3. **Adaptive strictness decay** (cross-session — research: "Alignment Faking", Anthropic):
   If first cycle of a new invocation, halve all `consecutiveClean` values in `auditorProfile`:
   ```
   for each taskType in auditorProfile:
     auditorProfile[taskType].consecutiveClean = floor(consecutiveClean / 2)
   ```

4. **Random full-audit flag** (blind monitoring):
   ```bash
   RANDOM_CHECK=$((RANDOM % 5))  # 0-4
   FORCE_FULL_AUDIT=false
   if [ "$RANDOM_CHECK" -eq 0 ]; then
     FORCE_FULL_AUDIT=true
   fi
   ```
   Pass `forceFullAudit` in Auditor context. Do NOT log whether triggered.

5. **Hash chain initialization** (tamper-evident ledger — research: SLSA transparency logs, in-toto):
   ```bash
   if command -v sha256sum &>/dev/null; then
     PREV_HASH=$(tail -1 .evolve/ledger.jsonl 2>/dev/null | sha256sum | cut -d' ' -f1)
   elif command -v shasum &>/dev/null; then
     PREV_HASH=$(tail -1 .evolve/ledger.jsonl 2>/dev/null | shasum -a 256 | cut -d' ' -f1)
   else
     PREV_HASH="unsupported"
   fi
   ```
   Every ledger entry MUST include `"prevHash":"<$PREV_HASH>"`. After writing each entry, update `PREV_HASH` to the hash of the entry just written.

### Atomic Cycle Number Allocation

1. Read `state.json`, note `version = V` and `lastCycleNumber`
2. `claimedCycle = lastCycleNumber + 1`
3. Write `state.json` with `lastCycleNumber = claimedCycle`, `version = V + 1`
4. Re-read `state.json` and verify `version == V + 1`
5. If version mismatch → re-read, re-claim next available number
6. Use `claimedCycle` as this iteration's cycle number `N`
7. Decrement `remainingCycles`

### Phase 2: DISCOVER

For the full Phase 2 protocol — convergence short-circuit, context pre-compute, prompt caching, Scout launch with full context schema, task claiming, eval quality check, and stagnation handling — see [phase2-discover.md](phase2-discover.md).

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

Log violations in build report under **Risks** and flag to Auditor.

### Per-Phase Context Selection Matrix

Each agent receives ONLY the fields it needs (Anthropic's **Select** strategy).

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

Builder does NOT need stateJson, projectDigest, benchmarkWeaknesses, or recentNotes. Auditor does NOT need instinctSummary or budgetRemaining. Saves ~3-5K tokens per agent invocation.

### Inter-Phase Handoff Format

Each phase writes `$WORKSPACE_PATH/handoff-<phase>.json` for the next phase.

```json
{
  "phase": "<scout|builder|auditor|ship>",
  "cycle": "<N>",
  "findings": "<1-3 sentence summary>",
  "decisions": ["<key decision 1>", "<key decision 2>"],
  "files_modified": ["<path/to/file1>"],
  "next_phase_context": { "<key>": "<value>" }
}
```

| File | Written by | Read by | Key fields |
|------|-----------|---------|------------|
| `handoff-scout.json` | Scout | Builder | `findings` (rationale), `files_modified`, `next_phase_context.taskSlug` |
| `handoff-builder.json` | Builder | Auditor | `findings` (approach), `files_modified`, `decisions`, `next_phase_context.worktreeBranch` |
| `handoff-auditor.json` | Auditor | Orchestrator (Ship) | `findings` (verdict), `decisions` (issues), `next_phase_context.verdict` |

If handoff file absent or malformed, fall back to full report (e.g., `scout-report.md`, `build-report.md`).

---

### Phase Boundary: DISCOVER -> BUILD

```bash
bash scripts/phase-gate.sh discover-to-build $CYCLE $WORKSPACE_PATH
# Exit 0 -> proceed to BUILD. Any other exit -> HALT.
```

### Phase 3: BUILD (loop per task)

For detailed Phase 3 implementation, see [phase3-build.md](skills/evolve-loop/phase3-build.md).

| Step | Detail |
|------|--------|
| Inline S-tasks | Execute first (per inst-007), commit sequentially |
| Independent worktree tasks | Launch IN PARALLEL using multiple Agent tool calls in a single message (one per independent task, each with `isolation: "worktree"`). See phase3-build.md Dependency Partitioning Algorithm. |
| Dependent tasks | Build sequentially within groups per phase3-build.md conflict graph |
| Isolation | Builder MUST use worktree isolation for coding projects |
| Retries | Max 3 attempts per task; failures logged to state.json |

---

### Phase Boundary: BUILD -> AUDIT

```bash
bash scripts/phase-gate.sh build-to-audit $CYCLE $WORKSPACE_PATH
# Exit 0 -> proceed to AUDIT. Any other exit -> HALT.
```

### Phase 4: AUDIT (Parallel)

The Auditor reviews Builder changes **in the worktree**. Worktree tasks built in parallel during Phase 3 MUST also be audited in parallel.

**Inline task audit:** Run eval graders before committing. If any grader fails, revert (`git checkout -- <files>`) and retry inline or escalate to Builder agent. Do NOT skip audit for inline tasks.

**Eval checksum verification** (before launching Auditor):
```bash
sha256sum -c $WORKSPACE_PATH/eval-checksums.json
```
If any checksum fails → HALT: "Eval tamper detected."

**Launch Auditor Agent** (model: tier-2 default, tier-1 for security-sensitive, tier-3 for clean builds):
- **Subagent invocation (REQUIRED):** Run via the runner script. The Auditor profile (`.evolve/profiles/auditor.json`) is read-only at the filesystem level (no `Edit`/`Write` outside the audit-report path) and bash is restricted to test runners and integrity scripts — Auditor cannot commit, push, or modify state.

  ```bash
  cat agents/evolve-auditor.md context.json | \
      MODEL_TIER_HINT="<resolved tier>" \
      bash scripts/subagent-run.sh auditor "$CYCLE" "$WORKSPACE_PATH"
  ```

  Legacy fallback: `LEGACY_AGENT_DISPATCH=1` for one A/B cycle only.

- Prompt: Read `agents/evolve-auditor.md` and pass as prompt
- Context:
  ```json
  {
    "workspacePath": "<$WORKSPACE_PATH>",
    "runId": "<$RUN_ID>",
    "evalsPath": ".evolve/evals/",
    "strategy": "<strategy>",
    "auditorProfile": "<state.json auditorProfile object>",
    "cycle": "<N>",
    "buildReport": "<$WORKSPACE_PATH>/build-report.md",
    "recentLedger": "<last 3 ledger entries>",
    "challengeToken": "<$CHALLENGE>",
    "forceFullAudit": "<$FORCE_FULL_AUDIT>",
    "handoffFromBuilder": "<contents of handoff-builder.json>"
  }
  ```

**Enhanced evaluation via `/evaluator` (optional):** When `strategy == "harden"` OR `forceFullAudit == true`, invoke `/evaluator --scope task --depth standard` for independent multi-dimensional assessment (6 dimensions with anti-gaming detection). Merge returned dimension scores into audit-report.md under `## Evaluator Scores`. Advisory only — supplements Auditor verdict, does not override. Skip when lean mode active or budget YELLOW/RED.

After Auditor completes:
- If `PASS-PENDING-EVAL` → proceed to eval gate (phase-gate runs `verify-eval.sh` as single source of truth)
- If `PASS` (post-eval) → proceed to **Phase 5 Ship**: `git apply` worktree patch, commit, push.
- If `WARN`, `FAIL`, or `SHIP_GATE_DENIED` → **drop the work, RECORD the failure, do NOT commit.** Lightweight failed-path:
  1. `git diff HEAD > "$WORKSPACE_PATH/failed.patch"` (capture failed code state for forensic review).
  2. `bash scripts/record-failure-to-state.sh "$WORKSPACE_PATH" "$VERDICT"` (append structured failure entry to `state.json.failedApproaches[]` — defects + verdict + audit-report SHA256 + git HEAD/tree state). Cost: 0 LLM calls, ~50ms.
  3. `git worktree remove --force "$WORKTREE_DIR"` (discard the failed code).

  The retrospective subagent is **NOT invoked per cycle.** It runs **separately, on demand or in batches** (e.g., weekly, or after N failures) — synthesizing cross-cycle patterns into failure-lesson YAMLs that feed into future `instinctSummary`. See [phase6-learn.md § 4c](phase6-learn.md) for the deferred-retrospective flow.
- For Builder retries (within the cycle), see [phase3-build.md](skills/evolve-loop/phase3-build.md) (retry/cleanup). Retry budget is consumed at Builder; once retries are exhausted and Auditor still says FAIL/WARN, the failed-path recording above runs.
- Phase-gate `audit-to-ship` promotes `PASS-PENDING-EVAL` → `PASS` only if `verify-eval.sh` passes
- Auditor does NOT run eval graders directly — this eliminates redundant eval execution

---

### Benchmark Delta Check & Pre-Ship Integrity Gate

For detailed steps, see [phase3-build.md](skills/evolve-loop/phase3-build.md).

| Check | Detail |
|-------|--------|
| Exemptions | Skip for `repair` strategy, first 3 cycles, or `meta`/`infrastructure` tasks |
| Regression | Any dimension -3 or more blocks shipping; 1 retry, then drop |
| Pre-ship gate | `verify-eval.sh` + `cycle-health-check.sh` run independently of all agents |
| Proceed | Only if both health check and eval verification pass |

```bash
bash scripts/phase-gate.sh audit-to-ship $CYCLE $WORKSPACE_PATH
# Exit 0 -> proceed to SHIP. Exit 1 -> block. Exit 2 -> HALT for human.
```

This is the **primary anti-cheating gate**: runs `verify-eval.sh` independently, runs `cycle-health-check.sh` (11 signals), verifies eval checksums.

### Phase 5: SHIP

For detailed instructions, see [phase5-ship.md](skills/evolve-loop/phase5-ship.md).

```bash
bash scripts/phase-gate.sh ship-to-learn $CYCLE $WORKSPACE_PATH
```
Verifies git is clean and updates `state.json.lastCycleNumber` (the SCRIPT does this, not the LLM).

---

### Phase 6: LEARN

For detailed instructions, see [phase6-learn.md](skills/evolve-loop/phase6-learn.md).

```bash
bash scripts/phase-gate.sh cycle-complete $CYCLE $WORKSPACE_PATH
```
Archives workspace, verifies all 3 reports exist, updates mastery ONLY if audit-report.md contains a genuine PASS verdict.
