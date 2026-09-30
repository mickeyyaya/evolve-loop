# Comment history: `acs/cycle1698`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1698/predicates_test.go:3` — above `package cycle1698`

```text
// Package cycle1698 materialises the acceptance criteria of the one
// fleet-scoped task pinned to this lane: `unify-auditor-ledger-readers`
// (.evolve/inbox/processing/cycle-1698/2026-08-27T06-00-00Z-unify-auditor-ledger-readers.json;
// triage top_n). Three readers each re-declare the auditor ledger row and
// their own scan — ship.findLatestAudit (internal/phases/ship/audit.go),
// cmd/evolve latestAuditEntry (cmd_composition_wiring.go) and
// releasepreflight.checkRecentAudit (a raw-line regex walk). The task folds
// them onto one leaf helper, internal/auditledger, with a typed
// ErrNoAuditorForRun sentinel each consumer maps onto its own vocabulary.
// redteamcheck is out of scope (cycle-scoped, all roles, no run scoping).
//
// PINNED HELPER SURFACE (the minimum every consumer needs; nothing more):
//
//	auditledger.LatestAuditorEntry(ledgerPath, runID string) (Entry|*Entry, error)
//	auditledger.ErrNoAuditorForRun   — an error value for errors.Is
//	Entry fields RunID, GitHEAD, ArtifactPath
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - POSITIVE : 001 — run-scoped binding, latest-any without a run, the
//     kind+role row identity, alien lines skipped, structured (not regex) parse.
//   - NEGATIVE : 002 — every "no bindable row" shape is the typed sentinel and
//     the foreign refusal names the refused entry; 008 — redteamcheck untouched
//     and the change set is non-vacuous.
//   - EDGE     : 003 — a missing ledger and an unreadable ledger stay
//     distinguishable from a miss; the release preflight keeps its advisory NONE.
//   - WIRING   : 005/006/007 — every consumer imports the helper, none re-declares
//     the row schema or regex, and the production caller releasepreflight.Run
//     binds exactly the row the helper binds.
//   - FLOOR    : 004 leaf, 009 stale TODO, 010 apicover graduation, 011 the
//     touched packages' own suites.
```

### `go/acs/cycle1698/predicates_test.go:242` — above `func TestC1698_002_NoBindableAuditorRowIsTheTypedSentinel(t *testing.T) {`

```text
// TestC1698_002_NoBindableAuditorRowIsTheTypedSentinel covers every miss shape.
// Each must be errors.Is(ErrNoAuditorForRun) — the one value ship maps to
// AUDIT_BINDING_NO_AUDITOR and composition fails closed on — and never read as
// a missing ledger. The foreign-run refusal must carry the refused entry (its
// run id and git_head) so an operator sees what would have been bound
// (cycle-1571 H3).
```
