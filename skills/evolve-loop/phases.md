# Evolve Loop — Phase Instructions

Detailed orchestrator instructions for each phase. Optimized for fast iteration with diverse small/medium tasks per cycle.

**Important:** Every agent context block must include `goal` (string or null) and `strategy` (one of: `balanced`, `innovate`, `harden`, `repair`, `ultrathink`).

## Phase 0: CALIBRATE (once per invocation)

Runs **once per `/evolve-loop` invocation**, not per cycle. Establishes a project-level benchmark baseline.

For detailed calibration instructions (deduplication check, automated + LLM scoring, composite computation, high-water mark tracking, benchmark report writing), see [phase0-calibrate.md](skills/evolve-loop/phase0-calibrate.md).

**Quick reference:**
- Skip if `projectBenchmark.lastCalibrated` is < 1 hour ago
- Run automated checks from `benchmark-eval.md` for all 8 dimensions
- Compute composite: `0.7 * automated + 0.3 * llm` per dimension
- Store in `state.json.projectBenchmark`, pass `benchmarkWeaknesses` to Scout
- Checksum `benchmark-eval.md` for tamper detection in Phase 3→4

---

## FOR each cycle (while remainingCycles > 0):

### Lean Mode (cycles 4+ of an invocation)

After the first 3 cycles of an invocation, the orchestrator switches to lean mode to prevent context bloat:
- **State.json**: Read ONCE at cycle start. Do NOT re-read before Phase 4 — use the in-memory copy.
- **Agent results**: Do NOT read full workspace files after agent completion. Instead, the agent's returned summary is sufficient. Only read the full file if the summary indicates a problem.
- **Scout report**: Extract task list from agent return value, not by reading scout-report.md separately.
- **Eval checksums**: Compute once, verify from memory — do not re-read the checksum file.
- **Benchmark delta**: Skip for cycles where all tasks are S-complexity docs-only changes (no behavioral risk).

Lean mode saves ~15-20K tokens per cycle (from ~50K to ~30K) by eliminating redundant file reads that accumulate in the orchestrator's conversation context.

### Cycle Integrity Setup

At the start of each cycle, before any agent is invoked, establish the integrity layer:

1. **Generate challenge token** (proof-of-invocation chain):
   ```bash
   CHALLENGE=$(openssl rand -hex 8 2>/dev/null || head -c 8 /dev/urandom | xxd -p)
   ```
   This token is passed to every agent in their context block. Each agent embeds it in their workspace output files and ledger entries. The `scripts/cycle-health-check.sh` script verifies consistency.

2. **Plant canary files** (honeypot detection — research basis: Canarytokens):
   ```bash
   # Workspace canary: random value that no legitimate agent should touch
   CANARY_VALUE=$(openssl rand -hex 16 2>/dev/null || head -c 16 /dev/urandom | xxd -p)
   echo "$CANARY_VALUE" > "$WORKSPACE_PATH/.integrity-canary"
   echo "$CANARY_VALUE" > "$WORKSPACE_PATH/.integrity-canary-expected"

   # Honeypot eval: fake eval file that should never be referenced
   if [ ! -f ".evolve/evals/_canary-eval.md" ]; then
     cat > ".evolve/evals/_canary-eval.md" << 'CANARY'
   # Canary Eval (DO NOT REFERENCE)
   This file monitors for eval system probing. Any access = anomaly.
   ## Graders
   - `echo "canary-sentinel-$(date +%s)"`
   CANARY
   fi
   ```

3. **Adaptive strictness decay** (cross-session — research basis: "Alignment Faking", Anthropic):
   If this is the first cycle of a new invocation, halve all `consecutiveClean` values in `auditorProfile`:
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
   Pass `forceFullAudit` in the Auditor context. Do NOT log whether this was triggered — blind monitoring means the agent should not know which cycles are scrutinized.

5. **Hash chain initialization** (tamper-evident ledger — research basis: SLSA transparency logs, in-toto):
   Compute the hash of the last ledger entry to seed this cycle's chain:
   ```bash
   if command -v sha256sum &>/dev/null; then
     PREV_HASH=$(tail -1 .evolve/ledger.jsonl 2>/dev/null | sha256sum | cut -d' ' -f1)
   elif command -v shasum &>/dev/null; then
     PREV_HASH=$(tail -1 .evolve/ledger.jsonl 2>/dev/null | shasum -a 256 | cut -d' ' -f1)
   else
     PREV_HASH="unsupported"
   fi
   ```
   Every ledger entry written during this cycle MUST include `"prevHash":"<$PREV_HASH>"` in its `data` object. After writing each entry, update `PREV_HASH` to the hash of the entry just written. This creates a Merkle chain where inserting, deleting, or modifying entries retroactively breaks the chain. The `scripts/cycle-health-check.sh` verifies chain integrity.

### Atomic Cycle Number Allocation

At the start of each cycle iteration, claim the next cycle number atomically:

1. Read `state.json`, note `version = V` and `lastCycleNumber`
2. `claimedCycle = lastCycleNumber + 1`
3. Write `state.json` with `lastCycleNumber = claimedCycle`, `version = V + 1`
4. Immediately re-read `state.json` and verify `version == V + 1`
5. If version mismatch (another run claimed first) → re-read, re-claim next available number
6. Use `claimedCycle` as this iteration's cycle number `N`
7. Decrement `remainingCycles`

This ensures parallel runs get non-colliding cycle numbers (e.g., Run A gets 8,9 while Run B gets 10,11).

### Phase 1: DISCOVER

**Convergence Short-Circuit** (check BEFORE launching Scout):
- Read `stagnation.nothingToDoCount` from state.json:
  - If `>= 2`: Skip Scout entirely. Jump to Phase 5 with Operator in `"convergence-check"` mode. Operator can reset `nothingToDoCount` to 0 if it detects new work (e.g., external changes via `git log`).
  - If `== 1`: **Escalation before convergence** — before running convergence-confirmation, the orchestrator attempts to unlock new work:
    1. Review the last 3 cycles' deferred tasks (from `evaluatedTasks` with `decision: "deferred"`) for items that could be combined into a single task
    2. Check if switching strategy (e.g., `balanced` → `innovate`, `harden`, or `ultrathink`) would surface new task candidates
    3. Propose a "radical" task that challenges an existing assumption or convention in the codebase (think harder — re-read code for new angles)
    If escalation produces a viable task → reset `nothingToDoCount` to 0, proceed with normal Scout launch using the escalated task as a seed.
    If escalation fails → Launch Scout in `"convergence-confirmation"` mode — Scout reads ONLY state.json + `git log --oneline -3` and MUST trigger new web research to find potential external tasks/updates, bypassing any cooldowns. No notes, no ledger, no instincts, no codebase scan. If still nothing found, increment to 2 and skip to Phase 5.
  - If `== 0`: Proceed with normal Scout launch below.

**Pre-compute context** (orchestrator reads files once, passes inline slices):
```bash
# Cycle 1: full mode — no digest exists yet
# Cycle 2+: incremental mode — read digest + changed files
# Check shared digest first, then run-local
if [ -f .evolve/project-digest.md ]; then
  MODE="incremental"
  DIGEST=$(cat .evolve/project-digest.md)
  CHANGED=$(git diff HEAD~1 --name-only 2>/dev/null)
elif [ -f $WORKSPACE_PATH/project-digest.md ]; then
  MODE="incremental"
  DIGEST=$(cat $WORKSPACE_PATH/project-digest.md)
  CHANGED=$(git diff HEAD~1 --name-only 2>/dev/null)
else
  MODE="full"
fi

# Read recent notes (last 5 cycle entries, not full file)
RECENT_NOTES=$(# extract last 5 "## Cycle" sections from notes.md)

# Read builder notes: own run first, then shared fallback
BUILDER_NOTES=$(cat $WORKSPACE_PATH/builder-notes.md 2>/dev/null || cat .evolve/workspace/builder-notes.md 2>/dev/null || echo "")

# Read recent ledger (last 3 lines)
RECENT_LEDGER=$(tail -3 .evolve/ledger.jsonl)

# instinctSummary and ledgerSummary come from state.json (already read)
```

**Shared values in agent context:** The Layer 0 core rules from `memory-protocol.md` AND the `sharedValues` block from `SKILL.md` must be included at the top of all agent context blocks. Include `sharedValues` as a JSON field in every agent's static context:
```json
{
  "sharedValues": { /* from SKILL.md Shared Agent Values section */ },
  // ... rest of agent context
}
```
Apply agent-specific overrides as documented in SKILL.md (e.g., Auditor adds confidence-scoring, Operator removes ledger-entry). Placing shared values first maximizes KV-cache reuse across parallel agent launches — concurrent builders or auditors that share the same static prefix benefit from prompt cache hits without re-encoding the rules.

**Prompt caching (provider-specific optimization):** Most LLM API providers support some form of prompt caching for repeated prefixes. The evolve-loop's context block is structured with static fields first (shared values, project context) and dynamic fields last (cycle number, changed files) to maximize cache reuse.

**Provider-specific implementation:**
- **Anthropic API:** Apply `cache_control: {"type": "ephemeral"}` to the last static block (typically `projectContext`). Dynamic fields must NOT carry this marker.
- **Google Gemini:** Use `cachedContent` resource for the static prefix. Create once per session, reference in subsequent calls.
- **OpenAI:** Enable `store: true` on the static prefix messages. The API auto-caches identical prefixes.
- **Generic / self-hosted:** Place static context first in system prompt. Many inference engines (vLLM, TGI) auto-cache matching prefixes.

The key principle is universal: **static fields first, dynamic fields last**. This maximizes cache hits regardless of provider, reducing prompt-processing cost by 20-40% on repeated context. See `docs/token-optimization.md` § KV-Cache Prefix Optimization for background.

**Operator brief pre-read:** Before launching Scout, check if `$WORKSPACE_PATH/next-cycle-brief.json` exists (from own run's previous cycle). If not found, fall back to `.evolve/latest-brief.json` (shared, written by most recent run). If present, pass its contents in the Scout context so Scout can apply `recommendedStrategy`, `taskTypeBoosts`, and `avoidAreas` during task selection.

Launch **Scout Agent** (model: per routing table — tier-1 if cycle 1 or goal-directed cycle ≤ 2 (strategic foundation sets trajectory for entire session), tier-3 if cycle 4+ with mature bandit data (3+ arms with pulls ≥ 3, selection becomes data-driven), tier-2 otherwise):
- **Platform dispatch:** Use your platform's subagent mechanism — Claude Code: `Agent` tool with `subagent_type: "general-purpose"`; Gemini CLI: `spawn_agent`; Generic: launch a new LLM session with the prompt below.
- Prompt: Read `agents/evolve-scout.md` and pass as prompt
- Context:
  ```json
  {
    // --- Static (stable across cycles, maximizes cache-like reuse) ---
    "projectContext": <auto-detected>,
    "projectDigest": "<contents of project-digest.md, or null if cycle 1>",
    "workspacePath": "<$WORKSPACE_PATH>",
    "runId": "<$RUN_ID>",
    "goal": <goal or null>,
    "strategy": <strategy>,
    // --- Semi-stable (changes slowly, every few cycles) ---
    "instinctSummary": "<from state.json, inline>",
    "stateJson": <state.json contents — evalHistory trimmed to last 5 entries>,
    // --- Dynamic (changes every cycle) ---
    "cycle": <N>,
    "mode": "full|incremental|convergence-confirmation",
    "changedFiles": ["<output of git diff HEAD~1 --name-only>"],
    "recentNotes": "<last 5 cycle entries from notes.md, inline>",
    "builderNotes": "<contents of builder-notes.md from last cycle, or empty string>",
    "recentLedger": "<last 3 ledger entries, inline>",
    "benchmarkWeaknesses": "<array of {dimension, score, taskTypeHint} from Phase 0, or empty>",
    "challengeToken": "<$CHALLENGE>",
    "handoffFromOperator": "<contents of handoff-operator.json from previous cycle, or null>"
  }
  ```

**IMPORTANT:** The Scout MUST write all output files (scout-report.md, cycle summaries) to `$WORKSPACE_PATH`, NOT to `.evolve/workspace/`. The `workspacePath` is passed in the context block above. If the Scout writes to the wrong location, copy the files to `$WORKSPACE_PATH` before proceeding.

After Scout completes:
- Verify `$WORKSPACE_PATH/scout-report.md` exists. If not, check `.evolve/workspace/scout-report.md` and copy it to `$WORKSPACE_PATH`.
- Read `$WORKSPACE_PATH/scout-report.md`

- **Task Claiming (parallel deduplication):**
  Before building, claim each selected task via OCC protocol to prevent two parallel runs from building the same task:
  1. Read `state.json.evaluatedTasks` (note version V)
  2. Filter out any task whose slug already has `decision: "selected"` or `decision: "completed"`
  3. Write remaining tasks to `evaluatedTasks` with `decision: "selected"`, `cycle: N`, `runId: $RUN_ID`
  4. Write state.json with `version = V + 1`, verify via OCC protocol
  5. If conflict → re-read, re-filter, retry (max 3 retries)
  6. Only build tasks that were successfully claimed

- **Prerequisite check:** For each proposed task that includes a `prerequisites` field, verify all listed slugs appear in `state.json.evaluatedTasks` with `decision: "completed"`. Any task with an unmet prerequisite is automatically deferred: add it to `evaluatedTasks` with `decision: "deferred"` and `deferralReason: "prerequisite not met: <slug>"`, then log the prerequisite slug so the Scout can propose it in the next cycle. Tasks without a `prerequisites` field are unaffected. This check is a lightweight sequencing aid — the Scout may override it by omitting `prerequisites` when a task is genuinely independent of its nominal dependency.
- Verify eval definitions were created in `.evolve/evals/`
- **Eval quality check** (deterministic rigor classification — research basis: cycle 101 tautological eval incident):
  Run only on eval files created for THIS cycle's tasks (not the entire evals directory):
  ```bash
  # Check each task's eval file individually
  for TASK_SLUG in <task slugs from scout-report>; do
    bash scripts/eval-quality-check.sh .evolve/evals/${TASK_SLUG}.md
    EVAL_QUALITY_EXIT=$?
    if [ "$EVAL_QUALITY_EXIT" -eq 2 ]; then
      echo "HALT: Eval quality check found Level 0 (no-op) commands in ${TASK_SLUG}. Scout must rewrite evals."
      # Log to ledger and halt cycle
    elif [ "$EVAL_QUALITY_EXIT" -eq 1 ]; then
      echo "WARN: Eval quality check found Level 1 (tautological) commands in ${TASK_SLUG}. Flagging for Auditor."
      # Pass warnings to Auditor context for deeper review
    fi
  done
  ```
  This catches the exact attack from cycle 101 (`grep -q "string" source.js`) deterministically.
- **Eval checksum capture:** Compute `sha256sum` of each eval file in `.evolve/evals/` and store in `workspace/eval-checksums.json`:
  ```bash
  sha256sum .evolve/evals/*.md > $WORKSPACE_PATH/eval-checksums.json
  ```
  These checksums are verified before Auditor runs evals (Phase 3) to detect tampering.
- Merge research query updates into state.json (if research was performed)
- If no tasks selected:
  - Increment `stagnation.nothingToDoCount` in state.json
  - If `stagnation.nothingToDoCount >= 3` → STOP: "Project has converged."
  - Otherwise → skip to Phase 5

- **Stagnation detection** (run after every Scout phase):
  Check `stagnation.recentPatterns` in state.json for repeated failure patterns:
  1. **Same-file churn** — if the same file(s) appear in `failedApproaches` across 2+ consecutive cycles → flag as stagnation
  2. **Same-error repeat** — if the same error message recurs across cycles → flag with suggestion to try alternative approach
  3. **Diminishing returns** — if the last 3 cycles each shipped fewer tasks than the previous → flag as diminishing returns

  When stagnation is detected, the orchestrator should:
  - Log the pattern in `stagnation.recentPatterns` with type and cycle range
  - Pass it to the Scout as context so it avoids the stagnant area
  - If 3+ stagnation patterns are active simultaneously → trigger Operator HALT

### Context Quality Checklist (CEMM)

Based on the Context Engineering Maturity Model (arXiv:2603.09619), apply these five criteria to every context block assembled in Phase 1 before passing it to the Scout:

1. **Relevance** — Is this context needed by the current phase? _(e.g., do not send the full benchmark report to Scout when Scout only needs `benchmarkWeaknesses`)_
2. **Sufficiency** — Does the context have enough info for the agent to complete its task? _(e.g., Scout needs `recentNotes` + `changedFiles` + `stateJson` to propose meaningful tasks — omitting any one risks shallow proposals)_
3. **Isolation** — Is this phase's context independent from other phases' internal state? _(e.g., Scout context must NOT include Builder worktree paths or Auditor verdicts from the same cycle — those are internal to later phases)_
4. **Economy** — Are we sending the minimum tokens needed? _(e.g., pass `recentLedger` as last 3 lines, not the full ledger; pass instinct compact summary, not raw YAML files)_
5. **Provenance** — Can we trace where each piece of context came from? _(e.g., `builderNotes` is sourced from `$WORKSPACE_PATH/builder-notes.md`, falling back to `.evolve/workspace/builder-notes.md` — always document the source path)_

Violations should be logged in the build report under **Risks** and flagged to the Auditor.

### Per-Phase Context Selection Matrix

Each agent should receive ONLY the fields it needs. This table defines the minimum context per phase, implementing Anthropic's **Select** strategy ([anthropic.com/engineering/effective-context-engineering-for-ai-agents](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents)).

| Field | Scout | Builder | Auditor | Operator |
|-------|:-----:|:-------:|:-------:|:--------:|
| `cycle` | ✅ | ✅ | ✅ | ✅ |
| `strategy` | ✅ | ✅ | ✅ | — |
| `challengeToken` | ✅ | ✅ | ✅ | — |
| `budgetRemaining` | ✅ | ✅ | — | ✅ |
| `instinctSummary` | ✅ | ✅ | — | — |
| `stateJson` (full) | ✅ | — | — | ✅ |
| `projectContext` | ✅ | — | — | — |
| `projectDigest` | ✅ | — | — | — |
| `changedFiles` | ✅ | — | — | — |
| `recentNotes` | ✅ | — | — | ✅ |
| `benchmarkWeaknesses` | ✅ | — | — | — |
| `task` (from scout-report) | — | ✅ | — | — |
| `buildReport` | — | — | ✅ | — |
| `auditorProfile` | — | — | ✅ | — |
| `recentLedger` | ✅ | — | ✅ | ✅ |

**Key savings:** Builder does NOT need stateJson, projectDigest, benchmarkWeaknesses, or recentNotes. Auditor does NOT need instinctSummary or budgetRemaining. These omissions save ~3-5K tokens per agent invocation.

### Inter-Phase Handoff Format

Each phase writes a structured handoff file to `$WORKSPACE_PATH/handoff-<phase>.json` for the next phase to consume. This compact contract eliminates redundant file re-reads across phase boundaries, targeting 40-60% token savings on cross-phase context.

**Schema (`phaseHandoff`):**
```json
{
  "phase": "<scout|builder|auditor|ship>",
  "cycle": <N>,
  "findings": "<1-3 sentence summary of what this phase discovered or verified>",
  "decisions": ["<key decision 1>", "<key decision 2>"],
  "files_modified": ["<path/to/file1>", "<path/to/file2>"],
  "next_phase_context": {
    "<key>": "<value>"
  }
}
```

**File naming convention:** `handoff-<phase>.json` where `<phase>` is the writing phase name (e.g., `handoff-scout.json`, `handoff-builder.json`, `handoff-auditor.json`).

**Ownership table:**

| File | Written by | Read by | Key fields passed |
|------|-----------|---------|-------------------|
| `handoff-scout.json` | Scout | Builder | `findings` (task rationale), `files_modified` (files to touch), `next_phase_context.taskSlug` |
| `handoff-builder.json` | Builder | Auditor | `findings` (approach summary), `files_modified` (changed files list), `decisions` (design choices), `next_phase_context.worktreeBranch` |
| `handoff-auditor.json` | Auditor | Orchestrator (Ship) | `findings` (audit verdict summary), `decisions` (issues found/waived), `next_phase_context.verdict` |

**Usage:** The receiving phase reads the handoff file instead of re-reading the full workspace report. If the handoff file is absent or malformed, fall back to reading the full report (e.g., `scout-report.md`, `build-report.md`).

---

### Phase Boundary: DISCOVER → BUILD

Before entering Phase 2, run precondition assertions:
```bash
# Verify Scout produced substantive output
[ -s "$WORKSPACE_PATH/scout-report.md" ] || { echo "HALT: Scout report missing or empty — Phase 1 may have been skipped"; exit 1; }
```

### Phase 2: BUILD (loop per task)

For detailed Phase 2 implementation including task partitioning, parallelization, worktree isolation, and Builder agent dispatch, see [phase2-build.md](skills/evolve-loop/phase2-build.md).

**Quick reference:**
- Inline S-tasks execute first (per inst-007 policy), committed sequentially
- Independent worktree tasks build in parallel via platform agent mechanism
- Dependent tasks (shared files) build sequentially within groups
- Builder MUST use worktree isolation for coding projects
- Max 3 retry attempts per task; failures logged to state.json

---

### Phase Boundary: BUILD → AUDIT

Before entering Phase 3, run precondition assertions:
```bash
# Verify Builder produced substantive output
[ -s "$WORKSPACE_PATH/build-report.md" ] || { echo "HALT: Build report missing or empty — Phase 2 may have been skipped"; exit 1; }
```

### Phase 3: AUDIT (Parallel)

The Auditor reviews the Builder's changes **in the worktree** (or reads the diff from the build report). The worktree remains intact during audit. **Parallelization:** Because worktree tasks are built in parallel during Phase 2, they MUST also be audited in parallel during Phase 3 to minimize latency.

**Inline task audit:** For tasks implemented inline by the orchestrator (per inst-007 S-complexity policy), the orchestrator MUST still run eval graders before committing. At minimum, execute all bash eval graders from the task's eval definition and verify exit 0. If any grader fails, revert the inline changes (`git checkout -- <files>`) and either retry inline or escalate to a Builder agent. Do NOT skip the audit phase for inline tasks — the audit is the quality gate, not the builder.

**Eval checksum verification** (before launching Auditor):
Verify that eval files haven't been tampered with since Scout created them:
```bash
sha256sum -c $WORKSPACE_PATH/eval-checksums.json
```
If any checksum fails → HALT: "Eval tamper detected — eval file modified after Scout created it. Investigate before proceeding."

Launch **Auditor Agent** (model: per routing table — tier-2 default, tier-1 for security-sensitive, tier-3 for clean builds):
- **Platform dispatch:** Use your platform's subagent mechanism (Claude Code: `Agent` tool; Gemini CLI: `spawn_agent`; Generic: new LLM session).
- Prompt: Read `agents/evolve-auditor.md` and pass as prompt
- Context:
  ```json
  {
    // --- Static ---
    "workspacePath": "<$WORKSPACE_PATH>",
    "runId": "<$RUN_ID>",
    "evalsPath": ".evolve/evals/",
    "strategy": <strategy>,
    // --- Semi-stable ---
    "auditorProfile": "<state.json auditorProfile object>",
    // --- Dynamic ---
    "cycle": <N>,
    "buildReport": "<$WORKSPACE_PATH>/build-report.md",
    "recentLedger": "<last 3 ledger entries, inline>",
    "challengeToken": "<$CHALLENGE>",
    "forceFullAudit": <$FORCE_FULL_AUDIT>,
    "handoffFromBuilder": "<contents of handoff-builder.json>"
  }
  ```

After Auditor completes, handle the verdict per the post-audit procedure in [phase2-build.md](skills/evolve-loop/phase2-build.md) (verdict handling, worktree merge/cleanup, auditor profile updates). Then proceed to next task or the Benchmark Delta Check if all tasks done.

---

### Benchmark Delta Check & Pre-Ship Integrity Gate

For detailed Benchmark Delta Check steps (exemptions, dimension comparison, block/retry logic) and Pre-Ship Integrity Gate (independent eval re-execution, cycle health fingerprint, phase transition logging), see [phase2-build.md](skills/evolve-loop/phase2-build.md).

**Quick reference:**
- Skip delta check for `repair` strategy, first 3 cycles, or `meta`/`infrastructure` tasks
- Any dimension regression of -3 or more blocks shipping; 1 retry allowed, then drop
- Pre-ship gate runs `verify-eval.sh` and `cycle-health-check.sh` independently of all agents
- Only proceed to Phase 4 if both health check and eval verification pass

### Phase 4: SHIP

For detailed Phase 4 instructions (serial ship lock, domain-specific shipping, state updates, process rewards), see [phase4-ship.md](skills/evolve-loop/phase4-ship.md).

---

### Phase 5: LEARN

For detailed Phase 5 instructions (instinct extraction, memory consolidation, operator check, meta-cycle self-improvement, context management), see [phase5-learn.md](skills/evolve-loop/phase5-learn.md).
