# ADR-0135: CI measures itself before it changes its shape, and each job proves only what it alone can prove

- **Status:** Accepted (2026-10-17). The operator approved the plan with "Ok".
- **Plan:** [ci-wall-time-2026-10](../../plans/ci-wall-time-2026-10.md). **Research:** [ci-wall-time-2026-10](../../research/ci-wall-time-2026-10.md). **Design:** [ci-telemetry-and-critical-path](../ci-telemetry-and-critical-path.md).
- **Relates to:**
  - [ADR-0131](0131-one-test-run-feeds-every-coverage-gate.md): one test run feeds every coverage gate. This ADR keeps that rule: each job runs its suite once, and its gates read that one run.
  - [ADR-0134](0134-fleet-landings-scale-through-content-keyed-verification-and-group-commit.md): its content keys are the base for test-result reuse, which this ADR defers.
- **Evidence:**
  - F3.1: the macOS job sets the 21–25 min wall time. The ubuntu jobs take 9–13 min.
  - F3.11: the CI writes per-test times and keeps none of them.
  - F3.13: the slowest packages spend 2.5–3.3 times more CPU in the kernel than in user code.
  - F3.10: 16 of 303 packages hold darwin-specific code.
  - F3.9: race and coverage add about 20% on the packages measured.

## Context

The CI takes 21–25 minutes, and the operator asked why. No record had the answer, because the CI deleted its own test stream. The investigation measured by hand and found the answer on five levels. The macOS job is the critical path. Inside it, single packages set the floor, and the slow packages spend their CPU in the kernel, on subprocesses.

Two forces push against each other. A fast fix is attractive: drop `-race`, or buy larger runners. But the measurement of two packages shows that `-race` adds about 20%, and larger runners do not shorten a serial chain. A change made without measurement can cost money and fix nothing. So the CI must first measure itself, and then each change must cut a measured cause.

Coverage of OS-independent code gives the same result on every OS, so one job owns it. A darwin-only file compiles only on macOS, so macOS keeps its gates. Go 1.27 checks only that the suite passes on the next Go version.

## Decision

1. Every Go job keeps its telemetry: the test streams, a rusage record per package and a runner sampler.
2. The telemetry joins into one report that names the critical path from the workflow down to one test.
3. Telemetry comes before any change to the shape of the CI.
4. Ubuntu 1.23 runs the full suite and the coverage gates for OS-independent code.
5. macOS runs only the darwin-sensitive packages, their coverage gates and the e2e tier.
6. Go 1.27 runs the full suite without the coverage gates.
7. The long poles get fixes at the unit level after the matrix change, in order of CPU time freed.
8. Sharding and test-result reuse wait until the report shows that the 8-minute target needs them.
9. A weekly job runs the full suite on macOS.

## Consequences

**Good:**
- The wall time falls from about 24 to about 12.5 minutes with the matrix change alone, and to about 9 with the unit fixes. This is an estimate from the plan.
- macOS runner minutes fall by about 55%. macOS minutes cost about 10 times Linux minutes.
- Each later change has a measured before and after, so a change that does not help is visible.
- The resource view shows the kind of fix that each package needs: fewer subprocesses, parallel tests, or a fake clock.

**Bad:**
- The macOS job no longer runs most packages. A macOS-only failure in a package without a darwin marker can reach main. A weekly full macOS run is the backstop.
- The telemetry adds artifacts and a sampler process to each job. Their size and cost are indexes of their own (`job.artifact.bytes`), so the cost stays visible.
- Coverage of darwin-only files stays on macOS, so two jobs own coverage gates, not one.

**What changes this decision:** if the darwin set misses a package that fails only on macOS, the set must grow. Then the unit-level fixes come first, because they help every job.

## Rejected

| Option | What it offers | Why it is not first |
|---|---|---|
| Shard the ubuntu suite by timing | It scales with the suite and needs no test change. | macOS stays the critical path, so it saves no wall time alone. Its shards also wait on the long-pole packages. |
| Fix the long poles first | It also speeds up local runs and the loop's build floor. | Each fix needs a measured cause first. The matrix change removes more macOS time for less work. |
| Reuse test results whose inputs did not change | It has the largest saving per PR, and the saving grows with the suite. | Its content key must cover non-Go inputs that tests read through `../`. It needs ADR-0134 S6 first. |
| Buy larger runners | It needs no code change, and the effect is immediate. | It costs money on every run, and it cannot shorten a serial chain. |
| Drop `-race` on macOS | It needs one flag change. | It saves about 20% only (F3.9). Scope saves much more. |
