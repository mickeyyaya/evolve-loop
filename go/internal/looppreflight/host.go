package looppreflight

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// defaultDirWritable reports whether dir can be created and written to, using
// the same mkdir → touch sentinel → remove probe as preflight.probeWritable
// (which is unexported there). Best-effort: any failure means "not writable".
func defaultDirWritable(dir string) bool {
	if dir == "" {
		return false
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	probe := filepath.Join(dir, fmt.Sprintf(".looppreflight-probe.%d", os.Getpid()))
	f, err := os.Create(probe)
	if err != nil {
		return false
	}
	_ = f.Close()
	_ = os.Remove(probe)
	return true
}

// defaultTmuxSessions returns the live tmux session names via `tmux ls`. An
// error (no tmux binary, or no running server) is returned to the caller, which
// treats it as "no stale sessions" — absence of a server is the healthy case.
func defaultTmuxSessions() ([]string, error) {
	out, err := exec.Command("tmux", "ls", "-F", "#{session_name}").Output()
	if err != nil {
		return nil, err
	}
	var sessions []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			sessions = append(sessions, s)
		}
	}
	return sessions, nil
}
