# Comment history: `acs/cycle550`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle550/predicates_test.go:3` — above `package cycle550`

```text
// Package cycle550 materialises the cycle-550 acceptance criteria for this
// fleet lane's sole `## top_n` task per triage-report.md:
//
//	supervisor-continuous-lane-keeping (L5, "the ceiling-keeper" of the
//	fleet-width architecture) — convert the wave-synchronized scheduler into
//	a rolling lane pool: the supervisor maintains target width =
//	fleet.count, and on any lane exit (PASS or FAIL) immediately selects and
//	dispatches the next disjoint pending task as a replacement lane instead
//	of waiting for the wave barrier. Config-gated via
//	`policy.fleet.scheduling: pool|wave` (default "wave" preserves today's
//	behavior byte-identically).
//
// Per triage-report.md's "Fleet scope note", the scout-proposed
// eliminate-sequential-fallback-min-width-lane, memo-phase-routing-restore,
// and fuzz-parser-surfaces tasks are OUT OF SCOPE for this lane (assigned to
// sibling concurrent cycles) and are NOT predicated here (AC-Materialization
// Contract R9.3: predicates bind ONLY to triage-committed `## top_n` work).
//
// Predicate strategy (mirrors cycle547/549): BEHAVIORAL predicates drive the
// system under test through its in-package RED tests via subprocess
// `go test`, asserting a non-degenerate pass (requireTestsRan closes the
// cycle-85 "no tests to run" trap) — never a source grep. The in-package
// tests were authored by the TDD engineer this cycle:
//
//	internal/fleet/pool_test.go                       (new RunPool contract)
//	internal/policy/fleet_config_scheduling_test.go   (new scheduling knob)
//
// RED today: both packages fail to BUILD (RunPool/PoolConfig/PoolTransition
// and FleetPolicy.Scheduling/FleetConfig.Scheduling are all undefined) — a
// subprocess `go test` on either package exits non-zero with a compile
// error, which is exactly what every predicate below asserts against. The
// Builder implements production code ONLY (the seams named in those files
// and their header doc-comments); it must not modify the tests.
```
