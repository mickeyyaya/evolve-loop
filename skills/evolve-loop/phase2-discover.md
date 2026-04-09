# Phase 2: DISCOVER

> Read this file when launching the Scout agent for codebase discovery, gap analysis, and task selection.

## Convergence Short-Circuit (check BEFORE launching Scout)

| `nothingToDoCount` | Action |
|--------------------|--------|
| `>= 2` | Check `discoveryVelocity`: if `discoveryVelocity.rolling3 > 0`, reset `nothingToDoCount` to 1 (discoveries pending → keep going). Otherwise skip Scout. Jump to Phase 6 with Operator in `"convergence-check"` mode. Operator can reset to 0 if new work detected. |
| `== 1` | **Escalation before convergence:** review last 3 cycles' deferred tasks for combinations, check strategy switch, propose a "radical" task. If viable task found → reset to 0, proceed. If not → launch Scout in `"convergence-confirmation"` mode (reads ONLY state.json + `git log --oneline -3`, MUST trigger new web research). If still nothing → increment to 2, skip to Phase 5. |
| `== 0` | Normal Scout launch. |

## Pre-compute Context

Read files once, pass inline slices:

```bash
# Cycle 1: full mode — no digest exists yet
# Cycle 2+: incremental mode — read digest + changed files
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

RECENT_NOTES=$(# extract last 5 "## Cycle" sections from notes.md)
BUILDER_NOTES=$(cat $WORKSPACE_PATH/builder-notes.md 2>/dev/null || cat .evolve/workspace/builder-notes.md 2>/dev/null || echo "")
RECENT_LEDGER=$(tail -3 .evolve/ledger.jsonl)
# instinctSummary and ledgerSummary come from state.json (already read)
```

**Shared values in agent context:** Include Layer 0 core rules from `memory-protocol.md` AND `sharedValues` from `SKILL.md` at the top of all agent context blocks. Place shared values first to maximize KV-cache reuse across parallel agent launches.

## Prompt Caching (provider-specific)

| Provider | Implementation |
|----------|---------------|
| Anthropic API | `cache_control: {"type": "ephemeral"}` on last static block |
| Google Gemini | `cachedContent` resource for static prefix |
| OpenAI | `store: true` on static prefix messages |
| Generic / self-hosted | Place static context first in system prompt |

Key principle: **static fields first, dynamic fields last** — reduces prompt-processing cost by 20-40%.

**Operator brief pre-read:** Check `$WORKSPACE_PATH/next-cycle-brief.json` (own run). Fall back to `.evolve/latest-brief.json` (shared). Pass contents to Scout context if present.

## Launch Scout Agent

Model selection: tier-1 if cycle 1 or goal-directed cycle <= 2; tier-3 if cycle 4+ with mature bandit data; tier-2 otherwise.

- **Platform dispatch:** Claude Code: `Agent` tool with `subagent_type: "general-purpose"`; Gemini CLI: `spawn_agent`; Generic: new LLM session.
- Prompt: Read `agents/evolve-scout.md` and pass as prompt
- Context:

```json
{
  // --- Static (stable across cycles, maximizes cache reuse) ---
  "projectContext": "<auto-detected>",
  "projectDigest": "<contents of project-digest.md, or null if cycle 1>",
  "workspacePath": "<$WORKSPACE_PATH>",
  "runId": "<$RUN_ID>",
  "goal": "<goal or null>",
  "strategy": "<strategy>",
  // --- Semi-stable (changes slowly) ---
  "instinctSummary": "<from state.json, inline>",
  "stateJson": "<state.json contents — evalHistory trimmed to last 5>",
  // --- Dynamic (changes every cycle) ---
  "cycle": "<N>",
  "mode": "full|incremental|convergence-confirmation",
  "changedFiles": ["<output of git diff HEAD~1 --name-only>"],
  "recentNotes": "<last 5 cycle entries from notes.md>",
  "builderNotes": "<contents of builder-notes.md, or empty>",
  "recentLedger": "<last 3 ledger entries>",
  "benchmarkWeaknesses": "<array of {dimension, score, taskTypeHint}, or empty>",
  "priorHypotheses": "<array of hypotheses from previous cycle's scout-report, for validation tracking>",
  "researchBrief": "<contents of research-brief.md from Phase 1>",
  "conceptCandidates": "<array of KEPT concept cards from Phase 1, each with +2 priority boost>",
  "challengeToken": "<$CHALLENGE>",
  "handoffFromOperator": "<contents of handoff-operator.json, or null>",
  "skillCategories": "<subset of skillInventory.categoryIndex matching projectContext language/framework — e.g., {testing: [...], language:python: [...], security: [...]}>"
}
```

**Skill routing context:** Pass `skillCategories` — the subset of `skillInventory.categoryIndex` keys that match `projectContext` (language, framework, domain). Always include `testing`, `security`, `code-review`, and `refactoring` categories regardless of project type. Scout uses these to populate `recommendedSkills` on each task. See [reference/skill-routing.md](reference/skill-routing.md) for precedence and conflict resolution, and [agent-templates.md](../../agents/agent-templates.md) § Skill Awareness.

Scout MUST write all output files to `$WORKSPACE_PATH`, NOT `.evolve/workspace/`.

**Implementation requirement:** Scout MUST identify target files for modification. Tasks that only create new documentation files are deprioritized unless:
- `projectContext.domain` is `"writing"` or `"research"`
- The goal explicitly requests documentation
- No existing files are suitable for the research finding

When research is performed, Scout writes a `Research → Implementation Map` in the scout-report showing how each finding translates to file changes.

## After Scout Completes

- Verify `$WORKSPACE_PATH/scout-report.md` exists. If not, check `.evolve/workspace/scout-report.md` and copy.
- Read `$WORKSPACE_PATH/scout-report.md`

## Task Claiming (parallel deduplication)

1. Read `state.json.evaluatedTasks` (note version V)
2. Filter out tasks with `decision: "selected"` or `"completed"`
3. Write remaining tasks with `decision: "selected"`, `cycle: N`, `runId: $RUN_ID`
4. Write state.json with `version = V + 1`, verify via OCC
5. If conflict → re-read, re-filter, retry (max 3)
6. Only build successfully claimed tasks

**Prerequisite check:** For each task with a `prerequisites` field, verify all listed slugs are `decision: "completed"` in `evaluatedTasks`. Unmet prerequisites → auto-defer with `deferralReason`. Tasks without `prerequisites` are unaffected.

## Eval Quality Check

Run on THIS cycle's eval files only:

```bash
for TASK_SLUG in <task slugs from scout-report>; do
  bash scripts/eval-quality-check.sh .evolve/evals/${TASK_SLUG}.md
  EVAL_QUALITY_EXIT=$?
  if [ "$EVAL_QUALITY_EXIT" -eq 2 ]; then
    echo "HALT: Level 0 (no-op) commands in ${TASK_SLUG}. Scout must rewrite evals."
  elif [ "$EVAL_QUALITY_EXIT" -eq 1 ]; then
    echo "WARN: Level 1 (tautological) commands in ${TASK_SLUG}. Flagging for Auditor."
  fi
done
```

## Eval Checksum Capture

```bash
sha256sum .evolve/evals/*.md > $WORKSPACE_PATH/eval-checksums.json
```

## Stagnation Handling

**If no tasks selected:** increment `stagnation.nothingToDoCount`. If >= 3 → STOP: "Project has converged." Otherwise → skip to Phase 5.

**Stagnation detection:** Handled by `scripts/cycle-health-check.sh` (deterministic). Scout reads stagnation findings from `$WORKSPACE/cycle-health.json` if present. Orchestrator HALTs if 3+ stagnation patterns active.

## Phase Boundary: DISCOVER → BUILD

```bash
bash scripts/phase-gate.sh discover-to-build $CYCLE $WORKSPACE_PATH
# Exit 0 -> proceed to BUILD. Any other exit -> HALT.
```
