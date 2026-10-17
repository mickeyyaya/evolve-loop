# Plan: CI wall time falls from about 24 to about 9 minutes, and every run keeps the data that shows why

- **Status:** proposed, 2026-10-17.
- **Research:** [ci-wall-time-2026-10](../research/ci-wall-time-2026-10.md). **Earlier campaign:** [test-system-efficiency-2026-10](test-system-efficiency-2026-10.md). **Decision:** an ADR follows when the operator accepts this plan.

## Table of contents

- [1. Request](#1-request)
- [2. Facts](#2-facts)
- [3. Goals and non-goals](#3-goals-and-non-goals)
- [4. Candidate designs](#4-candidate-designs)
- [4a. The telemetry index catalog](#4a-the-telemetry-index-catalog)
- [5. Decisions](#5-decisions)
- [6. Components](#6-components)
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
