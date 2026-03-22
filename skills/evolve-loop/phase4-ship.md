---
name: phase4-ship
description: "Phase 4 SHIP instructions — serial ship lock, domain-specific shipping, state updates, process rewards"
---

> Read this file when orchestrating Phase 4 (SHIP). Covers serial ship lock, domain-specific shipping, state updates, fitness scoring, and process rewards.

## Contents
- [Serial SHIP Lock](#serial-ship-lock-parallel-safety) — atomic lock acquisition
- [Shipping by Domain](#ship-by-domain-projectcontextshipmechanism) — git, file-save, export, custom
- [Mailbox Cleanup](#clearing-mailbox) — non-persistent message removal
- [State Updates](#updating-statejson) — OCC writes, fitness, ledger summary, eval history, mastery
- [Process Rewards](#process-rewards) — per-dimension scoring rubric, remediation triggers

# Evolve Loop — Phase 4: SHIP

Orchestrator inline — no agent needed. **This phase is not optional — every cycle MUST persist and distribute completed work.**

---

## Serial SHIP Lock (parallel safety)

Only one run can push at a time. Acquire lock before any git push:

```bash
LOCK_DIR=".evolve/.ship-lock"
MAX_WAIT=60
WAITED=0

while ! mkdir "$LOCK_DIR" 2>/dev/null; do
  if [ -f "$LOCK_DIR/info.json" ]; then
    LOCK_AGE=$(($(date +%s) - $(cat "$LOCK_DIR/info.json" | grep -o '"ts":[0-9]*' | cut -d: -f2)))
    if [ "$LOCK_AGE" -gt 300 ]; then
      rm -rf "$LOCK_DIR"  # break stale lock
      continue
    fi
  fi
  sleep 5
  WAITED=$((WAITED + 5))
  if [ "$WAITED" -ge "$MAX_WAIT" ]; then
    echo "HALT: Could not acquire ship lock after ${MAX_WAIT}s"
    exit 1
  fi
done

echo '{"runId":"'$RUN_ID'","ts":'$(date +%s)'}' > "$LOCK_DIR/info.json"
```

All git operations below happen inside the lock. Release at end: `rm -rf "$LOCK_DIR"`

---

## Ship by Domain (`projectContext.shipMechanism`)

### `git` (default — coding domain)

1. **Verify commits are clean:** `git status` + `git log --oneline -<N>`
2. **Commit uncommitted changes:** `git add <files>` + `git commit -m "<type>: <description>"`
3. **Rebase and push:** `git pull --rebase origin main` then `git push origin <branch>`. If rebase conflicts → abort, release lock, return to Builder.
4. **Auto-bump plugin version:**
   ```bash
   CURRENT_VER=$(python3 -c "import json; print(json.load(open('.claude-plugin/plugin.json'))['version'])")
   MAJOR=$(echo "$CURRENT_VER" | cut -d. -f1)
   MINOR=$(echo "$CURRENT_VER" | cut -d. -f2)
   PATCH=$(echo "$CURRENT_VER" | cut -d. -f3)
   NEW_VER="${MAJOR}.${MINOR}.$((PATCH + 1))"

   python3 -c "
   import json
   for f in ['.claude-plugin/plugin.json', '.claude-plugin/marketplace.json']:
       with open(f) as fh: d = json.load(fh)
       if 'version' in d: d['version'] = '$NEW_VER'
       if 'plugins' in d: d['plugins'][0]['version'] = '$NEW_VER'
       with open(f, 'w') as fh: json.dump(d, fh, indent=2)
   "

   git add .claude-plugin/plugin.json .claude-plugin/marketplace.json
   git commit -m "chore: bump version to v${NEW_VER}"
   git push origin <branch>
   ```

   | Bump Type | When |
   |-----------|------|
   | Patch | Each cycle (e.g., 7.2.0 -> 7.2.1) |
   | Minor | Meta-cycle milestones (every 5 cycles) or manual override |
   | Major | Manual only |

5. **Publish:** `./publish.sh` — mandatory after every push.

### `file-save` (writing, research domains)

1. Verify changes saved and non-empty
2. Backup to `.evolve/history/cycle-{N}/output/`
3. Log ship event in ledger (no git needed)
4. Skip publish.sh

### `export` (design domain)

1. Export artifacts from source files
2. Save to `.evolve/history/cycle-{N}/exports/`
3. Log ship event in ledger
4. Skip publish.sh

### `custom`

Read ship commands from `.evolve/domain.json` `shipCommands` array. Execute each in order. Fall back to `file-save` if undefined.

---

## Clearing Mailbox

Remove non-persistent messages from `$WORKSPACE_PATH/agent-mailbox.md`:
```bash
grep -v "| false |" $WORKSPACE_PATH/agent-mailbox.md > /tmp/mailbox-tmp.md && mv /tmp/mailbox-tmp.md $WORKSPACE_PATH/agent-mailbox.md
```

---

## Updating state.json

Use OCC protocol (see [memory-protocol.md](skills/evolve-loop/memory-protocol.md)):

| Update | Detail |
|--------|--------|
| `evaluatedTasks` | Mark completed tasks |
| `lastCycleNumber` | MAX with existing value |
| `stagnation.nothingToDoCount` | Reset to 0 |
| `lastUpdated` | Current timestamp |
| `version` | Increment |

**Fitness scoring:**
```
fitnessScore = round(0.25 * discover + 0.30 * build + 0.20 * audit + 0.15 * ship + 0.10 * learn, 2)
```
- Decreased 2 consecutive cycles → `fitnessRegression: true` (Operator HALT signal)
- Increased or steady → `fitnessRegression: false`
- Store in `fitnessScore` and `fitnessHistory` (last 3 scores)

**Ledger summary** (aggregated stats so agents never read full ledger):
```json
"ledgerSummary": {
  "totalEntries": "<count>", "cycleRange": ["<first>", "<last>"],
  "scoutRuns": "<count>", "builderRuns": "<count>",
  "totalTasksShipped": "<sum>", "totalTasksFailed": "<sum>", "avgTasksPerCycle": "<ratio>"
}
```

**Eval history:** Trim to last 5 entries. Append new entry with delta metrics:
```json
{
  "cycle": "<N>", "verdict": "PASS|WARN|FAIL", "checks": "<total>", "passed": "<N>", "failed": "<N>",
  "delta": {
    "tasksShipped": "<count>", "tasksAttempted": "<count>", "auditIterations": "<avg>",
    "successRate": "<ratio>", "instinctsExtracted": "<count>", "stagnationPatterns": "<count>"
  }
}
```

**Mastery updates:**
- `successRate === 1.0` → increment `consecutiveSuccesses`
- `consecutiveSuccesses >= 3` and not proficient → advance level, reset counter
- `successRate < 0.5` for 2 consecutive cycles → regress level, reset counter

---

## Process Rewards

Append to `processRewardsHistory` (rolling 3 entries):

| Phase | 1.0 | 0.5 | 0.0 |
|-------|-----|-----|-----|
| **discover** | All selected tasks shipped | 50%+ shipped | <50% shipped |
| **build** | All pass audit first attempt | Some need retry | 3+ audit failures |
| **audit** | No false positives, all evals run | 1 false positive or missing eval | Multiple false positives |
| **ship** | Clean commit, no post-commit fixes | Minor fixup needed | Failed push or dirty state |
| **learn** | Instincts extracted AND cited | Extracted but none cited | None extracted |
| **skillEfficiency** | Tokens decreased from baseline | Stable (+/-5%) | Tokens increased |

**Per-cycle remediation check:** If any dimension < 0.7 for 2+ consecutive entries → append to `pendingImprovements`:

| Dimension | Suggested Task |
|-----------|---------------|
| `discover < 0.7` | Improve Scout task sizing or relevance |
| `build < 0.7` | Add Builder guidance or simplify task complexity |
| `audit < 0.7` | Review eval grader quality and coverage |
| `ship < 0.7` | Fix commit workflow or git state issues |
| `learn < 0.7` | Extract instincts from recent successful cycles |
| `skillEfficiency < 0.7` | Reduce prompt overhead in skill/agent files |

Clear resolved entries when dimension rises above 0.7 for 2 consecutive cycles.

7. **Release ship lock:** `rm -rf "$LOCK_DIR"`
