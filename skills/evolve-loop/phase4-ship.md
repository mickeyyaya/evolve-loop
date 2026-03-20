---
name: phase4-ship
description: "Phase 4 SHIP instructions — serial ship lock, domain-specific shipping, state updates, process rewards"
---

# Evolve Loop — Phase 4: SHIP

Orchestrator inline — no agent needed. This phase handles persisting and distributing completed work after all tasks pass audit.

---

### Phase 4: SHIP (orchestrator inline — MANDATORY)

No agent needed. The orchestrator handles shipping directly. **This phase is not optional — every cycle MUST persist and distribute completed work.**

#### Serial SHIP Lock (parallel safety)

The SHIP phase is inherently serial — only one run can push at a time. Acquire a lock before any git push operations:

```bash
# Acquire lock (mkdir is atomic on POSIX)
LOCK_DIR=".evolve/.ship-lock"
MAX_WAIT=60  # seconds
WAITED=0

while ! mkdir "$LOCK_DIR" 2>/dev/null; do
  # Check for stale lock (>5 minutes old)
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

# Write lock info
echo '{"runId":"'$RUN_ID'","ts":'$(date +%s)'}' > "$LOCK_DIR/info.json"
```

**All git operations below happen inside the lock. Release the lock at the end of Phase 4:**
```bash
rm -rf "$LOCK_DIR"
```

The ship mechanism depends on `projectContext.shipMechanism` (set during initialization step 3):

#### Ship by domain (`projectContext.shipMechanism`)

**`git` (default — coding domain):**

1. **Verify all commits are clean:**
   ```bash
   git status
   git log --oneline -<N>  # verify N commits from this cycle
   ```

2. **Commit any uncommitted changes:**
   ```bash
   git add <changed files>
   git commit -m "<type>: <description>"
   ```

3. **Rebase and push to remote:**
   ```bash
   git pull --rebase origin main  # incorporate other parallel runs' changes
   # If rebase conflicts → abort rebase, release lock, return to Builder for conflict resolution
   git push origin <branch>
   ```
   The cycle is not complete until code is pushed.

4. **Publish plugin update:**
   ```bash
   ./publish.sh
   ```
   Syncs the local plugin cache and registry. **Mandatory after every push.**

**`file-save` (writing, research domains):**

1. **Verify changes are saved:** All modified files exist and are non-empty.
2. **Create backup:** Copy changed files to `.evolve/history/cycle-{N}/output/` as a restore point.
3. **Log ship event** in the ledger (no git operations needed):
   ```json
   {"ts":"<ISO-8601>","cycle":<N>,"runId":"<$RUN_ID>","role":"orchestrator","type":"ship","data":{"mechanism":"file-save","files":["<list>"]}}
   ```
4. **Skip publish.sh** — plugin publishing is coding-domain only.

**`export` (design domain):**

1. **Export artifacts** from source files (e.g., SVG export, asset compilation).
2. **Save to output directory:** `.evolve/history/cycle-{N}/exports/`
3. **Log ship event** in the ledger.
4. **Skip publish.sh.**

**`custom`:** Read ship commands from `.evolve/domain.json` `shipCommands` array and execute each in order. Fall back to `file-save` if `shipCommands` is not defined.

5. **Clear non-persistent mailbox messages:**
   Remove rows from `$WORKSPACE_PATH/agent-mailbox.md` where `persistent` is `false`. Retain rows where `persistent` is `true` so cross-cycle warnings survive into the next cycle.
   ```bash
   # Filter in-place: keep header rows and persistent=true rows
   grep -v "| false |" $WORKSPACE_PATH/agent-mailbox.md > /tmp/mailbox-tmp.md && mv /tmp/mailbox-tmp.md $WORKSPACE_PATH/agent-mailbox.md
   ```

6. **Update state.json** (using OCC protocol — see [memory-protocol.md](skills/evolve-loop/memory-protocol.md) Concurrency Protocol):
   - Mark completed tasks in `evaluatedTasks`
   - Update `lastCycleNumber` to current cycle number (use MAX with existing value — another run may have advanced it)
   - Reset `stagnation.nothingToDoCount` to 0
   - Update `lastUpdated`
   - Increment `version`
   - **Compute `fitnessScore`** — weighted average of processRewards dimensions as a single "did the project get better?" signal:
     ```json
     "fitnessScore": round(0.25 * discover + 0.30 * build + 0.20 * audit + 0.15 * ship + 0.10 * learn, 2)
     ```
     After computing, compare to previous cycle's fitnessScore in state.json:
     - If fitnessScore decreased for 2 consecutive cycles → set `fitnessRegression: true` in state.json. The Operator reads this as a HALT-worthy signal.
     - If fitnessScore increased or held steady → set `fitnessRegression: false`.
     - Store the score: `"fitnessScore": <value>` and `"fitnessHistory": [<last 3 scores>]` in state.json.

   - **Compute `ledgerSummary`** from ledger.jsonl (aggregated stats so agents never read the full ledger):
     ```json
     "ledgerSummary": {
       "totalEntries": <count>,
       "cycleRange": [<first>, <last>],
       "scoutRuns": <count>,
       "builderRuns": <count>,
       "totalTasksShipped": <sum of tasksShipped across evalHistory>,
       "totalTasksFailed": <sum of failed>,
       "avgTasksPerCycle": <shipped / cycles>
     }
     ```
   - **Trim `evalHistory`** in state.json to keep only the last 5 entries (older data is captured by `ledgerSummary`)
   - Record **process rewards** for each phase this cycle (step-level scoring):
     ```json
     {
       "processRewards": {
         "discovery": <0.0-1.0>,
         "build": <0.0-1.0>,
         "audit": <0.0-1.0>,
         "ship": <0.0-1.0>,
         "learn": <0.0-1.0>,
         "skillEfficiency": <0.0-1.0>
       }
     }
     ```
     **Scoring rubric** — compute each dimension deterministically:

     | Phase | Score = 1.0 | Score = 0.5 | Score = 0.0 |
     |-------|-------------|-------------|-------------|
     | **discover** | All selected tasks shipped | 50%+ tasks shipped | <50% tasks shipped |
     | **build** | All tasks pass audit first attempt | Some tasks need retry | 3+ audit failures |
     | **audit** | No false positives, all evals run | 1 false positive or missing eval | Multiple false positives |
     | **ship** | Clean commit, no post-commit fixes | Minor fixup needed | Failed to push or dirty state |
     | **learn** | Instincts extracted AND at least one instinct cited in scout-report or build-report `instinctsApplied` | Instincts extracted but none cited this cycle | No instincts extracted |
     | **skillEfficiency** | Total skill+agent tokens decreased from `skillMetrics` baseline | Tokens stable (±5% of baseline) | Tokens increased from baseline |

     Process rewards feed into meta-cycle reviews for targeted agent improvement. A consistently low discovery score means the Scout needs attention, not the Builder. A low skillEfficiency score signals prompt bloat that should be addressed.

   - **Update `processRewardsHistory`** — append this cycle's scores to the rolling array, trimming to keep only the last 3 entries:
     ```json
     "processRewardsHistory": [
       {"cycle": <N-2>, ...scores...},
       {"cycle": <N-1>, ...scores...},
       {"cycle": <N>, "discover": <score>, "build": <score>, "audit": <score>, "ship": <score>, "learn": <score>, "skillEfficiency": <score>}
     ]
     ```

   - **Per-cycle remediation check** (self-improvement trigger):
     After computing process rewards, check `processRewardsHistory` for sustained low scores:
     - If any dimension scores below 0.7 for 2+ consecutive entries in the history → append a remediation entry to `state.json.pendingImprovements`:
       ```json
       {"dimension": "<dim>", "score": <latest>, "sustained": true, "suggestedTask": "<what to fix>", "cycle": <N>, "priority": "high"}
       ```
     - Suggested task mapping:
       - `discover < 0.7` → "improve Scout task sizing or relevance"
       - `build < 0.7` → "add Builder guidance or simplify task complexity"
       - `audit < 0.7` → "review eval grader quality and coverage"
       - `ship < 0.7` → "fix commit workflow or git state issues"
       - `learn < 0.7` → "extract instincts from recent successful cycles"
       - `skillEfficiency < 0.7` → "reduce prompt overhead in skill/agent files"
     - Clear resolved entries: if a dimension's score rises above 0.7 for 2 consecutive cycles, remove its pendingImprovements entry

   - Add eval results to `evalHistory` with **delta metrics**:
     ```json
     {
       "cycle": <N>,
       "verdict": "PASS|WARN|FAIL",
       "checks": <total>,
       "passed": <passed>,
       "failed": <failed>,
       "delta": {
         "tasksShipped": <count>,
         "tasksAttempted": <count>,
         "auditIterations": <average iterations per task>,
         "successRate": <shipped / attempted>,
         "instinctsExtracted": <count this cycle>,
         "stagnationPatterns": <active patterns count>
       }
     }
     ```
   - The `delta` object enables trend analysis across cycles. The Operator and meta-cycle review use these metrics to detect improvement or degradation.
   - **Update mastery level:**
     - If `delta.successRate === 1.0` → increment `mastery.consecutiveSuccesses`
     - If `mastery.consecutiveSuccesses >= 3` and level is not `proficient` → advance level, reset counter
     - If `delta.successRate < 0.5` for 2 consecutive cycles → regress level, reset counter

7. **Release ship lock:**
   ```bash
   rm -rf "$LOCK_DIR"
   ```
