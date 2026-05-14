# Incident: Cycle-46 Ship Refused (abnormal-ship-refused-c46)

**Date:** 2026-05-14  
**Severity:** HIGH  
**Status:** Root cause fixed in cycle-47 (ship.sh counter-advance ordering)

## What Happened

During cycle-46, `ship.sh` pushed the commit successfully to `origin/main`, then fired `integrity_fail` with:

```
INTEGRITY BREACH: audit-bound tree SHA f2afae6e244d037485f66cc01851fdd23e11e1b3`
  != committed tree SHA 760b566a1de9e8d44a49f529021108e52a53f65f
```

The script exited rc=2. Because `lastCycleNumber` advance runs AFTER the integrity check in the pre-cycle-47 code, `state.json:lastCycleNumber` remained at 45 despite the cycle-46 commit landing on main. The abnormal event `ship-refused` was captured in `abnormal-events.jsonl` and promoted to HIGH carryoverTodo `abnormal-ship-refused-c46` (cycles_unpicked=2 before resolution).

## Root Cause

Two bugs compounded:

1. **Backtick corruption** (fixed in e6926af): `AUDIT_BOUND_TREE_SHA` was extracted from `audit-report.md` using a pattern that preserved surrounding backtick characters. The audit-report stored the value as `` `f2afae6e244d037485f66cc01851fdd23e11e1b3` `` (backtick-wrapped markdown code), so `AUDIT_BOUND_TREE_SHA` became `f2afae6e244d037485f66cc01851fdd23e11e1b3`` ` (trailing backtick). The comparison `f2afae6e...`` != 760b566a...` always failed.

2. **Counter-advance ordering** (fixed in cycle-47 T1): `lastCycleNumber` advance was positioned AFTER the post-push integrity check in the worktree ship path. When integrity_fail fires (rc=2), the advance block is never reached. The commit is already on remote, but the counter does not reflect it.

Bug 1 was the proximate cause of the failed integrity check. Bug 2 meant that even if the integrity check had fired for any other reason after a successful push, the counter would silently stay behind.

## Fix

**Bug 1 (backtick, e6926af):** Added `tr -d "[:space:]\`"` to the `AUDIT_BOUND_TREE_SHA` extraction in ship.sh so trailing/surrounding backticks are stripped.

**Bug 2 (ordering, cycle-47 T1):** Moved `lastCycleNumber` advance to immediately after `git push origin` succeeds in the worktree ship path, before the C1 tree-SHA binding verification. The counter now tracks "push succeeded", not "integrity verified".

## Connection

Both bugs share the same ship.sh worktree path. Bug 1 triggered integrity_fail; Bug 2 meant the counter didn't advance when integrity_fail fired. Neither bug alone would have been visible without the other in this specific cycle — the backtick bug caused the failure, and the ordering bug amplified the impact by leaving the counter stuck.

## Verification

ACS predicate `acs/cycle-47/009-lastcycle-counter-advance.sh` verifies the ordering fix: confirms the pre-integrity-check advance log message exists in ship.sh and that its line number precedes the `INTEGRITY BREACH` integrity_fail line.
