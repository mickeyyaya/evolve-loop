# Ship Added-Test Build Constraints — 2026-08-26

Cycle 1566 closes a ship-gate gap where newly added failing Go tests could be
missed or falsely classified when guarded by build tags.

## Finding

Go's `build.Context.MatchFile` is the standard-library decision point for
whether a filename and its build constraints apply. The ship gate now groups
new test packages by a satisfiable tag set, runs each group with `go test
-tags`, records host-exclusive exclusions, and records staged-diff discovery
failures instead of silently treating them as a clean scan.

The search is deliberately capped at 12 distinct tags per file. Files beyond
that ceiling are explicitly excluded with a backstop warning; a constraint
solver is the upgrade path if real repository tests exceed the ceiling.

## Sources

- Go `go/build.Context.MatchFile`: https://pkg.go.dev/go/build#Context.MatchFile
- Git diff filtering: https://git-scm.com/docs/git-diff
- Production implementation: `go/internal/phases/ship/repocontract.go`
