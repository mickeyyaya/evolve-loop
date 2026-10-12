# Plan: a test system that measures once, keeps its poles short and stops new duplicates

- **Status:** accepted, 2026-10-12. T0 (these documents) is in review. T1 and T2 are the first refactor slices.
- **Design:** [test-system.md](../architecture/test-system.md). **Decision:** [ADR-0131](../architecture/adr/0131-one-test-run-feeds-every-coverage-gate.md). **Research:** [test-system-efficiency-2026-10.md](../research/test-system-efficiency-2026-10.md).
- **Earlier campaign:** [test-refactoring-design-2026-09-14.md](../architecture/test-refactoring-design-2026-09-14.md). Its preservation protocol applies to each slice of this plan that removes or merges a test.

## Table of contents

1. [Request](#1-request)
2. [Facts](#2-facts)
3. [Goals and non-goals](#3-goals-and-non-goals)
4. [Candidate designs](#4-candidate-designs)
5. [Decisions](#5-decisions)
6. [Components](#6-components)
7. [Order and lanes](#7-order-and-lanes)
8. [Verification](#8-verification)
9. [Risks](#9-risks)
10. [Open questions](#10-open-questions)

## 1. Request

The operator wrote this on 2026-10-12: "Please provide a detailed explanation and design documentation regarding the design of our test cases and system to ensure efficiency, effectiveness, and architectural scalability, preventing code duplication. Additionally, please outline a plan for test efficiency and begin refactoring to optimize the test flow using Ultrathink."

During the same work, the operator also wrote: "Please leverage sonnet or haiku agent for building the test cases".

## 2. Facts

The research dossier has the evidence. The facts that set this plan:

| Fact | Research |
|---|---|
| The suite grew by 42% in 4 weeks, to 20,535 tests. | F1.3 |
| 55% of the local floor time runs tests that `test-integration` already ran. | F2.5 |
| A gate that reads the profile gives the same per-package numbers as a second run. | F2.6 |
| One test took 479 s because it read the 10 GB transcript store of the host. | F3.4 |
| Production reads the same 10 GB after each Claude dispatch. | F3.5 |
| A filter on the file time skips about 97% of those bytes with the same result. | F3.6 |
| `internal/core` passed the 10-minute default timeout under local load. | F3.7 |
| 9 skills-drift tests take 17 to 74 s each (296 s in all) and run one at a time. | F3.8 |
| 858 extra copies of equal helper bodies exist; 597 are in predicates. | F4.1, F4.2 |
| A package-variable seam makes its test serial. | F5.4, F5.5 |

## 3. Goals and non-goals

**Goals:**

- G1: the local floor runs each test one time.
- G2: no package of the integration run passes its wall budget of 180 s on CI.
- G3: no test reads the home of the host.
- G4: the count of duplicate helper bodies goes down only.
- G5: the design of a test case is written down, and each test lane uses it.

**Non-goals:**

- a change to the build tags or to the two axes of [go/docs/testing.md](../../go/docs/testing.md);
- a removal of tests (the 2026-09-14 protocol governs that);
- learned test selection for a gate;
- `testing/synctest` before the module baseline reaches Go 1.25;
- a rewrite of the 533 old cycle directories under `acs/`.

## 4. Candidate designs

| Design | What it does | Effect | Time saved | Cost | Risk | Score (of 20) |
|---|---|---|---|---|---|---|
| A. Raise limits | A longer timeout and more CI runners | 5 | 1 | 5 | 2 | 13 |
| B. Measure once, fix poles, ratchet duplicates | One instrumented run, judges that read, hermetic poles, parallel slow tests, kits and a ratchet | 5 | 5 | 3 | 4 | 17 |
| C. Learned selection for gates | Run only the tests that a model selects | 2 | 5 | 2 | 1 | 10 |
| D. Shard every run | Split the packages over more runners and local processes | 5 | 3 | 2 | 3 | 13 |

Each criterion has a score from 1 (bad) to 5 (good). Design B wins. Design A hides growth. Design C can miss a red, and a gate must not miss one. Design D does not help the local floor, and the local floor is the limit today. Design B keeps the option to add D later.

## 5. Decisions

| Id | Decision | Research |
|---|---|---|
| D1 | `make test-integration` is the only instrumented run of the runtime packages. | R1 |
| D2 | The run writes `coverage.partial.txt` and renames it to `coverage.txt` only when `go test` exits 0. It deletes the old `coverage.txt` first. | R2 |
| D3 | `evolve cover strict` reads `coverage.txt` and `go/.cover-strict`. It computes each percent through `covchanged.ParseProfile`. | R3 |
| D4 | `make cover-strict` calls `evolve cover strict`. It runs no test. | R3 |
| D5 | The local floor is: `lint`, `test-integration`, `test-e2e`, `test-acs-durable`, `cover-changed`, `cover-strict`, `apicover-check`. | R4 |
| D6 | `make apicover-enforce` stays for a scoped run (`APICOVER_PKGS`). The floor uses `apicover-check`. | R1 |
| D7 | `ScanConfigRoot` skips a transcript file whose modification time is more than 1 minute before the window start. | R5 |
| D8 | A test that builds a real bridge engine sets the Claude config root to a temporary directory. | R6 |
| D9 | The skills-drift integration tests call `t.Parallel`. Each other slow hermetic test does the same. | R7 |
| D10 | Each package has a wall budget of 180 s under `-race` on CI. `cmd/testlatency` reports each package over it. | R8 |
| D11 | The recipe of `test-integration` sets `-timeout 20m`. The budget, not the timeout, is the alarm. | R8 |
| D12 | A duplicate ratchet scans runtime test files. `go/.test-dup-baseline` holds the count. The count can only go down. | R9 |
| D13 | `pkg/acsassert` gets the six most-copied predicate helpers. The personas that write predicates name them. | R10 |
| D14 | Test lanes run on Sonnet. A mechanical lane runs on Haiku. Each lane uses names specific to its file. | R11 |
| D15 | A new seam is a field of a dependency struct, or a parameter. A package variable is the fallback. | R12 |
| D16 | Change-scoped iteration inside a lane is a shadow helper. It never replaces the floor. | R13 |

## 6. Components

Each component is one lane and one pull request. Each lane starts with a red test.

| Id | Component | Red test first | Lane model |
|---|---|---|---|
| T0 | These documents and the index entries | `evolve docs ste-lint --strict` on each file | console |
| T1 | The time filter in `ScanConfigRoot` (D7) and the hermetic subagent test (D8) | A transcript file older than the window is not read; a file inside the window still counts; `TestDefaultExecAdapter_DispatchesViaBridge` reads no host home | Sonnet |
| T2 | Measure once: the profile rename (D2), `evolve cover strict` (D3, D4), the floor (D5, D6), the CI step and `go/docs/testing.md` | A failed test leaves no `coverage.txt`; `evolve cover strict` refuses a package below its floor; its percent equals `go test -cover` on a sample package | Sonnet |
| T3 | Parallel skills-drift tests (D9) | The tests pass with `-race -count=3 -shuffle=on`, and the package wall time goes down | Sonnet |
| T4 | Wall budget and timeout (D10, D11) | `cmd/testlatency` flags a package over the budget in a recorded `go test -json` stream | Sonnet |
| T5 | The duplicate ratchet (D12) and the first standard-library replacements | The ratchet fails on a new copy in a fixture tree; the count goes down by the replaced copies | Haiku for the replacements, Sonnet for the ratchet |
| T6 | The predicate kit in `pkg/acsassert` (D13) and the persona text | Each new helper has a table test and a failure case | Sonnet |
| T7 | The seam order (D15) in the house rules and the lane brief | A review checklist item; no code | console |
| T8 | Change-scoped iteration (D16), in shadow | The selected set holds each importer of a changed package | Sonnet |
| T9 | The `internal/core` pole: the slowest 20 tests, with the remedies of design §7.3 | The package wall time on CI goes down, and the coverage of each touched function stays at 100% | Sonnet |

## 7. Order and lanes

1. T0 first. It is documents only.
2. T1 and T2 next. They give the largest gains: one pole of 479 s and 55% of the floor.
3. T3 and T4 after T2, because T4 reads the run that T2 defines.
4. T5 and T6 when a lane slot is free. They stop new duplicates.
5. T7 with T0 or T2. It is text only.
6. T8 and T9 last.

At most four lanes run at one time. The coverage backfill lanes (E1 to E4, F1 to F4) share the four slots. T1 and T2 come before F1 to F4, because each later floor is faster after T1 and T2.

## 8. Verification

| Measure | Before (2026-10-12) | After T1 and T2 |
|---|---|---|
| Local floor time, lane D shape | 23.4 min | the sum of `lint`, `test-integration`, `test-e2e`, `test-acs-durable` and about 10 s of judges |
| `internal/subagent` wall time | 295 to 485 s | under 10 s |
| Bytes read for one dispatch window of 60 min | 10 GB | about 250 MB |
| `cover-strict` CI step | 51 package runs | one read of `coverage.txt` |

Each lane reports its before and after numbers from a `go test -json` stream. The console runs one full floor after each merge of T1 and T2 and records the time.

## 9. Risks

| Risk | Control |
|---|---|
| A judge passes on a profile from a failed run. | D2: no green run, no profile. The 2026-09-14 S1 meta-tests stay. |
| The percent from the profile differs from `go test -cover`. | The T2 equality test on a sample package. |
| The time filter skips a transcript that counts. | D7 uses 1 minute of slack. A T1 test writes a line inside the window into a file with an old time stamp and checks the documented limit. |
| Parallel tests expose shared state. | T3 runs `-race -count=3 -shuffle=on` before the merge. |
| A ratchet blocks a valid copy. | The baseline is a file in the repository. A reviewed change can raise it, with the reason in the commit. |

## 10. Open questions

1. When does the module baseline move to Go 1.25? This decision opens `testing/synctest` (design §7.4). It is not part of this plan.
2. Does a CI shard (design D) become necessary after T9? Measure after T9.
