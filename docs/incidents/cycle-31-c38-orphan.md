# Incident Report: Cycle-31 c38 Ship-Integrity Breach — Orphaned Commit

**Date:** 2026-05-13 | **Severity:** HIGH | **Status:** Documented; recovery pending

---

## Part 1: What Happened

### Timeline

| Time (HKT) | Event |
|---|---|
| ~10:56 | Cycle-31 attempt 1 starts. Goal: `c38-inbox-audit-and-collision`. |
| 11:25:52 | Builder commits `332ac9d` to worktree branch `evolve/cycle-31` — 4 files, 339 insertions. |
| ~11:25 | Operator injects c40–c44 inbox JSON files into `.evolve/inbox/` (between audit and ship). |
| ~11:26 | Auditor verifies worktree (entry_seq 899). Verdict: PASS (0.92 confidence). |
| ~11:31 | `ship.sh --class cycle` runs from project root. |
| ~11:31 | `git merge --ff-only evolve/cycle-31` **blocked**: main working tree has pre-existing uncommitted modifications to `scripts/utility/inject-task.sh` and `scripts/tests/inject-task-test.sh` — the same files `332ac9d` modifies. Git refuses: "Your local changes would be overwritten." |
| ~11:31 | Orchestrator attempts `git stash` to unblock. After stash, ship.sh detects new untracked inbox JSON files → integrity check fails. |
| ~11:31 | Fallback: orchestrator invokes `ship.sh --class manual` (equivalent to `EVOLVE_BYPASS_SHIP_VERIFY=1`). |
| ~11:31 | Fallback `git add -A` runs in **project root** (not worktree). At this point project root contains: inbox JSONs (untracked/staged) but NOT the c38 code (exists only in the worktree commit `332ac9d`). |
| 11:31:41 | Commit `55d1dc5` created with **11 inbox JSON files only**. Commit subject fraudulently claims "4 files ~155 LoC" c38 deliverables. |
| ~11:37 | Orchestrator reports `FAILED-AND-LEARNED`. `state.json:lastCycleNumber` remains `30`. |
| ~11:39 | Cycle-31 attempt 2 starts for investigation. |
| ~11:39 | Triage Step-0a runs `git log main --oneline | grep c38-inbox-audit-and-collision` → matches `55d1dc5` subject → **FALSE `skip_shipped`**. Recursive bug: the investigation cycle is itself affected by the keyword-match flaw it needs to fix. |
| ~11:39 | Attempt 2 selects `c40-ghosh-research-dossier` instead (c38 wrongly believed shipped). |
| ~11:40 | Auditor finds FAIL: C1 = fraudulent commit `55d1dc5`, C2 = `lastCycleNumber=30`. Cycle refuses to ship c40 while breach is live. |
| ~12:26 | Cycle-31 attempt 3 starts. Intent persona correctly identifies c38-investigation as the cycle goal. This incident report is the deliverable. |

### Net State After Breach

- `HEAD` on `main`: commit `55d1dc5` — 11 inbox JSON files, zero c38 code files.
- Working tree: `M scripts/utility/inject-task.sh` and `M scripts/tests/inject-task-test.sh` — **partial** c38 changes (Tests 12–13 only; `--injected-by` flag; missing Tests 14–17, `--force`, git-log collision).
- `scripts/utility/inbox-audit.sh` → **ABSENT** from project root.
- `scripts/utility/inbox-reconcile.sh` → **ABSENT** from project root.
- `state.json:lastCycleNumber` = `30` (never advanced).
- Orphaned good commit `332ac9d` **intact** in git object store — dangling, unreachable from main.

---

## Part 2: Research — Evidence

### Commit `55d1dc5` (fraudulent) — `git show --stat`

```
commit 55d1dc52abfcdb3efbc613516da24325c1413f3b
Author: Mickey Yaya <mickeyyaya@gmail.com>
Date:   Wed May 13 11:31:41 2026 +0800

    feat: cycle 31 — c38-inbox-audit-and-collision: inbox-audit.sh NEW + ...

 .evolve/inbox/2026-05-13T00-18-59Z-9b7e7f9b.json | 1 +
 .evolve/inbox/2026-05-13T00-19-06Z-d31e4217.json | 1 +
 .evolve/inbox/2026-05-13T00-19-14Z-4d49558b.json | 1 +
 .evolve/inbox/2026-05-13T02-23-31Z-3f00e19d.json | 1 +
 .evolve/inbox/2026-05-13T02-23-39Z-526d1d7f.json | 1 +
 .evolve/inbox/2026-05-13T02-23-46Z-a42d8650.json | 1 +
 .evolve/inbox/2026-05-13T03-27-29Z-b8797f1f.json | 1 +
 .evolve/inbox/2026-05-13T03-27-42Z-5e9907d9.json | 1 +
 .evolve/inbox/2026-05-13T03-28-54Z-d591e5d.json  | 1 +
 .evolve/inbox/2026-05-13T03-29-27Z-28cb2938.json | 1 +
 .evolve/inbox/2026-05-13T03-29-34Z-49240944.json | 1 +
 11 files changed, 11 insertions(+)
```

**Zero script files.** The commit body's "Actual diff" section is generated from `build-report.md` content (not from `git show --stat` of the actual commit), so it erroneously lists 4 script files.

### Orphaned commit `332ac9d` (good) — `git show --stat`

```
commit 332ac9d89aee018de2eae76d25a2876514e821a9
Author: Mickey Yaya <mickeyyaya@gmail.com>
Date:   Wed May 13 11:25:52 2026 +0800

    feat: cycle 31 — c38-inbox-audit-and-collision: inbox-audit.sh NEW + ...

 scripts/tests/inject-task-test.sh  |  71 +++++++++++++++++++++
 scripts/utility/inbox-audit.sh     | 152 +++++++++++++++++++++++++++++++++++++
 scripts/utility/inbox-reconcile.sh |  62 +++++++++++++++++++++
 scripts/utility/inject-task.sh     |  63 +++++++++++++--
 4 files changed, 339 insertions(+), 9 deletions(-)
```

All 4 expected deliverables present and accounted for.

### Ledger entries (key)

| entry_seq | ts (UTC) | cycle | role | exit | sha8 | notes |
|---|---|---|---|---|---|---|
| 898 | 03:21:51Z | 31 | builder | 0 | `aaa8b522` | Attempt-1 Builder; worktree commit `332ac9d` |
| 899 | 03:25:38Z | 31 | auditor | 0 | `a3df6b8e` | PASS (0.92); worktree files verified |
| 900 | 03:37:31Z | 31 | retrospective | 0 | `feeac66b` | `git_head=55d1dc5`; fraudulent commit in main before retro ran |
| 901 | 03:39:37Z | 31 | orchestrator | 0 | `83231cc7` | FAILED-AND-LEARNED; operator action documented |
| 904 | 03:53:29Z | 31 | triage | 0 | `9b032d03` | Attempt-2 Triage; FALSE `skip_shipped[c38]` via subject-match |
| 906 | 04:05:25Z | 31 | auditor | 0 | `efec8fea` | Attempt-2 Audit; FAIL — C1+C2 CRITICAL defects |
| 909 | 04:13:20Z | 31 | orchestrator | 0 | `83231cc7` | Attempt-2 FAILED-AND-LEARNED |

### Build-report artifact

Build-report SHA `aaa8b522` (entry_seq 898): documented 4 files, 36/36 tests passing (17/17 inject-task + 19/19 lifecycle). Fully correct. The build phase itself succeeded; the failure was entirely in the ship phase.

### Archive paths

- Attempt-1 workspace: `.evolve/runs/cycle-31-20260513T033938Z/`
- Attempt-2 workspace: `.evolve/runs/cycle-31-20260513T042648Z/`
- Attempt-3 workspace: `.evolve/runs/cycle-31/` (current)

---

## Part 3: Root-Cause Reasoning

### Three candidate hypotheses (from goal spec)

**Hypothesis (a): Builder wrote to project tree instead of worktree.**

**PARTIAL — pre-existing, not Builder-caused.** The dirty working-tree files (`M inject-task.sh`, `M inject-task-test.sh`) were present before the Builder ran. These are partial c38 changes that existed in the project root prior to cycle-31 attempt 1. Builder correctly wrote to the worktree only. The pre-existing dirty files were the *trigger* for the ff-merge block, not a Builder failure.

**Hypothesis (b): `ship.sh git -C worktree diff` missed untracked-yet-staged files.**

**YES — primary mechanism.** The `git -C worktree diff --cached --quiet` check was TRUE (worktree was clean; Builder had already committed `332ac9d`). But the subsequent `git merge --ff-only evolve/cycle-31` blocked because main's *working tree* had pre-existing uncommitted changes to files also in `332ac9d`. ship.sh had no recovery path that preserved `--class cycle` integrity when the ff-merge blocked. The gap: ship.sh checks the *worktree* index cleanness but not the *destination (main) working tree* cleanness before attempting the merge.

**Hypothesis (c): Audit-bound SHA verification was bypassed.**

**YES — secondary mechanism, completing the cascade.** The `--class manual` fallback explicitly skips audit binding (audit-report SHA + tree-SHA checks). `EVOLVE_BYPASS_SHIP_VERIFY=1` bridges to `--class manual`. This flag is valid for operator commits but fatally wrong as a cycle-ship fallback because the audit-bound tree SHA verification is precisely the mechanism that should have caught the wrong-tree commit.

### The cascade (b) → (c)

```
ff-merge blocked (hypothesis b)
  ↓
ship.sh has no cycle-safe recovery → falls back to --class manual
  ↓
--class manual runs git add -A in project root (not worktree)
  ↓
project root contains: inbox JSONs (untracked) + no c38 code
  ↓
commit 55d1dc5: 11 inbox JSONs, 0 code files
  ↓
--class manual skips audit-bound tree-SHA check (hypothesis c)
  ↓
breach committed with wrong content; retrospective sees fraudulent HEAD
```

### Tertiary root cause: commit body generated from build-report, not git show

The v8.34.0+ commit body "Actual diff" section is templated from `build-report.md:## Files Changed`. It was NOT generated from `git show --stat` of the just-made commit. Had it been generated post-commit from actual git output, the 11-inbox-JSON reality would have been immediately visible in the commit body itself.

---

## Part 4: Fix (Recovery Plan — Do Not Execute This Cycle)

### Option A: Cherry-pick `332ac9d` (recommended — cleaner history)

```bash
# 1. Stash dirty working-tree files to avoid conflicts
git stash

# 2. Cherry-pick the good commit (all 4 files, complete implementation)
git cherry-pick 332ac9d

# 3. Ship via --class manual (operator commit; c38 code is now correct)
EVOLVE_SHIP_AUTO_CONFIRM=1 ship.sh --class manual \
  "feat: cycle 31 — c38: cherry-pick 332ac9d (inbox-audit.sh NEW, inbox-reconcile.sh NEW, inject-task.sh --injected-by+--force+git-log-collision, inject-task-test.sh 17/17 tests)"

# 4. Advance lastCycleNumber to 31 in state.json
# (ship.sh --class cycle would do this automatically;
#  --class manual does not — do it manually or via jq)
jq '.lastCycleNumber = 31' .evolve/state.json > .evolve/state.json.tmp.$$
mv .evolve/state.json.tmp.$$ .evolve/state.json
```

### Option B: Checkout files from `332ac9d` (safer if cherry-pick conflicts)

```bash
# 1. Checkout the 4 c38 files from the good commit
git checkout 332ac9d -- \
    scripts/utility/inbox-audit.sh \
    scripts/utility/inbox-reconcile.sh \
    scripts/utility/inject-task.sh \
    scripts/tests/inject-task-test.sh

# 2. Stage and ship
git add scripts/utility/inbox-audit.sh scripts/utility/inbox-reconcile.sh \
        scripts/utility/inject-task.sh scripts/tests/inject-task-test.sh
EVOLVE_SHIP_AUTO_CONFIRM=1 ship.sh --class manual \
  "feat: cycle 31 — c38: restore orphaned files from commit 332ac9d (4 files, 339 insertions)"

# 3. Advance lastCycleNumber
jq '.lastCycleNumber = 31' .evolve/state.json > .evolve/state.json.tmp.$$
mv .evolve/state.json.tmp.$$ .evolve/state.json
```

**Critical note on working-tree dirty files:** The dirty `M inject-task.sh` and `M inject-task-test.sh` contain only **partial** c38 changes (Tests 12–13; `--injected-by` only; missing Tests 14–17, `--force`, git-log collision check). Do NOT commit these directly — `332ac9d` has the complete implementation. These files will be superseded by Option A or B above.

### After recovery

1. Verify `git show --stat HEAD` intersects `{inbox-audit.sh, inbox-reconcile.sh, inject-task.sh, inject-task-test.sh}`.
2. Start the next cycle for C1 ship-gate tree-SHA binding (HIGH w=0.98).

---

## Part 5: Lessons

### Lesson 1: ship.sh must detect dirty destination before ff-merge

`ship.sh --class cycle` must check `git -C $PROJECT_ROOT diff --quiet HEAD` (or equivalent) **before** attempting `git merge --ff-only`. If main's working tree is dirty for any files in the merge branch, fail with a clear message: "Main working tree has uncommitted changes to files modified by this cycle branch. Stash or commit them before shipping." This prevents the silent ff-merge block that triggered the cascade.

**Target:** C1 task (`user-1778645962-3dea8ccb`); implement in `scripts/lifecycle/ship.sh`.

### Lesson 2: No `--class manual` fallback inside automated cycle ship

`ship.sh --class cycle` must NOT fall back to `--class manual` under any error condition. The audit-bound tree-SHA check is safety-critical for cycle ships. If ff-merge blocks, fail the phase with a non-zero exit code and actionable diagnostic. Let the orchestrator record the failure and invoke the retrospective; do not auto-degrade to an unverified commit path.

**Target:** C1 task; implement in `scripts/lifecycle/ship.sh` and `ship-gate`.

### Lesson 3: Commit body "Actual diff" must be generated from `git show --stat HEAD`, not from build-report

The v8.34.0+ commit body templates its "Files modified" section from `build-report.md:## Files Changed`. This means a miscommit can carry a truthful-looking body that contradicts the actual tree. Ship.sh should generate the commit body's diff section by running `git show --stat HEAD` **after** the commit, and embed the actual output — not the pre-commit build-report manifest.

**Target:** C1 task or follow-up; implement in `scripts/lifecycle/ship.sh`.

### Lesson 4: Triage Step-0a must content-verify, not keyword-match

The c37 mechanism `git log main --oneline | grep task_id` is insufficient. A commit subject can claim a task_id even when the commit tree contains none of the task's expected files (as proven by `55d1dc5`). The algorithm must:

1. Find candidate commits by subject keyword (current behavior).
2. For each candidate, intersect `git show --stat <sha>` file list against the task's expected deliverables.
3. Return `SHIPPED` only if the intersection is non-empty.
4. Return `INTEGRITY_BREACH` (not `NOT_SHIPPED`) if candidates exist but have zero intersection — escalate to operator rather than silently proceeding.

**Pseudocode** (for c37 hardening task):

```
function verify_shipped(task_id, expected_files[]):
    candidates = git log main --oneline | grep task_id | awk '{print $1}'
    if candidates is empty: return NOT_SHIPPED

    for sha in candidates:
        stat_files = git show --stat sha | grep -E '\|' | awk '{print $1}'
        intersection = stat_files ∩ expected_files
        if intersection is not empty: return SHIPPED(sha, matched_files=intersection)

    return INTEGRITY_BREACH(candidates=candidates, missing=expected_files)
```

Expected files for c38: `[scripts/utility/inbox-audit.sh, scripts/utility/inbox-reconcile.sh]`. These are NEW file deliverables extractable from the inbox JSON `action` field (grep for filename patterns with `.sh` extension).

**Target:** c37 hardening (to be scoped in a future cycle after C1 ships).

### Lesson 5: Operator inbox injections between audit and ship require a checkpoint

The operator injected c40–c44 inbox JSONs between the Auditor completing and ship.sh running. These untracked files perturbed the project root state at exactly the moment ship.sh needed it clean. The pipeline should either: (a) take a snapshot of the project root at audit time and verify nothing changed before ship, or (b) explicitly ignore `.evolve/inbox/` additions when evaluating project-root cleanliness. Either way, new untracked inbox files should not block a cycle ship.

**Target:** ship.sh or a new pre-ship check script (future cycle).

---

## Part 6: References

### Commits

| SHA | Description | Status |
|---|---|---|
| `332ac9d` | Correct c38 commit (4 files, 339 insertions, 36/36 tests) | Dangling — not in main |
| `55d1dc5` | Fraudulent commit in main (11 inbox JSONs, 0 code files) | In main — must be superseded |

### Ledger entries

Ledger file: `.evolve/ledger.jsonl`

| entry_seq | key evidence |
|---|---|
| 898 | Attempt-1 Builder exit 0; worktree `332ac9d` |
| 899 | Attempt-1 Auditor PASS (0.92 confidence); artifact SHA `a3df6b8e` |
| 900 | Retrospective sees `git_head=55d1dc5`; confirms fraudulent commit was in main before retro |
| 901 | Attempt-1 Orchestrator FAILED-AND-LEARNED |
| 904 | Attempt-2 Triage FALSE `skip_shipped[c38]` via keyword match |
| 906 | Attempt-2 Auditor FAIL (C1: fraudulent commit, C2: `lastCycleNumber=30`) |
| 909 | Attempt-2 Orchestrator FAILED-AND-LEARNED |

### Workspace archives

| Path | Attempt | Key artifact |
|---|---|---|
| `.evolve/runs/cycle-31-20260513T033938Z/` | 1 | build-report (SHA `aaa8b522`), audit-report (SHA `a3df6b8e`), orchestrator-report (SHA `83231cc7`) |
| `.evolve/runs/cycle-31-20260513T042648Z/` | 2 | triage-decision, audit-report (SHA `efec8fea`), orchestrator-report (SHA `83231cc7`) |
| `.evolve/runs/cycle-31/` | 3 (current) | This incident report |

### Related tasks (inbox)

| task_id | file | status |
|---|---|---|
| `user-1778644766-fcf0e86d` | `2026-05-13T03-59-26Z-fcf0e86d.json` | c38-investigation — this cycle |
| `user-1778645962-3dea8ccb` | — | C1 ship-gate tree-SHA binding — next cycle |
| `c38-inbox-audit-and-collision` | `2026-05-13T02-23-39Z-526d1d7f.json` | NOT shipped; stays in inbox pending recovery + C1 fix |
