# internal/gatehermetic

> The design: sections 7.4, 7.5, 7.7 and 7.8 of [gate-contract-g2-hermetic-guard](../gate-contract-g2-hermetic-guard.md), commit 3 (ADR-0165). It builds on [internal-goload](internal-goload.md) and [internal-gotool](internal-gotool.md).

## Purpose

`internal/gatehermetic` is the call-graph hermeticity guard. It walks the static call graph from the run function of a gate. It refuses each reach into git, a process, the clock of a file or the history of a repository. The operator ruled on 2026-11-11 that the walker comes first, as a measuring tool, before any gate moves.

## Design

- **`Contract`.** One table holds the refused list, the exec entries, the inert list and the closed allowed list. It is protected data, not an operator setting.
- **`RootOf`.** It reads the runtime name of a function value. It refuses a closure and a method, because their names are not stable.
- **`Load`.** It runs `goload` on the packages of the roots. A failed load, a type error and an undeclared root are each an error, never a pass. `goload` keeps only `Defs` and `Uses` for a module package, so the guard type-checks each package outside the standard library again, with `Types` and `Instances`. A module package reuses the syntax of `goload`. A third-party package is parsed with `go/build.ImportDir`, because `goload` keeps no syntax for it.
- **`Refusals`.** It walks breadth first from each root. A refusal holds the root, the class, the site and the chain. Each step of the chain has an edge kind: `call`, `value`, `live-type` or `init`.
- **The reach rules.** A body reaches each function it uses, as a call or as a value. A module type is live only when a reached body converts a value of it, or a pointer to it, to an interface type. A body that only names a type does not make it live. A live type reaches all its methods.
- **The structure of a live type.** The types in its fields and elements are live also.
- **Packages and initializers.** A reached package reaches its `init` functions and the calls of its initializers.
  - A function literal that an initializer calls is scanned, with the rules of a normal body.
  - A function value that an initializer passes as an argument of a call is a `value` edge.
  - A function value that an initializer only stores is reached only when a reached body reads that variable.
- **Generic calls.** Each type argument of a generic function or type is a conversion. Its methods are reached through a `live-type` edge.
- **Tuples.** A call that returns many values pairs each value with the type of its parameter, result or left-hand side. A map index converts the index to the key type, on each side of an assignment.
- **A bare reference to an exec entry** (for example `f := exec.Command`) is class `exec`. It passes only when it flows into `sysexec.RunFunc`: an assignment, a typed `var`, an argument, a result, a field or a conversion to that type.
- **A call through a value.** A call through any variable, field, index or call result whose type is identical to the signature of `sysexec.RunFunc` is judged as a `RunFunc` call.
  - A call through a type parameter is judged also, when one type of the type set of the constraint is identical to that signature. A union and a nested constraint count.
- **A boxed `RunFunc`.** A value of that signature that is converted to an interface type is class `exec`. The standard library can call it with `reflect.Value.Call`, and the guard does not walk the standard library.
- **The standard library is derived, not listed.** `Load` computes the import closure of each standard package from the program that `goload` returns. A call or a reference into a standard package whose closure holds `os/exec` is class `exec`. Three lists limit this rule:
  - `Exec` holds the entries that need a `Param` rule.
  - `Inert` holds the functions that start no process. For example `go/build.ImportDir` and `MatchFile`.
  - `ExecStd` holds the packages that a person has judged. A test lists the importable standard packages that reach `os/exec`. The test fails for a package that no list names, and for an `ExecStd` entry that no longer reaches `os/exec`. A new Go release that adds an exec path thus turns the test red.
- **The classes.** `git`, `exec`, `git-dir`, `mtime`, `history` and `inject`. A call is `git` when the program argument is the constant `git`. A reached package with an excluded source file or a function with no body is `exec`. A method `ModTime() time.Time` or `Sys() any` of any receiver is `mtime`.
- **The history class** holds `runtime/debug.ReadBuildInfo`, `debug/buildinfo.ReadFile` and `debug/buildinfo.Read`.
- **Constants.** The guard folds string constants without a type map. It handles literals, named constants and `+`.
- **`Audit`.** It judges each body of one package for `mtime`, `git-dir`, `history` and the git words of the contract. It does not walk.
- **`ToolchainAudit`.** It judges `gotool` by five rules:
  1. One function holds the only exec call, with the constant program `go`.
  2. Each call of that function passes a constant verb from the allowed list.
  3. No body uses that function as a value.
  4. The package imports only the standard library and `sysexec`.
  5. No other body uses `Path` or `Args` of `exec.Cmd`.

  The function of rule 1 is keyed by package, receiver and name. A method with the same name is not that function. A literal or a `new` of `exec.Cmd` is an exec site for rule 1.

## Invariants

- **The package imports only the standard library, `goload` and `sysexec`.**
- **The package is a protected surface** (`/go/internal/gatehermetic/` in `guards.ProtectedSurfaceManifest`).
- **Enforced at 100%** statement coverage of its own tests.
- **A fixture module is made at run time.** Test helpers write it into a temporary directory. It declares the real module path and has stub `sysexec`, `repostate` and `gotool` packages.

## Findings

- **A tagged test file is not an excluded source file.** `go/build` lists a test file with a build constraint among the ignored files. The first measure on the real tree refused 30 packages for this reason. The guard now skips every `_test.go` name.
- **The three floor entries are closures.** `core.FloorEntryRun` returns a closure, so `RootOf` refuses it. Commit 5 gives each floor entry a named run function.
- **The unreachable cases.** A literal of a slice of `exec.Cmd` is not judged. Review checks it.

- **The probe of 7.13 (go1.27.2, darwin).**
  - `net/http/cgi.Handler{Path: ...}.ServeHTTP` starts the program of `Path`. VERIFIED: a script wrote a marker file.
  - `go/importer.ForCompiler(fset, "source", nil).ImportFrom` of a module-local path did not start `go list`, with a fake `go` first in `PATH`. NOT reproduced. The derived rule refuses the package anyway, because its closure holds `os/exec` through `go/build`.
  - `net/http/fcgi` reaches `os/exec` only through `net/http/cgi`. It has no exec path of its own. The derived rule refuses it, and `ExecStd` records the judgement. `fcgi.ProcessEnv` is inert.
- **A qualified write is a read.** `pkg.V = 1` counts as a read of `V`, because the plain-write test sees only an unqualified name. The guard fails closed. A dot import is the form that the plain-write test covers.
- **The re-measure of the real registry** (four stand-in roots: `UnitTestFloorJudge`, `CoverChangedFloorJudge`, `FloorEntryRun`, `RunRepoContractPack`) gives 84 sites. That is the count before the derived rule, the boxing rule and the type-parameter rule. The diff of root, class and site is empty.

## Accepted design limits

- The `git-dir` check folds constants only. A path that a body builds from non-constant parts passes.
- A value of a type from an exec-reaching standard package that is converted to an interface is not judged. Only a call or a reference to a function of that package is.
- `fmt` printing a `FileInfo` is not judged, because the guard never walks the standard library.
- `holdsCode` refuses too much: an assigned variable whose type holds an interface is class `inject`. The guard fails closed.
- `ModTime` and `Sys` are matched by name and signature on any receiver, so an unrelated method of that shape is also `mtime`.

## Known false passes (the walker is a measuring tool)

The operator ruled on 2026-11-11 that this package lands as a measuring tool. Three review rounds of commit 3 each found new false passes, and the convergence rule stops a fourth round. Nothing enforces the walk yet: test 10 comes in commit 6. Before commit 6, a design pass makes the walk fail closed at its source. These false passes of round 3 are open:

- A boxed value of a type from an exec-reaching standard package passes. An example is `var h http.Handler = &cgi.Handler{Path: "git"}`, because `net/http` does not reach `os/exec` (`agent-architecture-reviewer-20261111T052136Z-07ff`).
- A function value inside a composite literal that is an argument of an initializer call passes, for example `var _ = apply(Opts{Fn: gitFn})` (`agent-code-reviewer-20261111T052136Z-e5b2`).
- A write to a code-holding package variable from a package outside the load closure is not seen. That package imports the gate helper, so `goload` never loads it (`agent-adversarial-review-20261111T052136Z-c1f5`).
