# Build Explanation — Cycle 2474

## Build Binding
- Cycle: 2474
- Base SHA: de22fd9eeaecac66f9b14ffb7631521b3ac0c4d5

## Summary
Git output is now read with NUL separators. Names with spaces or non-ASCII characters stay exact.

## Rationale
Git quotes non-ASCII names in line output. The old code also trimmed spaces that belong to names. The `-z` flag removes both problems with no custom parser.

## Changed Areas
- `go/internal/core/boot_preflight.go` — `porcelainPaths` reads `git status --porcelain -z` and skips the source entry of a rename or copy. The quote-stripping helper is removed.
- `go/internal/repostate/tracked.go` — `TrackedTree` reads `git ls-files -z` and keeps names unchanged.
- `go/internal/repostate/tracked_names_test.go` — tests that the tracked-tree reader keeps edge-space and non-ASCII names.
- `go/acs/cycle2474/predicates_test.go` — acceptance predicates for the quarantine and tracked-tree behavior.
- `.evolve/evals/porcelain-paths-nul-and-tracked-tree-spaces.md` — eval graders for this task.

## Design Decisions
Both readers use `-z`, the documented stable format, so no unquoting code is needed.

## Verification
The cycle 2474 predicates, the repostate tests and the core tests pass.

## Compatibility
The public function signatures are unchanged.

## Limitations
A rename source path is not quarantined separately; only the destination is.
