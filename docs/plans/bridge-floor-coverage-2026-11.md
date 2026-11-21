# Plan: the build floor measures the coverage of a bridge change during a live wave

- **Status:** built, 2026-11-20 (3 of 3 components on main). Earlier status: built, 2026-11-18. Revision 3, after three rounds of architecture review. The HIGH of round 3 is fixed in D3 and D12.
- **Inbox item:** `bridge-changes-skip-the-coverage-floor-during-a-wave`.
- **Related:** the floor coverage check is `go/internal/core/cover_changed_floor.go`. The tier table is `go/internal/ciparity/envexclusive.go`. The hermetic test rule is [hermetic-driver-tests.md](../conventions/hermetic-driver-tests.md).

## 1. Request

The inbox item says: "At the floor under a live loop, run the internal/bridge tests that need no real tmux (skip only the requireTmux tier), write a profile, and run cover changed on it. A function that only a requireTmux test covers is reported as not measured at the floor, not as a pass."

The operator asked on 2026-11-17 to build the Todo inbox items concurrently with the wave. This item is protected, so only the console can build it.

## 2. Facts

**F2.1** Cycle 2218 added an error return to `recipeSessionDriver.EnsureSession` with no test. The cycle shipped, and main CI turned red at `08b0d2d76` (EnsureSession 96.8%). PR #1371 (`a421cccc2`) added the test.
*Implication:* CI was the first coverage check of a bridge change. The floor let a partly covered function through.

**F2.2** `ciparity.TierScope` drops each env-exclusive package from the floor package list (`go/internal/ciparity/tierscope.go:26-28`). The only entry is `internal/bridge` (`go/internal/ciparity/envexclusive.go:13-17`).
*Implication:* the floor never runs a bridge test. The drop does not depend on a live loop, although the floor note says "under a live loop" (`go/internal/core/build_floor_retake.go:49`).

**F2.3** `coverChangedPackages` builds its list from `floorTestPackages` (`go/internal/core/cover_changed_floor.go:160-172`). `covchanged` judges only a function inside that package set (`go/internal/covchanged/measure.go:64-65`).
*Implication:* a changed bridge function is "outside the package set". The floor makes no line for it, so the skip is silent.

**F2.4** All 15 test files of `internal/bridge` that call `requireTmux(t)` have the build tag `integration` (11 files) or `integration && darwin` (4 files). Command: `head -1` of each file, 2026-11-17.
*Implication:* the tag `integration` selects the tmux tests and the bridge tests that need no tmux. The current tags cannot separate them.

**F2.5** Under a live wave, real tmux boots time out: the env-exclusive note records 13 timeouts (exit 80) on cycle 1543 (`go/internal/ciparity/envexclusive.go:15`).
*Implication:* the tmux tier must stay out of the floor during a wave.

**F2.6** Three spellings reach the tmux binary in bridge tests: `requireTmux(t)`, `testhost.Tool(t, "tmux")` (`go/internal/bridge/paneloss_integration_test.go:19`) and `exec.LookPath("tmux")` (`go/internal/bridge/sandbox_contract_private_tmux_integration_test.go:50`). Source: the architecture review of revision 1.
*Implication:* a marker read from one spelling misses tmux tests. The tier needs one marker and one enforcer.

**F2.7** The package `TestMain` calls `tmuxtest.Main` (`go/internal/bridge/tmuxtest_main_test.go:28`). On a host with tmux, it starts a real tmux server before any test runs (`go/internal/tmuxtest/tmuxtest.go:71-77`).
*Implication:* a skip of test names does not stop the server start. The server start must also belong to the tier.

**F2.8** Practice: a coverage gate fails closed on unknown coverage. A skipped tier is visible, not silent ([qaskills: uncovered changed lines](https://qaskills.sh/blog/changed-line-coverage-diff-hunks-gate), [covgate](https://docs.rs/crate/covgate/0.2.0), [hoop.dev: integration tests](https://hoop.dev/blog/your-integration-tests-are-leaking-time/)). A build constraint removes a file from the build, so the compiler enforces it ([go build constraints](https://pkg.go.dev/cmd/go#hdr-Build_constraints)).
*Implication:* the floor must run what it can, judge what it measured, and name what it did not measure.

**F2.9** The tag `integration` is a literal in at least eight homes. They are `covchanged.DefaultTags` (`go/internal/covchanged/workspace.go:16`) and three Makefile targets (`go/Makefile:125`, `:135`, `:259`). The others are the apicover wiring (`go/cmd/evolve/cmd_composition_wiring.go:59`) and the darwin set (`go/internal/ciparity/darwin_set.go:31`). The audit parity gate (`go/internal/phases/audit/ciparitygate/tier.go:28`) and the overlap lister (`go/internal/overlap/golist.go:16`) are the last two, as round 2 of the review found.
*Implication:* a design that adds a tag to the CI set must change each home. A design that adds an exclusion tag at the floor only changes none of them.

**F2.10** The shared integration helpers (`requireNativeSandbox`, `itConfig`, `itLaunch`, `itBoundedContext`, `itSession` and others) live in the same file as `requireTmux` (`go/internal/bridge/tmux_repl_integration_test.go:24-135`). Seven test files that do not reach tmux call them.
*Implication:* the tier boundary must not cut through this file. The helpers with no tmux reach move to their own file first.

**F2.11** Five untagged unit test files run the production controller `execTmux{}`, which runs the real `tmux` binary (`go/internal/bridge/tmux.go:114-116`). Examples: `tmux_capture_exec_test.go:13-14` and `realtmux_deps_test.go:15`. They pass today because `TestMain` starts the shared server.
*Implication:* these files are part of the tmux tier. When the floor starts no server, they must leave the floor build too.

**F2.12** A guard test already parses each `_test.go` file of the package, reads its build constraint, and matches the tmux server starters (`go/internal/bridge/tmux_own_server_guard_test.go:15-19`).
*Implication:* the tier guard extends this walker and shares its marker table. It is not a second parser.

## 3. Goals and non-goals

Goals:
1. At the floor, a changed bridge function with a new uncovered branch is refused at stage `enforce`.
2. No tmux test and no tmux server start runs at the floor.
3. The floor names each changed bridge function that it cannot measure. It does not skip in silence.

Non-goals: the audit tier does not change. CI still runs the tmux tier with tmux on ubuntu and macOS.

## 4. Candidate designs

| Design | How the floor leaves out the tmux tier | Goal 1 | Goal 2 | Goal 3 | Cost |
|---|---|---|---|---|---|
| A. A skip list parsed from the code | `-skip` with the `Test*` names that reach `requireTmux` | yes | no: F2.6, F2.7 and F2.11 escape the list | partly: a name match is not a call | a parser, a single-package run seam |
| B. Unit tier only | no `-tags integration` | no: an integration test can cover any function | no: F2.7, F2.11 | yes | small |
| C. A skip list kept by hand | `-skip` with a list in a data file | yes | no: a new tmux test runs until a person adds it | yes | a list that drifts |
| D. The full tier under the host lock | `-tags integration`, serial | yes | no: F2.5 | yes | wave stalls |
| E. An inclusion tag for the tier | the tier files need `tmuxtier`; CI adds it | yes | yes | yes | each of the eight homes of F2.9 changes |
| F. An exclusion tag for the tier | the tier files carry `!notmux`; only the floor adds `notmux` | yes | yes: the compiler leaves the files out, and a guard holds the tag | yes | one tag at the floor, one guard |

Design F wins. The compiler enforces the tier, so a new tmux test cannot run at the floor by accident (rejects A and C). A build tag works per file, so it does not leak to other packages, as a `-skip` regex does in a multi-package `go test` call. The tier files build in each place that does not set `notmux`, so CI and the eight homes of F2.9 do not change (rejects E).

## 5. Decisions

| Id | Decision | Because |
|---|---|---|
| D1 | The integration helpers with no tmux reach move from `tmux_repl_integration_test.go` to a new file with the tag `integration` only. | F2.10 |
| D2 | Each test file of `internal/bridge` that reaches tmux adds the constraint `!notmux` to its tags. This includes the untagged unit files of F2.11. | F2.4, F2.6, F2.11 |
| D3 | `tmuxtest.Main` splits into an isolate step and a serve step. The isolate step selects the private socket of the test process and closes the inherited files. The serve step starts the server and the orphan guard. Both builds of `TestMain` run the isolate step. Only the `!notmux` build runs the serve step. | F2.7; `go/internal/tmuxtest/tmuxtest.go:61-70` |
| D4 | The guard of F2.12 gets a closed marker table: `requireTmux`, `testhost.Tool` with `"tmux"`, `exec.LookPath` with `"tmux"`, `execTmux{}`, `itTmuxCtl`, `tmuxtest.StartServer`, `startOwnTmuxServer`, `ownTmuxServer` and `startContractServer`. A file that uses a marker and lacks `!notmux` fails the guard. Each marker has a fixture in the self-test of the guard. | F2.6, F2.11, F2.12 |
| D5 | Two uses are not markers. A call of `exec.LookPath("tmux")` that only skips the test is one. An `execTmux{}` literal that is only an interface conversion or a type assertion is the other. Each has a fixture. | `go/internal/bridge/driver_ollamatmux_test.go:86`, `go/internal/bridge/joined_capturer_test.go:7-9` |
| D6 | The env-exclusive entry of `internal/bridge` declares its exclusion tag `notmux`. The guard reads the tag from that entry. | F2.12 |
| D7 | The floor computes the tag set of each package: `covchanged.DefaultTags` plus the exclusion tag that the package declares, if any. A package that declares none keeps `DefaultTags`. The cover run puts each tiered package in its own `go test` call with its tag set. The headline of a red run names the tag set of the failed package. Other packages that call `tmuxtest.Main` keep their server at the floor. | F2.9 |
| D8 | The floor cover run keeps `internal/bridge`. The unit lane, the added-tagged lane and the plumbing fallback of the cover run keep the current drop. | F2.3; `go/internal/core/cover_changed_floor.go:47-48` |
| D9 | A changed function of a tiered package below 100% is refused at stage `enforce`, with one exception (D10). | goal 1, F2.1, F2.8 |
| D10 | A function at zero statements that exists unchanged in name at the floor base is "not measured at the floor". Its earlier coverage came from the tier. The floor lists it as a WARN line with the tag, and records a residual. The floor cannot prove that the tier alone covers the function. The WARN says only this: no test in the floor build covers it, and the tier did not run. | goal 3 |
| D11 | The floor note names the exclusion tag for the cover run. The note of the other lanes says that the cover run measures the package. | F2.2 |
| D12 | After `m.Run`, the `notmux` build of `TestMain` checks for the socket file of its private socket. If a stray reach started a server, it stops that own server and exits non-zero with "tmux reached at the floor". A test injects the check. D12 sees only the private socket of the test process; a reach that names its own socket depends on the marker table (D4). | review round 3, H1 |

**Consequences:** a function that is partly hermetic and partly tmux-only is refused. The builder then adds a hermetic test, as [hermetic-driver-tests.md](../conventions/hermetic-driver-tests.md) asks. F2.1 is that case: `EnsureSession` was at 96.8%, so D9 refuses it. A new function at zero statements is refused too. Only an old function that the tier alone covers gets a WARN. CI stays its check.

**What changes our mind:** the non-tmux bridge tests can time out under a live wave. Then the cover run drops the package again, and lists each changed bridge function as "not measured".

## 6. Components

| Id | Component | Red test first |
|---|---|---|
| K1 | D1 to D6 and D12: the helper move, the `!notmux` constraints, the split `TestMain`, the guard, and the standing check | The guard is red on today's tree. It names the files of F2.6 and F2.11. `go test -c -tags integration,notmux ./internal/bridge/` compiles on darwin and linux. When the compile fails, a hermetic caller of a tier helper (for example `itDeps`) moves or gets the tag. A test of D12 injects a stray server and sees the non-zero exit. |
| K2 | D7: the floor tag set | `DefaultTags` plus the declared exclusion tags is `["integration", "notmux"]`, and each use in `cover_changed_floor.go` reads it. |
| K3 | D8 to D11: the floor cover run and the split of its result | A simulated floor on a bridge change refuses a partly covered function and a new function at zero statements at `enforce`. An old function at zero statements gives a WARN line and a residual. The plumbing fallback does not run `internal/bridge`. |

The console builds K1 to K3 as one change, because the protected check routes `go/internal/core/cover_changed_floor.go` to the console.

## 7. Order

1. K1, because the floor can keep the package only when the tier is out of its build.
2. K2, because K3 reads the floor tag set.
3. K3.

## 8. Verification

- Before: a changed bridge function makes no floor line (F2.3).
- After: the red tests of K1 to K3 pass. Each changed function has 100% coverage.
- `go test -tags integration,notmux ./internal/bridge/` starts no tmux process. Check with `pgrep -P` of the test binary during the run.
- CI: the ubuntu and macOS jobs run the same bridge test count as before the change. The ubuntu go job gets one step, `make test-notmux`. It follows the `cover-changed` step, because that step must follow `make test-integration` at once. The step runs the `notmux` build of `internal/tmuxtest` and `internal/bridge`, which no other CI step builds.
- Live: in the first wave after the merge, check the floor log of a bridge change. It shows the tier note, and no tmux boot.

## 9. Risks

| Risk | Control |
|---|---|
| A tmux reach that is not in the marker table runs at the floor, for example through the default `Deps.Tmux` (`go/internal/bridge/engine.go:161-163`) | The isolate step (D3) sends the reach to the private socket, not to the live server. The standing check (D12) fails the floor run and names the reach. |
| The non-tmux bridge tests are slow under load | The cover run holds the verify lock (`go/internal/core/build_floor_reviewer.go:328`). "What changes our mind" names the fallback. |
| A function is renamed and then reads as new at zero statements | D10 refuses it. The builder adds a hermetic test, which the convention asks for. |

## 10. Open questions

None.

## Delivery status (2026-11-21)

| Component | Status | Change lists | Open item |
|---|---|---|---|
| K1 helper move, `!notmux` constraints, split `TestMain`, guard | done | [#1583](https://github.com/mickeyyaya/evolve-loop/pull/1583), [`296d481`](https://github.com/mickeyyaya/evolve-loop/commit/296d48100); `go/internal/bridge/tmux_own_server_guard_test.go`, `go/Makefile:114` | none (consumed) |
| K2 floor tag set | done | [#1583](https://github.com/mickeyyaya/evolve-loop/pull/1583), [`296d481`](https://github.com/mickeyyaya/evolve-loop/commit/296d48100); `go/internal/ciparity/envexclusive.go:19`, `go/internal/covchanged/declared.go` | none |
| K3 floor cover run and split result | done | [#1583](https://github.com/mickeyyaya/evolve-loop/pull/1583), [`296d481`](https://github.com/mickeyyaya/evolve-loop/commit/296d48100); `go/internal/core/cover_changed_floor.go`, `cover_changed_tier.go`, `cover_tier_runs.go` | none |

The plan document came in with [#1555](https://github.com/mickeyyaya/evolve-loop/pull/1555), [`da195e3`](https://github.com/mickeyyaya/evolve-loop/commit/da195e347). The three components landed together in one pull request, as the plan asks. The inbox item `bridge-changes-skip-the-coverage-floor-during-a-wave` was consumed by [#1619](https://github.com/mickeyyaya/evolve-loop/pull/1619), [`98c7baf`](https://github.com/mickeyyaya/evolve-loop/commit/98c7bcafcb011aab561ffa1166b15ac07da58a9c). Nothing is left. The verification rows were not run again for this review, so they are unverified here.
