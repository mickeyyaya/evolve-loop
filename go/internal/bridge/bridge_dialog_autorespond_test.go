// bridge_dialog_autorespond_test.go — cycle-245 task `bridge-dialog-auto-respond` (RED).
//
// Migration step 6 (carryover `bridge-weak-signal-profiles`, the "biggest
// latency lever ~10min/cycle"): two weak-signal gaps in the tmux bridge.
//
//  1. agy's end-of-session rating dialog ("Rate this response" / "How helpful
//     was this session") has NO rule in agy-tmux.json's interactive_prompts,
//     so the auto-responder noops and the run burns the full artifact-wait
//     window before failing. Contract: the dialog must auto-respond (some key
//     sequence is sent; the run is not stalled by a survey).
//  2. When the artifact-wait review checkpoint finds the agent IDLE with the
//     artifact missing, the driver gives up straight into the
//     ExitArtifactTimeout relaunch path. Contract: before giving up, send
//     EXACTLY ONE in-pane nudge naming the artifact path (the agent often
//     finished the work and just forgot the Write); only on a subsequent
//     idle checkpoint does the existing timeout path proceed.
//
// RED note: compiles against existing API, fails at RUNTIME today —
// the rating pane noops and zero nudges are delivered. Builder makes these
// GREEN via (a) a new interactive_prompts rule in
// go/internal/bridge/manifests/agy-tmux.json (config-only; policy
// auto_respond) and (b) a one-shot nudge guard in driver_tmux_repl.go's
// review-checkpoint path. DO NOT modify this file.
package bridge

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// --- 1. agy session-rating dialog -----------------------------------------

// TestAgyTmuxManifest_SessionRating_AutoResponds drives the REAL embedded
// agy-tmux manifest through the production decision engine: a pane showing
// agy's end-of-session rating dialog must produce an auto-response (keys
// sent), not a noop that stalls the run. RED today: no rule matches → noop.
//
// Deprecated: TestAgyTmuxManifest_SessionRating_AutoResponds
func TestAgyTmuxManifest_SessionRating_AutoResponds(t *testing.T) {
	m, err := LoadManifest("agy-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(agy-tmux): %v", err)
	}
	panes := []struct {
		name string
		pane string
	}{
		{name: "rate_this_response", pane: "│ Rate this response\n│ 1  2  3  4  5\n? for shortcuts"},
		{name: "how_helpful_session", pane: "How helpful was this session?\n(press a number)\n? for shortcuts"},
	}
	for _, tc := range panes {
		t.Run(tc.name, func(t *testing.T) {
			action, rc := decideAutoRespond(tc.pane, m.InteractivePrompts, map[string]int{}, false)
			if rc != 1 || !strings.HasPrefix(action, "send:") {
				t.Errorf("rating dialog must auto-respond; got action=%q rc=%d (want send:* rc=1)", action, rc)
			}
			if keys := strings.TrimPrefix(action, "send:"); strings.TrimSpace(keys) == "" {
				t.Errorf("auto-response must carry a non-empty key sequence; got %q", action)
			}
		})
	}
}

// TestAgyTmuxManifest_SessionRating_NoFalsePositives — the new rule must not
// hijack ordinary working output (negative axis), and the pre-existing
// escalation rules must keep escalating (the rating rule must not shadow
// auth/rate-limit panes, which contain no rating text).
func TestAgyTmuxManifest_SessionRating_NoFalsePositives(t *testing.T) {
	m, err := LoadManifest("agy-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(agy-tmux): %v", err)
	}
	t.Run("normal_output_noops", func(t *testing.T) {
		pane := "✦ Writing scout-report.md to the workspace…\ndone.\n? for shortcuts"
		action, rc := decideAutoRespond(pane, m.InteractivePrompts, map[string]int{}, false)
		if rc != 0 {
			t.Errorf("ordinary working pane must not trigger any rule; got action=%q rc=%d", action, rc)
		}
	})
	t.Run("auth_prompt_still_escalates", func(t *testing.T) {
		// Regression pin (pre-existing GREEN): adding the rating rule must not
		// reorder/shadow the escalate-class rules.
		action, rc := decideAutoRespond("Please log in to continue", m.InteractivePrompts, map[string]int{}, false)
		if rc != 85 {
			t.Errorf("auth pane must still escalate; got action=%q rc=%d", action, rc)
		}
	})
}

// --- 2. idle-artifact one-shot nudge ---------------------------------------

// nudgeRecordingTmux extends the scripted fakeTmux to also record the CONTENT
// delivered in-pane via the paste path (LoadBuffer reads the scratch/prompt
// file at call time). The nudge contract is channel-agnostic: a nudge counts
// whether the driver pastes it or send-keys it.
type nudgeRecordingTmux struct {
	*fakeTmux
	pastes []string
}

func (n *nudgeRecordingTmux) LoadBuffer(ctx context.Context, session, file string) error {
	if b, err := os.ReadFile(file); err == nil {
		n.pastes = append(n.pastes, string(b))
	}
	return n.fakeTmux.LoadBuffer(ctx, session, file)
}

// deliveriesNaming counts in-pane deliveries (pasted buffers + sent key
// strings) that mention sub — used with the absolute artifact path, which the
// fixture prompt and the launch command line never contain.
func (n *nudgeRecordingTmux) deliveriesNaming(sub string) int {
	count := 0
	for _, p := range n.pastes {
		if strings.Contains(p, sub) {
			count++
		}
	}
	for _, k := range n.sentKeys {
		if strings.Contains(k, sub) {
			count++
		}
	}
	return count
}

// runTmuxNudge drives a claude-tmux launch (REPL boots immediately, pane
// stays idle/static) with a recording fake and a wall-clock safety net: a
// runaway nudge loop hits the 30s context deadline instead of hanging `go
// test`, and then fails the delivery-count assertion.
func runTmuxNudge(t *testing.T, fx launchFixture, tmux *nudgeRecordingTmux) (int, string) {
	t.Helper()
	eng := NewEngine(Deps{
		Tmux:      tmux,
		Sleep:     func(time.Duration) {},
		LookupEnv: mapLookup(nil),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(ctx, fx.args("claude-tmux", "--allow-bypass"), nil, &stdout, &stderr)
	return code, stderr.String()
}

// TestTmuxREPL_IdleArtifactNudge_SentOnceThenTimeout — THE nudge contract:
// agent idle (static pane), artifact never appears → the driver delivers
// EXACTLY ONE in-pane nudge naming the artifact path, then the run still
// concludes with ExitArtifactTimeout (the relaunch path is unchanged — the
// nudge buys one extra interval, not immortality). RED today: 0 nudges.
func TestTmuxREPL_IdleArtifactNudge_SentOnceThenTimeout(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &nudgeRecordingTmux{fakeTmux: &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}}
	code, stderr := runTmuxNudge(t, fx, tmux)
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d (ExitArtifactTimeout); stderr=%q", code, ExitArtifactTimeout, stderr)
	}
	if got := tmux.deliveriesNaming(fx.artifact); got != 1 {
		t.Errorf("idle-artifact nudge naming %s must be delivered exactly once; got %d deliveries", fx.artifact, got)
	}
}

// TestTmuxREPL_NoNudgeWhenArtifactPresent — negative pin: a run whose
// artifact is already on disk completes without any nudge chatter (the nudge
// must be gated on artifact-missing, not fire on every review tick).
func TestTmuxREPL_NoNudgeWhenArtifactPresent(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	if err := os.WriteFile(fx.artifact, []byte("<!-- challenge-token: "+fx.token+" -->\nDONE\n"), 0o644); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	tmux := &nudgeRecordingTmux{fakeTmux: &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}}
	code, stderr := runTmuxNudge(t, fx, tmux)
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK; stderr=%q", code, stderr)
	}
	if got := tmux.deliveriesNaming(fx.artifact); got != 0 {
		t.Errorf("no nudge may be delivered when the artifact is present; got %d", got)
	}
}
