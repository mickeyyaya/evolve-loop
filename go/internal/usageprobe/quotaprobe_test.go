package usageprobe

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

const claudeUsagePane = "" +
	"   Current session\n" +
	"   █████████████▌                                     27% used\n" +
	"   Resets 4:10pm (Asia/Taipei)\n"

func readSession(family, pane string, _ time.Time) []quotastate.UsageWindow {
	if pane != claudeUsagePane {
		return nil
	}
	return []quotastate.UsageWindow{{Scope: "session", Kind: quotastate.KindSession, PercentUsed: 27, Family: family}}
}

func TestProbeQuota_OneScreenReportsEveryFamilyItsWindowsTarget(t *testing.T) {
	now := time.Date(2026, time.July, 3, 12, 0, 0, 0, time.UTC)
	read := func(string, string, time.Time) []quotastate.UsageWindow {
		return []quotastate.UsageWindow{
			{Scope: "GEMINI MODELS", Kind: quotastate.KindWeek, PercentUsed: 8.3, Family: "agy"},
			{Scope: "CLAUDE AND GPT MODELS", Kind: quotastate.KindFiveHour, PercentUsed: 100, Family: "agy-claude", Exhausted: true},
		}
	}

	got := ProbeQuota(context.Background(), []string{"agy"}, QuotaReader{Probe: func(context.Context, string) (string, error) { return "agy /usage", nil }, Read: read, Now: now})

	byFamily := map[string]quotastate.QuotaState{}
	for _, q := range got {
		byFamily[q.Family] = q
	}
	if len(got) != 2 || byFamily["agy"].Exhausted || !byFamily["agy-claude"].Exhausted {
		t.Fatalf("states = %+v; want a healthy agy and an exhausted agy-claude from one screen", got)
	}
}

func TestProbeQuota_ParsesHealthyOmitsFailed(t *testing.T) {
	now := time.Date(2026, time.July, 3, 12, 0, 0, 0, time.UTC)
	probe := func(_ context.Context, family string) (string, error) {
		switch family {
		case "claude":
			return claudeUsagePane, nil
		case "ollama":
			return "", fmt.Errorf("no usage command: %w", clicontrol.ErrUnsupported)
		case "codex":
			return "", errors.New("bridge timeout")
		case "garble":
			return "some banner text with no usage numbers at all\n", nil
		default:
			return "", nil
		}
	}

	got := ProbeQuota(context.Background(), []string{"claude", "codex", "ollama", "garble"}, QuotaReader{Probe: probe, Read: readSession, Now: now})

	if len(got) != 1 {
		t.Fatalf("ProbeQuota returned %d states, want 1 (only claude reported); got=%+v", len(got), got)
	}
	if got[0].Family != "claude" {
		t.Errorf("Family = %q, want claude", got[0].Family)
	}
	if got[0].Source != quotastate.SourceProbed {
		t.Errorf("Source = %q, want %q (numeric pane parsed)", got[0].Source, quotastate.SourceProbed)
	}
	if got[0].ObservedAt != now {
		t.Errorf("ObservedAt = %v, want injected now %v", got[0].ObservedAt, now)
	}
}

func TestProbeQuota_ConcurrentFailOpen(t *testing.T) {
	now := time.Date(2026, time.July, 3, 12, 0, 0, 0, time.UTC)
	families := []string{}
	for i := 0; i < 8; i++ {
		families = append(families, fmt.Sprintf("healthy-%d", i), fmt.Sprintf("dead-%d", i))
	}
	probe := func(_ context.Context, family string) (string, error) {
		if len(family) >= 7 && family[:7] == "healthy" {
			return claudeUsagePane, nil
		}
		return "", errors.New("down")
	}

	got := ProbeQuota(context.Background(), families, QuotaReader{Probe: probe, Read: readSession, Now: now})

	if len(got) != 8 {
		t.Fatalf("want 8 healthy states, got %d", len(got))
	}
	for _, q := range got {
		if q.Source != quotastate.SourceProbed {
			t.Errorf("%s Source = %q, want probed", q.Family, q.Source)
		}
	}
}

func TestProbeQuota_EmptyFamilies(t *testing.T) {
	if got := ProbeQuota(context.Background(), nil, QuotaReader{Probe: func(context.Context, string) (string, error) {
		return "", nil
	}, Read: readSession, Now: time.Now()}); len(got) != 0 {
		t.Errorf("ProbeQuota(nil families) = %+v, want empty", got)
	}
}
