package tokenusage

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const attributedWorktree = "/repo/worktrees/cycle-42"

func attributedTranscript(extra string) string {
	return `{"type":"user","cwd":"` + attributedWorktree + `","timestamp":"2026-07-07T10:00:01Z","message":{"id":"u1","content":[{"type":"text","text":"go"}]}}
` + extra + `{"type":"assistant","cwd":"` + attributedWorktree + `","timestamp":"2026-07-07T10:00:05Z","message":{"id":"m1","usage":{"input_tokens":70,"output_tokens":7,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
`
}

func attributedWindow(t *testing.T) Window {
	t.Helper()
	return Window{Worktree: attributedWorktree, Start: mustParse(t, launchWindowStart), End: mustParse(t, launchWindowEnd)}
}

func TestScanConfigRoot_AFailedWalkReportsNoSourceEvenAfterAMatch(t *testing.T) {
	root := t.TempDir()
	writeTranscript(t, filepath.Join(root, "projects", "p"), "s.jsonl", attributedTranscript(""))
	prev := walkDir
	t.Cleanup(func() { walkDir = prev })
	walkDir = func(dir string, fn fs.WalkDirFunc) error {
		if err := prev(dir, fn); err != nil {
			return err
		}
		return errors.New("walk interrupted")
	}

	res, err := ScanConfigRoot(root, attributedWindow(t))

	if err != nil || res.Source != SourceNone || res.Usage.Input != 0 {
		t.Errorf("= (%+v, %v), want an empty SourceNone result: a partial walk must not report a sum", res, err)
	}
}

func TestScanConfigRoot_UnparseableLinesAreSkippedNotFatal(t *testing.T) {
	root := t.TempDir()
	writeTranscript(t, filepath.Join(root, "projects", "p"), "s.jsonl", attributedTranscript("{not json\n\n\"just a string\"\n"))

	res, err := ScanConfigRoot(root, attributedWindow(t))

	if err != nil || res.Source != SourceTranscript || res.Usage.Input != 70 || res.Warn != "" {
		t.Errorf("= (%+v, %v), want the 70 input tokens counted with no warning", res, err)
	}
}

func TestReadLines(t *testing.T) {
	dir := t.TempDir()
	unreadable := writeTranscript(t, dir, "locked.jsonl", attributedTranscript(""))
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o644) })
	mixed := writeTranscript(t, dir, "mixed.jsonl", "{bad\n"+`{"type":"a"}`+"\n\n"+`{"type":"b"}`+"\n")
	empty := writeTranscript(t, dir, "empty.jsonl", "")
	overlong := writeTranscript(t, dir, "long.jsonl", `{"type":"a"}`+"\n"+strings.Repeat("x", 9*1024*1024)+"\n")
	cases := []struct {
		name      string
		path      string
		wantTypes []string
		wantErr   bool
	}{
		{"unopenable file is quietly empty", unreadable, nil, false},
		{"missing file is quietly empty", filepath.Join(dir, "absent.jsonl"), nil, false},
		{"bad lines are skipped", mixed, []string{"a", "b"}, false},
		{"empty file", empty, nil, false},
		{"overlong line keeps what came before and errors", overlong, []string{"a"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lines, err := readLines(tc.path)

			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error=%v", err, tc.wantErr)
			}
			var types []string
			for _, ln := range lines {
				types = append(types, ln.Type)
			}
			if strings.Join(types, ",") != strings.Join(tc.wantTypes, ",") {
				t.Errorf("types = %v, want %v", types, tc.wantTypes)
			}
		})
	}
}
