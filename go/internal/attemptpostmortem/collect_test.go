package attemptpostmortem

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestCollect_NamesTheKillServerProbeAsTheSuspectOfCycle1853BuildAttempt1(t *testing.T) {
	rec := collectAttempt1(t, DefaultConfig())

	if rec.Schema != SchemaVersion || rec.CommandSource != SourceTranscript || rec.SourceError != "" {
		t.Fatalf("header = %q %q %q, want the schema and the transcript source", rec.Schema, rec.CommandSource, rec.SourceError)
	}
	if rec.Suspect == nil {
		t.Fatal("Suspect = nil, want the kill-server probe")
	}
	text := rec.Suspect.Command.Text
	if !strings.Contains(text, "TMUX_TMPDIR=$d tmux -f /dev/null new-session -d -s probe;") || !strings.Contains(text, "tmux kill-server") {
		t.Fatalf("suspect = %q, want the TMUX_TMPDIR new-session … kill-server probe", text)
	}
	if rec.Suspect.Reason != ReasonEndWindow {
		t.Fatalf("reason = %q, want %q", rec.Suspect.Reason, ReasonEndWindow)
	}
	if got := rec.LastActivityAt.Format("15:04:05.000"); got != "09:21:08.277" {
		t.Fatalf("LastActivityAt = %s, want 09:21:08.277", got)
	}
	if rec.CauseCode != "review_pause" || rec.ExitCode != 81 || rec.Number != 1 {
		t.Fatalf("attempt = %+v, want review_pause / 81 / attempt 1", rec.Attempt)
	}
	if rec.PaneTail != "no server running on /private/tmp/tmux-501/evolve-bridge-p21421" {
		t.Fatalf("PaneTail = %q", rec.PaneTail)
	}
	if !strings.Contains(rec.WorktreeDelta, "dispatcher_test.go") {
		t.Fatalf("WorktreeDelta = %q", rec.WorktreeDelta)
	}
	want := []string{fixtureAttempt1, fixtureFinalPane1}
	if strings.Join(rec.EvidencePaths, ",") != strings.Join(want, ",") {
		t.Fatalf("EvidencePaths = %v, want %v", rec.EvidencePaths, want)
	}
}

func TestCollect_AMissingTranscriptFallsBackToThePaneTail(t *testing.T) {
	a := cycle1853BuildAttempt1(t)
	missing := filepath.Join(t.TempDir(), "absent.jsonl")
	rec, err := Collect(Input{Attempt: a, Transcript: TranscriptFor(a.CLI, missing), ScrollbackPath: fixtureFinalPane1}, DefaultConfig())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if rec.CommandSource != SourcePaneTail || !strings.Contains(rec.SourceError, "absent.jsonl") {
		t.Fatalf("source = %q (%q), want pane_tail with the transcript error", rec.CommandSource, rec.SourceError)
	}
	if len(rec.Commands) != 0 || rec.Suspect != nil {
		t.Fatalf("commands = %v suspect = %v, want none from a pane tail", rec.Commands, rec.Suspect)
	}
	if !strings.Contains(rec.PaneTail, "no server running") || strings.Join(rec.EvidencePaths, ",") != fixtureFinalPane1 {
		t.Fatalf("pane = %q evidence = %v", rec.PaneTail, rec.EvidencePaths)
	}
}

func TestCollect_AnAgyAttemptUsesThePaneTailOnly(t *testing.T) {
	a := cycle1853BuildAttempt1(t)
	a.CLI = "agy-claude-tmux"
	rec, err := Collect(Input{Attempt: a, Transcript: TranscriptFor(a.CLI, fixtureAttempt1), ScrollbackPath: filepath.Join(t.TempDir(), "none.txt")}, DefaultConfig())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if rec.CommandSource != SourcePaneTail || !strings.HasPrefix(rec.SourceError, ErrNoTranscript.Error()) || rec.PaneTail != "" || len(rec.EvidencePaths) != 0 {
		t.Fatalf("rec = %+v, want a pane-only record with no evidence files", rec)
	}
}

func TestCollect_TheCapsTruncateTheStoredText(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxCommands, cfg.MaxCommandRunes, cfg.MaxPaneTailRunes, cfg.MaxDeltaRunes = 2, 40, 10, 12
	dir := t.TempDir()
	pane := writeFile(t, dir, "pane.txt", "first line\n"+strings.Repeat("é", 30)+"\nLAST-LINE\n\n")
	a := cycle1853BuildAttempt1(t)
	rec, err := Collect(Input{Attempt: a, Transcript: TranscriptFor(a.CLI, fixtureAttempt1), ScrollbackPath: pane, WorktreeDelta: strings.Repeat("d", 50)}, cfg)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(rec.Commands) != 2 {
		t.Fatalf("got %d commands, want the last 2", len(rec.Commands))
	}
	if got := rec.Commands[0].StartedAt.Format("15:04:05.000") + " " + rec.Commands[1].StartedAt.Format("15:04:05.000"); got != "09:21:02.503 09:21:08.008" {
		t.Fatalf("commands = %+v, want the last two in order", rec.Commands)
	}
	for _, c := range append(rec.Commands, rec.Suspect.Command) {
		if n := utf8.RuneCountInString(c.Text); n != 40 || !strings.HasSuffix(c.Text, "…") {
			t.Fatalf("command %q has %d runes, want 40 that end in …", c.Text, n)
		}
	}
	if rec.PaneTail != "…LAST-LINE" {
		t.Fatalf("PaneTail = %q, want the last 10 runes with a leading …", rec.PaneTail)
	}
	if rec.WorktreeDelta != strings.Repeat("d", 11)+"…" {
		t.Fatalf("WorktreeDelta = %q, want 12 runes", rec.WorktreeDelta)
	}
}

func TestCollect_ASuspectOutsideTheLastCommandsIsStillNamed(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxCommands = 1
	body := strings.Join([]string{
		`{"type":"assistant","timestamp":"2026-10-09T10:00:00Z","message":{"content":[{"type":"tool_use","id":"a","name":"Bash","input":{"command":"tmux kill-server"}}]}}`,
		`{"type":"assistant","timestamp":"2026-10-09T10:20:00Z","message":{"content":[{"type":"tool_use","id":"b","name":"Bash","input":{"command":"ls"}}]}}`,
		`{"type":"user","timestamp":"2026-10-09T10:21:00Z","message":{"content":[{"type":"tool_result","tool_use_id":"b","content":"x"}]}}`,
	}, "\n")
	a := cycle1853BuildAttempt1(t)
	rec, err := Collect(Input{Attempt: a, Transcript: ClaudeTranscript(writeFile(t, t.TempDir(), "t.jsonl", body))}, cfg)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if rec.Suspect == nil || rec.Suspect.Command.Text != "tmux kill-server" || rec.Suspect.Reason != ReasonNoResult {
		t.Fatalf("Suspect = %+v, want the kill-server with no result", rec.Suspect)
	}
}

func TestCollect_WithNoSessionActivityTheWindowEndsAtTheAttemptEnd(t *testing.T) {
	a := cycle1853BuildAttempt1(t)
	at := a.EndedAt.Add(-10 * time.Second)
	trace := Trace{Commands: []Command{{Text: "rm -rf x", StartedAt: at, Status: StatusOK}}}
	rec, err := Collect(Input{Attempt: a, Transcript: func() (Trace, error) { return trace, nil }}, DefaultConfig())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if rec.Suspect == nil || rec.Suspect.Reason != ReasonEndWindow || len(rec.EvidencePaths) != 0 {
		t.Fatalf("rec = %+v, want the command in the window before EndedAt and no evidence path", rec)
	}
}

func TestCollect_RefusesAnInvalidConfigAnInvalidAttemptAndAnUnreadablePane(t *testing.T) {
	a := cycle1853BuildAttempt1(t)
	noTranscript := TranscriptFor("codex", "")
	if _, err := Collect(Input{Attempt: a, Transcript: noTranscript}, Config{}); err == nil {
		t.Error("an empty config: err = nil, want the Validate error")
	}
	bad := a
	bad.Phase = ""
	if _, err := Collect(Input{Attempt: bad, Transcript: noTranscript}, DefaultConfig()); err == nil {
		t.Error("an attempt with no phase: err = nil, want the Validate error")
	}
	_, err := Collect(Input{Attempt: a, Transcript: noTranscript, ScrollbackPath: t.TempDir()}, DefaultConfig())
	if err == nil || errors.Is(err, ErrNoTranscript) {
		t.Errorf("a directory as the scrollback: err = %v, want a read error", err)
	}
}
