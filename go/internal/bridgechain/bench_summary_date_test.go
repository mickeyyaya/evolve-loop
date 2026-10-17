package bridgechain_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

func benchReorderLine(t *testing.T, now, until time.Time) string {
	t.Helper()
	root := t.TempDir()
	clock := func() time.Time { return now }
	if err := clihealth.NewStore(root, clock).Bench(clihealth.Entry{
		Family: "agy-claude", Reason: "usage_probe", BenchedAt: now, BenchedUntil: until, Strikes: 1,
	}); err != nil {
		t.Fatal(err)
	}
	var logs []string
	plan := llmroute.Plan{Candidates: []string{"agy-claude-tmux", "claude-tmux"}}
	bridgechain.ApplyCLIHealthBench(root, "builder", plan, map[string]string{}, clock, func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) })
	if len(logs) != 1 || !strings.Contains(logs[0], "bench reordered chain") {
		t.Fatalf("a benched lead family logs one reorder line: %q", logs)
	}
	return logs[0]
}

func TestApplyCLIHealthBench_TheReorderLineCarriesTheDateOfABenchEndingOnALaterDay(t *testing.T) {
	cases := []struct {
		name      string
		until     time.Time
		want      string
		forbidden string
	}{
		{"a bench ending tomorrow", time.Date(2026, 10, 8, 21, 42, 0, 0, time.Local), "agy-claude until 2026-10-08 21:42 (usage_probe)", ""},
		{"a bench ending in a later month", time.Date(2026, 11, 2, 9, 5, 0, 0, time.Local), "agy-claude until 2026-11-02 09:05 (usage_probe)", ""},
		{"a bench ending later today", time.Date(2026, 10, 7, 23, 30, 0, 0, time.Local), "agy-claude until 23:30 (usage_probe)", "2026-10-07"},
	}
	now := time.Date(2026, 10, 7, 22, 43, 0, 0, time.Local)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			line := benchReorderLine(t, now, tc.until)
			if !strings.Contains(line, tc.want) {
				t.Errorf("reorder line lacks %q:\n%s", tc.want, line)
			}
			if tc.forbidden != "" && strings.Contains(line, tc.forbidden) {
				t.Errorf("a bench ending today keeps the short clock form, no %q:\n%s", tc.forbidden, line)
			}
		})
	}
}
