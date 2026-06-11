package looppreflight

import (
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

func resolvedWithBenches(t *testing.T, entries []clihealth.Entry) resolved {
	t.Helper()
	o, err := resolve(Options{
		ProjectRoot:     t.TempDir(),
		CLIHealthActive: func() []clihealth.Entry { return entries },
	})
	if err != nil {
		t.Fatal(err)
	}
	return o
}

// TestCheckCLIHealthPassWhenNoBenches: an empty bench store is a clean pass.
func TestCheckCLIHealthPassWhenNoBenches(t *testing.T) {
	t.Parallel()
	res := checkCLIHealth(resolvedWithBenches(t, nil))
	if res.Level != LevelPass {
		t.Errorf("level=%v, want pass", res.Level)
	}
}

// TestCheckCLIHealthWarnsNamedFamilies: active benches surface as a WARN
// naming the family, the until-time, and the fallback-first consequence —
// never a Halt (the fallback chain exists for exactly this).
func TestCheckCLIHealthWarnsNamedFamilies(t *testing.T) {
	t.Parallel()
	until := time.Date(2026, 6, 11, 6, 13, 0, 0, time.UTC)
	res := checkCLIHealth(resolvedWithBenches(t, []clihealth.Entry{
		{Family: "codex", Reason: "rate_limit", BenchedUntil: until, Strikes: 2},
	}))
	if res.Level != LevelWarn {
		t.Fatalf("level=%v, want warn (never halt — fallbacks exist)", res.Level)
	}
	for _, want := range []string{"codex", "rate_limit", "06:13", "fallback"} {
		if !strings.Contains(res.Detail, want) {
			t.Errorf("detail missing %q: %s", want, res.Detail)
		}
	}
}

// TestCheckCLIHealthDefaultSeamReadsStore: the default seam reads the real
// .evolve/cli-health.json under ProjectRoot.
func TestCheckCLIHealthDefaultSeamReadsStore(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Now()
	if err := clihealth.NewStore(root, nil).Bench(clihealth.Entry{
		Family: "codex", Reason: "rate_limit", BenchedAt: now, BenchedUntil: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	o, err := resolve(Options{ProjectRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	res := checkCLIHealth(o)
	if res.Level != LevelWarn || !strings.Contains(res.Detail, "codex") {
		t.Errorf("default seam did not surface the real bench: level=%v detail=%s", res.Level, res.Detail)
	}
}
