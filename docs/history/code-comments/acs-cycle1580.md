# Comment history: `acs/cycle1580`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1580/predicates_test.go:3` — above `package cycle1580`

```text
// Package cycle1580 materialises the cycle-1580 acceptance criteria for the
// single fleet-scoped task
// `transient-artifact-timeout-shortcircuit-the-silence-budget`: the ReviewStop
// variant of the transient-artifact-timeout shortcircuit.
//
// The defect. `classifyTransientPane` recognizes a family's manifest-declared
// transient upstream error, but it is consulted ONLY after the artifact wait
// has already timed out, where it merely annotates the marker line. Nothing
// reads it DURING the wait — so a session parked on "API Error: 529 Overloaded
// … usually temporary" burns the whole silence budget (3 of 4 observed router
// stalls, cycles 1523/1524/1526, ~600s each) before anything reacts.
//
// Predicate strategy. Each predicate below EXERCISES the system under test:
// six of the seven drive the real `runTmuxREPL` path (through
// `Engine.LaunchArgs`, a scripted pane sequence and the live cycle-1523 529
// pane fixture) by shelling ONE named package's behavioral contract —
// `go test -run '^(…)$' -count=1 ./internal/bridge` — and assert on its exit
// code. None is a source grep of production code (the cycle-85 degenerate-
// predicate ban). The one structural predicate (002) parses the exit-code
// constant TABLE and asserts set equality against the frozen contract, which
// no added magic string can satisfy — it is an ABSENCE criterion ("do not add
// a new exit code") and has no behavioral form.
//
// Flaky-shape discipline: exactly one named package per invocation
// (`./internal/bridge`, never a `/...` sweep, never `./internal/core` or
// `./cmd/evolve`), always narrowed with `-run`, always `-count=1`, no
// wall-clock bounds, no literal PIDs, no bare `git`, no load generators.
```

### `go/acs/cycle1580/predicates_test.go:45` — above `const deliverablePkg = "github.com/mickeyyaya/evolve-loop/go/internal/deliverable"`

```text
// deliverablePkg holds the pre-audit contract gate — the boundary the
// cycle-1580 audit-repair predicate 008 binds.
```

### `go/acs/cycle1580/predicates_test.go:150` — above `func TestC1580_004_BusyPaneIsNeverPreempted(t *testing.T) {`

```text
// TestC1580_004_BusyPaneIsNeverPreempted — AC-4: a pane showing the 529 text
// AND the live interrupt affordance is a WORKING agent. The stop-review prime
// directive (cycle-254/255 false-FAIL) outranks fast-fail, exactly as
// fatalPaneVerdict's ev.Busy guard encodes it.
```

### `go/acs/cycle1580/predicates_test.go:161` — above `func TestC1580_005_StageDialAndDurableTelemetry(t *testing.T) {`

```text
// TestC1580_005_StageDialAndDurableTelemetry — AC-5: the existing ADR-0044
// dial gates the ACTION (off = legacy and unclassified, shadow = observe-only,
// enforce = act) while shadow/enforce both leave a DURABLE
// would_fast_fail/fast_failed record in <workspace>/<phase>-interactions.ndjson
// — the false-positive evidence the soak reporter reads. stderr-only evidence
// left ADR-0044 C2's soak blind by construction (the R8.3 lesson).
```

### `go/acs/cycle1580/predicates_test.go:200` — above `func TestC1580_008_ScoutReportChallengeTokenIsEnforcedNatively(t *testing.T) {`

```text
// TestC1580_008_ScoutReportChallengeTokenIsEnforcedNatively — AC-R1 (cycle-1580
// audit repair, defect C1). The auditor found scout-report.md with no
// challenge-token header: the anti-forgery binding for the scout artifact
// failed OPEN because it lived in persona prose only, and scout's contract is
// the one report contract leaving RequireChallengeToken unset. The exemption's
// stated premise ("scout mints the token") is false — the orchestrator mints it
// (internal/core/cyclerun.go:726-748) and scout reads it
// (internal/phases/scout/scout.go:64). This predicate drives the real Verify
// boundary: a minted token on disk plus a report without the header must be a
// missing_challenge_token violation, a wrong token must not satisfy it, and a
// token-less run must still fail open.
```

### `go/acs/cycle1580/predicates_test.go:222` — above `func TestC1580_009_ErroredCaptureDoesNotReanchorThePaneDelta(t *testing.T) {`

```text
// TestC1580_009_ErroredCaptureDoesNotReanchorThePaneDelta — AC-R2 (cycle-1580
// audit repair, defect L1). Hoisting the completion-wait capture into one
// canonical frame dropped the `cerr == nil` guard, so an errored CapturePane
// feeds "" to PaneDelta.Next, re-anchors the delta and makes the next good
// frame re-emit the whole stable pane to <agent>-pane.live. The predicate
// drives the real driver through good/good/ERROR/good and asserts the answer
// streams exactly once, and that an errored capture neither aborts nor stalls
// the wait.
```
