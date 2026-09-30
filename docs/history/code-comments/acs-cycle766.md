# Comment history: `acs/cycle766`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle766/predicates_test.go:3` — above `package cycle766`

```text
// Package cycle766 materializes the cycle-766 acceptance criteria for the sole
// committed top_n task todo-fleet-lane-provisioning-split (triage-report.md
// ## top_n; scout's two leak-recovery proposals were DEFERRED by triage as
// out-of-scope for this fleet lane, so per R9.3 no predicates bind to them).
//
// Task source: inbox id fleet-lane-provisioning-split (weight 0.9, cycle-640
// incident): fleet provisioning never pinned ONE lane identity for the whole
// run — scout scouted lane A's goal while triage was handed lane B's
// fleet_scope, enabling the builder-task-binding failure class. Fix contract:
// pin the lane's scope (todo ids + goal hash) to <workspace>/lane-scope.json
// before any phase runs, inject THAT scope into every phase, and fail the
// scout→triage transition on a goal-hash mismatch.
//
// AC map (1:1), from the inbox item's acceptance[] list:
//
//	AC1 concurrent lanes each see ONLY their own scope, sourced from
//	    lane-scope.json → C766_001 (file injects to all phases),
//	    C766_002 (two lanes + cross-lane env drift; file must win),
//	    C766_003 (absent file keeps legacy env fallback),
//	    C766_004 (env-scoped run materializes lane-scope.json pre-phase)
//	AC2 scout-report goal-hash mismatch vs lane-scope.json fails the
//	    scout→triage transition with an explicit abort reason
//	    → C766_005 (mismatch aborts, triage never runs),
//	      C766_006 (match proceeds — no blanket abort),
//	      C766_007 (goal_hash-less report fails OPEN — no false aborts,
//	      the cycle-760..762 destruction class)
//	AC3 go test -race PASS on touched packages → every predicate runs the
//	    unit contract under -race (apicover runs in the repo-wide gate)
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// unit contract in internal/core, which EXERCISES the orchestrator through
// full RunCycle drives with recording fake runners and real workspace files —
// behavioral via subprocess, no source-grep predicates (cycle-85 rule). The
// `-v` + "--- PASS:" guard rejects a rename/no-tests-matched silent green.
// Adversarial axes embedded in the unit contract: negative (env drift must
// NOT leak through; mismatch must NOT proceed; match/keyless must NOT abort),
// edge (absent lane-scope.json, missing goal_hash key), semantic (pin vs
// inject vs coherence-gate are separate behaviors).
```

### `go/acs/cycle766/predicates_test.go:75` — above `func TestC766_002_two_lanes_see_only_own_scope(t *testing.T) {`

```text
// AC1 negative: two lanes with distinct lane-scope.json each see ONLY their
// own scope even when the env snapshot carries the OTHER lane's scope — the
// exact cycle-640 cross-lane drift must lose to the pinned file.
```

### `go/acs/cycle766/predicates_test.go:107` — above `func TestC766_007_goal_hash_absent_fails_open(t *testing.T) {`

```text
// AC2 fail-open edge: a scout report WITHOUT a goal_hash key proceeds — an
// over-strict gate would recreate the cycle-760..762 false-abort class.
```
