# internal/codequality

## Purpose

`internal/codequality` runs deterministic source-format gates that mirror the `vet + fmt` step of CI. The audit of an autonomous cycle then catches gofmt regressions before ship, and does not leak them to CI.

## Design

- **Same gofmt as CI.** `UnformattedGoFiles` shells out to the gofmt binary that CI uses, with `-l` to list offending files and `-s` for the simplification that CI requires. The gate and CI cannot disagree. Non-Go files are ignored. An empty result means the tree is clean.
- **Normalizer.** `FormatGoFiles` applies `gofmt -w -s` to every `.go` file that is not clean. It returns the parseable files that it ran gofmt over. The return value is for reporting. The audit gofmt gate verifies the result, so it is not a clean guarantee.
- **Module directory.** `ModuleDir` resolves `<root>/go` when it exists, else the root. Formatting callers keep that fallback on inspection errors. `ResolveModuleDir` is the checked form: it resolves a nested module only when `go/go.mod` exists, and it preserves filesystem errors.

## Invariants

- **Safety decisions use `ResolveModuleDir`.** A support-only `go/` directory is not a module, and an unreadable path cannot be treated as absent.
- **A missing gofmt binary is an error.** The gate cannot run, so the error is returned and never read as clean. The caller fails open: it warns, and does not silently treat the tree as clean.
- **A parse error is an offender, not an infra error.** gofmt exits non-zero on a parse error and still lists the valid but dirty siblings on stdout. `UnformattedGoFiles` reports the unparseable file as an offender, so the audit fails it. It does not fail open (`TestUnformattedGoFiles_ParseErrorIsOffenderNotInfraError`).
- **An empty root stays empty.** `ModuleDir("")` returns `""`, so `FormatGoFiles` never hands gofmt an empty directory argument. Gofmt scans the working directory for such an argument.
- **An unparseable file is left untouched.** `gofmt -w` cannot fix code that does not parse. The audit gate rejects that file, because such code must never ship. A gofmt run that fails on one file still rewrites the parseable files.

## Findings

- **Cycles 339 to 341** shipped to main with a red CI because the cycle-scoped audit never ran gofmt over the generated `go/acs/cycle<N>/*.go` predicate files. This package closes that gap in process.
