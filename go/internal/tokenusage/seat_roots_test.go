package tokenusage

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestClaudeTranscriptRoots_NamesTheStoredRootAndTheSeatRoot(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	cases := []struct {
		name string
		env  map[string]string
		want []string
	}{
		{"stored and seat", map[string]string{"HOME": "/h"}, []string{"/h/.claude", "/h/.evolve-seat"}},
		{"a seat env names the seat once", map[string]string{"HOME": "/h", "CLAUDE_CONFIG_DIR": "/h/.evolve-seat"}, []string{"/h/.evolve-seat"}},
		{"another config dir keeps the seat root", map[string]string{"HOME": "/h", "CLAUDE_CONFIG_DIR": "/cfg"}, []string{"/cfg", "/h/.evolve-seat"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClaudeTranscriptRoots(tc.env); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ClaudeTranscriptRoots(%v) = %v, want %v", tc.env, got, tc.want)
			}
		})
	}
}

func TestClaudeTranscriptRoots_NoHomeAndNoConfigDirNamesNoRoot(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("HOME", "")
	if got := ClaudeTranscriptRoots(nil); len(got) != 0 {
		t.Fatalf("ClaudeTranscriptRoots(nil) = %v, want no root", got)
	}
}

func writeSeatUsageTranscript(t *testing.T, home, worktree string) {
	t.Helper()
	dir := filepath.Join(home, ".evolve-seat", "projects", "-repo-worktrees-cycle-623")
	body := `{"type":"user","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:01Z","message":{"id":"u1","content":[{"type":"text","text":"start task"}]}}
{"type":"assistant","cwd":"` + worktree + `","timestamp":"2026-07-07T10:00:05Z","message":{"id":"m1","usage":{"input_tokens":200,"output_tokens":40,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
`
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "seat.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHostResolver_CountsTheSpendOfASeatThatRanOnTheToken(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	home := t.TempDir()
	const worktree = "/repo/worktrees/cycle-623"
	writeSeatUsageTranscript(t, home, worktree)
	res, err := HostResolver(map[string]string{"HOME": home})(Window{
		Collector: FormatClaudeJSONL, ContextWindow: 200_000, Worktree: worktree,
		Start: mustParse(t, launchWindowStart), End: mustParse(t, launchWindowEnd),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != SourceTranscript || res.Usage.Input != 200 {
		t.Fatalf("a seat transcript under the seat root must be counted: source %q, input %d", res.Source, res.Usage.Input)
	}
	stored, err := DefaultResolver(filepath.Join(home, ".claude"))(Window{
		Collector: FormatClaudeJSONL, ContextWindow: 200_000, Worktree: worktree,
		Start: mustParse(t, launchWindowStart), End: mustParse(t, launchWindowEnd),
	})
	if err != nil || stored.Source == SourceTranscript {
		t.Fatalf("the stored root alone must not see the seat transcript: source %q, err %v", stored.Source, err)
	}
}

func TestLocateTranscript_FindsASeatSessionAndPicksTheLatestAcrossRoots(t *testing.T) {
	const cwd = "/wt/cycle-1853"
	stored, seat := t.TempDir(), t.TempDir()
	older := writeTranscript(t, filepath.Join(stored, "projects", "-wt-cycle-1853"), "older.jsonl", sessionBody(cwd, "2026-10-09T09:19:29Z", "2026-10-09T09:21:08Z"))
	newer := writeTranscript(t, filepath.Join(seat, "projects", "-wt-cycle-1853"), "newer.jsonl", sessionBody(cwd, "2026-10-09T10:00:54Z", "2026-10-09T10:02:17Z"))
	w := Window{ArtifactPath: ".evolve/runs/cycle-1853/build-report.md", Worktree: cwd,
		Start: mustParse(t, "2026-10-09T09:00:00Z"), End: mustParse(t, "2026-10-09T11:00:00Z")}
	if got := LocateTranscript([]string{seat}, w); got != newer {
		t.Fatalf("a session under the seat root must be found: got %q, want %q", got, newer)
	}
	if got := LocateTranscript([]string{stored, seat}, w); got != newer {
		t.Fatalf("the latest session across the roots must win: got %q, want %q", got, newer)
	}
	if got := LocateTranscript([]string{seat, stored}, w); got != newer {
		t.Fatalf("the order of the roots must not change the pick: got %q, want %q", got, newer)
	}
	w.End = mustParse(t, "2026-10-09T09:59:00Z")
	if got := LocateTranscript([]string{seat, stored}, w); got != older {
		t.Fatalf("a window that holds only the stored session must pick it: got %q, want %q", got, older)
	}
}
