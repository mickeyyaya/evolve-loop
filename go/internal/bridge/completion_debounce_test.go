package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// completion_debounce_test.go — the RED contract for cycle-1233's
// artifact-ready-crosspoll-debounce.
//
// The defect (cycle-1198, observed): artifactDetector.poll completes on the
// FIRST non-empty read of the deliverable. An agent that writes a file and then
// fixes it up with a follow-up Edit seconds later gets its half-written
// intermediate accepted — the gate rejected a scout-report.md that parsed
// perfectly moments afterwards. The deliverable-side grace window that already
// shipped (deliverable.go:180) covers absence/emptiness only; a
// "parses fine, wrong content" read is not retried, by design.
//
// The fix is the artifact twin of stdoutDetector's stdoutIdlePolls debounce:
// artifactDetector must observe the SAME (size, mtime) across
// artifactStableTicks consecutive poll ticks (~2s apart, driven by the wait
// loop at driver_tmux_repl.go:562) before declaring ready. mtime participates
// because size alone is content-blind to a same-length rewrite.
//
// Explicitly NOT the fix (rejected, HIGH review of the cycle-1212 attempt): an
// in-poll settle sleep. A tens-of-ms sleep inside one poll() call cannot span
// the multi-second gap between an agent's Write and its Edit. The state must be
// carried ACROSS calls, on the detector.
//
// Test map:
//
//	AC-1 stability  → ReadyOnlyAfterCrossPollStability (positive) +
//	                  NotReadyWhileArtifactStillGrowing (negative, size axis) +
//	                  NotReadyOnSameSizeRewrite (negative, mtime axis)
//	AC-2 ctx-cancel → CtxCancelledShortCircuitsDebounce (+ its absent-file negative)
//	AC-3 relocation → RelocationNoteSurvivesUntilStable
//	AC-4 wiring     → TestRunTmuxREPL_ArtifactDebounceWiredIntoWaitLoop (drives the
//	                  REAL production wait loop; a detector-only test proves nothing
//	                  about the caller) + the untouched fixture-budget regression
//	                  guard TestRunTmuxREPL_ExtendNoEscalationReport.

// fixedMTime is an arbitrary but STABLE modification time. Tests set it
// explicitly with os.Chtimes rather than relying on the host filesystem's
// timestamp granularity, so a debounce keyed on mtime is exercised
// deterministically on every platform.
var fixedMTime = time.Unix(1_700_000_000, 0)

// writeArtifact writes body at path and pins its mtime, so the detector sees
// exactly the (size, mtime) pair the test intends.
func writeArtifact(t *testing.T, path, body string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
}

// pollUntilReady drives d.poll up to maxPolls times, invoking mutate(i) (when
// non-nil) immediately BEFORE each poll to model what the agent did to the file
// between two wait-loop ticks. It returns the 1-based poll index at which the
// detector reported ready and the note it carried, or (-1, "") if it never did.
func pollUntilReady(t *testing.T, d *artifactDetector, maxPolls int, mutate func(i int)) (int, string) {
	t.Helper()
	for i := 1; i <= maxPolls; i++ {
		if mutate != nil {
			mutate(i)
		}
		ready, _, note, err := d.poll(context.Background())
		if err != nil {
			t.Fatalf("poll %d: unexpected detector error: %v", i, err)
		}
		if ready {
			return i, note
		}
	}
	return -1, ""
}

func newArtifactDetectorAt(ws, artifact string) *artifactDetector {
	return &artifactDetector{cfg: &Config{Workspace: ws, Artifact: artifact}}
}

// --- AC-1: cross-poll stability ---------------------------------------------

// TestArtifactDetector_ReadyOnlyAfterCrossPollStability pins the positive half
// of AC-1: a settled file completes, but never on the tick it is first seen.
// The first-sighting assertion is the whole point — cycle-1198's truncated
// deliverable was a perfectly non-empty file on exactly that tick.
func TestArtifactDetector_ReadyOnlyAfterCrossPollStability(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	d := newArtifactDetectorAt(ws, canonical)

	// Absent → not ready, no note, no error (unchanged legacy behavior).
	if ready, _, note, err := d.poll(context.Background()); ready || err != nil || note != "" {
		t.Fatalf("absent artifact: got (ready=%v, note=%q, err=%v), want (false, \"\", nil)", ready, note, err)
	}

	writeArtifact(t, canonical, "# report\n\nDONE\n", fixedMTime)

	// First sighting must NOT complete: the file may still be mid Write→Edit.
	ready, _, _, err := d.poll(context.Background())
	if err != nil {
		t.Fatalf("first sighting: unexpected error %v", err)
	}
	if ready {
		t.Fatal("first sighting completed the phase — the cross-poll debounce is absent; " +
			"a mid-Write→Edit deliverable is accepted exactly here (cycle-1198)")
	}

	// Nothing changes afterwards → ready within the stability window.
	got, note := pollUntilReady(t, d, artifactStableTicks+2, nil)
	if got < 0 {
		t.Fatalf("a file that never changed again was never accepted within %d further polls — "+
			"the debounce must SETTLE, not stall", artifactStableTicks+2)
	}
	if !strings.Contains(note, "appeared") {
		t.Errorf("completion note = %q, want it to mention 'appeared' (operator-facing log line preserved)", note)
	}
}

// TestArtifactDetector_NotReadyWhileArtifactStillGrowing is the negative half of
// AC-1 on the SIZE axis: a deliverable still being appended to must never
// complete, no matter how many ticks pass. This is the anti-no-op assertion —
// an implementation that keeps returning ready on first sight passes the
// positive test above and fails here.
func TestArtifactDetector_NotReadyWhileArtifactStillGrowing(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	d := newArtifactDetectorAt(ws, canonical)

	grow := func(i int) {
		writeArtifact(t, canonical,
			"# report\n"+strings.Repeat("section\n", i),
			fixedMTime.Add(time.Duration(i)*time.Second))
	}
	if got, _ := pollUntilReady(t, d, 6, grow); got >= 0 {
		t.Fatalf("a still-growing artifact completed at poll %d — the stability counter "+
			"must RESET on every observed change", got)
	}
}

// TestArtifactDetector_NotReadyOnSameSizeRewrite is the negative half of AC-1 on
// the MTIME axis, and the reason mtime is in the key at all: an agent's fix-up
// Edit that swaps equal-length text (a typo, a flipped verdict word) leaves the
// size identical. A size-only debounce silently degrades back to the cycle-1198
// bug for exactly that shape.
func TestArtifactDetector_NotReadyOnSameSizeRewrite(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	d := newArtifactDetectorAt(ws, canonical)

	bodies := []string{"verdict: PASS\n", "verdict: FAIL\n"} // identical length
	rewrite := func(i int) {
		writeArtifact(t, canonical, bodies[i%len(bodies)],
			fixedMTime.Add(time.Duration(i)*time.Second))
	}
	if got, _ := pollUntilReady(t, d, 6, rewrite); got >= 0 {
		t.Fatalf("a same-SIZE, different-content, freshly-modified artifact completed at poll %d — "+
			"the stability key must include mtime, not size alone", got)
	}
}

// --- AC-2: ctx-cancel short-circuit -----------------------------------------

// TestArtifactDetector_CtxCancelledShortCircuitsDebounce pins AC-2. The wait
// loop makes ONE final poll after its context is cancelled
// (driver_tmux_repl.go:576) precisely so a finished session is not laundered
// into ExitArtifactTimeout. If that last look still demands a fresh stability
// window the detector will never get another tick to complete, the debounce
// converts every teardown-at-the-finish-line into a false timeout — turning a
// truncated-read fix into a worse false-FAIL generator.
func TestArtifactDetector_CtxCancelledShortCircuitsDebounce(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	d := newArtifactDetectorAt(ws, canonical)
	writeArtifact(t, canonical, "# report\n\nDONE\n", fixedMTime)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// A brand-new detector with NO stability history at all: the cancelled ctx
	// must make this single poll authoritative.
	ready, _, note, err := d.poll(ctx)
	if err != nil {
		t.Fatalf("cancelled ctx with artifact present: unexpected error %v", err)
	}
	if !ready {
		t.Fatal("cancelled ctx + a complete deliverable on disk did NOT complete — the final " +
			"post-cancel poll must short-circuit the debounce, not demand a window it can never get")
	}
	if note == "" {
		t.Error("short-circuit completion carried no note; the operator log line must survive the fast path")
	}

	// Negative: cancellation must not manufacture completion out of nothing.
	empty := t.TempDir()
	d2 := newArtifactDetectorAt(empty, filepath.Join(empty, "report.md"))
	if ready, _, _, err := d2.poll(ctx); ready || err != nil {
		t.Fatalf("cancelled ctx with NO artifact: got (ready=%v, err=%v), want (false, nil) — "+
			"an unfinished session must still report a timeout", ready, err)
	}
}

// --- AC-3: relocation-note survival -----------------------------------------

// TestArtifactDetector_RelocationNoteSurvivesUntilStable pins AC-3. artifactReady
// returns relocatedFrom exactly ONCE — on the tick it moves a non-canonical
// write into place (cycle-108/141 tolerance). Every later tick sees the file at
// the canonical path and returns from == "". A debounce that discards the note
// of a not-yet-stable tick therefore permanently swallows the "the agent wrote
// to the wrong place" diagnostic. The detector must stash it.
func TestArtifactDetector_RelocationNoteSurvivesUntilStable(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	fallback := filepath.Join(ws, "workspace", "report.md")
	d := newArtifactDetectorAt(ws, canonical)

	writeArtifact(t, fallback, "# report\n\nDONE\n", fixedMTime)

	got, note := pollUntilReady(t, d, artifactStableTicks+3, nil)
	if got < 0 {
		t.Fatalf("a relocated, then-unchanged artifact never completed within %d polls", artifactStableTicks+3)
	}
	if got == 1 {
		t.Error("relocation completed on the very first tick — relocation is not an exemption " +
			"from the stability window; a non-canonical write can be mid-Edit too")
	}
	if !strings.Contains(note, "relocated from non-canonical") {
		t.Errorf("completion note = %q, want the relocation diagnostic to SURVIVE the unstable "+
			"tick that observed it (the wrote-to-the-wrong-place signal is single-shot)", note)
	}
	if !strings.Contains(note, fallback) {
		t.Errorf("completion note = %q, want it to name the non-canonical source %s", note, fallback)
	}
}

// --- AC-4: the debounce is WIRED into the production wait loop --------------

// churningReviewer models an agent that keeps rewriting its deliverable: every
// review checkpoint appends another section, so the file's size changes on
// every tick and the stability window can never close. After `extends` verdicts
// it pauses, so the loop terminates deterministically instead of spinning.
type churningReviewer struct {
	artifact string
	calls    int
	extends  int
}

func (c *churningReviewer) Review(StopEvent) ReviewVerdict {
	c.calls++
	_ = os.WriteFile(c.artifact,
		[]byte("# report\n"+strings.Repeat("still writing…\n", c.calls)), 0o644)
	if c.calls >= c.extends {
		return ReviewVerdict{Action: ReviewPause, Reason: "stalled"}
	}
	return ReviewVerdict{Action: ReviewExtend, Reason: "still working"}
}

// TestRunTmuxREPL_ArtifactDebounceWiredIntoWaitLoop is the CALLER proof: it
// drives the real driver (Engine.LaunchArgs → runTmuxREPL → detector.poll at
// driver_tmux_repl.go:601) rather than the detector in isolation, so a debounce
// implemented on a struct nothing reaches cannot pass it.
//
// Contract: while the deliverable is still being rewritten on every tick, the
// launch must NOT report success. Before the fix the first write completes the
// phase immediately and the driver exits ExitOK — that is this test's RED.
func TestRunTmuxREPL_ArtifactDebounceWiredIntoWaitLoop(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	rev := &churningReviewer{artifact: fx.artifact, extends: 5}

	code, stderr := runTmuxOnStopReview(t, fx, tmux, rev, nil,
		Deps{ArtifactTimeoutS: 2}, "--allow-bypass", "--agent=scout")

	// Assert the POSITIVE outcome, not merely "not ExitOK". A churning
	// deliverable must exhaust the stop-review budget and die as
	// ExitArtifactTimeout; every other non-OK exit means the driver never
	// reached the artifact wait loop at all (REPL boot timeout, dead-shell
	// guard, auto-respond escalation/loop-guard — each of which narrates itself
	// on stderr). `!= ExitOK` accepted all of those as proof of a debounce they
	// never exercised, and the reviewer-count guard below then failed with a
	// count and nothing else: cycle-1252's audit red was exactly this shape
	// ("reviewer ran 0 time(s)", red-on-retry) and was undiagnosable afterwards
	// because the driver's own explanation was discarded. Both assertions now
	// carry stderr, so a recurrence names its cause instead of its symptom.
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d (ExitArtifactTimeout): a deliverable rewritten on every poll "+
			"tick must neither complete the phase nor exit before the wait loop — exit %d is "+
			"%s; stderr=%q",
			code, ExitArtifactTimeout, code, debounceExitDiagnosis(code), stderr)
	}
	if rev.calls < 2 {
		t.Fatalf("reviewer ran %d time(s): the loop completed before the artifact could churn, "+
			"so this test did not exercise what it claims; stderr=%q", rev.calls, stderr)
	}
}

// debounceExitDiagnosis names the exits that mean "the driver never reached the
// artifact wait loop", so a red says WHERE it died rather than only that it did.
func debounceExitDiagnosis(code int) string {
	switch code {
	case ExitOK:
		return "success — the cross-poll debounce is not reached from the wait loop"
	case ExitREPLBootTimeout:
		return "a boot failure — the REPL prompt marker never appeared, so the wait loop never ran"
	case ExitBadFlags:
		return "a launch/session-setup failure before the wait loop"
	case ExitUnknownPrompt, ExitRespondLoopGuard:
		return "an auto-respond abort on the first tick, before any review checkpoint"
	default:
		return "an unexpected early exit"
	}
}

// TestArtifactStableTicks_IsAMeaningfulWindow guards the constant itself: a
// builder can green every test above by defining artifactStableTicks = 1, which
// is arithmetically "one observation" — no window at all. Two consecutive
// identical observations, ~2s apart, is the minimum that can span an agent's
// Write→Edit gap, and it is the value the fixture-budget audit
// (TestRunTmuxREPL_ExtendNoEscalationReport, ArtifactTimeoutS=2) was sized for.
func TestArtifactStableTicks_IsAMeaningfulWindow(t *testing.T) {
	if artifactStableTicks < 2 {
		t.Fatalf("artifactStableTicks = %d — a window of fewer than 2 consecutive identical "+
			"observations is not a debounce", artifactStableTicks)
	}
	if artifactStableTicks > 3 {
		t.Fatalf("artifactStableTicks = %d — each extra tick costs ~2s on EVERY phase and "+
			"underruns the short-ArtifactTimeoutS fixtures (inbox MUST-ALSO (c))", artifactStableTicks)
	}
	// Keep the stdout twin in view: the two contracts should stay comparable.
	if stdoutIdlePolls < artifactStableTicks {
		t.Errorf("artifact window (%d) exceeds the stdout idle window (%d) — unexplained asymmetry",
			artifactStableTicks, stdoutIdlePolls)
	}
}

// TestRunTmuxREPL_ArtifactDebounceHermeticUnderAmbientFleetEnv is the
// ENVIRONMENT-invariance regression, and the reason it is pinned to the caller
// proof above rather than living in a general hygiene test: the debounce's
// gating predicate is exactly the one that kept losing runs to this.
//
// Cycles 1252 and 1254 were both FAILed at audit by the caller proof going red
// in the ACS/EGPS gate while the Builder, the Auditor and `evolve selfcheck
// build` all ran it green. Not a flake — a one-variable difference. The gate
// (internal/acsrunner/runner.go) shells `go test` with a bare exec and no
// cmd.Env, so it inherits the orchestrator's EVOLVE_FLEET=1 (internal/fleet).
// runTmuxREPL reads that key through lookupEnv, and with a nil Deps.LookupEnv
// the read reached the ambient process env: under a fleet supervisor with no
// --worktree, the CB.2 guard correctly refuses with errWorktreeRequired ->
// ExitBadFlags(10) BEFORE the artifact wait loop ever runs. 20 tests in this
// package failed that way with the variable set and 0 with it unset.
//
// The guard is right and stays untouched. What was wrong was fixtures reading
// the ambient environment at all, fixed at the SOURCE in newTestEngine
// (launch_test.go) rather than by sanitizing EVOLVE_* at yet another consumer —
// internal/core's sanitizeEnv is precisely that consumer-side workaround, and
// it names this failure in its own comment, which is why the defect survived
// to bite two more cycles.
//
// This test asserts the invariant directly: with EVOLVE_FLEET=1 exported into
// the process, the caller proof must still reach the wait loop and time out.
// It fails with exit 10 if anyone reintroduces an ambient env read here.
func TestRunTmuxREPL_ArtifactDebounceHermeticUnderAmbientFleetEnv(t *testing.T) {
	t.Setenv(ipcenv.FleetKey, "1")

	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	rev := &churningReviewer{artifact: fx.artifact, extends: 5}

	code, stderr := runTmuxOnStopReview(t, fx, tmux, rev, nil,
		Deps{ArtifactTimeoutS: 2}, "--allow-bypass", "--agent=scout")

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d (ExitArtifactTimeout) with %s=1 exported: the driver test "+
			"fixtures are reading the AMBIENT process environment again, so this suite is green "+
			"for a developer and red for the ACS/EGPS gate, which inherits the orchestrator's "+
			"fleet env (cycle-1252, cycle-1254). exit %d is %s; stderr=%q",
			code, ExitArtifactTimeout, ipcenv.FleetKey, code, debounceExitDiagnosis(code), stderr)
	}
}
