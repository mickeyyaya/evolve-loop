package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/clihealth"
)

func benchExpired(t *testing.T, root, family string, strikes int) {
	t.Helper()
	now := time.Now()
	if err := clihealth.NewStore(root, nil).Bench(clihealth.Entry{
		Family: family, Reason: "rate_limit",
		BenchedAt: now.Add(-2 * time.Hour), BenchedUntil: now.Add(-time.Hour), Strikes: strikes,
	}); err != nil {
		t.Fatal(err)
	}
}

// TestCanaryRecoveryClearsBench: an expired bench whose probe succeeds is
// cleared — the family is healthy again with one cheap probe instead of a
// full phase stall.
func TestCanaryRecoveryClearsBench(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchExpired(t, root, "codex", 1)
	var probed []string
	var out bytes.Buffer
	runCLIHealthCanary(root, nil, func(driver string) (int, string, string) {
		probed = append(probed, driver)
		return 0, "", ""
	}, &out)
	if len(probed) != 1 || probed[0] != "codex-tmux" {
		t.Fatalf("probed=%v, want one probe of codex-tmux", probed)
	}
	if benches, _ := clihealth.NewStore(root, nil).Load(); len(benches) != 0 {
		t.Errorf("recovered family still benched: %v", benches)
	}
}

// TestCanaryStillWalledRebenchesWithStrike: a probe that hits the wall again
// re-benches with strikes+1 (doubled cooldown when no reset hint parses).
func TestCanaryStillWalledRebenchesWithStrike(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchExpired(t, root, "codex", 1)
	var out bytes.Buffer
	runCLIHealthCanary(root, nil, func(driver string) (int, string, string) {
		return 85, "rate_limit", "no reset hint in this pane"
	}, &out)
	benches, _ := clihealth.NewStore(root, nil).Load()
	e, ok := benches["codex"]
	if !ok {
		t.Fatal("still-walled family lost its bench")
	}
	if e.Strikes != 2 {
		t.Errorf("strikes=%d, want 2", e.Strikes)
	}
	if d := e.BenchedUntil.Sub(e.BenchedAt); d < 55*time.Minute || d > 65*time.Minute {
		t.Errorf("cooldown=%v, want ~1h (doubled from 30m at strike 2)", d)
	}
}

// TestCanaryWallWithResetHintUsesIt: the re-bench honors the pane's own reset
// hint over the strike cooldown.
func TestCanaryWallWithResetHintUsesIt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchExpired(t, root, "codex", 1)
	var out bytes.Buffer
	runCLIHealthCanary(root, nil, func(driver string) (int, string, string) {
		return 85, "rate_limit", "You've hit your usage limit. try again in 3 hours."
	}, &out)
	benches, _ := clihealth.NewStore(root, nil).Load()
	e := benches["codex"]
	if d := e.BenchedUntil.Sub(e.BenchedAt); d < 3*time.Hour || d > 3*time.Hour+5*time.Minute {
		t.Errorf("benched_until-benched_at=%v, want ~3h (the pane's own hint)", d)
	}
}

// TestCanaryNonWallFailureClears: a non-wall probe failure clears the bench —
// other failure classes belong to the normal dispatch machinery.
func TestCanaryNonWallFailureClears(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	benchExpired(t, root, "codex", 1)
	var out bytes.Buffer
	runCLIHealthCanary(root, nil, func(driver string) (int, string, string) {
		return 80, "", "boot timeout"
	}, &out)
	if benches, _ := clihealth.NewStore(root, nil).Load(); len(benches) != 0 {
		t.Errorf("non-wall failure left the bench in place: %v", benches)
	}
	if !strings.Contains(out.String(), "not a wall") {
		t.Errorf("expected loud not-a-wall log, got: %s", out.String())
	}
}

// TestCanarySkipsActiveBenchesAndDisabledEnv: active benches are not probed;
// EVOLVE_CLI_HEALTH=0 disables the whole canary.
func TestCanarySkipsActiveBenchesAndDisabledEnv(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Now()
	_ = clihealth.NewStore(root, nil).Bench(clihealth.Entry{
		Family: "codex", Reason: "rate_limit", BenchedAt: now, BenchedUntil: now.Add(time.Hour),
	})
	probes := 0
	var out bytes.Buffer
	runCLIHealthCanary(root, nil, func(string) (int, string, string) { probes++; return 0, "", "" }, &out)
	if probes != 0 {
		t.Errorf("ACTIVE bench was probed (%d probes) — only expired benches get the canary", probes)
	}

	benchExpired(t, root, "agy", 1)
	runCLIHealthCanary(root, map[string]string{"EVOLVE_CLI_HEALTH": "0"},
		func(string) (int, string, string) { probes++; return 0, "", "" }, &out)
	if probes != 0 {
		t.Errorf("EVOLVE_CLI_HEALTH=0 still probed (%d)", probes)
	}
}
