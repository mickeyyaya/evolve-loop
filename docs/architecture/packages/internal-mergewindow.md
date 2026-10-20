# internal/mergewindow

> Plan: [pr-merge-latency-2026-10](../../plans/pr-merge-latency-2026-10.md) (P2, D2 to D4). Decision: [ADR-0148](../adr/0148-docs-inbox-and-ci-changes-merge-in-a-window-and-code-merges-at-a-boundary-that-the-console-can-call-early.md). Command: `evolve pr window` (`go/cmd/evolve/cmd_pr_window.go`). Operator use: [wave-operations.md](../../operations/wave-operations.md) §5.

## Purpose

`internal/mergewindow` decides if a PR is a window PR or a boundary PR. A window PR can merge while a wave runs. A boundary PR waits for a boundary. The package is pure: the caller gives the bytes of `required.yml` and the changed paths.

## Design

- **One rule with CI.** `Parse` reads the two `case "$path" in` tables inside the path loop of the job `changes` in `.github/workflows/required.yml`. The first table sets `docs_found` and `other_found`. The second table sets `go_required`, `landing_required` and `inbox_found`.
- **The class of a path.** A path under `.github/` is a window path. A path that the first table marks as docs and not other is a window path. A path that the second table routes to no Go suite and no landing suite is a window path. Each other path is a boundary path.
- **The class of a PR.** `Rule.Classify` gives `window` only when each path is a window path. It names each boundary path once, in input order. An empty change is a boundary change, because CI routes an empty change to each suite.
- **Fail closed.** `Parse` returns an error for each table that it cannot read:
  - no path loop or more than one, or a count of tables other than 2;
  - a table with no catch-all arm `*)` at the end, or a catch-all arm before the end;
  - a catch-all arm that does not set `other_found` (first table), or `go_required` and `landing_required` (second table);
  - an unknown flag, or a table that never sets a flag that the rule reads;
  - a glob with a character other than literal text and `*`.

  Thus a renamed flag in `required.yml` cannot make each path a window path.
- **Glob semantics.** A `*` in a `case` pattern matches each character, `/` and a newline included. `globRegexp` gives the same semantics with the flag `(?s)`.
- **One name for each path.** The command reads one JSON string for each file name from `gh`, and one name for each line from `--files-from`. Thus a name with a space stays one path.

## Tests

- `TestClassify_TheLiveCIRoutingTableDecidesEachPath` parses the live `required.yml`. A change of the CI router changes the result of this test.
- `TestParse_FailsClosedOnATableItCannotRead` covers each parse error.
- `cmd/evolve/cmd_pr_window_test.go` covers the stage, the exit codes, the JSON and the `gh` query.
