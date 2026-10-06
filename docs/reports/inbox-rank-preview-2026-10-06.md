# Inbox rank preview: today's lane order against the computed rank (2026-10-06)

> **What this is.** The operator's review artifact for P1 of [inbox prioritization](../plans/inbox-prioritization-2026-10.md) ([ADR-0121](../architecture/adr/0121-inbox-priority-is-a-computed-rank.md)). P1 is unwired: nothing the loop does changes until P2. This page shows what P2 would change for the lane-ready list, so the class order and the factor weights can be judged on real items before they drive dispatch.
>
> **Regenerated after the operator's correction (2026-10-06).** The first version of this page ranked with security first (the plan's original pipeline-health-first order). The operator corrected the class order the same day to correctness > stability > performance > debuggability > feature > maintainability > hygiene > security, with security last; every figure below uses the corrected order.

## How it was produced

| Input | Value |
|---|---|
| Inbox | the tracked `.evolve/inbox` root at `origin/main` 6490463a8: 241 pending items, of which 55 are lane-ready, 183 console-owned and 3 waiting on a dependency |
| Policy | the checked-in `.evolve/policy.json` `inbox_priority` block, equal to the compiled default: class order correctness > stability > performance > debuggability > feature > maintainability > hygiene > security; factors base 0.45, class 0.20, unblocks 0.15, recurrence 0.10, age 0.05, goal 0.05; half-life 30 days; unblocks cap 3; recurrence cap 5; no active campaign |
| Recurrence | 0 for every item: the recurrence ledger is runtime state (`.evolve/recurrence-ledger.json`) and is not in the tracked tree, so the preview reads none. On the plane the factor applies to items whose id is a ledger pattern or a pattern's `fix_item_id`. |
| As of | 2026-10-06T04:56:20Z |
| Today's order | `triagecap.RankForDispatch`, the order every lane consumer uses today: weight descending, then a declared fix surface first, then file-name order (`ReadInboxBacklog` reads the root sorted by path) |
| New order | `inboxrank.Order` on the same ready list (`evolve inbox rank --list ready`) |

Reproduce it from a checkout of that commit with the P1 binary:

```sh
EVOLVE_PROJECT_ROOT=$PWD ./go/bin/evolve inbox rank --list all --json > rank-all.json
jq '[.lists[] | select(.list=="ready") | .items[]] as $new
 | ($new | sort_by([-.weight, (if .declared then 0 else 1 end), .path]) | to_entries
    | map({key: .value.id, value: (.key+1)}) | from_entries) as $old
 | $new | map(. + {old_rank: $old[.id], delta: ($old[.id] - .rank)})' rank-all.json
```

The `jq` step only re-sorts the verb's own rows by today's keys (`weight`, `declared` and `path` are in every row); `evolve inbox rank --explain <id>` prints any item's factor breakdown.

## Summary

- **52 of 55 ready items move** (23 up, 29 down). `cli-boundary-run` keeps the top place; `cli-carryover-list` and `cli-doctor-versions` keep theirs.
- **The top ten change class.** Today's top ten are eight hygiene items and two stability items. The new top ten are nine stability items and one debuggability item. Correctness, the first class, has no lane-ready item; its one item is console-owned.
- **The one security item falls.** `cli-scan-secrets` (security, weight 0.35) moves from 35th to 52nd: security is now the last class, so its class feature is 1/8, below hygiene's 2/8.
- **Ties spread.** The ready list carries 14 distinct weights (the largest tie is 9 items) and 40 distinct scores at four decimals. Over the whole inbox, 37 distinct weights become 214 distinct scores; the 33 items tied at weight 0.5 get 31 distinct scores, and the 21 at 0.4 get 18. What still ties is identical in every fact (same weight, class and filing day) and falls to the declared-surface, filing-time and id tie-breaks.
- **No unknown class.** Every one of the 241 items carries one of the eight classes, so no item falls to the bottom for its class and `evolve inbox rank` printed no class warning.
- **Unblocks is rare today.** Five items have a queued dependent; three of them are lane-ready (`cli-dossier-publish`, `cli-phase-cycle-request`, `dossier-sweep-pairing`), each unblocking one item.

## Top 30, new order

Delta is today's rank minus the new rank: positive moved up.

| New | Today | Delta | Score | Weight | Class | Item |
|---:|---:|---:|---:|---:|---|---|
| 1 | 1 | 0 | 0.4512 | 0.6 | stability | `cli-boundary-run` |
| 2 | 24 | +22 | 0.4337 | 0.45 | stability | `cli-dossier-publish` |
| 3 | 5 | +2 | 0.4285 | 0.55 | stability | `overlay-rule-tier-selectors-canonicalize` |
| 4 | 23 | +19 | 0.4062 | 0.5 | stability | `repo-contract-test-level-selection` |
| 5 | 25 | +20 | 0.3837 | 0.45 | debuggability | `cli-phase-cycle-request` |
| 6 | 26 | +20 | 0.3837 | 0.45 | stability | `cli-land-patch` |
| 7 | 28 | +21 | 0.3835 | 0.45 | stability | `acs-cycle1723-package-doc-predicate-stale-after-753` |
| 8 | 30 | +22 | 0.3612 | 0.4 | stability | `releasepipeline-journal-errors-and-classify-pin` |
| 9 | 31 | +22 | 0.361 | 0.4 | stability | `gittest-ebadf-retry-class` |
| 10 | 34 | +24 | 0.3387 | 0.35 | stability | `cli-failures` |
| 11 | 38 | +27 | 0.3387 | 0.35 | stability | `releasepreflight-fail-open-pins` |
| 12 | 39 | +27 | 0.3387 | 0.35 | stability | `rollback-fail-open-and-vacuous-tests` |
| 13 | 16 | +3 | 0.335 | 0.5 | hygiene | `dossier-sweep-pairing` |
| 14 | 27 | +13 | 0.3337 | 0.45 | debuggability | `inbox-loader-truncation-reads-as-skipped` |
| 15 | 7 | -8 | 0.3203 | 0.54 | hygiene | `premium-rung-placement-law-and-narrative-dedup` |
| 16 | 9 | -7 | 0.3203 | 0.52 | hygiene | `intent-delta-gate-seam-threading` |
| 17 | 44 | +27 | 0.3162 | 0.3 | stability | `cli-release-promote` |
| 18 | 46 | +28 | 0.3157 | 0.3 | stability | `route-verbs-refuse-mid-wave` |
| 19 | 33 | +14 | 0.3147 | 0.35 | performance | `repo-contract-cancel-orphans-test-binary` |
| 20 | 2 | -18 | 0.312 | 0.56 | hygiene | `inboxbatch-utf8-and-resolution` |
| 21 | 29 | +8 | 0.3112 | 0.4 | debuggability | `cli-salvage-list` |
| 22 | 48 | +26 | 0.3108 | 0.3 | stability | `unsatisfiable-lint-follows-exit-codes-in-variables` |
| 23 | 3 | -20 | 0.3076 | 0.55 | hygiene | `router-silent-errors` |
| 24 | 4 | -20 | 0.3073 | 0.55 | hygiene | `phasespec-roots-silent-policy-error` |
| 25 | 6 | -19 | 0.3031 | 0.54 | hygiene | `policy-resolver-hygiene` |
| 26 | 8 | -18 | 0.2985 | 0.53 | hygiene | `phasecmd-silent-policy-fallbacks` |
| 27 | 10 | -17 | 0.294 | 0.52 | hygiene | `fleet-runpool-silent-success` |
| 28 | 11 | -17 | 0.2939 | 0.52 | hygiene | `llmroute-default-trigger-aliasing` |
| 29 | 12 | -17 | 0.2939 | 0.52 | hygiene | `interaction-rollup-tmp-collision` |
| 30 | 13 | -17 | 0.2895 | 0.51 | hygiene | `tokenusage-silent-reads-and-vacuous-test` |

## Biggest movers

### Up

| Item | Today | New | Delta | Class | Weight |
|---|---:|---:|---:|---|---:|
| `route-verbs-refuse-mid-wave` | 46 | 18 | +28 | stability | 0.3 |
| `releasepreflight-fail-open-pins` | 38 | 11 | +27 | stability | 0.35 |
| `rollback-fail-open-and-vacuous-tests` | 39 | 12 | +27 | stability | 0.35 |
| `cli-release-promote` | 44 | 17 | +27 | stability | 0.3 |
| `unsatisfiable-lint-follows-exit-codes-in-variables` | 48 | 22 | +26 | stability | 0.3 |
| `cli-failures` | 34 | 10 | +24 | stability | 0.35 |
| `cli-dossier-publish` | 24 | 2 | +22 | stability | 0.45 |
| `releasepipeline-journal-errors-and-classify-pin` | 30 | 8 | +22 | stability | 0.4 |
| `gittest-ebadf-retry-class` | 31 | 9 | +22 | stability | 0.4 |
| `acs-cycle1723-package-doc-predicate-stale-after-753` | 28 | 7 | +21 | stability | 0.45 |

### Down

| Item | Today | New | Delta | Class | Weight |
|---|---:|---:|---:|---|---:|
| `router-silent-errors` | 3 | 23 | -20 | hygiene | 0.55 |
| `phasespec-roots-silent-policy-error` | 4 | 24 | -20 | hygiene | 0.55 |
| `policy-resolver-hygiene` | 6 | 25 | -19 | hygiene | 0.54 |
| `inboxbatch-utf8-and-resolution` | 2 | 20 | -18 | hygiene | 0.56 |
| `phasecmd-silent-policy-fallbacks` | 8 | 26 | -18 | hygiene | 0.53 |
| `inbox-filename-stamp-single-source` | 15 | 33 | -18 | hygiene | 0.5 |
| `fleet-runpool-silent-success` | 10 | 27 | -17 | hygiene | 0.52 |
| `llmroute-default-trigger-aliasing` | 11 | 28 | -17 | hygiene | 0.52 |
| `interaction-rollup-tmp-collision` | 12 | 29 | -17 | hygiene | 0.52 |
| `tokenusage-silent-reads-and-vacuous-test` | 13 | 30 | -17 | hygiene | 0.51 |

`cli-scan-secrets` (security) falls 17 places, from 35th to 52nd, just outside the ten largest falls.

## What the operator should check

1. **Hygiene items that describe silent failures.** Every big faller is a `hygiene` item from the 2026-09-26 sweep, and several describe a defect that hides a failure: `router-silent-errors`, `phasespec-roots-silent-policy-error`, `phasecmd-silent-policy-fallbacks`, `fleet-runpool-silent-success`, `tokenusage-silent-reads-and-vacuous-test`. Each falls about 20 places on its class alone (hygiene's class feature is 0.25, stability's 0.875). The operator's answer (2026-10-06): they are not relabelled automatically; the P3 overlap judge and the operator re-class an item with `evolve inbox edit <id> --set priority_class=… --reason …`.
2. **Security is last on purpose.** With the corrected order a security item scores 0.025 below an otherwise equal hygiene item, the same as one class step. A filer who judges a security item urgent says so with its weight: at the default factors a weight about 0.06 higher overcomes one class step.
3. **Age barely moves anything yet.** 53 of the 55 ready items were filed within the last ten days, so age adds 0.01 or less to them; the two older ones, `premium-rung-placement-law-and-narrative-dedup` (34 days) and `intent-delta-gate-seam-threading` (56 days), get 0.027 and 0.036. Age matters more for the console-owned backlog, filed since July, and grows while an item waits (half of its 0.05 at 30 days).
4. **Console and waiting lists are ranked within themselves** and are not shown above. `evolve inbox rank --list console` puts the stability items at weights 0.9 to 0.94 first (`worktreephase-test-amplification-gap`, `continuation-findings-anchor-first`, `treediff-guard-worktree-phase-crosslane-residual`); the console list's two security items rank 175th and 183rd of 183. The three waiting items rank `chronicle-s7b-historian-filer`, `cli-phase-spec-phases`, `cli-dossier-sweep`.
