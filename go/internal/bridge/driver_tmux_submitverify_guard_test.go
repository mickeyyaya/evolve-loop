package bridge

// driver_tmux_submitverify_guard_test.go — RED contracts for the cycle-1526
// audit prescriptions that shipped unaddressed with 75a8aed9 (WARN verdict,
// red_count==0).
//
// Two holes, both about the guard being SILENT when it cannot do its job:
//
//  1. agy's promptMarker is the footer "? for shortcuts", not an input-line
//     prompt. pendingAtInputLine reads the text AFTER the last marker, so for
//     agy it reads whatever follows a footer — inert at best, and a SPURIOUS
//     re-send if that footer ever renders mid-pane. The guard has to know the
//     difference between "where the REPL said it booted" and "where the live
//     input line starts", and say so out loud when it has no input-line marker.
//
//  2. A CapturePane error inside the re-send loop returned with no log line at
//     all — a silent exit from a loop whose entire purpose is to be loud.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/repostate"
)

// Matchers for the keep-guard below. Whitespace-tolerant so gofmt realignment
// can never turn a green guard red.
var (
	emptyMarkerRe    = regexp.MustCompile(`inputLineMarker:\s*""`)
	declaredMarkerRe = regexp.MustCompile(`inputLineMarker:\s*\S`)
)

// captureErrTmux fails CapturePane from the errAfter'th call onward, so a test
// can drive the re-send loop into the error branch deterministically.
type captureErrTmux struct {
	*fakeTmux
	errAfter int // 1 = the first capture inside the loop fails
	n        int
}

func (c *captureErrTmux) CapturePane(ctx context.Context, session string, scrollback int) (string, error) {
	c.n++
	if c.errAfter > 0 && c.n >= c.errAfter {
		return "", errors.New("tmux: no server running on /tmp/tmux-501/default")
	}
	return c.fakeTmux.CapturePane(ctx, session, scrollback)
}

func submitVerifyDeps(tm TmuxController, stderr *bytes.Buffer) Deps {
	return Deps{
		Tmux:      tm,
		Sleep:     func(time.Duration) {},
		Stderr:    stderr,
		LookupEnv: mapLookup(nil),
	}.withDefaults()
}

// parkedPane renders text sitting unsubmitted at a claude-shaped input line.
func parkedPane(text string) string {
	return "● earlier scrollback\n\n" + tmuxPromptMarkerDefault + " " + text
}

const guardNudge = "Please write the deliverable to /ws/build-report.md to complete the phase."

// TestVerifySubmitted_NoInputLineMarker_IsLoudNotSilent pins prescription (2):
// a family that declares no input-line marker must not be verified silently.
// The guard sends NOTHING (an unanchored match could re-send agent text) and
// says why, so a stalled agy cycle's log shows the gap instead of implying the
// submission was checked.
func TestVerifySubmitted_NoInputLineMarker_IsLoudNotSilent(t *testing.T) {
	tm := &fakeTmux{paneSeq: []string{parkedPane(guardNudge)}}
	var stderr bytes.Buffer
	lp := tmuxLaunch{
		name:         "agy-tmux",
		session:      "s",
		promptMarker: "? for shortcuts", // a FOOTER — not where input begins
		// inputLineMarker deliberately unset: agy declares none.
	}
	got := verifySubmitted(context.Background(), submitVerifyDeps(tm, &stderr), lp,
		"[agy-tmux]", "prompt", parkedPane(guardNudge), guardNudge)

	if got.Resends != 0 {
		t.Errorf("outcome = %+v, want 0 — with no input-line marker there is nothing to anchor a match to", got)
	}
	if n := len(tm.sentSeq); n != 0 {
		t.Errorf("sent %d key(s) %v, want 0 — an unanchored re-send submits whatever the agent typed", n, tm.sentSeq)
	}
	if !strings.Contains(stderr.String(), "input-line marker") {
		t.Errorf("skipping verification must be LOUD: stderr does not explain why\ngot: %q", stderr.String())
	}

	// Load-bearing case (review M1): the pane above never contains the footer,
	// so pendingAtInputLine would return false on marker-absence alone and an
	// implementation that logs but FALLS THROUGH would still pass. Render the
	// footer mid-pane with the echo after it — the shape the file header warns
	// about — so only an actual early return keeps this at zero.
	footerPane := "● scrollback\n? for shortcuts\n" + guardNudge
	tm2 := &fakeTmux{paneSeq: []string{footerPane}}
	var stderr2 bytes.Buffer
	if n := verifySubmitted(context.Background(), submitVerifyDeps(tm2, &stderr2), lp,
		"[agy-tmux]", "prompt", footerPane, guardNudge); n.Resends != 0 {
		t.Errorf("outcome = %+v, want 0 — a footer rendered mid-pane must not be treated as an input line", n)
	}
	if n := len(tm2.sentSeq); n != 0 {
		t.Errorf("sent %d key(s) %v into a footer-anchored match, want 0", n, tm2.sentSeq)
	}
}

// TestVerifySubmitted_InputLineMarkerDrivesTheMatch pins that the match is
// anchored on inputLineMarker, NOT promptMarker. A family whose boot marker
// differs from its input-line marker must still verify correctly.
func TestVerifySubmitted_InputLineMarkerDrivesTheMatch(t *testing.T) {
	// Pending on the FIRST observation, cleared on the re-capture: exactly one
	// re-send, then the loop exits.
	tm := &fakeTmux{paneSeq: []string{"● done\n\n" + tmuxPromptMarkerDefault}}
	var stderr bytes.Buffer
	lp := tmuxLaunch{
		name:            "claude-tmux",
		session:         "s",
		promptMarker:    "boot-ready-banner", // deliberately NOT the input line
		inputLineMarker: tmuxPromptMarkerDefault,
	}
	got := verifySubmitted(context.Background(), submitVerifyDeps(tm, &stderr), lp,
		"[claude-tmux]", "nudge", parkedPane(guardNudge), guardNudge)

	if got.Resends != 1 {
		t.Errorf("outcome = %+v, want 1 — the parked input line must be re-sent once and then read clear", got)
	}
	if !strings.Contains(stderr.String(), "submit-verify") {
		t.Errorf("re-send must be loud; stderr:\n%s", stderr.String())
	}
}

// TestVerifySubmitted_CaptureErrorIsLogged pins prescription (4): the capture
// error inside the loop was swallowed with no log line, so an operator saw a
// re-send start and nothing after it.
func TestVerifySubmitted_CaptureErrorIsLogged(t *testing.T) {
	tm := &captureErrTmux{fakeTmux: &fakeTmux{paneSeq: []string{parkedPane(guardNudge)}}, errAfter: 1}
	var stderr bytes.Buffer
	lp := tmuxLaunch{
		name:            "claude-tmux",
		session:         "s",
		promptMarker:    tmuxPromptMarkerDefault,
		inputLineMarker: tmuxPromptMarkerDefault,
	}
	got := verifySubmitted(context.Background(), submitVerifyDeps(tm, &stderr), lp,
		"[claude-tmux]", "nudge", parkedPane(guardNudge), guardNudge)

	if got.Resends != 1 {
		t.Errorf("outcome = %+v, want 1 — one Enter went out before the capture failed", got)
	}
	out := stderr.String()
	if !strings.Contains(out, "capture") {
		t.Errorf("a capture failure inside the re-send loop must be logged, not swallowed\ngot:\n%s", out)
	}
	if !strings.Contains(out, "tmux: no server running") {
		t.Errorf("the underlying error must reach the operator\ngot:\n%s", out)
	}
}

// TestRealDriversDeclareInputLineMarker is a keep-guard over the tracked driver
// sources: every real tmux driver must state its input-line marker EXPLICITLY,
// so a new driver cannot inherit silent inertness by simply omitting the field.
// agy is the declared exception — it has no input-line prompt today — and the
// exception list is self-pruning: if agy ever declares one, this test fails
// until it is removed from the list.
func TestRealDriversDeclareInputLineMarker(t *testing.T) {
	// A driver whose boot marker is NOT an input-line prompt declares the field
	// empty; it must still NAME the field so the choice is visible in review.
	noInputLine := map[string]bool{"driver_agytmux.go": true}

	// The guard is textual and cannot see a marker emptied at its DEFINITION.
	// claude/codex point inputLineMarker at shared constants, so pin the shared
	// one here for free rather than leaving that hole entirely open.
	if tmuxPromptMarkerDefault == "" {
		t.Fatal("tmuxPromptMarkerDefault is empty — every driver pointing inputLineMarker at it goes inert while this guard stays green")
	}

	// Bind git-TRACKED state, not whatever sits on disk (ADR-0084 lens 5a): an
	// untracked scratch driver_*tmux.go would otherwise become a subject of this
	// guard and produce a false RED — the cd49274beab2 class.
	root, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("git rev-parse --show-toplevel: %v", err)
	}
	repoRoot := strings.TrimSpace(string(root))
	tracked, err := repostate.TrackedFiles(repoRoot, "go/internal/bridge")
	if err != nil {
		t.Fatalf("tracked files: %v", err)
	}
	var drivers []string
	for _, rel := range tracked {
		base := filepath.Base(rel)
		// Select by CONTENT, not filename: a driver named gemini_tmux_driver.go
		// would escape a `driver_*tmux.go` pattern and inherit exactly the silent
		// inertness this guard exists to prevent. Test harnesses are excluded —
		// they are not dispatch surfaces.
		if strings.HasSuffix(base, "_test.go") {
			continue
		}
		src, err := os.ReadFile(base)
		if err != nil {
			// Tracked but not on disk (staged deletion, mid-rebase): not this
			// guard's failure to report.
			t.Logf("skipping %s: %v", base, err)
			continue
		}
		if !strings.Contains(string(src), "tmuxLaunch{") {
			continue
		}
		drivers = append(drivers, base)
	}
	if len(drivers) < 4 {
		t.Fatalf("found %d driver(s) constructing tmuxLaunch %v — the guard has lost its subjects", len(drivers), drivers)
	}
	for _, d := range drivers {
		src := readFileT(t, d)
		if !strings.Contains(src, "inputLineMarker") {
			t.Errorf("%s constructs tmuxLaunch without naming inputLineMarker — submit-verify would be "+
				"silently inert for this family (cycle-1526 audit: agy's footer marker)", d)
			continue
		}
		// Whitespace-tolerant on purpose: gofmt re-pads struct-literal keys when a
		// sibling field name grows, so a single-space literal match would FALSE-RED
		// on a file nobody edited.
		declaresEmpty := emptyMarkerRe.MatchString(src) || !declaredMarkerRe.MatchString(src)
		if noInputLine[d] && !declaresEmpty {
			t.Errorf("%s is listed as having no input-line marker but now declares one — "+
				"delist it from noInputLine (self-pruning exception list)", d)
		}
		// Symmetry (review M2): without this, a driver regressing to an empty
		// marker — or to a comment that merely MENTIONS the field while the
		// literal omits it — passes green and silently joins agy on the inert
		// path. The exception list must bite in both directions.
		if !noInputLine[d] && declaresEmpty {
			t.Errorf("%s constructs tmuxLaunch with no (or an empty) inputLineMarker — submit-verify "+
				"goes inert for this family; add the marker, or list it in noInputLine with a reason", d)
		}
	}
}

// timelineRecorder captures the ORDER of the driver's tmux calls and sleeps, so
// a test can assert on the interleaving rather than on wall-clock timing.
type timelineRecorder struct {
	mu     sync.Mutex
	events []string
}

func (r *timelineRecorder) add(e string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

func (r *timelineRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

type timelineTmux struct {
	*fakeTmux
	rec *timelineRecorder
}

func (t *timelineTmux) SendKeys(ctx context.Context, session, keys string, enter bool) error {
	t.rec.add(fmt.Sprintf("send:%q|%v", keys, enter))
	return t.fakeTmux.SendKeys(ctx, session, keys, enter)
}

func (t *timelineTmux) CapturePane(ctx context.Context, session string, scrollback int) (string, error) {
	t.rec.add("capture")
	return t.fakeTmux.CapturePane(ctx, session, scrollback)
}

func (t *timelineTmux) PasteBuffer(ctx context.Context, session string) error {
	t.rec.add("paste")
	return t.fakeTmux.PasteBuffer(ctx, session)
}

// TestTmuxREPL_PromptDelivery_SettlesBeforeBaselineCapture pins prescription
// (3). The nudge site sleeps submitVerifySettle between its Enter and the
// capture that judges it; the prompt site did not — it fired the delivery Enter
// and let an unrelated capture much later serve as the first observation. A
// pane read before the REPL redraws still shows the prompt at the input line,
// so submit-verify would re-send an Enter into an already-submitted prompt:
// the double-submit this guard exists to prevent, caused by the guard itself.
func TestTmuxREPL_PromptDelivery_SettlesBeforeBaselineCapture(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	rec := &timelineRecorder{}
	tm := &timelineTmux{fakeTmux: &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}, rec: rec}
	eng := NewEngine(Deps{
		Tmux:      tm,
		Sleep:     func(d time.Duration) { rec.add("sleep:" + d.String()) },
		LookupEnv: mapLookup(nil),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var stdout, stderr bytes.Buffer
	eng.LaunchArgs(ctx, fx.args("claude-tmux", "--allow-bypass", "--agent=build", "--cycle=1526"), nil, &stdout, &stderr)

	ev := rec.snapshot()
	paste := timelineIndexOf(ev, "paste")
	if paste < 0 {
		t.Fatalf("precondition: prompt was never pasted; timeline=%v", ev)
	}
	enter := -1
	for i := paste; i < len(ev); i++ {
		if ev[i] == `send:""|true` {
			enter = i
			break
		}
	}
	if enter < 0 {
		t.Fatalf("precondition: no delivery Enter after the paste; timeline=%v", ev)
	}
	settled := false
	for i := enter + 1; i < len(ev); i++ {
		if ev[i] == "capture" {
			break // first observation reached without settling
		}
		// Pin the CONSTANT (review L2): accepting any non-zero sleep would let a
		// 1ns settle — or an unrelated pause — satisfy this contract.
		if ev[i] == "sleep:"+submitVerifySettle.String() {
			settled = true
			break
		}
	}
	if !settled {
		t.Errorf("no settle between the prompt-delivery Enter and the first capture that judges it — "+
			"a pre-redraw pane reads as still-parked and arms a spurious re-send (the nudge site at "+
			"driver_tmux_repl.go sleeps submitVerifySettle here)\ntimeline after Enter: %v", ev[enter:min(len(ev), enter+8)])
	}
}

func timelineIndexOf(ss []string, want string) int {
	for i, s := range ss {
		if s == want {
			return i
		}
	}
	return -1
}

func readFileT(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
