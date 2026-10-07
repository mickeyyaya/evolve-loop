package bridge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	agyFooterClaudeOpus  = "? for shortcuts                                        Claude Opus 5.5 · high"
	agyFooterGeminiFlash = "? for shortcuts                                        Gemini 3.8 Flash · low"
	agyFooterNoLabel     = "? for shortcuts"
)

type modelCheckLaunch struct {
	code   int
	err    error
	tmux   *artifactOnPasteTmux
	events []signalcenter.Event
	stderr string
}

func launchAgyClaudeOver(t *testing.T, frames []string) modelCheckLaunch {
	t.Helper()
	d, ok := LookupDriver("agy-claude-tmux")
	if !ok {
		t.Fatal("agy-claude-tmux is not a registered driver")
	}
	cfg := fixtureConfig(t)
	cfg.AllowBypass, cfg.Agent, cfg.Cycle = true, "auditor", 7
	cfg.Realization = RealizeFor("agy-claude-tmux", LaunchIntent{ModelTier: "deep", Permission: "bypass"})
	center := signalcenter.New()
	tm := &artifactOnPasteTmux{FakeTmuxController: &FakeTmuxController{CaptureFrames: frames}, artifact: cfg.Artifact}
	var stderr bytes.Buffer
	deps := Deps{
		Tmux: tm, Sleep: func(time.Duration) {}, Stderr: &stderr, Signals: center,
		LookupEnv: mapLookup(map[string]string{"EVOLVE_PHASE_RECOVERY": "off"}),
	}.withDefaults()
	code, err := d.Launch(context.Background(), cfg, deps)
	center.Flush()
	return modelCheckLaunch{code: code, err: err, tmux: tm, events: center.Recent(), stderr: stderr.String()}
}

func (l modelCheckLaunch) mismatches() []signalcenter.Event {
	var out []signalcenter.Event
	for _, e := range l.events {
		if e.Code == CodeDispatchModelMismatch {
			out = append(out, e)
		}
	}
	return out
}

func agyClaudeLabelWait(t *testing.T) int {
	t.Helper()
	check, err := launchModelCheckFor("agy-claude-tmux")
	if err != nil || !check.enabled() {
		t.Fatalf("agy-claude-tmux declares no launch model check (%+v, %v)", check, err)
	}
	return check.waitS
}

func TestLaunchModelVerification_AgyClaudeBootingGeminiFailsOverWithTheMismatchSignal(t *testing.T) {
	frames := append(slices.Repeat([]string{agyFooterGeminiFlash}, agyClaudeLabelWait(t)+2), "cleanup scrollback")

	l := launchAgyClaudeOver(t, frames)

	if l.code != ExitModelMismatch || l.err != nil {
		t.Fatalf("Launch = (%d, %v), want (%d, nil): a deep seat must never run on the Gemini agy booted\n%s", l.code, l.err, ExitModelMismatch, l.stderr)
	}
	if l.tmux.PasteCount != 0 {
		t.Fatalf("the prompt was delivered %d time(s) to the wrong model", l.tmux.PasteCount)
	}
	if len(l.tmux.KilledSessions) != 1 {
		t.Fatalf("killed sessions %v, want the wrong-model session killed", l.tmux.KilledSessions)
	}
	got := l.mismatches()
	if len(got) != 1 {
		t.Fatalf("mismatch signals %d, want 1 (events %+v)", len(got), l.events)
	}
	want := map[string]string{"expected": "claude", "observed": "gemini", "label": "Gemini 3.8 Flash · low", "cli": "agy-claude-tmux", "phase": "auditor"}
	for key, value := range want {
		if got[0].Fields[key] != value {
			t.Errorf("field %s = %q, want %q (fields %v)", key, got[0].Fields[key], value, got[0].Fields)
		}
	}
	if got[0].Phase != "auditor" || got[0].Cycle != 7 || got[0].Severity != signalcenter.SeverityWarn {
		t.Errorf("event identity %+v, want phase auditor, cycle 7, WARN", got[0])
	}
	if left := len(l.tmux.CaptureFrames); left != 0 {
		t.Errorf("%d frame(s) unread: a wrong family is a mismatch only at the deadline, never on the first label", left)
	}
}

func TestLaunchModelVerification_AgyClaudeBootingClaudeDeliversThePrompt(t *testing.T) {
	l := launchAgyClaudeOver(t, slices.Repeat([]string{agyFooterClaudeOpus}, 60))

	if l.code == ExitModelMismatch || l.tmux.PasteCount != 1 {
		t.Fatalf("Launch = (%d, %v), paste count %d: the dispatched family booted, so the prompt goes out once\n%s", l.code, l.err, l.tmux.PasteCount, l.stderr)
	}
	if got := l.mismatches(); len(got) != 0 {
		t.Fatalf("a verified launch emitted %+v", got)
	}
	if !strings.Contains(l.stderr, `launch model verified: "Claude Opus 5.5 · high" is claude`) {
		t.Errorf("stderr does not record the verified label:\n%s", l.stderr)
	}
}

func TestLaunchModelVerification_ALabelThatRendersAfterTheMarkerIsStillRead(t *testing.T) {
	frames := append([]string{agyFooterNoLabel, agyFooterNoLabel, agyFooterNoLabel}, slices.Repeat([]string{agyFooterClaudeOpus}, 60)...)

	l := launchAgyClaudeOver(t, frames)

	if l.code == ExitModelMismatch || l.tmux.PasteCount != 1 {
		t.Fatalf("Launch = (%d, %v), paste count %d: the footer label renders a moment after the marker and must be waited for\n%s", l.code, l.err, l.tmux.PasteCount, l.stderr)
	}
}

func TestLaunchModelVerification_AnUnreadableLabelFailsOverAfterTheWait(t *testing.T) {
	wait := agyClaudeLabelWait(t)
	frames := append(slices.Repeat([]string{agyFooterNoLabel}, wait+2), "cleanup scrollback")

	l := launchAgyClaudeOver(t, frames)

	if l.code != ExitModelMismatch || l.tmux.PasteCount != 0 {
		t.Fatalf("Launch = (%d, %v), paste count %d: an unverifiable model is a mismatch, never a delivery\n%s", l.code, l.err, l.tmux.PasteCount, l.stderr)
	}
	if got := l.mismatches(); len(got) != 1 || got[0].Fields["observed"] != "unreadable" || got[0].Fields["label"] != "" {
		t.Fatalf("mismatch signals %+v, want one naming an unreadable label", got)
	}
	if left := len(l.tmux.CaptureFrames); left != 0 {
		t.Fatalf("%d frame(s) unread: the check polls once per second for label_wait_s before failing over", left)
	}
}

func TestLaunchModelVerification_ATargetWithoutTheRuleDeliversWhateverItBooted(t *testing.T) {
	cfg := fixtureConfig(t)
	base := &FakeTmuxController{CaptureFrames: []string{"❯", "working ❯", "working ❯", "final scrollback", "cleanup scrollback"}}
	tm := &artifactOnPasteTmux{FakeTmuxController: base, artifact: cfg.Artifact}

	code, err := runTmuxREPL(context.Background(), cfg, fixtureDeps(tm), tmuxLaunch{
		name: "claude-tmux", session: "no-model-check", launchCmd: "claude", promptMarker: "❯", bootIntervalS: 1,
	})

	if err != nil || code != ExitOK || tm.PasteCount != 1 {
		t.Fatalf("runTmuxREPL = (%d, %v), paste %d: a launch with no model check is unchanged", code, err, tm.PasteCount)
	}
}

func TestLaunchModelCheckFor_ReadsTheRuleFromTheResolvedManifest(t *testing.T) {
	check, err := launchModelCheckFor("agy-claude-tmux")
	if err != nil {
		t.Fatal(err)
	}
	if check.family != "claude" || check.waitS <= 0 || check.labelRegex == "" {
		t.Fatalf("agy-claude-tmux check %+v: want the claude family, the base's label regex and a positive wait", check)
	}
	plain, err := launchModelCheckFor("agy-tmux")
	if err != nil || plain.enabled() {
		t.Fatalf("agy-tmux check %+v (%v): the base declares no rule, so its Gemini seats are not checked", plain, err)
	}
	if _, err := launchModelCheckFor("no-such-cli"); err == nil {
		t.Fatal("an unknown target must fail loudly, never read as unchecked")
	}
}

func TestVerifiesLaunchModel_IsTheTargetDriversOwnCheck(t *testing.T) {
	for cli, want := range map[string]bool{"agy-claude-tmux": true, "agy-tmux": false, "claude-tmux": false} {
		got, err := VerifiesLaunchModel(cli)
		if err != nil || got != want {
			t.Errorf("VerifiesLaunchModel(%s) = (%v, %v), want %v", cli, got, err, want)
		}
	}
	if _, err := VerifiesLaunchModel("no-such-cli"); err == nil {
		t.Error("an unregistered target must fail loudly")
	}
}

func TestEveryManifestDeclaringALaunchModelRuleIsDrivenByAVerifier(t *testing.T) {
	entries, err := embeddedManifests.ReadDir("manifests")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		cli := strings.TrimSuffix(entry.Name(), ".json")
		m, err := LoadManifest(cli)
		if err != nil || m.LaunchModelVerification == nil {
			continue
		}
		if verified, err := VerifiesLaunchModel(cli); err != nil || !verified {
			t.Errorf("%s declares launch_model_verification but its driver does not run it (%v, %v)", cli, verified, err)
		}
	}
}

func TestParseManifest_RefusesALaunchModelRuleItCannotRun(t *testing.T) {
	const regex = `"model_label_regex": "^(?P<footer>\\? for shortcuts)?\\s{2,}(?P<model>\\S.*)$"`
	cases := map[string]string{
		"no model family":                `{"cli":"x-tmux","binary":"x",` + regex + `,"launch_model_verification":{"label_wait_s":5}}`,
		"no label regex":                 `{"cli":"x-tmux","binary":"x","model_family":"claude","launch_model_verification":{"label_wait_s":5}}`,
		"no wait":                        `{"cli":"x-tmux","binary":"x","model_family":"claude",` + regex + `,"launch_model_verification":{}}`,
		"a label regex naming no footer": `{"cli":"x-tmux","binary":"x","model_family":"claude","model_label_regex":"^(?:\\? for shortcuts)?\\s{2,}(?P<model>\\S.*)$","launch_model_verification":{"label_wait_s":5}}`,
	}
	for name, body := range cases {
		if _, err := parseManifest("x-tmux", []byte(body)); err == nil || !strings.Contains(err.Error(), "launch_model_verification") {
			t.Errorf("%s: err = %v, want a launch_model_verification refusal", name, err)
		}
	}
	ok := `{"cli":"x-tmux","binary":"x","model_family":"claude",` + regex + `,"launch_model_verification":{"label_wait_s":5}}`
	if _, err := parseManifest("x-tmux", []byte(ok)); err != nil {
		t.Errorf("a complete rule: %v", err)
	}
}

func TestLaunchModelCheckFor_AnOperatorOverrideThatDropsTheRuleTurnsTheCheckOff(t *testing.T) {
	dir := t.TempDir()
	writeManifestFile(t, dir, "agy-claude-tmux", `{"cli":"agy-claude-tmux","base":"agy-tmux","model_family":"claude"}`)
	useBridgeManifestDir(t, dir)

	check, err := launchModelCheckFor("agy-claude-tmux")

	if err != nil || check.enabled() {
		t.Fatalf("check %+v (%v): the override declares no rule", check, err)
	}
	if verified, err := VerifiesLaunchModel("agy-claude-tmux"); err != nil || verified {
		t.Fatalf("VerifiesLaunchModel = (%v, %v): the guard must see the override's missing rule", verified, err)
	}
}

func TestModelMismatchExit_WalksTheChainToClaudeCodeAndBenchesNothing(t *testing.T) {
	plan := llmroute.Plan{Candidates: []string{"agy-claude-tmux", "claude-tmux"}, Triggers: llmroute.DefaultTriggers(), Model: "deep", Tiers: []string{"deep"}}
	exits := map[string]int{"agy-claude-tmux": ExitModelMismatch, "claude-tmux": ExitOK}

	walk := llmroute.DispatchTiered(plan, func(cli, _ string) (int, error) {
		if exits[cli] != ExitOK {
			return exits[cli], errors.New("launch model mismatch")
		}
		return ExitOK, nil
	}, nil)

	if walk.Err != nil || walk.CLI != "claude-tmux" || !slices.Equal(walk.Attempts, []string{"agy-claude-tmux@deep", "claude-tmux@deep"}) {
		t.Fatalf("walk = %+v, want the mismatch to move the deep seat to Claude Code", walk)
	}
	if walk.Walled {
		t.Fatal("a model mismatch is not a quota wall")
	}
	if llmroute.ExitModelMismatch != ExitModelMismatch {
		t.Fatalf("llmroute spells the mismatch exit %d, the bridge %d: the trigger list must name the exit the bridge returns", llmroute.ExitModelMismatch, ExitModelMismatch)
	}
	if clihealth.IsBootTimeoutExitCode(ExitModelMismatch) {
		t.Fatal("the REPL booted, so a mismatch records no boot strike")
	}
}

func TestBootedModelLabel_ACancelledLaunchStopsWaitingWithoutReadingThePane(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	slept := 0
	deps := Deps{Tmux: &FakeTmuxController{}, Sleep: func(time.Duration) { slept++ }}
	lp := tmuxLaunch{name: "agy-claude-tmux", session: "cancelled"}.withModelCheck(launchModelCheck{family: "claude", labelRegex: `(?P<model>\S.*)`, waitS: 10})

	if w := lp.awaitDispatchedFamily(ctx, bootedModelRun{deps: deps}); w.verified || !w.cancelled || w.footer.label != "" || slept != 0 {
		t.Fatalf("wait %+v after %d sleep(s): a cancelled launch reads no pane and waits for nothing", w, slept)
	}
}

func modelCheckPane(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "agy-model-check", name+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestLaunchModelVerification_TheReviewersHealthyPanesAllVerify(t *testing.T) {
	for _, name := range []string{"claude-opus-footer", "toast-below-footer", "footer-extra-segment", "footer-effort-in-parens"} {
		t.Run(name, func(t *testing.T) {
			l := launchAgyClaudeOver(t, slices.Repeat([]string{modelCheckPane(t, name)}, 60))

			if l.code == ExitModelMismatch || l.tmux.PasteCount != 1 || len(l.mismatches()) != 0 {
				t.Fatalf("Launch = (%d, %v), paste %d: a healthy Claude boot must never fail over\n%s", l.code, l.err, l.tmux.PasteCount, l.stderr)
			}
		})
	}
}

func TestLaunchModelVerification_TheReviewersGeminiPaneIsAMismatch(t *testing.T) {
	frames := append(append([]string{agyFooterNoLabel}, slices.Repeat([]string{modelCheckPane(t, "gemini-pro-footer")}, agyClaudeLabelWait(t)+1)...), "cleanup scrollback")

	l := launchAgyClaudeOver(t, frames)

	if got := l.mismatches(); l.code != ExitModelMismatch || len(got) != 1 || got[0].Fields["label"] != "Gemini 3.1 Pro · high" {
		t.Fatalf("Launch = (%d, %v), mismatches %+v: a Gemini footer on agy-claude fails over\n%s", l.code, l.err, got, l.stderr)
	}
}

func TestLaunchModelVerification_TheDispatchedFamilyArrivingBeforeTheDeadlineIsVerified(t *testing.T) {
	frames := append([]string{agyFooterGeminiFlash, agyFooterGeminiFlash, agyFooterGeminiFlash}, slices.Repeat([]string{agyFooterClaudeOpus}, 60)...)

	l := launchAgyClaudeOver(t, frames)

	if l.code == ExitModelMismatch || l.tmux.PasteCount != 1 {
		t.Fatalf("Launch = (%d, %v), paste %d: agy switching to the dispatched model inside the wait is a healthy boot\n%s", l.code, l.err, l.tmux.PasteCount, l.stderr)
	}
}

func TestLaunchModelVerification_TheAutoResponderAnswersADialogDuringTheWait(t *testing.T) {
	frames := append([]string{agyFooterNoLabel, "Help us improve: rate the CLI\n" + agyFooterNoLabel}, slices.Repeat([]string{agyFooterClaudeOpus}, 60)...)

	l := launchAgyClaudeOver(t, frames)

	if l.code == ExitModelMismatch || l.tmux.PasteCount != 1 || !slices.Contains(l.tmux.SentSeq, "0|false") {
		t.Fatalf("Launch = (%d, %v), paste %d, sent %v: the feedback dialog that appears during the wait is answered with 0\n%s", l.code, l.err, l.tmux.PasteCount, l.tmux.SentSeq, l.stderr)
	}
}

func TestLaunchModelVerification_AFrameTheResponderActedOnIsNotEvidence(t *testing.T) {
	wait := agyClaudeLabelWait(t)
	dialog := "Help us improve: rate the CLI\n" + agyFooterClaudeOpus
	frames := append(append([]string{agyFooterNoLabel, dialog}, slices.Repeat([]string{agyFooterGeminiFlash}, wait)...), "cleanup scrollback")

	l := launchAgyClaudeOver(t, frames)

	if l.code != ExitModelMismatch || l.tmux.PasteCount != 0 || !slices.Contains(l.tmux.SentSeq, "0|false") {
		t.Fatalf("Launch = (%d, %v), paste %d, sent %v: a Claude footer on a frame the responder acted on is no proof; the settled footer is Gemini\n%s", l.code, l.err, l.tmux.PasteCount, l.tmux.SentSeq, l.stderr)
	}
}

type dyingAfterBootTmux struct {
	*artifactOnPasteTmux
	alive int
}

func (d *dyingAfterBootTmux) HasSession(ctx context.Context, name string) bool {
	if len(d.Events) > d.alive {
		return false
	}
	return d.artifactOnPasteTmux.HasSession(ctx, name)
}

func TestLaunchModelVerification_APaneThatDiesDuringTheWaitKeepsTheFreshSessionRetry(t *testing.T) {
	for _, named := range []bool{false, true} {
		cfg := fixtureConfig(t)
		base := &FakeTmuxController{CaptureFrames: []string{agyFooterNoLabel, agyFooterNoLabel, agyFooterNoLabel}}
		tm := &dyingAfterBootTmux{artifactOnPasteTmux: &artifactOnPasteTmux{FakeTmuxController: base, artifact: cfg.Artifact}}
		var observed []fatalPaneObservation
		deps := fixtureDeps(tm)
		deps.onFatalPane = func(o fatalPaneObservation) { observed = append(observed, o) }
		lp := tmuxLaunch{name: "agy-claude-tmux", session: "dies-in-wait", named: named, launchCmd: "agy", promptMarker: "? for shortcuts", bootIntervalS: 1}.
			withModelCheck(launchModelCheck{family: "claude", labelRegex: agyLabelRegex(t), waitS: 10})
		tm.alive = 1 << 30
		deps.Sleep = func(time.Duration) { tm.alive = len(tm.Events) }

		code, err := runTmuxREPL(context.Background(), cfg, deps, lp)

		if err != nil || code != ExitArtifactTimeout || tm.PasteCount != 0 {
			t.Fatalf("named=%v: runTmuxREPL = (%d, %v), paste %d: a REPL that died after boot is the fresh-session retry's exit 81, never 87", named, code, err, tm.PasteCount)
		}
		if len(observed) != 1 || observed[0].cause != recovery.CauseDeadShell || !observed[0].cause.SessionRecoverable() || observed[0].named != named {
			t.Fatalf("named=%v: fatal observations %+v, want one session-recoverable dead shell carrying the launch's own named flag (the engine retries only an ephemeral session)", named, observed)
		}
	}
}

type shellAfterBootTmux struct {
	*FakeTmuxController
	answered int
}

func (s *shellAfterBootTmux) PaneCommand(ctx context.Context, session string) (string, error) {
	s.answered++
	if s.answered == 1 {
		return "node", nil
	}
	return "zsh", nil
}

func TestLaunchModelVerification_ALiveSessionWhosePaneFellBackToTheShellIsADeadShell(t *testing.T) {
	cfg := fixtureConfig(t)
	tm := &shellAfterBootTmux{FakeTmuxController: &FakeTmuxController{CaptureFrames: []string{agyFooterNoLabel, "cleanup scrollback"}}}
	var observed []fatalPaneObservation
	deps := fixtureDeps(tm)
	deps.onFatalPane = func(o fatalPaneObservation) { observed = append(observed, o) }
	lp := tmuxLaunch{name: "agy-claude-tmux", session: "shell-in-wait", launchCmd: "agy", promptMarker: "? for shortcuts", bootIntervalS: 1, guardDeadShell: true}.
		withModelCheck(launchModelCheck{family: "claude", labelRegex: agyLabelRegex(t), waitS: 10})

	code, _ := runTmuxREPL(context.Background(), cfg, deps, lp)

	if code != ExitArtifactTimeout || len(observed) != 1 || observed[0].cause != recovery.CauseDeadShell {
		t.Fatalf("code %d, observations %+v: the agy process exited back to the shell while the session lived on, which is a dead shell (81), not a deadline mismatch", code, observed)
	}
}

func TestLaunchModelVerification_AResponderLoopGuardTripDuringTheWaitIsABootFailureNotAMismatch(t *testing.T) {
	dialog := "Allow agy to read the workspace?\n" + agyFooterNoLabel
	frames := append(append([]string{agyFooterNoLabel}, slices.Repeat([]string{dialog}, autoRespondLoopGuardLimit+2)...), "cleanup scrollback")

	l := launchAgyClaudeOver(t, frames)

	if l.code != ExitREPLBootTimeout || l.tmux.PasteCount != 0 || len(l.mismatches()) != 0 {
		t.Fatalf("Launch = (%d, %v), paste %d, mismatches %d: a dialog that keeps returning trips the responder's loop guard, which is a boot failure (80) with no model-mismatch signal\n%s", l.code, l.err, l.tmux.PasteCount, len(l.mismatches()), l.stderr)
	}
}

func TestLaunchModelVerification_ANamedSessionOnTheWrongModelIsKilledNotPreserved(t *testing.T) {
	cfg := fixtureConfig(t)
	frames := append(slices.Repeat([]string{agyFooterGeminiFlash}, 4), "cleanup scrollback")
	tm := &FakeTmuxController{CaptureFrames: frames}
	lp := tmuxLaunch{name: "agy-claude-tmux", session: "named-wrong-model", named: true, launchCmd: "agy", promptMarker: "? for shortcuts", bootIntervalS: 1}.
		withModelCheck(launchModelCheck{family: "claude", labelRegex: agyLabelRegex(t), waitS: 2})

	code, _ := runTmuxREPL(context.Background(), cfg, fixtureDeps(tm), lp)

	if code != ExitModelMismatch || !slices.Contains(tm.KilledSessions, "named-wrong-model") {
		t.Fatalf("code %d, killed %v: a resumable session running the wrong model must not be resumed", code, tm.KilledSessions)
	}
}

func TestLaunchModelVerification_ABootSmokeVerifiesTheModelToo(t *testing.T) {
	for name, tc := range map[string]struct {
		frames []string
		want   int
	}{
		"a Claude boot": {slices.Repeat([]string{agyFooterClaudeOpus}, 4), ExitOK},
		"a Gemini boot": {append(slices.Repeat([]string{agyFooterGeminiFlash}, agyClaudeLabelWait(t)+2), "cleanup scrollback"), ExitModelMismatch},
	} {
		t.Run(name, func(t *testing.T) {
			d, _ := LookupDriver("agy-claude-tmux")
			cfg := fixtureConfig(t)
			cfg.AllowBypass, cfg.BootOnly = true, true
			cfg.Realization = RealizeFor("agy-claude-tmux", LaunchIntent{ModelTier: "deep", Permission: "bypass"})
			tm := &FakeTmuxController{CaptureFrames: tc.frames}

			code, _ := d.Launch(context.Background(), cfg, fixtureDeps(tm))

			if code != tc.want {
				t.Fatalf("boot smoke = %d, want %d: preflight and doctor see the booted model, not only the marker", code, tc.want)
			}
		})
	}
}

func TestLaunchModelCheckFor_ExpectsTheResolvedTargetsOwnFamilyAndWait(t *testing.T) {
	dir := t.TempDir()
	writeManifestFile(t, dir, "agy-claude-tmux", `{"cli":"agy-claude-tmux","base":"agy-tmux","model_family":"gemini","launch_model_verification":{"label_wait_s":3}}`)
	useBridgeManifestDir(t, dir)

	check, err := launchModelCheckFor("agy-claude-tmux")

	if err != nil || check.family != "gemini" || check.waitS != 3 {
		t.Fatalf("check %+v (%v): the expected family and the wait are the resolved manifest's, never a constant", check, err)
	}
}

func agyLabelRegex(t *testing.T) string {
	t.Helper()
	m, err := LoadManifest("agy-tmux")
	if err != nil || m.ModelLabelRegex == "" {
		t.Fatalf("agy-tmux label regex: %v", err)
	}
	return m.ModelLabelRegex
}

type footerProbe struct {
	name     string
	frames   []string
	observed string
}

func inputBox(ruleGlyph string) string {
	rule := strings.Repeat(ruleGlyph, 40)
	return rule + "\n>\n" + rule
}

func steadyFrames(wait int, frame string) []string {
	return append(append([]string{agyFooterNoLabel}, slices.Repeat([]string{frame}, wait+1)...), slices.Repeat([]string{frame}, 60)...)
}

func footerProbes(t *testing.T) []footerProbe {
	t.Helper()
	wait := agyClaudeLabelWait(t)
	steady := func(frame string) []string { return steadyFrames(wait, frame) }
	box := inputBox("─")
	geminiBoxed := "  agy output\n" + box + "\n" + agyFooterGeminiFlash
	claudeBoxed := "  agy output\n" + box + "\n" + agyFooterClaudeOpus
	claudeAbove := "  Claude Opus 5.5 · high\n"
	claudeToast := "\n  Claude Opus 5.5 quota reached (resets in 2h)"
	transient := append(append(append([]string{agyFooterNoLabel}, slices.Repeat([]string{geminiBoxed}, 3)...), geminiBoxed+claudeToast), slices.Repeat([]string{geminiBoxed}, wait+60)...)
	rounded := "╭" + strings.Repeat("─", 38) + "╮\n│ >" + strings.Repeat(" ", 35) + "│\n╰" + strings.Repeat("─", 38) + "╯"
	nbsp := strings.Repeat(" ", 3)
	return []footerProbe{
		{"P1 a Claude-only quota toast below a Gemini footer", steady(geminiBoxed + claudeToast), "gemini"},
		{"P1b a Switched-to-Claude toast below a Gemini footer", steady(geminiBoxed + "\n  Switched to Claude Opus 5.5 (High)"), "gemini"},
		{"P1c one transient Claude-only toast frame among Gemini frames", transient, "gemini"},
		{"P2 a heavy-rule box, Claude above it, an unlabelled footer", steady(claudeAbove + inputBox("━") + "\n" + agyFooterNoLabel), "unreadable"},
		{"P2b an ASCII-rule box, Claude above it, an unlabelled footer", steady(claudeAbove + inputBox("-") + "\n" + agyFooterNoLabel), "unreadable"},
		{"P2c a rounded box, Claude above it, an unlabelled footer", steady(claudeAbove + rounded + "\n" + agyFooterNoLabel), "unreadable"},
		{"P2d a heavy-rule box, Claude above it, a Gemini footer", steady(claudeAbove + inputBox("━") + "\n" + agyFooterGeminiFlash), "gemini"},
		{"P3 no box, a Claude output line in the tail, a Gemini footer", steady("  The deep seat runs on Claude Opus 5.5 (High)\n  more output\n" + agyFooterGeminiFlash), "gemini"},
		{"P3b no box, a Claude output line in the tail, an unlabelled footer", steady("  The deep seat runs on Claude Opus 5.5 (High)\n  more output\n" + agyFooterNoLabel), "unreadable"},
		{"P4 a footer split across two lines leaves the prefix line without a label, so it is unreadable and the walk moves on", steady("  agy output\n" + box + "\n? for shortcuts\nClaude Opus 5.5 · high"), "unreadable"},
		{"P4b a footer split mid-label leaves no whole label on the prefix line, so it is unreadable and the walk moves on", steady("  agy output\n" + box + "\n? for shortcuts      Claude Opus\n  5.5 · high"), "unreadable"},
		{"P4c a Gemini footer split mid-label under Claude above the box", steady(claudeAbove + box + "\n? for shortcuts      Gemini 3.8\n  Flash · low"), "unreadable"},
		{"P5 a bullet separator agy never draws is no label the pattern reads, so it is unreadable and the walk moves on", steady("  agy output\n" + box + "\n? for shortcuts      Claude Opus 5.5 • high"), "unreadable"},
		{"P5b no-break-space padding is not the pattern's whitespace, so it is unreadable and the walk moves on", steady("  agy output\n" + box + "\n? for shortcuts" + nbsp + "Claude Opus 5.5 · high"), "unreadable"},
		{"P5c a zero-width space in the family word under a Claude toast-shaped line", steady("  agy output\n" + box + "\n  Claude Opus 5.5 (High)\n? for shortcuts      Gem​ini 3.8 Flash · low"), "unreadable"},
		{"P6 an empty pane at the deadline", steady(""), "unreadable"},
		{"P7 a bare Opus label with no family word", steady("  agy output\n" + box + "\n? for shortcuts      Opus 5.5 · high"), ""},
		{"P8 a healthy Claude footer, a toast naming both families below it", steady(claudeBoxed + "\n  Claude Opus 5.5 ready (Gemini fallback off)"), ""},
		{"P9 an ANSI-coloured healthy Claude footer", steady("  agy output\n" + box + "\n\x1b[2m? for shortcuts\x1b[0m      \x1b[36mClaude Opus 5.5\x1b[0m · high"), ""},
		{"P10 a healthy Claude footer, a Gemini-only toast below it", steady(claudeBoxed + "\n  Gemini 3.8 Flash is ready (press tab)"), ""},
		{"P11 a GPT-OSS footer", steady("  agy output\n" + box + "\n? for shortcuts      GPT-OSS 120B (Medium)"), "gpt"},
		{"a toast naming both families above a Gemini footer", steady("  Using Gemini 3.1 Pro (Claude Opus 5.5 unavailable)\n" + agyFooterGeminiFlash), "gemini"},
		{"a footer naming two families", steady("? for shortcuts      Gemini 3.1 Pro (Claude Opus 5.5 unavailable)"), "ambiguous"},
		{"Claude above the input box over an unlabelled footer", steady(claudeAbove + box + "\n" + agyFooterNoLabel), "unreadable"},
		{"a stale Claude footer above the box, a Gemini footer at the bottom", steady(agyFooterClaudeOpus + "\n" + box + "\n" + agyFooterGeminiFlash), "gemini"},
		{"a stale Gemini footer above the box, a Claude footer at the bottom", steady(agyFooterGeminiFlash + "\n" + box + "\n" + agyFooterClaudeOpus), ""},
		{"a stale Claude footer above the box, an unlabelled footer at the bottom", steady(agyFooterClaudeOpus + "\n" + box + "\n" + agyFooterNoLabel), "unreadable"},
	}
}

func TestLaunchModelVerification_OnlyTheLabelOnTheFooterPrefixLineDecides(t *testing.T) {
	for _, p := range footerProbes(t) {
		t.Run(p.name, func(t *testing.T) {
			l := launchAgyClaudeOver(t, p.frames)

			got := l.mismatches()
			if p.observed == "" {
				if l.code == ExitModelMismatch || l.tmux.PasteCount != 1 || len(got) != 0 {
					t.Fatalf("Launch = (%d, %v), paste %d, mismatches %+v: the Claude label on the footer-prefix line verifies, whatever any other line names\n%s", l.code, l.err, l.tmux.PasteCount, got, l.stderr)
				}
				return
			}
			if l.code != ExitModelMismatch || l.tmux.PasteCount != 0 || len(got) != 1 || got[0].Fields["observed"] != p.observed {
				t.Fatalf("Launch = (%d, %v), paste %d, mismatches %+v: want a mismatch observing %s; only the label on the footer-prefix line counts, and no prefix line is unreadable\n%s", l.code, l.err, l.tmux.PasteCount, got, p.observed, l.stderr)
			}
		})
	}
}

func TestLaunchModelVerification_TheMismatchKeepsTheLastLabelledFooter(t *testing.T) {
	wait := agyClaudeLabelWait(t)
	frames := append(append(append([]string{agyFooterNoLabel}, slices.Repeat([]string{agyFooterGeminiFlash}, 3)...), slices.Repeat([]string{agyFooterNoLabel}, wait+2)...), "cleanup scrollback")

	l := launchAgyClaudeOver(t, frames)

	if got := l.mismatches(); len(got) != 1 || got[0].Fields["label"] != "Gemini 3.8 Flash · low" || got[0].Fields["observed"] != "gemini" {
		t.Fatalf("mismatches %+v: a footer whose label went blank before the deadline still reports the last label it showed", got)
	}
}

func TestLaunchModelVerification_AFooterNamingTwoFamiliesIsReportedAmbiguous(t *testing.T) {
	footer := "? for shortcuts      Gemini 3.1 Pro (Claude Opus 5.5 unavailable)"
	frames := append(append([]string{agyFooterNoLabel}, slices.Repeat([]string{footer}, agyClaudeLabelWait(t)+1)...), "cleanup scrollback")

	l := launchAgyClaudeOver(t, frames)

	if got := l.mismatches(); len(got) != 1 || got[0].Fields["observed"] != "ambiguous" || got[0].Fields["label"] != "Gemini 3.1 Pro (Claude Opus 5.5 unavailable)" {
		t.Fatalf("mismatches %+v: a footer label naming two families is reported as ambiguous, never as the family checked first", got)
	}
}

func TestLaunchModelVerification_ACancelledWaitExitsAsACancelledLaunchWithoutAMismatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	center := signalcenter.New()
	var stderr bytes.Buffer
	deps := Deps{Tmux: &FakeTmuxController{}, Sleep: func(time.Duration) {}, Stderr: &stderr, Signals: center}
	lp := tmuxLaunch{name: "agy-claude-tmux", session: "cancelled"}.withModelCheck(launchModelCheck{family: "claude", labelRegex: agyLabelRegex(t), waitS: 10})

	code := lp.verifyBootedModel(ctx, bootedModelRun{cfg: fixtureConfig(t), deps: deps})
	center.Flush()

	var mismatches int
	for _, e := range center.Recent() {
		if e.Code == CodeDispatchModelMismatch {
			mismatches++
		}
	}
	if code != ExitArtifactTimeout || mismatches != 0 || !strings.Contains(stderr.String(), artifactTimeoutMarker+"cause="+string(artifactTimeoutContextCancelled)) {
		t.Fatalf("code %d, mismatch signals %d, stderr %q: a cancelled launch exits 81 as the completion wait does on a cancel, never as a model mismatch", code, mismatches, stderr.String())
	}
}

func footerUnderToasts(footer string, toasts int) string {
	lines := []string{"  agy output", footer}
	for i := range toasts {
		lines = append(lines, fmt.Sprintf("  ✓ toast %d", i+1))
	}
	return strings.Join(lines, "\n")
}

func TestLaunchModelVerification_TheFooterIsReadUpToSixLinesAboveTheBottom(t *testing.T) {
	wait := agyClaudeLabelWait(t)
	for _, tc := range []struct {
		linesUp int
		want    int
	}{{3, ExitOK}, {6, ExitOK}, {7, ExitModelMismatch}} {
		t.Run(fmt.Sprintf("footer %d lines up", tc.linesUp), func(t *testing.T) {
			frame := footerUnderToasts(agyFooterClaudeOpus, tc.linesUp-1)
			frames := append(append([]string{agyFooterNoLabel}, slices.Repeat([]string{frame}, wait+1)...), slices.Repeat([]string{agyFooterClaudeOpus}, 60)...)

			l := launchAgyClaudeOver(t, frames)

			if verified := l.code != ExitModelMismatch && l.tmux.PasteCount == 1; verified != (tc.want == ExitOK) {
				t.Fatalf("Launch = (%d, %v), paste %d: a footer %d non-blank lines above the bottom is read only within the six-line tail", l.code, l.err, l.tmux.PasteCount, tc.linesUp)
			}
		})
	}
}

func TestLaunchModelVerification_TheMismatchReportsTheLastFooterRead(t *testing.T) {
	wait := agyClaudeLabelWait(t)
	geminiPro := "? for shortcuts      Gemini 3.1 Pro · high"
	frames := append(append(append([]string{agyFooterNoLabel}, slices.Repeat([]string{agyFooterGeminiFlash}, 3)...), slices.Repeat([]string{geminiPro}, wait-2)...), "cleanup scrollback")

	l := launchAgyClaudeOver(t, frames)

	if got := l.mismatches(); len(got) != 1 || got[0].Fields["label"] != "Gemini 3.1 Pro · high" || got[0].Fields["observed"] != "gemini" {
		t.Fatalf("mismatches %+v: the signal names the footer agy settled on at the deadline, not the first one it showed", got)
	}
}

func TestLaunchModelVerification_ALabelShapedToastUnderTheFooterIsSkipped(t *testing.T) {
	frame := agyFooterClaudeOpus + "\n  Saved the session (2 files)"

	l := launchAgyClaudeOver(t, slices.Repeat([]string{frame}, 60))

	if l.code == ExitModelMismatch || l.tmux.PasteCount != 1 {
		t.Fatalf("Launch = (%d, %v), paste %d: a toast the label pattern matches but that names no model family is not the footer; the Claude footer above it decides\n%s", l.code, l.err, l.tmux.PasteCount, l.stderr)
	}
}
