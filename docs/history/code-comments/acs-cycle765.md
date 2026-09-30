# Comment history: `acs/cycle765`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle765/predicates_test.go:3` — above `package cycle765`

```text
// Package cycle765 materializes the cycle-765 acceptance criteria for the sole
// committed top_n task width-scaled-binding-retry (triage-report.md ## top_n;
// the scout's two proposals were DROPPED by triage as out-of-scope for this
// fleet lane, so per R9.3 no predicates bind to them).
//
// Task source: inbox id width-scaled-binding-retry (weight 0.93, cycle-759
// incident): ship failed AUDIT_BINDING_HEAD_MOVED because a sibling landed
// during the audit→ship gap, and a FIXED recovery budget of 2 aborted a clean
// cycle — with N lanes racing one main the budget must scale max(2, width+1)
// for contention-class codes, with jittered backoff so siblings don't
// re-collide in lockstep, while non-contention transients keep the constant
// budget.
//
// AC map (1:1), from the inbox item's acceptance[] list:
//
//	AC1 contention budget scales with fleet width      → C765_001 (+ C765_004
//	    pinning GIT_FLEET_REBASE_NEEDED via the pure classifier)
//	AC2 jittered backoff between re-audits             → C765_002
//	AC3 non-contention transients keep constant budget → C765_003
//	AC4 go test -race PASS                             → every predicate runs
//	    the unit contract under -race (apicover runs in the repo-wide gate)
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// unit contract in internal/core, which EXERCISES the orchestrator's ship
// recovery through full RunCycle drives against a persistently-failing ship
// runner — behavioral via subprocess, no source-grep predicates (cycle-85
// rule). The `-v` + "--- PASS:" guard rejects a rename/no-tests-matched
// silent green. The unit contract embeds the adversarial axes: negative
// (transient must NOT scale; garbage/negative width must NOT scale), edge
// (width absent/1/0), semantic (budget scaling vs jitter vs classification
// are separate behaviors).
```
