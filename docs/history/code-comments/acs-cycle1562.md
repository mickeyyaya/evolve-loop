# Comment history: `acs/cycle1562`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1562/predicates_test.go:3` — above `package cycle1562`

```text
// Package cycle1562 materialises the cycle-1562 acceptance criteria for the
// two fleet-scoped tasks `retrospective-delivery-relaunch` and
// `retrospective-delivery-evidence-contract`.
//
// The defect (.evolve/runs/cycle-1510/retrospective-launch-error.txt +
// retrospective-interactions.ndjson): a retrospective launch logged "prompt
// delivered", produced zero tokens and zero cost, and then burned two full
// 900-second stop-review intervals before dying with ExitArtifactTimeout. The
// tmux driver's submit-verify guard (go/internal/bridge/driver_tmux_submitverify.go)
// had ALREADY classified that pane as `submit_wedged` within milliseconds —
// but both consumer sites in driver_tmux_repl.go (the prompt-paste site and
// the one-shot nudge site) pipe verifySubmitted's outcome straight into
// recordSubmitVerify, which appends to the ndjson ledger and returns nothing
// usable for control flow. The classification is produced and never consumed:
// at the control-flow level a detected delivery failure is indistinguishable
// from a healthy launch that simply never speaks, and the run spends the whole
// silence budget before the dispatcher's already-present one-relaunch recovery
// (cyclerun_dispatch.go, IsInfraTeardownError) ever gets a turn.
//
// Task 1 makes the evidenced delivery failure short-circuit into the EXISTING
// bounded ExitArtifactTimeout outcome — no new retry loop, no new sentinel.
// Task 2 carries the classified cause through the bridge boundary into the
// terminal <phase>-failure-diag.json as a machine-readable field, so an
// exhausted retro leaves durable evidence instead of a bare artifact timeout
// whose root cause exists only in discarded stderr.
//
// Predicate strategy — every seam this cycle touches (runTmuxREPL,
// verifySubmitted, artifactTimeoutSummary, writePhaseFailureDiag) is
// UNEXPORTED inside package bridge / package core, so a leaf acs package
// cannot call it. These predicates therefore use the sanctioned
// behavioural-via-subprocess shape (cycle-987/997/1532/1544/1550 precedent):
// a `-run`-narrowed, single-named-package `go test -v` that must print
// `--- PASS: <name>` for each binding test the Builder makes pass. Asserting
// on the PASS line (never on exit 0) is load-bearing: `go test -run` against a
// pattern matching no test exits 0 with "no tests to run", so a deleted or
// renamed binding test would otherwise false-GREEN. The `-run` narrowing is
// also what keeps ./internal/core inside the ACS lane's wall-clock budget.
```

### `go/acs/cycle1562/predicates_test.go:79` — above `func TestC1562_001_WedgedPromptShortCircuitsTheSilenceBudget(t *testing.T) {`

```text
// TestC1562_001_WedgedPromptShortCircuitsTheSilenceBudget — AC1-001, the
// cycle-1510 reproduction. A verified `submit_wedged` prompt must return
// ExitArtifactTimeout IMMEDIATELY, before the driver enters a single two-second
// artifact-wait poll. Today the outcome is discarded and the run consumes the
// entire silence budget first; the binding test counts the polls, so a fix that
// merely relabels the eventual timeout cannot satisfy it.
```

### `go/acs/cycle1562/predicates_test.go:106` — above `func TestC1562_003_WedgedNudgeCarriesItsClassifiedCause(t *testing.T) {`

```text
// TestC1562_003_WedgedNudgeCarriesItsClassifiedCause — AC1-003, the SECOND
// consumer site. The one-shot nudge fires from inside the stop-review pause
// branch, so it cannot skip a budget it has already spent — but its wedged
// outcome is the same evidence, and the terminal artifact-timeout marker must
// name the site and the classification instead of reporting the generic stall
// reason. Without this, cycle-1510's ndjson (`"result":"no_effect"` on every
// nudge) remains the only place the cause exists.
```
