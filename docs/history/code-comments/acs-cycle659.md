# Comment history: `acs/cycle659`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle659/predicates_test.go:3` — above `package cycle659`

```text
// Package cycle659 materialises the cycle-659 acceptance criteria for the single
// triage-committed (`## top_n`) task: statefile-rmw-flock-single-source.
//
// TASK BINDING (R9.3 — predicates bind ONLY to triage `## top_n` work):
//
//	triage-report.md commits exactly ONE task to this cycle:
//	  statefile-rmw-flock-single-source (H) — C659_001..005
//	Every `## deferred` item (difficulty-conditioned-phase-budgets,
//	token-telemetry-s6/s7/s8, tokenopt-*, boot-orphan-sweep, …) gets ZERO
//	predicates here.
//
// FEATURE CONTEXT
//
//	state.json's full-fidelity read-modify-write is implemented THREE times:
//	  (A) internal/phases/ship/statefile.go  readStateMap/writeStateMap  (flocked)
//	  (B) internal/core/reset.go             readJSONMapFile/writeJSONMapFileAtomic
//	  (C) internal/phaseintegrity/repin.go   inline unmarshal→mutate→rename
//	This cycle consolidates all three into ONE leaf package
//	internal/adapters/statemap (ReadStateMap + UpdateStateMap, importing only
//	internal/adapters/flock + stdlib — the verified-acyclic home from cycle-644's
//	retrospective) and deletes the duplicates, so exactly one lock-owning RMW
//	implementation exists. statemap is graduated into go/.apicover-enforce as a
//	new-package obligation (3rd-recurrence class: 640 stateutil, 644 import
//	cycle, 653 tree hygiene).
//
// PREDICATE QUALITY (cycle-85): every load-bearing predicate EXERCISES the SUT —
// it runs `go test -race`/`go vet`/`go build` against the real packages and
// asserts on the actual exit code, never a bare "source file contains text X"
// grep. The single-source pin (C659_002) additionally greps for the ABSENCE of
// the deleted duplicates (unforgeable — a magic string cannot re-satisfy it),
// but its load-bearing assertion is that the consolidated package builds.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive  : C659_001 statemap package tests pass under -race (primitive
//     works: round-trip + serialization).
//   - Negative  : C659_002 the duplicate RMW func definitions are GONE from
//     reset.go (an implementation that leaves them in place FAILS).
//   - Semantic  : C659_003 seal + repin + ship callers route through statemap.
//   - Hygiene   : C659_004 touched packages vet clean under the consolidation.
//   - Config    : C659_005 statemap graduated into .apicover-enforce.
```

### `go/acs/cycle659/predicates_test.go:102` — above `if acsassert.FileContainsAny(resetGo, dup) {`

```text
// ABSENCE check: use the non-failing FileContainsAny probe. FileContains(t,…)
// t.Errorf's when the substring is missing, so it cannot express "must be
// absent" (it would fail in the GREEN state) — corrected in build (cycle-659).
```

### `go/acs/cycle659/predicates_test.go:116` — above `func TestC659_003_AllWritersRouteThroughStatemap(t *testing.T) {`

```text
// TestC659_003_AllWritersRouteThroughStatemap is the semantic axis (AC2): the
// three former duplicate sites — ship (statefile.go), repin (repin.go), and seal
// (reset.go) — must all reference the single statemap source, and the grep
// target MUST be `statemap.` (never `storage.` — routing core through storage
// closes a forbidden core→storage import cycle, the cycle-644 failure). The
// load-bearing anchor is that the three caller packages BUILD together with the
// new dependency; the grep pins that each names statemap.
```

### `go/acs/cycle659/predicates_test.go:143` — above `if rel == "go/internal/core/reset.go" && acsassert.FileContainsAny(f, "storage.UpdateStateMap(") {`

```text
// Guard the exact cycle-644 regression: core must NOT be re-pointed at
// storage.UpdateStateMap( (import cycle). Only reset.go is core; the grep
// is a targeted anti-pattern check on that file.
// ABSENCE check: non-failing FileContainsAny (see C659_002 note).
```
