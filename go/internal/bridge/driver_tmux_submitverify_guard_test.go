package bridge

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

	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"github.com/mickeyyaya/evolve-loop/go/internal/repostate"
)

var (
	emptyMarkerRe    = regexp.MustCompile(`inputLineMarker:\s*""`)
	declaredMarkerRe = regexp.MustCompile(`inputLineMarker:\s*\S`)
)

type captureErrTmux struct {
	*fakeTmux
	errAfter int
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

func parkedPane(text string) string {
	return "● earlier scrollback\n\n" + tmuxPromptMarkerDefault + " " + text
}

const guardNudge = "Please write the deliverable to /ws/build-report.md to complete the phase."

func TestVerifySubmitted_NoInputLineMarker_IsLoudNotSilent(t *testing.T) {
	t.Parallel()
	tm := &fakeTmux{paneSeq: []string{parkedPane(guardNudge)}}
	var stderr bytes.Buffer
	lp := tmuxLaunch{
		name:         "nomarker-tmux",
		session:      "s",
		promptMarker: "? for shortcuts",
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

func TestVerifySubmitted_InputLineMarkerDrivesTheMatch(t *testing.T) {
	t.Parallel()
	tm := &fakeTmux{paneSeq: []string{"● done\n\n" + tmuxPromptMarkerDefault}}
	var stderr bytes.Buffer
	lp := tmuxLaunch{
		name:            "claude-tmux",
		session:         "s",
		promptMarker:    "boot-ready-banner",
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

func TestVerifySubmitted_CaptureErrorIsLogged(t *testing.T) {
	t.Parallel()
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

func TestRunTmuxREPL_InitialSubmitVerificationCaptureFailureIsLoud(t *testing.T) {
	t.Parallel()
	fx := newFixture(t, "claude-tmux", "")
	if err := os.WriteFile(fx.artifact, []byte("<!-- challenge-token: "+fx.token+" -->\nDONE\n"), 0o644); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	tmux := &captureErrTmux{
		fakeTmux: &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}},
		errAfter: 2,
	}
	eng := newTestEngine(Deps{
		Tmux:            tmux,
		Sleep:           func(time.Duration) {},
		CaptureBaseline: zeroBaselineCapture,
	})
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(),
		fx.args("claude-tmux", "--allow-bypass", "--agent=build", "--cycle=1526"), nil, &stdout, &stderr)

	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK; stderr:\n%s", code, stderr.String())
	}
	const warning = "submit-verify: prompt NOT verified \u2014 baseline capture failed"
	if got := strings.Count(stderr.String(), warning); got != 1 {
		t.Fatalf("baseline-capture warnings = %d, want exactly 1; stderr:\n%s", got, stderr.String())
	}
	var promptRecords []interaction.Outcome
	for _, outcome := range readInteractions(t, fx.ws, "build") {
		if outcome.Kind == interaction.KindSubmitVerify && strings.Contains(outcome.Payload, "site=prompt") {
			promptRecords = append(promptRecords, outcome)
		}
	}
	if len(promptRecords) != 1 || promptRecords[0].Result != interaction.ResultNotVerified {
		t.Fatalf("prompt submit-verification records = %+v, want one %q result", promptRecords, interaction.ResultNotVerified)
	}
}

func TestRealDriversDeclareInputLineMarker(t *testing.T) {
	t.Parallel()
	noInputLine := map[string]bool{}

	if tmuxPromptMarkerDefault == "" {
		t.Fatal("tmuxPromptMarkerDefault is empty — every driver pointing inputLineMarker at it goes inert while this guard stays green")
	}

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
		if strings.HasSuffix(base, "_test.go") {
			continue
		}
		src, err := os.ReadFile(base)
		if err != nil {
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
		declaresEmpty := emptyMarkerRe.MatchString(src) || !declaredMarkerRe.MatchString(src)
		if noInputLine[d] && !declaresEmpty {
			t.Errorf("%s is listed as having no input-line marker but now declares one — "+
				"delist it from noInputLine (self-pruning exception list)", d)
		}
		if !noInputLine[d] && declaresEmpty {
			t.Errorf("%s constructs tmuxLaunch with no (or an empty) inputLineMarker — submit-verify "+
				"goes inert for this family; add the marker, or list it in noInputLine with a reason", d)
		}
	}
}

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

func TestTmuxREPL_PromptDelivery_SettlesBeforeBaselineCapture(t *testing.T) {
	t.Parallel()
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
			break
		}
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
