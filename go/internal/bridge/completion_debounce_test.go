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

func TestArtifactDetector_ReadyOnlyAfterCrossPollStability(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	d := newArtifactDetectorAt(ws, canonical)

	if ready, _, note, err := d.poll(context.Background()); ready || err != nil || note != "" {
		t.Fatalf("absent artifact: got (ready=%v, note=%q, err=%v), want (false, \"\", nil)", ready, note, err)
	}

	writeArtifact(t, canonical, "# report\n\nDONE\n", fixedMTime)

	ready, _, _, err := d.poll(context.Background())
	if err != nil {
		t.Fatalf("first sighting: unexpected error %v", err)
	}
	if ready {
		t.Fatal("first sighting completed the phase — the cross-poll debounce is absent; " +
			"a mid-Write→Edit deliverable is accepted exactly here (cycle-1198)")
	}

	got, note := pollUntilReady(t, d, artifactStableTicks+2, nil)
	if got < 0 {
		t.Fatalf("a file that never changed again was never accepted within %d further polls — "+
			"the debounce must SETTLE, not stall", artifactStableTicks+2)
	}
	if !strings.Contains(note, "appeared") {
		t.Errorf("completion note = %q, want it to mention 'appeared' (operator-facing log line preserved)", note)
	}
}

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

func TestArtifactDetector_CtxCancelledShortCircuitsDebounce(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "report.md")
	d := newArtifactDetectorAt(ws, canonical)
	writeArtifact(t, canonical, "# report\n\nDONE\n", fixedMTime)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// A brand-new detector with no stability history at all: the cancelled ctx
	// must still make this single poll authoritative.
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

	empty := t.TempDir()
	d2 := newArtifactDetectorAt(empty, filepath.Join(empty, "report.md"))
	if ready, _, _, err := d2.poll(ctx); ready || err != nil {
		t.Fatalf("cancelled ctx with NO artifact: got (ready=%v, err=%v), want (false, nil) — "+
			"an unfinished session must still report a timeout", ready, err)
	}
}

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

// Drives the real driver (Engine.LaunchArgs → runTmuxREPL → detector.poll)
// rather than the detector in isolation, so a debounce implemented on a
// struct nothing reaches cannot pass it.
func TestRunTmuxREPL_ArtifactDebounceWiredIntoWaitLoop(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	rev := &churningReviewer{artifact: fx.artifact, extends: 5}

	code, stderr := runTmuxOnStopReview(t, fx, tmux, rev, nil,
		Deps{ArtifactTimeoutS: 2}, "--allow-bypass", "--agent=scout")

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

// Guards the constant directly: every test above could pass with
// artifactStableTicks = 1, which is arithmetically "one observation" — no
// window at all.
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

// With EVOLVE_FLEET=1 exported into the process, the caller proof must still
// reach the wait loop and time out rather than refuse early on
// errWorktreeRequired: fixtures must never read the ambient process
// environment.
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
