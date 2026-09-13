# ADR-0103: Component breakdown program — small units, one contract each

- **Status:** Accepted (operator direction, 2026-09-13)
- **Related:** [ADR-0101](0101-signal-center.md) (the Signal Center every unit reports through),
  [signal-center-design.md §12](../signal-center-design.md) (the decomposition order),
  [ADR-0044](0044-unified-phase-recovery-protocol.md) (the C1 recording chokepoint the first unit extracts)

## Context

The orchestrator's largest files (`orchestrator.go` 1169 lines, `phase_advisor.go` 1144,
`failure_learning.go` 1068, `cmd_cycle.go` 1058, `inboxmover.go` 993, `gitops.go` 989 …) each
hold several unrelated responsibilities behind package-internal names. When a cycle misbehaves,
the reader has to know which of a file's clusters produced a line, and the lines are hand-written
`[orchestrator] WARN …` strings with no code and no owner. The Signal Center (ADR-0101) gave the
pipeline one stream and one severity contract; this program gives each responsibility its own
small unit that reports into that stream under its own module tag and its own codes, so a root
cause is found by reading a few signal lines and opening one small package.

## Decision

Break the large modules into small, independent units in the order of design §12, one unit per
PR, each landing with all of the following — a unit without any of them is not done:

1. **A unit design document** at `docs/architecture/decomposition/NN-<unit>.md` in the fixed
   structure: Context · Boundary (what moves, what stays, why) · API (Go signatures) · Patterns
   (named, with the force each answers) · Module contract (module tag, error codes with reasons,
   log/signal lines with origin) · Tests (the red-first list; API and line coverage) · Why it is
   better than the original (a table: observability, isolation, changeability, measurement) ·
   Risks and the byte-identity guarantees · Migration (the facade the callers keep).
2. **One module tag** in the Signal Center's closed set, and **unified error codes**
   (`MODULE_SNAKE_CASE`, registered with a doc string, projected into `signal-codes.md`) for every
   condition the unit reports; the unit emits signals with its module and an `origin` naming the
   type and method, never a hand-written stderr line.
3. **TDD, red first**: every exported symbol has a test that names it (apicover) and the unit's
   line coverage is 100 % (`.cover-strict`); unreachable defensive branches are removed rather
   than excused; behaviour is pinned with build-confirmed mutants.
4. **Callers untouched** (Strangler Fig): the orchestrator keeps a one-call facade at the old
   seam, so the extraction changes no call site and no on-disk format; a characterization test
   proves the files written are byte-identical.
5. **Clean-code limits**: functions under 50 lines, files under 800, nesting ≤ 4; interfaces at
   the point of use; explicit dependency injection at construction (functional options).
6. **The reviewer fleet** (simplifier → architecture ∥ Go review; CRITICAL/HIGH block) and the
   commit gate, like every other change.

## Consequences

- Triage becomes a read of `signals.ndjson` filtered by module, then one small package: the
  ultimate goal the operator set (root cause within a few review steps).
- Each unit is measurable on its own: coverage, API coverage, signal counts by code.
- The orchestrator shrinks to composition plus the `RunCycle` engine (unit 3), with the units
  injected — the same explicit-DI shape the Signal Center campaign established.
- Cost: one more package and one facade per unit; the facade is the price of untouched callers
  and is removed only when the last caller migrates to the unit's API.

## Units (design §12 order)

| # | Unit | From | Doc |
|---|---|---|---|
| 01 | Phase-outcome recorder (the C1 chokepoint: cycle result, phase-timing log, usage sidecar, context fill) | `core/failure_learning.go` | [01-outcome-recorder.md](../decomposition/01-outcome-recorder.md) |
| 02 | Failure diagnostics and delivery-failure classification (the failure-diag sidecar writer, the delivery-failure classifier, the wire tokens) | `core/failure_learning.go` | [02-failure-diagnostics.md](../decomposition/02-failure-diagnostics.md) |
| 03 | Carryover-todo lifecycle (mint admission, the closeout merges, the retirements, the state.json persist; three sibling files collapsed) | `core/failure_learning.go` + `carryover_merge.go`, `prescription_carryover.go`, `carryover_triage_retire.go` | [03-carryover-lifecycle.md](../decomposition/03-carryover-lifecycle.md) |
| 03b | Failure-learning engine (the failed-approach recorder, the deterministic floor, remediation filing, the recurrence closure; `recordFailureLearning` split in place into a pure gate + four named steps for unit 05) | `core/failure_learning.go` | [03b-failure-learning-engine.md](../decomposition/03b-failure-learning-engine.md) |
| 04 | Phase advisor | `core/phase_advisor.go` | next |
| 05 | Orchestrator: composition root vs `RunCycle` engine | `core/orchestrator.go` | after 01–04 |
| 07 | Ship landing (the fleet ff-merge, the push with its inline push-race repair and its reclassification, the post-push head read, the shared git probes, the ship-binding writer; the rebase engine stays with the orchestrator — unit 05; staging → 07b, run-scope → 07c) | `phases/ship/worktree_ship.go` + `repair.go` + `gitops.go` + `pushonly.go` + `verify.go` | [07-shipgitops.md](../decomposition/07-shipgitops.md) |
| … | inbox mover, config, bridge engine, audit gates | design §12 | later |

Row 03b is a numbering insertion (the engine surfaced while unit 03 was designed), not a §12 reorder: it lands before unit 04 because the advisor's `truncateRunes` read now resolves through carryover, and its `ORCHESTRATOR_*` handoff (its doc §5) must be settled before unit 05 splits `RunCycle`. Row 07 is likewise an insertion while 04–06 are unassigned: the ship landing is design §12 row 6 and touches no core file, so it could be built in parallel with the advisor and the orchestrator split; its number is the brief's, not a §12 reorder.
