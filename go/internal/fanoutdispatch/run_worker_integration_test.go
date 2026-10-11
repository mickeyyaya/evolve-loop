//go:build integration

package fanoutdispatch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunWorker_MapsEveryOutcomeToAnExitCode(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		name, command string
		cfg           Config
		parent        context.Context
		want          int
	}{
		{"success", "true", Config{TimeoutSecs: 10}, context.Background(), 0},
		{"exit status", "exit 7", Config{TimeoutSecs: 10}, context.Background(), 7},
		{"timeout", "sleep 5", Config{TimeoutSecs: 1}, context.Background(), 124},
		{"canceled before start", "true", Config{TimeoutSecs: 10}, canceled, 143},
		{"unlaunchable command", "echo a\x00b", Config{TimeoutSecs: 10}, context.Background(), 1},
		{"unlaunchable environment", "true", Config{TimeoutSecs: 10, CachePrefixFile: "bad\x00prefix"}, context.Background(), 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.runWorker(tc.parent, "w", tc.command, t.TempDir()); got != tc.want {
				t.Fatalf("runWorker(%q) = %d, want %d", tc.command, got, tc.want)
			}
		})
	}
}

func TestRunWorker_CapturesStreamsAndPassesTheCachePrefix(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{TimeoutSecs: 10, CachePrefixFile: "/tmp/prefix.txt"}

	rc := cfg.runWorker(context.Background(), "judge", `echo "$EVOLVE_FANOUT_CACHE_PREFIX_FILE"; echo oops >&2`, dir)

	out, _ := os.ReadFile(filepath.Join(dir, "judge.out"))
	errOut, _ := os.ReadFile(filepath.Join(dir, "judge.err"))
	if rc != 0 || string(out) != "/tmp/prefix.txt\n" || string(errOut) != "oops\n" {
		t.Fatalf("rc %d out %q err %q; want 0, the prefix path and the stderr line", rc, out, errOut)
	}
}

func TestRunWorker_MissingResultsDirStillReportsTheExitCode(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent")

	rc := Config{TimeoutSecs: 10}.runWorker(context.Background(), "w", "echo lost; exit 3", missing)

	if rc != 3 {
		t.Fatalf("runWorker = %d, want 3 even without result files", rc)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("results dir was created: %v", err)
	}
}
