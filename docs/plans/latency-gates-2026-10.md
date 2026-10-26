# Plan: tests and gates that keep the latency fixes in place

- **Status:** proposed, 2026-10-20. Each component of §6 has an inbox item since 2026-10-27.
- **Research:** [latency-gates-2026-10](../research/latency-gates-2026-10.md). **Decision:** [ADR-0144](../architecture/adr/0144-latency-is-gated-by-invariants-at-merge-and-by-trends-on-main.md).
- **Related:** [wave-latency-2026-10](wave-latency-2026-10.md), [ci-wall-time-2026-10](ci-wall-time-2026-10.md).

## Table of contents

- [1. The problem](#1-the-problem)
- [2. Candidate designs](#2-candidate-designs)
- [3. The test plan for each fix](#3-the-test-plan-for-each-fix)
- [4. The measured gates](#4-the-measured-gates)
- [5. Decisions](#5-decisions)
- [6. Components and order](#6-components-and-order)
- [7. Limits](#7-limits)

## 1. The problem

Seven fixes cut the CI wall time from a median of 80 minutes to 22.4 minutes and raise the width of a wave (research §2). Nothing stops a later change from undoing one of them. A wall-time check on a CI runner cannot block a merge, because the runner noise is too large (G4). The plan needs a check that does not flake and still catches the regression.

## 2. Candidate designs

Each criterion scores 1 (poor) to 5 (good).

| Design | Catches a regression | Never flakes | Cost to run | Total |
|---|---|---|---|---|
| A. A wall-time budget on each CI run, as a hard fail | 3 | 1 | 5 | 9 |
| B. Behavior invariants and a seeded simulation as hard gates; measured trends as soft gates | 5 | 5 | 4 | **14** |
| C. Trends only: change-point detection on main, no hard gate | 3 | 5 | 4 | 12 |
| D. Dedicated quiet runners for exact wall-time benchmarks | 4 | 3 | 1 | 8 |

B wins. A fails on runner noise (G4). C finds a regression only after it lands, while B blocks the regressions that a test can prove. D needs hardware that the project does not have. [ADR-0144](../architecture/adr/0144-latency-is-gated-by-invariants-at-merge-and-by-trends-on-main.md) records the choice.

## 3. The test plan for each fix

Each hard gate tests behavior or counts work. A hard gate never reads the wall clock (G7, G8).

| Fix | Hard gate (blocks the merge) | What it proves |
|---|---|---|
| #988 CI concurrency | A test parses `required.yml` and `go.yml` and evaluates the concurrency expression for `pull_request`, `push` and `workflow_dispatch`. A PR run gets a group for its PR number and `cancel-in-progress: true`. A push gets the group of its ref and `false`. A dispatch gets a group for its run. An inbox-only diff takes the light route. | a stale PR run is cancelled, and a running main run is never cancelled |
| #990 the identity carry | `make test` fails on any path that a test leaves in the tree (the tree-walk check). A test proves that a byte-identical rebase with green gates keeps its audit | a rebase does not start a new audit |
| #991 pool resupply | The wave simulation of §3.1: no slot is idle while an admissible todo waits, after each event | the pool takes work that arrives after the start |
| W1 admission | A property test over seeded random lane sets. Two running lanes never share an edited package. At `enforce`, a core lane does not hold a candidate with no shared edit. At `shadow`, the admitted set equals the old rule. | the width gain, and the safety rule |
| W5 verdict store | Count the packages that the ship runs: 0 at `enforce` for an unchanged key, all at `shadow`; a FAIL is never reused | the ship does not test again what the floor tested |
| W4 build order | Fixture tests of the chain: headroom moves a family to the tail, and a shorter lane time to a PASS goes first | the faster family leads while it has quota |
| C2 macOS scope | The drift guard: a darwin-tagged file in a package outside the set fails the test | the macOS job stays small |

### 3.1 The wave simulation

The simulation drives the real `fleet.RunPool` and the real admission with no clock (F3.4, G8):

- The test seeds a generator. The generator gives each todo a lane time from the distribution of waves 130 to 133, and some todos arrive after the start.
- A fake `LaunchFn` ends each lane when the test sends on its channel. The test sends in the order of the virtual lane times, so the event order is the clock. No goroutine sleeps.
- Arrivals reach the pool through `Resupply`. The pool calls it when a lane exits, not on a kick.
- The test checks the invariants after each settle terminal event (`PoolLaunched`, `PoolBackfilled` and `PoolIdle`). It does not check at `PoolExited`, because `settle` emits `PoolExited` before the fill and the resupply (`go/internal/fleet/pool.go:132-138`).
  - No slot is idle while an admissible todo waits. A todo is admissible when its files are disjoint from the claimed set. `Prepare` always succeeds in the simulation.
  - The live count never passes the target.
- The bound on the virtual length of the wave (Cmax) is the bound of Graham for greedy list scheduling: Cmax ≤ Σp/m + (1 − 1/m)·pmax. Here p is a lane time and m is the target.
  - The bound holds for a pool that never idles, has no holds, and has all work at time 0.
  - The test asserts the bound only on seeds with no file conflicts and with `Window` nil. For arrivals, it adds the latest arrival time: Cmax ≤ r_max + Σp/m + pmax.
  - On seeds with conflicts, the test asserts the invariants and Cmax ≤ Σp.
- A failure prints the seed, so the run replays exactly.

The test runs 200 seeds in less than 2 seconds. Go 1.23 has no synctest, so the event order replaces the clock (F3.5).

## 4. The measured gates

These gates watch a trend and file an item. They never block a merge (G4, G12).

| Measure | Source | Breach rule |
|---|---|---|
| CI run time for each job and OS | the jobs API (research method) | see §4.1 |
| CI queue time for each job | `started_at` minus `created_at` | p90 over 7 days above 2 minutes |
| Package wall time in CI | `test-integration.json` (C0), read by `cmd/testlatency` | a package above min(2 × its class ceiling, `TEST_TIMEOUT`), or above its recorded ratchet (§4.2) |
| Wave width and gate share | `evolve wave latency` (W0) at each boundary | see §4.3 |

### 4.1 The rule for CI times

- One point for each main run of each job and OS. Superseded and cancelled runs are left out.
- The tested window is the last 10 main runs, the minimum of G3. The baseline is the 30 good main runs before the window, at most 28 days old. The baseline never includes a tested run (G6).
- The gate turns on when the baseline has 10 points and the window has 10 runs.
- A breach needs two facts together (G5):
  - the median of the window is more than 15% above the baseline median;
  - the increase is more than 60 seconds for the ubuntu job and more than 90 seconds for the macOS job.
- A one-sided exact Mann-Whitney U test with a tie correction of the window against the baseline must give p < 0.01. The smallest exact p is about 1.2e-9 for 10 runs against 30, and 5.4e-6 for the 10-against-10 minimum, so the level is reachable. The sample size is fixed, and there is no rerun (G3).
- Two breaches in a row mean two disjoint windows. A rolling window shares runs, so it cannot give two independent breaches. The two windows file one inbox item for that measure. The item closes when the series recovers.

### 4.2 The duration ratchet for test packages

- Each package has a size class like Bazel's (G10): small 60 s, medium 300 s, large 600 s.
- The Makefile sets `TEST_TIMEOUT ?= 20m` for each package (`go/Makefile:116`). The project uses 600 s for the large class, not the 900 s of Bazel (G10). Twice the ceiling is 1200 s, which equals `TEST_TIMEOUT`. A ceiling of 900 s can never trigger.
- `go/.test-latency.json` records the class and the p90 of each package, in the pattern of `sizeratchet` (F3.2).
- CI fails only when a package runs more than min(2 × the class ceiling, `TEST_TIMEOUT`). That headroom is too large for runner noise to reach.
- T4 reuses `addedtests.PackageWallBudget` and the flags `--fail-over-budget` and `--package-budget` (F3.1). It adds only the class and the ratchet file.
- A package above its recorded p90 by 50% and by more than 30 seconds gets a warning in the job summary.
- `testlatency tighten` lowers a recorded value once a week from main. Only a reviewed diff raises one.

### 4.3 The wave gate

- `evolve wave latency --check` (pending W0, PR #997) runs at each boundary and reads the bounds from policy.
- T6 adds the gate share and a flag for a brake or a quota pause to `wavemeasure.Facts` (F3.3).
- A wave breaches when the ready supply stays at or above the target and the average live lanes is below 3.2 of 4. It also breaches when the host gate share is above 25%.
- A wave is left out when one of these sources shows a pause or a brake:
  - the signal `ORCHESTRATOR_QUOTA_PAUSED` (`go/internal/core/signal_cycle.go:14`);
  - the `stop_reason` of the loop (`go/cmd/evolve/cmd_loop_outcome.go:27`);
  - the operator brake (`go/cmd/evolve/cmd_loop_stop.go`).
- Two breaching waves in a row emit `LATENCY_BUDGET_EXCEEDED` (WARN) and file one item.

## 5. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | Every fix gets a hard gate that tests behavior or counts work, never the wall clock. | G4, G7, G8 | A wall-time hard gate: G4. |
| D2 | A seeded wave simulation over the real pool and admission is a hard gate. | G8, G9, F3.4 | A live-wave check only: it finds the regression after it lands. |
| D3 | CI times, package times and wave measures are soft gates with a relative and an absolute threshold and two breaches in a row. | G1, G5, G12 | One threshold: G5. One slow run: G12. |
| D4 | The package gate fails CI only above min(2 × the class ceiling, `TEST_TIMEOUT`), and warns below that. The large class is 600 s. | G10, F3.1 | A tight per-package budget: runner noise fails it. |
| D5 | Every soft breach files one item for each measure, and the item closes on recovery. | G12 | An alert in a log only: no one acts on it. |

## 6. Components and order

| # | Component | Home | Route | Inbox item |
|---|---|---|---|---|
| T1 | The workflow tests of #988 | `go/internal/repocontract` or a new `ciworkflow` test | lane | `latency-gates-t1-ci-concurrency-tests` |
| T2 | The wave simulation of §3.1 | `go/internal/fleet`, `go/internal/orchestrator` tests | lane | `latency-gates-t2-wave-simulation` |
| T3 | The W1 property test, the W5 count test, the W4 fixtures | with each PR's package | the PR owner | `latency-gates-t3-wave-fix-proofs` |
| T4 | The duration ratchet: `go/.test-latency.json`, the class and `tighten` on the existing flags, and the CI step | `go/cmd/testlatency`, `.github/workflows/go.yml` | console | `latency-gates-t4-duration-ratchet` |
| T5 | `evolve ci latency check`: the CI trend gate of §4.1, run at each boundary, files items | `go/cmd/evolve`, a new `internal/citrend` | lane | `latency-gates-t5-ci-trend-gate` |
| T6 | `evolve wave latency --check` and `LATENCY_BUDGET_EXCEEDED` (§4.3), after W0 | `go/internal/wavemeasure`, `go/internal/lanetime`, `go/cmd/evolve` | console | `latency-gates-t6-wave-latency-check` |
| T7 | The boundary runs T5 and T6 | `evolve wave next` steps | console | `latency-gates-t7-boundary-runs-checks` |

T1, T2 and T3 come first, because they protect the fixes that are already live. T5 and T6 both change `go/cmd/evolve`, so T6 starts after T5 merges.

The plan audit of 2026-10-27 filed one inbox item for each component, and each item is `lane`. T4, T6 and T7 touch no gate file, so admission decides. The test of T1 extends `go/internal/ciparity`, which holds the workflow tests now. The W1 property test of T3 is on `main` (`go/internal/admission/optimistic_test.go`). The T3 item checks the W5 and W4 proofs.

## 7. Limits

- A soft gate finds a regression only after it lands on main. The hard gates cover only what a test can prove.
- The simulation uses the distribution of four waves. A new kind of work can change the distribution, so the bound can need a new baseline.
- The CI rule needs 10 baseline points and 10 tested runs before it turns on.
