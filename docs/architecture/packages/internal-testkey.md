# internal/testkey

> The design of record: [fleet-landing-at-scale-2026-10.md](../../plans/fleet-landing-at-scale-2026-10.md) §5.3 (the content key), decision D2 and D4, component S3. The decision: ADR-0134. The data-edge catalog: [fleet-landing-queue.md](../fleet-landing-queue.md) §7.4.

## Purpose

`internal/testkey` computes the content key of one package test run. Two runs with an equal key must give an equal result. A later scan can then skip a run when a green result of the same key exists in `internal/testverdict`.

`Compute` is a pure function. It takes its inputs as values. It reads no file, starts no process and holds no global state. Component S5 wires it in shadow (`internal/phases/ship/scanshadow.go`). It gives every run an unknown test log, so each key is a whole-tree key until a producer of test logs exists.

## Design

- **The inputs.** `Compute` hashes these inputs with SHA-256, in a canonical encoding:
  - the package under test, the Go version, the build tags and the test flags;
  - the build environment (`Toolchain.BuildEnv`), taken from `go env -json`: `GOOS`, `GOARCH`, `GOFLAGS`, `GOEXPERIMENT`, `CGO_ENABLED`, `CC`, `CXX`, `CGO_CFLAGS`, `CGO_CXXFLAGS`, `CGO_LDFLAGS`, `GOAMD64`, `GOARM`, `GO386`, `GOARM64` and `GOWASM`. `Compute` returns an error when one of them is absent;
  - the content ids of `go/go.mod` and `go/go.sum`;
  - the content id of each file that `internal/overlap` names for each package in the test closure;
  - the tree id of the `testdata/` directory of each such package. Each package must have an id. A package without `testdata/` gets an explicit empty-tree id. A missing id is an error;
  - the content id of each data root that a package in the closure reads (the catalog of §7.4);
  - the names of the environment variables that the last run read (from its test log), and the names on the declared list of child-process variables;
  - the value of each such variable, or the fact that it was not set.
- **The closure.** The closure is the package, its module-internal `Deps` and the packages that its tests import, each with its own `Deps`. `overlap.Module.Deps` has no test imports, so the caller gives them in `Input.TestImports`.
- **The ids are values.** The caller gives every content id (`Input.IDs`, `TestdataIDs`, `RootIDs`, `WholeTreeID`). They come from git objects, so the key costs object lookups and no file reads. A missing id is an error. The function never guesses.
- **The whole tree.** The key uses the id of the whole tree instead of the closure when one of these is true:
  - the test is a tree-reading test (`Input.TreeReading`, from `repocontract.TreeReadingTests()`);
  - a package in the closure has an unresolved data root (`Input.UnresolvedRoots`);
  - the test log is missing, malformed or holds a record that the parser does not know (`Log.Unknown`);
  - the test log holds a path that the key does not hold, other than a path under the run's own temp directories;
  - a symlink lies in a covered path (`Input.Symlinks`, from the git tree entries of mode 120000).
- **What the log holds.** `ParseLog` reads the record of `go test -test.testlogfile`: `getenv`, `open`, `stat` and `chdir`. A relative path resolves against the working directory of the test, and a `chdir` moves it. The key holds a path in these cases:
  - the path is a file of the closure, `go.mod` or `go.sum`;
  - the path is under a `testdata/` directory or under a data root;
  - the path is a parent directory of one of these paths and the record is a `stat` or a `chdir`. This case lets a test walk up to find the repository root. An `open` of such a directory lists its entries, and a new entry changes what the test sees but not the key. So an `open` of a parent directory uses the whole tree.
- **Two steps.** `Key.File` hashes everything except the values of the variables. `Key.Full` adds the values. A store finds the entries for `File`, then compares the recorded values with the current values. `Key.Env` holds the values for that comparison. `Key.Reads` holds what the log showed: the context (repository root, working directory, temp directories), the paths opened and if the log was unknown.
- **The key and its log agree.** `Key.Consistent(log)` is true only when the log gives the same reads as `Key.Reads`, and every variable that the log reads has an entry in `Key.Env`. An unknown log needs a whole-tree key. `Key.Valid()` recomputes `Full` from `File` and `Env`.
- **Flags.** `-shuffle` is refused unless it has a fixed seed or is `off`. A random order can give equal keys for different orders.
- **The encoding.** Each string is written with its length. Sets (tags, variable names, packages, files, roots) are sorted first. Test flags keep their order, because a flag and its value are two tokens.

## What invalidates a hit

A hit is valid only while all inputs are equal. The key changes in these cases:

- a file of the closure changes;
- a `testdata/` tree changes;
- a data root changes;
- a tag or a flag changes;
- a variable that the test reads, or that is on the declared list, changes or becomes set or unset;
- for a whole-tree key, any file of the tree changes.

A change in an unrelated package does not change the key.

## The integrity rule

A hit must never hide a red.

The key is the first half of that rule. It must hold every input that can change a result. It must use the whole tree when it cannot know the inputs. The second half is in `internal/testverdict`, which stores green results only. Each doubt in this package resolves to the whole tree. That gives a larger key and fewer hits.

## Known limits

- **A child process.** The test log shows the reads of the test process only. A child process reads files and variables that no log records. The declared list covers the variables. The files stay a limit. Component S5 counts each escape in shadow, and S6 cannot enforce while the count is not zero.
- **Production reads.** The catalog of §7.4 holds test reads only. A test reads `.evolve/*` through production loaders. The log catches such a read as a path that the key does not hold, and the key then uses the whole tree. A catalog of production reads is later work.
- **The log must be the log of the run.** The names of the variables come from the log. After each run, the caller computes the key again with the log of that run and stores the result under that key. `Key.Consistent` checks the pair.

  `testverdict` refuses a record, and ignores a stored record, when the key and the log disagree. A log from an older run, or from another lane, cannot find an entry that its own reads do not support. The store enforces this invariant. A stale log is not harmless.
- **Test imports.** `overlap.LoadModule` does not list test imports. A caller that gives a known log must add them (`TestImports` of `go list`) before it calls `Compute`. Component S5 gives an unknown log, so it gives a module map of one package and needs no test imports.
- **Absent build variables.** `go env -json` omits the variables of other platforms. On `darwin/arm64` it has no `GOAMD64`, and `Compute` refuses a build environment with a gap. The caller must give an empty value for each absent name. Component S5 does this in `goEnv`.
- **The data-edge catalog.** The catalog home (`internal/testimpact`, component Q5) does not exist yet. `Compute` takes the roots as `Input.DataRoots`, so Q5 needs no change in this package.
- **Hidden inputs.** The test log does not see these inputs:
  - an `os.Environ()` dump;
  - the environment that a child process inherits beyond `ChildVars`;
  - the clock, the network and random numbers;
  - the host name and the number of CPUs.

  A test that depends on them can hit with a stale green result.
- **Symlinks.** The caller must list the symlinks of the tree. A symlink in a covered path forces the whole tree. A symlink that the caller does not list is a gap.
- **Broad temp directories.** A path under `Input.TempDirs` is never an outside read. A temp directory that is too broad (for example `/tmp`) hides real outside reads.
- **Platform paths.** Temp directories must use the same spelling in `Input.TempDirs` and in the log. On macOS, `/var` and `/private/var` are two spellings of one directory.
