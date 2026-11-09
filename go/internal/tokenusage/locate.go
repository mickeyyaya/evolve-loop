package tokenusage

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

type locatedTranscript struct {
	path string
	at   time.Time
}

func LocateTranscript(roots []string, w Window) string {
	var best locatedTranscript
	for _, root := range roots {
		best = locateIn(root, w, best)
	}
	return best.path
}

func locateIn(root string, w Window, best locatedTranscript) locatedTranscript {
	if root == "" {
		return best
	}
	_ = filepath.WalkDir(filepath.Join(root, "projects"), func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || !d.Type().IsRegular() || !strings.HasSuffix(path, ".jsonl") || unchangedSince(d, w.Start) {
			return nil
		}
		lines, _ := readLines(path)
		if !attributes(lines, w) {
			return nil
		}
		if at, ok := latestStampWithin(lines, w); ok && at.After(best.at) {
			best = locatedTranscript{path: path, at: at}
		}
		return nil
	})
	return best
}

func ClaudeTranscriptRoots(env map[string]string) []string {
	var roots []string
	for _, root := range []string{ClaudeConfigRoot(env), paths.SeatRoot(envOrProcess(env, "HOME"))} {
		if root != "" && !slices.Contains(roots, root) {
			roots = append(roots, root)
		}
	}
	return roots
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
	for _, dir := range []string{env["CLAUDE_CONFIG_DIR"], os.Getenv("CLAUDE_CONFIG_DIR")} {
		if filepath.IsAbs(dir) {
			return filepath.Clean(dir)
		}
	}
	home := envOrProcess(env, "HOME")
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".claude")
}

func envOrProcess(env map[string]string, key string) string {
	if v := env[key]; v != "" {
		return v
	}
	return os.Getenv(key)
}
