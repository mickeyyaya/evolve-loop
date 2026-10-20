# internal/mergewindow

> Plan: [pr-merge-latency-2026-10](../../plans/pr-merge-latency-2026-10.md) (P2, D2 to D4). Decision: [ADR-0148](../adr/0148-docs-inbox-and-ci-changes-merge-in-a-window-and-code-merges-at-a-boundary-that-the-console-can-call-early.md). Commands: `evolve pr window` (`go/cmd/evolve/cmd_pr_window.go`), `evolve pr merge` (`cmd_pr.go`) and `evolve pr boundary` (`cmd_pr_boundary.go`). Operator use: [wave-operations.md](../../operations/wave-operations.md) §5.

## Purpose

`internal/mergewindow` decides if a PR is a window PR or a boundary PR. A window PR can merge while a wave runs. A boundary PR waits for a boundary. The package is pure: the caller gives the bytes of `required.yml` and the changed paths.

## Design

- **One rule with CI.** `Parse` reads the two `case "$path" in` tables inside the path loop of the job `changes` in `.github/workflows/required.yml`. The first table sets `docs_found` and `other_found`. The second table sets `go_required`, `landing_required` and `inbox_found`.
- **The class of a PR, as CI routes it.** `Rule.Classify` first drops each path under `.github/`, because a CI file does not change the plane binary or a lane base. Then it gives the routing of CI for the other paths of the PR:
  - When the first table marks each path as docs and none as other, CI routes the PR to no suite. The PR is a window PR.
  - Else, when the second table routes no path to the Go suite or the landing suite, the PR is a window PR.
  - Else the PR is a boundary PR. The verdict names each path that the second table routes to a Go or landing suite, once, in input order.
- **A worked example.** `README.md` alone is a window PR: it is a docs-only PR. `README.md` with `.evolve/inbox/x.json` is a boundary PR. The inbox path is "other" to the first table. Thus CI routes `README.md` through the catch-all of the second table to the Go and landing suites. An empty change is a boundary change, because CI routes an empty change to each suite.
- **Fail closed.** `Parse` returns an error for each part of the job `changes` that it does not model:
  - no job `changes`, no path loop or more than one, or a count of tables other than 2;
  - a line in the path loop of another type. The loop can have blank lines, comments, `case "$path" in`, `esac` and the arms of an open table;
  - an assignment of a routing flag (`*_required=`, `*_found=`) outside the two tables, with a value other than `false`;
  - a change to a line of the job `changes` outside the loop: the outputs, the steps, the `env` and the script. Comments and blank lines do not count;
  - a change to the lines of the jobs `go` and `landing`, which gate the two suites on the outputs of `changes`;
  - a table with no catch-all arm `*)` at the end, or a catch-all arm before the end;
  - a catch-all arm that does not set `other_found` (first table), or `go_required` and `landing_required` (second table);
  - an unknown flag, or a table that never sets a flag that the rule reads;
  - a glob with a character other than literal text and `*`.

  Thus a routing change in `required.yml` that the rule does not model stops the verb with exit 2. The console then updates the rule, and the window stays closed until then.
- **Limit of the model.** A new job that runs a Go or landing suite with no gate on `changes` is outside the model. Such a change is a CI PR, so it gets a review before it merges. The job `validate` (`ci.yml`) runs for each PR that is not docs-only, so it does not change the class. The pinned lists copy `required.yml`, so each edit of the router needs the same edit in `mergewindow.go`.
- **Glob semantics.** A `*` in a `case` pattern matches each character, `/` and a newline included. `globRegexp` gives the same semantics with the flag `(?s)`.
- **One name for each path.** The command reads one JSON string for each file name from `gh`, and one name for each line from `--files-from`. Thus a name with a space stays one path.

## Consumers

- **`evolve pr merge` at an open gate.** The gate is open at `pr.merge_window.stage: on` and `fleet.integration.stage: enforce`. While a loop runs, the verb asks `evolve pr window` for each PR, before the view and again before the merge. A window PR merges. A boundary PR gets the refusal `a loop is running: ... is live; #<n> waits for a boundary: <reason>`.
- **`evolve pr merge` at a closed gate.** A live loop refuses each PR before any `gh` call, as before P2b. At each gate, the verb checks the plane again before the merge, so a loop that starts during the CI wait also counts.
- **`evolve pr boundary`.** It gives the early-boundary call of ADR-0148 decision 7 in one line. `DecideEarlyBoundary` is the pure rule:
  - The input is each green boundary PR with its wait, the pool (live lanes and width) and the bound.
  - The PR that waited longest is the candidate. A tie keeps the first PR in the list.
  - The call is due when that wait is longer than the bound and the live lanes are fewer than the width. A wait equal to the bound is not due.
- **The facts of `evolve pr boundary`.** The bound is `pr.merge_window.early_boundary_after_s` (compiled default 3600 s, plan Q2). The width is `fleet.count`, by the operator decision of 2026-10-20 ("Combine them to fleet count"). The live lanes are the pool signal of the last wave record (`wave.PoolLive`), the same count as `evolve wave status`.
- **Green and waiting.** A green PR is a non-draft PR whose newest `required.yml` run on its head is `completed` and `success`. Its wait starts at the `updatedAt` of that run. A window PR does not wait, so the rule skips it.
- **No call.** No live loop exits 3, because the plane is at a boundary. A missing pool signal for a live loop exits 2, because the verb cannot prove that the pool is below its width.

## Tests

- `TestClassify_TheLiveCIRoutingTableDecidesEachPath` parses the live `required.yml`. A change of the CI router changes the result of this test.
- `TestParse_FailsClosedOnATableItCannotRead` covers each parse error.
- `cmd/evolve/cmd_pr_window_test.go` covers the stage, the exit codes, the JSON and the `gh` query.
- `TestDecideEarlyBoundary_DueOnlyWhenTheLongestWaitPassesTheBoundAndThePoolIsBelowItsWidth` covers the early-boundary rule.
- `cmd/evolve/cmd_pr_merge_window_test.go` covers `evolve pr merge` with a live loop at each gate. `cmd/evolve/cmd_pr_boundary_test.go` covers `evolve pr boundary`.
