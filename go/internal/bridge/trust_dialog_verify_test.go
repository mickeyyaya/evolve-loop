package bridge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

type trustPaneState int

const (
	paneShell trustPaneState = iota
	paneCursorOnNo
	paneCursorOnYes
	paneREPL
	paneExitedToShell
)

const (
	trustDialogHeader = " Accessing workspace:\n" +
		" /private/var/folders/11/T/evolve-looppreflight-wt-2929984512\n" +
		" Quick safety check: Is this a project you created or one you trust? (Like your own code, a well-known open source project, or work from your team). If not, take a moment to review what's in this folder first.\n" +
		" Claude Code'll be able to read, edit, and execute files here.\n" +
		" Security guide\n"
	trustDialogFooter      = " Enter to confirm · Esc to cancel"
	trustDialogCursorOnNo  = trustDialogHeader + " ❯ No, exit\n   Yes, I trust this folder\n" + trustDialogFooter
	trustDialogCursorOnYes = trustDialogHeader + "   No, exit\n ❯ Yes, I trust this folder\n" + trustDialogFooter
	claudeREPLFrame        = "❯ Try \"how do I log an error?\"\n  /private/var/folders/11/T/evolve-looppreflight-wt-2929984512 | Haiku 4.5\n  ⏸ manual mode on · ← for agents"
	paneShellPrompt        = "user@host evolve-looppreflight-wt-2929984512 % "
	paneLaunchLine         = paneShellPrompt + "claude --dangerously-skip-permissions\n"
	trustDialogTornRedraw  = trustDialogHeader + " ❯ No, exit\n ❯ Yes, I trust this folder\n" + trustDialogFooter
)

type trustDialogPane struct {
	state               trustPaneState
	downsToSwallow      int
	downIsInert         bool
	capturesBeforeMount int
	frameAfterLaunch    string
	capturesAfterLaunch int
	keysBeforeReady     int
	downs               int
	enters              int
	entersOnNo          int
	readyOn             trustPaneState
	readySeen           bool
}

func (f *trustDialogPane) dialogMounted() bool {
	return f.capturesAfterLaunch > f.capturesBeforeMount
}

func (f *trustDialogPane) HasSession(context.Context, string) bool            { return false }
func (f *trustDialogPane) NewSession(context.Context, string, int, int) error { return nil }
func (f *trustDialogPane) LoadBuffer(context.Context, string, string) error   { return nil }
func (f *trustDialogPane) PasteBuffer(context.Context, string) error          { return nil }
func (f *trustDialogPane) KillSession(context.Context, string) error          { return nil }

func (f *trustDialogPane) SendKeys(_ context.Context, _, keys string, enter bool) error {
	if f.state != paneShell && !f.readySeen && keys != "/exit" {
		f.keysBeforeReady++
	}
	switch {
	case keys == "Down" && !enter:
		f.pressDown()
	case keys == "" && enter:
		f.pressEnter()
	case keys == "/exit" && enter && !f.readySeen:
		f.readySeen, f.readyOn = true, f.state
	case enter && f.state == paneShell && isClaudeLaunchLine(keys):
		f.state = paneCursorOnNo
	}
	return nil
}

func isClaudeLaunchLine(keys string) bool {
	fields := strings.Fields(keys)
	for _, field := range fields {
		if field == "claude" {
			return true
		}
	}
	return false
}

func (f *trustDialogPane) pressDown() {
	f.downs++
	if f.state != paneCursorOnNo && f.state != paneCursorOnYes || !f.dialogMounted() {
		return
	}
	if f.downsToSwallow > 0 {
		f.downsToSwallow--
		return
	}
	if f.downIsInert {
		return
	}
	if f.state == paneCursorOnNo {
		f.state = paneCursorOnYes
		return
	}
	f.state = paneCursorOnNo
}

func (f *trustDialogPane) pressEnter() {
	f.enters++
	switch f.state {
	case paneCursorOnNo:
		f.entersOnNo++
		f.state = paneExitedToShell
	case paneCursorOnYes:
		f.state = paneREPL
	}
}

func (f *trustDialogPane) CapturePane(context.Context, string, int) (string, error) {
	if f.state == paneShell {
		return paneShellPrompt, nil
	}
	f.capturesAfterLaunch++
	if f.frameAfterLaunch != "" {
		return f.frameAfterLaunch, nil
	}
	if !f.dialogMounted() {
		return paneLaunchLine, nil
	}
	switch f.state {
	case paneCursorOnNo:
		return trustDialogCursorOnNo, nil
	case paneCursorOnYes:
		return trustDialogCursorOnYes, nil
	case paneREPL:
		return claudeREPLFrame, nil
	case paneExitedToShell:
		return trustDialogCursorOnNo + "\n" + paneShellPrompt, nil
	}
	return paneShellPrompt, nil
}

func (f *trustDialogPane) PaneCommand(context.Context, string) (string, error) {
	if f.state == paneShell || f.state == paneExitedToShell {
		return "zsh", nil
	}
	return "claude", nil
}

func bootClaudeThroughTrustDialog(t *testing.T, pane TmuxController) (int, string) {
	t.Helper()
	var stderr bytes.Buffer
	deps := Deps{Tmux: pane, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil), Stderr: &stderr}
	rc, _ := BootSmokeTest(context.Background(), "claude-tmux", &Config{Workspace: t.TempDir()}, deps)
	return rc, stderr.String()
}

func TestClaudeTrustDialog_ASwallowedDownNeverLetsEnterConfirmNo(t *testing.T) {
	pane := &trustDialogPane{downsToSwallow: 1}

	rc, stderr := bootClaudeThroughTrustDialog(t, pane)

	if pane.entersOnNo != 0 {
		t.Fatalf("Enter reached the trust dialog with the cursor on 'No, exit' %d time(s): claude exited to the shell; stderr=%q", pane.entersOnNo, stderr)
	}
	if rc != ExitOK || !pane.readySeen || pane.readyOn != paneREPL {
		t.Fatalf("boot = rc %d, ready on pane state %d (seen=%v); want ExitOK declared on the REPL; stderr=%q", rc, pane.readyOn, pane.readySeen, stderr)
	}
	if pane.downs != 2 || pane.enters != 1 {
		t.Fatalf("keys = %d Down, %d Enter; want 2 Down (one swallowed) and 1 Enter", pane.downs, pane.enters)
	}
}

func TestClaudeTrustDialog_ADownThatLandsBootsWithOneDownAndOneEnter(t *testing.T) {
	pane := &trustDialogPane{}

	rc, stderr := bootClaudeThroughTrustDialog(t, pane)

	if rc != ExitOK || !pane.readySeen || pane.readyOn != paneREPL {
		t.Fatalf("boot = rc %d, ready on pane state %d (seen=%v); want ExitOK declared on the REPL, never on the dialog; stderr=%q", rc, pane.readyOn, pane.readySeen, stderr)
	}
	if pane.downs != 1 || pane.enters != 1 {
		t.Fatalf("keys = %d Down, %d Enter; want exactly 1 Down and 1 Enter", pane.downs, pane.enters)
	}
}

func TestClaudeTrustDialog_ACursorThatNeverMovesEndsTheBootOnTheLoopGuardWithoutConfirming(t *testing.T) {
	pane := &trustDialogPane{downIsInert: true}

	rc, stderr := bootClaudeThroughTrustDialog(t, pane)

	if pane.enters != 0 {
		t.Fatalf("Enter was sent %d time(s) while the cursor never left 'No, exit'", pane.enters)
	}
	if rc != ExitREPLBootTimeout || !clihealth.IsBootTimeoutExitCode(rc) {
		t.Fatalf("rc = %d, want ExitREPLBootTimeout (%d), the class the boot-failure bench counts; stderr=%q", rc, ExitREPLBootTimeout, stderr)
	}
	if pane.downs != autoRespondLoopGuardLimit {
		t.Fatalf("Down sent %d times, want the loop guard's budget of %d", pane.downs, autoRespondLoopGuardLimit)
	}
	wantFail := fmt.Sprintf("[claude-tmux] FAIL: auto-respond loop guard: rule trust_prompt_cursor_on_no matched more than %d times", autoRespondLoopGuardLimit)
	if !strings.Contains(stderr, wantFail) {
		t.Fatalf("boot must fail loudly naming the rule; want %q in stderr=%q", wantFail, stderr)
	}
}

func TestClaudeBoot_AnEscalatePromptThatNeverClearsEndsAsABootTimeoutWithoutSendingKeys(t *testing.T) {
	pane := &trustDialogPane{frameAfterLaunch: "Welcome to Claude Code\nLogin expired. Please log in again.\n"}

	rc, stderr := bootClaudeThroughTrustDialog(t, pane)

	if !clihealth.IsBootTimeoutExitCode(rc) {
		t.Fatalf("rc = %d: a boot that never reached the REPL must be the boot-timeout class the bench counts; stderr=%q", rc, stderr)
	}
	if pane.keysBeforeReady != 0 {
		t.Fatalf("%d key(s) sent to the CLI after launch; an escalate rule sends none", pane.keysBeforeReady)
	}
	wantFail := fmt.Sprintf("[claude-tmux] FAIL: auto-respond loop guard: rule auth_recheck matched more than %d times", autoRespondLoopGuardLimit)
	if !strings.Contains(stderr, wantFail) {
		t.Fatalf("the FAIL line must count matches, not responses never sent; want %q in stderr=%q", wantFail, stderr)
	}
}

func TestClaudeTrustDialog_ADialogMountingAfterLaunchStillBootsOnYes(t *testing.T) {
	pane := &trustDialogPane{capturesBeforeMount: 1}

	rc, stderr := bootClaudeThroughTrustDialog(t, pane)

	if rc != ExitOK || pane.readyOn != paneREPL || pane.entersOnNo != 0 {
		t.Fatalf("boot = rc %d, ready on pane state %d, %d Enter(s) on No; want ExitOK on the REPL and none; stderr=%q", rc, pane.readyOn, pane.entersOnNo, stderr)
	}
}

type captureFailsOnceOnTheDialog struct {
	*trustDialogPane
	textOnFailure string
	failed        bool
}

func (f *captureFailsOnceOnTheDialog) CapturePane(ctx context.Context, session string, scrollback int) (string, error) {
	frame, _ := f.trustDialogPane.CapturePane(ctx, session, scrollback)
	if !f.failed && f.state == paneCursorOnNo && f.dialogMounted() {
		f.failed = true
		if f.textOnFailure != "" {
			frame = f.textOnFailure
		}
		return frame, errors.New("tmux capture-pane: timed out")
	}
	return frame, nil
}

var failedCaptureTexts = []struct {
	name string
	text string
}{
	{"the live dialog, cursor on No", ""},
	{"a stale frame with the cursor on Yes", trustDialogCursorOnYes},
}

type enterLostOnceOnYes struct {
	*trustDialogPane
	lost bool
}

func (f *enterLostOnceOnYes) SendKeys(ctx context.Context, session, keys string, enter bool) error {
	if keys == "" && enter && !f.lost && f.state == paneCursorOnYes {
		f.lost = true
		f.enters++
		return nil
	}
	return f.trustDialogPane.SendKeys(ctx, session, keys, enter)
}

func TestClaudeTrustDialog_AFailedCaptureOfTheDialogIsNeverReadiness(t *testing.T) {
	for _, fc := range failedCaptureTexts {
		t.Run(fc.name, func(t *testing.T) {
			pane := &trustDialogPane{}

			rc, stderr := bootClaudeThroughTrustDialog(t, &captureFailsOnceOnTheDialog{trustDialogPane: pane, textOnFailure: fc.text})

			if rc != ExitOK || pane.readyOn != paneREPL || pane.entersOnNo != 0 {
				t.Fatalf("boot = rc %d, ready on pane state %d, %d Enter(s) on No; want ExitOK declared on the REPL: a failed capture is no observation, neither ticked nor judged; stderr=%q", rc, pane.readyOn, pane.entersOnNo, stderr)
			}
		})
	}
}

func TestClaudeTrustDialog_ALostEnterOnYesIsRetriedAndBootsOnTheREPL(t *testing.T) {
	pane := &trustDialogPane{}

	rc, stderr := bootClaudeThroughTrustDialog(t, &enterLostOnceOnYes{trustDialogPane: pane})

	if rc != ExitOK || pane.readyOn != paneREPL || pane.enters != 2 || pane.entersOnNo != 0 {
		t.Fatalf("boot = rc %d, ready on pane state %d, %d Enter(s), %d on No; want ExitOK on the REPL after 2 Enters, both on Yes; stderr=%q", rc, pane.readyOn, pane.enters, pane.entersOnNo, stderr)
	}
}

func TestClaudeBoot_AnEscalateMatchOnTheREPLLeavesReadinessToTheMarker(t *testing.T) {
	pane := &trustDialogPane{frameAfterLaunch: claudeREPLFrame + "\n  Login expired · run /login"}

	rc, stderr := bootClaudeThroughTrustDialog(t, pane)

	if rc != ExitOK || !pane.readySeen {
		t.Fatalf("boot = rc %d (ready seen=%v); want ExitOK: an escalate verdict sends nothing, so the pass judges the REPL marker on its frame; stderr=%q", rc, pane.readySeen, stderr)
	}
	if pane.keysBeforeReady != 0 {
		t.Fatalf("%d key(s) sent before readiness; an escalate rule sends none", pane.keysBeforeReady)
	}
}

func newTrustDialogRecipeDriver(t *testing.T, pane *trustDialogPane) *recipeSessionDriver {
	t.Helper()
	ws := t.TempDir()
	deps := covDeps()
	deps.Tmux = pane
	return &recipeSessionDriver{
		cfg:        &Config{Workspace: ws, Agent: "recipe"},
		deps:       deps,
		session:    "recipe-trust",
		launchCmd:  "claude --dangerously-skip-permissions",
		workingDir: ws,
		marker:     "❯",
		scrollback: recipeBootScrollback,
		ar:         newAutoResponder("claude-tmux", ws, deps, false, recipeBootScrollback),
	}
}

func TestRecipeBoot_TrustDialogIsConfirmedOnYesBeforeReady(t *testing.T) {
	pane := &trustDialogPane{downsToSwallow: 1}

	err := newTrustDialogRecipeDriver(t, pane).EnsureSession(context.Background())

	if err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	if pane.entersOnNo != 0 || pane.state != paneREPL {
		t.Fatalf("recipe boot returned on pane state %d with %d Enter(s) on 'No, exit'; want the REPL and none", pane.state, pane.entersOnNo)
	}
}

func TestRecipeBoot_ADialogMountingAfterLaunchIsAnsweredBeforeTheFirstCommand(t *testing.T) {
	pane := &trustDialogPane{capturesBeforeMount: 1}
	d := newTrustDialogRecipeDriver(t, pane)

	err := d.EnsureSession(context.Background())
	sendErr := d.SendCommand(context.Background(), "/usage")

	if err != nil || sendErr != nil {
		t.Fatalf("EnsureSession = %v, SendCommand = %v", err, sendErr)
	}
	if pane.entersOnNo != 0 || pane.state != paneREPL {
		t.Fatalf("the recipe's command reached pane state %d with %d Enter(s) on 'No, exit'; want the REPL and none", pane.state, pane.entersOnNo)
	}
}

func TestRecipeBoot_AFailedCaptureOfTheDialogIsNeverReadiness(t *testing.T) {
	for _, fc := range failedCaptureTexts {
		t.Run(fc.name, func(t *testing.T) {
			pane := &trustDialogPane{}
			d := newTrustDialogRecipeDriver(t, pane)
			d.deps.Tmux = &captureFailsOnceOnTheDialog{trustDialogPane: pane, textOnFailure: fc.text}

			err := d.EnsureSession(context.Background())
			sendErr := d.SendCommand(context.Background(), "/usage")

			if err != nil || sendErr != nil {
				t.Fatalf("EnsureSession = %v, SendCommand = %v", err, sendErr)
			}
			if pane.entersOnNo != 0 || pane.state != paneREPL {
				t.Fatalf("the recipe's command reached pane state %d with %d Enter(s) on 'No, exit'; want the REPL and none", pane.state, pane.entersOnNo)
			}
		})
	}
}

func TestRecipeBoot_ACursorThatNeverMovesFailsOnTheLoopGuard(t *testing.T) {
	pane := &trustDialogPane{downIsInert: true}

	err := newTrustDialogRecipeDriver(t, pane).EnsureSession(context.Background())

	if err == nil || !strings.Contains(err.Error(), "loop guard: rule trust_prompt_cursor_on_no matched more than") {
		t.Fatalf("EnsureSession err = %v; want the loop guard naming trust_prompt_cursor_on_no", err)
	}
	if pane.enters != 0 {
		t.Fatalf("Enter was sent %d time(s) while the cursor never left 'No, exit'", pane.enters)
	}
}

func TestAutoRespond_ClaudeTrustRulesAreDisjointOnEveryDialogFrame(t *testing.T) {
	m, err := LoadManifest("claude-tmux")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	frames := []struct {
		name     string
		pane     string
		wantRule string
		wantKeys string
	}{
		{"unnumbered, cursor on No", trustDialogCursorOnNo, "trust_prompt_cursor_on_no", "Down"},
		{"unnumbered, cursor on Yes", trustDialogCursorOnYes, "trust_prompt_cursor_on_yes", "Enter"},
		{"numbered, cursor on Yes", "Quick safety check: Is this a project you created or one you trust?\n ❯ 1. Yes, I trust this folder\n   2. No, exit\n Enter to confirm · Esc to cancel", "trust_prompt", "Enter"},
		{"torn redraw, a cursor on both lines", trustDialogTornRedraw, "trust_prompt_cursor_on_no", "Down"},
		{"claude exited on No", trustDialogCursorOnNo + "\n" + paneShellPrompt, "", ""},
		{"cursor-on-No dialog quoted above the REPL", trustDialogCursorOnNo + "\n" + claudeREPLFrame, "", ""},
		{"cursor-on-Yes dialog quoted above the REPL", trustDialogCursorOnYes + "\n" + claudeREPLFrame, "", ""},
		{"REPL", claudeREPLFrame, "", ""},
	}
	for _, fr := range frames {
		t.Run(fr.name, func(t *testing.T) {
			var matched []string
			for _, p := range m.InteractivePrompts {
				if !strings.HasPrefix(p.Name, "trust_prompt") {
					continue
				}
				if _, rc := decideAutoRespond(fr.pane, []ManifestPrompt{p}, map[string]int{}, false); rc == 1 {
					matched = append(matched, p.Name)
				}
			}
			want := []string{}
			if fr.wantRule != "" {
				want = []string{fr.wantRule}
			}
			if strings.Join(matched, ",") != strings.Join(want, ",") {
				t.Fatalf("trust rules matching %q = %v, want %v", fr.name, matched, want)
			}
			if fr.wantRule == "" {
				return
			}
			if a, rc := decideAutoRespond(fr.pane, m.InteractivePrompts, map[string]int{}, false); a != "send:"+fr.wantKeys || rc != 1 {
				t.Fatalf("full manifest on %q = (%q,%d), want (send:%s,1)", fr.name, a, rc, fr.wantKeys)
			}
		})
	}
}

func TestAutoRespond_ClaudeTrustNavigationWinsAFrameBothRulesMatch(t *testing.T) {
	m, err := LoadManifest("claude-tmux")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	overlap := trustDialogHeader + " ❯ No, exit\n   No, exit\n ❯ Yes, I trust this folder\n" + trustDialogFooter
	for _, name := range []string{"trust_prompt_cursor_on_no", "trust_prompt_cursor_on_yes"} {
		var rule []ManifestPrompt
		for _, p := range m.InteractivePrompts {
			if p.Name == name {
				rule = append(rule, p)
			}
		}
		if _, rc := decideAutoRespond(overlap, rule, map[string]int{}, false); rc != 1 {
			t.Fatalf("precondition: %s must match the overlap frame, or this test no longer probes the tiebreak", name)
		}
	}

	a, rc := decideAutoRespond(overlap, m.InteractivePrompts, map[string]int{}, false)

	if a != "send:Down" || rc != 1 {
		t.Fatalf("a frame both trust rules match = (%q,%d), want (send:Down,1): navigation precedes confirmation, so ambiguity never sends Enter", a, rc)
	}
}
