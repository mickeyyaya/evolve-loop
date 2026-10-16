# internal/apicover

## Purpose

`internal/apicover` measures the coverage of the public API of a package. It lists every exported symbol with `go/ast` (standard library only). It then applies a two-signal check.

A test must name the symbol. `go tool cover -func` must show more than 0 percent for it. It reports the symbols that no test names and the "false greens" (named by a test but 0 percent executed). It only warns by default.

`-enforce` makes it exit with a non-zero code. Both `cmd/apicover` and the `evolve apicover` command call `Main`. The in-process API gate of the audit calls `Run`. One source gives all three the same measurement. No second first-party executable is built at run time.

## Design

- **Two signals, because one hides failures.** A bare line-coverage number hides a symbol that a test names but never executes. A function or method is covered only if a test names it and its executed coverage is above 0 percent. A type, variable or constant has no coverage signal, so it is covered if a test names it.
- **Names are matched loosely.** A method appears in the tests by its bare selector name. Without `go/types`, the test AST has no receiver type. `namedLookupKey` therefore looks up `T.Method` as `Method`. `NamesReferencedInTests` is deliberately broad and does not resolve types.
- **The cover join uses the import path, the file and the line.** `go tool cover -func` prints a method under its bare name. A key by name is not possible. The import-path-qualified path and line are exact and do not collide between packages that share a file name (`config.go`, `doc.go`). `packageImportPath` walks up to the enclosing `go.mod`, reads the module line and appends the relative directory. `go/build` cannot resolve module import paths, and the package avoids `golang.org/x/tools`.
- **`ParseCoverFunc`** takes the function name and the percentage as the last two whitespace fields, and everything before them is the location. A path with spaces, common on developer machines, then parses. It skips the `total:` line and any malformed line instead of failing the whole report.
- **The measured surface is what an untagged `go test` compiles.** `Enumerate` skips `_test.go` files. It also skips each file that `build.Default.MatchFile` excludes. Those are the integration, e2e, acs-tagged and platform-specific files. A method on an unexported type is not public API.
- **Ignore directive.** A declaration is excluded with `//apicover:ignore reason=...` in its doc comment. `parseIgnore` is the only code that reads a comment as data. The reason is mandatory, so each suppression is justified in the source and can be audited. A directive with an empty reason is an error.
- `Enumerate` still buckets such a symbol as ignored, so it never shows as a false `UNCOVERED` line. It also returns the first error, so a caller can fail hard. `printIgnored` always lists every ignored symbol with its reason, so a suppression is never silent. No file of this package carries the directive. The fixtures in `testdata/` carry it, and they must keep it.
- **`HasDoc` backs `-require-doc`.** `MissingDoc` lists the exported symbols with no doc comment, except the ignored ones. It only reports. The exit code counts untested symbols, not missing docs.
- **The context bounds the walk.** `Run`, `Enumerate` and `NamesReferencedInTests` check the context at each directory and file boundary. Without it, a wedged AST walk (a pathological package or a hung file system) escaped the deadline of the audit gate. A cancelled context surfaces as the measurement error with exit code 2 and `ctx.Err()` in the chain. A caller can then tell an interrupted run from a real finding. The CLI itself has no deadline (the operator can interrupt it), so `Main` uses `context.Background()`.
- **Exit codes.** 0 is a warning-only run, or a clean run under `-enforce`. 1 is a run under `-enforce` with unresolved symbols. 2 is a flag error or a measurement error.
- The message goes to stderr. `-h` and `-help` exit with 0, because a help request is not an error, and the standalone binary keeps the behavior it had with `flag.CommandLine`. `Main` makes a flag set per call, so it is safe in process.
- **Diff-scoped enforcement.** `Config.ChangedFilesByDir` maps each directory to the base names of the files that changed. Violations in files outside the change are listed under a `PRE-EXISTING DEBT` header and do not count toward the `-enforce` exit code. A lane owns the hygiene of the files it touches, not the debt of other files. This came from four pre-existing false greens in the core, which blocked every lane that touched it. A nil map is the classic behavior, and the repo-wide run in CI is unchanged.
- **The cover file is read-only.** `Run` ignores the close error of the cover file. A failed close of a file that was opened for reading has no action.

## Invariants

- **The directive needs a reason.** `TestIgnoreDirective_EmptyReasonIsError`, `TestIgnoreDirective_MarksSymbolWithReason` and `TestEnumerate_MalformedIgnore_ReturnsError` pin the directive, with the `badignore` and `sample` fixtures.
- **Tagged files are skipped.** The `tagged_integration.go` fixture pins that the default measurement does not see it.
- **`Main` exit codes.** `TestMainEntry` pins all four (0, 1, 2 for a flag error, 2 for a measurement error) and the help case.
- **The context bounds the walk.** `TestRun_CancelledContext_ReturnsMeasurementError`, `TestEnumerate_CancelledContext_Stops` and `TestNamesReferencedInTests_CancelledContext_Stops` pin the cancellation.
- **Diff scope.** `TestDiffScope_PreExistingDebtWarnsNotFails` and `TestDiffScope_NilFilterKeepsClassicBehavior` pin that debt outside the change does not count.

## Findings

- None.
