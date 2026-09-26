# internal/reachabilityprobe

> The live gate, its seam and its fail-open rule: [runtime-reference.md](../../operations/runtime-reference.md) (Frozen-pin reachability gate row). The caller's side and the cycle-1238 wiring: [internal-cli-phasecmd](internal-cli-phasecmd.md) Findings. The TDD agent's obligation: `agents/evolve-tdd-engineer.md`. This page keeps the package-level detail those do not.

## Purpose

`internal/reachabilityprobe` answers one question deterministically: would a frozen structural-test pin force its production file to close an import cycle? Such a pin makes the acceptance criterion permanently unsatisfiable. It is called from `evolve phase verify tdd` (`FrozenTestFiles` then `CheckFrozenPins`) and from `evolve reachability check-pin` (`BuildImportGraph` then `CheckCallSite`).

## Design

- **Two layers.** `CheckCallSite` checks one known call site against a graph the caller supplies, with no toolchain call. The frozen-pin layer (`frozenpins.go`) derives the call sites from what the tdd phase produces and runs the whole path: deliverable, frozen test files, pins, import graph, violations.
- **The graph comes from the real toolchain.** `BuildImportGraph` shells out to `go list -deps -json` through `sysexec` and keeps each reached package's direct imports. `CheckFrozenPins` builds one graph per module root (`./...`) and memoizes it; a nil entry marks an underivable module, so a broken module costs one `go list`, not one per pin.
- **Chain search is breadth-first.** `findImportChain` returns the shortest path from the referenced package to the pinning package, both ends included. `Violation.Cycle` is that path.
- **What a pin is.** One source line that holds two Go string literals: a `.go` path (the production file the requirement lands in) and an `ident.Symbol(` call site. This is the `acsassert.FileContains(t, "go/internal/core/state.go", "storage.UpdateStateMap(")` idiom. The scanner reads string literals, not raw line text, so the surrounding assertion call never matches as a pin. The first literal of each kind on the line wins.
- **The pinning package is the pinned file's package**, not the test's own package, because the requirement lands in the production file. `packageOfFile` finds it by walking up from the file's directory to the nearest `go.mod`, stopping at the worktree root.
- **Frozen means `doNotModifyTests: true`.** `FrozenTestFiles` reads the first fenced `json` block in the deliverable that has a non-empty `testFiles`. If that handoff does not freeze its tests, it returns nil: an unfrozen test can still be edited, so its pins are not permanent commitments.
- **Extraction does not resolve.** `ExtractFrozenPins` reports `ReferencedPackage` as the identifier written in the pin. `CheckFrozenPins` resolves it, because only there is a real import graph available.
- **Resolution order in `resolvePackage`:** an exact import path, then a base-name match, then an import alias from the frozen test file.
  - Among base-name matches, the candidate sharing the longest prefix with the pinning package wins (its nearest neighbour in the module), and a tie breaks lexically.
  - Aliases come from the frozen test file's own import block. That is the only place such a binding can live: if the pinned production file imported the referenced package, the module would already be cyclic and `go list` would produce no graph. `go list -deps` ignores `_test.go` imports, so the module still lists cleanly.
  - Only aliased imports are collected. An unaliased import is already found by base-name matching. `_` and `.` bind no usable identifier and are never resolved. A test file that does not parse contributes no aliases.

## Invariants

- **Fail open on anything unprovable.** An unreadable deliverable is an error. An unreadable frozen file, a file outside any module, a `go list` failure, an unresolvable identifier, or a pinning package absent from the graph all yield no violation. Only a compiler-provable cycle is reported, because a false HALT here taxes every cycle. Pinned by `TestCheckFrozenPins_AliasUnresolvableFailsOpen`, `TestResolvePackage_BaseNameFallback/no_candidate_fails_open` and `TestCheckCallSite/pinning_package_absent_from_graph`.
- **A self-pin is a violation.** When the referenced and pinning packages are the same, `findImportChain` returns a one-element chain, so `CheckCallSite` reports it. Pinned by `TestCheckCallSite/self_referential_pin`.
- **An alias never displaces a real match.** The identifier compiles in the pinned production file's scope, not the test file's, so an alias in the test file is a hint about intent, not a binding. Consulted first, one alias line in an agent-authored test file could rebind `storage` to a harmless package and hide a real cycle, and an alias to a package outside the graph could block resolution altogether. Consulted last, an alias can only add reach, for identifiers such as `st` that match no base name. An alias to a package absent from the graph resolves nothing. Pinned by `TestResolvePackage_AliasNeverSuppressesBaseNameMatch`; the code keeps a one-line why.
- **The verdict is deterministic.** Go randomizes map iteration, so the lexical tie-break between equally scored candidates keeps the gate from blaming a different package on each run. Pinned by `TestResolvePackage_DeterministicLexicalTieBreak`.
- **Base-name matching is exact.** `stor` never matches `storage`. Pinned by `TestResolvePackage_BaseNameFallback/partial_base_name_is_not_a_match_fails_open`.

## Findings

- **cycle-644**: the root incident. A frozen structural test pinned `storage.UpdateStateMap(` inside a `core` file while `storage` already imported `core`. The acceptance criterion could never be met, and the build phase spent the whole cycle finding out.
- **cycle-1225**: the library (`ImportGraph`, `CallSite`, `Violation`, `CheckCallSite`) landed from inbox item `tdd-structural-test-reachability-probe`, together with the tdd agent's obligation to probe before freezing a pin.
- **cycle-1226**: `BuildImportGraph` replaced hand-built graphs with real `go list` output.
- **cycle-1238**: the probe only helped if the tdd agent remembered to call it, which is the same lapse cycle-644 showed. `FrozenTestFiles`, `ExtractFrozenPins` and `CheckFrozenPins` made the check deterministic inside `phase verify tdd`.
- **cycle-1246**: a pin spelled through an import alias (`st.UpdateStateMap(`) matched no base name and failed open, so a real cycle passed the gate. Alias resolution from the frozen test file fixed it; `frozenpins_test.go` is the permanent guard, since the cycle's ACS predicates leave with the cycle.
- **cycle-1248**: an audit found the resolver consulted aliases first and so could suppress a real match (see the alias invariant above). The base-name branch also had no tests; `resolvepackage_test.go` added them.
