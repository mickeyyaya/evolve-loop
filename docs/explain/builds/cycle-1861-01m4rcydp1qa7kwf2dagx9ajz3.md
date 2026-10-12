# Build Explanation — Cycle 1861

## Build Binding
- Cycle: 1861
- Base SHA: 73c878259726a754474200846f21eb60fa98d0c1

## Summary
Updates `deleteBranch` to execute `git branch -d` with `LC_ALL=C` so squash-landed branches delete under non-English locales.

## Rationale
In `go/internal/gc/worktrees.go`, `deleteBranch` executes `git branch -d` and inspects standard error for `not fully merged`. Under non-English locales, Git translates this message. As a result, squash-landed branches remain unmerged and stay undeleted. The environment override forces standard POSIX English messages.

## Changed Areas
- `go/internal/gc/worktrees.go` — passes `LC_ALL=C` in the environment when `deleteBranch` executes `git branch -d`.

## Design Decisions
A helper filters any existing `LC_ALL` entry from `os.Environ()` and appends `LC_ALL=C`. `gitCapture` accepts the environment slice and forwards it to `o.Exec`.

## Verification
`evolve acs suite --cycle 1861` passes. Unit tests `TestDeleteBranch_SquashLandedDeleteWithNonEnglishLocale` and `TestDeleteBranch_UnmergedBranchStaysRefusedUnderLocaleOverride` pass.

## Compatibility
Existing behavior remains preserved. Git error messages parse reliably across all system locales.

## Limitations
The environment override applies to `git branch -d` branch deletion in `deleteBranch`.
