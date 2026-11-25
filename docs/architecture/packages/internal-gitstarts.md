# internal/gitstarts

## Purpose

`internal/gitstarts` counts and checks git process starts in Go test files.

## Design

- `Count` parses every `*_test.go` file in one directory with Go AST.
- `Count` skips production files and nested subdirectories.
- `Count` reads `gittest.StartCosts` once and charges each call expression from that table:
  - Package calls `gittest.<Name>(...)` read `table[<Name>]`.
  - Selector calls `<expr>.<Method>(...)` on non-`gittest` receivers read `table["Repo."+<Method>]`.
  - Other calls with a `"git"` string literal argument cost `1`.
- `MissingCosts` parses every non-test `.go` file in one directory and reports exported functions or methods that start git without an entry in `gittest.StartCosts`.
- `Verify` reads a JSON file with `{"recorded": N}`.
- `Verify` returns an error if the file is absent, invalid, negative, or lacks the `recorded` field.
- `Verify` fails if `Count` exceeds `recorded`.
- A static AST count replaces a full-binary subprocess measurement because it runs in under one second without timing flakiness.

## Invariants

- `Count` and `MissingCosts` return an error when the directory does not exist or holds invalid Go syntax.
- `callCost` holds no helper-name switch; `gittest.StartCosts` is the single cost table.
- `Verify` refuses a broken baseline so an absent or malformed file cannot pass silently.
- `./internal/gitstarts` is enrolled in `go/.apicover-enforce` with `apicover_named_test.go`.

## Findings

- None open.
