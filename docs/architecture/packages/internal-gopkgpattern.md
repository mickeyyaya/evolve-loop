# internal/gopkgpattern

## Purpose

`internal/gopkgpattern` is the shared scope rule for `go test` package-pattern strings found in Go source. Two ACS predicate lints must never disagree about this rule.

## Design

- **Two callers.** `internal/acssuite` (`scopelint.go`) applies the rule at run time. It demotes a cycle predicate that sweeps packages outside the git-derived touched set of the cycle. `internal/evalqualitycheck` (`flakylint.go`) applies it at authoring time. It flags suite-scope shells before the predicate enters the corpus.
- **One copy.** Both callers once kept private copies of the rule. Two copies drift: a shape that the authoring lint stops recognizing also stops being demoted at run time, and the reverse. This leaf extracts the pure decision and leaves the policy in the callers.
- **No dependencies beyond the stdlib.** The authoring-time pre-flight must not pull in the suite runner (policy and verifylock). The run-time lint must not pull in the eval-quality CLI.
- **Scope.** This is not the only package-path normalizer in the repo. `internal/ciparity`, `internal/phases/audit` and `internal/changedpkgs` each normalize patterns for their own purpose, over git paths and `go list` output. Folding them in couples unrelated decisions. The contract here is the rule for string literals in agent-authored source.
- **Exports.** `ModulePrefix` is the import-path prefix of the module. `WholeModule` is the bare recursive sweep `./...`. Callers compare against the constant and do not re-spell it. `IsRecursive` is true for `./...` and any `<pkg>/...` suffix. `Key` maps a pattern to a module-relative directory.

## Invariants

- **Conservative matching.** `IsPackagePattern` accepts only unambiguous shapes: `./x`, `./x/y/...`, `./...` and module import paths under `ModulePrefix`. In both callers a false positive costs more than a false negative. It demotes a real gate at run time, and it prints a false claim to a predicate author at authoring time. File paths, prose with whitespace, URLs and bare words never match. The table test has explicit rows for each.
- **Recursive sweeps are not bounded by `-run`.** A recursive sweep builds and loads every package under it, so narrowing it with `-run` does not bound its cost. Callers treat it differently from a single known-slow package.
- **Key agrees across spellings.** `./internal/bridge/...`, `./internal/bridge` and the full import path all give `internal/bridge`. The bare `./...` names no directory and gives `""`, as do unrecognized shapes. Pinned by `TestKeyAgreesAcrossSpellings`. Without it, a touched-set match at run time and a known-slow match at authoring time silently disagree.
- **Recursive is a refinement of package pattern.** Every recursive shape is also a package pattern, so no recursive shape slips past a caller that filters on `IsPackagePattern` first. Pinned by `TestRecursivePatternsAreAlsoPackagePatterns`.
