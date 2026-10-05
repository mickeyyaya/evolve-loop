package bridge

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

type codexPaneState int

const (
	codexOnShell codexPaneState = iota
	codexOnUpdateMenu
	codexOnComposer
	codexExitedToShell
)

const (
	codexMenuUpdateNow       = 1
	codexMenuSkip            = 2
	codexMenuSkipUntilNext   = 3
	codexUpdateMenuOptions   = 3
	codexUpdateMenuHeader    = "  ✨ Update available! 0.153.4 -> 0.160.0\n\n  Release notes: https://github.com/openai/codex/releases/latest\n\n"
	codexUpdateMenuFooter    = "\n  Press enter to continue"
	codexVariantMenuFooter   = "\n  Press enter to confirm or esc to skip"
	codexShellPrompt         = "user@host cycle-cd3ae73e-1791 % "
	codexComposerFrame       = "╭──────────────────────────────────────────╮\n│ >_ OpenAI Codex (v0.153.4)               │\n│                                          │\n│ model:     gpt-5.6-terra   /model to change │\n╰──────────────────────────────────────────╯\n› Ask Codex to do anything\n  ? for shortcuts"
	codexBrewUpgradeFailure  = "Updating Codex via `brew upgrade --cask codex`...\nError: /opt/homebrew/Cellar is not writable.\nError: `brew upgrade --cask codex` failed with status exit status: 1\n"
	codexTrustDialogFrame    = "> You are in /private/tmp/wd\n  Do you trust the contents of this directory? Working with untrusted contents comes with higher risk of prompt injection.\n› 1. Yes, continue\n  2. No, quit\n\n  Press enter to continue"
	codexUpdateCheckOverride = "check_for_update_on_startup=false"
)

var codexUpdateMenuLabels = [codexUpdateMenuOptions + 1]string{
	"",
	"Update now (runs `brew upgrade --cask codex`)",
	"Skip",
	"Skip until next version",
}

func codexUpdateMenuFrame(cursors ...int) string {
	return codexUpdateMenuFrameWith(codexUpdateMenuFooter, cursors...)
}

func codexUpdateMenuFrameWith(footer string, cursors ...int) string {
	var b strings.Builder
	b.WriteString(codexUpdateMenuHeader)
	for option := 1; option <= codexUpdateMenuOptions; option++ {
		mark := "  "
		for _, c := range cursors {
			if c == option {
				mark = "› "
			}
		}
		fmt.Fprintf(&b, "%s%d. %s\n", mark, option, codexUpdateMenuLabels[option])
	}
	b.WriteString(footer)
	return b.String()
}

type codexUpdateMenuPane struct {
	state                 codexPaneState
	cursor                int
	navKeysToSwallow      int
	navigationIsInert     bool
	launchLine            string
	downs                 int
	digits                int
	enters                int
	entersOnUpdateNow     int
	promptsSubmitted      int
	keysBeforeReady       int
	composerText          string
	answeredMenuAbove     bool
	readyOn               codexPaneState
	readySeen             bool
	readyWithMenuAnswered bool
	startOnComposer       bool
	menuFooter            string
}

func (f *codexUpdateMenuPane) HasSession(context.Context, string) bool            { return false }
func (f *codexUpdateMenuPane) NewSession(context.Context, string, int, int) error { return nil }
func (f *codexUpdateMenuPane) LoadBuffer(context.Context, string, string) error   { return nil }
func (f *codexUpdateMenuPane) PasteBuffer(context.Context, string) error          { return nil }
func (f *codexUpdateMenuPane) KillSession(context.Context, string) error          { return nil }

func (f *codexUpdateMenuPane) SendKeys(_ context.Context, _, keys string, enter bool) error {
	if f.state == codexOnShell {
		if enter && isCodexLaunchLine(keys) {
			f.launch(keys)
		}
		return nil
	}
	if keys == "/quit" && enter && !f.readySeen {
		f.readySeen, f.readyOn, f.readyWithMenuAnswered = true, f.state, f.answeredMenuAbove
		return nil
	}
	if !f.readySeen {
		f.keysBeforeReady++
	}
	f.pressKey(keys)
	if enter {
		f.pressEnter()
	}
	return nil
}

func (f *codexUpdateMenuPane) launch(line string) {
	f.launchLine = line
	if f.startOnComposer {
		f.state = codexOnComposer
		return
	}
	f.state, f.cursor = codexOnUpdateMenu, codexMenuUpdateNow
}

func isCodexLaunchLine(keys string) bool {
	for _, field := range strings.Fields(keys) {
		if filepath.Base(field) == "codex" {
			return true
		}
	}
	return false
}

func (f *codexUpdateMenuPane) pressKey(keys string) {
	switch {
	case keys == "":
		return
	case keys == "Down":
		f.downs++
		f.navigate(func() { f.cursor = f.cursor%codexUpdateMenuOptions + 1 })
	case len(keys) == 1 && keys >= "1" && keys <= "3":
		f.digits++
		f.navigate(func() { f.choose(int(keys[0] - '0')) })
	case f.state == codexOnComposer:
		f.composerText += keys
	}
}

func (f *codexUpdateMenuPane) navigate(move func()) {
	if f.state != codexOnUpdateMenu {
		return
	}
	if f.navKeysToSwallow > 0 {
		f.navKeysToSwallow--
		return
	}
	if f.navigationIsInert {
		return
	}
	move()
}

func (f *codexUpdateMenuPane) pressEnter() {
	f.enters++
	switch f.state {
	case codexOnUpdateMenu:
		f.choose(f.cursor)
	case codexOnComposer:
		if f.composerText != "" {
			f.promptsSubmitted++
			f.composerText = ""
		}
	}
}

func (f *codexUpdateMenuPane) choose(option int) {
	if option == codexMenuUpdateNow {
		f.entersOnUpdateNow++
		f.state = codexExitedToShell
		return
	}
	f.cursor = option
	f.state, f.answeredMenuAbove = codexOnComposer, true
}

func (f *codexUpdateMenuPane) CapturePane(context.Context, string, int) (string, error) {
	switch f.state {
	case codexOnUpdateMenu:
		return codexUpdateMenuFrameWith(f.footer(), f.cursor), nil
	case codexOnComposer:
		if f.answeredMenuAbove {
			return codexUpdateMenuFrame(codexMenuSkip) + "\n" + codexComposerFrame, nil
		}
		return codexComposerFrame, nil
	case codexExitedToShell:
		return codexUpdateMenuFrame(codexMenuUpdateNow) + "\n\n" + codexBrewUpgradeFailure + codexShellPrompt, nil
	}
	return codexShellPrompt, nil
}

func (f *codexUpdateMenuPane) footer() string {
	if f.menuFooter == "" {
		return codexUpdateMenuFooter
	}
	return f.menuFooter
}

func (f *codexUpdateMenuPane) PaneCommand(context.Context, string) (string, error) {
	if f.state == codexOnShell || f.state == codexExitedToShell {
		return "zsh", nil
	}
	return "codex", nil
}

func bootCodexThroughUpdateMenu(t *testing.T, pane *codexUpdateMenuPane, realization Realization) (int, string) {
	t.Helper()
	var stderr bytes.Buffer
	deps := Deps{Tmux: pane, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil), Stderr: &stderr}
	rc, _ := BootSmokeTest(context.Background(), "codex-tmux", &Config{Workspace: t.TempDir(), Realization: realization}, deps)
	return rc, stderr.String()
}

func launchArgv(line string) []string {
	fields := strings.Fields(line)
	argv := make([]string, len(fields))
	for i, field := range fields {
		argv[i] = strings.Trim(field, "'")
	}
	return argv
}

func carriesConfigOverride(argv []string, override string) bool {
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] == "-c" && argv[i+1] == override {
			return true
		}
	}
	return false
}

func TestCodexLaunch_ArgvCarriesTheUpdateCheckOffOverride(t *testing.T) {
	pane := &codexUpdateMenuPane{startOnComposer: true}
	phaseRealization := RealizeFor("codex-tmux", LaunchIntent{ModelTier: "balanced", Permission: "bypass"})

	rc, stderr := bootCodexThroughUpdateMenu(t, pane, phaseRealization)

	if rc != ExitOK || pane.launchLine == "" {
		t.Fatalf("boot = rc %d, launch line %q; want a booted codex launch; stderr=%q", rc, pane.launchLine, stderr)
	}
	if !carriesConfigOverride(launchArgv(pane.launchLine), codexUpdateCheckOverride) {
		t.Fatalf("codex launch line %q does not carry -c %s: codex shows its update menu at startup, and its default choice runs brew", pane.launchLine, codexUpdateCheckOverride)
	}
}

func TestCodexUpdateMenu_ASwallowedDownNeverLetsEnterConfirmUpdateNow(t *testing.T) {
	pane := &codexUpdateMenuPane{navKeysToSwallow: 1}

	rc, stderr := bootCodexThroughUpdateMenu(t, pane, Realization{})

	if pane.entersOnUpdateNow != 0 {
		t.Fatalf("the update menu confirmed 'Update now' %d time(s): codex ran brew and exited to the shell; stderr=%q", pane.entersOnUpdateNow, stderr)
	}
	if rc != ExitOK || !pane.readySeen || pane.readyOn != codexOnComposer {
		t.Fatalf("boot = rc %d, ready on pane state %d (seen=%v); want ExitOK declared on the composer; stderr=%q", rc, pane.readyOn, pane.readySeen, stderr)
	}
	if pane.downs != 2 || pane.enters != 1 || pane.digits != 0 {
		t.Fatalf("keys = %d Down, %d Enter, %d digit; want 2 Down (one swallowed), 1 Enter on the observed Skip, no digit", pane.downs, pane.enters, pane.digits)
	}
}

func TestCodexUpdateMenu_ADownThatLandsBootsWithOneDownAndOneEnterOnSkip(t *testing.T) {
	pane := &codexUpdateMenuPane{}

	rc, stderr := bootCodexThroughUpdateMenu(t, pane, Realization{})

	if rc != ExitOK || !pane.readySeen || pane.readyOn != codexOnComposer || !pane.readyWithMenuAnswered {
		t.Fatalf("boot = rc %d, ready on pane state %d (seen=%v); want ExitOK declared on the composer below the answered menu; stderr=%q", rc, pane.readyOn, pane.readySeen, stderr)
	}
	if pane.downs != 1 || pane.enters != 1 || pane.digits != 0 || pane.cursor != codexMenuSkip {
		t.Fatalf("keys = %d Down, %d Enter, %d digit, chose option %d; want exactly 1 Down, 1 Enter, no digit, and Skip", pane.downs, pane.enters, pane.digits, pane.cursor)
	}
	if pane.promptsSubmitted != 0 {
		t.Fatalf("%d prompt(s) submitted to the composer during boot; the answered menu above it must not be answered again", pane.promptsSubmitted)
	}
}

func TestCodexUpdateMenu_ACursorThatNeverMovesEndsTheBootAsABootTimeoutWithoutConfirming(t *testing.T) {
	pane := &codexUpdateMenuPane{navigationIsInert: true}

	rc, stderr := bootCodexThroughUpdateMenu(t, pane, Realization{})

	if pane.enters != 0 || pane.entersOnUpdateNow != 0 {
		t.Fatalf("Enter was sent %d time(s) (%d on 'Update now') while the cursor never left 'Update now'; stderr=%q", pane.enters, pane.entersOnUpdateNow, stderr)
	}
	if rc != ExitREPLBootTimeout || !clihealth.IsBootTimeoutExitCode(rc) {
		t.Fatalf("rc = %d, want ExitREPLBootTimeout (%d), the class the boot-failure bench counts; stderr=%q", rc, ExitREPLBootTimeout, stderr)
	}
	if pane.downs != autoRespondLoopGuardLimit {
		t.Fatalf("Down sent %d times, want the loop guard's budget of %d", pane.downs, autoRespondLoopGuardLimit)
	}
	wantFail := fmt.Sprintf("[codex-tmux] FAIL: auto-respond loop guard: rule update_menu_cursor_off_skip matched more than %d times", autoRespondLoopGuardLimit)
	if !strings.Contains(stderr, wantFail) {
		t.Fatalf("boot must fail loudly naming the rule; want %q in stderr=%q", wantFail, stderr)
	}
}

func TestCodexUpdateMenu_AnAnsweredMenuLeftInScrollbackAboveTheComposerIsNeverAnsweredAgain(t *testing.T) {
	pane := &codexUpdateMenuPane{startOnComposer: true, answeredMenuAbove: true}

	rc, stderr := bootCodexThroughUpdateMenu(t, pane, Realization{})

	if rc != ExitOK || !pane.readySeen || pane.readyOn != codexOnComposer {
		t.Fatalf("boot = rc %d, ready on pane state %d (seen=%v); want ExitOK declared on the composer; stderr=%q", rc, pane.readyOn, pane.readySeen, stderr)
	}
	if pane.keysBeforeReady != 0 || pane.promptsSubmitted != 0 {
		t.Fatalf("%d key(s) sent and %d prompt(s) submitted to the composer before readiness; a menu above the composer is history, not a dialog", pane.keysBeforeReady, pane.promptsSubmitted)
	}
}

func TestCodexUpdateMenu_AMenuTheRulesCannotParseEndsTheBootAsABootTimeoutWithoutAnyKey(t *testing.T) {
	pane := &codexUpdateMenuPane{menuFooter: codexVariantMenuFooter}

	rc, stderr := bootCodexThroughUpdateMenu(t, pane, Realization{})

	if pane.readySeen || pane.keysBeforeReady != 0 || pane.entersOnUpdateNow != 0 {
		t.Fatalf("an update menu no rule parses was declared ready (seen=%v, on pane state %d) after %d key(s), %d on Update now: its cursor is the codex marker, so the prompt's Enter would confirm Update now; stderr=%q", pane.readySeen, pane.readyOn, pane.keysBeforeReady, pane.entersOnUpdateNow, stderr)
	}
	if rc != ExitREPLBootTimeout || !clihealth.IsBootTimeoutExitCode(rc) {
		t.Fatalf("rc = %d, want ExitREPLBootTimeout (%d), the class the boot-failure bench counts; stderr=%q", rc, ExitREPLBootTimeout, stderr)
	}
	wantFail := fmt.Sprintf("[codex-tmux] FAIL: auto-respond loop guard: rule update_menu_unparsed matched more than %d times", autoRespondLoopGuardLimit)
	if !strings.Contains(stderr, wantFail) {
		t.Fatalf("boot must fail loudly naming the hold rule; want %q in stderr=%q", wantFail, stderr)
	}
}

func codexUpdateMenuRules(t *testing.T) (Manifest, []ManifestPrompt, ManifestPrompt) {
	t.Helper()
	m, err := LoadManifest("codex-tmux")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	var shaped []ManifestPrompt
	var catchAll ManifestPrompt
	for _, p := range m.InteractivePrompts {
		switch {
		case p.Name == "update_menu_unparsed":
			catchAll = p
		case strings.HasPrefix(p.Name, "update_menu"):
			shaped = append(shaped, p)
		}
	}
	if len(shaped) != 2 || catchAll.Name == "" {
		t.Fatalf("want the navigation and confirm update_menu rules and the update_menu_unparsed hold, got %d shaped rule(s) and catch-all %q", len(shaped), catchAll.Name)
	}
	return m, shaped, catchAll
}

func matchingRules(pane string, rules []ManifestPrompt) []string {
	matched := []string{}
	for _, p := range rules {
		if _, rc := decideAutoRespond(pane, []ManifestPrompt{p}, map[string]int{}, false); rc != 0 {
			matched = append(matched, p.Name)
		}
	}
	return matched
}

func TestAutoRespond_CodexUpdateMenuRulesAreDisjointOnEveryMenuFrame(t *testing.T) {
	m, shaped, catchAll := codexUpdateMenuRules(t)
	frames := []struct {
		name     string
		pane     string
		wantRule string
		wantKeys string
		wantHeld bool
	}{
		{"cursor on Update now", codexUpdateMenuFrame(codexMenuUpdateNow), "update_menu_cursor_off_skip", "Down", true},
		{"cursor on Skip", codexUpdateMenuFrame(codexMenuSkip), "update_menu_cursor_on_skip", "Enter", true},
		{"cursor on Skip until next version", codexUpdateMenuFrame(codexMenuSkipUntilNext), "update_menu_cursor_off_skip", "Down", true},
		{"torn redraw, cursors on Update now and Skip", codexUpdateMenuFrame(codexMenuUpdateNow, codexMenuSkip), "update_menu_cursor_off_skip", "Down", false},
		{"torn redraw, cursors on Skip and Skip until next version", codexUpdateMenuFrame(codexMenuSkip, codexMenuSkipUntilNext), "update_menu_cursor_off_skip", "Down", false},
		{"codex 0.138 menu, cursor on Update now", cycle274CodexUpdateMenu, "update_menu_cursor_off_skip", "Down", true},
		{"a footer no shaped rule knows, cursor on Update now", codexUpdateMenuFrameWith(codexVariantMenuFooter, codexMenuUpdateNow), "", "", true},
		{"answered menu above the composer", codexUpdateMenuFrame(codexMenuSkip) + "\n" + codexComposerFrame, "", "", false},
		{"Skip just confirmed, the composer starting to draw below the menu", codexUpdateMenuFrame(codexMenuSkip) + "\n" + strings.Join(strings.Split(codexComposerFrame, "\n")[:2], "\n"), "", "", true},
		{"Update now just confirmed, the upgrade starting below the menu", codexUpdateMenuFrame(codexMenuUpdateNow) + "\n\nUpdating Codex via `brew upgrade --cask codex`...", "", "", true},
		{"Update now taken, codex exited to the shell", codexUpdateMenuFrame(codexMenuUpdateNow) + "\n\n" + codexBrewUpgradeFailure + codexShellPrompt, "", "", true},
		{"codex trust dialog", codexTrustDialogFrame, "", "", false},
		{"composer", codexComposerFrame, "", "", false},
	}
	for _, fr := range frames {
		t.Run(fr.name, func(t *testing.T) {
			want := []string{}
			if fr.wantRule != "" {
				want = []string{fr.wantRule}
			}
			if got := matchingRules(fr.pane, shaped); strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("shaped update_menu rules matching %q = %v, want %v", fr.name, got, want)
			}
			if held := len(matchingRules(fr.pane, []ManifestPrompt{catchAll})) == 1; held != fr.wantHeld {
				t.Fatalf("update_menu_unparsed matching %q = %v, want %v: it holds every live menu frame and lets go once the composer is drawn below", fr.name, held, fr.wantHeld)
			}
			if fr.wantRule == "" && !fr.wantHeld {
				return
			}
			wantAction, wantRC := "send:"+fr.wantKeys, 1
			if fr.wantRule == "" {
				wantAction, wantRC = "hold:update_menu_unparsed", 4
			}
			if a, rc := decideAutoRespond(fr.pane, m.InteractivePrompts, map[string]int{}, false); a != wantAction || rc != wantRC {
				t.Fatalf("full manifest on %q = (%q,%d), want (%s,%d): a shaped rule answers the menu it parses, and the hold takes every menu it does not", fr.name, a, rc, wantAction, wantRC)
			}
		})
	}
}

func TestAutoRespond_CodexUpdateMenuTailLinesRejectsAFooterFarBelowTheOptions(t *testing.T) {
	m, shaped, _ := codexUpdateMenuRules(t)
	farFooter := strings.Repeat("\n", 5) + codexUpdateMenuFooter
	frames := map[string]string{
		"update_menu_cursor_off_skip": codexUpdateMenuFrameWith(farFooter, codexMenuUpdateNow),
		"update_menu_cursor_on_skip":  codexUpdateMenuFrameWith(farFooter, codexMenuSkip),
	}
	for _, p := range shaped {
		pane := frames[p.Name]
		untailed := p
		untailed.TailLines = 0
		if got := matchingRules(pane, []ManifestPrompt{untailed}); len(got) != 1 {
			t.Fatalf("precondition: %s without tail_lines must match the far-footer frame, or this test no longer probes tail_lines", p.Name)
		}
		if got := matchingRules(pane, []ManifestPrompt{p}); len(got) != 0 {
			t.Fatalf("%s matched options that sit more than its tail_lines (%d) above the footer", p.Name, p.TailLines)
		}
		if a, rc := decideAutoRespond(pane, m.InteractivePrompts, map[string]int{}, false); a != "hold:update_menu_unparsed" || rc != 4 {
			t.Fatalf("full manifest on the far-footer frame for %s = (%q,%d), want (hold:update_menu_unparsed,4)", p.Name, a, rc)
		}
	}
}

func TestAutoRespond_CodexUpdateMenuHoldSkipsABusyPane(t *testing.T) {
	_, _, catchAll := codexUpdateMenuRules(t)
	unparsed := codexUpdateMenuFrameWith(codexVariantMenuFooter, codexMenuUpdateNow)

	idle, idleRC := decideAutoRespond(unparsed, []ManifestPrompt{catchAll}, map[string]int{}, false)
	busy, busyRC := decideAutoRespond(unparsed, []ManifestPrompt{catchAll}, map[string]int{}, true)

	if idle != "hold:update_menu_unparsed" || idleRC != 4 {
		t.Fatalf("idle pane = (%q,%d), want (hold:update_menu_unparsed,4)", idle, idleRC)
	}
	if busy != "noop" || busyRC != 0 {
		t.Fatalf("busy pane = (%q,%d), want noop: a working agent that prints the menu's header is quoting it, and holding would spend its loop guard", busy, busyRC)
	}
}

func TestAutoRespond_CodexUpdateMenuNavigationWinsAFrameBothRulesMatch(t *testing.T) {
	m, shaped, _ := codexUpdateMenuRules(t)
	overlap := codexUpdateMenuHeader + "› 1. Update now (runs `brew upgrade --cask codex`)\n" +
		strings.TrimPrefix(codexUpdateMenuFrame(codexMenuSkip), codexUpdateMenuHeader)
	if got := matchingRules(overlap, shaped); len(got) != 2 {
		t.Fatalf("precondition: both shaped rules must match the overlap frame, got %v, or this test no longer probes the tiebreak", got)
	}

	a, rc := decideAutoRespond(overlap, m.InteractivePrompts, map[string]int{}, false)

	if a != "send:Down" || rc != 1 {
		t.Fatalf("a frame both update_menu rules match = (%q,%d), want (send:Down,1): navigation precedes confirmation, so ambiguity never sends Enter", a, rc)
	}
}

func TestAutoRespond_CodexUpdateMenuRulesDoNotMatchThisRepositorysOwnFiles(t *testing.T) {
	t.Parallel()
	_, shaped, catchAll := codexUpdateMenuRules(t)
	files := []string{
		"codex_update_menu_verify_test.go",
		"tmux_repl_fixture_test.go",
		"adversarial_faults_test.go",
		"manifests/codex-tmux.json",
		"../../../docs/incidents/2026-10-05-codex-update-menu.md",
		"../../../docs/architecture/adr/0118-confirm-a-menu-only-on-the-observed-choice.md",
		"../../../docs/architecture/packages/internal-bridge.md",
		"../../../docs/operations/runtime-reference.md",
		"../../../docs/architecture/full-tmux-control.md",
		"../../../docs/architecture/logic-first-delivery-design.md",
		"../../../CHANGELOG.md",
	}
	requireNoRuleFiresAtAnyPaneBottom(t, append(shaped, catchAll), files)
}
