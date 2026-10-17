# Plan: CI wall time falls from about 24 to about 9 minutes, and every run keeps the data that shows why

- **Status:** accepted, 2026-10-17 (the operator approved it with "Ok").
- **Research:** [ci-wall-time-2026-10](../research/ci-wall-time-2026-10.md). **Earlier campaign:** [test-system-efficiency-2026-10](test-system-efficiency-2026-10.md). **Decision:** [ADR-0135](../architecture/adr/0135-ci-measures-itself-before-it-changes-its-shape.md). **Design:** [ci-telemetry-and-critical-path](../architecture/ci-telemetry-and-critical-path.md).

## Table of contents

- [1. Request](#1-request)
- [2. Facts](#2-facts)
- [3. Goals and non-goals](#3-goals-and-non-goals)
- [4. Candidate designs](#4-candidate-designs)
- [4a. The telemetry index catalog](#4a-the-telemetry-index-catalog)
- [5. Decisions](#5-decisions)
- [6. Components](#6-components)
- [6a. Components after the first instrumented run (C6 to C14)](#6a-components-after-the-first-instrumented-run-c6-to-c14)
- [7. Order](#7-order)
- [8. Verification](#8-verification)
- [9. Risks](#9-risks)
- [10. Open questions](#10-open-questions)

## 1. Request

On 2026-10-17 the operator asked three things:
- "Ultrathink to improve the efficiency of ci time, it takes way too long to complete, deep dive and research to investigate".
- "Think from the smallest unit to improve the telemetry and how we measure the time and find the critical path and identify the bottleneck".
- "Please ensure that everything is documented in detail, propose at least three different approaches, and identify the winning solution while outlining the advantages of the alternatives."

Later the same day the operator added: "Scale the telemetry to include not just time spent but also the memory and I/o and etc..., logging the indexes that matter to the morden software."

## 2. Facts

| Fact | Research id |
|---|---|
| The workflow takes 21–25 min. The macOS job sets that time; the ubuntu jobs take 9–13 min. | F3.1 |
| The test step is most of each Go job: 17.5 min on macOS and 10 min on ubuntu. | F3.2 |
| All three Go jobs run the same full suite and all the coverage gates. | F3.3 |
| Single packages set the step floor: about 200 s on ubuntu and 350 s on macOS. | F3.4 |
| The poles have three shapes: a serial chain (`cmd/evolve`, 228 s), one heavy parallel cluster (`phases/audit`, about 750 s locally), and real waits (`bridge`). | F3.5–F3.8 |
| Race and coverage add about 20% on the two packages measured. | F3.9 |
| 16 of 303 packages hold darwin-specific code. | F3.10 |
| The CI writes per-test timings and then throws them away. | F3.11 |
| The two packages measured, `phases/audit` and `phases/ship`, spend 2.5–3.3 times more CPU in the kernel than in user code (local, 14 CPUs). | F3.13 |
| Package accounting cannot see contention between packages; a runner sampler can. | F3.15 |

## 3. Goals and non-goals

**Goals**

- **G1:** the `required CI` wall time is about 9 min after designs A and C. Today it is 21–25 min. 8 min or less needs design B as well.
- **G2:** every run keeps its telemetry: per-test time, per-package resources and the runner's load. A report names the critical path from L5 (workflow) down to L1 (test), and the resource that limits it.
- **G3:** no loss of evidence. Each check that runs today still runs on the system where its result can differ.
- **G4:** the runner-minute cost goes down, not up.

**Non-goals**

- A different CI provider, or self-hosted runners.
- Fewer tests. A test is removed only when it duplicates another test, and that is a separate review.
- The loop's own build floor. Its coverage defect is inbox `cover-changed-floor-never-measures-acs-package-setup-fails`.

## 4. Candidate designs

Five designs reach for G1 in different ways. The time estimates are synthesis from F3.1–F3.10. Each one gives its arithmetic, or it says "assumption".

### A. Scope the matrix: each job proves only what it alone can prove

Ubuntu 1.23 keeps the full suite and owns every coverage gate. macOS runs only the darwin-sensitive packages (F3.10) and the e2e tier. Go 1.27 runs vet, build and the full suite without coverage gates.

- **Estimate:** the macOS test step becomes the sum of its darwin packages over 3 CPUs. `cmd/evolve` (348 s) sets the floor, so the step takes about 6 min. With e2e (about 2.9 min, F3.2) and setup (up to 2.5 min), the job takes about 11 min. The workflow wall becomes the slowest ubuntu job: 10.5–12.5 min for 1.23, and up to 13.3 min for 1.27 (F3.1).
- **Advantages:** one workflow change, low risk, and it cuts runner cost the most, because macOS minutes cost about 10× Linux minutes (P4.3).

### B. Shard the ubuntu suite by measured timing

`gotestsum tool ci-matrix`, or our own `testlatency` history, splits the packages into N balanced jobs (P4.1, P4.2). A fan-in job merges the coverage profiles and runs the gates once.

- **Estimate:** with N = 3, each shard's floor is the longest package, `phases/audit` at 198 s. Each shard job takes about 5.5 min: 198 s, plus about 2.5 min of setup (F3.2). The fan-in adds about 1.5 min (assumption), so the ubuntu path takes about 7 min. This helps only after design A, because macOS stays at 24 min.
- **Advantages:** it scales with the suite, and it needs no change to any test.

### C. Shrink the long poles at the unit level

Fix each pole by its shape (F3.5):
- `phases/audit` shares one build of the worktree generator per distinct tree, or checks through a small binary (F3.6).
- `cmd/evolve` moves its serial tests to injected environments, or runs them in separate processes (F3.7).
- `bridge` gets an injected clock for its dwell tests (F3.8).

- **Estimate (assumption):** one shared build leaves one compile and nine cheap runs, so `phases/audit` falls from 198 s to about 40 s on ubuntu. With the other two fixes, the ubuntu test step falls from about 600 s to about 400 s, which is a 3.3-min cut.
- **Advantages:** it is the only design that also speeds up local runs and the loop's build floor, because the floor runs these same tests. It also raises the floor that A and B leave behind.

### D. Reuse test results whose inputs did not change

A content key per package covers its Go files, its dependency graph, the Go version, the build flags and its non-Go inputs. A PR runs only the packages whose key changed (P4.5). ADR-0134's `internal/testkey` and `internal/testverdict` are the base. A scheduled full run is the backstop.

- **Estimate (assumption):** most PRs touch a few packages, so the test step takes 1–3 min. No measurement of the PR mix exists yet.
- **Advantages:** this is the largest win per PR, and it grows with the suite.
- **Limit:** many tests read `.evolve/profiles` through `../`, which the key must cover. The design must also fall back to a full run when its dependency data is incomplete.

### E. Pay for larger runners

Use `macos-latest-xlarge` and an 8-core ubuntu runner (P4.3).

- **Estimate:** the macOS step falls by about half. The single-package floors (F3.4) stay.
- **Advantages:** no code change, and the effect is immediate.

### Scored comparison

Scores are 1 (worst) to 5 (best). The wall-time score is for the design alone, applied to today's tree.

| Criterion (weight) | A. Scope matrix | B. Shard | C. Unit poles | D. Reuse results | E. Larger runners |
|---|:-:|:-:|:-:|:-:|:-:|
| Wall time saved (×3) | 5 (24 → about 12.5 min) | 1 (macOS stays the path unless A lands first) | 3 (macOS step falls about 2–3 min, assumption) | 4 (PRs only) | 3 |
| Keeps all evidence, G3 (×3) | 4 | 5 | 5 | 3 | 5 |
| Effort and risk (×2) | 5 | 3 | 2 | 1 | 5 |
| Runner cost, G4 (×2) | 5 | 2 | 4 | 5 | 1 |
| Helps local runs and the loop (×1) | 1 | 1 | 5 | 2 | 1 |
| **Weighted total (out of 55)** | **48** | **29** | **41** | **35** | **37** |

C ranks second at 41, and B ranks last at 29.

### The winner: telemetry first, then A, then C, then B or D only if needed

**Design A wins**, with telemetry as step 0. A alone moves the critical path off macOS and halves the wall time, at the lowest risk and cost. It does not depend on any other design. It also exposes the next floor, the ubuntu jobs at about 12.5–13.3 min, so the next steps have one target. A also leaves the preconditions of every other design intact.

**Each alternative keeps a role:**
- **C** is the second step. It is the only design that also speeds up local runs and the loop's build floor. It also lowers the floor that A and B leave.
- **B** is the third step. After A and C the wall is about 9 min, so B is the step that reaches 8 min. Without C, its shards wait on `phases/audit`.
- **D** is the long-term step. It has the largest ceiling per PR, but it needs the content-key work of ADR-0134 S6 and a careful integrity review.
- **E** stays a fallback. It buys time when a deadline comes before the work, and it fixes none of the causes.

**What changes this decision:** the macOS-only results (F3.10) can depend on more packages than the build tags show. Then A's macOS scope must grow, and C becomes the first step.

## 4a. The telemetry index catalog

Each level keeps the indexes that explain its time. The last column says which decision an index serves. An index that serves no decision is not kept.

**L1, one test** (source: the `go test -json` stream)

| Index | Meaning | Serves |
|---|---|---|
| `test.duration` | elapsed seconds | the slowest tests |
| `test.mode` | serial or parallel (a `pause` event marks parallel) | the serial chain of a package |
| `test.outcome` | pass, fail or skip | the failure rate |
| `test.retries` | out of scope now: CI does not retry a test | — |
| `test.flaky` | fail and pass on the same tree hash across runs (needs `--history`; the tree hash comes from the run's commit) | the flaky-test list |

**L2, one package** (sources: the `-json` stream and the `go test -exec` rusage wrapper, F3.13)

| Index | Meaning | Serves |
|---|---|---|
| `pkg.wall` | wall seconds of the test binary | the long poles |
| `pkg.build` | seconds to compile and link the test binary: the `build-output` events of `go test -json`, else the step time minus the Σ `pkg.wall` | build-cache and link cost |
| `pkg.floor` | serial sum + longest parallel test, and which part dominates | serial fix or parallel fix |
| `pkg.cpu.user`, `pkg.cpu.system` | CPU seconds of the binary and its waited-for descendants | own code or subprocesses and syscalls |
| `pkg.cpu.efficiency` | (user + system) / wall. Above 1 means parallel CPU use; the ceiling is the runner's vCPU count (3–4 on CI) | CPU-bound against wait-bound |
| `pkg.memory.peak` | peak RSS in MB, scaled per OS (F3.14) | memory pressure and runner size |
| `pkg.io.blocks` | block reads and writes (Linux only) | disk-bound packages |
| `pkg.ctxswitch.voluntary`, `.involuntary` | waits, and preemptions by the scheduler | contention |
| `pkg.tests`, `pkg.coverage` | test count and statement coverage | growth and the coverage gates |

**L3, the test step** (source: L2 rows of one step)

| Index | Meaning | Serves |
|---|---|---|
| `step.wall` | wall seconds | the job path |
| `step.concurrency` | Σ `pkg.wall` / `step.wall` | how well `-p` fills the CPUs |
| `step.long_pole` | the package with the largest `pkg.wall` | the next fix |

**L4, one job** (sources: the GitHub jobs API, the `setup-go` cache output and the runner sampler, F3.15)

| Index | Meaning | Serves |
|---|---|---|
| `job.queue` | seconds from create to start | runner shortage |
| `job.setup` | checkout, setup and module download seconds | cache tuning |
| `job.cache.hit` | the `setup-go` cache hit flag | cache tuning |
| `runner.cpu.busy`, `runner.runqueue` | CPU busy % and run-queue length, sampled each second | utilization and saturation |
| `runner.memory.used`, `runner.swap` | memory and swap in MB | memory pressure |
| `runner.disk.wait` | the share of time the disk is busy (Linux) | disk saturation |
| `job.cost` | job minutes × the billing multiplier of its runner type (a table kept in the repo) | the cost goal G4 |
| `job.cache.size`, `job.cache.restore` | restored cache MB and restore seconds | if the cache saves more than it costs |
| `job.artifact.bytes` | the bytes this job uploads | the cost of the telemetry itself |
| `job.network.bytes` | module download MB (deferred: not kept now, because modules come from the cache) | dependency growth |

**L5, the workflow** (source: the GitHub runs API; names follow OpenTelemetry CI/CD, P4.6)

| Index | Meaning | Serves |
|---|---|---|
| `cicd.pipeline.run.duration` | run wall seconds, by result | G1 |
| `cicd.pipeline.run.errors` | errors of a run, by error type; the failed job or step is an attribute | failure classes |
| `ci.critical_path` | the job, step, package and test that set the run's time | the next fix |
| `ci.main.red_time` | how long main stays red after a red commit (history) | time to green |
| `ci.change_failure_rate` | the share of merges whose main run is red (history; a proxy for the DORA change failure rate) | gate quality |

*Worked example (F3.13):* `phases/ship` has `pkg.wall` 87 s, `pkg.cpu.user` 88 s and `pkg.cpu.system` 293 s. Its `pkg.cpu.efficiency` is 4.4, so it is CPU-bound, but mostly in the kernel. The next fix for it is fewer `git` subprocesses, not a faster runner. On 2026-10-17, main was red from f4a9005a7 to 57531f436, five commits. `ci.main.red_time` is the wall time between those two commits.

## 5. Decisions

| Id | Decision | Research id |
|---|---|---|
| D1 | Every Go job uploads its `go test -json` streams, test-integration and e2e, as an artifact. | F3.11, P4.1 |
| D2 | `cmd/testlatency` splits each package's time into the serial sum (tests that never emit `pause`) and the parallel phase. It names the longest parallel test and which part sets the floor. Today it reports only the sum over all tests and the slowest test. | F3.5, F3.7 |
| D3 | A `ci-critical-path` report reads the run's jobs and steps from the GitHub API and joins them with D2. It names the path from L5 to L1 in the job summary. | F3.1, F3.2 |
| D4 | Ubuntu 1.23 owns the coverage gates for OS-independent code: cover-changed, cover-strict, apicover and the function warnings. macOS keeps cover-changed and cover-strict for the darwin set, because ubuntu never compiles a darwin-only file. | F3.3, F3.10 |
| D5 | macOS runs the darwin-sensitive package set and the e2e tier, computed by a script from build tags and `GOOS` checks, not a hand list. | F3.10 |
| D6 | Go 1.27 runs vet, build and the full suite without coverage gates. | F3.3 |
| D7 | The skills-drift tests share one generator build per distinct source tree. | F3.6 |
| D8 | `cmd/evolve`'s slowest serial tests move to injected environments, or to `-run` lists in separate processes. | F3.7 |
| D9 | The `bridge` dwell tests use an injected clock. | F3.8 |
| D10 | A `go test -exec` wrapper (`cmd/testrusage`) writes one rusage record per test binary to an NDJSON file. It never changes the exit code or the output. | F3.13, F3.14, P4.7 |
| D11 | One runner sampler per Go job writes CPU, run queue, memory, swap and disk wait each second, and stops when the job ends. | F3.15, P4.8 |
| D12 | `evolve ci telemetry` joins L1–L5 for one run from its artifacts and the GitHub API, and writes the §4a indexes. With `--history N`, it gives trends, the flaky list, and the P50 and P95 of `cicd.pipeline.run.duration`, `step.wall` and `pkg.wall`, because a mean hides the tail. | F3.11, P4.6, P4.8 |
| D13 | The index names follow the OpenTelemetry CI/CD conventions where they exist. The `ci.*` names are local, because OpenTelemetry has no equivalent. No collector or backend is added now. | P4.6 |

## 6. Components

| Id | Component | Red test first | Lane |
|---|---|---|---|
| C0 | D1 artifact upload, plus D2 in `cmd/testlatency` | a `testlatency` test: a fixture stream with a serial chain reports that chain as the floor | Sonnet |
| C0b | D10 rusage wrapper and D11 runner sampler, both uploaded with the C0 artifacts | a wrapper test: a child allocates 64 MB and spawns one subprocess. The record shows peak RSS ≥ 64 MB and system CPU > 0. The exit code passes through. A sampler test: a 2 s run writes at least one record per second, and the file closes when the job ends. | Sonnet |
| C1 | D3 `ci-critical-path` report, as `evolve ci telemetry` (D12). Its red-test fixture also holds a rusage record and a sampler record, so the join is proven | a fixture of jobs, steps and test JSON gives the expected L5→L1 path | Sonnet |
| C2 | D4–D6 matrix scope, plus the darwin-set script | a script test: a package with a `_darwin.go` file is in the set, and a plain package is not | Sonnet, workflow review by the console |
| C3 | D7 skills-drift build sharing | a test counts generator builds per run: one per distinct tree | Sonnet |
| C4 | D8 `cmd/evolve` serial chain | the AST parallel guard shows the moved tests as parallel | Sonnet |
| C5 | D9 `bridge` injected clock | each dwell test passes with the fake clock in under 1 s | Sonnet |

## 6a. Components after the first instrumented run (C6 to C14)

On 2026-10-17 the operator asked: "Continue investigating the performance, latency, resource usage for ci pipeline and test cases". The C0b run 39031338517 gave the data (research F3.20 to F3.35). This section adds nine components, ranked by the wall time that they save on the critical path, then by CPU-seconds. PR #935 owns D14 and C5b, so the new ids start at D15 and C6.

**Terms.** A *slot-second* is one second of one `-p` slot. A test binary holds its slot while it sleeps, so a sleep costs slot-seconds but no CPU. *Wall* is the time that a person waits.

*Worked example (F3.26):* `GORACE` sleeps 1 s at the exit of each of 301 race binaries. That is 301 slot-seconds. macOS has 3 slots, so the sleep can add up to 301 / 3 ≈ 100 s to its test step.

### The ranked components

The savings are synthesis from the research ids in the last column. "mac" is the macOS 1.23 job, and "u23" is the ubuntu 1.23 job.

| Rank | Id | Component | Measured target | Expected saving: wall | Expected saving: CPU-s | Red test first | Guarantee kept | Research |
|--:|---|---|---|---|---|---|---|---|
| 1 | C6 | D15: the e2e step runs only the `e2e`-tagged tests | mac e2e step 118–176 s; 188.9 of 247 test-seconds repeat the integration step | mac −110 s (176 → about 65 s), on the critical path before and after C2; u23 −15 to −25 s | mac about 150; u23 about 30 | a `ciparity` test: the `make -n test-e2e` command selects each test of an `e2e`-tagged file, and no test of a plain file | every `e2e`-tagged test runs on every OS. Every other `cmd/*` test still runs under `-race` in the integration step | F3.27 |
| 2 | C7 | D16: `GORACE=atexit_sleep_ms=0` for each `-race` test run | 301 binaries × 1.0 s on each Go job; no race binary below 1.008 s | mac up to −100 s before C2, about −6 s after C2. u23 up to −75 s of slot time; its CPU is saturated, so less wall | 0; the local floor of each lane saves about 300 slot-seconds | a `ciparity` test: a race test binary with one empty test, run with the Makefile's environment, exits in under 0.5 s | the detector checks every access before the exit. Only a race in the last second after `main` returns can escape | F3.26, P4.9 |
| 3 | C8 | D17: the deliverable read grace is a value that tests set | `deliverable` 40 s (mac 59 s) at CPU/wall 0.08; `phases/runner` 29 s (mac 32 s), with two tests of 8.2 s | u23 about −55 package-seconds; mac about −70; off the critical path today, on it after C2 for `phases/runner` users | about 0 | the two `ArtifactTimeout` tests pass in under 1 s; a missing-file read under the test value returns in under 50 ms | production keeps 500 ms. `grace_test` keeps one real-window test | F3.25 |
| 4 | C9 | D18: one parse of the tree per test binary, one package of ASTs in memory at a time | `repocontract` 1,222 MB peak RSS and 2 full parses of 4,441 files; `cmd/evolve` ratchet and inbox scans | u23 about −20 to −30 package-seconds; mac about −40 | about 25 | a parse counter seam: each file parses once per binary. A heap test: `repocontract` scan peak heap < 400 MB | the scanners give the same findings: a golden run of the old and the new scanner on the real tree | F3.25, F3.28 |
| 5 | C10 | D19: the `bridge` tick-loop tests reach their state in fewer ticks | `TransientDwell` Σ 102 s on main (167 s in the PR #935 run); the three pane-loss tests 600–1,200 ticks each, 20–27 s | u23 about −60 to −120 test-seconds in `bridge`, the second CPU hog; mac about −100 | about 60–100 | each test asserts its tick count through the fake sleep, and passes in under 2 s | each test asserts the same state changes. One test keeps the production thresholds (300 s and 1,200 s) | F3.25 |
| 6 | C11 | D20: vet and gofmt run once per GOOS, not once per job | vet + fmt 37–59 s on mac and 38–51 s on u27 | mac −45 s on the critical path; u27 −10 s (gofmt only) | about 100 (assumption) | a workflow test: only ubuntu 1.23 runs gofmt; ubuntu runs `GOOS=darwin go vet ./...`; mac runs no vet | the darwin files are still type-checked and vetted. Go 1.27 keeps its own vet for new analyzers | F3.30 |
| 7 | C12 | D21: the C1 report fixes B1 to B4 | the `limit` field is wrong for 303 u27 packages (B1) and 184 mac packages (B2) | 0; it makes the next decisions correct | 0 | fixture tests with the C0b numbers. A package window at 60% busy is not `cpu-saturation`. A 1 s binary with 0.011 s of CPU is `wait`. mac has 3 CPUs. `core` at 4× its floor is `contention` | the report never changes a run. Its JSON keeps every current field | F3.34 |
| 8 | C13 | D22: the macOS sampler records CPU busy; a spawn counter counts the `git` and `go` children of each package | mac has no CPU samples; mac system CPU is 6.3× ubuntu | 0; it decides C14 and the `-p` question on macOS | about 1 per job | a sampler test on darwin: each record has `cpu_busy_pct` in 0–100. A shim test: one `git` call under package P adds one count to P | the sampler and the shim never change a test result or its exit code | F3.23, F3.35 |
| 9 | C14 | D23: fewer `git` processes in the macOS-heavy tests, only if C2 keeps them on macOS | `core` 362 s with 310 s system CPU; `gc`, `explanationdocs`, `inboxstamps` at 7–11× ubuntu | up to −250 s on mac if C2 does not land; about 0 if it does, because these packages leave the macOS job | up to 400 | C13's counter shows fewer spawns per scenario, with the same assertions | each scenario still runs the real `git` path once per distinct shape | F3.22, F3.33 |

**Two corrections to earlier components.**
- **C5** assumes that the `bridge` dwell tests wait in real time (F3.8). The code shows a simulated loop with a fake sleep (F3.25), so an injected clock cannot shorten it. C10 replaces C5 for those tests.
- **C4** must move seams and env writes into injected values. Seam swaps and env writes cause 171 s of the 238 s macOS serial chain of `cmd/evolve` (F3.29).

**Combined effect (synthesis).** Before C2, C6 + C7 + C11 can cut the macOS job by about 4–4.5 min: 110 + up to 100 + 45 s. The run is then about 20 min on a slow day. After C2, C6 and C11 still apply to macOS, because macOS keeps the e2e tier. The ubuntu step loses CPU-seconds through C8, C9 and C10. On a saturated runner, those seconds become wall time.

### C6: four approaches, scored

The problem (F3.27): `go test -tags "e2e evolve_test_phases" ./cmd/... ./test/e2e/...` builds each `cmd/*` package with the e2e tag and then runs all of its tests. Only 28 top-level tests in `cmd/evolve` need the tag.

- **A. A `-run` list from the tagged files.** A small Go command lists the `Test` functions in each file whose build line names `e2e`. `make test-e2e` passes them as one `-run` pattern per package. *Advantage:* no test moves, and a new e2e test joins the list by its build tag. *Limit:* a plain test with the name of an e2e test also runs, which costs only time.
- **B. Move the e2e tests to their own package.** For example, `cmd/evolve/e2e` holds them, and the e2e step runs only that package. *Advantage:* the tag selects the package, so no list is needed. *Limit:* the tests use the internals of package `main`, so most of them must change.
- **C. Run the e2e tests in the integration step.** Add `e2e` to the integration tags and remove the e2e step. *Advantage:* one compile and one run. *Limit:* the e2e tests then run under `-race` and `-cover`, which the e2e tier does not want (`go.yml` "e2e tier (no race …)"). The integration step on macOS also grows.
- **D. Keep the double run.** *Advantage:* the non-e2e tests also pass without `-race` and with the `evolve_test_phases` tag. *Limit:* about 110 s on the critical path for one weak signal.

Rejected without a score: a `//go:build !e2e` line on every plain test file. It touches about 1,000 files, and a new file without the line joins the e2e run again.

| Criterion (weight) | A. `-run` list | B. own package | C. merge into integration | D. keep |
|---|:-:|:-:|:-:|:-:|
| Wall time saved (×3) | 5 (−110 s mac) | 5 (−110 s mac) | 3 (the e2e tests move into a longer step) | 1 |
| Keeps all evidence, G3 (×3) | 4 (loses the no-race repeat) | 4 (the same) | 3 (no no-race e2e run) | 5 |
| Effort and risk (×2) | 4 (one small command, one Makefile line) | 1 (moves 28 tests out of `main`) | 3 (timeouts and race cost of the e2e tests) | 5 |
| Runner cost, G4 (×2) | 5 | 5 | 4 | 1 |
| Helps local runs and the loop (×1) | 4 (`make test-e2e` is faster) | 4 | 2 | 1 |
| **Weighted total (out of 55)** | **49** | **43** | **34** | **31** |

In approach A, the `-run` pattern is anchored (`^(…)$`), so a plain test with the same name never matches. The C6 change also updates `internal/ciparity/makefile_integration_test.go`, which pins the e2e target.

**Winner: A.** It saves the most for the least change, and its selection follows the build tag, so it cannot drift from the code. **Each alternative keeps a role.** B is the clean end state if the e2e tests are later rewritten against a public seam. C is the fallback if a future e2e test needs `-race`. D's no-race signal can come back as a weekly scheduled run if a defect ever appears only without `-race`.

### Decisions D15 to D23

| Id | Decision | Research id |
|---|---|---|
| D15 | `make test-e2e` runs only the tests of `e2e`-tagged files, through a `-run` list that a command builds from the build tags (C6, approach A). | F3.27 |
| D16 | Each `-race` test recipe sets `GORACE=atexit_sleep_ms=0`, in CI and in the loop's floor. | F3.26, P4.9 |
| D17 | `deliverable` reads its grace window from a value that tests can set. Production keeps 500 ms. | F3.25 |
| D18 | The whole-tree scanners share one parse per test binary, and they release the ASTs of each package after its scan. | F3.25, F3.28 |
| D19 | The `bridge` tick-loop tests set test-scoped thresholds, so each reaches its state in at most about 10 ticks. One test keeps the production thresholds. | F3.25 |
| D20 | gofmt runs on ubuntu 1.23 only. vet runs on ubuntu 1.23 for both `GOOS=linux` and `GOOS=darwin`, and on ubuntu 1.27. macOS runs no vet. | F3.30 |
| D21 | The C1 report decides `cpu-saturation` over the package's own time window. It skips the system-CPU rule below 0.5 s of CPU. It reads the CPU count per runner type. It adds the limit `contention` when the wall is more than 2× the floor. | F3.34 |
| D22 | The darwin sampler reads CPU busy from one `iostat -w 1` child. The rusage wrapper exports `TESTRUSAGE_PKG`, and a `git`/`go` shim on `PATH` counts the spawns of each package. | F3.23, F3.35 |
| D23 | C14 starts only if C2 keeps `core` or the other `git`-heavy packages on macOS, and only with C13's spawn counts. | F3.22, F3.33 |

### Order of the new components

1. **C12 and C13 first.** They measure, and they cost little. The rule of §7 applies: measure before you change.
2. **C7, then C6, then C11.** C7 is one line. C6 and C11 save macOS wall now, and they still save it after C2.
3. **C8, C10, C9.** They cut ubuntu CPU-seconds and package walls. Each re-measures with C1.
4. **C14 only under D23.**

### Build status of C6, C7 and C11

C7, C6 and C11 are built, in that order, on the branch `perf/ci-c6-c7-c11`.

| Id | Built as | Red test | What the build records |
|---|---|---|---|
| C7 | `go/Makefile` exports `GORACE` with `atexit_sleep_ms=0` appended to any value that the caller set. The line covers `make test`, `make test-component` and `make test-integration`. | `TestMakeRaceRecipes_DropTheExitSleepAndKeepOtherOptions` runs the recipes in a fixture and checks the value that the test binary sees, with and without a preset `GORACE`. | The plan's timing test (a race binary exits in under 0.5 s) is not built. The speed of the runner can change the time. The test checks the value, not the time. |
| C6 | `make test-e2e` builds `cmd/e2elist`. The command prints one anchored pattern, `^(TestA\|TestB\|...)$`, and the recipe passes it to `go test -run`. `internal/ciparity` holds the logic (`E2ETestNames`). A test function is on the list when only the `e2e` tag adds its file to the package. The list follows the build tag, not a name. | `TestE2ETestNames_PicksUpANewE2EFileAndIgnoresANewPlainFile` and `TestMakeE2E_RunsOnlyTheTestsOfE2ETaggedFiles`. | The count is 28 tests, and the no-race repeat of the other `cmd/*` tests is lost; the scoring accepted this loss. A plain test with the name of an e2e test in **another** package still runs. One `-run` pattern applies to every package. This costs only time. The anchor stops a prefix match. The recipe fails when the list is empty. |
| C11 | The step `vet + fmt` leaves the matrix job `build-test`. The job `lint` (ubuntu, Go 1.23) runs `go vet ./...`, `GOOS=darwin go vet ./...` and gofmt. The job `vet-current` (ubuntu, Go 1.27) runs `go vet ./...`. | `TestGoWorkflow_VetsAndFormatsOncePerPlatformNotOncePerMatrixLeg` | The `required` workflow forbids a conditional `run` step in a validation job. The platform split is therefore made with jobs, not with a step `if`. A failed job still fails the reusable workflow. The job names `lint` and `vet-current` replace the step name `vet + fmt` that some audit messages quote. |

## 7. Order

1. **C0, then C0b, then C1.** Measure before you change: time first, then resources, then the joined report. The history is also the input for any sharding later.
2. **C2.** This is the largest wall-time cut, and it is a workflow-only change.
3. **C3, then C4 and C5.** These are ordered by CPU-seconds freed (F3.5, F3.6).
4. **Re-measure with C1.** Start design B if the wall is still above 8 min, as the estimate for A and C predicts. Open design D as its own plan with ADR-0134 S6.

Each component is one PR, merged at a wave boundary. Each one follows the usual review chain and floor.

## 8. Verification

| Goal | Before (2026-10-16) | After target | How it is measured |
|---|---|---|---|
| G1 wall time | 21–25 min | about 9 min after C2–C5; ≤ 8 min needs design B | `gh run view --json createdAt,updatedAt` over ten PR runs |
| G1 macOS job | 20.7–24.6 min | ≤ 11 min after C2 | C1 report |
| G1 ubuntu test step | 596–616 s | ≤ 400 s after C3–C5 | C1 report |
| G2 telemetry | none kept | every Go job has the `-json`, rusage and sampler artifacts, and a critical-path summary with its limiting resource | artifact list of each run |
| G3 evidence | — | every darwin-tagged package still runs on macOS | the C2 script test, and a diff of the macOS package list against the set |
| G4 cost | 3 full jobs | macOS minutes cut by about 55% | billing summary |

## 9. Risks

| Risk | Control |
|---|---|
| A macOS-only failure sits in a package without a darwin build tag. | The C2 script also selects `GOOS` checks and `sandbox-exec` callers. A weekly scheduled full macOS run is the backstop. |
| Coverage gates move to ubuntu, and ubuntu never compiles a darwin-only file, so such a file leaves its coverage. | macOS keeps cover-changed and cover-strict for the darwin set (D4). Only the OS-independent gates move to ubuntu alone. |
| A shared generator build in C3 hides a per-tree difference. | The build is keyed on the tree's content hash, not on the test name. |
| Injected environments in C4 change what a test proves. | Each moved test keeps its assertions. The review checks that the environment it reads is the one it injects. |
| The wrapper changes a test result or its timing. | The wrapper passes stdin, stdout, stderr and the exit code through, and its cost is one `exec` per package. A test proves the pass-through. |
| The runner sampler costs CPU on a 3-vCPU runner. | It samples once per second and reads kernel counters only. |

## 10. Open questions

- **Q1:** keep a weekly full macOS run, or a nightly one? Proposed: weekly, on main.
- **Q2:** does Go 1.27 need the e2e tier, or only the unit and integration suite? Proposed: the suite only.
- **Q3:** do the five `sandbox-exec`-only packages in F3.10 need macOS? Some name the binary only in a string. Proposed: keep them in the set until a test proves otherwise.
- **Q4:** does `fetch-depth: 0` need the full history in all three Go jobs? Checkout takes 21–45 s per job (F3.30). Proposed: measure a blobless clone (`filter: blob:none`) before a change.
- **Q5:** does the macOS runner gain from `-p 2` or `-p 4`? C13 must record its CPU first (F3.23).
