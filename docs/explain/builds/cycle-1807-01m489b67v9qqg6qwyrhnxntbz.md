# Build Explanation — Cycle 1807

## Build Binding
- Cycle: 1807
- Base SHA: c03f7d4bb0294a4d7dd6c02a64b2dbd7d61351b9

## Summary
This build adds two operator verbs. `evolve backups verify [--dir D]` proves that every head of every ref bundle and every patch in the backups directory already exists elsewhere before anyone deletes the backups. `evolve release-promote <tag> [--rerun]` re-promotes a demoted release by clearing its prerelease flag, but only after the release workflow run is green and every asset is present.

## Rationale
Both procedures were manual `git` and `gh api` sequences, so a skipped check could lose unmerged work or promote a broken release. Putting each check in a verb makes the refusal deterministic and testable. The git classification lives in `internal/gitexec`, next to the other git helpers, so the command layer holds only orchestration and output. Both verbs are read-only until their own checks pass.

## Changed Areas
- `go/internal/gitexec/backup.go` — adds bundle-head listing, head classification (`IN_MAIN` when `origin/main` contains the head, `OTHER_BRANCH` when another remote-tracking branch does, `ONLY_IN_BACKUP` otherwise) and a patch-applied probe that reverse-checks the patch against a scratch index read from `origin/main`, so neither the checkout nor the real index can make an unlanded patch look applied.
- `go/internal/gitexec/backup_test.go` — covers the classification of heads and patches against real repositories, including a patch committed locally but absent from `origin/main`.
- `go/cmd/evolve/cmd_backups.go` — the `backups verify` verb: walks the directory, prints one verdict per head and patch, exits 0 when safe, 1 on any unsafe item (named on stderr) and 2 on I/O or git failure.
- `go/cmd/evolve/cmd_release_promote.go` — the `release-promote` verb: optional `gh run rerun --failed`, then a green-run and full-asset check, then the single PATCH that clears the prerelease flag; exit 1 refuses, 2 is a gh or usage error. The repository is bound with `-R` on `gh run` calls only, because `gh api` has no repository flag and names the repository in its endpoint.
- `go/cmd/evolve/registry.go` — registers `release-promote` and `backups` beside the other release commands.
- `go/cmd/evolve/main.go` — lists both verbs in the top-level usage.
- `docs/operations/runtime-reference.md` — replaces the hand-written `gh api` re-promotion with the verb and its exit codes.
- `docs/operations/workspace-layout.md` — documents `backups verify` next to the backups directory.
- `go/acs/cycle1807/predicates_test.go` — the TDD-authored predicates for both tasks.
- `go/acs/cycle1807/helpers_test.go` — the TDD-authored helpers for those predicates.
- `.evolve/evals/cli-backups-verify.md` — pins the backups acceptance to the cycle 1807 predicates.
- `.evolve/evals/cli-release-promote.md` — pins the promotion acceptance to the cycle 1807 predicates.

## Design Decisions
Exit 1 means the check refused and exit 2 means the check could not run, so an I/O failure can never be read as a refusal or a pass. The rerun happens before the green check and before the patch, and a still-red rerun refuses without patching. Usage errors exit 2 before any gh call. A patch counts as applied only when its reverse applies to the `origin/main` tree in a throwaway index, so the checkout, the real index and the refs stay untouched, and a missing `origin/main` is an exit-2 error rather than a safe verdict.

## Verification
The cycle 1807 predicates cover each exit code, the one-unsafe-item-among-safe-ones case, the empty directory, the rerun ordering, the no-patch-on-refusal rule, a fake gh that rejects `-R` on `gh api` as real gh does, and patches present only in the checkout, only on the local branch, or only on `origin/main`. The gitexec tests run the classifiers against real repositories.

## Compatibility
Both verbs are additive; no existing command, flag or exit code changes. The documentation edit replaces one manual command with the verb.

## Limitations
`backups verify` treats a head as safe when `origin/main` or any remote-tracking branch contains it, but a patch only when `origin/main` contains it; it reads the remote-tracking refs as last fetched and never fetches or deletes anything. `release-promote` trusts the release workflow's conclusion as reported by gh.
