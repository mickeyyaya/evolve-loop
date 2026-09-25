package bridge

// nudge_stale_deliverable_test.go — F39 (2026-09-26, cycle 1691): a
// correction re-dispatch whose fix lived in another file left the deliverable
// from the earlier attempt untouched. The completion baseline (size+mtime at
// dispatch) rightly refused it as a pre-dispatch leftover, but the idle nudge
// only said "Please write the deliverable" — so the agent, seeing the report
// was already correct, re-verified and stopped, and the phase would have
// closed as exit 81. The nudge now says what the host sees, why a rewrite is
// needed, and binds the carry-forward to a re-check.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIdleNudgeFor_NamesAnUnrewrittenDeliverableAndBindsARecheck(t *testing.T) {
	ws := t.TempDir()
	report := filepath.Join(ws, "build-report.md")
	if err := os.WriteFile(report, []byte("# Build Report\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Artifact: report, Workspace: ws}
	base := captureArtifactBaseline(cfg)

	n := idleNudgeFor(cfg, base)
	for _, want := range []string{report, "was not rewritten", "Re-check it against this attempt's work", "append a line recording what this attempt changed and that you re-verified it", "if not, rewrite it"} {
		if !strings.Contains(n.msg, want) {
			t.Errorf("an unchanged pre-dispatch deliverable's nudge must carry %q: %q", want, n.msg)
		}
	}
	if n.label != "idle with an unrewritten deliverable" || n.trigger != "idle_unrewritten_deliverable" {
		t.Errorf("the log label and the trigger come from the same decision: %+v", n)
	}

	absentCfg := &Config{Artifact: filepath.Join(ws, "audit-report.md"), Workspace: ws}
	if n := idleNudgeFor(absentCfg, captureArtifactBaseline(absentCfg)); n != (idleNudge{msg: "Please write the deliverable to " + absentCfg.Artifact + " to complete the phase.", label: "idle with missing artifact", trigger: "idle_no_artifact"}) {
		t.Errorf("an absent deliverable keeps the plain reminder, label and trigger: %+v", n)
	}

	// Rewritten since dispatch: the baseline no longer matches, so the plain
	// reminder (not reached in practice — completion fires first).
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(report, later, later); err != nil {
		t.Fatal(err)
	}
	if n := idleNudgeFor(cfg, base); n.trigger != "idle_no_artifact" {
		t.Errorf("a deliverable changed since dispatch is not called unrewritten: %+v", n)
	}
}

// TestIdleNudgeFor_CarriesANonMarkdownDeliverableForwardInFull (architecture
// review M2): appending a line to a JSON deliverable breaks its parse, so the
// remedy for any non-markdown format is a full rewrite in the same format.
func TestIdleNudgeFor_CarriesANonMarkdownDeliverableForwardInFull(t *testing.T) {
	ws := t.TempDir()
	plan := filepath.Join(ws, "routing-plan.json")
	if err := os.WriteFile(plan, []byte(`{"phases":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Artifact: plan, Workspace: ws}
	n := idleNudgeFor(cfg, captureArtifactBaseline(cfg))
	if n.trigger != "idle_unrewritten_deliverable" || strings.Contains(n.msg, "append a line") || !strings.Contains(n.msg, "write it again in full, in the same format") {
		t.Fatalf("a JSON deliverable is re-checked and rewritten in full, never appended to: %+v", n)
	}
}

// TestIdleNudgeFor_SeesALeftoverAtAFallbackLocation (go review F39 MAJOR): the
// baseline snapshots EVERY candidate location, so an unrewritten leftover at a
// fallback (<workspace>/workspace/<base>) is named too — resolved through
// artifactLocate, the completion poll's own resolution — with the canonical
// path the rewrite must land on.
func TestIdleNudgeFor_SeesALeftoverAtAFallbackLocation(t *testing.T) {
	ws := t.TempDir()
	canonical := filepath.Join(ws, "build-report.md")
	fallback := filepath.Join(ws, "workspace", "build-report.md")
	if err := os.MkdirAll(filepath.Dir(fallback), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fallback, []byte("# Build Report\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Artifact: canonical, Workspace: ws}
	n := idleNudgeFor(cfg, captureArtifactBaseline(cfg))
	if n.trigger != "idle_unrewritten_deliverable" || !strings.Contains(n.msg, fallback) || !strings.Contains(n.msg, "written to "+canonical) {
		t.Fatalf("a fallback leftover is named with the canonical target: %+v", n)
	}
}

// appendOnNudgeTmux is an agent that does what the explaining nudge asks:
// re-checks and appends an attestation line to the deliverable.
type appendOnNudgeTmux struct {
	*nudgeRecordingTmux
	artifact string
}

func (a *appendOnNudgeTmux) SendKeys(ctx context.Context, session, keys string, enter bool) error {
	if err := a.nudgeRecordingTmux.SendKeys(ctx, session, keys, enter); err != nil {
		return err
	}
	if strings.Contains(keys, "was not rewritten") {
		f, err := os.OpenFile(a.artifact, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.WriteString("\nCorrection: re-verified; this attempt fixed the explanation document.\n")
		return err
	}
	return nil
}

func launchWithLeftover(t *testing.T, tm func(fx launchFixture) TmuxController) (int, string, launchFixture) {
	t.Helper()
	fx := newFixture(t, "claude-tmux", "")
	if err := os.WriteFile(fx.artifact, []byte("<!-- challenge-token: "+fx.token+" -->\nfrom the earlier attempt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	eng := NewEngine(Deps{
		Tmux:            tm(fx),
		Sleep:           func(time.Duration) {},
		LookupEnv:       mapLookup(nil),
		CaptureBaseline: captureArtifactBaseline,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(ctx, fx.args("claude-tmux", "--allow-bypass"), nil, &stdout, &stderr)
	return code, stderr.String(), fx
}

// TestTmuxREPL_IdleNudge_ExplainsAnUnrewrittenDeliverable drives the real
// claude-tmux wait with the PRODUCTION baseline: the deliverable exists from an
// earlier attempt and the idle agent never rewrites it — the one nudge says
// so (and the operator's log line names it), instead of the plain reminder
// the 1691 agent could not act on.
func TestTmuxREPL_IdleNudge_ExplainsAnUnrewrittenDeliverable(t *testing.T) {
	var tmux *nudgeRecordingTmux
	code, stderr, _ := launchWithLeftover(t, func(launchFixture) TmuxController {
		tmux = &nudgeRecordingTmux{fakeTmux: &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}}
		return tmux
	})
	if code != ExitArtifactTimeout {
		t.Fatalf("an unrewritten leftover never completes the phase: exit=%d stderr=%q", code, stderr)
	}
	if got := tmux.deliveriesNaming("was not rewritten"); got != 1 || !strings.Contains(stderr, "idle with an unrewritten deliverable; sent one-shot nudge") {
		t.Fatalf("exactly one explaining nudge, labeled in the log: %d deliveries; stderr=%q", got, stderr)
	}
}

// TestTmuxREPL_IdleNudge_AnAgentThatReChecksAndAppendsCompletes (architecture
// review m2): the prescribed fix works end to end — an agent that appends its
// re-verification line after the explaining nudge completes the phase (exit 0)
// under the production baseline, with no operator touch.
func TestTmuxREPL_IdleNudge_AnAgentThatReChecksAndAppendsCompletes(t *testing.T) {
	code, stderr, _ := launchWithLeftover(t, func(fx launchFixture) TmuxController {
		return &appendOnNudgeTmux{nudgeRecordingTmux: &nudgeRecordingTmux{fakeTmux: &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}}, artifact: fx.artifact}
	})
	if code != ExitOK {
		t.Fatalf("an agent that re-checks and appends completes the phase: exit=%d stderr=%q", code, stderr)
	}
}
