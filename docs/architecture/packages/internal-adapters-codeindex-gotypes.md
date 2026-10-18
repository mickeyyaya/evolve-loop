# internal/adapters/codeindex/gotypes

> The spec: section 9 of [code-index-cli](../code-index-cli.md). The plan: component G1 and decisions D1, D2, D4 and D5 of [the code-index backends plan](../../plans/code-index-backends-2026-10.md). The decision: [ADR-0140](../adr/0140-the-type-checker-answers-callers-and-each-query-goes-to-its-best-measured-backend.md). The layer that it implements: [internal-codeindex](internal-codeindex.md).

## Purpose

`internal/adapters/codeindex/gotypes` type-checks a Go module with the standard library only, and writes a compact index of it. It implements `codeindex.Backend` over that index. It serves `callers` and `api`. `find` and `map` give `ErrUnsupported`, so the route sends them to trail-code.

## Design

- **`Build(moduleDir, out)` writes the index.** It runs `go list -buildvcs=false -json -deps -test ./...` through a `sysexec.RunFunc`. The environment is the given base environment plus `CGO_ENABLED=0`. It parses each listed file with `go/parser` and type-checks each package with `go/types`, in dependency order. A standard-library package is checked without its function bodies. The test variants are checked too, so the index has the test files.
- **Each module package is indexed right after its check.** The set of module paths comes from the `go list` answer before the first check. After a package goes into the index, its syntax trees and its `types.Info` are dropped. On 2026-10-17 this cut the peak memory from 919–926 MB to 434–461 MB (two runs of each, load average 20 to 38).
- **The importer is the loader itself.** The importer of a package reads its `ImportMap` and checks the mapped package from the same `go list` answer. We do not use `go/importer` "source": it reads the global `build.Default` and can run `go tool cgo` outside the injected runner. The truth program of research F2.13 uses the same loader, so the index and the truth agree by construction.
- **The index is one gob file, written atomically.** `atomicwrite.Bytes` writes it, so a reader sees the old index or the new one. The file holds the files, the function definitions, the module call edges, the top-level declarations of each module file, and the interface links. If `go list` fails, or the index does not encode or write, `Build` returns an error and writes nothing. The last good index then stays.
- **A broken package is a warning, not a failure.** `types.Config.Error` collects the type errors, and the partial package goes into the index. A file that does not parse is left out of its package. `Build` prints one `gotypes: WARN <package>: N error(s), the first: ...` line for each broken module package, and the index keeps the list. The errors of a standard-library package are dropped: they are not ours to fix, and a body that is not checked gives no error.
- **What the index keeps.** It keeps each call to a module function and each call through an interface method of any package. It also keeps each call through a function value, with the name of the variable. It drops a static call to a function outside the module, as the prototype of research F2.20 did.
- **Package-level initializers.** A call in the value of a package-level `var`, also inside a function literal such as `var h = func() int { return g() }`, has the caller `<package>.<init>`. On this repository, `<init>` callers make 184 call edges.
- **Names (D4).** A definition answers to its bare name, to `Type.Method` and to the package-qualified name, such as `router.Digest` or `loopwave.Launcher.Run`. Each hit's `Text` starts with the qualified name of its definition. So a bare name with two definitions gives hits that the agent can tell apart.
- **Kinds (D5).** The second word of `Text` is the kind: `static`, `interface` or `func-value`. A concrete method with no static caller gets one hit for each interface method that it implements, from `go/types.Implements`. For example: `audit.fakeBridge.Launch implements core.Bridge.Launch: 6 call(s) through it`.
- **The fields of a hit.**

  | Query | `File:Line` | `Symbol` | `Text` |
  |---|---|---|---|
  | `callers --direction in` | the call site | the qualified caller | `<definition> <kind>` |
  | `callers --direction out` | the call site | the qualified callee, or the variable of a function value | `<definition> <kind>` |
  | the implements line | the concrete method | the interface method | `<method> implements <interface method>: N call(s) through it` |
  | `api` | the declaration | the name, or `Type.Method` | `func`, `method`, `type`, `const` or `var` |

- **Paths.** A path is relative to the nearest directory above the module that holds `.git`, so `go/cmd/evolve/cmd_code.go` matches the paths of trail-code. With no `.git` above it, a path is relative to the module directory.
- **Errors map to the typed errors.** No index file is `ErrAbsent`, with the path. An index of another format version is `ErrAbsent`, because the boundary must rebuild it. An unknown symbol, a definition with no call and a file with no declaration are `ErrNotFound`. A file that does not decode is a plain error with the path.
- **The index path.** `IndexPath(indexDir)` gives `<indexDir>-gotypes/index.gob`. For the default `dev/code-index`, that is `dev/code-index-gotypes/index.gob`, beside the index worktree and outside every git tree. G2 writes the index there.

## Measurement

On 2026-10-17, `Build` on `dev/code-index/go` took 3.7–8.2 s, with a peak memory of 434–461 MB, and printed no warning. The index is 5.4 MB. `evolve code callers` took 0.034–0.037 s at p50 for 5 symbols, through the route `["gotypes", "trail"]`. The load average was 30 to 38, so the times are noisy.

`router.Digest` gave 62 sites, the same count as the truth of research F2.15. For `core.Bridge.Launch`, the index has 6 interface calls, and research F2.22 counted 5 for `fakeBridge.Launch`. The sixth site is `go/internal/core/apicover_ports_test.go:58`. It is not new: `git log -S` dates it to commit `889144023` of 2026-06-17, and it is in the research copy.

The truth program missed it, because the site is in the test variant `core [core.test]`, and `go/types` sees that variant as a second package. So the `Bridge` type at the site is not identical to the `Bridge` type that the concrete method implements, and `types.Implements` is false. The index keys a definition by its source position, so both variants are one definition. The right number is 6.

## Limits

- The index is Go only, and static only. A call through reflection is not in it (research F2.14).
- The index has the files of the host platform only. `CGO_ENABLED=0` leaves out the cgo files, and a build constraint for another `GOOS` or `GOARCH` leaves out its files.
- A method value or a function reference that is not a call, such as `b.Apply(Seed)` for `Seed`, is not counted. Only the call `b.Apply` is.
- A call through an interface counts for the interface method. The index does not give the concrete target of that call (plan Q4).
- A static call to a function outside the module is not in the index. So `callers --direction out` does not list a call such as `fmt.Sprintf`.
- A method of an unnamed interface type has the type name `interface`, for example `b.interface.Close`.
- `go/types.Implements` compares types by identity. A test variant of a package is a second package, so a link across a test variant can be missing.
- The index shows the checkout that `Build` read. The boundary builds it at `main` (G2).

## Invariants

- The adapter passes the shared conformance suite on a fixture module of three packages. Pinned by `TestGotypesBackend_PassesTheConformanceSuite`.
- A qualified name gives only the callers of its own definition. Pinned by `TestGotypesBackend_AQualifiedNameGivesOnlyTheCallersOfItsOwnDefinition`.
- A bare name names the definition of each hit. Pinned by `TestGotypesBackend_ABareNameWithTwoDefinitionsNamesTheDefinitionOfEachHit`.
- Each hit names its kind. Pinned by `TestGotypesBackend_EachHitNamesItsKind`.
- A concrete method that only an interface calls gets its implements line. Pinned by `TestGotypesBackend_AConcreteMethodReachedOnlyThroughAnInterfaceGetsItsImplementsLine`.
- A failed build writes nothing. Pinned by `TestBuild_FailsLoudly`.
- A broken package warns once, and the other packages are still indexed. Pinned by `TestBuild_ABrokenPackageWarnsOnceAndTheOtherPackagesAreStillIndexed`. A type error of a standard-library package is not a warning. Pinned by `TestBuild_ATypeErrorInAStandardPackageIsNotAWarning`.
- A call in a package-level initializer has the caller `<package>.<init>`. Pinned by `TestGotypesBackend_CallsInPackageLevelInitializersBelongToTheInitCallerOfThePackage`.
- Every export is named by a test. The package is enrolled in `go/.apicover-enforce` and held at 100% statement coverage in `go/.cover-strict`.

## Wiring

`buildCodeBackends` in `go/cmd/evolve/cmd_code.go` registers the name `gotypes`. It creates the adapter on `gotypes.IndexPath` of the policy key `code_index.index_dir`. The default of `code_index.backends` stays `["trail"]` until G4.
