# internal/codeshape

> Decision: [ADR-0153](../adr/0153-quality-is-a-panel-of-separate-indexes-with-intervals-that-the-signal-center-publishes.md). Spec of record: [llm-quality-indexes.md](../llm-quality-indexes.md) §6. Plan: [llm-quality-indexes-2026-10](../../plans/llm-quality-indexes-2026-10.md) component Q3. Caller: `go/cmd/evolve/cmd_quality.go` (the wave and tree scopes of `evolve quality report`).

## Purpose

`internal/codeshape` scans Go source and the persona prose and computes four code indexes. CQ1 is unit conformance, CQ3 is interface depth, CQ4 is duplication and CQ5 is the judgment surface. It hands each reading to [internal/qualitypanel](internal-qualitypanel.md).

## Design

- **Production files.** `Production` takes a path relative to `go/`. It keeps `.go` files that are not tests and not under `acs/`, `test/`, `testdata/`, `vendor/` or a hidden directory.
- **The four limits.** `ParseFuncs` measures each function: lines, cyclomatic complexity, nesting depth and parameters. Cyclomatic is 1 plus each `if`, `for`, `range`, `case`, `select` case, `&&` and `||`. Nesting counts `if`, `for`, `range`, `switch`, `select` and function literals. A function conforms at 30 lines, cyclomatic 10, nesting 3 and 4 parameters or fewer.
- **CQ1 on the tree.** `TreeConformance` is a census of every production function. Its own threshold is 0.878, one point under the 0.888 baseline, because the registry threshold of 0.90 is for changed functions.
- **CQ1 on changed functions.** `ChangedFuncs` reads `git show -U0 --no-renames --diff-merges=first-parent` of a ship commit. It parses each changed production file at that commit and keeps each function that a new hunk touches. `ChangedConformance` emits one reading per ship of the closing wave, with its cycle, and one reading for the wave.
- **CQ3 and CQ4.** `Scan` walks the tree once. It counts logical lines (no blank, brace-only, `package` or `import` lines), exported symbols per directory and each 5-line window. CQ3 is the median of lines per exported symbol over `internal/` packages. CQ4 counts the windows that occur two or more times, per 1,000 lines. The added form counts the added windows of the closing wave that also occur twice or more in the tree.
- **CQ5.** `RuleLines` counts the lines in `agents/*.md` and `skills/*/SKILL.md` that match `^\s*[-*0-9.]+\s*\**(If|When|Unless)\b`.

## Invariants

- Each git call starts through `sysexec.Command` with a one-minute timeout.
- The scan only reads. The stages come from the registry: CQ1 at `report`, CQ3, CQ4 and CQ5 at `shadow`. No gate reads these readings.

## Findings

- On 2026-10-21 at `12624d1e1`, CQ1 was 0.888 of 11,367 functions and CQ3 a median of 14.8 over 312 packages. CQ4 was 19.5 per 1,000 lines, and CQ5 was 49 rule lines in 140 files.
- The research CQ4 baseline of 22.7 also read `go/vendor/`. Without the third-party code, the rate is 19.5.
- The added form of CQ4 matches the added windows against the tree at the time of the report, not the tree of the ship. A later edit of either copy changes the count.
- A function literal adds one nesting level, so an `if` inside a closure counts 2. CQ3 counts exported top-level names and exported methods, not struct fields or interface methods.
