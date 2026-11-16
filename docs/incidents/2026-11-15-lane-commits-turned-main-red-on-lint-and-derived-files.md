# 2026-11-15: lane commits turned main red on lint and on a stale derived file, twice

Related documents: the model of the derived-file catalog is `go/internal/derived/derived.go`, and the package note [cmd-testlatency.md](../architecture/packages/cmd-testlatency.md) describes the tool that carried the lint finding.

## Summary

- **Impact:** main CI was red from 2026-11-13 and again on 2026-11-15. Every pull request inherited the red. PR #1466 and PR #1467 failed CI only on this cause.
- **Root cause:** the lane build floor does not run `golangci-lint`. The catalog entry `darwin-set` declares no inputs, so the post-build normalizer never selects `go/.darwin-set` for regeneration.
- **Effect:** a lane commit that adds an unchecked `f.Close()` or a Go package change lands on main. CI is the first place that finds the fault.
- **Fix:** PR #1449 repaired the first red. The second red (commit `671e768a8`) has a fix in PR #1469 per the fact sheet. Neither the fix of the cause nor PR #1469 is on main in this worktree (see Limits).
- **Lesson:** a guard must not be weakened to make main green. The macOS ratchet stayed a hard fail.

## Terms used throughout

- **Lane commit:** a commit that a cycle lane lands on main through the ship path.
- **Build floor:** the checks that the build phase runs before the lane lands.
- **Derived file:** a file that a generator writes, such as `go/.darwin-set`.
- **Normalizer:** the step after the build that regenerates stale derived files.
- **errcheck:** the `golangci-lint` check that flags an unchecked error return.

## Timeline

| Time | Event | Evidence |
|---|---|---|
| 2026-11-13 and 2026-11-14 | Lane commits `ba124fd27` and `155352335` (wave 171) land. `ba124fd27` adds `go/cmd/testlatency/cpujudge.go` with an unchecked `defer f.Close()`. Required CI fails. | commit `ba124fd27` diff, fact sheet |
| 2026-11-14 | PR #1449 lands. It fixes `cpujudge.go`, adds `./cmd/testlatency` to `go/.darwin-set`, and adds the macOS ratchet `-test-timeout`. | commit `fed20383d` |
| 2026-11-15 | Lane commit `671e768a8` (wave 173) adds another unchecked `defer f.Close()` at `go/cmd/testlatency/report.go:157`. Main is red again. | commit `671e768a8` diff |
| 2026-11-15 | PR #1466 and PR #1467 fail CI only on this finding. PR #1469 fixes it. | fact sheet; not in the git history of this worktree |

## Root cause

**F1.1** `ba124fd27` adds `defer f.Close()` in `loadRusage` of `go/cmd/testlatency/cpujudge.go`. `671e768a8` adds the same form at `go/cmd/testlatency/report.go:157`. The `errcheck` check of `golangci-lint` flags both.
*Implication:* the same defect entered twice in two waves.

**F1.2** The lane build floor runs no `golangci-lint` (fact sheet).
*Implication:* CI is the first place that runs lint, after the lane has landed on main.

**F1.3** The entry `darwin-set` in `go/internal/derived/derived.go:62-67` has `Outputs`, `Generate` and `Check`, but no `Inputs`. Other entries, such as `research-index` at lines 55-61, set `Inputs`.
*Implication:* the normalizer has no input path to compare, so it never selects `go/.darwin-set` as stale. A lane that changes the macOS package set leaves the file old, and the test that checks it fails in CI.

**F1.4** `go/.darwin-set` lists `./cmd/testlatency` at line 8 only after `fed20383d`.
*Implication:* the file was stale on main until PR #1449 repaired it by hand.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| `defer func() { _ = f.Close() }()` in `cpujudge.go`, `./cmd/testlatency` in `go/.darwin-set`, the `-test-timeout` wiring of the macOS ratchet in `.github/workflows/go.yml`, host-dependent sandbox tests and gc coverage gaps | #1449 (`fed20383d`) | the required CI jobs |
| The same close pattern in `report.go` | #1469 | not verified in this worktree |
| The `darwin-set` entry declares its inputs; the build floor runs `golangci-lint` on the changed packages through one commit-gate runner | #1462 (fix A), per the fact sheet | not verified in this worktree |

## Limits

- The git history of this worktree holds `fed20383d` for PR #1449 only. It holds no commit for PR #1462, #1466, #1467 or #1469. This record states their content as the fact sheet gives it.
- The fact sheet names `commitgate.LintPackages`, the signal `BUILD_LINT_FLOOR_SKIPPED` and the field `InputExt`. A search of `go/` finds none of them, and `derived.Entry` has no `InputExt` field (`go/internal/derived/derived.go:24-31`). This record leaves them out.
- The fact sheet says that `ba124fd27` and `155352335` "added `go/cmd/testlatency`". That is wrong: the tool exists since 2026-06-02. `ba124fd27` added `cpujudge.go` to it.
- The test names `TestDarwinSetFile` and the "mutants killed" claim do not appear in the code that was checked. `TestPRRefresh_*` tests exist in `go/cmd/evolve/cmd_pr_refresh_test.go`.

## What we learned

- A check that only CI runs finds the fault after the lane has landed. Move the check to the build floor, where the lane can still fix it.
- A derived file with no declared inputs is never stale to the normalizer. Each catalog entry must declare its inputs.
- Do not weaken a guard to make main green. The macOS ratchet stayed a hard fail, because its cause was a missing `-test-timeout`, not the guard.
