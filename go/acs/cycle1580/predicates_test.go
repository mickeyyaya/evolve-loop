//go:build acs

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
package cycle1580

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// bridgePkg is the ONE package under test. Named explicitly (never a sweep) so
// the predicate cannot become a whole-repo staleness check under fleet load.
const bridgePkg = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"

// deliverablePkg holds the pre-audit contract gate — the boundary the
// cycle-1580 audit-repair predicate 008 binds.
const deliverablePkg = "github.com/mickeyyaya/evolve-loop/go/internal/deliverable"

// runBridgeTests shells `go test -run '^(<pattern>)$' -count=1 <bridgePkg>` and
// reports whether it exited cleanly plus the combined output. -count=1 defeats
// the test cache so the predicate always exercises current source. A compile
// failure or an assertion failure surfaces as a non-zero exit — the intended
// RED before Builder wires the seam. code < 0 is a launch failure (binary
// missing / killed), never a test verdict, so it is a hard error rather than a
// silent RED.
func runBridgeTests(t *testing.T, pattern string) (ok bool, out string) {
	t.Helper()
	return runPkgTests(t, bridgePkg, pattern)
}

// runPkgTests is runBridgeTests generalised over the package, for the two
// audit-repair predicates whose behaviour lives in ./internal/deliverable
// rather than ./internal/bridge. Still exactly ONE named package per
// invocation — never a sweep.
func runPkgTests(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC1580_001_TransientRegexResolvedOnceFromTheManifest — AC-1: the transient
// pattern is resolved ONCE, in newAutoResponder, from the LAUNCHED cli's
// manifest (mirroring exhaustedRegex) instead of being recompiled on every 2s
// poll. Both halves ride here: the value must equal the manifest's declared
// pattern (so a hard-coded literal fails) for claude AND codex (so a
// claude-only implementation fails).
func TestC1580_001_TransientRegexResolvedOnceFromTheManifest(t *testing.T) {
	ok, out := runBridgeTests(t, "TestAutoResponder_TransientRegexResolvedAtConstruction|TestAutoResponder_TransientRegexIsFamilyAgnostic")
	if !ok {
		t.Errorf("AC-1 unmet: the transient pattern is not resolved from the launched CLI's manifest at construction\n%s", out)
	}
}

// frozenExitCodes is the bridge's numeric exit contract as it stands BEFORE
// this cycle (internal/bridge/exitcodes.go). AC-2 forbids adding to it: the
// shortcircuit must exit through the existing ExitArtifactTimeout (81), because
// docs, skills and the dispatcher's failure classifier all key on these values.
var frozenExitCodes = map[string]string{
	"ExitOK": "0", "ExitSafetyGate": "2", "ExitCostLeak": "3", "ExitBadFlags": "10",
	"ExitREPLBootTimeout": "80", "ExitArtifactTimeout": "81", "ExitUnknownPrompt": "85",
	"ExitRespondLoopGuard": "86", "ExitRequireFullUnmet": "99", "ExitCmdTimeout": "124",
	"ExitMissingBinary": "127",
}

// exitConstRE captures every `Exit<Name> = <int>` row of the const table.
var exitConstRE = regexp.MustCompile(`(?m)^\s*(Exit\w+)\s+=\s+(\d+)`)

// TestC1580_002_NoNewExitCode — AC-2 (absence criterion). Two load-bearing
// assertions: (a) the exit-code TABLE still declares exactly the frozen
// name→value set — a new `ExitTransient…` row fails here — and (b) the
// shortcircuit itself actually returns ExitArtifactTimeout when driven through
// the real driver.
func TestC1580_002_NoNewExitCode(t *testing.T) {
	root := acsassert.RepoRoot(t)
	src, err := os.ReadFile(filepath.Join(root, "go", "internal", "bridge", "exitcodes.go"))
	if err != nil {
		t.Fatalf("read the bridge exit-code contract: %v", err)
	}
	got := map[string]string{}
	for _, m := range exitConstRE.FindAllStringSubmatch(string(src), -1) {
		got[m[1]] = m[2]
	}
	if len(got) == 0 {
		t.Fatalf("parsed no exit-code constants — the predicate is reading the wrong file")
	}
	for name, want := range frozenExitCodes {
		if got[name] != want {
			t.Errorf("exit code %s = %q, want %q — the numeric contract is load-bearing and must not drift", name, got[name], want)
		}
	}
	for name, val := range got {
		if _, known := frozenExitCodes[name]; !known {
			t.Errorf("AC-2 violated: new exit code %s = %s — the transient shortcircuit must reuse ExitArtifactTimeout (81)", name, val)
		}
	}
	if ok, out := runBridgeTests(t, "TestRunTmuxREPL_TransientDwell_ReusesExistingExitAndArtifacts"); !ok {
		t.Errorf("AC-2 unmet: the shortcircuit does not exit through the existing ExitArtifactTimeout path\n%s", out)
	}
}

// TestC1580_003_SixtySecondDwellWithReset — AC-3, all three edges driven
// through the real wait loop: the dwell FIRES on a pane parked on the live 529
// error (before the 300s reviewer is ever consulted), does NOT fire inside the
// 60s window (a 40s checkpoint still reaches the reviewer), and RESETS on any
// non-matching frame (an alternating pane never crosses).
func TestC1580_003_SixtySecondDwellWithReset(t *testing.T) {
	ok, out := runBridgeTests(t, "TestRunTmuxREPL_TransientDwell_EnforceStopsBeforeTheArtifactReviewer|"+
		"TestRunTmuxREPL_TransientDwell_DoesNotFireBeforeSixtySeconds|"+
		"TestRunTmuxREPL_TransientDwell_ResetsOnNonMatchingFrame|"+
		"TestRunTmuxREPL_TransientPaneSkipsFullArtifactTimeout")
	if !ok {
		t.Errorf("AC-3 unmet: the 60s transient dwell tracker does not fire/hold/reset as specified\n%s", out)
	}
}

// TestC1580_004_BusyPaneIsNeverPreempted — AC-4: a pane showing the 529 text
// AND the live interrupt affordance is a WORKING agent. The stop-review prime
// directive (cycle-254/255 false-FAIL) outranks fast-fail, exactly as
// fatalPaneVerdict's ev.Busy guard encodes it.
func TestC1580_004_BusyPaneIsNeverPreempted(t *testing.T) {
	ok, out := runBridgeTests(t, "TestRunTmuxREPL_TransientDwell_BusyPaneIsNeverPreempted")
	if !ok {
		t.Errorf("AC-4 unmet: a BUSY pane was fast-failed — the dwell must carry the same busy guard as fatalPaneVerdict\n%s", out)
	}
}

// TestC1580_005_StageDialAndDurableTelemetry — AC-5: the existing ADR-0044
// dial gates the ACTION (off = legacy and unclassified, shadow = observe-only,
// enforce = act) while shadow/enforce both leave a DURABLE
// would_fast_fail/fast_failed record in <workspace>/<phase>-interactions.ndjson
// — the false-positive evidence the soak reporter reads. stderr-only evidence
// left ADR-0044 C2's soak blind by construction (the R8.3 lesson).
func TestC1580_005_StageDialAndDurableTelemetry(t *testing.T) {
	ok, out := runBridgeTests(t, "TestRunTmuxREPL_TransientDwell_ShadowObservesWithoutActing|"+
		"TestRunTmuxREPL_TransientDwell_EnforceRecordsFastFailed|"+
		"TestRunTmuxREPL_TransientDwell_OffStageIsLegacy")
	if !ok {
		t.Errorf("AC-5 unmet: the ADR-0044 stage dial and/or the would/did telemetry is not wired\n%s", out)
	}
}

// TestC1580_006_ReviewStopReusesTheCompletedBlock — AC-6: the fired dwell sets
// a ReviewStop verdict and breaks into the EXISTING `!completed` machinery, so
// the escalation report (action=stop, transient reason, pane evidence), the
// self-describing marker line (transient=true, last_review=stop) and exit 81
// are all REUSED rather than duplicated.
func TestC1580_006_ReviewStopReusesTheCompletedBlock(t *testing.T) {
	ok, out := runBridgeTests(t, "TestRunTmuxREPL_TransientDwell_ReusesExistingExitAndArtifacts")
	if !ok {
		t.Errorf("AC-6 unmet: the shortcircuit does not route through the existing ReviewStop/!completed path\n%s", out)
	}
}

// TestC1580_007_RedispatchDelayScopedToTheShortcircuit — AC-7: fast-failing at
// 60s only pays off if the retry avoids the same upstream weather window, so
// the enforce stop asks for a deliberate pause — and ONLY on that path (a
// blanket delay would tax every artifact timeout in the pipeline).
func TestC1580_007_RedispatchDelayScopedToTheShortcircuit(t *testing.T) {
	ok, out := runBridgeTests(t, "TestRunTmuxREPL_TransientDwell_EnforceDelaysRedispatch|"+
		"TestRunTmuxREPL_TransientDwell_NoDelayOnOrdinaryTimeout")
	if !ok {
		t.Errorf("AC-7 unmet: the re-dispatch delay is missing on the shortcircuit path (or leaked onto the ordinary one)\n%s", out)
	}
}

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
func TestC1580_008_ScoutReportChallengeTokenIsEnforcedNatively(t *testing.T) {
	ok, out := runPkgTests(t, deliverablePkg, "TestVerify_Scout_MissingChallengeToken_Violation|"+
		"TestVerify_Scout_WrongToken_Violation|"+
		"TestVerify_Scout_TokenEchoed_OK|"+
		"TestVerify_Scout_NoTokenFile_FailOpen|"+
		"TestContract_RequireChallengeToken_CoversTokenConsumingPhases")
	if !ok {
		t.Errorf("AC-R1 unmet: the scout challenge-token binding is not enforced at the pre-audit contract gate — it still fails open on persona prose\n%s", out)
	}
}

// TestC1580_009_ErroredCaptureDoesNotReanchorThePaneDelta — AC-R2 (cycle-1580
// audit repair, defect L1). Hoisting the completion-wait capture into one
// canonical frame dropped the `cerr == nil` guard, so an errored CapturePane
// feeds "" to PaneDelta.Next, re-anchors the delta and makes the next good
// frame re-emit the whole stable pane to <agent>-pane.live. The predicate
// drives the real driver through good/good/ERROR/good and asserts the answer
// streams exactly once, and that an errored capture neither aborts nor stalls
// the wait.
func TestC1580_009_ErroredCaptureDoesNotReanchorThePaneDelta(t *testing.T) {
	ok, out := runBridgeTests(t, "TestRunTmuxREPL_CaptureErrorDoesNotReanchorPaneDelta|"+
		"TestRunTmuxREPL_CaptureErrorStillCompletes")
	if !ok {
		t.Errorf("AC-R2 unmet: an errored CapturePane still re-anchors the pane delta and duplicates the live stream\n%s", out)
	}
}
