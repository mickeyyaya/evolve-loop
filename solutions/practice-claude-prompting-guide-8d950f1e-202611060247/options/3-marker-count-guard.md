# Option 3 — add a deterministic marker-count check and a follow-up item

All numbers cite `assumptions-and-evidence.md`.

## What changes, for whom

A check counts CRITICAL and MUST markers per persona and records the count. A follow-up inbox item tracks the empty-body defect of the watcher. The console and the watcher are the users.

## Causal chain to the goal metric

A recorded count gives the before measure of D8 without a manual grep (E4). The follow-up item stops empty snapshots from reaching builds.

## Quantified expected effect

6 personas get a count (E4). The effect on ship rate is not sourced (A2).

## Cost and time to effect

One build cycle for the check and one for the watcher fix (E6). Time to effect is two cycles.

## Top two risks

1. The check becomes a metric that persona authors game. Detection: the D8 outcome measures stay the judge.
2. M2 may already plan the same tool (E3). Detection: read the plan before the build.
