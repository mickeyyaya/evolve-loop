# 2026-08-24 — Wave-2 zero-ship: triage bookkeeping defeats in-commit consumption (cycle-1552→1553)

## Symptom

soak-20260824a wave 2 (cycles 1553-1555): 0 ships, 3 FAILs, consecutive-failures
halt. Cycle-1553's lane was assigned
`premise-challenge-fail-never-reaches-failure-learning` — an item cycle-1552
had ALREADY implemented and shipped (df322f6c, PASS). premise-challenge
correctly BLOCKed the duplicate, build delivered nothing, audit correctly
FAILed the zero-delivery. A full lane burned re-proving finished work because
the finished work's landing did not retire its item.

## Root cause

`committedInboxIDs` (ship consumption + postship promotion) resolved ids from
triage-decision.json `top_n`/`skip_shipped` ONLY, with `lane-scope.json` as a
nil-body fallback. Cycle-1552's triage put its assigned id in `dropped[]`
("already-shipped: PR #479") with `top_n: []` — build shipped the item's real
implementation anyway — so consumption resolved ZERO ids and the tracked item
stayed pickable. Second recorded shape of one class: cycle-1515's triage
DECOMPOSED its assigned id into sub-ids `top_n` named instead (item
`consumption-id-linkage-lane-scope-union`, filed 2026-08-18, unfixed until
this burn).

## Fix

`committedInboxIDs` merges the lane-scope ids under a per-id rule that keeps
every previously-pinned contract:

The union applies ONLY to a PASS landing (a WARN ship resolves exactly the
pre-union set — partial work stays pickable), and per-id:

| triage's word on the assigned id | consumption |
|---|---|
| deferred | stays pickable — beats every other arm (postponed wholesale; remainder rides carryover) |
| dropped with a CLOSE-CLASS reason (already-shipped / duplicate / superseded / stale / obsolete …) | consumes — an affirmative close; the carryover twin is already retired on the same signal |
| dropped with any OTHER reason (requires-split, out-of-scope, unknown) | stays pickable — the persona routes VALID work into dropped[]; forgetting a live todo is worse than carrying a stale one |
| unmentioned, and triage engaged the scope BY NAME (some scope id committed) | stays pickable — lane scopes are multi-item menus; an unworked menu mate remains dispatchable backlog |
| unmentioned, decision committed work naming NO scope id (the cycle-1515 rename/decomposition shape) | consumes — the committed work is the scope's work under another name |
| unmentioned, decision committed NOTHING | stays open — the declined-menu contract (`TestPromoteInbox_EmptyCommittedDeclinedMenuStaysOpen` and the consume-site twin, both untouched and green) |

`inboxmover.DeferredIDs`/`ClosedDroppedIDs` parse the `id` key only — the same
key the core sibling reader uses — with cross-references in both files so the
two readers of `dropped[]` cannot silently diverge.

## Wave-2 verification results (the halt was NOT a fixed-class recurrence)

- Splice fix (#494) PROVEN live: bug-reproduction executed pre-build
  (`repro@4 build@6`, cycle-1553) — the cycle-1550 mis-slot is gone.
- Persona fix (#493): zero `load agent` lane kills.
- 1554/1555: legitimate hard-task failures — both lanes wrote correct
  red-first tests (1554: the cycle-1550 bridge stale-artifact defect; 1555:
  the red-first ship-gate) and exhausted correction budgets before greening
  them; the build handoff floor correctly kept both red tests off main.
- The escalation's canned "verdict-surface forged a negative verdict"
  hypothesis was wrong for ALL THREE cycles — every gate verdict was correct.
- Per-CLI dispatch share wave 2: claude 12 / codex 11 (~50/50).

## Regression pins

`internal/phases/ship/consume_lanescope_union_test.go` (5 unit pins incl. the
sharpened multi-id deferred shape that two initial mutants exposed as
redundant), `consume_integration_test.go` cycle-1552 end-to-end replay,
existing declined-menu pins at both sites untouched. Mutation-tested: 6/6
killed.
