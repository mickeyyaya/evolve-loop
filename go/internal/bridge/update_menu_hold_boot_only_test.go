package bridge

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

const codexIdlePaneQuotingTheMenuHeader = "• Here is what the incident says codex draws at startup:\n\n" +
	"  Update available! 0.153.4 -> 0.160.0\n\n" +
	"  Its default choice runs brew upgrade; I have written the summary.\n\n" +
	"› Implement {feature}\n\n" +
	"  ? for shortcuts                                  100% context left"

const (
	holdBootOnlyTestDeadline        = 30 * time.Second
	reviewIntervalPastTheLoopGuardS = 4 * (autoRespondLoopGuardLimit + 1)
)

func runPhaseConfig(t *testing.T) *Config {
	t.Helper()
	cfg := fixtureConfig(t)
	cfg.ArtifactTimeoutS = reviewIntervalPastTheLoopGuardS
	return cfg
}

func runPhaseDeps(tm TmuxController, stderr *bytes.Buffer) Deps {
	return Deps{Tmux: tm, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil), Stderr: stderr, CaptureBaseline: zeroBaselineCapture}.withDefaults()
}

func requireRunReachedItsNudge(t *testing.T, code int, stderr string) {
	t.Helper()
	if code == ExitRespondLoopGuard || strings.Contains(stderr, "loop guard tripped") {
		t.Fatalf("the run was abandoned on the loop guard (exit %d): the update-menu hold is a boot rule and must not count an idle pane that quotes the header; stderr=%q", code, stderr)
	}
	if code != ExitArtifactTimeout || !strings.Contains(stderr, "sent one-shot nudge") {
		t.Fatalf("exit %d; want the idle run to reach its one-shot nudge and end ExitArtifactTimeout (%d); stderr=%q", code, ExitArtifactTimeout, stderr)
	}
}

func TestCodexRunPhase_AnIdlePaneQuotingTheUpdateMenuHeaderNeverTripsTheLoopGuard(t *testing.T) {
	cfg := runPhaseConfig(t)
	cfg.CLI, cfg.AllowBypass = "codex-tmux", true
	pane := &fakeTmux{paneSeq: []string{codexComposerFrame, codexIdlePaneQuotingTheMenuHeader}}
	var stderr bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), holdBootOnlyTestDeadline)
	defer cancel()

	code, _ := codexTmuxDriver{}.Launch(ctx, cfg, runPhaseDeps(pane, &stderr))

	requireRunReachedItsNudge(t, code, stderr.String())
}

func TestCodexRunPhase_AResumedNamedSessionHasNoBootSoNoHold(t *testing.T) {
	cfg := runPhaseConfig(t)
	pane := &fakeTmux{existing: map[string]bool{"codex-named": true}, paneSeq: []string{codexIdlePaneQuotingTheMenuHeader}}
	var stderr bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), holdBootOnlyTestDeadline)
	defer cancel()

	code, _ := runTmuxREPL(ctx, cfg, runPhaseDeps(pane, &stderr), tmuxLaunch{
		name: "codex-tmux", session: "codex-named", named: true, launchCmd: "codex",
		promptMarker: "›", inputLineMarker: "›", bootScrollback: 200, bootIntervalS: 2, tickDuringBoot: true,
	})

	requireRunReachedItsNudge(t, code, stderr.String())
}

func TestRecipeBoot_TheUpdateMenuHoldEndsWithTheBoot(t *testing.T) {
	cases := []struct {
		name string
		pane *fakeTmux
	}{
		{"a fresh boot that reached the composer", &fakeTmux{paneSeq: []string{codexComposerFrame, codexIdlePaneQuotingTheMenuHeader}}},
		{"an attach to a running session", &fakeTmux{existing: map[string]bool{"recipe-codex": true}, paneSeq: []string{codexIdlePaneQuotingTheMenuHeader}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			deps := covDeps()
			deps.Tmux = tc.pane
			d := &recipeSessionDriver{
				cfg: &Config{Workspace: ws, Agent: "recipe"}, deps: deps, session: "recipe-codex",
				launchCmd: "codex", workingDir: ws, marker: "›", scrollback: recipeBootScrollback,
				ar: newAutoResponder("codex-tmux", ws, deps, false, recipeBootScrollback),
			}
			if err := d.EnsureSession(context.Background()); err != nil {
				t.Fatalf("EnsureSession: %v", err)
			}

			for step := 1; step <= autoRespondLoopGuardLimit+2; step++ {
				if abandon, _ := d.AutoRespond(context.Background()); abandon {
					t.Fatalf("recipe step %d abandoned on an idle pane quoting the update-menu header: the hold is a boot rule", step)
				}
			}
		})
	}
}

func TestAutoRespond_CodexUpdateMenuHoldNeverShadowsALaterRule(t *testing.T) {
	m, err := LoadManifest("codex-tmux")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	cursorlessMenuAboveTrustDialog := codexUpdateMenuFrame() + "\n\n" + codexTrustDialogFrame

	a, rc := decideAutoRespond(cursorlessMenuAboveTrustDialog, m.InteractivePrompts, map[string]int{}, false)

	if a != "send:1,Enter" || rc != 1 {
		t.Fatalf("a cursorless update menu above codex's trust dialog = (%q,%d), want trust_prompt's (send:1,Enter,1): the hold is the manifest's last resort, never ahead of a rule that can answer", a, rc)
	}
	if last := m.InteractivePrompts[len(m.InteractivePrompts)-1]; last.Name != "update_menu_unparsed" {
		t.Fatalf("the last codex-tmux rule is %s; update_menu_unparsed must come last so it shadows no rule", last.Name)
	}
}
