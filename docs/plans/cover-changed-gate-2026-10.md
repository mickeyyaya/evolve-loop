# Plan: `evolve cover changed`, a gate that refuses a changed function below 100%

- **Status:** built, 2026-10-11, in lane `cl-cover-changed` (branch `fix/cover-changed-gate`). Fix round 1 (2026-10-12) applied the review decisions. CI enforces the gate. The loop build floor runs it at stage `shadow`.
- **Package notes:** [internal-covchanged.md](../architecture/packages/internal-covchanged.md).
- **Operator text:** [runtime-reference.md](../operations/runtime-reference.md) and [go/docs/testing.md](../../go/docs/testing.md).

## Table of contents

1. [Request](#1-request)
2. [Facts](#2-facts)
3. [Goals and non-goals](#3-goals-and-non-goals)
4. [Candidate designs](#4-candidate-designs)
5. [Decisions](#5-decisions)
6. [Components](#6-components)
7. [Verification](#7-verification)
8. [Shadow to enforce in the loop floor](#8-shadow-to-enforce-in-the-loop-floor)
9. [Open questions](#9-open-questions)

## 1. Request

The operator wrote this on 2026-10-11: "Could you also review the changes made in the past 14 days are 100% test coverage. Hence in each tests, we also consider different edge cases and considering a variety of possible ways to test."

## 2. Facts

The console audit measured the changes of the last 14 days. Its data is in the lane scratchpad (`cov14/`).

1. In the last 14 days, 4,320 functions changed. Of these, 3,304 are at 100%, 974 are partial and 42 have no profile.
2. The 100% rule for changed functions started on 2026-10-08. After that date, 1,351 functions changed, and 192 of them are below 100%. They have 583 statements that no test runs.
3. Attribution of the 192 gaps: #814 61, #808 35, #807 31, the loop cycles 31, the comment batches 12.
4. All 35 functions with no profile after the rule are in files that the profile cannot measure. They are GOOS files (`_linux.go`, `_other.go`) and `testdata` fixtures.
5. **Root cause.** No gate checks "100% on every changed function":
   - `make cover-strict` checks only the package floors in `go/.cover-strict`.
   - A lane reports coverage only for the functions that the lane names.
   - The commit gate and the loop build floor do not measure coverage per function.
6. The loop build floor does not run `cover-strict`. It runs the unit tests of each changed package and a scoped coverage run for the packages in `go/.apicover-enforce` (`go/internal/core/build_floor_reviewer.go`).

## 3. Goals and non-goals

Goals:

- One check finds each function that a change touches and refuses it when it is below 100% statement coverage.
- The same engine runs from the command line, from `make`, from CI and from the loop build floor.
- CI enforces it on each pull request and each push.

Non-goals:

- The verb does not run tests. It reads a coverage profile that a test run made.
- The check does not measure branch coverage or mutation score. Mutation checks stay a review step.
- The check does not fix the 192 gaps. A lane that touches one of those functions backfills it.

## 4. Candidate designs

| Design | How it works | Result |
|---|---|---|
| A. Raise the package floors | Put every changed package in `go/.cover-strict` at 100. | Rejected. A package floor hides a gap in one function behind the average. Packages with old debt block every lane that touches them. |
| B. Join `go tool cover -func` | Join the function list of `go tool cover -func` with the changed functions by name. | Rejected. Names are not unique across files and build tags. The output has no statement counts, and it cannot tell "no profile" from "zero statements". |
| C. Changed lines only (diff-cover) | Refuse each changed line that no test runs. | Rejected. The operator asked for the whole changed function at 100%. A test that runs only the changed line does not prove the function. |
| D. A pure core with ports and four wiring points | Port the console prototype into `internal/covchanged`. The diff, the files and the profile come through ports. | **Selected.** It measures by profile block, so a function with a profile is exact. The same `Check` serves the verb, `make`, CI and the floor. |

## 5. Decisions

- **D1. No allowance file (fix round 1).** The first build had `go/.cover-changed-allow`, a ratchet that listed the 192 old gaps. The review removed it for two reasons. The allowance waived the 100% rule for the lanes that touched those functions, and a lane that deleted the file got past the ratchet. Now a lane that touches a function with a gap must test it to 100%.
- **D2. The commit gate (fix round 1).** The console rejected the freshness proof of the first plan (a tree hash beside `coverage.txt`). The selected way: the Go lane of the commit gate (`go/internal/commitgate/lanes.go`) adds `-coverprofile` to its `go test`. Then it calls `Workspace.Check` with base `HEAD` and the tags of that run. It is not in this round, for two reasons:
  - each scripted-runner test of the Go lane must then write a profile and answer five more git calls;
  - that `go test` has no `integration` tag, so a function that only an integration test covers is refused.

  CI is the backstop until that lane lands.
- **D3. CI is the enforcement point.** `.github/workflows/go.yml` runs `make cover-changed` right after `make test-integration`, with no `if:`. The base is the pull-request base or the commit that the push replaced, and `HEAD~1` when neither is a commit. The step runs on Ubuntu and on macOS, so each system measures its own GOOS files.
- **D4. Changed code lines.** A changed function has an added line that holds a Go token. The check reads the tokens of the new file with `go/scanner`. Thus a blank line, a line comment and a line inside a block comment do not count, also when the comment starts above the hunk.
- **D5. Removed code (fix round 1).** A hunk that removes code marks the function that held it. The parser records the new-side line where the removal starts. A function holds that point when the point is from its first line up to the line before its last line. A hunk that removes only comments, or a removal right after the closing line of a function, marks nothing.
- **D6. Moves and new files.** The diff turns rename detection off, so a moved file or function counts as changed. Untracked `.go` files count as wholly added. The diff starts at the merge base of the base and `HEAD`, so a base that moved on adds nothing.
- **D7. The function of a block.** A profile block counts in the function that holds the start of the block. Duplicate blocks merge by their key, and a block is hit when one entry hits it.
- **D8. Exempt files.** `_test.go`, files under `testdata` or `vendor`, and files that the build constraints exclude are exempt and listed. The constraints use `go/build` with the host GOOS and GOARCH and `Options.Tags`. The verb and the floor give `covchanged.DefaultTags` (`integration`, the tag of `make test-integration`). Generated files are not exempt (fix round 1): no file under `go/` has the header, and the exemption only invited gaming.
- **D9. No profile is a refusal.** A changed function with no profile block is refused and shown as "no profile". A function with zero statements and a block is at 100%.
- **D10. A stale profile is an error (fix round 1).** When a changed file is newer than the profile, the check stops with exit 2 and tells the operator to run `make -C go test-integration` again.
- **D11. Exit codes.** 0 when clean, 1 when refused, 2 for an IO error and 10 for a usage error, the same as `evolve pr merge`.
- **D12. The profile of the floor.** `make apicover-enforce` writes `coverage.apicover.txt` and passes it to `apicover-check` through `COVER_PROFILE` (default `coverage.txt`, so CI does not change). Thus `coverage.txt` stays the integration profile that `cover-changed` reads.
- **D13. The loop build floor.** `coverChangedFailures` (`go/internal/core/cover_changed_floor.go`) runs `scopedCoverProfile` with `-tags integration` over the changed packages, then `Check` against the cycle base. The policy key `cover_changed.stage` selects `off`, `shadow` (the compiled default, it warns) or `enforce` (it rejects the handoff). A test failure in that run refuses at `enforce` and warns at `shadow`. A plumbing fault only warns.

## 6. Components

| Component | Where |
|---|---|
| The diff parser `ParseDiff` (fuzzed) | `go/internal/covchanged/diff.go` |
| The profile parser `ParseProfileLine` (fuzzed) and `ParseProfile` | `go/internal/covchanged/profile.go` |
| The measurement `Measure` | `go/internal/covchanged/measure.go` |
| The verdict `Judge` and `Verdict` | `go/internal/covchanged/verdict.go` |
| The git and file adapter `Workspace.Check` | `go/internal/covchanged/workspace.go` |
| The verb `evolve cover changed` | `go/cmd/evolve/cmd_cover.go` |
| The floor target `make cover-changed` | `go/Makefile` (`COVER_CHANGED_BASE`, default `origin/main`) |
| The CI step | `.github/workflows/go.yml` |
| The loop build floor and the policy key `cover_changed` | `go/internal/core/cover_changed_floor.go`, `go/internal/policy/budgets_recovery.go` |

The local floor is now 8 targets: `lint`, `test`, `test-integration`, `test-e2e`, `test-acs-durable`, `apicover-enforce`, `cover-strict` and `cover-changed`. Run `cover-changed` after `test-integration`, because it reads the `coverage.txt` of that run.

## 7. Verification

- Table tests cover these cases:
  - a comment-only change, a block comment that starts above the hunk and an empty diff;
  - removed code, a removed comment and a function removed after another one;
  - a rename and a function moved to another file;
  - a GOOS file, `testdata` and a generated file;
  - duplicate blocks and the receiver forms;
  - a function with zero statements and a function with no profile.
- A real git fixture (`internal/gittest`) commits a module, changes one function, and gives a profile. The check refuses only that function, also when the base moved on. An untracked file is refused too.
- `FuzzParseDiff` and `FuzzParseProfileLine` have seed corpora.
- A Makefile test proves that `apicover-enforce` does not touch `coverage.txt`. A workflow test proves the CI step position and its base.

## 8. Shadow to enforce in the loop floor

Move `cover_changed.stage` to `enforce` only when all of these are true:

- **M4 is fixed.** Today the floor runs the tests of the changed packages twice: the selfcheck run and the coverage run of this check. One run with `-tags integration -coverprofile` must feed the selfcheck, the apicover naming check and `Check`.
- **The shadow warnings are true.** Over 3 waves, each `cover-changed (shadow)` warning names a function that CI also refuses.
- **The cost is known.** The added floor time over those waves is recorded.

## 9. Open questions

- **Q1. The commit-gate lane (D2).** It is the next item.
- **Q2. A gap that a lane did not make.** A lane that touches an old function with a gap must backfill it. Watch the number of CI refusals of this kind in the first week.
