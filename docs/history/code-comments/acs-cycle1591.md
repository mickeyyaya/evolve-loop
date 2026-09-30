# Comment history: `acs/cycle1591`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1591/predicates_test.go:3` — above `package cycle1591`

```text
// Package cycle1591 materializes the cycle-1591 acceptance criteria for the two
// committed tasks of this fleet lane (scout-report.md ## Selected Tasks;
// triage-report.md ## top_n): `retire-stale-retro-prompt-delivery-stall` and
// `retro-delivery-format-binding`. Per R9.3 no predicate binds to a deferred or
// dropped item (`todo-retire-vacuous-retro-prompt-delivery-stall-eval` is
// dropped and gets ZERO predicates here).
//
// The defect: the live inbox record
// `.evolve/inbox/2026-08-18T02-30-00Z-retro-prompt-delivery-stall.json` keeps
// being "retired" by a filesystem-only removal (a delete, or a move into the
// .gitignore'd `.evolve/inbox/processed/`) with no matching `git rm`, so it stays
// in the Git INDEX and the next fresh checkout brings it back live. The
// build-floor gate that exists to catch a false removal claim,
// core.RemovalClaimFailures, asks only the worktree filesystem
// (build_removal_check.go:60-62), so the false retirement passes the floor. The
// second task pins the producer→consumer format binding the record's own
// incident depends on: the bridge's classified `submit_wedged` cause must
// survive into core.DeliveryFailureCause, which is what retro.go:203 keys its
// single relaunch on.
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	AC1 tracked-but-absent claim → exactly one failure          → C1591_001
//	AC2 NEGATIVE: untracked-absent honest, non-repo fails open   → C1591_002
//	AC3 the live inbox record is retired as a TRACKED deletion   → C1591_003
//	AC4 eval `retire-stale-retro-prompt-delivery-stall` is rigorous → C1591_004
//	AC5 real bridge submit_wedged error is classified by the
//	    consumer's own classifier (format binding)               → C1591_005
//	AC6 NEGATIVE: a real generic silence timeout is NOT classified → C1591_006
//	AC7 retro's relaunch consumer stays green on that classifier → C1591_007
//	AC8 eval `retro-delivery-format-binding` is rigorous         → C1591_008
//	AC9 dispositions/provenance preserved in build evidence      → manual+checklist
//	AC10 repo-wide `go test -count=1 ./...` green                → manual+checklist
//	     (a /... sweep is a banned flaky-predicate shape; CI + the ship gate own it)
//
// Adversarial axes: negative (C1591_002 — an index-aware check must not start
// failing every absent path, and must fail open outside a repo; C1591_006 — the
// classifier must not over-fire on ordinary silence), edge (C1591_003 asserts
// BOTH index absence and disk absence — either alone is the exact half-retirement
// that caused the recurrence), semantic (index truth, record lifecycle, producer
// format, consumer relaunch and eval rigor are distinct behaviors).
//
// No source-grep predicates (cycle-85 rule): C1591_001/002/005/006/007 execute
// the real production code as subprocess `go test` runs and require a NAMED
// `--- PASS:` marker (a bare exit 0 would hide a renamed, skipped, or non-existent
// test); C1591_003 interrogates the real Git index and filesystem; C1591_004/008
// run the SSOT eval-quality checker. Every `go test` invocation names ONE package
// and is -run-narrowed (flaky-predicate-shape rule: no `/...` sweeps).
```
