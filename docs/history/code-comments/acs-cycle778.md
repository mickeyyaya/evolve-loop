# Comment history: `acs/cycle778`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle778/predicates_test.go:3` — above `package cycle778`

```text
// Package cycle778 materializes the cycle-778 acceptance criteria for the sole
// committed top_n task ship-window-lease (triage-report.md ## top_n; this
// lane's fleet_scope assigns exactly that id, so per R9.3 no predicates bind
// to the scout report's other-lane findings).
//
// Task source: inbox id ship-window-lease (weight 0.97, operator-boosted
// 2026-07-13, campaign tokenopt-2026-07). Measured cycles 767-774: audit ran
// ~10x for 8 cycles — AUDIT_BINDING_HEAD_MOVED re-audits from siblings landing
// on main between a lane's audit-binding snapshot and its push. The fix is a
// ship-window lease (go/internal/shipwindow) serializing ONLY the
// binding-snapshot→push section, with TTL + holder-death recovery (run-lease
// liveness pattern) and FIFO fairness.
//
// AC map (1:1), from the inbox item's acceptance[] list:
//
//	AC1 sibling waits instead of re-auditing (two lanes, one main HEAD,
//	    zero AUDIT_BINDING_HEAD_MOVED)
//	    → C778_001 (mutual exclusion + zero head-moved + both lanes ship),
//	      C778_002 (NEGATIVE: a fresh live-holder lease blocks a sibling
//	      until its ctx expires — the anti-no-op predicate).
//	AC2 holder death recovered: stale lease broken (dead pid before TTL;
//	    TTL expiry despite live pid) → C778_003.
//	AC3 FIFO fairness among queued waiters → C778_004.
//	AC4 batch soak (audit runs ≈ cycle count) + go test -race + apicover
//	    → soak is manual+checklist (test-report.md, addressed to Auditor);
//	      -race is exercised by every predicate below; apicover runs in the
//	      repo-wide gate. C778_005 pins the on-disk lease path contract.
//
// Each predicate shells `go test -race -count=1 -v -run '^<name>$'` over the
// shipwindow unit contract, which EXERCISES Acquire/Release behaviorally — no
// source-grep predicates (cycle-85 rule). The `-v` + "--- PASS:" guard rejects
// a rename/no-tests-matched silent green. Adversarial axes: negative (held
// lease must BLOCK, not yield), edge (dead-pid-within-TTL and
// TTL-expired-live-pid boundary breaks), semantic (exclusion vs recovery vs
// fairness vs path are separate behaviors).
```
