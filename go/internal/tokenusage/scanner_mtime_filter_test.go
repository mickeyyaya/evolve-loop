package tokenusage

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type infoFailingEntry struct{ fs.DirEntry }

func (infoFailingEntry) Info() (fs.FileInfo, error) { return nil, fs.ErrPermission }

func stampTranscript(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
}

func countTranscriptReads(t *testing.T) *[]string {
	t.Helper()
	var reads []string
	prev := readTranscript
	t.Cleanup(func() { readTranscript = prev })
	readTranscript = func(path string) ([]transcriptLine, error) {
		reads = append(reads, filepath.Base(path))
		return prev(path)
	}
	return &reads
}

func TestScanConfigRoot_MtimeFilterReadsOnlyFilesThatCanHoldAWindowTurn(t *testing.T) {
	w := attributedWindow(t)
	tests := []struct {
		name      string
		mtime     time.Time
		wantRead  bool
		wantInput int
	}{
		{"modified long before the window", w.Start.Add(-48 * time.Hour), false, 0},
		{"modified after the window start", w.Start.Add(time.Second), true, 70},
		{"modified one second inside the slack", w.Start.Add(-transcriptMtimeSlack + time.Second), true, 70},
		{"modified exactly at the slack edge", w.Start.Add(-transcriptMtimeSlack), true, 70},
		{"modified one second past the slack", w.Start.Add(-transcriptMtimeSlack - time.Second), false, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			p := writeTranscript(t, filepath.Join(root, "projects", "p"), "s.jsonl", attributedTranscript(""))
			stampTranscript(t, p, tc.mtime)
			reads := countTranscriptReads(t)

			res, err := ScanConfigRoot(root, w)

			if err != nil {
				t.Fatal(err)
			}
			if got := len(*reads) == 1; got != tc.wantRead {
				t.Errorf("read = %v (%v), want %v", got, *reads, tc.wantRead)
			}
			if res.Usage.Input != tc.wantInput {
				t.Errorf("Usage.Input = %d, want %d", res.Usage.Input, tc.wantInput)
			}
		})
	}
}

func TestScanConfigRoot_ZeroStartReadsEveryFile(t *testing.T) {
	root := t.TempDir()
	p := writeTranscript(t, filepath.Join(root, "projects", "p"), "s.jsonl", attributedTranscript(""))
	stampTranscript(t, p, time.Unix(1000, 0))
	reads := countTranscriptReads(t)
	w := Window{Worktree: attributedWorktree, End: attributedWindow(t).End}

	res, err := ScanConfigRoot(root, w)

	if err != nil || len(*reads) != 1 || res.Usage.Input != 70 {
		t.Errorf("= (%+v, %v) after reads %v, want the old file read and 70 input tokens counted", res, err, *reads)
	}
}

func TestScanConfigRoot_AnInfoErrorFailsOpenAndReadsTheFile(t *testing.T) {
	root := t.TempDir()
	p := writeTranscript(t, filepath.Join(root, "projects", "p"), "s.jsonl", attributedTranscript(""))
	stampTranscript(t, p, attributedWindow(t).Start.Add(-48*time.Hour))
	reads := countTranscriptReads(t)
	prev := walkDir
	t.Cleanup(func() { walkDir = prev })
	walkDir = func(dir string, fn fs.WalkDirFunc) error {
		return prev(dir, func(path string, d fs.DirEntry, err error) error {
			return fn(path, infoFailingEntry{d}, err)
		})
	}

	res, err := ScanConfigRoot(root, attributedWindow(t))

	if err != nil || len(*reads) != 1 || res.Usage.Input != 70 {
		t.Errorf("= (%+v, %v) after reads %v, want the file read as before and 70 input tokens counted", res, err, *reads)
	}
}

func TestScanConfigRoot_AnOtherwiseAttributableOldFileDoesNotChangeTheResult(t *testing.T) {
	w := attributedWindow(t)
	build := func(withOld bool) Result {
		root := t.TempDir()
		dir := filepath.Join(root, "projects", "p")
		writeTranscript(t, dir, "live.jsonl", attributedTranscript(""))
		if withOld {
			p := writeTranscript(t, dir, "old.jsonl", attributedTranscript(""))
			stampTranscript(t, p, w.Start.Add(-72*time.Hour))
		}
		res, err := ScanConfigRoot(root, w)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	without, with := build(false), build(true)

	if without.Usage.Input != 70 {
		t.Fatalf("baseline Usage.Input = %d, want 70", without.Usage.Input)
	}
	if with != without {
		t.Errorf("with old file = %+v, without = %+v, want equal", with, without)
	}
}

func TestScanConfigRoot_ATreeOfTwoHundredOldFilesReadsOnlyTheNewOne(t *testing.T) {
	w := attributedWindow(t)
	root := t.TempDir()
	dir := filepath.Join(root, "projects", "p")
	for i := 0; i < 200; i++ {
		p := writeTranscript(t, dir, fmt.Sprintf("old-%03d.jsonl", i), attributedTranscript(""))
		stampTranscript(t, p, w.Start.Add(-24*time.Hour))
	}
	writeTranscript(t, dir, "new.jsonl", attributedTranscript(""))
	reads := countTranscriptReads(t)

	res, err := ScanConfigRoot(root, w)

	if err != nil || len(*reads) != 1 || (*reads)[0] != "new.jsonl" || res.Usage.Input != 70 {
		t.Errorf("= (%+v, %v) after %d reads %v, want exactly new.jsonl read", res, err, len(*reads), *reads)
	}
}
