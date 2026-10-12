# ADR-0131: One test run feeds every coverage gate, and each test helper has one home

- **Status:** Accepted (2026-10-12). T0 is these documents. T1 to T9 are the slices of the [plan](../../plans/test-system-efficiency-2026-10.md).
- **Plan:** [test-system-efficiency-2026-10.md](../../plans/test-system-efficiency-2026-10.md).
- **Research:** [test-system-efficiency-2026-10.md](../../research/test-system-efficiency-2026-10.md).
- **Design:** [test-system.md](../test-system.md).
- **Relates to:**
  - [ADR-0101](0101-signal-center.md) §11: the strict package floors stay. Only their measurement changes;
  - [ADR-0111](0111-code-carries-no-comments.md): tests have no comments;
  - the [2026-09-14 test refactoring design](../test-refactoring-design-2026-09-14.md): its preservation protocol governs each removal or merge of a test.
- **Evidence:**
  - 55% of the local floor time ran tests a second or third time (research F2.5).
  - One test read the 10 GB transcript store of the host and took 479 s (F3.4). Production reads the same store after each Claude dispatch (F3.5).
  - `internal/core` passed the 10-minute default timeout of `go test` under load (F3.7).
  - The suite has 858 extra copies of equal helper bodies (F4.1).

## Context

The suite grew by 42% in 4 weeks, to 20,535 tests. The rule "each changed function at 100%" and the backfill lanes add tests fast. Each coverage gate ran its own tests. No gate read the numbers of another run. Helpers were copied into each package and each cycle.

## Decision

1. **One instrumented run.** `make test-integration` is the only run that measures coverage of the runtime packages.
2. **A profile only from a green run.** The run writes `coverage.txt` only when every test passes.
3. **Gates read.** `cover-changed`, `cover-strict` and `apicover-check` read `coverage.txt`. They run no tests.
4. **One parser.** The Go gates parse the profile through `covchanged.ParseProfile`.
5. **Hermetic home.** A test never reads the home directory of the host.
6. **Poles first.** Each package has a wall budget of 180 s under `-race` on CI. A report shows each package over it.
7. **One home for each helper.** A helper that two packages need goes to the kit of its domain.
8. **A duplicate ratchet.** The count of equal helper bodies in runtime test files can only go down.
9. **Seams keep tests parallel.** A new seam is a field or a parameter first. A package variable is the fallback.

## Consequences

- The local floor drops `make test` and runs each test one time.
- A gate with no profile, or with a stale profile, refuses.
- The transcript scan skips files that changed before the dispatch started. Production and tests both get faster.
- A new copy of a helper fails the structure test. The author moves it to a kit.
- Predicate authors get the helpers they copied before, in `pkg/acsassert`.

## Rejected

- **A longer timeout only.** It hides the growth of the suite.
- **Learned test selection for gates.** A gate must not miss a red.
- **A shard of every run.** It does not shorten the local floor, which is the limit today. The plan keeps it as a later option.
- **`testing/synctest` now.** The module baseline is Go 1.23, and CI has a Go 1.23 lane.
