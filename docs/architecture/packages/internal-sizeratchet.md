# internal/sizeratchet

## Purpose

`internal/sizeratchet` enforces the 50-line function limit across a whole Go module. A function longer than `MaxLines` fails unless it is a listed offender. An allowance is a ceiling: growth past it fails, and one writer, the boundary `evolve ratchet tighten`, removes the slack under it.

## Design

- **Size.** A function's size runs from the `func` keyword to its closing brace. The doc comment is excluded.
- **Keys.** A key is `<slash dir>.<Name>`, or `<slash dir>.<Receiver>.<Name>` for a method. The receiver is keyed by its base type, so same-named methods on different types keep separate allowances. Generic and parenthesized receivers resolve to the base type too.
- **Build-tagged twins.** Same-named declarations in build-tagged files share one allowance. `Check` measures the largest of them.
- **Walk.** The walk reads non-test `.go` files whatever their build constraints. It skips `vendor`, `testdata`, and directories starting with `.` or `_`. A file that does not parse is an error, and an unreadable directory fails the walk rather than being skipped. `parseSpans` measures one source from disk or from bytes, and `measured` says if the walk reads a module-relative path; both are in-package helpers.
- **Allowances are ceilings.** A listed function under its allowance, within the limit or gone is slack. Slack is never a failure. Growth past an allowance, and any unlisted function past the limit, fail.
- **One writer.** No lane edits `offenders.json`. The boundary step `ratchet-tighten` proposes the tightened list as a pull request at each boundary, and it merges that pull request when its gates pass. The console can run `evolve ratchet tighten` by hand. The verb refuses to write inside a phase or under a live lease (`refuseBoundaryWrite`, shared with `evolve cli-routing set`); `--check` is never refused. The verb writes the result of these pure steps:
  - `Tighten` lowers each allowance to the size of its function and drops an entry whose function is within the limit or gone. It never raises an allowance, and it returns a new map.
  - `Encode` is byte-stable: `json.MarshalIndent` with two spaces and a final newline, which reproduces the checked-in file.
  - `TightenModule` runs the walk, the load, `Tighten` and `Encode` from one root. `Changes` names each removed or lowered entry in key order.
- **The check against the base.** `CheckAgainstBase` serves the build floor. It reads the offender list and the changed files at the base, measures the same files on disk, and runs `Check` against `Tighten(base spans, base list)`. So the ceiling of a listed function is its size at the base, never its slack. A lane cannot regrow a function before the boundary tighten removes the slack. The refusal tells the lane to shrink the function back to its base size, not to edit the list; it never names the writer.
  - A module without a list at the base is silent. A read or parse failure is an error, which the floor reports as a WARN and never as a refusal.
- **One constant.** `OffendersRelPath` is the module-relative path of the list. Each message and each caller uses it.
- **Loud failure.** A missing offender list is an error, not an empty allowance set.

## The merge gate

This section is the one home of the contract of the merge gate. [runtime-reference.md](../../operations/runtime-reference.md) and [internal-policy.md](internal-policy.md) link here.

The step first writes the tightened lists from `origin/main` into its worktree, and both scans must be clean on them. Then it checks the open tighten pull request. The merge needs each of these gates:

1. The head that `git fetch` reads equals the `headRefOid` that `gh pr list` gave.
2. The head commit has one parent. A merge commit can bring any history through its second parent, so the step refuses it.
3. The parent of the head commit is an ancestor of `origin/main`.
4. The files that change between the merge base and the head are a non-empty subset of `ratchetListPaths()`. This is the same table that the tighten step writes, so the two lists cannot differ.
5. The lists at the head are equal to the lists that the step wrote from `origin/main`. So the merge lands only the tree that the step itself writes now. A list that raises an allowance, adds a key or is stale fails this gate.
6. Required CI is green on the head. The check is `awaitGreen` of `evolve pr merge`, and the merge is its `mergeHead` with `--match-head-commit`.

Gates 2 to 5 make the branch prefix and the non-fork bit insufficient for a merge. A writer who pushes a `chore/ratchet-tighten-*` branch with other code gets no merge. The step also never selects a fork's pull request.

The stage key is `boundary.ratchet_merge.stage`, and the default is `shadow`:

| Stage | What the step does |
|---|---|
| `shadow` | It checks each gate, prints `ratchet merge: would merge #<n> at <sha>` and does not merge. |
| `enforce` | It merges and prints `ratchet merge: merged #<n> <sha>`. |
| `off` | It checks no gate. |

The step reports a failed gate like this:

- A refusal from gate 5, the CI wait or the merge admission prints `ratchet merge: #<n> held: <cause>` on stdout. A stale list is the normal case of gate 5.
- Any other gate failure prints `WARN: ratchet merge: #<n>: <cause>` on stderr. An unknown stage word is one example, because `policy.Load` refuses it.
- In both cases the step then replaces the tip with its own commit of the lists from `origin/main`.

After it tries a merge at `enforce`, the step checks out the new `origin/main`, lists the open pull requests again and tightens again. So it never pushes onto the branch of a merged pull request. A shadow or held gate does not refresh.

**No idle force-push.** The step does not push when the tip is a commit that passes gates 2 to 4 and holds the regenerated lists. Then it prints `ratchet tighten: #<n> already holds this list`. A push restarts required CI, which takes about 20 minutes. A tip with other content always gets the step's own commit.

**The gate lays no list over the worktree.** An earlier version checked out the lists of the pull request into the worktree and then restored them. A failed restore left the lists of the pull request in the worktree that the step tightens from. Gate 5 compares the lists with `git diff` and writes nothing, so that state cannot occur.

**Limits.** Gate 5 accepts only the current lists. When a listed function shrinks on `main` between two boundaries, the step replaces the tip. Then CI runs again before the next boundary can merge it. The tests use a fake CI result, so they do not prove the CI wait against GitHub. Gate 6 trusts the required checks of `main`.

The key goes to `enforce` after 3 clean boundaries at `shadow` (plan AD4-5). The design is option E of section 4.2 of `docs/research/architecture-decisions-2026-11.md` (docs branch, 2026-11-03). Pinned by the `TestBoundaryRatchetMerge_*` tests in `go/cmd/evolve`.

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
  - **Wired at the boundary (2026-10-20).** The boundary step `ratchet-tighten` (`go/cmd/evolve/cmd_boundary_ratchet.go`) runs `TightenModule` and `Scan` in a temporary worktree of `origin/main`. It commits only `offenders.json` on a branch `chore/ratchet-tighten-<date>` and opens a pull request, or updates the open one. The pull request merges at a wave boundary. A failure warns and does not stop the boundary. The rejected options are in [runtime-reference.md](../../operations/runtime-reference.md) (`evolve ratchet tighten`).
  - **The raw-git half (2026-10-21).** `rawgitratchet/baseline.json` had the same shared-file shape, and `rawgitratchet.Check` was exact-match. Now one tighten over a `map[string]int` serves both lists, with a floor for each list. `TightenCounts` and `TightenEncoded` take the floor: `MaxLines` here and zero sites for the raw-git list. `Tighten` and `TightenModule` call them.
    - The boundary step commits both lists in one pull request ([internal-rawgitratchet.md](internal-rawgitratchet.md)). Pinned by `TestTightenCounts_KeepsOnlyCountsAboveTheFloorAtTheirLowerValue` and `TestTightenEncoded_NamesEachChangeAndEncodesTheTightList`.
  - **The step merges its own pull request (2026-11-03, plan AD4-4).** The pull request stayed open because no step merged it (PR #1070, two weeks). Now the step merges the open tighten pull request, then it tightens again from the new `origin/main`. The gate is in `go/cmd/evolve/cmd_boundary_ratchet_merge.go`, and [the merge gate](#the-merge-gate) has its contract.
