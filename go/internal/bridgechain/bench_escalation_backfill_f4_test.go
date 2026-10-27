package bridgechain_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

type logSinkF4 struct {
	mu    sync.Mutex
	lines []string
}

func (s *logSinkF4) logf(format string, args ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lines = append(s.lines, strings.TrimSpace(strings.NewReplacer("%v", "?", "%s", "?").Replace(format)))
}

func (s *logSinkF4) joined() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Join(s.lines, "\n")
}

func writeReportF4(t *testing.T, ws string, report map[string]any) {
	t.Helper()
	b, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "escalation-report.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBenchOnEscalation_NoBenchIsWrittenWhenTheReportDoesNotQualify(t *testing.T) {
	now := time.Date(2026, 10, 6, 15, 32, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	start := now.Add(-time.Minute)
	cases := []struct {
		name   string
		setup  func(t *testing.T, ws string)
		off    bool
		cli    string
		silent bool
	}{
		{"health bench disabled by policy", func(t *testing.T, ws string) {
			writeReportF4(t, ws, map[string]any{"captured_at": now, "cli": "claude-tmux", "pattern_name": "exhausted", "pane_tail": "x"})
		}, true, "claude-tmux", true},
		{"no report in the workspace", func(*testing.T, string) {}, false, "claude-tmux", true},
		{"report is not JSON", func(t *testing.T, ws string) {
			if err := os.WriteFile(filepath.Join(ws, "escalation-report.json"), []byte("{broken"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, false, "claude-tmux", true},
		{"pattern is not benchable", func(t *testing.T, ws string) {
			writeReportF4(t, ws, map[string]any{"captured_at": now, "cli": "claude-tmux", "pattern_name": "trust_dialog", "pane_tail": "x"})
		}, false, "claude-tmux", true},
		{"report names another CLI", func(t *testing.T, ws string) {
			writeReportF4(t, ws, map[string]any{"captured_at": now, "cli": "codex-tmux", "pattern_name": "exhausted", "pane_tail": "x"})
		}, false, "claude-tmux", true},
		{"report predates this dispatch", func(t *testing.T, ws string) {
			writeReportF4(t, ws, map[string]any{"captured_at": start.Add(-time.Second), "cli": "claude-tmux", "pattern_name": "exhausted", "pane_tail": "x"})
		}, false, "claude-tmux", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, ws := t.TempDir(), t.TempDir()
			tc.setup(t, ws)
			sink := &logSinkF4{}
			if tc.off {
				writeCLIHealthOff(t, root)
			}

			bridgechain.BenchOnEscalation(bridgechain.Escalation{ProjectRoot: root, Workspace: ws, CLI: tc.cli, DispatchStart: start}, clock, sink.logf)

			if active := clihealth.NewStore(root, clock).Active(); len(active) != 0 {
				t.Errorf("no family may be benched: %+v", active)
			}
			if tc.silent && sink.joined() != "" {
				t.Errorf("a non-qualifying report logs nothing; got %q", sink.joined())
			}
		})
	}
}

func TestBenchOnEscalation_AlreadyBenchedWallIsLoggedAndNotStruckAgain(t *testing.T) {
	now := time.Date(2026, 10, 6, 15, 32, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	root, ws := t.TempDir(), t.TempDir()
	wallSeen := now.Add(-time.Second)
	if _, err := clihealth.NewStore(root, func() time.Time { return now }).BenchWall("claude", "exhausted", "earlier"); err != nil {
		t.Fatal(err)
	}
	writeReportF4(t, ws, map[string]any{"captured_at": wallSeen, "cli": "claude-tmux", "pattern_name": "exhausted", "pane_tail": "limit"})
	sink := &logSinkF4{}

	bridgechain.BenchOnEscalation(bridgechain.Escalation{ProjectRoot: root, Workspace: ws, CLI: "claude-tmux", DispatchStart: now.Add(-time.Minute)}, clock, sink.logf)

	if !strings.Contains(sink.joined(), "already benched for this wall; not benched again") {
		t.Errorf("log = %q", sink.joined())
	}
	if e := clihealth.NewStore(root, clock).Active()["claude"]; e.Strikes != 1 {
		t.Errorf("strikes = %d, want the original single strike", e.Strikes)
	}
}

func TestBenchOnEscalation_BenchWriteFailureIsLoggedAsWarnAndDoesNotPanic(t *testing.T) {
	now := time.Date(2026, 10, 6, 15, 32, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	root, ws := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".evolve"), []byte("a file where the state dir should be"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeReportF4(t, ws, map[string]any{"captured_at": now, "cli": "claude-tmux", "pattern_name": "exhausted", "pane_tail": "limit"})
	sink := &logSinkF4{}

	bridgechain.BenchOnEscalation(bridgechain.Escalation{ProjectRoot: root, Workspace: ws, CLI: "claude-tmux", DispatchStart: now.Add(-time.Minute)}, clock, sink.logf)

	if !strings.Contains(sink.joined(), "WARN cli-health bench write failed") {
		t.Errorf("log = %q", sink.joined())
	}
	if strings.Contains(sink.joined(), "benched family") {
		t.Errorf("success must not be logged after a failed write: %q", sink.joined())
	}
}
