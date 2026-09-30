# Comment history: `acs/cycle523`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle523/predicates_test.go:3` — above `package cycle523`

```text
// Package cycle523 materialises the cycle-523 acceptance criteria for the
// SINGLE triage-committed task (this lane's assigned fleet_scope id):
//
//	wave-seed-partitions-on-id-not-real-files — populate fleet.Todo.Files from
//	each triage-decision top_n[] card's declared files[] in
//	fleet.PlanFromTriage (go/internal/fleet/triageplan.go, today
//	Todo{ID: id, Files: []string{id}}), falling back to []string{id} only when
//	a card declares no files, so overlapping declared files collapse to ONE
//	partition lane while disjoint files still spread to `count` lanes.
//	→ C523_001..007
//
// TASK BINDING (cycle-522 lesson): cycle 522 FAILed because TDD bound to
// scout-report's broader "Task 2" while Builder bound to triage's narrower
// committed top_n. This file binds to the triage-report `## top_n` id
// (`wave-seed-partitions-on-id-not-real-files`) ONLY — NOT scout's bundled
// treediff-guard task nor the 0.94 cmd_loop_wave.go half, both of which
// triage-report `## deferred` to sibling lanes/future cycles. Predicates bind
// only to triage-committed work (R9.3).
//
// 1:1 AC-materialization (see the eval
// .evolve/evals/wave-seed-partitions-on-id-not-real-files.md): 7 predicates +
// 0 manual+checklist + 0 removed = 7 ACs total, none double-counted.
//
// Why these predicates exercise the SUT directly (cycle-85 predicate-quality):
// every load-bearing predicate CALLS fleet.PlanFromTriage in-process with a
// crafted triage-decision.json and asserts on the returned []CycleSpec lane
// grouping — never a "source file contains text X" check. The id-as-file
// placeholder the fix removes means today's code groups lanes by unique id,
// so an overlap-collapse assertion is RED now and GREEN only once real
// declared files[] are threaded through.
//
// RED strategy (verified in test-report.md "RED Run Output"): the package
// COMPILES against the current PlanFromTriage signature (adding a `files` JSON
// key to top_n cards is not a signature change; json.Unmarshal ignores the
// unknown field today), so C523_001 and C523_005 are RED on their ASSERTIONS
// — two cards sharing a declared file still land in SEPARATE lanes because the
// current code keys partitioning on the id-as-file placeholder, not the file.
// C523_002/003/004 are pre-existing-GREEN behavior-preservation pins (the fix
// must NOT break disjoint-spread, the no-files id fallback, or count<2
// single-lane collapse). C523_006/007 are repo-gate pins (fleet suite +
// vet stay green through the change).
//
// Adversarial diversity (skills/adversarial-testing SKILL §6):
//
//	Negative:   C523_005 — a disjoint card must NEVER be swept into the
//	            overlap lane, and an overlapping pair must NEVER be split; the
//	            single assertion kills BOTH the always-spread no-op (id-as-file
//	            → alpha/beta split) and the always-collapse fake (everything in
//	            one lane). C523_004 — count<2 must NOT fabricate extra lanes.
//	Edge / OOD: C523_003 — cards declaring NO files[] fall back to the id
//	            island (unknown-footprint boundary); C523_004 — count=1.
//	Semantic:   C523_001 (overlap → 1 lane) vs C523_002 (disjoint → 2 lanes)
//	            are DISTINCT behaviors driven only by the declared files — a
//	            fake that always returns the same lane count passes one and
//	            fails the other.
```
