# Build Explanation — Cycle 1860

## Build Binding
- Cycle: 1860
- Base SHA: ba30cf5c2553d86834db0a087a8b2a79d53af79b

## Summary
Updates `steDisplay` to keep root files prefixed with `..` relative, and updates `formatVerb` to parse argument index before `*` precision.

## Rationale
In `go/cmd/evolve/cmd_docs.go`, `steDisplay` used `!strings.HasPrefix(rel, "..")`. That check marked root files whose basename begins with `..` as outside the project root. In `go/internal/stelint/scan.go`, `formatVerb` omitted argument index skipping after a precision dot, misclassifying verbs such as `%[3]*.[2]*[1]f` as prose words.

## Changed Areas
- `go/cmd/evolve/cmd_docs.go` — updates `steDisplay` to treat relative paths as outside only when equal to `..` or beginning with `../`.
- `go/cmd/evolve/cmd_docs_test.go` — adds regression and preservation tests for `steDisplay` and the `ste-lint` command.
- `go/internal/stelint/scan.go` — updates `formatVerb` to parse argument indices after precision dots before width runes.
- `go/internal/stelint/gocheck_test.go` — adds tests verifying indexed width and precision verbs scan as single code tokens.
- `.evolve/evals/stelint-two-scanner-defects.md` — records score caps and evidence commands for both scanner fixes.

## Design Decisions
Path relativity checks `slashed != ".." && !strings.HasPrefix(slashed, "../")` on `filepath.ToSlash(rel)` to handle separators cleanly. The format verb scanner invokes `s.skipArgIndex(j+1, to)` after finding a dot before scanning width runes.

## Verification
`evolve acs suite --cycle 1860` passes (170/170 PASS). Targeted unit tests in `cmd_docs_test.go` and `gocheck_test.go` pass.

## Compatibility
The change preserves existing behavior and fixes path formatting and format verb parsing bugs.

## Limitations
The update applies to `steDisplay` relative path calculation and `formatVerb` argument index scanning.
