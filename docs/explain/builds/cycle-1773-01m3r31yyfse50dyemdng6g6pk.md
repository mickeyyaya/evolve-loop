# Build Explanation — Cycle 1773

## Build Binding
- Cycle: 1773
- Base SHA: 7207ba8d232e8a0be61766ad144a28cff3783568

## Summary
`evolve branches prune` now keeps two kinds of superseded `cycle-*` ref
before any PR check or `git branch -D`. The first is a ref checked out in any
worktree. The second is a ref that a continuation-registry binding names.
Each kept ref is reported with its own reason: `kept-checked-out`,
`kept-bound`, or `kept-delete-failed` when git refuses the delete for any
other reason. The walk continues past every kept ref. When a keep cannot be
decided, because `git worktree list` fails or the registry is unreadable, the
walk aborts before any delete.

## Rationale
The inbox item has two acceptances. First, a checked-out ref must not abort
the walk and must be reported `kept-checked-out`. Second, a
continuation-bound ref must never be pruned. The first build in this cycle
fixed only the abort. It kept any ref whose delete failed, but
`OrphanVerdict` carried no reason, so the CLI's fallback labeled that ref
`kept-open-pr`. It also never read the registry, so a bound ref with no open
PR was still deleted. That deletes resumable work (audit H1/H2).

The fix decides each keep from ground truth before any mutation. Checked-out
status comes from `git worktree list --porcelain`, the same list git consults
when it refuses the delete. Bindings come from
`continuation.ListRegistryEntries`, the registry's own reader. Deciding up
front means a checked-out ref never reaches `git branch -D`. It also means a
failed delete is never mistaken for a checked-out ref, which was audit M1.

## Changed Areas
- `go/internal/core/prune_superseded_orphans.go` — adds the `KeptCheckedOut`,
  `KeptBound` and `KeptDeleteFailed` verdict fields. The walk now loads the
  checked-out and bound branch sets before its first ref, and either error
  aborts it. `keepOrPrune` applies the keeps in the order checked-out, then
  bound, then open PR, then delete, and reports a refused delete on its ref.
- `go/internal/core/prune_superseded_orphans_test.go` — TDD-authored
  scripted-git tests for each keep reason, the exact-name worktree match, the
  single-reason rule, and fail-closed aborts on a worktree-list failure, a
  corrupt registry, and a hasOpenPR error.
- `go/cmd/evolve/cmd_branches.go` — `runBranchesPrune` prints
  `kept-checked-out`, `kept-bound` and `kept-delete-failed` before the
  dry-run and open-PR fallbacks, so neither fallback label covers a kept ref.
- `go/cmd/evolve/cmd_branches_test.go` — TDD-authored real-git CLI tests. A
  linked worktree, a registry binding, and a held ref lock produce each
  label in both force and dry-run modes.
- `go/acs/cycle1773/predicates_test.go` — the ACS predicates that run those
  named tests as this cycle's gate.
- `.evolve/evals/branches-prune-aborts-on-checked-out-branch.md` — adds the task's eval spec so the auditor grades both inbox acceptances and the per-ref kept reason.
- `docs/architecture/packages/internal-core.md` — records the keep order and
  the fail-closed rule on the package page.

## Design Decisions
- Keep reasons are separate booleans with at most one set. The CLI label
  derives from them, and the zero state keeps its old meaning (kept behind an
  open PR, or would-prune in dry-run). A string reason field was the
  alternative. It would change the struct's comparison shape for every
  existing test, which buys nothing with only three reasons.
- The checked-out check reads `branch refs/heads/<name>` lines with an exact
  match, so `cycle-1` never matches a worktree on `cycle-10`. A small parser
  in `core` was chosen over exporting `gc`'s private porcelain parser.
  Exporting it would add a package dependency for five lines.
- A refused delete is kept and reported rather than aborting the walk. Git's
  stderr is still forwarded by `gitCapture`. A failure to run git at all
  still aborts.

## Verification
`go test -count=1 ./internal/core/... ./cmd/evolve/... ./internal/continuation/...`:
all packages pass. That includes 13 `TestPruneSupersededOrphans_*` tests and 7
`TestBranches*` tests against real git. Native ACS suite for cycle 1773:
`verdict=PASS red=0`, with all six `TestC1773_*` predicates green.

## Compatibility
The `PruneSupersededOrphans` signature is unchanged. `OrphanVerdict` gains
three fields, and existing field meanings are preserved. `evolve branches
audit` output is unchanged. A corrupt continuation registry or a failing
`git worktree list` now makes both `audit` and `prune` exit 1 rather than
proceed.

## Limitations
- `prune` still exits 0 when a delete is refused. The refusal appears only as
  `kept-delete-failed` on that ref's line and in git's stderr.
- A binding is matched by branch name alone. It is not checked against its
  snapshot SHA or cycle.
- The walk loads the worktree list and the registry once, so a worktree
  created or a binding written during the walk is not seen. Git still refuses
  to delete a ref checked out mid-walk, and that ref is reported
  `kept-delete-failed`.
