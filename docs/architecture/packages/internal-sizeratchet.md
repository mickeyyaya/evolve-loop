# internal/sizeratchet

## Purpose

`internal/sizeratchet` enforces the 50-line function limit across a whole Go module. A function longer than `MaxLines` fails unless it is a listed offender. An allowance is a ceiling: growth past it fails, and one writer, the boundary `evolve ratchet tighten`, removes the slack under it.

## Design

- **Size.** A function's size runs from the `func` keyword to its closing brace. The doc comment is excluded.
- **Keys.** A key is `<slash dir>.<Name>`, or `<slash dir>.<Receiver>.<Name>` for a method. The receiver is keyed by its base type, so same-named methods on different types keep separate allowances. Generic and parenthesized receivers resolve to the base type too.
- **Build-tagged twins.** Same-named declarations in build-tagged files share one allowance. `Check` measures the largest of them.
- **Walk.** The walk reads non-test `.go` files whatever their build constraints. It skips `vendor`, `testdata`, and directories starting with `.` or `_`. A file that does not parse is an error, and an unreadable directory fails the walk rather than being skipped. `parseSpans` measures one source from disk or from bytes, and `measured` says if the walk reads a module-relative path; both are in-package helpers.
- **Allowances are ceilings.** A listed function under its allowance, within the limit or gone is slack. Slack is never a failure. Growth past an allowance, and any unlisted function past the limit, fail.
- **One writer.** No lane edits `offenders.json`. The console runs `evolve ratchet tighten` at the wave boundary. The verb refuses to write inside a phase or under a live lease (`refuseBoundaryWrite`, shared with `evolve cli-routing set`); `--check` is never refused. The boundary step itself (`evolve boundary run`) does not call the verb yet, see Findings. The verb writes the result of these pure steps:
  - `Tighten` lowers each allowance to the size of its function and drops an entry whose function is within the limit or gone. It never raises an allowance, and it returns a new map.
  - `Encode` is byte-stable: `json.MarshalIndent` with two spaces and a final newline, which reproduces the checked-in file.
  - `TightenModule` runs the walk, the load, `Tighten` and `Encode` from one root. `Changes` names each removed or lowered entry in key order.
- **The check against the base.** `CheckAgainstBase` serves the build floor. It reads the offender list and the changed files at the base, measures the same files on disk, and runs `Check` against `Tighten(base spans, base list)`. So the ceiling of a listed function is its size at the base, never its slack. A lane cannot regrow a function before the boundary tighten removes the slack. The refusal tells the lane to shrink the function back to its base size, not to edit the list; it never names the writer.
  - A module without a list at the base is silent. A read or parse failure is an error, which the floor reports as a WARN and never as a refusal.
- **One constant.** `OffendersRelPath` is the module-relative path of the list. Each message and each caller uses it.
- **Loud failure.** A missing offender list is an error, not an empty allowance set.

## Invariants

- **Every allowance exceeds `MaxLines`.** `parseOffenders` rejects an entry at or under the limit, so a function within the limit cannot hold an entry.
- **`Scan` walks, loads and checks from one root.** `TestScan_RunsWalkOffendersAndCheckFromOneRoot` pins this.
- **The module gate reads the module it is tested in.** `moduleRoot` walks up to the nearest `go.mod`.
- **`Tighten` never raises an allowance and never mutates its input.** Pinned by `TestTighten_LowersEachAllowanceToItsFunctionAndDropsHealedOrGoneOnes`.
- **`Encode` reproduces the checked-in list byte for byte.** Pinned by `TestEncode_ReproducesTheCheckedInListByteForByte`.
- **Two lanes that shrink listed functions in different packages rebase identically.** Neither lane touches the list, `treedelta.Identical` proves the rebase, and one boundary tighten then lowers both entries. Pinned by `TestTighten_TwoLanesThatShrinkListedFunctionsInDifferentPackagesRebaseIdentically`.
- **The floor refuses regrowth into slack and a new offender against the base list.** Pinned by `TestCheckAgainstBase_RefusesRegrowthIntoSlackAndANewOffenderAgainstTheBaseList`.
- **`test/structure` shares the cap.** `CheckLimits` reads `MaxLines`, so a function of exactly 50 lines passes both checks.

## Findings

- **2026-09-28, the inbox architecture review (`sizeratchet-allowances-not-a-shared-registry`).** `offenders.json` held 283 exact-match allowances, and the ratchet refused a shrunk function whose allowance was not lowered. So every change to a listed function edited the one file: nine commits in four hours. Two lanes that both touch the file never carry an audited verdict across a rebase, so cycle 1731 re-authored after cycle 1732. The ceiling rule (`fix/sizeratchet-is-a-ceiling`) removed the need to write.
  - This landing adds the one writer (`Tighten`, `Encode`, `evolve ratchet tighten`), the build floor's check against the base, one path constant and one cap shared with `test/structure`. A lane that grows a listed function inside its old slack fails the floor. After the next boundary tighten the ratchet itself refuses the same growth, so the floor only moves that refusal earlier.
  - **Not wired: a follow-up.** R3 of the logic-first design (§5.13) is built, not wired: `evolve boundary run` and `wave next` do not call the tighten. A boundary step that writes `offenders.json` edits the main tree, and the boundary has no step that commits. An uncommitted edit does not reach the next wave's worktrees, and `sync-main` refuses a dirty tree at the next boundary. So the step needs a landing path for its write: a console commit through `evolve ship --class manual`, or a branch the next boundary merges. Until that lands, the console runs `evolve ratchet tighten --root go` by hand at the boundary and lands the result as a console commit.
  - **The raw-git half: a follow-up.** `rawgitratchet/baseline.json` has the same shared-file shape, and `rawgitratchet.Check` is still exact-match. Two lanes that each shrink raw-git sites must both edit that file. The fix is the same ceiling rule and the same writer. One tighten over a `map[string]int` with a per-list floor (`MaxLines` here, zero sites there) serves both lists. That is a design step, not a copy of `tighten.go`, so this landing leaves `rawgitratchet` as it is.
