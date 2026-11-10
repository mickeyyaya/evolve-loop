# Component G2: the call-graph hermeticity guard

> Plan row G2 of [gate-contract-2026-11.md](../plans/gate-contract-2026-11.md). The contract and the G1 design are in [gate-contract.md](gate-contract.md), and its section 7 links this page. The headings here keep the numbers 7.1 to 7.14. Inbox item `gate-contract-g2-hermetic-guard`. ADR-0165 decisions 3 and 4. The measures are from `92df3d291` (2026-11-10).
>
> The rows that name a G4 file are measures of `ce4189a2c`. The operator approved the toolchain boundary of 7.4 to 7.8 on 2026-11-11.

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
| `covchanged/workspace.go:129-140` | `ModTime` of each source against the profile | none: the floor reader has no freshness check (7.8) |
| `changedpkgs/changedfiles.go:22,35` | `diff --name-status base`, `ls-files --others` | `ChangedSince` |
| `explanationdocs/gitio.go:42,46,251,390`, `rebind.go:216` | names and contents against the base | `ChangedSince`, `ReadAtBase` |
| `core/phase_bindings_selfcheck.go:35-42`, through `floorPaths` (`base_red_floor.go:44-52`) | `diff base --name-only`, `ls-files --others` | `ChangedSince` |
| `core/base_red_floor.go:72-89`, `baseattr/attribute.go:81,132,142,144` | `rev-parse`, `worktree add`, `worktree remove`, `worktree prune` | none: outside the entry (7.8) |
| `phases/ship/repocontract.go:221-230`, `repocontract_cmd_unix.go:11-12` | `go test` with a built argument slice; the process-group fields of `exec.Cmd` | `gotool.Test`; the pair moves to `sysexec` (7.8) |
| `core/build_floor_reviewer.go:385`, `core/phase_bindings_selfcheck.go:99,110` | `go test` with a built argument slice | `gotool.Test` (7.8) |
| `core/build_floor_reviewer.go:216`, `changedpkgs/importerclosure.go:98` | `go list` | `gotool.List` (7.8) |
| `core/build_floor_reviewer.go:310` | `go tool cover -func` on the profile | none: `covchanged.FuncPercents` reads the profile in process (7.8) |
| `explanationdocs/gitio.go:178,262`, `rebind.go:132,176`, `core/lost_landing_floor.go:76-96` | `HEAD`, ancestry and a second commit | none: question 1 of 7.14 |

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
| R2 | It refuses each class of 7.5 outside the boundary packages `repostate` and `gotool`: `git`, `exec`, `git-dir`, `mtime`, `history` and `inject`. | Must |
| R3 | `repostate` reads no mtime, no `.git` path and no `HEAD`, history or date. It accepts only a commit id as `Base`. | Must |
| R4 | `testdup` loses `Date`, `since`, `loadAcsAges`, the `HEAD` diff and the mtime walk. | Must |
| R5 | An `acs/` directory is new when the lane tree changes it against `Base`. | Must |
| R6 | A gate on the tracked set judges a staged new file that `Base` does not hold. | Must |
| R7 | The floors and the `phasecoherence` scanners read git through `repostate`. The `fleet` preflight is not a gate and stays as it is. | Must |
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

1. A root is the named function of an entry `Run`. A closure is refused, because its static identity is not stable. Each registered `Run` is thus a named function whose body names its judge.
2. From a reached function body, each `*types.Func` that the body uses is reached, as a call or as a value. A generic function is reached through `Origin()`.
3. Each module named type that a reached body mentions is live. The structure of a live type is live also: each field, embedded field, element and type argument. Each method of a live type is reached. This covers interface calls and stdlib callbacks such as `sort.Sort` and `json.Marshal`, with no CHA.
4. When a package holds a reached function, variable or constant, its package-level initializers and its `init` functions are reached.
5. A stdlib body is never walked. A `sysexec` body is never walked: each `sysexec` call is judged at its site. A third-party body is walked like a module body.
6. A boundary body is never walked and never judged. The boundary packages are `repostate` and `gotool`. A separate test judges each one and its imports (tests 9 and 42).
7. Rule 7 covers each package variable that holds a function, interface or channel type at any depth. A reached body reads such a variable only when no statement assigns it. Each other read is class `inject`.
8. A reached package is class `exec` when it has an excluded file, a cgo file or a body-less function. One load sees one build configuration, so the guard refuses what it cannot read.

Each step of a chain holds its edge kind: `call`, `value`, `live-type` or `init`. A refusal through a `live-type` edge is thus visible as a possible false refusal.

### 7.5 The refused list (one table, `gatehermetic.Contract()`)

| Class | Refused when | Entries |
|---|---|---|
| `git` | a reached body calls an exec entry, and the program argument is the constant `git` | `os/exec.Command` (`name`), `os/exec.CommandContext` (`name`), `os/exec.LookPath` (`file`); `sysexec.Command`, `Capture`, `Output`, `CombinedOutput` and `DefaultRunner` (`name`); a call of a `sysexec.RunFunc` value (`name`) |
| `exec` | a reached body calls an exec entry, and the program is not the constant `git` | The same entries. Also `os.StartProcess`, `syscall.Exec`, `syscall.ForkExec`, `syscall.StartProcess`, `syscall.Syscall*`, `syscall.RawSyscall*`. A write to `exec.Cmd.Path` or `exec.Cmd.Args`. A call of a `sysexec` function that is not in the table. |
| `git-dir` | a constant string expression in a reached body has a `/`-separated segment `.git` | each folded constant, so `".gi"+"t"` and a named constant count |
| `mtime` | a reached body uses a method `ModTime` or `Sys` of a type that implements `io/fs.FileInfo`, or one of the listed functions | `syscall.Stat`, `Lstat`, `Fstat`; `fs.FormatFileInfo`, `fs.FormatDirEntry`; `tar.FileInfoHeader`, `zip.FileInfoHeader` |
| `history` | a reached body uses the listed function | `runtime/debug.ReadBuildInfo`, which gives the commit and its date with no git process |
| `inject` | rule 7 of 7.4 | |

- A gate starts no process itself. `repostate` is the boundary for git, and `gotool` is the boundary for the Go toolchain. No site outside them passes, so the guard reads no argument of a gate site.
- The allowed list is closed, and only `gotool` uses it. It holds two rows: `go list` and `go test`. Test 42 judges `gotool` against it.
- So `sh -c`, `env git`, `/usr/bin/git`, `find -newer`, `go run` and `go tool` are refused. A `go test` call that a gate writes itself is refused also.
- The child process of a `gotool.Test` call is outside the static reach (7.13).
- Each exec entry names its program parameter by name, not by index. The load fails when the loaded signature lacks it.
- A reference to an exec entry without a call passes. Each call of the `sysexec.RunFunc` value that holds it is judged at its site.
- A write to another field of `exec.Cmd`, such as `Dir` or `Env`, does not change the program, and it passes.
- `os.Stat` is allowed, because existence and size are not mtime. `testdup` keeps its root check.
- A skip list that names `.git` is refused also. Commit 5 changes `changedpkgs/direct_importers.go:88` to the dot-prefix rule of `Scan`. No allow list is added.
- `gitexec` is not a special case. Its `Capture` passes the constant `gitBin` to `sysexec.Capture` (`gitexec.go:14,32`), so rule `git` refuses it.
- The table is protected contract data, not an operator setting. A value in `.evolve/policy.json` that loosens it is an off switch, which R9 of ADR-0165 rejects.

## Decision

Approach A. It is the only approach that holds each Must with no new module. It reuses the loader that the tree already tests. G2 does not catch a call through `reflect`, `plugin`, `unsafe` or `go:linkname` (contract section 5).

`testdup` splits its verdict into two parts, so that a main-side run with any `Base` gives the count of the lane:

- `Copies` counts the groups of non-`acs` sites only. It does not depend on `Base`, so the count stays stable after a landing.
- `Fresh` holds each group with a site in an `acs/` directory that the lane tree changes against `Base`. Each `Fresh` group is a red, with owner `tdd`.

Today a new `acs/` copy raises the count. After G2, the same copy is a `Fresh` red. The lane stays as strict as today.

Not in G2: the key baseline (G6), the parity harness and the 13 other debt keys (G3), base-red reruns (G4), the TDD floor list (G5). G2 changes one part of G4: each floor entry gets the lane ruler (7.8).

### 7.6 The packages and the dependency direction

| Package | Holds | Imports |
|---|---|---|
| `go/internal/goload` (new) | the `go list` and `go/types` loader, moved from `gotypes` | stdlib, `sysexec` |
| `go/internal/gatehermetic` (new) | `Contract`, `RootOf`, `Load`, `Refusals`, `Audit`, `ToolchainAudit` | stdlib, `goload`, `sysexec` |
| `go/internal/repostate` | the git facts of each gate | stdlib, `sysexec` |
| `go/internal/gotool` (new) | `List`, `Test` and the one `command` function | stdlib, `sysexec` |
| `go/internal/covchanged` | adds `FuncPercents` | no new import |
| `go/internal/core`, `changedpkgs`, `phases/ship` | the floors and the pack | add `gotool` |
| `go/internal/adapters/codeindex/gotypes` | the index, now on `goload` | `goload` |
| `go/internal/testdup`, `phasecoherence` | gate logic | add `repostate` |
| `go/internal/fleet` | gate logic | no new import |
| `go/internal/gateregistry` (G1) | two new entries in `seed_phasecoherence.go` and `seed_fleet.go`; a named run function for each floor entry; the hermetic real-tree test | adds `phasecoherence`, `fleet`; its test adds `gatehermetic` |

Each run function stays in a `gateregistry/seed_*.go` file, which is the G1 shape. A gate package gives a plain function and does not import `gatecontract`. No gate package imports `gatehermetic` or `gateregistry`. `gatehermetic` imports no gate. The guard is a contract test like the G1 registry tests, so it is not a registry entry. That choice also avoids the cycle `gateregistry → gatehermetic → gateregistry`.

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

type Request struct {
	Dir      string
	Patterns []string
	Env      []string
}

type Loader struct {
	Run sysexec.RunFunc
}

func (l Loader) Load(ctx context.Context, req Request) (Program, error)
func (l Loader) LoadWithTests(ctx context.Context, req Request) (Program, error)
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
	Param   string
}

type Command struct {
	Name string
	Verb string
}

type Rules struct {
	Exempt   []string
	Boundary []string
	Exec     []ExecEntry
	Allowed  []Command
	Mtime    []string
	History  []string
	GitWords []string
}

type Step struct {
	Func string
	Edge string
}

type Refusal struct {
	Root  Root
	Class string
	Site  string
	Chain []Step
}

type Source struct {
	ModuleDir string
	Roots     []Root
}

type Graph struct {
	program goload.Program
	roots   []Root
	decls   map[*types.Func]*ast.FuncDecl
}

func Contract() Rules
func RootOf(fn any) (Root, error)
func Load(ctx context.Context, loader goload.Loader, src Source) (Graph, error)
func Refusals(g Graph, rules Rules) ([]Refusal, error)
func Audit(g Graph, pkg string, rules Rules) []Refusal
func ToolchainAudit(g Graph, pkg string, rules Rules) []Refusal
```

`Graph` keeps its roots, so `Refusals` cannot judge a second list. `Audit` judges each body of one package against the rules, and test 9 calls it for `repostate`. `ToolchainAudit` judges `gotool` by the four rules of 7.8, and test 42 calls it.

```go
package gotool

type Call struct {
	Dir  string
	Env  []string
	Args []string
}

func List(ctx context.Context, call Call) *exec.Cmd
func Test(ctx context.Context, call Call) *exec.Cmd
```

`Args` holds the words after the verb. The caller sets `Stderr`, `WaitDelay` and the process group on the result, and the write rule for `Path` and `Args` stays.

```go
package covchanged

func FuncPercents(profile Profile, read func(rel string) ([]byte, error)) ([]Func, error)
```

```go
package apicover

type Config struct {
	Enforce           bool
	Dirs              []string
	CoverPath         string
	Cover             []CoverEntry
	ChangedFilesByDir map[string][]string
}
```

`Cover` is new. `CoverPath` stays for the `evolve apicover` command, which is not a gate.

```go
package repostate

type Change struct {
	Path  string
	Added bool
}

type Lane struct {
	root, base string
}

func LaneTree(ctx context.Context, root, relDir string) ([]string, error)
func Untracked(ctx context.Context, root, relDir string) ([]string, error)
func Ignored(ctx context.Context, root string, paths []string) (map[string]bool, error)
func At(root, base string) (Lane, error)
func (l Lane) ChangedSince(ctx context.Context, relDir string) ([]Change, error)
func (l Lane) ReadAtBase(ctx context.Context, rel string) ([]byte, error)
func (l Lane) DiffAgainstBase(ctx context.Context, paths ...string) ([]byte, error)
```

`TrackedFiles`, `TrackedTree` and `TrackedSet` stay as they are. The new functions take a context, because a run function has one. `At` is the one place that checks the base, so no function gets a base that is not checked.

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

func ReadTree(ctx context.Context, moduleDir, base string) (Tree, error)
func Scan(fsys fs.FS, tree Tree) (Report, error)
func Check(report Report, baseline Baseline) error
func GroupsUnder(groups []Group, dir string) []Group
func ParseBaseline(raw []byte) (Baseline, error)
func FormatBaseline(baseline Baseline) []byte
```

```go
package phasecoherence

func TreeReds(ctx context.Context, root string) ([]string, error)
```

```go
package fleet

func ZoneReds(root string) ([]string, error)
```

```go
package gateregistry

func runPhaseCoherence(ctx context.Context, in gatecontract.Input) (gatecontract.Verdict, error)
func runFleetZone(ctx context.Context, in gatecontract.Input) (gatecontract.Verdict, error)
func runRepoContractFloor(ctx context.Context, in gatecontract.Input) (gatecontract.Verdict, error)
func runUnitTestFloor(ctx context.Context, in gatecontract.Input) (gatecontract.Verdict, error)
func runCoverChangedFloor(ctx context.Context, in gatecontract.Input) (gatecontract.Verdict, error)
```

The body of each floor run function names its judge. For example, `runRepoContractFloor` calls `core.FloorEntryRun(core.RepoContractFloorJudge(ship.RunRepoContractPack))(ctx, in)`. Rule 2 then reaches the judge and the pack function. `floorSpec` holds the run function in place of the judge.

### 7.8 The behavior and the signal of each failure

**The guard.**

1. `RootOf` reads `runtime.FuncForPC` of the value. It splits the result into a package path and a name.
   - Failure: the name holds `.func` before a digit, or a receiver. Cause: a closure or a method is the run function. Evidence: the runtime name. Next step: lift the body into a named function.
2. `Load` runs `go list -buildvcs=false -json -deps` on the packages of the roots, with `CGO_ENABLED=0`. Then it type-checks each module package with full `Types`, `Defs` and `Uses`.
   - Failure: a non-zero exit or a type error. Cause: the tree does not build. Evidence: the stderr text or the first type error. Next step: fix the build. This is an error ("did not judge"), never a pass.
   - Failure: a root that the loaded program does not declare. Cause: a renamed run function. Evidence: the root. Next step: an error.
3. `Refusals` walks breadth first from each root with the rules of 7.4. It keeps a parent map, and it judges each reached body against 7.5.
4. Each refusal gives `Root`, `Class`, the site `file:line:col` and the chain from the root. Each step has its edge kind.
5. Commit 5 runs the guard on the real registry before its other changes. Its message records each refusal by edge kind. If many come through a `live-type` edge, the rule becomes narrower. A method is then reached only when a reached body converts its type to an interface. No allow list is added.

**`repostate`.**

1. `At` checks that `base` is 40 or 64 lowercase hex digits.
   - Failure: the base is `HEAD`, a branch, a short id, upper-case hex or empty. Cause: the caller passed a name, not a commit. Evidence: the value. Next step: pass the lane base of the run.
2. `LaneTree` runs `ls-files -z --cached --others --exclude-standard --deduplicate` and removes the output of `ls-files -z --deleted`. The result is the tree that `git add -A` commits. `rawgitratchet.BoundTestFiles` reads `LaneTree` in place of `TrackedTree`, so an unstaged test file is bound.
3. `ChangedSince` runs `diff -z --name-status --no-renames --relative <base> -- <relDir>` and adds `Untracked`. A path is `Added` when its status is `A` or when it is untracked.
4. `ReadAtBase` runs `cat-file blob <base>:<rel>`. A missing path returns `fs.ErrNotExist`. A missing base returns an error that names the base. This is the contract of `commentaudit.ReadAtBase` (`across.go:61-70`), which moves here.
5. `DiffAgainstBase` runs `diff -U0 --no-color --no-ext-diff --no-textconv --no-renames --relative <base> -- <paths>`. The flag set is fixed, so each caller and each host gets the same bytes.
6. `Ignored` runs `check-ignore -z --stdin`. Exit 1 means that no path is ignored, and it is not an error.
7. Each git error returns the arguments, the exit code and the stderr text.
8. `repostate` imports only the stdlib and `sysexec`. A helper package has no exemption.

**`testdup`.**

1. `ReadTree` calls `LaneTree(moduleDir, ".")` for the `_test.go` files. It calls `ChangedSince` of the lane for `acs` and keeps the first two segments of each path.
   - Failure: either call fails. Signal: an error, so the gate did not judge.
2. `Scan` skips each segment `vendor`, `testdata`, `.*` and `_*`. It skips `acs/<dir>` unless `<dir>` is in `Changed`. A listed file that does not parse is an error with the path.
3. `Copies` counts the extra sites of each group of non-`acs` sites. `Fresh` holds each group of two or more sites with a site in a changed `acs/` directory.
4. `Check` keeps both count messages (`testdup.go:218-225`). It adds one red for `Fresh`, which lists each group and names `go/acs/cycle*/` as the owner of `tdd`.
5. `ParseBaseline` needs only `count`. It ignores a `date` line, like each unknown key today (`testdup.go:238`). G2 removes the line from the file.

**The real-tree tests.** G3 names the base of a main-side run. Until then, a real-tree test reads the commit id of `HEAD` through `gitexec` and passes it as `Base`. A test is not a gate, so the guard does not cover it.

At ship, the pack runs on the staged tree, so `HEAD` is the lane base. This fact is ASSUMED from the incident record. Commit 4 has a condition: a test at `phases/ship` proves that the pack runs before the commit of the lane. If the test shows a different order, commit 4 stops and the design changes.

G3a retires the real-tree call of the `testdup` entry, because `RealTree` refuses a `diff` entry. `evolve gate check --base` runs that entry from then on.

**The floors.** Each read of the floor table becomes the named `repostate` call. `floorBase` returns the recorded base or the empty string, never `HEAD`. An empty base fails the `repostate` base check. The floor then reports "did not judge" with the cause "the run has no recorded worktree base".

**The floor entries.** An entry `Run` gives the reds of the lane tree, and it reaches no `baseattr`. `ReviewInput` gets a ruler, which is a strategy with two implementations. The lane ruler blames the lane for each red, and `floorEntryInput` sets it. The base-red ruler holds the rerun of G4, and only the build-floor reviewer sets it. The entry never names the type of the base-red ruler, so rule 3 does not reach its methods.

The rerun methods of `ReviewInput` move to the base-red ruler type. They are `attributor`, `baseRunner`, `laneReds`, `reportAttributed`, `openMainRedStop`, `mainRedSink` and `reportUnjudged` (`core/base_red_floor.go:71,82,89,182,195,202,209`, from G4). `rule`, `laneFailures` and `coverRedLines` stay, and they only call the ruler. Rule 3 reaches each method of `ReviewInput`, so no git reader stays on it. The zero value of the ruler field is the lane ruler, so the five other sites that build a `ReviewInput` do not change. Test 44 pins both facts.

A plain decorator on the judge does not fit. Each of the three judges calls the rule in the middle of its work (`repo_contract_floor.go:55`, `base_red_floor.go:158,175`), and the ruling changes the lines that it writes.

**The Go toolchain boundary.** Each `go` site of 7.1 calls `gotool.List` or `gotool.Test`. The argument builders (`unitTestArgs`, `taggedTestArgs`, `coverTestArgs`) drop the verb word. `ToolchainAudit` holds four rules for `gotool`:

1. The unexported function `command` holds the only call of an exec entry, and its program is the constant `go`.
2. Each call of `command` passes a constant verb, and the verb is a row of the allowed list.
3. No body uses `command` as a value.
4. The package imports only the stdlib and `sysexec`.

The verb is thus a constant in two bodies of one package, not at each gate site. The first design read it at each gate site, and each real site builds its arguments in a slice.

**The function coverage.** `scopedCoverFunc` no longer starts `go tool cover`. It parses the profile with `ParseProfile` and calls `FuncPercents`, which reads each profiled source file and counts with `Func.count` (`covchanged/measure.go:150`). The unexported function `coverEntries` of `core` makes one `apicover.CoverEntry` from each `Func`, with the module path before the file. The floor gives the entries to `apicover.Run` in `Config.Cover`. `FuncPercents` rounds each percent to one decimal, as `go tool cover` prints it.

**The process group.** The process-group pair of `repocontract_cmd_unix.go` and `repocontract_cmd_other.go` moves to `sysexec` with no change of its body. Rule 5 never walks `sysexec`. The build verifies that no other reached package holds an excluded file, and it records the list.

**`covchanged`.** The floor measures the profile in the same run, so its reader is a plain file read with no freshness check. The mtime check leaves `Workspace`. It goes to the command that accepts a profile of an earlier run, which is not a gate. The build reads each caller of `Workspace.sources` first and records the list in the commit message.

**`fleet`.** The preflight (`preflight.go:13`) is not a gate, and `ZoneReds` does not reach it. G2 does not change it.

### 7.9 The two debt keys that leave `Unregistered()`

| Key | Entry | Kind | Files | Owners | Inputs (class) | Run |
|---|---|---|---|---|---|---|
| `./internal/phasecoherence/...` | `phase-coherence` | `scanner` | `go/internal/phasecoherence/treegate.go` | each file → `build` | `agents/`, `.evolve/profiles/`, `.evolve/`, `.gitignore`, the lane tree (each `subject`) | `gateregistry.runPhaseCoherence` on `phasecoherence.TreeReds` |
| `./internal/fleet/...` | `fleet-global-zone` | `scanner` | `go/internal/fleet/zonegate.go` | each file → `build` | `docs/architecture/packages/internal-fleet.md`, `go/internal/fleet/` (each `subject`) | `gateregistry.runFleetZone` on `fleet.ZoneReds` |

- `TreeReds` runs five checks in `treegate.go`: `persona-profile-pairing`, `auditor-strip-contract`, `persona-strip-sentinels`, `corpus-dir-births` and `runtime-artifacts-tracked`. It prefixes each red with the check name. `TrackedTree` replaces `ls-files`, and `Ignored` replaces `check-ignore`.
- The sixth judge of 7.1 is the decoy test, which mirrors the real profiles today. It becomes a `gittest.Fixture` test with three made-up profiles, so it no longer reads the real tree.
- The raw fixtures at `unpaired_tracked_test.go:14,85` and `unpaired_tracked_edge_test.go:16` move to `gittest.Fixture`. The two `baseline.json` rows go.
- `ZoneReds` compares the global-zone clause of the document with `GlobalZoneFiles()`. The `packagegraph` and `partition` tests that use `../..` move to a fixture module made with `gittest.Fixture` and `Repo.Write`. They then no longer read the real tree. `TestUnrelatedLeafFixture_SharesNoPackageWithFleet` retires, because its fixture assumption becomes fixture data.
- Each entry is at `enforce`. These judges block a landing today through the pack, so decision 15 does not apply (the same reason as in section 6.4).
- Each entry `Covers` its pattern. One real-tree test in an external test package calls each `Run`.

## Blueprint

### 7.10 The tests (each one is red without G2)

| # | Test | Entry point | Red case |
|---|---|---|---|
| 1 | `gatehermetic`: a fixture gate whose helper package runs `exec.Command("git", "log")` is refused, with a chain of three steps and their edge kinds | `Load`, `Refusals` on a `gittest.Fixture` module | no guard |
| 2 | a fixture with the pre-G2 `testdup` shape (`gitLines`, the `ModTime` walk) gives `git` and `mtime` | same | plan proof: the current `testdup` fails |
| 3 | a call through a method value, a `sysexec.RunFunc` parameter and a package `var` initializer is refused | same | a walk of calls only |
| 4 | a method of a live type that `sort.Sort` reaches is refused | same | no live-type rule |
| 5 | `filepath.Join(root, ".gi"+"t", "HEAD")` is refused; `".gitignore"` is not | same | a literal-only scan |
| 6 | `sysexec.Output` with `go` and `list` in a gate package is refused, and the same call through the fixture `gotool.List` passes. `go run`, `sh -c`, `env git`, a variable program and an unlisted `sysexec` function are refused. | same | a git-only rule, or a pass for a gate site |
| 7 | the same `git log` call passes in a fixture `repostate` and is refused in its sibling package | same | no exemption, or an exemption by name prefix |
| 8 | `RootOf` refuses a closure. A failed `go list`, a type error and an undeclared root are each an error with no refusal list. | `RootOf`, `Load` with a fake runner | a closure root; a failed load that passes |
| 9 | `repostate` itself has no `mtime` or `git-dir` refusal and no `GitWords` constant (`HEAD`, `log`, `reflog`, `rev-list`, `merge-base`, `%ct`, `--since`). It imports only the stdlib and `sysexec`. | `Audit` on the real package | an mtime read or `HEAD` in `repostate`; a helper import |
| 10 | `gateregistry`: each entry `Run` of the real registry has no refusal | the real `Entries()`, `RootOf`, `Load`, `Refusals` | the current `testdup`; a floor entry that is a closure (acceptance 1; the build records the red run on the parent commit) |
| 11 | `repocontract.TreeReadingTests()` holds test 10 | the real selection that ship runs | the guard is off the ship path |
| 12 | `repostate`: `LaneTree` holds a staged new file that `Base` lacks; `ChangedSince` marks it `Added` | `gittest.Fixture` | answers from `Base` only (R11 of the ADR) |
| 13 | `repostate`: `LaneTree` holds an untracked file, drops an ignored file and drops a file deleted on disk | same | index only, or a disk walk |
| 14 | `repostate`: a base of `HEAD`, `main`, a short id, upper-case hex or the empty string is an error that names it | `At` | a symbolic base |
| 15 | `testdup` entry run: a staged new `_test.go` file with a copy is a red, and the same copy unstaged is the same red | the registry `Run` on a `gittest.Fixture` | acceptance 3; a disk-only or `Base`-only file list |
| 16 | `testdup`: a copy in a new `acs/cycleN` is a `Fresh` red; an old unchanged `acs/` copy is not counted; `Copies` is equal for two bases | `Scan` on `fstest.MapFS`, `ReadTree` on a fixture | a count that depends on `Base` |
| 17 | `testdup`: a baseline with `count` only parses; a `date` line is ignored; a baseline with no `count` is an error | `ParseBaseline` | the `date` requirement |
| 18 | TDD floor: a copy in the untracked `acs/cycleN` of the run is refused | `tddPredicateFloor.Review` with a worktree fixture | the floor on the old `Scan` |
| 19 | `phasecoherence`: one red fixture for each of the five checks; the real-tree test calls the registry `Run` | `TreeReds` | a check that always passes |
| 20 | `fleet`: a documented zone that differs gives a red | `ZoneReds` | a check that always passes |
| 21 | `gateregistry` (G1 test 1): `Unregistered()` has 13 keys, and its literal list drops one key in each of the last two commits | the real registry | a key that stays |
| 22 | `gotypes`: its whole suite is unchanged on `goload` | the existing suite | a changed index |
| 23 | `rawgitratchet`: `BoundTestFiles` holds an untracked `_test.go` file and drops an ignored one | `BoundTestFiles` on a `gittest.Fixture` | the index-only list |
| 24 | a method of a field type of a live type, which `json.Marshal` reaches, is refused | `Load`, `Refusals` on a fixture | a live set of named expressions only |
| 25 | a read of a package-level variable makes the `init` function of its package reached, and its `git` call is refused | same | initializers that follow only a reached function |
| 26 | a read of a function variable that a second function assigns is class `inject`; the same variable with no assignment passes. A map variable that an `init` fills with a function is `inject` also. | same | a seam that carries a git reader past the walk |
| 27 | `gatecontract.Input` holds no function, interface or channel type at any depth | the real type through `go/types` | an input that carries code |
| 28 | a reached package with a file behind a build tag, and one with a function that has no body, are each class `exec` | a fixture | a body that the load does not see |
| 29 | a generic function with a `git` call is refused through its instance | same | a walk that skips an instance |
| 30 | an exec entry whose named parameter is missing from the loaded signature is a load error | `Load` with changed rules | an index that points at the wrong parameter |
| 31 | a root in the real tree that reaches a `gitexec` reader gives class `git` | `Load`, `Refusals` with `Contract()` on the real module | a contract that fits only the fixtures |
| 32 | `debug.ReadBuildInfo` gives `history`; `fs.FormatFileInfo` gives `mtime`; a write to `exec.Cmd.Path` or `exec.Cmd.Args` gives `exec`, and a write to `exec.Cmd.Dir` passes | a fixture | the short entry list |
| 33 | a third-party package body with a `git` call is refused | a fixture with a vendored module | a walk of module bodies only |
| 34 | `repostate`: `Ignored` with exit 1 gives an empty map; a real git error gives the arguments, the exit code and the stderr text | `gittest.Fixture` | an error that hides its cause |
| 35 | `repostate`: `ReadAtBase` gives `fs.ErrNotExist` for a missing path and an error that names a missing base | same | one error for both |
| 36 | `repostate`: `Untracked` lists a new file and drops an ignored one; `DiffAgainstBase` gives the same bytes when the repository sets an external diff tool | same | host configuration in the bytes |
| 37 | `testdup`: `FormatBaseline` writes no `date` line | `FormatBaseline` | the date that stays |
| 38 | ship: the contract pack runs before the commit of the lane, so `HEAD` is the lane base | the ship phase on a worktree fixture | the ASSUMED order of 7.8 |
| 39 | `changedpkgs`: the importer walk skips each dot directory, and its source holds no `.git` constant | the importer walk on a fixture | the skip list by name |
| 40 | a floor entry `Run` gives the raw reds when the base is red also, and it adds no worktree | the registry `Run` on a `gittest.Fixture` with a red base | an entry that reruns on the base |
| 41 | a package variable that holds `sysexec.DefaultRunner` passes when no reached body calls it; a call of that variable in a reached body is class `exec` | `Load`, `Refusals` on a fixture | a reference that is refused, or a call through a value that passes |
| 42 | the real `gotool` has no refusal | `ToolchainAudit` on the real package | a third verb; a second exec site; a helper import |
| 43 | `List` and `Test` give the words `go list` and `go test` before `Args`, with `Dir` and `Env` of the call | `List`, `Test` | a verb that is missing or wrong |
| 44 | the build-floor reviewer path sets the base-red ruler; a zero `ReviewInput` rules with the lane ruler and reaches no `baseattr` | `core` on a `gittest.Fixture` | a reviewer that loses the rerun; an entry that keeps it |
| 45 | a fixture `gotool` is refused for each case: a variable verb, the verb `run`, a second exec call, `command` as a value | `ToolchainAudit` on a fixture | an audit that reads the program only |
| 46 | The percents of a fixture profile are equal to a recorded `go tool cover -func` result. A function with no statement has a case. A missing source file is an error. | `FuncPercents` | a count that differs from the tool; a missing file that passes |
| 47 | the entries of a fixture profile are equal to the entries that `ParseCoverFunc` reads from the recorded tool result | `scopedCoverFunc` of `core` on a fixture module | a path with no module path; a floor that starts `go tool` |

Tests 1 to 7, 24 to 26, 28 to 30, 32, 33, 41 and 45 use a fixture module that declares the real module path. The fixture has a stub `internal/sysexec`, a stub `internal/repostate` and a stub `internal/gotool`. Each of these tests calls `Contract()`, so the tests pin the real rule data. Only test 30 changes one exec entry of the result. Tests 27 and 31 read the real tree. The seven names that `TestC2231_001` pins stay in `testdup`.

### 7.11 The build order (each commit is green on its own)

| # | Commit | Depends on | Risk | Verifiable by |
|---|---|---|---|---|
| 1 | `repostate`: `Lane`, `At`, the three plain functions, the manifest row `/go/internal/repostate/`; `rawgitratchet.BoundTestFiles` on `LaneTree` | G1 | Low: `git` flags on an older git (`--deduplicate` needs 2.31) | tests 12-14, 23, 34-36 |
| 2 | `goload` from `gotypes`, with no change of behavior; the manifest row `/go/internal/goload/` | — | Medium: the order of the index | test 22 |
| 2b | `gotool` with `List` and `Test`; `covchanged.FuncPercents`; the manifest row `/go/internal/gotool/` | — | Low: the percent of a function differs from `go tool cover` | tests 43, 46 |
| 3 | `gatehermetic` with `Contract`; the manifest row `/go/internal/gatehermetic/` | 2, 2b | Medium: a false refusal through a live type | tests 1-9, 24-33, 41, 42, 45 |
| 4 | `testdup` on `repostate`; the new baseline count; the TDD floor; the real-tree test. The `cycle2231` predicates `002`, `003` and `004` go to the superseded-predicate archive with a reason. | 1, 3 | High: the count changes | tests 15-18, 37, 38 |
| 5 | The floor reads of 7.1 move to `repostate`; `floorBase` loses `HEAD`; `commentaudit.ReadAtBase` moves; `changedpkgs` gets the dot-prefix rule. Each floor entry gets a named run function and the lane ruler, and the rerun methods move to the base-red ruler. `covchanged` gets the plain profile reader and `FuncPercents` feeds the floor. Each `go` site of 7.1 calls `gotool`, and the process-group pair moves to `sysexec`. The message records the refusals of the real registry by edge kind. | 1, 2b, 4 | High: a floor that skips with no base | the floor suites; tests 39, 40, 44, 47 |
| 6 | the guard on the real registry: test 10 and its `TreeReadingTests` row | 3, 4, 5 | Medium: a refusal that commits 4 and 5 did not remove | tests 10, 11 |
| 7 | `phasecoherence`: `treegate.go`, `seed_phasecoherence.go`, the fixtures; two `baseline.json` rows go | 1, 6 | Medium: an import cycle through `prompts` | tests 19, 21 |
| 8 | `fleet`: `zonegate.go`, `seed_fleet.go`, the module fixtures | 6 | Medium: about fifteen tests move | tests 20, 21 |

Commit 4 lowers `count` to the new measure, because it now leaves out each `acs/` site. It can only go down. The commit message gives the old and the new value. Test 10 goes in after commit 5, because the three floor entries are closures until then, and rule 1 refuses a closure.

### 7.12 How the files stay protected

G2 adds these `GateGuarantee` rows to `guards.ProtectedSurfaceManifest`:

- `/go/internal/repostate/`: each gate trusts its tree, so a lane that changes it can show a gate another tree.
- `/go/internal/gatehermetic/` and `/go/internal/goload/`: the guard and the loader that it trusts.
- `/go/internal/gotool/`: the one place where a gate starts the Go toolchain, so a lane that changes it can add a verb.
- `/go/internal/phasecoherence/treegate.go` and `/go/internal/fleet/zonegate.go`: the `Files` of the two entries. G1 test 6 requires them.

The test files of the guard are in `gateregistry` and `gatehermetic`, both on the manifest. `repocontract.go` is on the manifest since G1. After G2, a lane cannot add a `repostate` function or loosen `Contract()`. A lane that needs a new git fact sends a held proposal, and the console lands it. The console builds G2 itself, because `testdup`, `rawgitratchet/baseline.json`, `gateregistry` and `repocontract.go` are protected since G1.

## Risks

- **A false refusal.** The live-type rule reaches each method of each type in the structure of a type that a gate names. Next step: the refusal chain names the type and the edge kind, and the gate names a smaller type. No allow list is added.
- **A missed reach.** `reflect`, `plugin` and `go:linkname` are outside the static reach. Review checks them (contract section 5).
- **The real-tree `Base` is `HEAD`** until G3. The staged lane is stricter than main, never laxer, so it cannot turn main red.
- **Floor scope.** G4 lands first, so commit 5 must cover the three floor entries that G4 registers. Test 10 lists each refusal, and that list is the work list of commit 5. Each other floor that commit 5 moves has no entry, so the guard does not cover it. Its own suite holds the `repostate` call until G5 registers it.
- **The `goload` move** touches an adapter that the console uses. Test 22 holds it.

### 7.13 Limits of G2

- G2 proves hermeticity, not parity. G3 runs the three states.
- 13 debt keys stay for G3.
- `go/.test-dup-baseline` stays a lane-writable count until G6. G2 removes the date, which was its second off switch.
- A check that needs `HEAD`, ancestry or a second commit cannot become an entry (question 1 of 7.14).
- The child process of a `gotool.Test` call is outside the static reach. A test that the child runs can read git. G3 parity controls that child, because it runs the entry in three states.
- The guard does not judge the `Args` of a `gotool` call. A flag such as `-exec` can start a program in the child. Review checks each argument builder.
- A `.git` path that a body makes at run time from parts that are not constants is not seen. Review checks it.
- `Fresh` is empty when `HEAD` is not the lane base, because no `acs/` directory then changes against `Base`. The `acs/` ratchet is thus lane-side only until G3 names the base of a main-side run.
- One load sees one build configuration, which is that of the host with `CGO_ENABLED=0`. Rule 8 refuses a package that has more than one.

### 7.14 Open questions for the operator

The design reviews of 2026-11-10 gave a recommendation for each question. The operator has not ruled on questions 1 and 3. The design holds the recommendation of questions 2 and 4.

The operator approved the toolchain boundary on 2026-11-11. Three review rounds refused the first `exec` rule, which read a constant verb at each gate site. The commit `c2cc026c3` holds the revision after rounds 1 and 2, and round 3 refused that revision.

1. **Checks that read history.** `lost_landing_floor.go:76-96`, `explanationdocs/gitio.go:178,262` and `rebind.go:132,176` read `HEAD` or ancestry by design: they witness a landing. Recommended: these are landing witnesses, not lane-tree gates. They stay out of the registry. One typed list in one home names them, and the guard test reads that list. Commit 5 proves that no floor entry reaches them.
2. **The mtime freshness of `covchanged`** (`workspace.go:129-140`). The review closed this question, and 7.8 holds the result. The floor measures in the same run and takes a plain reader. The check of an older profile leaves `Workspace`. A SHA-256 of each source is an option only if Go code in the tree writes the profile. This fact is ASSUMED.
3. **The `fleet` key.** Its judges read no git, so section 6.5 gives the wrong reason for G2. Recommended: keep it in G2 as the last commit, with no change of the preflight. The commit also corrects the reason in section 6.5. The other choice is to move it to G3 with the other 13 keys.
4. **The base-red rerun in a floor entry.** G4 put the rerun inside the entry `Run`. The rerun runs git by design: it adds a worktree of the base. Recommended: the entry gives the raw reds, and only the build-floor reviewer reruns (7.8). The other choice needs an exemption for `baseattr` in `Contract()`.
