package bridgechain_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

func writeAgyClaudeWall(t *testing.T, ws string, captured time.Time) {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"captured_at": captured, "cli": "agy-claude-tmux", "pattern_name": "exhausted",
		"pane_tail": "⚠ Individual quota reached. Please upgrade your subscription to increase your limits. Resets in 1h3m47s."})
	if err := os.WriteFile(filepath.Join(ws, "escalation-report.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBenchOnEscalation_OneReportBenchesItsFamilyOnceAndKeepsALongerBench(t *testing.T) {
	root, ws := t.TempDir(), t.TempDir()
	now := time.Date(2026, 10, 6, 15, 32, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	writeAgyClaudeWall(t, ws, now.Add(-time.Second))
	store := clihealth.NewStore(root, clock)
	untilReset := now.Add(2 * time.Hour)
	if _, err := store.BenchWallUntil("agy-claude", clihealth.Wall{Pattern: "usage_probe", Evidence: "CLAUDE AND GPT MODELS 5h window 100% used", Reset: untilReset}); err != nil {
		t.Fatal(err)
	}

	bridgechain.BenchOnEscalation(bridgechain.Escalation{ProjectRoot: root, Workspace: ws, CLI: "agy-claude-tmux", DispatchStart: now.Add(-time.Minute), Env: map[string]string{}}, clock, nil)

	e := store.Active()["agy-claude"]
	if e.Strikes != 1 || e.BenchedUntil.Before(untilReset) {
		t.Fatalf("bench = %+v; the report was already benched for this wall, so it must neither add a strike nor shorten the bench", e)
	}
}

func TestBenchOnEscalation_AWallSeenAfterTheLastBenchIsANewStrikeWhetherOrNotThatBenchIsStillActive(t *testing.T) {
	now := time.Date(2026, 10, 6, 15, 32, 0, 0, time.UTC)
	wallSeen := now.Add(-time.Second)
	for name, benchedAt := range map[string]time.Time{
		"an hour-old bench that has expired":             now.Add(-time.Hour),
		"a bench thirty seconds before, still in effect": wallSeen.Add(-30 * time.Second),
	} {
		t.Run(name, func(t *testing.T) {
			root, ws := t.TempDir(), t.TempDir()
			clock := func() time.Time { return now }
			if _, err := clihealth.NewStore(root, func() time.Time { return benchedAt }).BenchWall("agy-claude", "exhausted", "an earlier wall"); err != nil {
				t.Fatal(err)
			}
			writeAgyClaudeWall(t, ws, wallSeen)

			bridgechain.BenchOnEscalation(bridgechain.Escalation{ProjectRoot: root, Workspace: ws, CLI: "agy-claude-tmux", DispatchStart: now.Add(-time.Minute), Env: map[string]string{}}, clock, nil)

			if e := clihealth.NewStore(root, clock).Active()["agy-claude"]; e.Strikes != 2 || !e.BenchedAt.Equal(now) {
				t.Fatalf("bench = %+v; a wall captured after the last bench is a new strike", e)
			}
		})
	}
}
