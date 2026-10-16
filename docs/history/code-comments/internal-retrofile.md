# Comment history: `internal/retrofile`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d8

### `go/internal/retrofile/retrofile_test.go:1` — above `package retrofile`

```text
// White-box behavioral suite for the retro→inbox preventive-actions
// autofiler (task retro-preventive-actions-autofile-inbox, cycle 657).
//
// These tests encode the acceptance criteria for the NEW leaf package
// internal/retrofile, which closes the learning→action loop: a retro's
// structured "preventive_actions" become weighted .evolve/inbox todos on the
// SAME deterministic seam the FAILED_UNEXPLAINED classifier already uses
// (cmd/evolve/cmd_loop_outcome.go:fileUnexplainedOutcomeDefect). They are the
// system-under-test for the cycle-657 ACS predicates
// (go/acs/cycle657/predicates_test.go), which shell `go test -run <name>
// ./internal/retrofile/` — so a predicate greens only when the real injector
// files/deduplicates the right items, never on a source-grep.
//
// RED until the Builder writes retrofile.go implementing:
//
//	type PreventiveAction struct {
//	    ID, Title  string
//	    WeightHint float64  // >0 overrides defaultWeight (recurrence hint)
//	    Files      []string
//	    Evidence   string
//	    Recurrence int
//	}
//	func ParsePreventiveActions(report []byte) ([]PreventiveAction, error)
//	func FileActions(inboxDir string, cycle int, actions []PreventiveAction,
//	    defaultWeight float64, now time.Time) (written []string, err error)
//
// DO NOT modify this file — implement production code to make it GREEN.
```
