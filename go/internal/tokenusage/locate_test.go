package tokenusage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const cycle1853Anchor = "Artifact path: .evolve/runs/cycle-1853/build-report.md"

func sessionBody(cwd string, stamps ...string) string {
	body := `{"type":"user","cwd":"` + cwd + `","timestamp":"` + stamps[0] + `","message":{"content":"` + cycle1853Anchor + `"}}` + "\n"
	for _, ts := range stamps[1:] {
		body += `{"type":"assistant","cwd":"` + cwd + `","timestamp":"` + ts + `","message":{"id":"m` + ts + `"}}` + "\n"
	}
	return body
}

func TestLocateTranscript_PicksTheSessionInsideTheDispatchWindowWhenTwoShareAnchorAndCwd(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "projects", "-wt-cycle-1853")
	const cwd = "/wt/cycle-1853"
	first := writeTranscript(t, dir, "c61dea20-648d-4002-a794-85f794e80ba0.jsonl", sessionBody(cwd, "2026-10-09T09:19:29Z", "2026-10-09T09:21:08.277Z"))
	second := writeTranscript(t, dir, "86c13298-bb96-466f-9c7e-60cdb0d6f387.jsonl", sessionBody(cwd, "2026-10-09T10:00:54Z", "2026-10-09T10:02:17.581Z"))
	writeTranscript(t, dir, "other.jsonl", sessionBody("/elsewhere", "2026-10-09T10:01:00Z"))
	base := Window{ArtifactPath: ".evolve/runs/cycle-1853/build-report.md"}

	cases := []struct {
		name       string
		start, end string
		want       string
	}{
		{"attempt 1", "2026-10-09T09:19:20Z", "2026-10-09T09:59:52Z", first},
		{"attempt 2", "2026-10-09T10:00:40Z", "2026-10-09T10:05:00Z", second},
		{"both windows", "2026-10-09T09:00:00Z", "2026-10-09T11:00:00Z", second},
		{"no session", "2026-10-09T12:00:00Z", "2026-10-09T13:00:00Z", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := base
			w.Start, w.End = mustParse(t, tc.start), mustParse(t, tc.end)
			if got := LocateTranscript(root, w); got != tc.want {
				t.Fatalf("LocateTranscript = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLocateTranscript_FallsBackToTheCwdAndNeedsAStampInTheWindow(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "projects", "p")
	byCwd := writeTranscript(t, dir, "a.jsonl", sessionBody("/wt/a", "2026-10-09T10:00:00Z"))
	writeTranscript(t, dir, "b.jsonl", `{"type":"user","cwd":"/wt/b","message":{"content":"x"}}`+"\n"+`{"type":"assistant","cwd":"/wt/b","timestamp":"not a time"}`+"\n")
	start, end := mustParse(t, "2026-10-09T09:59:00Z"), mustParse(t, "2026-10-09T10:01:00Z")

	if got := LocateTranscript(root, Window{Worktree: "/wt/a", Start: start, End: end}); got != byCwd {
		t.Fatalf("by cwd = %q, want %q", got, byCwd)
	}
	if got := LocateTranscript(root, Window{Worktree: "/wt/b", Start: start, End: end}); got != "" {
		t.Fatalf("a session with no stamp in the window = %q, want none", got)
	}
	if got := LocateTranscript(filepath.Join(root, "absent"), Window{Worktree: "/wt/a", Start: start, End: end}); got != "" {
		t.Fatalf("a root with no projects = %q, want none", got)
	}
	if got := LocateTranscript(root, Window{Worktree: "/wt/a", Start: start.Add(2 * time.Minute), End: end.Add(time.Hour)}); got != "" {
		t.Fatalf("a window after the session = %q, want none", got)
	}
	unchanged := mustParse(t, "2026-10-09T09:00:00Z")
	if err := os.Chtimes(byCwd, unchanged, unchanged); err != nil {
		t.Fatal(err)
	}
	if got := LocateTranscript(root, Window{Worktree: "/wt/a", Start: start, End: end}); got != "" {
		t.Fatalf("a file unchanged since the window start = %q, want it not read", got)
	}
}

func TestClaudeConfigRoot_PrefersTheRequestHomeAndIsEmptyWithNoHome(t *testing.T) {
	t.Setenv("HOME", "/process/home")
	if got := ClaudeConfigRoot(map[string]string{"HOME": "/request/home"}); got != "/request/home/.claude" {
		t.Fatalf("ClaudeConfigRoot(request HOME) = %q", got)
	}
	if got := ClaudeConfigRoot(nil); got != "/process/home/.claude" {
		t.Fatalf("ClaudeConfigRoot(process HOME) = %q", got)
	}
	t.Setenv("HOME", "")
	if got := ClaudeConfigRoot(map[string]string{"HOME": ""}); got != "" {
		t.Fatalf("ClaudeConfigRoot(no HOME) = %q, want empty, never a relative .claude", got)
	}
}

func TestLocateTranscriptAndTranscriptCollector_AnEmptyRootNeverReadsTheWorkingDirectory(t *testing.T) {
	cwd := t.TempDir()
	chdirForTest(t, cwd)
	writeTranscript(t, filepath.Join(cwd, "projects", "p"), "s.jsonl", sessionBody("/wt/a", "2026-10-09T10:00:00Z", "2026-10-09T10:00:05Z"))
	w := Window{Worktree: "/wt/a", Start: mustParse(t, "2026-10-09T09:59:00Z"), End: mustParse(t, "2026-10-09T10:01:00Z")}
	if got := LocateTranscript(".", w); got == "" {
		t.Fatal("control: the relative root must find the session, or the test proves nothing")
	}
	if got := LocateTranscript("", w); got != "" {
		t.Fatalf("LocateTranscript(\"\") = %q, want none", got)
	}
	if res := TranscriptCollector(".", w)(); res.Source != SourceTranscript {
		t.Fatalf("control: TranscriptCollector(\".\") = %+v, want the relative session", res)
	}
	if res := TranscriptCollector("", w)(); res.Source != SourceNone {
		t.Fatalf("TranscriptCollector(\"\") = %+v, want SourceNone", res)
	}
}

func TestLocateTranscript_SkipsASymlinkedTranscript(t *testing.T) {
	root := t.TempDir()
	outside := writeTranscript(t, t.TempDir(), "real.jsonl", sessionBody("/wt/a", "2026-10-09T10:00:00Z", "2026-10-09T10:00:05Z"))
	dir := filepath.Join(root, "projects", "p")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link.jsonl")); err != nil {
		t.Fatal(err)
	}
	w := Window{Worktree: "/wt/a", Start: mustParse(t, "2026-10-09T09:59:00Z"), End: mustParse(t, "2026-10-09T10:01:00Z")}
	if got := LocateTranscript(root, w); got != "" {
		t.Fatalf("LocateTranscript = %q, want the symlink skipped", got)
	}
}

func chdirForTest(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(prev); err != nil {
			t.Errorf("restore the working directory: %v", err)
		}
	})
}
