package usageevidence

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

func usageFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "quotastate", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestNew_ReadsTheFailingDriversBinaryScreenAndJudgesItsRoutingFamily(t *testing.T) {
	root := t.TempDir()
	var probed []string
	explain := New(Options{
		ProjectRoot: root, EvolveDir: filepath.Join(root, ".evolve"), Act: true,
		Probe: func(_ context.Context, family string) (string, error) {
			probed = append(probed, family)
			return usageFixture(t, "agy_usage_claude_drained.txt"), nil
		},
	})

	ev := explain(context.Background(), "agy-claude-tmux", time.Time{})

	if ev.Verdict != usageprobe.VerdictExhausted || ev.CLI != "agy" || ev.Family != "agy-claude" || len(probed) != 1 || probed[0] != "agy" {
		t.Fatalf("evidence %+v after probing %v; want agy's screen read once and agy-claude judged exhausted", ev, probed)
	}
	if bench, ok := clihealth.NewStore(root, nil).Active()["agy-claude"]; !ok || !bench.BenchedUntil.After(time.Now().Add(2*time.Hour)) {
		t.Errorf("bench = %+v (present=%v); want agy-claude benched until the drained window's 2h15m reset", bench, ok)
	}
	if gemini := explain(context.Background(), "agy-tmux", time.Time{}); gemini.Verdict != usageprobe.VerdictHealthy || len(probed) != 1 {
		t.Errorf("agy-tmux = %+v after %d probes; want healthy from the cached screen", gemini, len(probed))
	}
}

func TestNew_TheTimeoutAndTTLComeFromTheCLIHealthBlock(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	probes, remaining := 0, time.Duration(0)
	explain := New(Options{
		ProjectRoot: root, EvolveDir: filepath.Join(root, ".evolve"), Now: func() time.Time { return now },
		Config: policy.CLIHealthConfig{UsageEvidenceTimeoutS: 45, UsageEvidenceTTLS: 90},
		Probe: func(ctx context.Context, _ string) (string, error) {
			probes++
			if deadline, ok := ctx.Deadline(); ok {
				remaining = time.Until(deadline)
			}
			return "", errors.New("the CLI did not answer")
		},
	})

	ev := explain(context.Background(), "claude-tmux", time.Time{})

	if ev.Verdict != usageprobe.VerdictUnavailable || remaining <= 44*time.Second || remaining > 45*time.Second {
		t.Fatalf("evidence %+v with the probe's deadline %v away; want unavailable under the configured 45s timeout", ev, remaining)
	}
	now = now.Add(89 * time.Second)
	explain(context.Background(), "claude-tmux", time.Time{})
	if probes != 1 {
		t.Errorf("%d probes inside the configured 90s TTL, want 1", probes)
	}
	now = now.Add(2 * time.Second)
	explain(context.Background(), "claude-tmux", time.Time{})
	if probes != 2 {
		t.Errorf("%d probes once the configured TTL passed, want 2", probes)
	}
}
