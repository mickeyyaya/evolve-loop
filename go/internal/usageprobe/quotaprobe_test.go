package usageprobe

// quotaprobe_test.go — ProbeQuota is the MEASUREMENT sibling of the boolean
// Prober: it captures each family's /usage pane through the same injected probe
// seam and PARSES it into a quotastate.QuotaState (instead of discarding the
// numbers after a capped/not classification). Fail-open: a family whose probe is
// unsupported (ollama) or errors is omitted — never a fabricated state. The
// budget allocator consumes the returned slice.

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

// a minimal REAL-shaped claude /usage pane: one numeric window is enough for
// Parse to return Source=probed (bucket fidelity is quotastate's own test).
const claudeUsagePane = "" +
	"   Current session\n" +
	"   █████████████▌                                     27% used\n" +
	"   Resets 4:10pm (Asia/Taipei)\n"

// TestProbeQuota_ParsesHealthyOmitsFailed pins the two core contracts: a family
// whose probe succeeds is parsed to a probed QuotaState; a family whose probe is
// unsupported OR errors is OMITTED (fail-open, no fabricated entry). Naming
// ProbeQuota by identifier also satisfies apicover -enforce for this package.
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
			return "some banner text with no usage numbers at all\n", nil // responds, unparseable
		default:
			return "", nil
		}
	}

	// claude parses (kept); codex errors, ollama is unsupported, garble responds
	// but yields no numbers (Source=unknown) — all three omitted.
	got := ProbeQuota(context.Background(), []string{"claude", "codex", "ollama", "garble"}, probe, now)

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

// TestProbeQuota_ConcurrentFailOpen exercises the concurrent fan-out with a mix
// of healthy/errored families under -race: the goroutines append to a shared
// slice, so the race detector guards the accumulation, and a panicking-free
// error path proves fail-open. Every "healthy-<n>" family must survive.
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

	got := ProbeQuota(context.Background(), families, probe, now)

	if len(got) != 8 {
		t.Fatalf("want 8 healthy states, got %d", len(got))
	}
	for _, q := range got {
		if q.Source != quotastate.SourceProbed {
			t.Errorf("%s Source = %q, want probed", q.Family, q.Source)
		}
	}
}

// TestProbeQuota_EmptyFamilies returns an empty slice (never nil-deref) for no
// families — the shadow-safe caller passes nil when budgeting is off.
func TestProbeQuota_EmptyFamilies(t *testing.T) {
	if got := ProbeQuota(context.Background(), nil, func(context.Context, string) (string, error) {
		return "", nil
	}, time.Now()); len(got) != 0 {
		t.Errorf("ProbeQuota(nil families) = %+v, want empty", got)
	}
}
