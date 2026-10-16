# Plan: CI wall time falls from about 24 to about 9 minutes, and every run keeps the data that shows why

- **Status:** proposed, 2026-10-17.
- **Research:** [ci-wall-time-2026-10](../research/ci-wall-time-2026-10.md). **Earlier campaign:** [test-system-efficiency-2026-10](test-system-efficiency-2026-10.md). **Decision:** an ADR follows when the operator accepts this plan.

## Table of contents

- [1. Request](#1-request)
- [2. Facts](#2-facts)
- [3. Goals and non-goals](#3-goals-and-non-goals)
- [4. Candidate designs](#4-candidate-designs)
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

## 3. Goals and non-goals

**Goals**

- **G1:** the `required CI` wall time is about 9 min after designs A and C. Today it is 21–25 min. 8 min or less needs design B as well.
- **G2:** every run keeps its per-test timings. A report names the critical path from L5 (workflow) down to L1 (test).
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

## 6. Components

| Id | Component | Red test first | Lane |
|---|---|---|---|
| C0 | D1 artifact upload, plus D2 in `cmd/testlatency` | a `testlatency` test: a fixture stream with a serial chain reports that chain as the floor | Sonnet |
| C1 | D3 `ci-critical-path` report | a fixture of jobs, steps and test JSON gives the expected L5→L1 path | Sonnet |
| C2 | D4–D6 matrix scope, plus the darwin-set script | a script test: a package with a `_darwin.go` file is in the set, and a plain package is not | Sonnet, workflow review by the console |
| C3 | D7 skills-drift build sharing | a test counts generator builds per run: one per distinct tree | Sonnet |
| C4 | D8 `cmd/evolve` serial chain | the AST parallel guard shows the moved tests as parallel | Sonnet |
| C5 | D9 `bridge` injected clock | each dwell test passes with the fake clock in under 1 s | Sonnet |

## 7. Order

1. **C0, then C1.** Measure before you change. The history from C0 is also the input for any sharding later.
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
| G2 telemetry | none kept | every Go job has a `-json` artifact and a critical-path summary | artifact list of each run |
| G3 evidence | — | every darwin-tagged package still runs on macOS | the C2 script test, and a diff of the macOS package list against the set |
| G4 cost | 3 full jobs | macOS minutes cut by about 55% | billing summary |

## 9. Risks

| Risk | Control |
|---|---|
| A macOS-only failure sits in a package without a darwin build tag. | The C2 script also selects `GOOS` checks and `sandbox-exec` callers. A weekly scheduled full macOS run is the backstop. |
| Coverage gates move to ubuntu, and ubuntu never compiles a darwin-only file, so such a file leaves its coverage. | macOS keeps cover-changed and cover-strict for the darwin set (D4). Only the OS-independent gates move to ubuntu alone. |
| A shared generator build in C3 hides a per-tree difference. | The build is keyed on the tree's content hash, not on the test name. |
| Injected environments in C4 change what a test proves. | Each moved test keeps its assertions. The review checks that the environment it reads is the one it injects. |

## 10. Open questions

- **Q1:** keep a weekly full macOS run, or a nightly one? Proposed: weekly, on main.
- **Q2:** does Go 1.27 need the e2e tier, or only the unit and integration suite? Proposed: the suite only.
- **Q3:** do the five `sandbox-exec`-only packages in F3.10 need macOS? Some name the binary only in a string. Proposed: keep them in the set until a test proves otherwise.
