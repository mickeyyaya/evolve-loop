# Comment history: `acs/cycle1666`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1666/predicates_test.go:3` — above `package cycle1666`

```text
// Package cycle1666 materializes the acceptance criteria of the ONE inbox item
// this fleet lane committed (lane-scope.json todo_ids ∩ triage-report.md
// ## top_n) — and nothing else (R9.3):
//
//	dossier-corpus-carries-retro-mislabel  (medium, weight 0.84, defect)
//
// The lane's second scoped id, lost-ship-dossier-evidence, was triage-DROPPED
// (stale-consumed: its only record is .evolve/inbox/consumed/…, shipped
// 2026-09-13) and gets ZERO predicates here.
//
// The defect. PR #389 made the retro-skip mislabel fix FORWARD-ONLY: a
// pre-fix dossier whose `skipped_phases:[{phase:retro,reason:FAIL}]` means
// "retro RAN and its verdict was declined" is byte-for-byte the same shape as
// a post-fix dossier whose identical entry means "retro did not run". 134
// committed records (cycles 823-1217) carry that mislabel; the ledger holds a
// {role:retro, kind:agent_subprocess} receipt for every one of them, while
// their run dirs are gone. The remedy chosen is the inbox record's option
// (b): a `schema_version` discriminator on every new record, and a corpus
// seam that refuses to read a legacy entry as a skip — never the backfill.
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	AC1 a count of affected dossiers, derived not estimated, with the
//	    artifact cross-check that proves each one's retro really ran     → 001
//	AC2 the discriminator (not the backfill), choice justified in the
//	    commit body — the corpus is left untouched                        → 002
//	AC3 TestSchema_NoDrift stays green: the field on BOTH sides           → 003
//	AC4 a consumer-side test proving a pre-fix record is not treated as
//	    evidence retro was skipped                                       → 004
//
// Adversarial axes (skills/adversarial-testing §6). NEGATIVE: 002's
// no-rewrite pin over the real corpus (a backfill greens 001 and 004 and
// fails this), the absent-corpus exit in 001's binding, the legacy
// never-trusted rows in 004's binding. EDGE/OOD: wrong-cycle receipt, corrupt
// ledger line, empty run dir, nil record, empty phase name. SEMANTIC: derived
// count (001), forward-only stamp + untouched history (002), schema lockstep
// (003), consumer degrade (004) — four distinct behaviours.
//
// Flaky-shape contract: ONE named package per invocation, always -run
// narrowed (cmd/evolve is a known-slow suite), no wall-clock bounds, no
// literal PIDs, every git call is -C anchored.
//
// Reachability probe (cycle-644 rule): this package imports only
// pkg/acsassert and the standard library — a leaf. The frozen bindings are
// in-package (internal/dossier) or in package main (cmd/evolve, which already
// imports internal/dossier via cmd_dossier.go) — no new import edge is pinned.
```

### `go/acs/cycle1666/predicates_test.go:72` — above `legacyRetroSkipFloor = 134`

```text
// legacyRetroSkipFloor is the derived (not estimated) size of the affected
// legacy set at RED time: 134 unversioned records carrying a retro
// skipped_phases entry, cycles 823-1217, every one with a ledger receipt.
// Legacy history is frozen — the count can only grow (an unversioned record
// written by a pre-fix binary during this batch) — so fewer means the
// corpus was rewritten.
```

### `go/acs/cycle1666/predicates_test.go:81` — above `var (`

```text
// ---------------------------------------------------------------------------
// Harness: the real CLI, built once (the cycle-1648/1659 TestMain shape).
// ---------------------------------------------------------------------------
```

### `go/acs/cycle1666/predicates_test.go:384` — above `func TestC1666_003_SchemaLockstepAndGoldensChangeOnlyByTheStamp(t *testing.T) {`

```text
// TestC1666_003_SchemaLockstepAndGoldensChangeOnlyByTheStamp — AC3. The
// schema drift guard is bidirectional, so the field must land on BOTH sides
// (TestSchema_NoDrift + the by-name integer/not-required pin). The stamp
// changes every produced record's bytes, so the two producer goldens the
// cycle-1663 floor froze MUST be regenerated — legitimately, and ONLY by the
// added `schema_version` line: each golden minus that key must equal the
// lane-base golden minus that key, and the core byte-pins must print PASS.
```
