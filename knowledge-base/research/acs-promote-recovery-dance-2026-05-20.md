# ACS Promote Recovery Dance — Cycles 94-98 (2026-05-20)

**Status:** Open (eliminated only when watchdog post-memo SIGTERM is structurally fixed)
**Severity:** LOW-MEDIUM (predictable operator overhead; ~$1 + 2-3 min per occurrence)
**Functional impact:** Zero — `acs/cycle-N/*.sh` predicates were already shipped in the cycle's feat-commit; the rename to `regression-suite/` is a follow-up housekeeping step that just needs committing.
**Structural impact:** Reveals that the orchestrator's `promote-acs-to-regression.sh` step is scheduled inside the watchdog window. If watchdog SIGTERMs, the rename happens on disk but doesn't get committed.

## 1. What happened

Across the v10.17.0 batch (cycles 94-98), every cycle's feat-commit landed on `main` BEFORE the post-memo watchdog SIGTERM. Each cycle's orchestrator then attempted to run `promote-acs-to-regression.sh` (which moves ACS predicates from `acs/cycle-N/*.sh` to `acs/regression-suite/cycle-N/*.sh`) — but the watchdog SIGTERM fired during that step or shortly after, before the orchestrator could commit the rename.

Result: 5 separate manual recovery commits were required, one per cycle:

| Cycle | Feat commit | Promote chore commit | Rename count |
|---|---|---|---|
| 94 | `d24b403` | `89f2d08` | 5 files |
| 95 | `392b064` | `fb938bf` | 2 files |
| 96 | `1f40061` | `2af50aa` | 3 files |
| 97 | `a10ca24` | `3dbde30` | 5 files |
| 98 | `6466a3a` | `6461884` | 5 files |

Each recovery commit was a clean 100% rename (0 insertions, 0 deletions), captured by `git status` as:
```
 D acs/cycle-N/predicate-name.sh
?? acs/regression-suite/cycle-N/
```

The operator dance for each was:
```bash
git add acs/cycle-N/ acs/regression-suite/cycle-N/
EVOLVE_SHIP_AUTO_CONFIRM=1 bash scripts/lifecycle/ship.sh --class manual \
  "chore(cycle-N): promote ACS predicates to regression-suite"
```

This worked but added 5 commits to the v10.17.0 batch history — one per cycle's promote step.

## 2. Research

### Why the orchestrator can't commit the rename in-flight

The orchestrator profile (`.evolve/profiles/orchestrator.json`) explicitly DISALLOWS direct `git commit` and `git push` calls (visible in the trust-kernel `--disallowedTools` flag). The orchestrator must invoke `scripts/lifecycle/ship.sh` for any commit, which is in `--allowedTools`. So:

1. Orchestrator calls `promote-acs-to-regression.sh` → file moves happen on disk
2. Orchestrator should then invoke `ship.sh --class trivial` or similar to commit the rename
3. But this step happens during the post-memo finalization window, which is the long-idle window the watchdog SIGTERMs
4. SIGTERM fires before step 2 completes
5. The dispatcher's checkpoint writes `cycle-state.json` with the partial state
6. On --resume, the dispatcher tries to continue the cycle, but the role-gate blocks the next operations because `cycle-state.json` still indicates phase=learn

The role-gate (`scripts/guards/role-gate.sh`) refuses Edit/Write to scripts/ paths when cycle-state.json indicates an active learn-phase, even for operator commands run from the outer Claude Code session. So the operator workaround is:
1. Commit the promote via `ship.sh --class manual`
2. Clear cycle-state.json via `cycle-state.sh clear`
3. Remove the cycle worktree
4. Re-dispatch any remaining cycles

### Why ship.sh --class manual works for this

`ship.sh --class manual` is the operator-only commit path. It skips audit-binding verification (which would fail because the audit was for a different commit) and just commits + pushes whatever's staged. Per memory `feedback_ship_bypass_var.md`: "Non-cycle commits use EVOLVE_BYPASS_SHIP_VERIFY (not _GATE) and must go through scripts/ship.sh". For these promote commits, `--class manual` is the documented correct path.

### What promote-acs-to-regression.sh does

The script moves files from `acs/cycle-N/predicate.sh` to `acs/regression-suite/cycle-N/predicate.sh`. Per the trust-kernel model:
- `acs/cycle-N/` is the "active" predicates for cycle N (verified during audit phase)
- `acs/regression-suite/cycle-N/` is the archived suite (run on future cycles to verify the cycle-N change didn't regress)

The move is functionally a `git mv`, which git detects as a 100% rename (similarity_index=100, status R100). The rename has no content change — only the path.

## 3. Reasoning

The orchestrator's promote step is correctly scheduled (after ship-commit, before cycle close), but it's not granular enough to survive a watchdog SIGTERM. Two structural fixes possible:

1. **Move promote BEFORE the watchdog stall window.** Run `promote-acs-to-regression.sh` immediately after ship.sh, in the same orchestrator turn. This shrinks the gap between the feat-commit and the promote-commit to seconds rather than the 15-22 min finalization window.

2. **Make promote part of the feat-commit itself.** Have ship.sh include the promote as a single combined commit ("feat(cycle-N): X + promote ACS predicates"). This eliminates the chore commit entirely.

Either fix eliminates the recovery dance. Option 2 is cleaner (fewer commits) but changes the commit-message convention. Option 1 is the minimum-change fix.

The dance itself is benign — operator overhead is ~2-3 min and ~$1 per occurrence — but it pollutes the git history with chore commits that don't add information. At 5 cycles per batch, this is 5 extra commits.

## 4. Fix

### Short-term (operator runbook, currently in use)

For each cycle that SIGTERMs post-memo (currently 100% of cycles per [`watchdog-post-memo-sigterm-pattern-2026-05-20.md`](watchdog-post-memo-sigterm-pattern-2026-05-20.md)):

```bash
# After the SIGTERM:
git status --short  # confirm acs/cycle-N/*.sh deleted, acs/regression-suite/cycle-N/ untracked
git add acs/cycle-N/ acs/regression-suite/cycle-N/
EVOLVE_SHIP_AUTO_CONFIRM=1 bash scripts/lifecycle/ship.sh --class manual \
  "chore(cycle-N): promote ACS predicates to regression-suite"
bash scripts/lifecycle/cycle-state.sh clear
git worktree remove --force /private/var/folders/...  # the cycle's worktree
git branch -D evolve/cycle-N
```

### Long-term (proposed, not yet shipped)

Move the promote step to ship.sh's post-commit hook. Concretely:
- `ship.sh` after the feat-commit landing on main, before declaring DONE, runs `promote-acs-to-regression.sh` and creates a single `chore: promote` commit
- This shrinks the gap between feat and chore commits to <10s, well within any watchdog threshold

Alternative: combine into the feat-commit ("feat(cycle-N): X + ACS predicates promoted to regression-suite"). Cleaner but requires changing the orchestrator's commit-message template.

## 5. Lessons

- **`feedback_ship_bypass_var.md`** (memory) — confirms `ship.sh --class manual` is the correct path for these chore commits
- **[[watchdog-post-memo-sigterm-pattern-2026-05-20]]** — root cause; this dossier is a downstream symptom
- **[[cycle-94-retry-fast-fail-pattern]]** through **[[cycle-98-psmas-phase-skip-foundation]]** — all 5 cycles exhibit this pattern; the lesson schema does not capture it because each cycle's feat-commit was successful

## 6. References

- 5 chore-promote commits: `89f2d08`, `fb938bf`, `2af50aa`, `3dbde30`, `6461884`
- Script: `scripts/lifecycle/ship.sh` (--class manual mode)
- Script: `scripts/lifecycle/cycle-state.sh clear` (recovery step)
- Script: `scripts/dispatch/promote-acs-to-regression.sh` (the operation that needs to land in a commit)
- Cross-references:
  - [`watchdog-post-memo-sigterm-pattern-2026-05-20.md`](watchdog-post-memo-sigterm-pattern-2026-05-20.md)
  - [`dual-root-plugin-pattern-bite-2026-05-20.md`](dual-root-plugin-pattern-bite-2026-05-20.md)
