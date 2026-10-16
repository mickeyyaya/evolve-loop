# internal/sizeratchet

## Purpose

`internal/sizeratchet` enforces the 50-line function limit across a whole Go module. A function longer than `MaxLines` fails unless it is a listed offender, and a listed offender's allowance may only shrink until its entry disappears.

## Design

- **Size.** A function's size runs from the `func` keyword to its closing brace. The doc comment is excluded.
- **Keys.** A key is `<slash dir>.<Name>`, or `<slash dir>.<Receiver>.<Name>` for a method. The receiver is keyed by its base type, so same-named methods on different types keep separate allowances. Generic and parenthesized receivers resolve to the base type too.
- **Build-tagged twins.** Same-named declarations in build-tagged files share one allowance. `Check` measures the largest of them.
- **Walk.** The walk reads non-test `.go` files whatever their build constraints. It skips `vendor`, `testdata`, and directories starting with `.` or `_`. A file that does not parse is an error, and an unreadable directory fails the walk rather than being skipped.
- **Allowances are ceilings.** A listed function under its allowance, within the limit or gone is slack. A boundary tighten removes it, and it is never a failure. Growth past an allowance, and any unlisted function past the limit, fail.
- **Loud failure.** A missing offender list is an error, not an empty allowance set.

## Invariants

- **Every allowance exceeds `MaxLines`.** `LoadOffenders` rejects an entry at or under the limit, so a function within the limit cannot hold an entry.
- **`Scan` walks, loads and checks from one root.** `TestScan_RunsWalkOffendersAndCheckFromOneRoot` pins this.
- **The module gate reads the module it is tested in.** `moduleRoot` walks up to the nearest `go.mod`.

## Findings

- None.
