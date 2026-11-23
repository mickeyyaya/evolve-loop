# internal/gitstarts

## Purpose

`internal/gitstarts` counts and checks git process starts in Go test files.

## Design

- `Count` parses every `*_test.go` file in one directory with Go AST.
- `Count` skips production files and nested subdirectories.
- `Count` assigns one weight per matching call expression so one call counts once:
  - `gittest.Fixture`, `gittest.Bare`, and `gittest.At` cost 5 (one init and four config starts).
  - `gittest.Clone` costs 1.
  - `gittest.FromTemplate` and `gittest.BareFromTemplate` cost 0 because they copy a cached tree.
  - Any `<expr>.Git(...)` call costs 1.
  - Any other call with a string literal argument `"git"` costs 1.
- `Verify` reads a JSON file with `{"recorded": N}`.
- `Verify` returns an error if the file is missing, invalid, negative, or lacks the `recorded` field.
- `Verify` fails if `Count` exceeds `recorded`.
- A static AST count replaces a full-binary subprocess measurement because it runs in under one second without timing flakiness.

## Invariants

- `Count` returns an error when the directory is missing or holds invalid Go syntax.
- `Verify` refuses a broken baseline so a missing or malformed file cannot pass silently.
- `./internal/gitstarts` is enrolled in `go/.apicover-enforce` with `apicover_named_test.go`.

## Findings

- None open.
