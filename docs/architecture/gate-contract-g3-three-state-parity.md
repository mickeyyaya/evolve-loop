# Component G3: the three-state parity harness

> Plan row G3 of [gate-contract-2026-11.md](../plans/gate-contract-2026-11.md). The contract and the G1 design are in [gate-contract.md](gate-contract.md), and its section 7 links this page. The headings here keep the numbers 8.1 to 8.16. Inbox item `gate-contract-g3-three-state-parity`. ADR-0165 decisions 2 and 5. The measures are from `ce4189a2c` (2026-11-10), the tree that holds G1 and G4.

## Current State

### 8.1 What the tree holds today

| Fact | Where | Measure |
|---|---|---|
| `testdup` marks an `acs/` directory as touched when `git diff HEAD` names it. | `go/internal/testdup/testdup.go:106` | one `HEAD` diff |
| `testdup` also reads commit times (`git log --format=@%ct`) and file mtimes. | `testdup.go:96-105`, `testdup.go:127-140` | two more undeclared inputs |
| The registry holds six entries: three `scanner` seeds and three `phase-floor` entries at `enforce`. Each floor reads `Base` and runs `go test`. | `go/internal/gateregistry/gateregistry.go:10-12`, `seed_floors.go:18-54` | `repo-contract-floor`, `unit-test-floor`, `cover-changed-floor` |
| `rawgitratchet` binds the files of `git ls-files`, the index only. An untracked new test file is outside it. | `go/internal/rawgitratchet/rawgitratchet.go:27`, `go/internal/repostate/tracked.go:32` | unstaged and staged can differ |
| The raw-git ratchet counts the `"init"` literals of a `_test.go` file that also holds a `"git"` literal. | `rawgitratchet.go:126-158` | a `gittest.Repo.Git` call adds no site |
| The git fixture helper: `gittest.Fixture`, `FromTemplate`, `Clone`, `(*Repo).Git`, `(*Repo).Write`. `Git` sets no commit date. | `go/internal/gittest/fixture.go:47,73,104`, `template.go:44,118` | no date seam |
| The ship gate runs the 15 pack patterns and the 141 named tests as `go test`. | `go/internal/phases/ship/repocontract.go:30-32,81-84` | a `go test` subprocess |
| The ship gate passes the lane base to the pack log, the base-red check, the added-test backstop and the main-red stop. No scanner gets it. It falls back to the literal `"HEAD"`. | `go/internal/phases/ship/ship.go:142-147,197-205`, `repocontract.go:441-477` | `"HEAD"` moves at commit |
| The build floor runs the same pack with no base. | `go/internal/core/repo_contract_floor.go:19-45`, `cmd/evolve/cmd_cycle_config.go:51` | `RepoContractPackFn(ctx, projectRoot, root)` |
| `ReviewInput` already carries the lane base. | `go/internal/core/reviewer.go:16` | `WorktreeBaseSHA` |
| CI computes a base twice: the PR base or `github.event.before`, else `HEAD~1`. | `.github/workflows/go.yml:55-62`, `go.yml:186-193` | no gate reads it |
| The required workflow uses the merge-base for a pull request. | `.github/workflows/required.yml:36-57` | a 40-hex check |
| The train holds `Batch.Base` and diffs each layer against it. No production code builds a `BatchTest` today. | `go/internal/integration/compose.go:23-30`, `batchtest.go:136` | shadow (ADR-0147) |
| `evolve ratchet check` runs `size` and `rawgit` through their combined `Scan`. | `cmd/evolve/cmd_ratchet.go` (`ratchetScans`), `cmd/evolve/cmd_floor.go:55` | a second judge path |
| `gateregistry` imports `core` and `phases/ship`. Its dependency list holds `policy`, `repocontract`, `guards`, `phasespec`, `profiles`, `ipcenv`, `acssuite`, `inboxrank` and `integration`. | `gateregistry/seed_floors.go:4-6`, `go list -deps ./internal/gateregistry` | an in-package test of these packages cannot import the registry |
| The train `Gate` type takes a tip and a tier, and no base. | `go/internal/integration/compose.go:32,197` | the base is per batch |
| A `//go:build integration` tier exists. CI runs it, and the fast lane floor does not. | `go/Makefile:56,107,125` | — |

### 8.2 The 13 pack patterns that G3 registers

The tree judges come from the pack detector (`sharedTreeScan(t).Reading`, run on a copy of the module). The detector names no test in `phasespec`, `profiles` and `routingtest`, because they read the repository root. Their counts come from a grep of the repository-root helpers (`repoRoot(`, `realTreeProfiles(`, `"..", "..", ".."`). Those three counts are approximate.

| Pattern | Tree judges | Where the logic is | Work |
|---|---|---|---|
| `rawgitratchet` | 1 | the G1 entry | set `Covers` |
| `sizeratchet` | 2 | the G1 entry; `TestEncode_...ByteForByte` calls `moduleRoot(t)` | one judge into the entry |
| `testmainexit` | 1 | exported `SkippedDefers` | small |
| `inboxrank` | 1 | `weightOrderingSites(t, …)` in a test file | small |
| `policy` | 1 | inline in `TestParamPackages_EnvAgnostic` | small |
| `routingtest` | 1 | exported `phasespec` calls | small |
| `repocontract` | 7 | `sharedTreeScan(t)` and the detector, in test files | medium |
| `acssuite` | 3 | test-file walks | small |
| `evalqualitycheck` | 2 | `corpusProofFindings(t, …)` | small |
| `ipcenv` | 3 | `parseTree(t, …)` | small |
| `guards` | 15 | helpers that take `t` | large |
| `phasespec` | about 17 | `repoRoot()` helpers | large |
| `profiles` | about 24 | `realTreeProfiles(t)` and others | large |

About 78 judges hold their logic in a `_test.go` helper that takes `t`. A run function cannot call a test helper. So each judge moves to a non-test function that returns its findings and an error. That is about 78 moves, and they do not fit one pull request.

## Requirements

| # | Requirement | Level |
|---|---|---|
| R1 | Each registered entry runs on one fixture in three states with one `Base`. The verdicts match. | Must |
| R2 | The current `testdup` fails the harness: its `HEAD` diff gives two verdicts. It passes after its G2 change. | Must |
| R3 | A gate that diffs against `HEAD` fails the harness. | Must |
| R4 | A fixture with an untracked `acs/cycle<N>/` that holds a copied helper is red in all three states. | Must |
| R5 | `gateregistry.Unregistered()` is empty at the end of G3. | Must |
| R6 | The harness makes the states with no raw `git init` site. | Must |
| R7 | Each CI run and each lane run pass a named `Base`. No run passes `"HEAD"`. The train is not in G3 (8.14). | Must |
| R8 | `Stage` selects what the ship gate and the build floor block on. | Must |
| R9 | No import cycle. `core` and `integration` get the registry by injection. | Must |
| R10 | The harness is in the integration tier, and the lane floors stay fast. | Must |
| R11 | Each pull request of the split is green by itself. | Must |
| R12 | The 141 named tree-reading tests get a home that can only get smaller. | Wanted |

## Design

### 8.3 Approaches

| Approach | ADR fit | Blast radius | Simplicity | Score (of 5) |
|---|---|---|---|---|
| A. One run function per judge, moved to a non-test file. One `gatetest` harness on `gittest` templates. In-process judges at ship and the floor. A `gate check` verb for CI. (chosen) | holds 1, 2, 3, 5: each judge is a function, and G2's guard can reach it | six pull requests, each green | medium | 4.6 |
| B. A generic run function that runs `go test -run '^(names)$' <pattern>` in `Root` and maps the failed names to reds | breaks the ADR definition of a gate; G2's guard stops at the `go` subprocess | small | high | 2.4 |
| C. One run function per pattern that calls all judges of that pattern | holds the definition, but one entry mixes owners and inputs | large reviews | medium | 3.3 |
| D. A harness that copies the real module into each fixture and runs the pack per state | no code moves | very slow: three `go test` runs of 13 patterns | low | 1.8 |

Ranking: correctness first. B and D do not let G2's guard see the judge, so they fail decision 3. A and C pass. A wins on simplicity of each entry: one entry has one rule, one owner list and one input list. A groups judges that share one scanner, such as the three `ipcenv` spawner tests, into one entry.

## Decision

Approach A. Each tree judge becomes a function in a non-test file. The registry entry wraps it. One table test of `gateregistry` calls `gatetest.RealTree` for each entry (8.4). The harness `gatetest.Parity` runs each `scanner` entry on its fixtures.

The ship gate and the build floor run the selected `scanner` entries in process. CI calls `evolve gate check --base <commit>`.

Not in G3:

- G3 does not migrate the 141 named tree-reading tests. It gives them a debt count that can only get smaller (section 8.9).
- G3 does not change what a judge decides. Each move keeps the verdict of the real tree.
- G3 does not wire the train. ADR-0147 owns that wiring (8.14).

### 8.4 The packages and the dependency direction

| Package | Holds | Imports |
|---|---|---|
| `gatecontract` (G1) | the types, and the new `Scope` words | standard library |
| `gaterun` (new) | `Judge`, `Select`, the `Result` and the base rule | `gatecontract` |
| `gateregistry` (G1) | `Entries()`, `Unregistered()`, the run functions, and the new `UnregisteredTests()` | `gatecontract`, `core`, `phases/ship`, each gate package |
| `gatetest` (new, a test helper) | `Parity`, `RealTree`, `Fixture` | `gatecontract`, `gaterun`, `gateregistry`, `gittest`, `testing` |
| `phases/ship` | the in-process judge step | `gatecontract`, `gaterun` |
| `cmd/evolve` | the composition root and the `gate check` verb | `gateregistry`, `gaterun`, `ship` |

- Only `cmd/evolve` and `gatetest` import `gateregistry` in a non-test file. A test pins that list.
- `ship.Config` gets the entries and the debt patterns from `withRegistryGates` at the composition site `cmd/evolve/cmd_cycle.go:523` (8.5). `core` gets a function value, as it gets `repoContractPack` today (`cmd_cycle_config.go:51`).
- The real-tree run of each entry is one table test in the external test package `gateregistry_test`. Each gate package keeps its fixture tests only.
- No other test file imports `gatetest`, and test 11 pins it. An in-package test of a package that `gateregistry` imports is an import cycle.
- `integration` never imports the registry, because `core` imports `integration` (`core/lost_landing_floor.go`) and the registry imports `core`. Its `Gate` type takes no base (`compose.go:32`), so G3 does not wire the train (8.14).
- `gatetest` joins `testOnlyHelpers()` (`go/internal/repocontract/testhelper_imports_test.go:15`). Production code cannot import it.
- G3 adds no setting. `Stage` is registry data on the protected surface, not policy. The whole-gate switch stays the policy value that `repocontract.GateOn` reads.

### 8.5 The types and the signatures

G3 adds one field to the G1 entry. A gate that reads `Base` says so.

```go
package gatecontract

type Entry struct {
	Name   string
	Kind   string
	Scope  string
	Files  []string
	Covers []string
	Owners []Owner
	Inputs []Declared
	Stage  string
	Run    func(context.Context, Input) (Verdict, error)
}

func Scopes() []string
```

- `Scope` is `tree` or `diff`. A `tree` gate ignores `Base`. A `diff` gate diffs the lane tree against `Base`.
- After G2, `testdup-ratchet` is the one `diff` scanner (ASSUMED until G2 lands). Each phase floor is `diff` also.
- `Scope` is a required field. G3a adds the check "unknown scope" to `entryProblems` (`gateregistry_test.go:124`), which is the G1 test of each entry.

```go
package gaterun

type Outcome struct {
	Name    string
	Stage   string
	Verdict gatecontract.Verdict
	Err     error
}

type Result struct {
	Outcomes []Outcome
}

func Select(entries []gatecontract.Entry, kind string) []gatecontract.Entry
func Judge(ctx context.Context, e gatecontract.Entry, in gatecontract.Input) Outcome
func JudgeAll(ctx context.Context, entries []gatecontract.Entry, in gatecontract.Input) Result
func (r Result) Blocking() []Outcome
func (r Result) Undecided() []Outcome
func (r Result) Shadowed() []Outcome
```

- `Select` takes the kind only. Ship and `gate check` select `scanner` entries. Each caller judges both stages, and `Result` splits them with `Blocking`, `Undecided` and `Shadowed`.
- `Judge` refuses a `diff` entry with an empty `Base`. The error is "did not judge", not a red.
- `Blocking` holds each `enforce` outcome with a red. `Undecided` holds each `enforce` outcome with an error. `Shadowed` holds each `shadow` outcome with a red or an error.

```go
package gatetest

type Fixture struct {
	Name   string
	Dir    string
	Remove []string
}

type State string

const (
	AtBase    State = "at-base"
	Unstaged  State = "unstaged"
	Staged    State = "staged"
	Committed State = "committed"
	Cloned    State = "cloned"
	Rebased   State = "rebased"
	Redated   State = "redated"
)

type Run struct {
	State   State
	Verdict gatecontract.Verdict
	Err     string
}

type Report struct {
	Entry   string
	Fixture string
	Runs    []Run
}

func Parity(tb testing.TB, e gatecontract.Entry, f Fixture) Report
func (r Report) Problems() []string
func RealTree(tb testing.TB, name string) gatecontract.Verdict
```

```go
package gittest

func (r *Repo) GitAt(when time.Time, args ...string) string
```

```go
package core

type RepoContractPackFn func(ctx context.Context, projectRoot, root, base string) (reds []string, diagnostic string, err error)
```

```go
package gateregistry

func UnregisteredTests() []repocontract.TestSelection
```

`ship.Config` gets two fields: `Gates []gatecontract.Entry` and `Pack []string`.

```go
package ship

type PackInput struct {
	ProjectRoot string
	Root        string
	Base        string
	Gates       []gatecontract.Entry
	Pack        []string
}

func RunRepoContractPack(ctx context.Context, in PackInput) (reds []string, diagnostic string, err error)
func repoContractGateRoot(ctx context.Context, opts *Options) (root, base string, err error)
```

```go
package repocontract

func Readers(root string) (map[string][]string, error)
func Importers(root string, forbidden []string) ([]string, error)
func TestImporters(root string, forbidden []string) ([]string, error)
```

```go
package gateregistry

func PackSelection() (gates []gatecontract.Entry, pack []string)
func PackInputFor(projectRoot, root, base string) ship.PackInput
func RepoContractPack(ctx context.Context, projectRoot, root, base string) (reds []string, diagnostic string, err error)
```

Each name below is new in G3, or it changes its shape:

- `RunRepoContractPack` takes three strings today (`repocontract.go:108`). Three sites use it: its own file, `cmd_cycle_config.go:51` and `gateregistry/seed_floors.go:23`. One function builds the `PackInput` for the last two sites (see "One home for the pack input").
- The pack gives `Base` to each selected scanner. Its `go test` part does not read `Base`.
- `repoContractGateRoot` takes no context today (`ship.go:197`). It gets one, because it reads the commit of `HEAD` (8.8). It has two callers: `runRepoContractGate` and `main_red_stop.go:32`.
- `Readers` holds the logic of `treeScan.Reading` (`repocontract/treescan_test.go:29,124`). `Importers` holds the logic of `productionImporters` (`testhelper_imports_test.go:20`), which skips each `_test.go` file. `TestImporters` uses the same walker and reads only the `_test.go` files.
- `SHIP_GATE_SHADOW` is a new warning code. Ship registers it with `signalcenter.RegisterCode` and `ModuleShip`, as `held_proposal_gate.go:23` does.
- The verb `evolve gate check` is new. Its file is `cmd/evolve/cmd_gate.go`, and its function is `runGate(args []string, _ io.Reader, stdout, stderr io.Writer) int`. That is the shape of `runRatchet` (`cmd_ratchet.go:28`). Its flags are `--base`, which is required, and the root flag of `ratchet check`.
- `ratchet check` loses the `ratchetScans` map (`cmd_ratchet.go:18`). It selects the registry entries by name and judges each one with `gaterun.Judge`. It reads the entries from `gateEntries`, a package variable of `cmd/evolve` that a test can replace, as `repoContractPack` is (`cmd_cycle_config.go:51`).

**One home for the pack input.** `PackSelection` returns the `scanner` selection of `Entries()` and the patterns of `Unregistered()`. `PackInputFor` is a pure function that builds the one `PackInput` from `PackSelection`; `RepoContractPack` calls `ship.RunRepoContractPack` with it. The entry at `seed_floors.go:23` uses it, and `cmd_cycle_config.go:51` sets `repoContractPack` to it. The function `withRegistryGates(cfg ship.Config) ship.Config` of `cmd/evolve` fills `Gates` and `Pack` from `PackSelection`, and `cmd_cycle.go:523` calls it. So the registered floor, the production floor and the ship gate judge the same set; tests 26 and 29 pin it.

**The phase-floor entries.** The three floors stay in the registry with the kind `phase-floor` and the scope `diff`. The build-floor reviewer runs them, as it does today. Ship and `gate check` never select them, so the ship gate does not run the pack inside itself.

The floors have no parity fixture. Each floor runs `go test` on a module, so one fixture costs six `go test` runs. Test 1 holds the three floor names in a literal exempt list, and that list can only get smaller. The state `rebased` applies to a `tree` entry only, so it never applies to a floor.

### 8.6 The fixture model

A fixture is a directory under `go/internal/gateregistry/testdata/parity/<entry>/<fixture>/`. It holds `base/` and `change/`. Each file path is relative to the repository root, so a Go file is under `go/`. Each fixture holds a `go/go.mod` with the evolve-loop module path. The scanners skip `testdata`, so a copied helper in a fixture never reaches the real-tree verdicts.

`Parity` runs these steps in one repository directory, except the clone of step 7. `later` is 2001-01-01T00:00:00Z, a second constant of `gatetest`.

1. Build a template once for each fixture with `gittest.FromTemplate`. Copy `base/`, run `git add -A`, and commit with `GitAt(epoch)`. `epoch` is 2000-01-01T00:00:00Z, a constant of `gatetest`.
2. Read `Base` from the copy with `r.Git("rev-parse", "HEAD")`. This is the harness, not the gate.
3. Set the mtime of each file to `epoch`. Run the entry: state `at-base`.
4. Copy `change/` over the tree, delete each `Remove` path, and set each mtime to `epoch`. Run: state `unstaged`.
5. Run `git add -A`. Run: state `staged`.
6. Commit with `GitAt(epoch)`. Run: state `committed`.
7. `gittest.Clone` the directory, and set each mtime of the clone to `epoch`. Run on the clone with the same `Base`: state `cloned`. This is the main-side run.
8. For a `tree` entry only, run the committed tree with `Base` set to the new commit: state `rebased`.
9. Change the date of the last commit to `later` with `GitAt(later)`, and set each mtime to `later`. Run with the same `Base`: state `redated`.

The state `redated` isolates the date, as `rebased` isolates `Base`. A date reader or an mtime reader gives one verdict for `committed` and a different one for `redated`. One fixed instant alone does not show such a reader.

`gittest.Clone` makes a second directory (`gittest/fixture.go:73`). So `Parity` replaces each `Root` in the reds, the notes and the error text with the token `<root>` before it compares. The harness calls only `gittest` methods, so it adds no `"init"` site.

### 8.7 What "the verdicts match" means

`Problems()` returns each of these, with the entry, the fixture and both states:

| Check | Problem text names |
|---|---|
| `unstaged`, `staged`, `committed` and `cloned` have the same sorted `Reds`, the same sorted `Notes` and the same error text. | the first differing pair |
| `rebased` equals `committed` for a `tree` entry. | "declares tree and reads Base" |
| `redated` equals `committed`. | "reads a date or an mtime" |
| `at-base` has no red and no error. | "the fixture base is red" |
| `committed` has at least one red. | "the fixture proves nothing" |
| No state has an error. | "the entry did not judge the fixture" |

- `Notes` count, because a note is output of the gate. A note that differs shows a state-dependent read.
- An error in each state is not parity. The fixture is broken, and the harness says so.
- The error text compares exactly after the `<root>` token replaces each `Root` (8.6). So the path of the clone gives no false problem.

### 8.8 The base of each run

| Run | `Base` | Code that passes it |
|---|---|---|
| lane build floor | `ReviewInput.WorktreeBaseSHA` (`core/reviewer.go:16`) | `RepoContractFloorChecks` passes it to `RepoContractPackFn` |
| lane ship gate | `opts.WorktreeBaseSHA` (`ship.go:201-202`) | `runRepoContractGateAt` |
| ship on the project root | the commit of `HEAD` before the commit, read once with `captureGitOutput(ctx, opts, "rev-parse", "HEAD")` (`audit.go:84`) | `repoContractGateRoot` returns a commit id, never `"HEAD"` |
| CI push | `github.event.before`; else `HEAD~1` | one new step `gate-base` in `go.yml` writes it to `$GITHUB_OUTPUT` |
| CI pull request | `git merge-base <base.sha> <head.sha>`, as `required.yml:57` | the same step |
| the train | `Batch.Base` (`compose.go:27`) | not in G3: the `Gate` type takes no base (`compose.go:32`), and ADR-0147 owns that change |
| a real-tree `_test.go` | none | `RealTree` refuses a `diff` entry |

- The `gate-base` step replaces the two copies at `go.yml:55-60` and `go.yml:186-191`. `cover-changed` reads the step output.
- `evolve gate check` requires `--base` as a 40-hex commit id. It refuses a ref name, so no run passes a moving `HEAD`.
- A `diff` scanner has no `go test` judge. Its main-side judge is `evolve gate check` in CI.
- A phase floor has no main-side judge in G3. CI runs the full suites on main.

### 8.9 The 141 named tree-reading tests

G3 keeps them as `go test` selections. The count 141 and the estimate for `cmd/evolve` are measures of design review round 1 at `ce4189a2c`. The contract page holds the older count 139, and G3a corrects it to 141.

`gateregistry.UnregisteredTests()` returns `repocontract.TreeReadingTests()`. A `gateregistry` test holds the count at a literal ceiling. G3c measures the count and records the ceiling, and a migration lowers it in the same commit. The pack detector refuses a reading test that no selection names, so the set cannot grow. A later plan item moves them; `cmd/evolve` holds about 87 of them, and `package main` cannot be imported.

A migrated test leaves its package, and the table test of `gateregistry` judges the real tree. G3 teaches the detector that a `RealTree` call reads the tree through the registry. So the count falls.

### 8.10 Behavior steps and their failure signals

| Step | Failure | Cause | Evidence | Next step |
|---|---|---|---|---|
| Ship selects the `scanner` entries and runs them in process | an `enforce` red | the lane tree breaks the rule | `shiperr.CodeRepoContractGate` with the entry name and its reds | the builder fixes the change |
| — | an `enforce` error | the gate did not judge (an unreadable baseline, no base) | `shiperr.CodeRepoContractInfra` with the entry name and the error | re-dispatch; a repeat is an inbox item |
| — | a `shadow` red or error | a new gate at shadow | a scan log line and a `SHIP_GATE_SHADOW` warning signal | no block; the operator reads it before a stage move |
| Ship runs `go test` on `Pack` and the named tests | a red | as today | as today (`repocontract.go:596`) | as today |
| The build floor runs the same function with the lane base | a red | as at ship | a floor red with the entry name | the builder fixes it before handoff |
| — | an empty base | an old `run.json` | a floor WARN; ship judges again | none |
| `evolve gate check` | exit 1 | an `enforce` red | one line for each red | fix main through the G4 path |
| — | exit 0 and one line | a `shadow` red or a `shadow` error | the line | no block; the operator reads it before a stage move |
| — | exit 2 | an `enforce` error, or a `--base` that is not a commit id | the error | fix the workflow or the input |
| Ship, the gate is on | `Gates` and `Pack` are both empty | a `Config` with `RepoContractGate` on, no `Gates` and no `Pack` | `shiperr.CodeRepoContractInfra` "no judge selected" | fix the composition; never a pass |
| `Parity` | a problem of 8.7 | the gate reads an undeclared state | the two runs side by side | make the gate hermetic (G2), or fix the fixture |

## Blueprint

### 8.11 The tests (each one is red without its step)

| # | Test | Entry point | Red case |
|---|---|---|---|
| 1 | `TestParity_EachEntryHasAFixtureAndHoldsParity` (integration tier) | `gatetest.Parity` on each `scanner` of `gateregistry.Entries()` | a scanner with no fixture; any problem of 8.7; a fourth name in the exempt list |
| 2 | `TestParity_TheTestdupFixtureIsRedInEveryState` | `Parity` on `testdup-ratchet/new-acs-copy` | the current `testdup`: `staged` red and `committed` green (red proof run on the G1 tree, recorded in the pull request) |
| 3 | `TestParity_AGateThatDiffsAgainstHEADFails` | `Parity` on a fake entry that reds each path of `git diff --name-only HEAD` | the harness passes it |
| 4 | `TestParity_AnUntrackedAcsDirWithACopiedHelperIsRedInAllStates` | `Parity` on `testdup-ratchet/tdd-untracked-acs` | an index-only read: `unstaged` green |
| 5 | `TestParity_ADateReaderAndAnMtimeReaderFail` | `Parity` on two fake entries. One reds each file with a last commit date after 2000-06-01. The other reds each file with an mtime after that instant. | a harness with no `redated` state |
| 6 | `TestParity_AnIgnoredFileThatADiskWalkerJudgesFails` | `Parity` with a `.gitignore`d decoy | a harness with no `cloned` state |
| 7 | `TestParity_ATreeEntryThatReadsBaseFails` | `Parity` on a fake `tree` entry that reads `Base` | a harness with no `rebased` state |
| 8 | `TestUnregistered_IsEmpty` | `gateregistry.Unregistered()` | any pattern left (last pull request) |
| 9 | `TestCovers_ACoveredPatternHasNoUnjudgedTreeReader` | `repocontract.Readers` on the real module | a covered pattern with a raw tree-reading test |
| 10 | `TestUnregisteredTests_CountIsTheCeiling` | `UnregisteredTests()` | a selection above the ceiling; a migration that leaves the count |
| 11 | `TestRegistryImporters_AreTheRootAndTheHarness` | `repocontract.Importers` and `repocontract.TestImporters` | `core` or `integration` imports `gateregistry`; a test file outside `gateregistry_test` imports `gatetest` |
| 12 | `TestJudge_ADiffEntryWithNoBaseDidNotJudge` | `gaterun.Judge` | a red or a pass for an empty base |
| 13 | `TestShipGate_AShadowRedLogsAndAnEnforceRedBlocks` | `Phase.runRepoContractGate` on a `gittest` worktree with five fake entries. They are a `shadow` red, a `shadow` error, an `enforce` red, an `enforce` error, and one that records `Input.Base` | a gate that ignores `Stage`. A ship that blocks on the `shadow` error, or that emits no `SHIP_GATE_SHADOW` signal for it. An empty `Base` on the lane path. An `enforce` entry error that does not give `CodeRepoContractInfra` with the entry name |
| 14 | `TestShipGate_TheRootPathPassesACommitIDNeverHEAD` | `repoContractGateRoot` | the `"HEAD"` literal |
| 15 | `TestRepoContractFloor_PassesTheLaneBase` | `core.RepoContractFloorChecks` | a floor that drops `WorktreeBaseSHA` |
| 16 | `TestGateCheck_RefusesARefNameAndExitsOneOnAnEnforceRed` | `evolve gate check` through `runGate` on a fixture repository with the real registry, and with a fake `shadow` red and a fake `shadow` error through `gateEntries` | a verb that accepts `HEAD`; a `shadow` red or error that does not exit 0, or that prints no line for it |
| 17 | `TestGoWorkflow_EveryBaseComesFromTheGateBaseStep` | a parse of `go.yml` | a second base computation; no `gate check` step |
| 18 | `TestGitAt_SetsTheAuthorAndCommitterDate` | `gittest.(*Repo).GitAt` | a commit at the host time |
| 19 | `TestRealTree_RefusesADiffEntry` | `gatetest.RealTree` | a `diff` judge in `go test` |
| 20 | For each migrated judge: its fixture tests move with it, and the table test of `gateregistry` holds its real-tree run | the judge function | a judge that changes its real-tree verdict |
| 21 | `TestGatetest_HoldsNoRawGitInitSite` | the site counter of `rawgitratchet` on the non-test files of `gatetest` | a `"git", "init"` call in `parity.go` |
| 22 | `TestParity_EachCallerIsInTheIntegrationTier` | a parse of each file that calls `gatetest.Parity` | a caller with no `//go:build integration` line |
| 23 | `TestRepoContractPack_AShadowRedIsNotAFloorRed` | `ship.RunRepoContractPack` with a `shadow` fake, an `enforce` fake and a `diff` fake | a floor that blocks on the shadow red; a `diff` entry with an empty `PackInput.Base` that gives a red and no error |
| 24 | `TestRatchetCheck_JudgesThroughTheRegistry` | `runRatchet` with a fake red entry in `gateEntries` | the old combined `Scan` path |
| 25 | `TestParity_ANoteThatHoldsRootGivesNoProblem` | `Parity` on a fake entry that puts `Root` in a note | a false "differs" problem for `cloned` |
| 26 | `TestRepoContractPack_GivesThePackTheRegistrySelection` | `gateregistry.PackInputFor`, and the value of `repoContractPack` in `cmd/evolve` | a `PackInput` with no `Gates`; a `Pack` that differs from `Unregistered()`; a dropped `Base`; a `repoContractPack` that is not `gateregistry.RepoContractPack` |
| 27 | `TestRepoContractFloor_AnEmptyBaseIsAWarnNotARed` | `core.RepoContractFloorChecks` with an empty `WorktreeBaseSHA` | a floor that gives a red, or no line, for the "did not judge" error |
| 28 | `TestGateCheck_AnEntryErrorExitsTwo` | `runGate` with a fake `enforce` entry error in `gateEntries` | a verb that exits 0 or 1 when an entry did not judge |
| 29 | `TestShipConfig_GetsTheRegistrySelection` | `withRegistryGates` in `cmd/evolve` with a `ship.Config` that has `RepoContractGate` set | a `ship.Config` with no `Gates`; a `Pack` that differs from `PackSelection()`; a field of the input that the result drops |
| 30 | `TestRepoContractGate_AnEmptySelectionIsNotAPass` | the ship gate with a `Config` that has `RepoContractGate` on, no `Gates` and no `Pack` | a gate that passes when `Gates` and `Pack` are both empty |
| 31 | `TestResult_SplitsEachStageAndOutcome` | `gaterun` with six fake outcomes: a pass, a red and an error for each of `enforce` and `shadow` | a method that does not return exactly its set. `Blocking` is the `enforce` red, and `Undecided` is the `enforce` error. `Shadowed` is the two `shadow` outcomes |

Test 20 has one instance for each moved judge, in G3c to G3f. Tests 9, 11, 17, 21 and 22 and the table test read the real tree. Each one joins the `./internal/gateregistry` selection of `TreeReadingTests()` in its own PR, before G3c records the ceiling.

Tests 13, 16, 24 and 27 go through the real entry points of the pipeline: the ship phase and the CLI verbs. Test 20 keeps 100% coverage of each moved function. The existing ratchet does not see `gatetest`, because it counts sites in `_test.go` files only (`rawgitratchet.go:125-158`). Test 21 closes that gap for R6, test 22 pins R10, and test 23 pins R8 for the build floor.

### 8.12 The build order (six pull requests, each green)

| PR | Steps | Depends on | Risk | Verifiable by |
|---|---|---|---|---|
| G3a | `gittest.GitAt`; `Scope` and `Scopes()`; `gaterun`; `gatetest`. The fixtures of the three scanner entries, with `new-acs-copy` and `tdd-untracked-acs`, and the exempt list of the three phase floors. The table test replaces the three G1 real-tree files (`rawgitratchet/gate_realtree_test.go`, `sizeratchet/gate_realtree_test.go`, `repocontract/testdup_gate_realtree_test.go`). The count 141 on the contract page. Tests 1-7, 12, 18, 19, 21, 22, 25, 31. | G1, G2 | Medium: a seed fails parity because G2 left a git read | tests 1-7 |
| G3b | `ship.Config.Gates` and `Pack`. Ship runs `enforce` and `shadow` in process, and the go-test pack is `Pack`. The base column of 8.8, `PackInput`, and `RepoContractPackFn` gets `base`; `repocontract.Importers` and `TestImporters` move to a non-test file; `gateregistry.PackSelection`, `PackInputFor` and `RepoContractPack`. `evolve gate check`, and `ratchet check` judges through `gaterun`; the `gate-base` step in `go.yml`. `ship` stops reading `repocontract.Packages()`, which stays for the G1 split test until G3f; test 26 replaces `TestRepoContractPackSeam_DefaultsToShipsOwnPack`. Tests 11, 13-17, 23, 24, 26-30. | G3a | High: ship time and heap. Measure against the pack time that `scanShadow` records | tests 13-16 |
| G3c | `repocontract.Readers` moves to a non-test file. `TestRepoContract_PeakHeapUnder400MB` joins `TreeReadingTests()`; `UnregisteredTests()` and its measured ceiling. The small patterns: `rawgitratchet`, `sizeratchet`, `testmainexit`, `inboxrank`, `policy`, `routingtest`. The commit that moves a test changes each reference to its name, such as `TestRatchet_NoNewRawGitFixtures`. Tests 9, 10. | G3b | Low | tests 1, 9 |
| G3d | `repocontract` (seven), `acssuite`, `evalqualitycheck`, `ipcenv` | G3c | Medium: `sharedTreeScan` parses once for two scanners (`stream_parse_test.go`) | tests 1, 9 |
| G3e | `guards` (15) | G3d | Medium: the helpers of `guards` take `t` | tests 1, 9 |
| G3f | `phasespec` and `profiles`; test 8. `repocontract.Packages()` and `repoContractPackages` go. | G3e | High: about 41 judges. Split in two if the diff passes 1500 lines | test 8 |

- Each PR lowers the literal list of G1 test 2 by the patterns it covers. G3f leaves it empty.
- A judge that measures the scanner, not the tree, is not a gate. `TestRepoContract_PeakHeapUnder400MB` is one. G3c moves it to `TreeReadingTests()` before it measures the ceiling, so the ceiling never rises.
- Each moved judge keeps its corpus floor, such as "scanned fewer than 500 files", in its row of the table test. A fixture is small, so the floor is not part of the gate.
- G3b keeps `go test` for each pattern that `Unregistered()` still names, so no PR leaves a pattern unjudged.

### 8.13 How the changed files stay protected

G3 adds these `GateGuarantee` rows to `go/internal/guards/integrity_surface.go`:

- `/go/internal/gaterun/` and `/go/internal/gatetest/`;
- `/go/cmd/evolve/cmd_gate.go`, the verb;
- `/.github/workflows/go.yml`, because a lane that removes the `gate check` step removes the main-side judge.

G1 test 6 already refuses an entry whose `Files` path has no row. So each moved judge file gets its row in its own PR. `gateregistry/testdata/parity/` is under the G1 row of `gateregistry`. `gittest` and `ship.go` keep their rows (`integrity_surface.go:155` for `ship.go`). `core/repo_contract_floor.go` has its row from G4 (`integrity_surface.go:242`).

After G3, a lane cannot change a judge, a fixture, a stage or the base step. The console builds G3 and each later change to these files, or a held proposal carries it (G1 section 6.8).

## Risks

### 8.14 Limits of G3

- G3 needs G2. A seed that keeps a git read fails test 1, and G3a cannot land before G2 fixes it.
- The harness proves parity for its fixtures only.
- The ship gate runs the gate code of the `evolve` binary, not the code of the lane tree. A lane cannot change a gate after G1, so that is the safe side. CI builds `evolve` from the tree and judges the new code.
- G3 does not wire the train. The `Gate` type takes no base today, and ADR-0147 owns that change and the wiring with `Batch.Base`.
- The three phase floors have no parity fixture. The exempt list of test 1 names them.
- G3 does not migrate the 141 named tests. A new plan item does, after `cmd/evolve` judges move out of `package main`.
- The TDD floor (G5) adds its own entry. G5 then runs the `tdd-untracked-acs` fixture against that entry.
- In-process judges can raise the peak heap of the ship process. G3b measures it and runs the entries one at a time first.

### 8.15 Open questions for the operator

| # | Question | Recommended answer |
|---|---|---|
| O1 | G3 adds `Scope` to the G1 entry. Accept the change to a G1 type? | Yes. Without it, a `go test` caller passes no base, and a `diff` gate gives a wrong main-side verdict. |
| O2 | With `Base` = `github.event.before`, an `acs/` directory of the push before is old. A count baseline that holds its copies then falls, and `Check` reds "below the baseline". Who fixes it? | G2: a copy in a new `acs/` directory is its own red with no allowance, and the count baseline counts no `acs/` file. Else G3b lands the CI step after G6. |
| O3 | Ship judges with the binary's gate code. Accept? | Yes, for the reason in 8.14. |
| O4 | Split G3 into six inbox sub-items G3a to G3f? | Yes, one item for each PR, so each gets its own red proof and review. |
| O5 | A new plan item for the 141 named tests? | Yes, after G6. It starts with the tests of `cmd/evolve`, about 87. |
| O6 | The three phase floors get no parity fixture, and test 1 holds them in an exempt list. Accept? | Yes. One floor fixture costs six `go test` runs, and the base-red rule of G4 already judges the floors against the base. The operator accepted this on 2026-11-11. |

### 8.16 ASSUMED facts (from G2, not yet in the tree)

- G2 makes `testdup` read `Base` and drops its `HEAD` diff, dates and mtimes (plan row G2).
- G2 gives `repostate` a lane-tree read (the index plus untracked files that `.gitignore` does not ignore), and `rawgitratchet.BoundTestFiles` uses it. If it stays on the index, test 4 and test 1 fail for `raw-git-ratchet`, and `testmainexit` with it.
- The G1 run functions live in `gateregistry` and call exported functions of each gate package (section 6.4).
- G2 removes `phasecoherence` and `fleet` from the debt set. G3 registers 13 of the 15 patterns of `Unregistered()` (`gateregistry.go:14-32`), so R5 holds only with that G2 change.
- The G2 decision on the `exec` class decides if a phase floor, which runs `go test`, stays a registered entry. If it does not, the exempt list of test 1 becomes empty.
