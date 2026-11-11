# Component G4: base-red attribution and the main-red stop

> Plan row G4 of [gate-contract-2026-11.md](../plans/gate-contract-2026-11.md). The contract and the G1 design are in [gate-contract.md](gate-contract.md), and its section 7 links this page. ADR-0165 decisions 10 to 14. Inbox item `gate-contract-g4-base-red-attribution`. The measures are from `92df3d291` (2026-11-10). G4 assumes that G1 lands first (the order of the plan), and the G1 and G4 code is on `feat/gate-contract-g4c-audit-attribution`, not on `main`.

## 1. Current State (what the tree holds today)

### 1.1 The three rejection points

| Point | Where | What it knows about its reds | What happens on red |
|---|---|---|---|
| Build floor, repo-contract pack | `core/repo_contract_floor.go:19-41`, wired at `cmd/evolve/cmd_cycle_config.go:48,50` to `ship.RunRepoContractPack` | `reds []string` of `pkg.Test` names (`ship/repocontract.go:59-65`), a diagnostic, an error | `floorRejection` returns `Retry: true` (`core/build_handoff_floor.go:60-69`). The builder retries on the same base. |
| Build floor, unit self-check | `core/build_floor_reviewer.go:115-145` | `selfCheckFailure{Pkg, Output}`. Test names come from `goTestFailureLines` (`core/build_floor_retake.go:151-169`). | One serialized retake (`build_floor_retake.go:92-99`), then the same retry. |
| Build floor, cover-changed | `core/cover_changed_floor.go:28-52` | The raw output of one coverage run and the package list | Same retry. PR #1260 adds base subtraction here only. |
| Audit CI-parity, integration tier | `audit/ciparitygate/tier.go:43-119` | Offender text lines (`offenders.go:41`, `tierlog.go:43-49`). No test keys. | `applyCIGate` FAILs the audit (`audit/gates.go:62-75`). |
| Audit CI-parity, acs-durable | `audit/ciparitygate` `ACSDurable`, called at `audit/ciparity.go:34-36` | Offender text lines | Same FAIL. |
| Ship repo-contract gate | `ship/ship.go:123-145`, `ship/repocontract.go:417-607` | `packFailure{Package, Test}` (`repocontract.go:47`), structured from `go test -json` | `contractRed` returns `CodeRepoContractGate` (`repocontract.go:596-607`). The cycle fails. |

No point reruns a red on the lane base today. The ship gate reruns a red package alone to catch a flake (`repocontract.go:576-594`). The build floor and the audit tier retake under a lock. Each retake runs the lane tree again, never the base.

### 1.2 The seams G4 reuses

| Fact | Where |
|---|---|
| A ship error `GIT_FLEET_REBASE_NEEDED` makes the orchestrator rebase the lane onto `main`, then run the identity carry or a new audit. A conflict routes to the debugger. | `shiperr/shiperr.go:77`, `core/ship_recovery.go:20-79`, `rebaseLane` at `ship_recovery.go:268-289`, `git rebase main` at `:384-402` |
| A resumed ship that finds `main` past the base becomes the same rebase. | `core/resume_ship_recovery.go:17-23` |
| The quota pause writes a checkpoint with a reason, ends the cycle and keeps the worktree. | `core/quota_pause.go:10-38`, hook `core/orchestrator.go:39`, set at `checkpoint/checkpoint.go:166-172` |
| A paused lane is resumable only for the reason `quota-likely`. | `core/lane_resume.go:11,69-71` |
| The pool puts each ready paused lane first, so the wave resumes them together. GC keeps their worktrees. | `cmd/evolve/cmd_loop_pool_paused.go:27-47`, `cmd_loop_pool.go:75`, `gc/worktrees.go:370` |
| The incident class `main-red` exists. The catalog action `revert-last-landing` needs an open main-red incident. | `selfheal/incident.go:14`, `selfheal/catalog.go:70,80` |
| The only main-red detector polls `gh run list` in `evolve selfheal`. | `cmd/evolve/cmd_selfheal.go:156-172`, `selfheal/ci.go:51-67` |
| The live incident path records an observation at once, with no poll. | `selfheal/live.go:38-55`, `cmd/evolve/cmd_selfheal_live.go:72-84` |
| The train lands a batch through `Main.FastForward` after a green batch CI. A red batch splits. | `integration/batchpr.go:31-34,65-109`, `integration/train.go:247-265` |
| The rank inputs have one loader. The order is a score sort. | `inboxrank/rankinputs/rankinputs.go:17-30`, `inboxrank/inputs.go:12-21`, `inboxrank/inboxrank.go:113-127` |
| Landing-gate stages live in `gates` of the policy. | `policy/routing_gates.go:63-80` |
| The git seam can add a detached worktree with retries. | `gitexec/worktree.go:35-41` |

### 1.3 PR #1260 (open, based on an older main)

PR #1260 adds `core/cover_changed_base.go`: `failedTests` (keys `pkg.Test`), `baseFailingTests`, `laneRedLines`, and `gitexec.Git.ExportTree`. It covers one of the six points. Its own open question names the defect: an exported tree has no `.git`. A gate that reads git, such as `testdup` until G2, is then red on the base for a false reason. G4 takes its parser and its partition rule, runs the base in a real detached worktree, and supersedes the PR.

### 1.4 The 2026-11-09 timeline in these terms

- 2240: ship gate red on base `7452aeec4`. `main` was `fe9550c22`, green. Today: FAIL. Wanted: rebase.
- 2242: build floor, repo-contract pack red on a red base, red `main`. The builder retried for 333 minutes. Wanted: not blamed, then park.
- 2249: ship gate red at 336 on a red base, red `main`. Wanted: park.

## 2. Requirements

| # | Requirement | Weight |
|---|---|---|
| R1 | One attribution function serves the build floor, the audit and ship. | Must |
| R2 | It reruns only the red test keys, on the lane base, in a tree with `.git`. | Must |
| R3 | A red on the base does not reject the lane at build or audit. | Must |
| R4 | At ship, base-red with a green `main` rebases through the existing rebase path. | Must |
| R5 | At ship, base-red with a red `main` parks the lane through the existing pause. | Must |
| R6 | Parked lanes resume together through the pool when the stop releases or `main` moves. | Must |
| R7 | The main-red state comes from the attribution runs and the landings, never from a poll. | Must |
| R8 | During a stop, a landing path lands only a fix: it turns one stop key green and adds no red. | Must |
| R9 | A fix-main item leads the rank while the stop holds. | Must |
| R10 | Past the policy bound (default 120 minutes) the stop escalates through the ADR-0155 incident path once, with the cause and the resume condition. | Must |
| R11 | An attribution that cannot run fails closed: the lane is blamed as today, with a WARN and a signal. | Must |
| R12 | Ship never lands a tree with a subtracted red. | Must |
| R13 | The three test-running build floor checks become `phase-floor` registry entries. | Wanted |
| R14 | The first part stops the 2242 waste. | Must |

## 3. Design (options)

| Option | Shape | Correct on R1-R12? | Simple | Blast radius | Score |
|---|---|---|---|---|---|
| A. Extend PR #1260 to each point | Six copies of the partition, each with its own base run | No: R1 fails, and the export tree misjudges git-reading gates | medium | wide | 1 |
| B. One package `baseattr` plus one record `mainstop`, with typed results at each point (chosen) | Each point maps its reds to keys and calls `Attribute`. Ship maps the result to the existing rebase error or a new park error. | Yes | high | each point changes about 20 lines | 5 |
| C. Read main-red from CI (`selfheal.MainGreen`) and blame by run conclusion | A `gh` call per rejection | No: R7 fails. CI red says nothing about these keys. | medium | low | 2 |
| D. Attribute only at ship, and let the build floor rerun on `main` | One point | No: R1 and R3 fail, and 2242 still loops at build (ADR-0165 R1) | high | low | 2 |

Correctness ranks B first. B is also the simplest correct one: the rebase, the pause and the incident paths exist, and B adds one judge and one record.

## 4. Decision

Build option B. `baseattr` is the one judge of "red on the base and on `main`". `mainstop` is the one record of the stop. Ship turns a verdict into the existing `GIT_FLEET_REBASE_NEEDED` error or a new park error.

Not in G4:

- No rebase at build or audit. The build worktree holds an uncommitted change, and `rebaseLane` rebases commits (`ship_recovery.go:384`). Build and audit subtract, and ship decides.
- No halt of new lanes during a stop (ADR-0165 rejects a halt).
- No subtraction by failure text. A key is a package and a top-level test.
- No train change in the first two parts. The train is at `shadow` (ADR-0147 decision 11), so each lane still lands itself.

## 5. Packages and dependency direction

| Package | Holds | Imports |
|---|---|---|
| `go/internal/baseattr` (new) | `Key`, `Reds`, `Verdict`, `ParseGoTest`, `Attributor`, the detached-tree runner | `gitexec`, `sysexec`, `ipcenv`, `lanebudget`, `addedtests`, stdlib |
| `go/internal/mainstop` (new) | the stop record, its pure transitions, the fix proof, the store | `baseattr`, `adapters/flock`, `paths`, stdlib |
| `go/internal/core` | the floor adapters, the park route, the build-floor stop opener | adds `baseattr`, `mainstop` |
| `go/internal/phases/ship` | the ship attribution and the stop check | adds `baseattr`, `mainstop` |
| `go/internal/phases/audit/ciparitygate` | the tier and acs-durable attribution | adds `baseattr` |
| `go/internal/checkpoint` | the reason `main-red` | unchanged imports |
| `go/internal/inboxrank`, `rankinputs` | the lead list | `rankinputs` adds `mainstop` |
| `go/internal/gateregistry` | three `phase-floor` entries | adds `core`, `phases/ship` |
| `cmd/evolve` | wiring, the pool readiness, the bound check, the fix item and the incident | adds `mainstop` |

- `baseattr` and `mainstop` import no pipeline package, so no cycle can start from them.
- `core`, `ship` and `ciparitygate` never import `gateregistry`. `gateregistry` imports them, as G1 allows.
- The fix item and the incident are written from `cmd/evolve`, which already imports `inboxfile` and `selfheal`. `core` gets them through one injected hook, as with `QuotaBoundaryCheckpointer`.
- No feature flag. The settings are policy data in `gates` (section 6.4).
- Section 13 adds `integration/stopgate.go` and two files in `selfheal` for the train. `integration` gets no new import.

## 6. Types and signatures

### 6.1 `baseattr`

```go
package baseattr

type Key struct {
	Package string
	Test    string
}

type Reds struct {
	Keys    []Key
	Unnamed bool
	Flags   []string
}

type Verdict struct {
	LaneOnly []Key
	BaseRed  []Key
	MainRed  []Key
	Base     string
	Main     string
	Took     time.Duration
}

type Runner func(ctx context.Context, moduleDir string, r Reds) ([]Key, error)

type Attributor struct {
	Git      gitexec.Git
	Run      Runner
	MainRef  string
	Scratch  string
	Timeout  time.Duration
	Serialize func(context.Context) func()
}

func ParseGoTest(out string) Reds
func KeysOf(names []string) Reds
func (r Reds) Attributable() bool
func (a Attributor) Attribute(ctx context.Context, base string, r Reds) (Verdict, error)
func (a Attributor) RunAt(ctx context.Context, rev string, r Reds) ([]Key, error)
func GoTestRunner(env []string) Runner
func (v Verdict) Blamed() bool
func (v Verdict) MainGreen() bool
```

- `ParseGoTest` is the one parser of `--- FAIL:` and `FAIL\t<pkg>` lines. It takes the rule of PR #1260 `failedTests`: a panic, a `FAIL` with no named test or a pending name sets `Unnamed`. `core.goTestFailureLines` and `ciparitygate.offenderLines` keep their display lines and take their keys from it.
- `KeysOf` maps the `pkg.Test` names of the ship pack to keys. The ship pack already has `packFailure{Package, Test}`.
- `Attributable` is false when `Unnamed` is set or `Keys` is empty. Then the caller blames the lane, as today.
- `MainRef` is `main`, the ref that `rebaseAll` targets. One ref defines both "main" and "the rebase target".
- `Scratch` is `.evolve/ephemeral/base-red`, under the GC sweep of the ephemeral tree.

### 6.2 `mainstop`

```go
package mainstop

type Parked struct {
	Cycle     int      `json:"cycle"`
	Scope     []string `json:"scope"`
	Workspace string   `json:"workspace"`
}

type Stop struct {
	Schema     string         `json:"schema"`
	Keys       []baseattr.Key `json:"keys"`
	MainSHA    string         `json:"main_sha"`
	OpenedAt   time.Time      `json:"opened_at"`
	FixItem    string         `json:"fix_item"`
	Parked     []Parked       `json:"parked"`
	Escalated  *time.Time     `json:"escalated_at,omitempty"`
	ReleasedAt *time.Time     `json:"released_at,omitempty"`
	ReleasedBy string         `json:"released_by,omitempty"`
}

type Proof struct {
	Base     string
	Green    []baseattr.Key
	LaneOnly []baseattr.Key
}

type Admission int

const (
	AdmitNoStop Admission = iota
	AdmitFix
	RefuseRebase
	RefuseNotAFix
)

func Path(evolveDir string) string
func Read(evolveDir string) (Stop, bool, error)
func Update(evolveDir string, fn func(Stop, bool) (Stop, error)) (Stop, error)
func (s Stop) Holds() bool
func (s Stop) Opened(v baseattr.Verdict, fixItem string, now time.Time) Stop
func (s Stop) WithParked(p Parked) Stop
func (s Stop) Admit(mainSHA string, p Proof) Admission
func (s Stop) Released(green []baseattr.Key, by string, now time.Time) Stop
func (s Stop) Overdue(now time.Time, bound time.Duration) bool
func (s Stop) Escalate(now time.Time) Stop
func (s Stop) Cause() string
func (s Stop) ResumeCondition() string
func FixItemID(mainSHA string) string
```

- Each transition is pure and returns a new value. `Update` holds a `flock` on the record and writes through a temporary file and `rename`.
- `Opened` merges the keys of an open stop. It never moves `OpenedAt`, so a refresh cannot reset the bound.
- `Admit` returns `RefuseRebase` when `p.Base` differs from `mainSHA`. A proof on a stale base says nothing about the landed tree.
- `Admit` returns `AdmitFix` when `p.Green` holds a stop key and `p.LaneOnly` is empty (ADR-0165 decision 13).
- `Released` drops the green keys. With no key left, it sets `ReleasedAt` and `ReleasedBy`.

### 6.3 The changes in existing packages

Amended by G2 7.8: the rerun methods of `ReviewInput` below move to a base-red ruler type there.

```go
package core

type FloorJudge func(ctx context.Context, in ReviewInput) ([]string, error)

type MainRedSink func(projectRoot string, v baseattr.Verdict, cycle int) error

var OpenMainRedStop MainRedSink

var MainRedBoundaryCheckpointer func(cs CycleState, projectRoot string, now time.Time) error

const laneMainRedReason = "main-red"

func FailOpen(name string, judge FloorJudge) BuildFloorCheckFn
func FloorEntryRun(judge FloorJudge) func(context.Context, gatecontract.Input, gatecontract.Ports) (gatecontract.Verdict, error)
func RepoContractFloorJudge(run RepoContractPackFn) FloorJudge
func UnitTestFloorJudge(ctx context.Context, in ReviewInput) ([]string, error)
func CoverChangedFloorJudge(ctx context.Context, in ReviewInput) ([]string, error)
func (in ReviewInput) attributor() baseattr.Attributor
func (in ReviewInput) laneReds(ctx context.Context, reds baseattr.Reds) (baseattr.Reds, error)
```

```go
package ship

const CodeMainRedPark shiperr.ShipErrorCode = "MAIN_RED_PARK"

func attributeContractRed(ctx context.Context, opts *Options, o packOutcome) error
func checkMainRedStop(ctx context.Context, opts *Options, stage string) error
func releaseMainRedStop(opts *Options, landed string, green []baseattr.Key) error
```

Section 13.3 amends this block: the release moves to `selfheal.StopRelease.Keys`, and ship calls it.

```go
package ciparitygate

func WithAttribution(a func(ctx context.Context, r baseattr.Reds) (baseattr.Verdict, error)) Option
```

```go
package checkpoint

const ReasonMainRed Reason = "main-red"
```

```go
package inboxrank

type Inputs struct {
	Config     policy.InboxPriorityConfig
	Now        time.Time
	Recurrence map[string]int
	Lead       []string
}
```

- `CodeMainRedPark` lives in `shiperr` beside `CodeGitFleetRebaseNeeded`, with class `ShipClassPrecondition`. The block above shows its role only.
- `ReasonMainRed` is `Reason(laneMainRedReason)`, so the word has one home in `core`.
- `laneQuotaPaused` becomes `laneParked`, true for the set `{quota-likely, main-red}`. `PausedScope` gains `Reason`. GC then keeps a parked worktree with no other change.

### 6.4 Policy

```go
package policy

type GatesPolicy struct {
	MainRedStop             string `json:"main_red_stop,omitempty"`
	MainRedStopBoundMinutes int    `json:"main_red_stop_bound_minutes,omitempty"`
	BaseRedRerunSeconds     int    `json:"base_red_rerun_seconds,omitempty"`
}

type GatesConfig struct {
	MainRedStop      string
	MainRedStopBound time.Duration
	BaseRedRerun     time.Duration
}
```

The block shows only the new fields. The compiled defaults in `GatesConfig()` are `shadow`, 120 minutes and 300 seconds. `MainRedStop` is a gate stage (ADR-0165 decision 15). It decides only the refusal of a non-fix landing. Attribution, rebase, park, release and escalation run at each stage, because they never land a red (R12).

## 7. Behavior

### 7.1 `Attribute(ctx, base, reds)`

1. If `reds` is not attributable, return `Verdict{LaneOnly: reds.Keys}`. Signal: none. The lane is blamed as today.
2. Take `Serialize` (the integration-tier lock, as the floor retake does).
3. `RunAt(base)`: add a detached worktree at `base` under `Scratch` through `AddWorktreeWithRetry`. Run `go test -count=1 -timeout <Timeout> <Flags> -run ^(names)$ <pkgs>` in its module. Remove it with `git worktree remove --force` in a `defer`.
4. Split the keys into `LaneOnly` and `BaseRed`. If `BaseRed` is empty, return.
5. Resolve `MainRef` through the git seam. If it equals `base`, set `MainRed = BaseRed` and return with no second run.
6. `RunAt(main)` on `BaseRed` only. Set `MainRed`.

| Failure | Cause | Evidence | Next step |
|---|---|---|---|
| The worktree add fails | a busy `.git` or no disk | the error with the git stderr | Return the error. The caller blames the lane and emits `BASE_RED_UNJUDGED`. |
| The base test run times out | a slow package | `Took` and the deadline in the error | Same as above. `base_red_rerun_seconds` is the knob. |
| `go test` cannot start | no toolchain | the start error | Same as above. PR #1260 ignored this case. |
| The `main` ref is unreadable | a broken ref | the git error | Return `BaseRed` with an empty `Main`. The caller treats `main` as red: it parks, and it never rebases on an unknown tip. |

Cost: one worktree add (a checkout of the tree, seconds), and one `go test` of the red packages only. The test compile dominates. The base tree has new paths, so expect a cold compile of those packages. The `repocontract` pack package is the worst case. `Took` goes into each signal, so the ADR-0165 rule "over 10% of the ship time of a wave" can be measured. A second run happens only when the base is red and differs from `main`.

### 7.2 Build floor

Each of the three test-running checks becomes a `FloorJudge`. `FailOpen` keeps today's WARN for a judge error. On a red, the judge builds `Reds` and calls `in.laneReds`:

1. Not attributable: reject as today.
2. `LaneOnly` not empty: reject with the lane-only keys. List the base-red keys as information (the rule of #1260).
3. All base-red: pass the check and write a NOTE with the keys and `Took`. Emit `BASE_RED_ATTRIBUTED` (kind `gate.corrected`, module `build`).
4. If `MainRed` is not empty, call `OpenMainRedStop`. The hook writes the stop, files the fix item and emits `MAIN_RED_STOP_OPENED`.

The floor does not park. The lane goes on to audit and parks at ship, with its work audited. This removes the 2242 loop: no `Retry` on a red that the builder cannot fix.

### 7.3 Audit

`IntegrationTier` and `ACSDurable` call the attribution after `decideTier` keeps a red (`cause` `retake_red`). All base-red: the gate returns a `gateWarning` and no offender, so `applyRepositoryGate` style warns. Lane-only: FAIL as today. `go vet`, apicover and graduation run no tests, so they get no attribution.

### 7.4 Ship

After a red pack, `attributeContractRed` maps `packFailure` to keys and decides:

| Verdict | Ship returns | Orchestrator does |
|---|---|---|
| not attributable, or `LaneOnly` not empty | `CodeRepoContractGate`, as today | the existing route |
| all base-red, `main` green | `CodeGitFleetRebaseNeeded` with the keys in `Debug` | the existing rebase, then the identity carry or a new audit; a conflict goes to the debugger |
| all base-red, `main` red | `CodeMainRedPark` with the keys | open or refresh the stop, then park |

After a green pack (or a rebase), `checkMainRedStop` runs before `runStages`:

| Stop | `Admit` | Stage `enforce` | Stage `shadow` |
|---|---|---|---|
| none | `AdmitNoStop` | land | land |
| holds | `RefuseRebase` | `CodeGitFleetRebaseNeeded` | land, emit `MAIN_RED_STOP_WOULD_REFUSE` |
| holds | `AdmitFix` | land, then `releaseMainRedStop` | the same |
| holds | `RefuseNotAFix` | `CodeMainRedPark` | land, emit `MAIN_RED_STOP_WOULD_REFUSE` |

The proof runs the stop keys on the lane worktree itself (`RunAt` with the worktree, no checkout). `LaneOnly` is empty there, because the pack passed or its reds were all base-red.

The park route in `recoverShipErrorRoute`: for `CodeMainRedPark`, call `MainRedBoundaryCheckpointer`, add `Parked` to the stop, emit `MAIN_RED_PARKED` and end the cycle with `ErrMainRedParked`. The failure digest records the class `main-red`, not the lane. A parked cycle is not a zero-ship cycle.

### 7.5 Resume and release

- A paused lane with the reason `main-red` is ready when the stop no longer holds, or when the `main` tip differs from `Stop.MainSHA`. A ref compare is cheap and needs no poll.
- The pool puts each ready parked lane first, so they run together in the next fill.
- A resumed lane re-enters ship. Its gate meets the new base: green `main` rebases, red `main` parks again and refreshes `MainSHA`.
- A landing that passes `AdmitFix` releases its green keys. An attribution run that finds each stop key green on the current tip also releases, with `ReleasedBy: "main <sha>"`.
- Release emits `MAIN_RED_STOP_RELEASED` and moves the fix item to `processed` if it is still open.

### 7.6 Fix item, signal and bound

- `FixItemID` is `fix-main-red-<first 8 of MainSHA>`. Fields: kind `pipeline-repair`, class `correctness`, weight 1.0, the key packages as `files`, acceptance "each key is green on main".
- `rankinputs.Load` reads the stop. While it holds, `Inputs.Lead` is `[FixItem]`, and `Order` puts lead ids first in the given order. This is data, not a score change.
- The bound check runs where the stop is already read: each pool fill, each ship stop check and each `OpenMainRedStop`. No new loop.
- `Overdue` and no `Escalated`: `Escalate`, then record a `main-red` observation through `selfheal.DetectLive` with `Cause()` and `ResumeCondition()`. Emit `MAIN_RED_STOP_ESCALATED` (kind `loop.escalation`). The ADR-0155 catalog then sees an open main-red incident, which `revert-last-landing` needs.
- `Cause()` names the keys, `MainSHA`, `OpenedAt` and the parked cycles. `ResumeCondition()` says: "a landing turns each key green on main, or a rerun finds them green on the main tip; the parked lanes then resume in the next pool fill".

The signal codes register in `signalcenter` under module `gate.repo`, except the two attribution codes, which register under the phase module.

## 8. Tests

The tests of the train are in section 13.5, with their own numbers.

Each test is red without the change. "Real entry" marks a test that runs the pipeline entry with a `gittest` repository.

| # | Test | Entry point | Red case |
|---|---|---|---|
| 1 | `TestParseGoTest_KeysEachFailByPackageAndMarksAPanicUnnamed` | `baseattr.ParseGoTest` | A panic counts as a named key. |
| 2 | `TestAttribute_RunsOnlyTheRedKeysInADetachedBaseCheckout` | `Attributor.Attribute`, `gittest.Fixture`, a fixture module with a test that reads `git ls-files` | An export with no `.git` makes the key base-red. |
| 3 | `TestAttribute_SkipsTheMainRunWhenMainIsTheBase` | `Attribute` with a counting `Runner` | Two runs. |
| 4 | `TestAttribute_AFailedCheckoutIsAnErrorAndRemovesTheWorktree` | `Attribute` with `gittest.FaultOn(tb, "worktree")` | A nil error, or a worktree left in `git worktree list`. |
| 5 | `TestRepoContractFloor_ABaseRedPackRedDoesNotRejectTheLane` | `core.BuildHandoffFloor.Review` with `RepoContractFloorChecks` and a fixture pack | `Approve: false, Retry: true` (the 2242 loop). |
| 6 | `TestRepoContractFloor_ALaneOnlyRedStillRejectsAndListsTheBaseRedKeys` | the same | The lane-only red passes. |
| 7 | `TestUnitAndCoverFloors_SubtractOnlyKeysRedOnTheBase` | `UnitTestFloorJudge`, `CoverChangedFloorJudge` | A base-red key rejects. (Acceptance of `cover-changed-subtracts-base-red`.) |
| 8 | `TestFloor_AnUnjudgedAttributionBlamesTheLaneAndSignals` | `Review` with a failing `Runner` | The lane passes. |
| 9 | `TestIntegrationTier_AllBaseRedOffendersWarnAndDoNotFail` | `ciparitygate.Gates.IntegrationTier` with `WithAttribution` | The audit FAILs. |
| 10 | `TestShipGate_BaseRedWithGreenMainAsksForTheFleetRebase` | `ship.Phase` native run, `gittest` repo | `CodeRepoContractGate`. |
| 11 | `TestShipGate_BaseRedWithRedMainParks` | the same | `CodeRepoContractGate`. |
| 12 | `TestRecoverShipError_MainRedParkWritesAMainRedCheckpointAndAStopRow` | `Orchestrator.recoverFromShipError`, real `checkpoint` hook | The cycle fails, and `PausedLaneScopes` lists nothing. |
| 13 | `TestStop_AdmitsOnlyAFixOnTheMainTip` | `mainstop.Stop.Admit`, table | A partial fix on a stale base is admitted. |
| 14 | `TestShipStop_AFixLandsAndReleasesTheStop` | `ship.Phase` native run with an open stop at `enforce` | The stop holds after the landing. (Acceptance 3.) |
| 15 | `TestShipStop_ANonFixParksAtEnforceAndLandsAtShadow` | the same | A non-fix lands at `enforce`. |
| 16 | `TestPool_ParkedLanesResumeTogetherWhenTheStopReleases` | `pausedPlan.order` with two parked scopes | One lane, or none, leads. |
| 17 | `TestRank_TheFixItemLeadsWhileTheStopHolds` | `evolve inbox rank --json` through `runInboxRank` | The fix item is not first. |
| 18 | `TestStop_PastTheBoundEscalatesOnceWithCauseAndResumeCondition` | the cmd bound check, a fake clock, `selfheal.LoadIncidents` | No incident, two incidents, or a summary without the resume condition. (Acceptance 4.) |
| 19 | `TestPolicy_MainRedStopDefaults` | `policy.Policy{}.GatesConfig()` | Not `shadow`, 120 minutes and 300 seconds. |
| 20 | `TestReplay_20261109` (real entry) | build floor `Review`, ship native run, `recoverFromShipError`, pool order and rank, on one `gittest` repository | Any step below. |
| 21 | `TestRegistry_FloorEntriesJudgeAFixtureAndReportAFailedReadAsAnError` | `gateregistry.Entries()`, the three `phase-floor` entries | A failed read gives a red, or a red gives a pass. |

Test 20 builds the timeline in one fixture module. The test of package `dup` fails when the file `count` is above `baseline`.

1. `C0` green. `R` (like `7452aeec4`) red. `F` (like `fe9550c22`) green, on top of `R`.
2. Lane 2240 on base `R`, `main` at `F`: ship returns the fleet rebase. The rebase moves the base to `F`, and the gate is green. Acceptance 1.
3. `main` moves to `M2` (like `9a2bbe0e1`), red. Lane 2242 on `M2`: the build floor approves. The stop opens, and the fix item leads the rank.
4. Lane 2249 on `M2`: ship parks it. Acceptance 2.
5. A fix lane on `M2` lowers `count`: it lands at `enforce`, and the stop releases. Acceptance 3.
6. The pool puts 2242 and 2249 first, together.

The fixture uses `gittest`, never a raw `git init`, so `rawgitratchet` stays green. No test kills a process. A test that needs the real `go` runs under the existing serial registry of `core`.

## 9. Protection (ADR-0157 manifest)

New `GateGuarantee` rows in `guards/integrity_surface.go`, in the commit that adds each file (decision 16):

- `/go/internal/baseattr/` and `/go/internal/mainstop/`;
- `/go/internal/core/repo_contract_floor.go` and `/go/internal/core/base_red_floor.go` (the new adapter file);
- `/go/internal/core/main_red_park.go` and `/go/internal/phases/ship/base_red.go`;
- `/go/internal/inboxrank/rankinputs/` (the lead list decides what runs first).
- `/go/internal/integration/stopgate.go`, `/go/internal/selfheal/main_red_hold.go` and `/go/internal/selfheal/main_red_release.go` (section 13).

Already protected: `build_floor_reviewer.go`, `cover_changed_floor.go`, `ciparitygate/`, `ship_recovery.go`, `ship.go`, and `phases/ship/repocontract.go` after G1.

Two inventory rows in `gate-input-inventory.md`:

| Gate | Input | Writer | Trust | Check |
|---|---|---|---|---|
| Ship main-red stop (`checkMainRedStop`) | `.evolve/main-red-stop.json` | harness (`mainstop.Update`) | authority | Every profile denies it. |
| Base-red attribution (`baseattr.Attribute`) | the lane base and the `main` tree | git | verified | The harness checks out the commit and reruns the keys itself. |

After G4, only the console changes these files, from a held proposal or an operator decision. A lane can still change an unprotected caller, and review checks that.

## 10. Build order and parts

G4 is too large for one pull request. Three parts, each green and useful alone.

**G4a: attribution at build and ship (stops the 333 minutes).**

1. `baseattr`: types, `ParseGoTest`, `KeysOf`, the runner, `Attribute`, its manifest row. Tests 1-4.
2. Policy `base_red_rerun_seconds`. Test 19 (part).
3. Build floor: the `FloorJudge` split with `FailOpen`, attribution in the three checks. Port the tests of #1260. Tests 5-8. Close PR #1260 as superseded.
4. Ship: `attributeContractRed` with the rebase and a typed main-red FAIL (`CodeMainRedPark`, routed as a plain failure until G4b). Tests 10-11, replay steps 1-3.
5. The three `phase-floor` registry entries. Test 21.

**G4b: the stop, the park and the bound.**

1. `mainstop` with its manifest row and inventory row. Test 13.
2. `ReasonMainRed`, `laneParked`, `PausedScope.Reason`, the park route. Test 12.
3. `OpenMainRedStop` and the fix item, wired in `cmd/evolve`. The rank lead. Test 17.
4. The ship stop check, the fix proof and the release. Policy `main_red_stop`. Tests 14-15.
5. Pool readiness. Test 16.
6. The bound and the escalation. Tests 18-19. Replay steps 4-6, so test 20 is complete.

**G4c: audit and the train.**

1. `WithAttribution` in `ciparitygate`. Test 9.
2. The train: during a stop at `enforce`, only a batch that turns a key green lands. It releases that key. Section 13 is the detail design.

Limits after G4:

- A key is a name. A lane that worsens a base-red test the same way is excused until ship rebases onto a green `main` and judges it with no subtraction.
- `go vet` and the apicover gates get no attribution, because they name no test.
- The stop refuses non-fix landings only at `enforce`.
- The three `phase-floor` entries are `repo-contract-floor`, `unit-test-floor` and `cover-changed-floor`. Each one runs its `FloorJudge` through `FloorEntryRun`.
- The other build floor checks stay outside the registry. G5 registers them when it builds one floor list.
- Three checks read `HEAD` or ancestry to witness a landing: `lost_landing_floor.go`, `explanationdocs/gitio.go` and `rebind.go`. G4 makes no entry for them. Question 1 of the G2 page asks the operator to confirm this.
- A red that depends on two changes still needs the train (plan limit).

## 11. Risks

| Risk | Level | Mitigation and test |
|---|---|---|
| A leaked detached worktree after a crash | Medium | The `defer` removes it. It lives under the ephemeral tree, and `git worktree prune` drops its row. Test 4. |
| A false base-red from a flaky test | Medium | The base run takes the tier lock. A base-red never lands: ship rebases or parks, and a resumed lane runs again. |
| Many parked lanes rerun the same keys on one new tip | Low | The first rerun updates `MainSHA` or releases the stop. The later lanes read the record first. |
| The base run costs more than 10% of the ship time of a wave | Medium | `Took` is in each signal. The ADR names this as the trigger to revisit. |
| The stop stays `shadow` and lanes pile onto a red `main` | Low | Lanes on a red base park anyway. Only lanes whose gates miss the keys land at `shadow`. |

## 12. Open questions for the operator

1. **Start the stop at `enforce`, against decision 15?** Recommended: no. Start at `shadow` for one wave, read `MAIN_RED_STOP_WOULD_REFUSE`, then move it.
2. **Does a parked cycle count toward "two cycles that ship nothing"?** Recommended: no. Count `main-red` parks separately, as the quota pause is.
3. **May the stop's escalation propose `revert-last-landing` on its own?** Recommended: keep the ADR-0155 stage. The proposal stays a proposal at `shadow`.

## 13. The train and the stop (detail of G4c step 2)

Section 10 names step 2 in one line. This section is the design that the build follows.

The rule of this section is ADR-0165 decision 13: while `main` is red, a landing path lands only a fix. `mainstop.Stop.Admit` is the one home of that rule. Ship calls it today. The train calls it too, with the same proof.

### 13.1 Current state

- `Merger.Land` reads the batch CI through the `CI` interface and moves `main` through the `Main` interface (`integration/batchpr.go`).
- A red batch splits, and each single record that stays red is ejected with a CI comment on its pull request.
- The train does not read the stop. On a red `main`, each record of a batch is red and gets the blame.
- Only ship releases the stop (`stopRelease.release` in `phases/ship/main_red_stop.go`). A fix that lands through the train leaves the stop open.
- `inboxmover` imports `integration`, so `integration` cannot import the mover of the fix item.
- A batch of documents only runs no Go job (`.github/workflows/required.yml`). Its green CI is no proof for a Go test.

### 13.2 Options

| Option | Change | Result |
|---|---|---|
| A. Hook fields in `Merger` | `Merger` gets two new fields, and `land` gets two new branches | Rejected. `land` is at its size limit, and each caller of `Merger` learns about the stop. |
| B. A check in `Train.Land` before the merger | One read of the stop before the batch | Rejected. The train cannot know before the CI run that a batch is the fix. |
| C. Decorators of `Main` and `CI` | A `StopGate` value wraps the two interfaces. `Merger` does not change. | Chosen. The merger stays closed to change. |

Option C has a cost. `Watch` runs more than one time for a red batch, so the value behind the gate must keep state for the shadow event. The two decorators share no call order in the type. The cost is accepted, because the state has one home (`StopHold`) and test 9 of 13.5 pins it.

### 13.3 Types

In `internal/integration` (new file `stopgate.go`):

```go
type StopGate struct {
	Red   func(pr PR) error
	Admit func(ctx context.Context, main, tip string) (landed func(), err error)
}

func (g StopGate) Main(inner Main) Main
func (g StopGate) CI(inner CI) CI
```

- The decorator of `CI` calls `Red` after a red `Watch`. An error of `Red` is the error of `Watch`.
- The decorator of `Main` calls `Admit` before the inner `FastForward`, with the head of `main` and the tip of the batch.
  - An error of `Admit` is the error of `FastForward`, and `main` does not move.
  - The decorator calls `landed` only when the inner `FastForward` returns nil.
- `landed` returns no error, because the landing is complete when it runs. The function behind it writes each failure as a warning (see 13.4).
- `integration` gets no new import and no new error value.

In `internal/selfheal`, next to `StopBound` (new files `main_red_hold.go` and `main_red_release.go`):

```go
var ErrMainRedHold = errors.New("main is red and a stop holds")

func ReadStop(evolveDir string, warn io.Writer) (stop mainstop.Stop, holds bool)

type KeyRun func(ctx context.Context, rev string, keys []baseattr.Key) (red []baseattr.Key, err error)

type StopHold struct {
	EvolveDir string
	Enforced  bool
	Event     signalcenter.Event
	Emit      func(signalcenter.Event)
	Warn      io.Writer
	Run       KeyRun
}

func (h *StopHold) Red(landing string) error
func (h *StopHold) Admit(ctx context.Context, main, tip string) (green []baseattr.Key, err error)

type StopRelease struct {
	EvolveDir string
	Now       time.Time
	Event     signalcenter.Event
	Emit      func(signalcenter.Event)
	Consume   func(fixItem string) error
}

func (r StopRelease) Keys(green []baseattr.Key, by string) (mainstop.Stop, error)
```

- `ReadStop` is the one home of the read with its fail-open rule. A record that cannot be read gives a warning on `warn` and the result "no stop that holds".
- Ship calls `ReadStop` in `checkMainRedStop`. Ship does not build a `StopHold`, and it keeps its own lane event with the admission of the lane.
- `StopHold` is the admission of a batch. Its two methods end in one private decision, which calls `mainstop.Stop.Admit` with a proof.
  - `Admit` reads the stop through `ReadStop`. With no stop that holds, it returns nil and runs no key.
  - `Admit` runs the keys of the stop on the tip through `Run`. The proof is `mainstop.Proof{Base: main, Green: <each key that is not red>}`.
  - A key run that fails gives a warning and a proof with no green key.
  - `Red` is the same decision with a proof that has no green key, because a red batch proves no key.
  - Each refusal of `mainstop.Stop.Admit` gives the same hold, so the base of that proof does not change the result.
- The decision has three results.
  - `AdmitFix`: `Admit` returns the green keys.
  - A refusal at `enforce`: an error that wraps `ErrMainRedHold`. The text names the landing, the keys and the remedy.
  - A refusal at `shadow`: nil, and one `MAIN_RED_STOP_WOULD_REFUSE` event for each `StopHold` value. The value remembers that it emitted.
- `StopRelease.Keys` is the one home of the release: the record update, the call of `Consume` and `MAIN_RED_STOP_RELEASED`. Ship calls it.
- The release has no stage. It updates the record at `shadow` and at `enforce`, as the release of ship does (section 6.4).
- `Event` is the base event of the caller: the cycle, the run, the phase and the origin. Ship gives the values that its tests assert today. The train gives the origin `integration batch`.
- `Now` is a value, as in `StopBound`. It is the start time of the command, so `ReleasedAt` can be earlier than the release by the CI time. No reader needs more.
- `Consume` is one call of the new function `inboxmover.ConsumeFixItem(opts, id, promote)`. This function holds the sequence that ship has today: locate the item, accept a missing item, move it to `processed`.

In `internal/core`, next to `BaseRedAttributor`:

- `StopProofReds(keys)` returns the `baseattr.Reds` of a stop proof: the keys and the tag flags.
- It is the one home of those flags. `stopCheck.proof` of ship and the default `KeyRun` of the train both call it.

In `cmd/evolve` (the composition root):

- `integrationEnv` gets three fields: `Emit` (the signal sink), `Now` (the clock) and `KeyRun` (the key run).
- `productionIntegrationEnv` sets each of the three.
- The default `KeyRun` is `core.BaseRedAttributor(root, run, warn).RunAt` with `core.StopProofReds(keys)`. It runs the keys on a detached tree of the tip.
- `train()` becomes `train(warn io.Writer)`. The two callers (`batchVerb` and `recordPR`) pass their stderr.
- The stop record and the policy file are in the project root, not in the working directory. Ship and the pool write and read them there.
  - The project root is `chooseRoot(flag, env.ProjectRootEnv, env.Root)`, the resolution of the `status` verb. The `batch` verb gets the `--project-root` parameter.
  - `env.Root` stays the directory of the git commands.
- `train` reads the stage from the policy file of the project root (`GatesConfig.MainRedStopEnforced`), as ship does.
- The wiring test reads the inner ports before the wrap. `integrationEnv.ports()` returns the bare `GhCI` and `GitMain`, and `train` wraps them.
  - `TestIntegrationProductionEnvSetsEveryPort` keeps its assertion of `Gh` and `Settle` on the result of `ports()`.
- `train` builds one `StopHold` and one `StopRelease`, and wraps `CI` and `Main` with one `StopGate`.
  - The `Admit` closure calls `StopHold.Admit`. Its `landed` function calls `StopRelease.Keys` with the green keys and `"batch main <tip>"`.
- `refusalExit` gets a second arm, after the arm of `*integration.Refusal`: `errors.Is(err, selfheal.ErrMainRedHold)` gives the refused code.
  - The two arms give the same code, so the new arm cannot change the result of the first one.

### 13.4 Behavior

| Batch CI | Stop | Keys on the batch tip | Stage | Result |
|---|---|---|---|---|
| green | none | not run | each | The batch lands, as today. |
| green | holds | one or more green | each | The batch lands. Then each green key is released. |
| green | holds | each one red | `enforce` | `FastForward` returns the hold error. `main` does not move. Each record stays for the next batch. |
| green | holds | each one red | `shadow` | The batch lands. One `MAIN_RED_STOP_WOULD_REFUSE` event for the run of the command. |
| red | none | not run | each | The batch splits, as today. |
| red | holds | not run | `enforce` | `Watch` returns the hold error. No rerun, no split, no ejection, no comment. Each record stays for the next batch. |
| red | holds | not run | `shadow` | The batch splits, as today. The same one event for the run of the command. |

- A green batch CI is not the proof of a fix. The proof is the run of the keys on the tip of the batch, before `main` moves.
- The train and ship thus have one admission rule. The green batch CI is the "no gate red" half of decision 13.
- A batch of documents only does not land during a stop at `enforce`. The keys stay red on its tip.
- A `main` that moved gives no fast-forward, and thus no key run and no release.
- The hold error of a red batch comes from the first red `Watch`, before a rerun.
- A release that fails does not fail the landing. The closure writes `[integration] WARN main-red stop release: <error>` to stderr, as ship does. The stop then holds until a later release.
- A key run that fails proves no fix. Stderr has the warning, and the batch gets the refusal of its stage.
- A stop record that cannot be read does not hold the batch. The batch lands or splits as today, and stderr has the warning.
- The hold error names the keys and the remedy. The remedy is one batch that has each fix and no other change, or ship.
- `evolve integration batch` exits with the refused code for `ErrMainRedHold`, not with the I/O code.
- The decorator of `CI` passes `Open` and `Rerun` to the inner value with no change. The decorator of `Main` passes `Head`.

### 13.5 Tests

1. `Merger.Land`, fake CI green, `StopGate.Main`: `Admit` runs before the inner `FastForward`.
   - `Admit` gets the head and the tip.
   - `landed` runs one time, after the fast-forward.
2. `Merger.Land`, the fast-forward fails: zero `landed` calls.
   - `main` moved: zero `Admit` calls.
3. `Merger.Land`, fake CI green, `Admit` returns the hold error: the error is `ErrMainRedHold`.
   - The inner `FastForward` has zero calls.
4. `Merger.Land`, fake CI red, `Red` returns the hold error: the error is `ErrMainRedHold`.
   - The run has zero `Eject` calls and zero `Rerun` calls, and the outcome has no ejected record.
5. `Merger.Land`, fake CI red, `Red` returns nil: the batch splits as before.
6. `selfheal.StopHold`, one table for `Admit` and `Red`.
   - No stop: nil, and the key run does not start.
   - One green key: the green keys, at each stage.
   - Each key red, or a key run that fails: the hold error at `enforce`, and nil with one event at `shadow`.
   - Two refusals of one value at `shadow`: one event.
7. Ship: the release tests of G4b pass through `StopRelease.Keys` with no change of their assertions.
8. `evolve integration batch` through the real command, stage `enforce`, a stop on disk, red CI: the refused exit code.
   - The record of the pull request is kept, and the stop holds.
   - Stderr names the keys of the stop and the remedy.
9. The same command at stage `shadow` with two red records: the split result of today.
   - The run emits exactly one `MAIN_RED_STOP_WOULD_REFUSE` event.
10. The same command with green CI and keys that are green on the tip.
    - `main` moves, and the stop record is released.
    - The fix item is in `processed`.
11. The same command at `enforce` with green CI, and keys that stay red on the tip.
    - The batch has documents only.
    - The exit code is the refused code, and `main` does not move.
    - The stop holds, and the fix item stays in the inbox.
12. The same command and batch at `shadow`: `main` moves, the stop holds, and the run emits one event.
13. The same command at `enforce` with a stop record that cannot be parsed, and red CI: the batch splits.
    - Stderr has the warning of the read.
14. The same command with a fix batch and a release that fails: the batch lands.
    - The exit code is that of a landing.
    - Stderr has the release warning, and the stop holds.
15. The decorators: `Open`, `Rerun` and `Head` reach the inner value with the same arguments and results.
16. `inboxmover.ConsumeFixItem`: an open item moves to `processed`, and a missing item is not an error.
17. Wiring: `TestIntegrationProductionEnvSetsEveryPort` also requires `Emit`, `Now` and `KeyRun`.
18. The default `KeyRun` on a `gittest` repository with an injected `baseattr.Runner`.
    - The runner gets the tree of the given revision and the flags of `core.StopProofReds`.
19. `core.StopProofReds`: the ship proof test and test 18 fail when the flags of one caller change.
20. Guard: a production `integration.Train` value is built only in `integrationEnv.train`.
    - The `Main` and the `CI` of that value are the decorators of `StopGate`.
    - The test puts a hold error behind the gate and gets it from `FastForward` and from a red `Watch`.
21. The real command from a linked worktree. The stop and stage `enforce` are in the project root.
    - With `--project-root`, and with the environment value, the exit code is the refused code.
    - Red case: with the working directory as the root, the batch lands.

The numbers of this list are local to section 13. Section 8 has the tests of the other steps.

Each test is proven red without its production line. The command tests give a fake `KeyRun` through `integrationEnv`.

### 13.6 Limits

- The key run costs one rerun budget at most before each landing during a stop. This is an estimate from the policy value, not a measurement.
- A batch that holds the fix and a red change is held at `enforce`. The remedy is a batch of the fix alone.
- A stop can hold the keys of two causes. A pull request that fixes one cause stays red in the batch CI, and the train holds it.
  - Ship admits such a partial fix, because ship subtracts the base reds. The remedy is one batch with each fix, or ship.
- A `main` that moves during the key run fails the push. The next run of the command composes the batch again.
- With no `--project-root` and no environment value, the project root is the working directory. A command from a linked worktree then reads no stop.
- The hold does not close the pull request of the batch on the host. This is the same as each other error path of `Watch`.
