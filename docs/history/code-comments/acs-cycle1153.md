# Comment history: `acs/cycle1153`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1153/predicates_test.go:3` — above `package cycle1153`

```text
// Package cycle1153 materialises the acceptance criteria for the two tasks
// triage committed to THIS cycle, both under the fleet-scoped todo-id
// `artifact-name-ssot-remaining-callsites`:
//
//   - audit-name-ssot-hook-and-ledger  → the phase-hook (phases/audit,
//     phases/build) and ledger-binding (core/phase_bindings) layers must
//     derive their report filename from the phasecontract SSOT.
//   - audit-name-ssot-dispatch-and-gates → the cross-CLI dispatch
//     (consensusdispatch), verdict reader (coherence), build-removal gate
//     (core/build_removal_check) and ship manifest (phases/ship) must do the
//     same.
//
// The deferred id (artifact-name-lint-guard) carries ZERO predicates — R9.3:
// predicates bind only to triage-committed work, and a predicate gating
// deferred work starves the committed task (the cycle-280 failure mode).
//
// Continuation context (ADR-0076). This worktree is a salvage continuation:
// commit bd9d408d ("salvage snapshot") already carries a candidate
// implementation for both tasks, so these predicates are pre-existing GREEN on
// HEAD. Their RED was demonstrated against the pre-salvage tree state (parent
// commit 77dfdbc9) — see test-report.md § RED Run Output. They remain
// load-bearing: they are the contract the audit gate replays, and they fail
// LOUDLY if the salvaged implementation is reverted, partially landed, or
// re-drifts.
//
// Predicate strategy. This is a pure refactor: every literal being removed is
// currently EQUAL to the registry's value, so no runtime observation can
// distinguish "reads the SSOT" from "carries an equal copy" — duplication is
// inherently a source-level property. The suite therefore pairs three axes so
// no single one is gameable alone:
//
//   - 001 and 005 are BEHAVIORAL: they invoke the SSOT accessors and the
//     exported consumer (coherence.ReadCycleVerdicts) and assert on returned
//     values and real side effects, including negative and edge inputs. 001 is
//     the pairing anchor — a builder who greens the absence checks by deleting
//     or renaming the registry's audit/build contracts fails here.
//   - 002 and 003 are duplication-ABSENCE checks scoped to the exact functions
//     and var the two tasks name, paired with a positive "delegates to the
//     accessor" assertion so deleting the call site cannot green them.
//   - 004 is the repo-wide invariant AC, enforced by PARSING go/internal with
//     go/parser and inspecting string-literal AST nodes — not grep. Prose in
//     comments and error messages that merely mentions a report name is
//     correctly ignored; only a literal that IS the filename trips it.
//
// The absence checks are un-gameable by string insertion: adding the magic
// string makes them FAIL, never pass (the inverse of the cycle-85 degenerate
// predicate failure mode).
```

### `go/acs/cycle1153/predicates_test.go:306` — above `if v, _, ran := coherence.ReadCycleVerdicts(t.TempDir()); ran || v != "" {`

```text
// EDGE: an empty workspace yields no verdict and no error path — never a
// fabricated verdict (the cycle-603 echo bug this reader guards).
```
