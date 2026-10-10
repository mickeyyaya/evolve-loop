package attemptpostmortem

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeTranscript_ReplaysTheLastCommandsOfCycle1853BuildAttempt1(t *testing.T) {
	trace, err := ClaudeTranscript(fixtureAttempt1)()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if trace.Path != fixtureAttempt1 {
		t.Fatalf("Path = %q, want %q", trace.Path, fixtureAttempt1)
	}
	if len(trace.Commands) != 4 {
		t.Fatalf("got %d commands, want 4: %+v", len(trace.Commands), trace.Commands)
	}
	last := trace.Commands[3]
	if !strings.Contains(last.Text, "TMUX_TMPDIR=$d tmux kill-server") || last.Status != StatusOK || last.ExitCode != 0 {
		t.Fatalf("last command = %+v, want the ok kill-server probe", last)
	}
	if got := last.StartedAt.Format("15:04:05.000"); got != "09:21:08.008" {
		t.Fatalf("last StartedAt = %s, want 09:21:08.008", got)
	}
	if got := trace.LastActivityAt.Format("15:04:05.000"); got != "09:21:08.277" {
		t.Fatalf("LastActivityAt = %s, want 09:21:08.277", got)
	}
}

func TestClaudeTranscript_ReadsAnExitBySignalFromCycle1853BuildAttempt2(t *testing.T) {
	trace, err := ClaudeTranscript(fixtureAttempt2)()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	last := trace.Commands[len(trace.Commands)-1]
	if last.Status != StatusSignal || last.ExitCode != 137 || !strings.Contains(last.Text, "kill-server") {
		t.Fatalf("last command = %+v, want a signal exit 137 of the kill-server probe", last)
	}
}

func TestClaudeTranscript_ClassifiesEachResultForm(t *testing.T) {
	body := strings.Join([]string{
		`not json`,
		`{"type":"mode","mode":"normal"}`,
		`{"type":"assistant","timestamp":"2026-10-09T10:00:00Z","message":{"content":[{"type":"tool_use","id":"r","name":"Read","input":{"file_path":"x"}}]}}`,
		`{"type":"assistant","timestamp":"2026-10-09T10:00:01Z","message":{"content":"plain text"}}`,
		`{"type":"assistant","timestamp":"2026-10-09T10:00:02Z","message":{"content":[{"type":"tool_use","id":"a","name":"Bash","input":{"command":"false"}}]}}`,
		`{"type":"user","timestamp":"2026-10-09T10:00:03Z","message":{"content":[{"type":"tool_result","tool_use_id":"a","is_error":true,"content":[{"type":"text","text":"Exit code 1\nboom"}]}]}}`,
		`{"type":"assistant","timestamp":"2026-10-09T10:00:04Z","message":{"content":[{"type":"tool_use","id":"b","name":"Bash","input":{"command":"bad tool"}}]}}`,
		`{"type":"user","timestamp":"2026-10-09T10:00:05Z","message":{"content":[{"type":"tool_result","tool_use_id":"b","is_error":true,"content":"tool failed"}]}}`,
		`{"type":"assistant","timestamp":"2026-10-09T10:00:06Z","message":{"content":[{"type":"tool_use","id":"c","name":"Bash","input":{"command":"sleep 999"}}]}}`,
		`{"type":"user","timestamp":"2026-10-09T10:00:07Z","message":{"content":[{"type":"tool_result","tool_use_id":"zz","content":"orphan"}]}}`,
		`{"type":"assistant","timestamp":"2026-10-09T10:00:06Z","message":{"content":[{"type":"tool_use","id":"d","name":"Bash","input":{"command":"odd"}}]}}`,
		`{"type":"user","timestamp":"2026-10-09T10:00:06Z","message":{"content":[{"type":"tool_result","tool_use_id":"d","is_error":true,"content":42}]}}`,
		`{"type":"assistant","timestamp":"2026-10-09T10:00:06Z","message":{"content":[{"type":"tool_use","id":"e","name":"Bash","input":{"command":"huge"}}]}}`,
		`{"type":"user","timestamp":"2026-10-09T10:00:06Z","message":{"content":[{"type":"tool_result","tool_use_id":"e","is_error":true,"content":"Exit code 1370"}]}}`,
		`{"type":"assistant","timestamp":"2026-10-09T10:00:06Z","message":{"content":[{"type":"tool_use","id":"f","name":"Bash","input":{"command":"exit 128"}}]}}`,
		`{"type":"user","timestamp":"2026-10-09T10:00:06Z","message":{"content":[{"type":"tool_result","tool_use_id":"f","is_error":true,"content":"Exit code 128"}]}}`,
		`{"type":"attachment","timestamp":"not a time"}`,
	}, "\n")
	path := writeFile(t, t.TempDir(), "t.jsonl", body)
	trace, err := ClaudeTranscript(path)()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := []struct {
		text   string
		status Status
		exit   int
	}{{"false", StatusError, 1}, {"bad tool", StatusError, 0}, {"sleep 999", StatusNoResult, 0}, {"odd", StatusError, 0}, {"huge", StatusError, 0}, {"exit 128", StatusError, 128}}
	if len(trace.Commands) != len(want) {
		t.Fatalf("commands = %+v, want %d", trace.Commands, len(want))
	}
	for i, w := range want {
		got := trace.Commands[i]
		if got.Text != w.text || got.Status != w.status || got.ExitCode != w.exit {
			t.Errorf("command %d = %+v, want %q %s exit %d", i, got, w.text, w.status, w.exit)
		}
	}
	if got := trace.LastActivityAt.Format("15:04:05"); got != "10:00:07" {
		t.Fatalf("LastActivityAt = %s, want 10:00:07", got)
	}
}

func TestClaudeTranscript_AMissingFileIsAnError(t *testing.T) {
	_, err := ClaudeTranscript(filepath.Join(t.TempDir(), "absent.jsonl"))()
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v, want os.ErrNotExist", err)
	}
}

func TestClaudeTranscript_ALineOverTheScannerLimitIsAnError(t *testing.T) {
	path := writeFile(t, t.TempDir(), "big.jsonl", strings.Repeat("x", 9*1024*1024))
	if _, err := ClaudeTranscript(path)(); err == nil {
		t.Fatal("err = nil, want the scanner error for a line over the limit")
	}
}

func TestTranscriptFor_SelectsTheClaudeAdapterOnlyForAClaudeCLIWithAPath(t *testing.T) {
	cases := []struct {
		cli, path string
		claude    bool
	}{
		{"claude-tmux", fixtureAttempt1, true},
		{"claude", fixtureAttempt1, true},
		{"claude-tmux", "", false},
		{"agy-claude-tmux", fixtureAttempt1, false},
		{"codex", fixtureAttempt1, false},
	}
	for _, tc := range cases {
		t.Run(tc.cli+"|"+tc.path, func(t *testing.T) {
			trace, err := TranscriptFor(tc.cli, tc.path)()
			if tc.claude {
				if err != nil || len(trace.Commands) == 0 {
					t.Fatalf("TranscriptFor(%q) = %+v, %v; want the Claude adapter", tc.cli, trace, err)
				}
				return
			}
			if !errors.Is(err, ErrNoTranscript) {
				t.Fatalf("err = %v, want ErrNoTranscript", err)
			}
		})
	}
}
