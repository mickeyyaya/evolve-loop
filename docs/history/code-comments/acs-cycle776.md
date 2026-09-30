# Comment history: `acs/cycle776`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle776/predicates_test.go:3` — above `package cycle776`

```text
// Package cycle776 materializes the cycle-776 acceptance criteria for the sole
// committed top_n task fleet-lane-provisioning-split (triage-report.md
// ## top_n; scout's three unrelated proposals were DEFERRED by triage as
// out-of-lane-scope, so per R9.3 no predicates bind to them).
//
// Task source: inbox id fleet-lane-provisioning-split (weight 0.9, cycle-640
// incident). Cycle-766 landed the pin (lane-scope.json → Context["fleet_scope"]
// for every phase) and the scout→triage goal-hash coherence gate. The residual
// slice — proven live by THIS run, whose scout prompt carried no lane scope and
// whose scout consequently scouted three out-of-scope tasks — is the PROMPT
// layer: only triage renders the scope into what the LLM actually reads.
//
// AC map (1:1), from the inbox item's acceptance[] list:
//
//	AC1 concurrent lanes each see ONLY their own scope in scout/triage/build
//	    prompts (fixture asserts injected scope matches lane-scope.json)
//	    → C776_001 (scout renders), C776_002 (scout two-lane isolation,
//	      negative), C776_003 (scout typed-envelope source),
//	      C776_004 (scout unscoped edge — no over-render),
//	      C776_005 (build renders + foreign-id negative),
//	      C776_006 (build unscoped edge),
//	      C776_007 (tdd renders + foreign-id negative),
//	      C776_008 (tdd unscoped edge).
//	    Triage rendering is pre-existing GREEN (triage.go +
//	    triage_phaseio_test.go); Context-level injection from lane-scope.json
//	    is pre-existing GREEN (cycle-766, core/lanescope_pin_test.go).
//	AC2 scout-report goal-hash mismatch vs lane-scope.json fails the
//	    scout→triage transition with abort_reason (no silent proceed)
//	    → gate itself pre-existing GREEN, re-bound as regression by
//	      C776_009; the NEW teeth are C776_010 (lane-scoped scout prompt
//	      must instruct the Decision Trace goal_hash echo — without it the
//	      gate fails open forever, exactly what happened this run).
//	AC3 go test -race PASS on touched packages; apicover clean → every
//	    predicate runs its unit contract under -race (apicover runs in the
//	    repo-wide gate).
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// unit contract, which EXERCISES ComposePrompt / RunCycle behaviorally — no
// source-grep predicates (cycle-85 rule). The `-v` + "--- PASS:" guard rejects
// a rename/no-tests-matched silent green. Adversarial axes: negative
// (foreign lane ids must NOT appear; mismatch must NOT proceed), edge
// (unscoped prompt must NOT over-render), semantic (render vs isolation vs
// gate-teeth are separate behaviors).
```

### `go/acs/cycle776/predicates_test.go:85` — above `func TestC776_002_scout_prompt_two_lanes_only_own_scope(t *testing.T) {`

```text
// AC1 negative: two lanes each render ONLY their own ids in the scout prompt —
// the other lane's id must be absent (cycle-640 cross-lane drift).
```

### `go/acs/cycle776/predicates_test.go:122` — above `func TestC776_009_goal_hash_mismatch_gate_regression(t *testing.T) {`

```text
// AC2 regression re-bind (pre-existing GREEN, cycle-766): scout-report
// goal_hash ≠ lane-scope.json goal_hash aborts before triage with an explicit
// mismatch reason — no silent proceed.
```
