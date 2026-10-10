package tokenusage

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

func LocateTranscript(root string, w Window) string {
	best, bestAt := "", time.Time{}
	if root == "" {
		return ""
	}
	_ = filepath.WalkDir(filepath.Join(root, "projects"), func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || !d.Type().IsRegular() || !strings.HasSuffix(path, ".jsonl") || unchangedSince(d, w.Start) {
			return nil
		}
		lines, _ := readLines(path)
		if !attributes(lines, w) {
			return nil
		}
		if at, ok := latestStampWithin(lines, w); ok && at.After(bestAt) {
			best, bestAt = path, at
		}
		return nil
	})
	return best
}

func latestStampWithin(lines []transcriptLine, w Window) (time.Time, bool) {
	var latest time.Time
	found := false
	for _, ln := range lines {
		at, err := time.Parse(time.RFC3339, ln.Timestamp)
		if err != nil || at.Before(w.Start) || at.After(w.End) {
			continue
		}
		if !found || at.After(latest) {
			latest, found = at, true
		}
	}
	return latest, found
}

func unchangedSince(d os.DirEntry, start time.Time) bool {
	info, err := d.Info()
	return err != nil || info.ModTime().Before(start)
}

func ClaudeConfigRoot(env map[string]string) string {
	home := env["HOME"]
	if home == "" {
		home = os.Getenv("HOME")
	}
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".claude")
}
