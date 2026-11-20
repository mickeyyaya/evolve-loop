# 2026-11-19: the console applied the backlog dispositions with two errors

Related documents: the dispositions are in the research document of [PR #1571](https://github.com/mickeyyaya/evolve-loop/pull/1571), and the procedure is the [issue-record](../../skills/issue-record/SKILL.md) skill.

## Summary

- **Impact:** at the boundary of wave 181, the console applied the approved dispositions D1 to D7 to the inbox. Two errors followed. First, 18 consumed items lost the evidence text of their resolution. Second, 9 of the 31 items that went to lanes were not lane work, and one of them bounced back in cycle 2553.
- **Root cause, error 1:** the console ran the D1 `inbox consume` calls in a zsh loop with a loop variable named `path`. In zsh, `path` is tied to `PATH`, so the loop removed each tool except the evolve binary, which the loop called by a relative path. The verb ran 18 times with a resolution that had no claim and no evidence.
- **Root cause, error 2:** D6 listed 35 items with a first-pass LANE claim. The adversarial pass of the review had checked only 3 of them (F5.2 of the review). 30 of the 35 went to lanes, and with the merged boot-refresh item of D5 that made 31. The route does not check that the prerequisites of an item are on main or in its `deps`.
- **Fix:** a code reviewer of the inbox PR (#1580) found error 1. The console rewrote the 18 resolutions with the claim and the evidence of the adversarial seats before the merge. For error 2, the console ran the adversarial pass on the 31 routed items in wave 182. The verdicts wait for the next boundary, because a mid-wave route or edit is refused.
- **Lesson:** a loop that runs a state-changing verb prints its full argument list first. A batch of routes to lanes gets the adversarial check before the routes, not after a bounce.

## Terms used throughout

- **Disposition:** an operator-approved change to an inbox item: consume, narrow, merge, route or retire.
- **First-pass claim:** the verdict of one review seat on one item, which no second seat checked.
- **Bounce:** lane triage refuses a claimed item and routes it back to the console.

## Timeline

| Time (UTC, 2026-11-19) | Event | Evidence |
|---|---|---|
| 20:43 | The console consumes the 18 D1 items in one loop. Each call returns rc 0. The output files are empty, because `cat` is not on the PATH. | `consumed.at` of the 18 items |
| 20:46 | D6 starts. 30 of the 35 routes succeed. 3 are refused as protected, and 2 wait for a verb that can drop a done protected path. | `routed_at` of `agy-tier-map-single-source` |
| 21:07 | The code reviewer of the inbox PR reports a MAJOR finding: the 18 resolutions start with a comma and end with a colon. | `agent-code-reviewer-20261119T210751Z-6ba0` |
| 21:10 | The console rewrites the 18 resolutions in the runtime inbox and in the PR branch. A patch check passes. | `agent-code-reviewer-20261119T211022Z-f6ec` |
| 21:39 | Wave 182 starts with 8 lanes. | `wave: launched wave 182` |
| 21:48 | Lane triage of cycle 2553 drops `shadow-contract-k5-stage-flip`: prerequisites K3 and K4b are absent. | `INBOX_ITEM_ROUTED_CONSOLE`, cycle 2553 |
| 21:49 | The console runs the adversarial pass on the 31 routed items. 22 are lane work. 9 are not. | `agent-adversarial-review-20261119T214924Z-3772` |

The resolution of one D1 item before the fix:

```text
"resolution": ", confirmed by adversarial review 2026-11-19 (backlog review D1, PR #1571):"
```

The bounce of cycle 2553:

```text
route-console: 'shadow-contract-k5-stage-flip' is now console-manual — lane triage (cycle 2553) dropped: stale: prerequisites K3 and K4b absent at HEAD f25d49e4a; go/cmd/evolve/cmd_stage.go missing
```

## Root cause

**F1.1** The loop read each row with `while IFS=$'\t' read id ok st path`. In zsh, the array `path` is tied to `PATH`. The first row set `PATH` to an inbox file path.
*Implication:* `awk`, `jq`, `sed` and `cat` were not found. The resolution text was built from empty values.

**F1.2** The loop called the evolve binary as `./go/bin/evolve`, a relative path that does not use `PATH`. The verb ran and changed the inbox.
*Implication:* the state change happened, but the argument check and the output did not.

**F1.3** The loop printed no argument list before it ran. No verb can rewrite the resolution of a consumed item.
*Implication:* only the review of the inbox PR found the error. The console fixed it with a direct edit of the 18 data files.

**F2.1** The review checked 3 of the 35 LANE claims (F5.2 of the review). It chose to check the drops first, because a wrong drop loses work and a wrong route costs one claim.
*Implication:* 9 wrong routes reached the lanes. 4 need a protected path, and 3 lack a `deps` entry for an open prerequisite. 1 names a file that moved, and 1 is done on main.

**F2.2** `inbox route-lane` refuses a declared protected path. It does not check the prerequisites that the summary, the fix or the plan names.
*Implication:* the route of an item with an open prerequisite succeeds, and lane triage finds the gap one claim later.

**F2.3** A mid-wave route or edit is refused (shadow stage of the inbox lifecycle).
*Implication:* the console cannot correct a wrong route during the wave. Each wrong route can cost one lane claim until the boundary.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| The 18 D1 resolutions hold the claim and the evidence of the adversarial seats | #1580 | none: a data fix, checked by `agent-code-reviewer-20261119T211022Z-f6ec` |
| The 9 wrong routes get their corrections at the boundary of wave 182. These are 4 routes to the console, 3 `deps` entries, 1 `files[]` edit and 1 consume | next boundary inbox PR | open |
| The console names no shell variable `path`, and prints the arguments of a state-changing loop before it runs | console practice | none: a practice, not code |

## Limits

- No test can pin F1. It is a defect of a console shell session, not of the pipeline. The procedure is the only control.
- F2.2 has no class fix yet. A check of prerequisites in prose needs a judgment, so the adversarial pass stays the control.
- Until the boundary, the 9 wrong routes can each cost one lane claim.

## What we learned

- In zsh, never name a variable `path`, `status`, `prompt`, `cdpath` or `fpath`.
- Before a loop runs a state-changing verb, print each argument list and read it.
- Check a batch of routes to lanes with an adversarial seat before the routes. A first-pass LANE claim was wrong 9 times in 31.
