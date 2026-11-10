# Component G2: the call-graph hermeticity guard

> Plan row G2 of [gate-contract-2026-11.md](../plans/gate-contract-2026-11.md). The contract and the G1 design are in [gate-contract.md](gate-contract.md), and its section 7 links this page. The headings here keep the numbers 7.1 to 7.14. Inbox item `gate-contract-g2-hermetic-guard`. ADR-0165 decisions 3 and 4. The measures are from `92df3d291` (2026-11-10).

## Current State

### 7.1 What the tree holds today

| Fact | Where |
|---|---|
| `go.mod` holds `go 1.23`, `pgregory.net/rapid` and `gopkg.in/yaml.v3`. It holds no `golang.org/x/tools`. | `go/go.mod:3-7` |
| CI runs Go 1.23 for the unit jobs. The host toolchain is go1.27.2. | `.github/workflows/ci.yml:31`, `go.yml:30` |
| A stdlib-only typed loader exists: `go list -json -deps -test` plus `go/types`, with stdlib bodies skipped. | `internal/adapters/codeindex/gotypes/build.go:91-181` |
| That index keeps only calls into module code. It drops each stdlib call and each non-call function reference. | `gotypes/indexer.go:228-257` (line 237) |
| `repostate` has three exports. Each one runs `git ls-files`, which reads the index. | `internal/repostate/tracked.go:16,31,49` |
| `testdup.Baseline` holds `Date`. `Scan` takes `since`. | `internal/testdup/testdup.go:36-41` |
| `loadAcsAges` runs `ls-files`, `log --format=@%ct` and `diff HEAD`. It drops each error silently. | `testdup.go:87-122` |
| `isOld` walks an untracked `acs/` directory and compares `ModTime` with the date. | `testdup.go:124-139` |
| `ParseBaseline` requires a `date` line. `FormatBaseline` writes one. | `testdup.go:245-261` |
| `go/.test-dup-baseline` holds `count=334` and `date=2026-11-09`. | the file |
| `testdup.Scan` has three callers outside the package: the TDD floor, the real-tree ratchet test and the `cycle2231` predicates. | `core/tdd_predicate_floor.go:52`, `repocontract/testdup_ratchet_test.go:25`, `acs/cycle2231/predicates_test.go:55,80,97,135,139,166` |
| `TestC2231_001` pins seven `testdup` test names. | `acs/cycle2231/predicates_test.go:20-37` |
| The `fleet` preflight runs `git status --porcelain` through `sysexec.Command`. | `internal/fleet/preflight.go:13`, caller `internal/loopwave/dispatch.go:171` |
| Ship stages the lane with `git add -A`, so a file deleted from disk leaves the lane tree. | `internal/phases/ship/paths.go:57` |
| `floorBase` falls back to the word `HEAD` when a run has no base. | `internal/core/comment_floor.go:103-108` |
| The K10 guard refuses a production `exec.Command` outside `sysexec.Command`. | `internal/sysexec/command_guard_test.go` (manifest row) |
| The manifest is `guards.ProtectedSurfaceManifest`, with `{Fragment, Class, Rationale}` rows. | `internal/guards/integrity_surface.go:15,59,253` |

The `phasecoherence` git readers (the four files of the plan row):

| File:line | Read | Kind |
|---|---|---|
| `gitignore_birth_test.go:22` | `git ls-files -- .evolve/` | real-tree scanner |
| `gitignore_birth_test.go:74` | `git check-ignore -- <probes>` | real-tree scanner |
| `tracked_artifact_hygiene_test.go:62` | `git ls-files` | real-tree scanner |
| `unpaired_tracked_test.go:14,85` | `git init`, `config`, `add`, `commit` | raw fixture (in `rawgitratchet/baseline.json:90`) |
| `unpaired_tracked_edge_test.go:16` | `git init` | raw fixture (in `baseline.json:89`) |

The real-tree judges of `./internal/phasecoherence/...` are six tests: `TestRepoPersonaProfilePairing` (`unpaired_test.go:71`), `TestAuditorStripKeepsOperationalContract` (`persona_strip_operational_test.go:29`), `TestPersonaStripKeepsOperationalSentinels` (`:50`), `TestTrackedProfiles_RealTreePlantedDecoyNotBound` (`unpaired_tracked_test.go:56`), `TestTrackedCorpusDirsAllowNewBirths` (`gitignore_birth_test.go:11`) and `TestNoRuntimeArtifactsTracked` (`tracked_artifact_hygiene_test.go:12`).

The real-tree judges of `./internal/fleet/...` read no git. Five tests in `packagegraph_test.go` and ten calls in `partition_test.go` run `go list` on `../..` (`packagegraph.go:26`). `TestGlobalZoneFiles_MatchesDocumentedGlobalZone` reads `docs/architecture/packages/internal-fleet.md` (`packagegraph_test.go:76`).

The build-floor git reads, measured one level into the helper packages:

| File:line | Read | `repostate` call after G2 |
|---|---|---|
| `core/floor_lists_base.go:53-58` (and `floor_lists_regression.go:38`) | `show base:rel`, `rev-parse base^{commit}` | `ReadAtBase` |
| `core/comment_floor.go:27`, `core/size_floor.go:22` | `commentaudit.ReadAtBase` with `cat-file` and `show` (`commentaudit/across.go:61-70`) | `ReadAtBase` |
| `core/comment_floor.go:60` | the same, against the word `HEAD` | `ReadAtBase` against the lane base |
| `core/ste_floor.go:111,118` | `cat-file -e base:path`, `diff -U0 base -- path` | `ReadAtBase`, `DiffAgainstBase` |
| `covchanged/workspace.go:32,35` | `rev-parse base`, `merge-base base HEAD` | the base check of `repostate`; the fork point is `Base` |
| `covchanged/workspace.go:98,106` | `diff -U0 --relative base`, `ls-files --others` | `DiffAgainstBase`, `Untracked` |
| `covchanged/workspace.go:129-134` | `ModTime` of each source against the profile | open question 2 |
| `changedpkgs/changedfiles.go:22,35` | `diff --name-status base`, `ls-files --others` | `ChangedSince` |
| `explanationdocs/gitio.go:42,46,251,390`, `rebind.go:216` | names and contents against the base | `ChangedSince`, `ReadAtBase` |
| `explanationdocs/gitio.go:178,262`, `rebind.go:132,176`, `core/lost_landing_floor.go:76-96` | `HEAD`, ancestry and a second commit | none: open question 1 |

### 7.2 A measured prototype

A stdlib prototype (`go list -deps -json` plus `go/types`, stdlib bodies skipped) ran on the host with go1.27.2:

| Roots | Packages | `go list` | type check | walk | Reached functions |
|---|---|---|---|---|---|
| `testdup` | 72 | 46 ms | 90 ms | 0.06 ms | 60 |
| the three seeds | 89 | 48 ms | 100 ms | 0.1 ms | 110 |
| the seeds, `phasecoherence`, `fleet` | 292 | 132 ms | 307 ms | 0.4 ms | 231 |
| each export of `core` | 285 | 163 ms | 301 ms | 5 ms | 1762 (0.47 s wall, 218 MB) |

The prototype refused the current `testdup` at `testdup.go:117` (git) and at `testdup.go:133-134` (mtime). It also found `fleet/preflight.go:13`. The guard needs no new module.

## Requirements

| # | Requirement | Level |
|---|---|---|
| R1 | The guard reaches each function from each registered run function. Calls, function values, method values, methods of live types and package initializers count. | Must |
| R2 | It refuses a git process, an unknown program, a `.git` path constant and an mtime read, outside `repostate`. | Must |
| R3 | `repostate` reads no mtime, no `.git` path and no `HEAD`, history or date. It accepts only a commit id as `Base`. | Must |
| R4 | `testdup` loses `Date`, `since`, `loadAcsAges`, the `HEAD` diff and the mtime walk. | Must |
| R5 | An `acs/` directory is new when the lane tree changes it against `Base`. | Must |
| R6 | A gate on the tracked set judges a staged new file that `Base` does not hold. | Must |
| R7 | The floors, `fleet` and the `phasecoherence` scanners read git through `repostate`. | Must |
| R8 | `./internal/phasecoherence/...` and `./internal/fleet/...` leave `Unregistered()`. | Must |
| R9 | The guard, `repostate` and each new gate file join the manifest in the commit that adds them. | Must |
| R10 | The guard runs with the go1.23 toolchain and the modules of `go.mod`, in under 2 s. | Must |
| R11 | No duplicated loader logic. No comment. Functions under 50 lines. 100% coverage of each changed function. | Must |
| R12 | Each refusal names the root, the class, the site and the call chain. | Wanted |

## Design

### 7.3 Approaches

Weights: correctness of the static reach 0.35, fit with the tree 0.25, blast radius 0.20, simplicity 0.20. The score is 1 to 5.

| Approach | Reach | Fit | Blast | Simple | Score |
|---|---|---|---|---|---|
| A. A typed walk on the stdlib. The loader moves from `codeindex/gotypes` to a shared `goload` package. | 4 | 5 | 3 | 4 | 4.05 |
| B. Extend the `gotypes` index with stdlib calls and references, and read the index | 3 | 2 | 3 | 3 | 2.75 |
| C. `golang.org/x/tools` SSA with a CHA or VTA call graph | 5 | 2 | 2 | 2 | 3.05 |
| D. Refuse by import closure: no gate imports `gitexec` or runs git anywhere in its closure | 1 | 4 | 4 | 5 | 3.15 |

- **A** reuses the loader that the tree already trusts. The walk is about 200 lines. Without reflection, the static reach is sound: each function value is reached where code names it.
- **B** couples a ship guard to an LLM tool index. The index drops each stdlib call by design (`indexer.go:237`), so it needs a second mode.
- **C** adds three modules (`x/tools`, `x/mod`, `x/sync`) to `go.mod` and `go.sum`, and a fetch to each CI job. A new `x/tools` version can require a toolchain newer than 1.23. It is more exact for function values, but A is sound for them already.
- **D** misses Must R1 at the function level. It refuses each floor in `core` forever, because `core` imports `gitexec` for work that is not a gate (`core/continuation_stamp.go:23`).

### 7.4 The reach rules of approach A

1. A root is the named function of an entry `Run`. A closure is refused, because its static identity is not stable.
2. From a reached function body, each `*types.Func` that the body uses is reached, as a call or as a value. A generic function is reached through `Origin()`.
3. Each module named type that is the type of an expression in a reached body is live. Each method of a live type is reached. This covers interface calls and stdlib callbacks such as `sort.Sort`, with no CHA.
4. When a package holds a reached function, its package-level initializers and its `init` functions are reached.
5. A stdlib body is never walked. A `sysexec` body is never walked: each `sysexec` call is judged at its site.
6. A `repostate` body is never walked and never judged. A separate test judges `repostate` itself (test 9).

### 7.5 The refused list (one table, `gatehermetic.Contract()`)

| Class | Refused when | Entries |
|---|---|---|
| `git` | an exec entry's program argument is the constant `git` | `os/exec.Command` (arg 0), `os/exec.CommandContext` (1), `os/exec.LookPath` (0), `sysexec.Command` (1), `sysexec.Capture` (3), `sysexec.Output` (3), `sysexec.CombinedOutput` (3), `sysexec.DefaultRunner` (1), a call of a `sysexec.RunFunc` value (1) |
| `exec` | the program argument of an exec entry is not a constant, or the entry has no program | the same entries; `os.StartProcess`, `syscall.Exec`, `syscall.ForkExec`, `syscall.StartProcess`; a call of a `sysexec` function that is not in the table |
| `git-dir` | a constant string expression in a reached body has a `/`-separated segment `.git` | each folded constant, so `".gi"+"t"` and a named constant count |
| `mtime` | a reached body uses a method `ModTime` or `Sys` of a type that implements `io/fs.FileInfo`, or `syscall.Stat`, `Lstat` or `Fstat` | |

- A program other than `git` is allowed, for example `go list` in `fleet/packagegraph.go:26`.
- `os.Stat` is allowed, because existence and size are not mtime. `testdup` keeps its root check.
- `gitexec` is not a special case. Its `Capture` passes the constant `gitBin` to `sysexec.Capture` (`gitexec.go:14,32`), so rule `git` refuses it.
- The table is protected contract data, not an operator setting. A value in `.evolve/policy.json` that loosens it is an off switch, which R9 of ADR-0165 rejects.

## Decision

Approach A. It is the only approach that holds each Must with no new module. It reuses the loader that the tree already tests. G2 does not catch a call through `reflect`, `plugin`, `unsafe` or `go:linkname` (contract section 5).

`testdup` splits its verdict into two parts, so that a main-side run with any `Base` gives the count of the lane:

- `Copies` counts the groups of non-`acs` sites only. It does not depend on `Base`, so the count stays stable after a landing.
- `Fresh` holds each group with a site in an `acs/` directory that the lane tree changes against `Base`. Each `Fresh` group is a red, with owner `tdd`.

Today a new `acs/` copy raises the count. After G2, the same copy is a `Fresh` red. The lane stays as strict as today.

Not in G2: the key baseline (G6), the parity harness and the 13 other debt keys (G3), base-red reruns (G4), the TDD floor list (G5).

### 7.6 The packages and the dependency direction

| Package | Holds | Imports |
|---|---|---|
| `go/internal/goload` (new) | the `go list` and `go/types` loader, moved from `gotypes` | stdlib, `sysexec` |
| `go/internal/gatehermetic` (new) | `Contract`, `RootOf`, `Load`, `Refusals` | stdlib, `goload`, `sysexec` |
| `go/internal/repostate` | the git facts of each gate | stdlib, `sysexec` |
| `go/internal/adapters/codeindex/gotypes` | the index, now on `goload` | `goload` |
| `go/internal/testdup`, `phasecoherence`, `fleet` | gate logic | add `repostate` and `gatecontract` |
| `go/internal/gateregistry` (G1) | two new entries; the hermetic real-tree test | adds `phasecoherence`, `fleet`; its test adds `gatehermetic` |

No gate package imports `gatehermetic` or `gateregistry`. `gatehermetic` imports no gate. The guard is a contract test like the G1 registry tests, so it is not a registry entry. That choice also avoids the cycle `gateregistry → gatehermetic → gateregistry`.

### 7.7 The types and the signatures

```go
package goload

type Package struct {
	Path     string
	Dir      string
	Standard bool
	Module   bool
	Types    *types.Package
	Info     *types.Info
	Files    []*ast.File
	Errors   []error
}

type Program struct {
	Fset     *token.FileSet
	Packages map[string]*Package
	Order    []string
}

func Load(ctx context.Context, run sysexec.RunFunc, env []string, moduleDir string, patterns []string, withTests bool) (Program, error)
```

```go
package gatehermetic

type Root struct {
	Package string
	Func    string
}

type ExecEntry struct {
	Package string
	Name    string
	Program int
}

type Rules struct {
	Exempt   []string
	Boundary []string
	Exec     []ExecEntry
	Mtime    []string
	GitWords []string
}

type Refusal struct {
	Root  Root
	Class string
	Site  string
	Chain []string
}

type Graph struct {
	program goload.Program
	decls   map[*types.Func]*ast.FuncDecl
}

func Contract() Rules
func RootOf(fn any) (Root, error)
func Load(ctx context.Context, run sysexec.RunFunc, moduleDir string, roots []Root) (Graph, error)
func Refusals(g Graph, roots []Root, rules Rules) ([]Refusal, error)
func RepostateRefusals(g Graph, rules Rules) []Refusal
```

```go
package repostate

type Change struct {
	Path  string
	Added bool
}

func LaneTree(ctx context.Context, root, relDir string) ([]string, error)
func Untracked(ctx context.Context, root, relDir string) ([]string, error)
func ChangedSince(ctx context.Context, root, base, relDir string) ([]Change, error)
func Ignored(ctx context.Context, root string, paths []string) (map[string]bool, error)
func ReadAtBase(ctx context.Context, root, base, rel string) ([]byte, error)
func DiffAgainstBase(ctx context.Context, root, base string, paths ...string) ([]byte, error)
```

`TrackedFiles`, `TrackedTree` and `TrackedSet` stay as they are. The new functions take a context, because a run function has one.

```go
package testdup

type Report struct {
	Copies int
	Groups []Group
	Fresh  []Group
}

type Baseline struct {
	Count int
}

type Tree struct {
	Files   []string
	Changed []string
}

func Lane(ctx context.Context, moduleDir, base string) (Tree, error)
func Scan(fsys fs.FS, tree Tree) (Report, error)
func Check(report Report, baseline Baseline) error
func GroupsUnder(groups []Group, dir string) []Group
func ParseBaseline(raw []byte) (Baseline, error)
func FormatBaseline(baseline Baseline) []byte
```

```go
package phasecoherence

func TreeRun(ctx context.Context, in gatecontract.Input) (gatecontract.Verdict, error)
```

```go
package fleet

func ZoneRun(ctx context.Context, in gatecontract.Input) (gatecontract.Verdict, error)
func PreflightControlPlane(ctx context.Context, repoRoot, base string) error
```

### 7.8 The behavior and the signal of each failure

**The guard.**

1. `RootOf` reads `runtime.FuncForPC` of the value. It splits the result into a package path and a name.
   - Failure: the name holds `.func` or a receiver. Cause: a closure or a method is the run function. Evidence: the runtime name. Next step: lift the body into a named function.
2. `Load` runs `go list -buildvcs=false -json -deps` on the packages of the roots, with `CGO_ENABLED=0`. Then it type-checks each module package with full `Types`, `Defs` and `Uses`.
   - Failure: a non-zero exit or a type error. Cause: the tree does not build. Evidence: the stderr text or the first type error. Next step: fix the build. This is an error ("did not judge"), never a pass.
   - Failure: a root that the loaded program does not declare. Cause: a renamed run function. Evidence: the root. Next step: an error.
3. `Refusals` walks breadth first from each root with the rules of 7.4. It keeps a parent map, and it judges each reached body against 7.5.
4. Each refusal gives `Root`, `Class`, the site `file:line:col` in the module, and the chain from the root.

**`repostate`.**

1. Each function that takes `base` checks that `base` is 40 or 64 lowercase hex digits.
   - Failure: the base is `HEAD`, a branch or empty. Cause: the caller passed a name, not a commit. Evidence: the value. Next step: pass the lane base of the run.
2. `LaneTree` runs `ls-files -z --cached --others --exclude-standard --deduplicate` and removes the output of `ls-files -z --deleted`. The result is the tree that `git add -A` commits. `rawgitratchet.BoundTestFiles` reads `LaneTree` in place of `TrackedTree`, so an unstaged test file is bound.
3. `ChangedSince` runs `diff -z --name-status --no-renames --relative <base> -- <relDir>` and adds `Untracked`. A path is `Added` when its status is `A` or when it is untracked.
4. `ReadAtBase` runs `cat-file blob <base>:<rel>`. A missing path returns `fs.ErrNotExist`. A missing base returns an error that names the base. This is the contract of `commentaudit.ReadAtBase` (`across.go:61-70`), which moves here.
5. `Ignored` runs `check-ignore -z --stdin`. Exit 1 means that no path is ignored, and it is not an error.
6. Each git error returns the arguments, the exit code and the stderr text.

**`testdup`.**

1. `Lane` calls `LaneTree(moduleDir, ".")` for the `_test.go` files. It calls `ChangedSince(moduleDir, base, "acs")` and keeps the first two segments of each path.
   - Failure: either call fails. Signal: an error, so the gate did not judge.
2. `Scan` skips each segment `vendor`, `testdata`, `.*` and `_*`. It skips `acs/<dir>` unless `<dir>` is in `Changed`. A listed file that does not parse is an error with the path.
3. `Copies` counts the extra sites of each group of non-`acs` sites. `Fresh` holds each group of two or more sites with a site in a changed `acs/` directory.
4. `Check` keeps both count messages (`testdup.go:218-225`). It adds one red for `Fresh`, which lists each group and names `go/acs/cycle*/` as the owner of `tdd`.
5. `ParseBaseline` needs only `count`. It ignores a `date` line, like each unknown key today (`testdup.go:238`). G2 removes the line from the file.

**The real-tree tests.** G3 names the base of a main-side run. Until then, a real-tree test reads the commit id of `HEAD` through `gitexec` and passes it as `Base`. A test is not a gate, so the guard does not cover it. At ship, the pack runs on the staged tree, so `HEAD` is the lane base. This fact is ASSUMED from the incident record, and the build verifies it at `phases/ship`.

G3a retires the real-tree call of the `testdup` entry, because `RealTree` refuses a `diff` entry. `evolve gate check --base` runs that entry from then on.

**The floors.** Each read of the floor table becomes the named `repostate` call. `floorBase` returns the recorded base or the empty string, never `HEAD`. An empty base fails the `repostate` base check. The floor then reports "did not judge" with the cause "the run has no recorded worktree base".

**`fleet`.** `loopwave/dispatch.go:171` reads the commit id of `HEAD` through `gitexec` and passes it to `PreflightControlPlane`. The preflight refuses each `ChangedSince` path that `guards.IsProtectedSurface` protects. This is what `git status --untracked-files=all` reports today.

### 7.9 The two debt keys that leave `Unregistered()`

| Key | Entry | Kind | Files | Owners | Inputs (class) | Run |
|---|---|---|---|---|---|---|
| `./internal/phasecoherence/...` | `phase-coherence` | `scanner` | `go/internal/phasecoherence/treegate.go` | each file → `build` | `agents/`, `.evolve/profiles/`, `.evolve/`, `.gitignore`, the lane tree (each `subject`) | `phasecoherence.TreeRun` |
| `./internal/fleet/...` | `fleet-global-zone` | `scanner` | `go/internal/fleet/zonegate.go` | each file → `build` | `docs/architecture/packages/internal-fleet.md`, `go/internal/fleet/` (each `subject`) | `fleet.ZoneRun` |

- `TreeRun` runs the six judges of 7.1 as checks in `treegate.go`. It prefixes each red with the check name. `TrackedTree` replaces `ls-files`, and `Ignored` replaces `check-ignore`.
- The decoy test mirrors the real profiles today. It becomes a `gittest.Fixture` test with three made-up profiles, so it no longer reads the real tree.
- The raw fixtures at `unpaired_tracked_test.go:14,85` and `unpaired_tracked_edge_test.go:16` move to `gittest.Fixture`. The two `baseline.json` rows go.
- `ZoneRun` compares the global-zone clause of the document with `GlobalZoneFiles()`. The `packagegraph` and `partition` tests that use `../..` move to a fixture module made with `gittest.Fixture` and `Repo.Write`. They then no longer read the real tree. `TestUnrelatedLeafFixture_SharesNoPackageWithFleet` retires, because its fixture assumption becomes fixture data.
- Each entry is at `enforce`. These judges block a landing today through the pack, so decision 15 does not apply (the same reason as in section 6.4).
- Each entry `Covers` its pattern. One real-tree test in an external test package calls each `Run`.

## Blueprint

### 7.10 The tests (each one is red without G2)

| # | Test | Entry point | Red case |
|---|---|---|---|
| 1 | `gatehermetic`: a fixture gate whose helper package runs `exec.Command("git", "log")` is refused, with a chain of three steps | `Load`, `Refusals` on a `gittest.Fixture` module | no guard |
| 2 | a fixture with the pre-G2 `testdup` shape (`gitLines`, the `ModTime` walk) gives `git` and `mtime` | same | plan proof: the current `testdup` fails |
| 3 | a call through a method value, a `sysexec.RunFunc` parameter and a package `var` initializer is refused | same | a walk of calls only |
| 4 | a method of a live type that `sort.Sort` reaches is refused | same | no live-type rule |
| 5 | `filepath.Join(root, ".gi"+"t", "HEAD")` is refused; `".gitignore"` is not | same | a literal-only scan |
| 6 | `sysexec.Output(..., "go", ...)` passes; a variable program and an unlisted `sysexec` function are refused | same | a git-only rule |
| 7 | the same `git log` call passes in a fixture `repostate` and is refused in its sibling package | same | no exemption, or an exemption by name prefix |
| 8 | `RootOf` refuses a closure; a failed `go list` is an error and no refusal list | `RootOf`, `Load` with a fake runner | a closure root; a failed load that passes |
| 9 | `repostate` itself has no `mtime` or `git-dir` refusal and no `GitWords` constant (`HEAD`, `log`, `reflog`, `rev-list`, `merge-base`, `%ct`, `--since`) | `RepostateRefusals` on the real package | an mtime read or `HEAD` in `repostate` |
| 10 | `gateregistry`: each entry `Run` of the real registry has no refusal | the real `Entries()`, `RootOf`, `Load`, `Refusals` | the current `testdup` (acceptance 1; the build records the red run on the parent commit) |
| 11 | `repocontract.TreeReadingTests()` holds test 10 | the real selection that ship runs | the guard is off the ship path |
| 12 | `repostate`: `TrackedTree` and `LaneTree` hold a staged new file that `Base` lacks; `ChangedSince` marks it `Added` | `gittest.Fixture` | answers from `Base` only (R11 of the ADR) |
| 13 | `repostate`: `LaneTree` holds an untracked file, drops an ignored file and drops a file deleted on disk | same | index only, or a disk walk |
| 14 | `repostate`: a base of `HEAD` or `main` is an error that names it | same | a symbolic base |
| 15 | `testdup` entry run: a staged new `_test.go` file with a copy is a red, and the same copy unstaged is the same red | the registry `Run` on a `gittest.Fixture` | acceptance 3; a disk-only or `Base`-only file list |
| 16 | `testdup`: a copy in a new `acs/cycleN` is a `Fresh` red; an old unchanged `acs/` copy is not counted; `Copies` is equal for two bases | `Scan` on `fstest.MapFS`, `Lane` on a fixture | a count that depends on `Base` |
| 17 | `testdup`: a baseline with a `date` line and no `count` is an error; with `count` only it parses | `ParseBaseline` | the `date` requirement |
| 18 | TDD floor: a copy in the untracked `acs/cycleN` of the run is refused | `tddPredicateFloor.Review` with a worktree fixture | the floor on the old `Scan` |
| 19 | `phasecoherence`: one red fixture for each of the six checks; the real-tree test calls `TreeRun` | `TreeRun` | a check that always passes |
| 20 | `fleet`: a documented zone that differs gives a red; a protected path that changes since `base` refuses the preflight | `ZoneRun`, `PreflightControlPlane` | a check that always passes |
| 21 | `gateregistry` (G1 test 1): `Unregistered()` has 13 keys, and its literal list drops the two keys | the real registry | a key that stays |
| 22 | `gotypes`: its whole suite is unchanged on `goload` | the existing suite | a changed index |
| 23 | `rawgitratchet`: `BoundTestFiles` holds an untracked `_test.go` file and drops an ignored one | `BoundTestFiles` on a `gittest.Fixture` | the index-only list |

The seven names that `TestC2231_001` pins stay in `testdup`.

### 7.11 The build order (each commit is green on its own)

| # | Commit | Depends on | Risk | Verifiable by |
|---|---|---|---|---|
| 1 | `repostate`: the six functions, the base check, the manifest row `/go/internal/repostate/`; `rawgitratchet.BoundTestFiles` on `LaneTree` | G1 | Low: `git` flags on an older git (`--deduplicate` needs 2.31) | tests 12-14, 23 |
| 2 | `goload` from `gotypes`, with no change of behavior; the manifest row `/go/internal/goload/` | — | Medium: the order of the index | test 22 |
| 3 | `gatehermetic` with `Contract`; the manifest row `/go/internal/gatehermetic/` | 2 | Medium: a false refusal through a live type | tests 1-9 |
| 4 | `testdup` on `repostate`; the new baseline count; the TDD floor; the real-tree test; test 10 and its `TreeReadingTests` row. The `cycle2231` predicates `002`, `003` and `004` go to the superseded-predicate archive with a reason. | 1, 3 | High: the count changes | tests 10, 11, 15-18 |
| 5 | the floor reads of 7.1 move to `repostate`; `floorBase` loses `HEAD`; `commentaudit.ReadAtBase` moves | 1, 4 | High: a floor that skips with no base | the floor suites; test 10 on the three floor entries of G4 (`repo-contract-floor`, `unit-test-floor`, `cover-changed-floor`) |
| 6 | `phasecoherence`: `treegate.go`, the entry, the fixtures; two `baseline.json` rows go | 1, 3 | Medium: an import cycle through `prompts` | tests 19, 21 |
| 7 | `fleet`: `zonegate.go`, the entry, the module fixtures, the preflight on `ChangedSince` | 1, 3 | Medium: about fifteen tests move | tests 20, 21 |

Commit 4 lowers `count` to the new measure, because it now leaves out each `acs/` site. It can only go down. The commit message gives the old and the new value.

### 7.12 How the files stay protected

G2 adds these `GateGuarantee` rows to `guards.ProtectedSurfaceManifest`:

- `/go/internal/repostate/`: each gate trusts its tree, so a lane that changes it can show a gate another tree.
- `/go/internal/gatehermetic/` and `/go/internal/goload/`: the guard and the loader that it trusts.
- `/go/internal/phasecoherence/treegate.go` and `/go/internal/fleet/zonegate.go`: the `Files` of the two entries. G1 test 6 requires them.

The test files of the guard are in `gateregistry` and `gatehermetic`, both on the manifest. `repocontract.go` is on the manifest since G1. After G2, a lane cannot add a `repostate` function or loosen `Contract()`. A lane that needs a new git fact sends a held proposal, and the console lands it. The console builds G2 itself, because `testdup`, `rawgitratchet/baseline.json`, `gateregistry` and `repocontract.go` are protected since G1.

## Risks

- **A false refusal.** The live-type rule reaches each method of a type that a gate names. Next step: the refusal chain names the type, and the gate names a smaller type. No allow list is added.
- **A missed reach.** `reflect`, `plugin` and `go:linkname` are outside the static reach. Review checks them (contract section 5).
- **The real-tree `Base` is `HEAD`** until G3. The staged lane is stricter than main, never laxer, so it cannot turn main red.
- **Floor scope.** G4 lands first, so commit 5 must cover the three floor entries that G4 registers. Test 10 lists each refusal, and that list is the work list. Each other floor that commit 5 moves has no entry, so the guard does not cover it. Its own suite holds the `repostate` call until G5 registers it.
- **The `goload` move** touches an adapter that the console uses. Test 22 holds it.

### 7.13 Limits of G2

- G2 proves hermeticity, not parity. G3 runs the three states.
- 13 debt keys stay for G3.
- `go/.test-dup-baseline` stays a lane-writable count until G6. G2 removes the date, which was its second off switch.
- A check that needs `HEAD`, ancestry or a second commit cannot become an entry (open question 1).

### 7.14 Open questions for the operator

1. **Checks that read history.** `lost_landing_floor.go:76-96`, `explanationdocs/gitio.go:178,262` and `rebind.go:132,176` read `HEAD` or ancestry by design: they witness a landing. Recommended: these are landing witnesses, not lane-tree gates. G4 keeps them out of the registry and lists them in the spec, and the guard does not cover them.
2. **The mtime freshness of `covchanged`** (`workspace.go:129-134`). Recommended: the floor writes the profile in the same run, so freshness holds by construction. Replace the mtime read with a SHA-256 of each source, recorded when the profile is made. Then the check is a content check, and the guard passes it.
3. **The `fleet` key.** Its judges read no git, so section 6.5 gives the wrong reason for G2. Recommended: keep it in G2 as the plan says. The work is one small entry and a module fixture. The other choice is to move it to G3 with the other 13 keys.
