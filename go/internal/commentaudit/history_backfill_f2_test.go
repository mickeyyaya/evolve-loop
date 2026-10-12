package commentaudit

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnmatchedHistory_ReportsAReadOrParseFailureOfEachSideF2(t *testing.T) {
	t.Parallel()
	boom := errors.New("read boom")
	valid := readFrom(map[string]string{"p.go": "package p\n"})
	broken := readFrom(map[string]string{"p.go": "package p\nfunc ("})
	failing := func(string) ([]byte, error) { return nil, boom }
	cases := []struct {
		name          string
		before, after func(string) ([]byte, error)
		wantIs        error
	}{
		{"before unreadable", failing, valid, boom},
		{"after unreadable", valid, failing, boom},
		{"before does not parse", broken, valid, nil},
		{"after does not parse", valid, broken, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gone, arrived, err := unmatchedHistory("p.go", tc.before, tc.after)

			if gone != nil || arrived != nil || err == nil || (tc.wantIs != nil && !errors.Is(err, tc.wantIs)) {
				t.Errorf("unmatchedHistory = (%v, %v, %v), want nil groups and the failure", gone, arrived, err)
			}
		})
	}
}

func TestRemovedHistory_ReturnsTheFailureOfAnUnreadableFileF2(t *testing.T) {
	t.Parallel()
	boom := errors.New("read boom")

	got, err := RemovedHistoryAcrossDiff([]string{"a.go"}, func(string) ([]byte, error) { return nil, boom }, readFrom(map[string]string{"a.go": "package a\n"}))

	if got != nil || !errors.Is(err, boom) {
		t.Errorf("RemovedHistoryAcrossDiff = (%v, %v), want nil and the read failure", got, err)
	}
}

func TestRemovedHistory_AnchorsAGroupWithNoCodeBelowAsEmptyF2(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"history at the end of the file":     "package p\n\nfunc f() {}\n\n// Cycle 1: x\n",
		"only comments and blanks below":     "package p\n\nfunc f() {}\n\n// Cycle 1: x\n\n// other\n\n/* more */\n",
		"history after the last declaration": "package p\n\nvar v = 1\n// Cycle 7: done\n",
	}
	for name, before := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			after := strings.Replace(before, "// Cycle 1: x\n", "", 1)
			after = strings.Replace(after, "// Cycle 7: done\n", "", 1)

			got, err := RemovedHistoryAcrossDiff([]string{"p.go"}, readFrom(map[string]string{"p.go": before}), readFrom(map[string]string{"p.go": after}))

			if err != nil || len(got) != 1 || got[0].Anchor != "" {
				t.Errorf("RemovedHistoryAcrossDiff = (%+v, %v), want one entry with an empty anchor", got, err)
			}
		})
	}
}

func TestWriteHistoryArchive_FailuresLeaveNothingHalfRecordedF2(t *testing.T) {
	t.Parallel()
	entries := []HistoryEntry{{File: "go/internal/p/p.go", Line: 1, Lines: []string{"// Cycle 1: x"}}}

	t.Run("the archive dir cannot be created", func(t *testing.T) {
		t.Parallel()
		if os.Geteuid() == 0 {
			t.Skip("root creates dirs in a read-only parent")
		}
		parent := t.TempDir()
		if err := os.Chmod(parent, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })

		written, err := writeHistoryArchive(filepath.Join(parent, "archive"), "round 1", entries)

		if written != nil || !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), "history archive") {
			t.Errorf("writeHistoryArchive = (%v, %v), want nil and a history archive permission error", written, err)
		}
	})

	t.Run("a page that cannot be read is refused, not overwritten", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		page := filepath.Join(dir, "internal-p.md")
		if err := os.Mkdir(page, 0o755); err != nil {
			t.Fatal(err)
		}

		written, err := writeHistoryArchive(dir, "round 1", entries)

		if written != nil || err == nil || !strings.Contains(err.Error(), page) {
			t.Errorf("writeHistoryArchive = (%v, %v), want nil and an error naming %s", written, err, page)
		}
		if _, statErr := os.Stat(filepath.Join(dir, historyIndex)); !os.IsNotExist(statErr) {
			t.Errorf("a refused run wrote the index (%v)", statErr)
		}
	})

	t.Run("a label that spans lines is refused", func(t *testing.T) {
		t.Parallel()
		for _, label := range []string{"a\nb", "a\rb"} {
			dir := t.TempDir()

			written, err := writeHistoryArchive(dir, label, entries)

			if written != nil || err == nil || !strings.Contains(err.Error(), "spans lines") {
				t.Errorf("label %q: writeHistoryArchive = (%v, %v), want the spans-lines refusal", label, written, err)
			}
		}
	})
}

func TestRefuseRecordedLabel_ReportsAnUnreadablePageAndSkipsAMissingOneF2(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.md")
	unreadable := filepath.Join(dir, "dir.md")
	if err := os.Mkdir(unreadable, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := refuseRecordedLabel([]string{missing}, "round 1"); err != nil {
		t.Errorf("a page that does not exist yet must not refuse: %v", err)
	}
	err := refuseRecordedLabel([]string{missing, unreadable}, "round 1")

	if err == nil || !strings.Contains(err.Error(), unreadable) {
		t.Errorf("refuseRecordedLabel = %v, want an error naming %s", err, unreadable)
	}
}

func TestWriteHistoryIndex_ReportsEachFailureOfItsDirF2(t *testing.T) {
	t.Parallel()

	t.Run("the dir pattern is malformed", func(t *testing.T) {
		t.Parallel()

		err := writeHistoryIndex(filepath.Join(t.TempDir(), "bad[dir"))

		if !errors.Is(err, filepath.ErrBadPattern) {
			t.Errorf("writeHistoryIndex = %v, want ErrBadPattern", err)
		}
	})

	t.Run("a page cannot be read", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		page := filepath.Join(dir, "broken.md")
		if err := os.Mkdir(page, 0o755); err != nil {
			t.Fatal(err)
		}

		err := writeHistoryIndex(dir)

		if err == nil || !strings.Contains(err.Error(), "history index "+page) {
			t.Errorf("writeHistoryIndex = %v, want an error naming the page", err)
		}
	})

	t.Run("the index cannot be written", func(t *testing.T) {
		t.Parallel()
		dir := filepath.Join(t.TempDir(), "absent")

		err := writeHistoryIndex(dir)

		if err == nil || !strings.Contains(err.Error(), "history index "+dir) {
			t.Errorf("writeHistoryIndex = %v, want an error naming the dir", err)
		}
		if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
			t.Errorf("a failed index created the dir (%v)", statErr)
		}
	})
}
