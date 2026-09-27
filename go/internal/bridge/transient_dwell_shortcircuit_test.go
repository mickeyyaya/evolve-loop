package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

// transientRedispatchDelayFloor is a floor, not the exact value: it only pins
// that the enforce shortcircuit's re-dispatch delay is meaningful (visibly
// longer than the 2s poll and the 500ms submit-verify settle).
const transientRedispatchDelayFloor = 15 * time.Second

// transientDwellRun is one real driver launch's observable outcome.
type transientDwellRun struct {
	code    int
	stderr  string
	sleeps  []time.Duration
	reviews []StopEvent
	ws      string
}

// runTransientDwell drives a REAL claude-tmux launch through Engine.LaunchArgs
// with a scripted pane sequence and an explicit RecoveryStage. It does not
// reuse runTmuxOnStopReview because that helper overwrites Deps.Sleep with a
// no-op unconditionally, and Deps.Sleep is the only seam through which the
// enforce path's re-dispatch delay is observable; sleeps are recorded, not
// slept, so the loops still iterate instantly.
func runTransientDwell(t *testing.T, fx launchFixture, paneSeq []string, stage string, timeoutS int) transientDwellRun {
	t.Helper()
	rev := &scriptedReviewer{verdicts: []ReviewVerdict{{Action: ReviewPause, Reason: "full artifact timeout reached"}}}
	var sleeps []time.Duration
	d := Deps{
		Tmux:               &fakeTmux{paneSeq: paneSeq},
		Sleep:              func(dur time.Duration) { sleeps = append(sleeps, dur) },
		CaptureBaseline:    zeroBaselineCapture,
		Reviewer:           rev,
		ArtifactTimeoutS:   timeoutS,
		ArtifactMaxExtends: 5,
		RecoveryStage:      stage,
	}
	eng := newTestEngine(d)
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(),
		fx.args("claude-tmux", "--allow-bypass", "--agent=router"), nil, &stdout, &stderr)
	return transientDwellRun{code: code, stderr: stderr.String(), sleeps: sleeps, reviews: rev.events, ws: fx.ws}
}

// longestSleep is the biggest pause the driver asked for.
func (r transientDwellRun) longestSleep() time.Duration {
	var max time.Duration
	for _, d := range r.sleeps {
		if d > max {
			max = d
		}
	}
	return max
}

// transientOutcomes reads the DURABLE interaction ledger this launch wrote
// (<workspace>/<phase>-interactions.ndjson) and returns the transient-dwell
// records. Reading the emitted artifact — not an in-memory spy — is the
// point: the soak reporter and the would/did parity check read exactly this
// file, and stderr-only evidence would leave the soak blind.
func (r transientDwellRun) transientOutcomes(t *testing.T) []map[string]any {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(r.ws, "*-interactions.ndjson"))
	if err != nil {
		t.Fatalf("glob interaction ledgers: %v", err)
	}
	var out []map[string]any
	for _, p := range paths {
		body, rerr := os.ReadFile(p)
		if rerr != nil {
			t.Fatalf("read %s: %v", p, rerr)
		}
		for _, ln := range strings.Split(string(body), "\n") {
			if strings.TrimSpace(ln) == "" {
				continue
			}
			var rec map[string]any
			if json.Unmarshal([]byte(ln), &rec) != nil {
				continue
			}
			kind, _ := rec["kind"].(string)
			if strings.Contains(kind, "transient") {
				out = append(out, rec)
			}
		}
	}
	return out
}

// resultsOf projects the ledger records onto their result field.
func resultsOf(recs []map[string]any) []string {
	var out []string
	for _, r := range recs {
		s, _ := r["result"].(string)
		out = append(out, s)
	}
	return out
}

// alternatingPanes builds a pane sequence that flips between the live 529 pane
// and a clean idle REPL, so the transient pattern is never present on two
// consecutive observations. n entries, long enough that fakeTmux never falls
// back to repeating its last value inside the wait window.
func alternatingPanes(t *testing.T, n int) []string {
	t.Helper()
	transient := livePane529(t)
	seq := make([]string, 0, n)
	for i := 0; i < n; i++ {
		if i%2 == 0 {
			seq = append(seq, transient)
		} else {
			seq = append(seq, tmuxPromptMarkerDefault)
		}
	}
	return seq
}

// --- AC-1 -----------------------------------------------------------------

func TestAutoResponder_TransientRegexResolvedAtConstruction(t *testing.T) {
	m, err := LoadManifest("claude-tmux")
	if err != nil {
		t.Fatalf("load claude-tmux manifest: %v", err)
	}
	if m.TransientRegex == "" {
		t.Fatal("claude-tmux manifest declares no transient_regex — the fixture premise is gone")
	}
	ar := newAutoResponder("claude-tmux", t.TempDir(), Deps{}, false, 0)

	fv := reflect.ValueOf(*ar).FieldByName("transientRegex")
	if !fv.IsValid() {
		t.Fatalf("RED (AC-1): autoResponder has no transientRegex field — the transient pattern is still "+
			"re-resolved on every tick (classifyTransientPane loads the manifest per call). "+
			"Mirror exhaustedRegex: resolve it in newAutoResponder. fields=%v", fieldNames(reflect.TypeOf(*ar)))
	}
	if fv.Kind() != reflect.String {
		t.Fatalf("transientRegex must be the manifest pattern string (mirroring exhaustedRegex); kind=%s", fv.Kind())
	}
	if got := fv.String(); got != m.TransientRegex {
		t.Errorf("transientRegex was not resolved from the launched CLI's manifest at construction\n  got:  %q\n  want: %q", got, m.TransientRegex)
	}
}

// fieldNames lists a struct type's fields for a diagnosable RED message.
func fieldNames(rt reflect.Type) []string {
	out := make([]string, 0, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		out = append(out, rt.Field(i).Name)
	}
	return out
}

func TestAutoResponder_TransientRegexIsFamilyAgnostic(t *testing.T) {
	for _, cli := range []string{"claude-tmux", "codex-tmux"} {
		m, err := LoadManifest(cli)
		if err != nil {
			t.Fatalf("load %s manifest: %v", cli, err)
		}
		fv := reflect.ValueOf(*newAutoResponder(cli, t.TempDir(), Deps{}, false, 0)).FieldByName("transientRegex")
		if !fv.IsValid() {
			t.Fatalf("RED (AC-1): autoResponder has no transientRegex field (cli=%s)", cli)
		}
		if fv.String() != m.TransientRegex {
			t.Errorf("%s: transientRegex=%q, want the family's own manifest pattern %q — recognition must be family-agnostic",
				cli, fv.String(), m.TransientRegex)
		}
	}
}

// --- AC-2 / AC-3 / AC-6 ---------------------------------------------------

func TestRunTmuxREPL_TransientDwell_EnforceStopsBeforeTheArtifactReviewer(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	run := runTransientDwell(t, fx, []string{livePane529(t)}, "enforce", 300)

	if run.code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d (ExitArtifactTimeout); stderr=%q", run.code, ExitArtifactTimeout, run.stderr)
	}
	if len(run.reviews) != 0 {
		t.Fatalf("RED (AC-3/AC-6): the transient pane burned the FULL 300s silence budget and reached the "+
			"artifact-timeout reviewer instead of stopping after its 60s dwell; reviews=%+v", run.reviews)
	}
}

func TestRunTmuxREPL_TransientDwell_ReusesExistingExitAndArtifacts(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	run := runTransientDwell(t, fx, []string{livePane529(t)}, "enforce", 300)

	if run.code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d — AC-2 forbids a new exit code for this path; stderr=%q", run.code, ExitArtifactTimeout, run.stderr)
	}
	summary := artifactTimeoutSummary(run.stderr)
	if summary == "" {
		t.Fatalf("RED (AC-6): the shortcircuit skipped the %q marker line — the death is no longer self-describing; stderr=%q",
			artifactTimeoutMarker, run.stderr)
	}
	if !strings.Contains(summary, "transient=true") {
		t.Errorf("the marker line must still report transient=true on the shortcircuit path\n  got: %s", summary)
	}
	if !strings.Contains(summary, "last_review=stop") {
		t.Errorf("AC-6: the dwell must set a ReviewStop verdict so the report/marker record WHY the wait ended\n  got: %s", summary)
	}

	body, err := os.ReadFile(filepath.Join(run.ws, "router-escalation-report.json"))
	if err != nil {
		t.Fatalf("RED (AC-6): no escalation report on the shortcircuit path (%v) — the operator loses the evidence "+
			"the full-timeout path already writes", err)
	}
	var rep escalationReport
	if err := json.Unmarshal(body, &rep); err != nil {
		t.Fatalf("escalation report is not valid JSON: %v\n%s", err, body)
	}
	if rep.Action != string(ReviewStop) {
		t.Errorf("escalation report action = %q, want %q (AC-6 routes through the existing ReviewStop verdict)", rep.Action, ReviewStop)
	}
	if !strings.Contains(strings.ToLower(rep.Reason), "transient") {
		t.Errorf("escalation report reason must name the transient upstream cause; got %q", rep.Reason)
	}
	if !strings.Contains(rep.FinalPane, "529") {
		t.Errorf("escalation report carries no pane evidence — a report without the deciding frame is the "+
			"cycle-286 masked-evidence class; final_pane=%q", rep.FinalPane)
	}
}

func TestRunTmuxREPL_TransientDwell_DoesNotFireBeforeSixtySeconds(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	run := runTransientDwell(t, fx, []string{livePane529(t)}, "enforce", 40)

	if len(run.reviews) == 0 {
		t.Fatalf("the shortcircuit preempted the 40s checkpoint — it fired INSIDE the 60s dwell, so a single " +
			"transient frame can kill a working phase (AC-3 requires a 60s dwell)")
	}
	if got := run.reviews[0].ElapsedS; got < 40 {
		t.Errorf("first review at elapsed=%ds, want >= 40 (the configured interval)", got)
	}
}

func TestRunTmuxREPL_TransientDwell_ResetsOnNonMatchingFrame(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	run := runTransientDwell(t, fx, alternatingPanes(t, 4000), "enforce", 300)

	if len(run.reviews) == 0 {
		t.Fatalf("RED (AC-3): an intermittent transient frame fast-failed the phase — the dwell must RESET on " +
			"every non-matching observation, so a flickering pattern never crosses 60s")
	}
	if run.code != ExitArtifactTimeout {
		t.Errorf("exit = %d, want %d (the ordinary reviewer-paused timeout)", run.code, ExitArtifactTimeout)
	}
	if d := run.longestSleep(); d >= transientRedispatchDelayFloor {
		t.Errorf("AC-7: the re-dispatch delay (%v) fired on a path the shortcircuit never took", d)
	}
}

// --- AC-4 -----------------------------------------------------------------

func TestRunTmuxREPL_TransientDwell_BusyPaneIsNeverPreempted(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	busy := livePane529(t) + "\nesc to interrupt\n"
	if !panestream.PaneBusy(busy, panestream.Profiles["claude"]) {
		t.Fatal("fixture premise gone: the busy affordance no longer reads as busy")
	}
	run := runTransientDwell(t, fx, []string{busy}, "enforce", 300)

	if len(run.reviews) == 0 {
		t.Fatalf("RED (AC-4): a BUSY pane was fast-failed — the transient dwell must carry the same busy guard " +
			"as fatalPaneVerdict, or a working agent is killed for text it merely rendered")
	}
	if d := run.longestSleep(); d >= transientRedispatchDelayFloor {
		t.Errorf("AC-7: re-dispatch delay (%v) applied although the shortcircuit was (correctly) not taken", d)
	}
}

// --- AC-5 -----------------------------------------------------------------

func TestRunTmuxREPL_TransientDwell_ShadowObservesWithoutActing(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	run := runTransientDwell(t, fx, []string{livePane529(t)}, "shadow", 300)

	if len(run.reviews) == 0 {
		t.Fatalf("RED (AC-5): shadow ACTED — the default stage must be behavior-neutral (legacy verdict decides)")
	}
	recs := run.transientOutcomes(t)
	if len(recs) == 0 {
		t.Fatalf("RED (AC-5): shadow left NO durable transient-dwell record in %s/*-interactions.ndjson — the soak "+
			"has no false-positive evidence to read (the R8.3 lesson: stderr-only evidence is invisible)", run.ws)
	}
	if got := resultsOf(recs); !contains(got, "would_fast_fail") {
		t.Errorf("shadow must record result=would_fast_fail for the would/did parity check; got %v", got)
	}
	if contains(resultsOf(recs), "fast_failed") {
		t.Errorf("shadow recorded fast_failed — that result is the enforce-stage signal only; got %v", resultsOf(recs))
	}
}

func TestRunTmuxREPL_TransientDwell_EnforceRecordsFastFailed(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	run := runTransientDwell(t, fx, []string{livePane529(t)}, "enforce", 300)

	recs := run.transientOutcomes(t)
	if len(recs) == 0 {
		t.Fatalf("RED (AC-5): the enforce shortcircuit recorded nothing in %s/*-interactions.ndjson", run.ws)
	}
	if got := resultsOf(recs); !contains(got, "fast_failed") {
		t.Errorf("enforce must record result=fast_failed; got %v", got)
	}
}

func TestRunTmuxREPL_TransientDwell_OffStageIsLegacy(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	run := runTransientDwell(t, fx, []string{livePane529(t)}, "off", 300)

	if len(run.reviews) == 0 {
		t.Fatalf("RED (AC-5): stage=off still preempted the reviewer — off must be the legacy flow exactly")
	}
	if recs := run.transientOutcomes(t); len(recs) != 0 {
		t.Errorf("stage=off must not classify (no observation on a disabled path); got %+v", recs)
	}
}

// --- AC-7 -----------------------------------------------------------------

func TestRunTmuxREPL_TransientDwell_EnforceDelaysRedispatch(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	run := runTransientDwell(t, fx, []string{livePane529(t)}, "enforce", 300)

	if run.code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d; stderr=%q", run.code, ExitArtifactTimeout, run.stderr)
	}
	if d := run.longestSleep(); d < transientRedispatchDelayFloor {
		t.Fatalf("RED (AC-7): longest pause on the transient shortcircuit path was %v, want >= %v — the re-dispatch "+
			"retries straight back into the outage it just detected", d, transientRedispatchDelayFloor)
	}
}

type cancelOnTransientCaptureTmux struct {
	fakeTmux
	pane     string
	waitTick int
	cancel   context.CancelFunc
	canceled bool
}

func (t *cancelOnTransientCaptureTmux) CapturePane(_ context.Context, _ string, _ int) (string, error) {
	if t.waitTick >= transientDwellObservations && !t.canceled {
		t.canceled = true
		t.cancel()
	}
	return t.pane, nil
}

func TestRunTmuxREPL_TransientDwell_CancelBeforeCooldownSkipsDelay(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tmux := &cancelOnTransientCaptureTmux{pane: livePane529(t), cancel: cancel}
	var sleeps []time.Duration
	deps := Deps{
		Tmux: tmux,
		Sleep: func(delay time.Duration) {
			sleeps = append(sleeps, delay)
			if delay == 2*time.Second {
				tmux.waitTick++
			}
		},
		CaptureBaseline:    zeroBaselineCapture,
		Reviewer:           &scriptedReviewer{verdicts: []ReviewVerdict{{Action: ReviewPause, Reason: "must not run"}}},
		ArtifactTimeoutS:   300,
		ArtifactMaxExtends: 5,
		RecoveryStage:      "enforce",
	}
	eng := newTestEngine(deps)
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(ctx,
		fx.args("claude-tmux", "--allow-bypass", "--agent=router"), nil, &stdout, &stderr)

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want ExitArtifactTimeout; stderr=%q", code, stderr.String())
	}
	if !tmux.canceled {
		t.Fatal("test premise failed: context was not canceled during the terminal transient observation")
	}
	for _, delay := range sleeps {
		if delay >= transientRedispatchDelayFloor {
			t.Fatalf("canceled run requested a %v cooldown; sleeps=%v", delay, sleeps)
		}
	}
}

func TestRunTmuxREPL_TransientDwell_NoDelayOnOrdinaryTimeout(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	run := runTransientDwell(t, fx, []string{tmuxPromptMarkerDefault}, "enforce", 40)

	if run.code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d; stderr=%q", run.code, ExitArtifactTimeout, run.stderr)
	}
	if len(run.reviews) == 0 {
		t.Fatal("premise gone: a silent pane must still reach the reviewer")
	}
	if d := run.longestSleep(); d >= transientRedispatchDelayFloor {
		t.Errorf("AC-7: the re-dispatch delay (%v) leaked onto the ordinary artifact-timeout path", d)
	}
}

// contains reports whether needle is in hay.
func contains(hay []string, needle string) bool {
	for _, s := range hay {
		if s == needle {
			return true
		}
	}
	return false
}

// alternatingFlakyTmux returns the 529 pane on even captures and a transport
// error on odd ones — the realistic shape of a tmux server under load.
type alternatingFlakyTmux struct {
	fakeTmux
	pane string
	n    int
}

func (a *alternatingFlakyTmux) CapturePane(context.Context, string, int) (string, error) {
	a.n++
	// Boot cleanly (the boot wait shares CapturePane), then flake every other
	// capture for the rest of the run — the artifact wait's dwell must ride
	// through the errors on the successful frames alone.
	if a.n <= 8 {
		return a.pane, nil
	}
	if a.n%2 == 1 {
		return "", errCapture
	}
	return a.pane, nil
}

func TestRunTmuxREPL_TransientDwell_CaptureErrorsDoNotResetTheDwell(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	rev := &scriptedReviewer{verdicts: []ReviewVerdict{{Action: ReviewPause, Reason: "full artifact timeout reached"}}}
	var sleeps []time.Duration
	d := Deps{
		Tmux:               &alternatingFlakyTmux{pane: livePane529(t)},
		Sleep:              func(dur time.Duration) { sleeps = append(sleeps, dur) },
		CaptureBaseline:    zeroBaselineCapture,
		Reviewer:           rev,
		ArtifactTimeoutS:   300,
		ArtifactMaxExtends: 5,
		RecoveryStage:      "enforce",
	}
	eng := newTestEngine(d)
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(),
		fx.args("claude-tmux", "--allow-bypass", "--agent=router"), nil, &stdout, &stderr)

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d; stderr=%q", code, ExitArtifactTimeout, stderr.String())
	}
	if len(rev.events) != 0 {
		t.Fatalf("the dwell never accumulated under alternating capture errors — the run burned the full "+
			"silence budget and reached the artifact-timeout reviewer; every errored frame reset the gate. reviews=%+v", rev.events)
	}
}
