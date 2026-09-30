# Comment history: `acs/cycle1145`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1145/predicates_test.go:3` — above `package cycle1145`

```text
// Package cycle1145 materialises the cycle-1145 acceptance criteria for the two
// fleet-scoped SSOT tasks triage committed to THIS cycle:
//
//   - artifact-name-ssot-scout-report-backfill  → route the hardcoded
//     "scout-report.md" literals through phasecontract's ArtifactName SSOT
//   - required-roles-ssot-subagent-dispatch     → derive subagent dispatch's
//     agentRoles allow-list from the phasecontract registry
//
// The third fleet-scoped id (cycle-docs-floor-architecture-changes) was
// DEFERRED by triage and therefore carries ZERO predicates here (R9.3:
// predicates bind only to triage-committed work; a predicate gating deferred
// work starves the committed task — the cycle-280 failure mode).
//
// Predicate strategy. Both tasks are duplication-removal refactors, so the two
// crux predicates split along the only two axes that can actually observe the
// defect:
//
//   - 001 is an ABSENCE check (acsassert-style FileNotContains semantics over
//     the production tree, the sanctioned form per go/acs/README.md "Absence
//     checks"): the quoted literal must survive in exactly ONE declaration
//     site. Duplication is inherently a source-level property — no runtime
//     observation can distinguish five copies of an equal string from one
//     shared one — so this is the predicate that red-fails today.
//     003 is its anti-gaming twin: greening 001 by deleting or renaming the
//     SSOT declaration itself fails 003.
//   - 002 is BEHAVIORAL and exercises evalgate (one of the five literal sites)
//     end-to-end through its exported core.DeliverableReviewer: the gate must
//     still find and parse a scout report named by the registry, and must fail
//     open when it is named anything else. It is pre-existing GREEN and stays
//     green only if the refactor preserves the resolved value — the regression
//     half of "consolidate the literal".
//   - 004 is BEHAVIORAL and red-fails today: every LLM-dispatchable agent the
//     phasecontract registry declares must be accepted by the subagent dispatch
//     allow-list. "router" is registered (agents/evolve-router.md,
//     .evolve/profiles/router.json) yet absent from run.go:120's hand-typed
//     agentRoles slice — that IS the drift the task removes.
//   - 005/006 bound the fix from both sides: 005 pins the five roles that exist
//     ONLY in the dispatch list (inspirer, evaluator, plan-reviewer, memo,
//     tester) so a naive slice→registry swap cannot silently drop dispatch
//     capability; 006 pins that "ship" — registered but NoArtifact, a native
//     host-side phase with no profile — stays REJECTED, so the derivation
//     cannot over-reach and break the existing profile-conformance invariant.
//
// Roots: production source and profiles are read under acsassert.RepoRoot (the
// worktree — where Builder's change lands and is committed), never main.
```
