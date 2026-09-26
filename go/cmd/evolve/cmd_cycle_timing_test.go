package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

func writeTimingFixture(t *testing.T, root string, cycle string, entries []phasetiming.Entry) {
	t.Helper()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-"+cycle)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(entries)
	if err := os.WriteFile(filepath.Join(ws, phasetiming.FileName), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunCycleTiming_RendersTableAndRollup(t *testing.T) {
	root := t.TempDir()
	writeTimingFixture(t, root, "42", []phasetiming.Entry{
		{Phase: "scout", DurationMS: 400_000, Verdict: "PASS", Archetype: "plan", AttemptCount: 1, StartedAt: "2026-06-25T00:00:00Z", EndedAt: "2026-06-25T00:06:40Z"},
		{Phase: "build", DurationMS: 700_000, Verdict: "PASS", Archetype: "build", AttemptCount: 2, StartedAt: "2026-06-25T00:06:40Z", EndedAt: "2026-06-25T00:18:20Z"},
		{Phase: "audit", DurationMS: 300_000, Verdict: "PASS", Archetype: "evaluate", AttemptCount: 1},
	})

	var out, errb bytes.Buffer
	code := runCycleTiming([]string{"--project-root", root, "42"}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit=%d, stderr=%s", code, errb.String())
	}
	s := out.String()
	for _, want := range []string{"scout", "build", "audit", "plan", "evaluate"} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q:\n%s", want, s)
		}
	}
	if !strings.Contains(s, "Longest: build") {
		t.Errorf("roll-up must name the longest phase 'build'; got:\n%s", s)
	}
}

func TestRunCycleTiming_ParallelProjection(t *testing.T) {
	root := t.TempDir()
	writeTimingFixture(t, root, "55", []phasetiming.Entry{
		{Phase: "build", DurationMS: 600_000, Verdict: "PASS", Archetype: "build", AttemptCount: 1},
		{Phase: "coverage-gate", DurationMS: 240_000, Verdict: "PASS", Archetype: "evaluate", AttemptCount: 1},
		{Phase: "adversarial-review", DurationMS: 300_000, Verdict: "PASS", Archetype: "evaluate", AttemptCount: 1},
		{Phase: "audit", DurationMS: 290_000, Verdict: "PASS", Archetype: "evaluate", AttemptCount: 1},
	})
	var out, errb bytes.Buffer
	if code := runCycleTiming([]string{"--project-root", root, "--concurrency", "2", "55"}, &out, &errb); code != 0 {
		t.Fatalf("exit=%d, stderr=%s", code, errb.String())
	}
	s := out.String()
	if !strings.Contains(s, "Parallel-evaluate projection") {
		t.Errorf("expected the shadow projection line; got:\n%s", s)
	}
	// audit is excluded as the brancher, so the group is coverage-gate + adversarial-review.
	if !strings.Contains(s, "coverage-gate") || strings.Contains(s, "audit — ") {
		t.Errorf("projection group must include checking phases and exclude audit; got:\n%s", s)
	}
}

func TestRunCycleTiming_DefaultsToLatestCycle(t *testing.T) {
	root := t.TempDir()
	writeTimingFixture(t, root, "7", []phasetiming.Entry{{Phase: "scout", DurationMS: 1000, Archetype: "plan", AttemptCount: 1}})
	writeTimingFixture(t, root, "41", []phasetiming.Entry{{Phase: "tdd", DurationMS: 2000, Archetype: "plan", AttemptCount: 1}})

	var out, errb bytes.Buffer
	if code := runCycleTiming([]string{"--project-root", root}, &out, &errb); code != 0 {
		t.Fatalf("exit=%d, stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "cycle-41") {
		t.Errorf("default must select the latest cycle (41); got:\n%s", out.String())
	}
}

func TestRunCycleTiming_JSON(t *testing.T) {
	root := t.TempDir()
	writeTimingFixture(t, root, "9", []phasetiming.Entry{
		{Phase: "build", DurationMS: 5000, Archetype: "build", AttemptCount: 1},
	})
	var out, errb bytes.Buffer
	if code := runCycleTiming([]string{"--project-root", root, "--json", "9"}, &out, &errb); code != 0 {
		t.Fatalf("exit=%d, stderr=%s", code, errb.String())
	}
	var got phasetiming.Summary
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("--json must emit a valid Summary: %v\n%s", err, out.String())
	}
	if got.LongestPhase != "build" {
		t.Errorf("summary LongestPhase=%q, want build", got.LongestPhase)
	}
}

func TestRunCycleTiming_NoLogs(t *testing.T) {
	root := t.TempDir()
	var out, errb bytes.Buffer
	if code := runCycleTiming([]string{"--project-root", root}, &out, &errb); code == 0 {
		t.Errorf("must fail when no timing logs exist; got exit 0")
	}
}
