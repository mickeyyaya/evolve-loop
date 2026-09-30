# Comment history: `acs/cycle1129`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1129/predicates_test.go:3` — above `package cycle1129`

```text
// Package cycle1129 materialises the cycle-1129 acceptance criteria for the one
// triage-committed top_n task (see scout-report.md / triage-report.md):
//
//   - exhaustion-checkpoint-raw-pane-stripping: the exhaustion-regex DRIFT alarm
//     (exhaustion_drift.go, diagnostic-only, fired on the exit-81 teardown) is
//     handed the RAW lastGoodPane at driver_tmux_repl.go:813, while the primary
//     exhaustion detector one line up (704) correctly scans
//     strippedForExhaustionScan(pane, ar.injectedPrompt). Agent-authored content
//     — an edit diff, an echoed prompt — that the real detector deliberately
//     ignores can therefore still raise "POSSIBLE EXHAUSTION-REGEX DRIFT",
//     sending an operator after a regex that is working as intended.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…574 / cycle-976
// precedent). Each predicate shells `go test -run` over the RED integration
// tests authored this cycle in internal/bridge (exhaustion_drift_test.go). None
// is a source-grep: every one drives the REAL tmux driver loop (Engine.LaunchArgs
// over a fake tmux) to its exit-81 teardown and asserts on the stderr the alarm
// actually emitted — so they pin the CALL SITE's pane treatment, not a helper
// signature. RED now on 001/002 (raw pane ⇒ false alarm); GREEN once Builder
// strips at the call site. 003 is the anti-no-op pin (already GREEN) that keeps
// the fix from being "achieved" by deleting or blanketing the alarm.
```

### `go/acs/cycle1129/predicates_test.go:62` — above `func TestC1129_002_DriftAlarmIgnoresPromptEcho(t *testing.T) {`

```text
// TestC1129_002_DriftAlarmIgnoresPromptEcho — AC-1 (second axis).
// The other half of strippedForExhaustionScan: a pane line that is a verbatim
// echo of the injected prompt must not raise the alarm either (cycle-641/642
// class). RED now for the same single root cause; a call-site fix that strips
// only diff lines would leave this one red.
```
