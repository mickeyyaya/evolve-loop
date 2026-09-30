# Comment history: `acs/cycle813`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle813/predicates_test.go:3` — above `package cycle813`

```text
// Package cycle813 materializes the cycle-813 acceptance criteria for this
// fleet lane's sole committed task, verify-fleet-soak-ci-green /
// confirm-integration-tag-in-go-workflow (fix-fleet-soak-red-ci, P0 0.99).
//
// Scout found the original inbox defect (soak test asserting pre-orphan-sweep
// behavior) already fixed on main as of cycle-809: the actual CI red was an
// apicover-enforce gap (go/internal/core.LaneScope/LaneScopeFile,
// go/internal/cyclestate.SkippedPhase), both now covered by
// go/internal/core/lanescope_apicover_test.go and
// go/internal/cyclestate/result_test.go. This cycle's job is verification,
// not re-fix (see scout-report.md "Selected Tasks").
//
// Task 1's AC ("gh run list shows HEAD completed success") is inherently
// non-hermetic — it reads live GitHub Actions state that changes independently
// of this repo's tree and cannot be pinned as a repeatable regression
// predicate. It is dispositioned manual+checklist in test-report.md instead
// (AC-Materialization Contract) rather than gamed with a source-grep stand-in.
//
// Task 2's AC ("go workflow YAML passes -tags=integration on the step that
// runs ./cmd/evolve/...") is a genuine config-presence check: the acceptance
// criterion IS "does this exact configuration line exist", not "does a
// magic string mentioning it exist somewhere". That is the documented
// `// acs-predicate: config-check` waiver case (Predicate Quality section) —
// the sole exception to the FileContains-over-source-is-degenerate rule.
```
