package bridge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProbeBootAttempts_ComeFromTheFamilyManifest(t *testing.T) {
	for driver, want := range map[string]int{
		"agy-tmux":        2,
		"agy-claude-tmux": 2,
		"claude-tmux":     1,
		"codex-tmux":      1,
		"no-such-tmux":    1,
	} {
		if got := ProbeBootAttempts(driver); got != want {
			t.Errorf("ProbeBootAttempts(%q) = %d, want %d", driver, got, want)
		}
	}
}

func TestParseManifest_RefusesANegativeProbeBootRetryCount(t *testing.T) {
	_, err := parseManifestWithStderr("x-tmux", []byte(`{"cli":"x-tmux","binary":"x","probe_boot_retries":-1}`), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "probe_boot_retries") {
		t.Fatalf("a negative probe_boot_retries parsed (err=%v); want a refusal naming the field", err)
	}
}

type scriptedBoot struct {
	results []struct {
		rc   int
		wall string
	}
	calls int
}

func (s *scriptedBoot) attempt() (int, string) {
	r := s.results[min(s.calls, len(s.results)-1)]
	s.calls++
	return r.rc, r.wall
}

func boots(pairs ...any) *scriptedBoot {
	s := &scriptedBoot{}
	for i := 0; i < len(pairs); i += 2 {
		s.results = append(s.results, struct {
			rc   int
			wall string
		}{pairs[i].(int), pairs[i+1].(string)})
	}
	return s
}

func TestBootProbeRetry_ABootTimeoutIsRetriedOnceAndTheLogNamesTheColdStart(t *testing.T) {
	var log bytes.Buffer
	s := boots(ExitREPLBootTimeout, "", ExitOK, "")

	rc := BootProbe{Driver: "agy-tmux", Log: &log}.Retry(context.Background(), s.attempt)

	if rc != ExitOK || s.calls != 2 {
		t.Fatalf("rc=%d after %d launch(es); want ExitOK on the second launch", rc, s.calls)
	}
	for _, want := range []string{"[agy-tmux] cold start: boot attempt 1 of 2", "[agy-tmux] cold start: boot attempt 2 of 2 got past the REPL boot"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("log lacks %q:\n%s", want, log.String())
		}
	}
}

func TestBootProbeRetry_ASecondTimeoutFailsLoudly(t *testing.T) {
	var log bytes.Buffer
	s := boots(ExitREPLBootTimeout, "")

	rc := BootProbe{Driver: "agy-tmux", Log: &log}.Retry(context.Background(), s.attempt)

	if rc != ExitREPLBootTimeout || s.calls != 2 {
		t.Fatalf("rc=%d after %d launch(es); want ExitREPLBootTimeout after exactly 2", rc, s.calls)
	}
	if !strings.Contains(log.String(), "[agy-tmux] FAIL: the REPL never drew its prompt on any of 2 boot attempts") {
		t.Errorf("the final timeout is not logged as a failure of both attempts:\n%s", log.String())
	}
}

func TestBootProbeRetry_OnlyABareBootTimeoutIsRetried(t *testing.T) {
	for name, s := range map[string]*scriptedBoot{
		"booted":            boots(ExitOK, ""),
		"artifact timeout":  boots(ExitArtifactTimeout, ""),
		"walled at boot":    boots(ExitREPLBootTimeout, "auth_recheck"),
		"missing binary":    boots(ExitMissingBinary, ""),
		"rate limit at run": boots(ExitUnknownPrompt, "rate_limit"),
	} {
		t.Run(name, func(t *testing.T) {
			var log bytes.Buffer
			BootProbe{Driver: "agy-tmux", Log: &log}.Retry(context.Background(), s.attempt)
			if s.calls != 1 {
				t.Fatalf("%d launches; a %s is no cold start and must not be retried", s.calls, name)
			}
			if strings.Contains(log.String(), "cold start") {
				t.Errorf("the log names a cold start for a %s:\n%s", name, log.String())
			}
		})
	}
}

func TestBootProbeRetry_ADriverWhoseManifestDeclaresNoRetryLaunchesOnce(t *testing.T) {
	var log bytes.Buffer
	s := boots(ExitREPLBootTimeout, "")

	rc := BootProbe{Driver: "claude-tmux", Log: &log}.Retry(context.Background(), s.attempt)

	if rc != ExitREPLBootTimeout || s.calls != 1 {
		t.Fatalf("rc=%d after %d launch(es); claude-tmux declares no probe_boot_retries", rc, s.calls)
	}
	if log.Len() != 0 {
		t.Errorf("a single-attempt probe logs nothing of its own; got:\n%s", log.String())
	}
}

func TestBootProbeRetry_ACancelledContextIsNotRetried(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := boots(ExitREPLBootTimeout, "")

	BootProbe{Driver: "agy-tmux", Log: nil}.Retry(ctx, s.attempt)

	if s.calls != 1 {
		t.Fatalf("%d launches after the context was cancelled; the interrupt wins", s.calls)
	}
}

type coldStartTmux struct {
	*fakeTmux
	sessions []string
}

func (c *coldStartTmux) NewSession(ctx context.Context, name string, w, h int) error {
	c.sessions = append(c.sessions, name)
	return c.fakeTmux.NewSession(ctx, name, w, h)
}

func (c *coldStartTmux) CapturePane(ctx context.Context, session string, scrollback int) (string, error) {
	if _, err := c.fakeTmux.CapturePane(ctx, session, scrollback); err != nil {
		return "", err
	}
	if len(c.sessions) > 0 && session == c.sessions[0] {
		return "Antigravity CLI\n  signing in…", nil
	}
	return "Antigravity CLI\n\n? for shortcuts", nil
}

func TestBootProbeRetry_AnAgyThatDrawsItsREPLLateOnItsFirstLaunchOnlyBootsOnTheRetry(t *testing.T) {
	tmux := &coldStartTmux{fakeTmux: &fakeTmux{}}
	deps, log := bootSmokeDeps(tmux.fakeTmux)
	deps.Tmux = tmux

	rc := BootProbe{Driver: "agy-tmux", Log: log}.Retry(context.Background(), func() (int, string) {
		rc, _ := BootSmokeTest(context.Background(), "agy-tmux", &Config{Workspace: t.TempDir()}, deps)
		return rc, ""
	})

	if rc != ExitOK || len(tmux.sessions) != 2 {
		t.Fatalf("rc=%d after %d launch(es); want the cold first launch retried once and booted\n%s", rc, len(tmux.sessions), log)
	}
	for _, want := range []string{"FAIL: REPL prompt never appeared", "cold start: boot attempt 1 of 2", "REPL prompt (? for shortcuts) detected"} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("log lacks %q:\n%s", want, log)
		}
	}
}

func TestBootProbeRetry_AnAgyThatNeverDrawsItsREPLFailsWithThePaneCaptured(t *testing.T) {
	tmux := &fakeTmux{paneSeq: []string{"Antigravity CLI\n  signing in…"}}
	deps, log := bootSmokeDeps(tmux)
	launches := 0
	var scrollback string

	rc := BootProbe{Driver: "agy-tmux", Log: log}.Retry(context.Background(), func() (int, string) {
		launches++
		var rc int
		rc, scrollback = BootSmokeTest(context.Background(), "agy-tmux", &Config{Workspace: t.TempDir()}, deps)
		return rc, ""
	})

	if rc != ExitREPLBootTimeout || launches != 2 {
		t.Fatalf("rc=%d after %d launch(es); want ExitREPLBootTimeout after both attempts", rc, launches)
	}
	if !strings.Contains(scrollback, "signing in") {
		t.Errorf("the failing attempt's pane was not captured; scrollback=%q", scrollback)
	}
	if !strings.Contains(log.String(), "FAIL: the REPL never drew its prompt on any of 2 boot attempts") {
		t.Errorf("the final failure is not loud:\n%s", log)
	}
}

func TestEscalationPattern_ReadsTheClassifiedWallAWorkspaceRecorded(t *testing.T) {
	ws := t.TempDir()
	if got := EscalationPattern(ws); got != "" {
		t.Errorf("a workspace with no escalation report = %q, want none", got)
	}
	if err := os.WriteFile(filepath.Join(ws, "escalation-report.json"), []byte(`{"pattern_name":"auth_recheck"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := EscalationPattern(ws); got != "auth_recheck" {
		t.Errorf("EscalationPattern = %q, want auth_recheck", got)
	}
}
