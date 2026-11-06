package bridgechain_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
)

func writeReport(t *testing.T, ws, cli, pattern, tail string, at time.Time) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"captured_at": at, "cli": cli, "pattern_name": pattern, "pane_tail": tail})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "escalation-report.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAttemptEvidence(t *testing.T) {
	start := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	now := start.Add(time.Minute)
	cases := []struct {
		name      string
		cli       string
		pattern   string
		at        time.Time
		tail      string
		wantPath  bool
		wantReset time.Time
	}{
		{"matching report with hint", "agy", "rate_limit", start.Add(time.Second), "resets at 9:00pm", true, time.Date(2026, 11, 6, 21, 0, 0, 0, time.UTC)},
		{"matching report without hint", "agy", "rate_limit", start.Add(time.Second), "limit reached", true, time.Time{}},
		{"other cli", "codex", "rate_limit", start.Add(time.Second), "resets at 9:00pm", false, time.Time{}},
		{"other pattern", "agy", "auth_recheck", start.Add(time.Second), "resets at 9:00pm", false, time.Time{}},
		{"stale report", "agy", "rate_limit", start.Add(-time.Second), "resets at 9:00pm", false, time.Time{}},
		{"login report has no reset", "agy", "auth_recheck", start.Add(time.Second), "resets at 9:00pm", true, time.Time{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			writeReport(t, ws, tc.cli, tc.pattern, tc.tail, tc.at)
			want := "rate_limit"
			if tc.name == "login report has no reset" {
				want = "auth_recheck"
			}
			path, reset := bridgechain.AttemptEvidence(bridgechain.Escalation{Workspace: ws, CLI: "agy", DispatchStart: start}, want, now)
			if (path != "") != tc.wantPath {
				t.Fatalf("path=%q wantPath=%v", path, tc.wantPath)
			}
			if tc.wantPath {
				if filepath.Dir(path) != ws || path == filepath.Join(ws, "escalation-report.json") {
					t.Fatalf("copy must sit beside the report under its own name, got %q", path)
				}
				if _, err := os.Stat(path); err != nil {
					t.Fatalf("copy missing: %v", err)
				}
			}
			if !reset.Equal(tc.wantReset) {
				t.Fatalf("reset=%v want %v", reset, tc.wantReset)
			}
		})
	}
}

func TestAttemptEvidence_OneNamePerAttempt(t *testing.T) {
	start := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	ws := t.TempDir()
	writeReport(t, ws, "agy", "rate_limit", "", start.Add(time.Second))
	a, _ := bridgechain.AttemptEvidence(bridgechain.Escalation{Workspace: ws, CLI: "agy", Tier: "t1", DispatchStart: start}, "rate_limit", start)
	b, _ := bridgechain.AttemptEvidence(bridgechain.Escalation{Workspace: ws, CLI: "agy", Tier: "t2", DispatchStart: start}, "rate_limit", start)
	if a == "" || a == b {
		t.Fatalf("each attempt needs its own copy: %q %q", a, b)
	}
}

func TestAttemptEvidence_AnUnreadableReportYieldsNothing(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "escalation-report.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, reset := bridgechain.AttemptEvidence(bridgechain.Escalation{Workspace: ws, CLI: "agy", DispatchStart: start}, "rate_limit", start)
	if path != "" || !reset.IsZero() {
		t.Fatalf("path=%q reset=%v", path, reset)
	}
}

func TestAttemptEvidence_ACopyThatCannotBeWrittenYieldsNothing(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	ws := t.TempDir()
	writeReport(t, ws, "agy", "rate_limit", "resets at 9:00pm", start.Add(time.Second))
	probe, _ := bridgechain.AttemptEvidence(bridgechain.Escalation{Workspace: ws, CLI: "agy", Tier: "t1", DispatchStart: start}, "rate_limit", start)
	if err := os.Remove(probe); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(probe, 0o755); err != nil {
		t.Fatal(err)
	}
	path, reset := bridgechain.AttemptEvidence(bridgechain.Escalation{Workspace: ws, CLI: "agy", Tier: "t1", DispatchStart: start}, "rate_limit", start)
	if path != "" || !reset.IsZero() {
		t.Fatalf("path=%q reset=%v", path, reset)
	}
}

func TestAttemptEvidence_AMissingReportYieldsNothing(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	path, reset := bridgechain.AttemptEvidence(bridgechain.Escalation{Workspace: t.TempDir(), CLI: "agy", DispatchStart: start}, "rate_limit", start)
	if path != "" || !reset.IsZero() {
		t.Fatalf("path=%q reset=%v", path, reset)
	}
}
