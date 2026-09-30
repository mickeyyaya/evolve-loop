# Comment history: `acs/cycle1206`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1206/predicates_test.go:3` — above `package cycle1206`

```text
// Package cycle1206 encodes the cycle-1206 ACS predicates for task
// `reject-inboxbatch-rootcause-rule`: the formal closure of
// `add-inboxbatch-rootcause-rule` as a DESIGN REJECTION rather than an
// implementation.
//
// The predicates are deliberately shaped as anti-regression assertions. The
// cycle-1204 attempt (state.json failedApproaches[54], audit FAIL, D1-D4) added
// a `rootCauseRule` binding inbox items whose `root_cause` field matched by
// exact string equality. It was fully reverted. Live measurement of the real
// backlog (20/20 non-empty root_cause values unique free-form prose, 122-1564
// bytes, zero duplicates) proves exact-match binding on that field emits zero
// edges — so the correct deliverable is that the rule STAYS ABSENT and the
// reason is recorded where the next author will look.
```

### `go/acs/cycle1206/predicates_test.go:28` — above `func TestC1206_001_DefaultRulesHasNoRootCauseSignal(t *testing.T) {`

```text
// TestC1206_001_DefaultRulesHasNoRootCauseSignal asserts the compiled default
// rule set is EXACTLY the documented structural signals — campaign and
// file-area — by probing each rule with a pair of items bound by one signal
// only and requiring every rule to be accounted for by one probe. (The dep
// signal was removed in cycle 1724: under ADR-0106 W3 a dependent never shares
// a lane menu with its unlanded dependency, so dep edges could not bind.)
//
// This is the load-bearing anti-regression assertion: a re-added rootCauseRule
// responds to none of the structural probes (the Go Item type carries no
// root_cause field at all), so it lands as UNACCOUNTED and this predicate goes
// RED. A source grep for "RootCause" would be gameable by renaming; this is not.
```
