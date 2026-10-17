# internal/acspathscan

> Related: [internal-cycleoutcome.md](internal-cycleoutcome.md#protected-predicate-route-tdd) (the route that uses this scan). This page keeps the package-level detail that the code cannot say.

## Purpose

`internal/acspathscan` finds the repository paths that an ACS predicate reads. `Needs` returns the paths that a membership function accepts. `Snapshot` copies the predicate files, so the closeout can scan them again.

## Design

- **A leaf package.** It imports only the standard library. The core decision and the closeout both call it, so the rule has one definition.
- **Reads, not mentions.** A path counts only inside a `func Test*` body, as an argument of a file read. A variable counts if it carries the path to such a read.
- **The repo root is named.** A `filepath.Join` counts if all arguments are literals, or if the first argument is `RepoRoot(t)` or a variable that holds it. A temp directory base does not count.
- **Faults are loud.** A bad pattern or a file that does not parse returns an error. The paths from the other files are still returned.

## Invariants

- **The scan never runs code.** It uses `go/parser` only.
- **A nil membership function returns nothing.**

## Findings

- A scan that took any literal or any `Join` was a cheap escape hatch. A dead-code read turned an expected FAIL into a route. The reads rule and the root rule close it.
