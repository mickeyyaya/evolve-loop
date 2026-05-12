> Reference detail for phases.md. Load on-demand when implementation specifics are needed.
> Activation layer (phases.md) links here for: kernel hooks, skill inventory schema, rate limit recovery, context budget gate, cycle integrity code blocks, inter-phase handoff JSON.

## Contents
- [Kernel Hooks](#kernel-hooks) — v8.13.1 lifecycle integration code
- [Skill Inventory](#skill-inventory) — routing categories, JSON schema, fallback
- [Rate Limit Recovery](#rate-limit-recovery) — detection code + 4-step protocol
- [Context Budget Gate](#context-budget-gate) — bash invocation block
- [Cycle Integrity Setup](#cycle-integrity-setup) — challenge token, canary, decay, hash chain code
- [Inter-Phase Handoff](#inter-phase-handoff) — JSON schema
- [Phase 4 Audit](#phase-4-audit) — subagent invocation + context JSON

---

## Kernel Hooks

v8.13.1 lifecycle integration code blocks:

```bash
# At cycle start (run-cycle.sh handles this automatically):
bash scripts/lifecycle/cycle-state.sh init <cycle> <workspace>

# At each phase transition (orchestrator advances; runner DOES NOT auto-advance):
bash scripts/lifecycle/cycle-state.sh advance <new_phase> <agent> [worktree]

# At cycle end:
bash scripts/lifecycle/cycle-state.sh clear
```

Bypasses (emergency only, logged WARN):
- `EVOLVE_BYPASS_ROLE_GATE=1`
- `EVOLVE_BYPASS_PHASE_GATE=1`
- `EVOLVE_BYPASS_SHIP_GATE=1`

---

## Skill Inventory

### Scopes scanned (precedence: project > user > plugin — first-seen wins on name collision)

| Scope | Path | Typical count |
|---|---|---|
| `project` | `./skills/**/SKILL.md` | 5-20 |
| `user` | `~/.claude/skills/*/SKILL.md` or `~/.gemini/skills/*/SKILL.md` | 50-300 |
| `plugin/extension` | `~/.claude/plugins/.../SKILL.md` or `~/.gemini/extensions/*/skills/*/SKILL.md` | 10-100 per plugin |

IDE-specific mirror dirs (`.cursor/skills/`, `.kiro/skills/`, `.agents/skills/`) are **skipped** — only the canonical `skills/` directory of each plugin version is consumed.

### Routing Categories

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

For skill precedence, conflict resolution, phase eligibility, and budget-aware depth routing, see [reference/skill-routing.md](skill-routing.md).

### Inventory schema (`.evolve/skill-inventory.json`)

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
    "code-review": ["code-reviewer", "pr-review-workflow"],
    "security": ["security-review", "security-patterns-code-review"],
    "language:python": ["python-patterns", "python-review-patterns"]
  },
  "skills": {
    "e2e-testing": {
      "name": "e2e-testing",
      "description": "Playwright E2E testing patterns, Page Object...",
      "origin": "user",
      "path": "/Users/.../e2e-testing/SKILL.md",
      "referenceFiles": ["references/playwright-config.md"],
      "categories": ["testing", "e2e"]
    }
  }
}
```

**Fallback:** If `.evolve/skill-inventory.json` missing, invoke `bash scripts/utility/setup-skill-inventory.sh` before launching Scout. If the script fails, fall back to legacy LLM-parsing and log WARN to ledger.

---

## Rate Limit Recovery

Full detection code block:

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

**On rate limit detection (4-step protocol):**
1. Complete current phase (never break mid-phase)
2. Write handoff using Session Break Handoff Template (cause: "API rate limit")
3. Auto-schedule resume: try `/schedule` first (remote trigger at next hour), fall back to `/loop 5m`, fall back to manual resume instructions
4. **STOP** — do not start next phase

---

## Context Budget Gate

Full bash invocation:

```bash
CYCLES_THIS_SESSION=${CYCLES_THIS_SESSION:-0}
BUDGET_JSON=$(bash scripts/verification/context-budget.sh "$CYCLE_NUMBER" "$CYCLES_THIS_SESSION" "$WORKSPACE_PATH" 2>/dev/null)
BUDGET_EXIT=$?
BUDGET_STATUS=$(echo "$BUDGET_JSON" | grep -o '"status": *"[^"]*"' | cut -d'"' -f4)
REMAINING_ESTIMATE=$(echo "$BUDGET_JSON" | grep -o '"remainingCyclesEstimate": *[0-9]*' | grep -o '[0-9]*$')
```

**On RED (first occurrence):** Write enriched `handoff.md` as a safety checkpoint. Then **continue immediately** — auto-compaction should free context.

**On RED (second consecutive):** STOP. Output resume command: `/evolve-loop <remaining> <strategy> <goal>`.

Increment after each cycle: `CYCLES_THIS_SESSION=$(( CYCLES_THIS_SESSION + 1 ))`

---

## Cycle Integrity Setup

### 1. Challenge token generation

```bash
CHALLENGE=$(openssl rand -hex 8 2>/dev/null || head -c 8 /dev/urandom | xxd -p)
```
Pass to every agent in their context block. Each agent embeds it in workspace output and ledger entries.

### 2. Canary files

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

### 3. Adaptive strictness decay

If first cycle of a new invocation, halve all `consecutiveClean` values in `auditorProfile`:
```
for each taskType in auditorProfile:
  auditorProfile[taskType].consecutiveClean = floor(consecutiveClean / 2)
```

### 4. Random full-audit flag (blind monitoring)

```bash
RANDOM_CHECK=$((RANDOM % 5))  # 0-4
FORCE_FULL_AUDIT=false
if [ "$RANDOM_CHECK" -eq 0 ]; then
  FORCE_FULL_AUDIT=true
fi
```
Pass `forceFullAudit` in Auditor context. Do NOT log whether triggered.

### 5. Hash chain initialization

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

---

## Inter-Phase Handoff

JSON schema for `$WORKSPACE_PATH/handoff-<phase>.json`:

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

---

## Phase 4 Audit

### Subagent invocation

```bash
cat agents/evolve-auditor.md context.json | \
    MODEL_TIER_HINT="<resolved tier>" \
    bash scripts/dispatch/subagent-run.sh auditor "$CYCLE" "$WORKSPACE_PATH"
```

### Context JSON

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

### Audit verdict routing

- If `PASS-PENDING-EVAL` → proceed to eval gate (phase-gate runs `verify-eval.sh`)
- If `PASS` (post-eval) → proceed to Phase 5 Ship: `git apply` worktree patch, commit, push
- If `WARN`, `FAIL`, or `SHIP_GATE_DENIED`:
  1. `git diff HEAD > "$WORKSPACE_PATH/failed.patch"` (capture failed code state)
  2. `bash scripts/failure/record-failure-to-state.sh "$WORKSPACE_PATH" "$VERDICT"`
  3. `git worktree remove --force "$WORKTREE_DIR"`
