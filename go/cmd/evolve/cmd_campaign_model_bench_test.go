package main

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

func TestCampaignQuotaCooldown_AModelBenchIsNoReasonToWait(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store := clihealth.NewStore(root, nil)
	if _, err := store.BenchScopedWall("codex", "gpt-5.6-sol", clihealth.ModelUnsupportedPattern, "not supported"); err != nil {
		t.Fatal(err)
	}
	if got := campaignQuotaCooldown(root)(); got != 0 {
		t.Fatalf("cooldown = %v: the chain already routes around an unsupported model, so a campaign retry does not wait for it", got)
	}
	if _, err := store.BenchScopedWall("agy", "Gemini 4 Argon", "quota_exhausted", "Individual quota reached. Resets in 2h."); err != nil {
		t.Fatal(err)
	}
	if got := campaignQuotaCooldown(root)(); got < time.Hour {
		t.Errorf("cooldown = %v, want the quota wall's wait", got)
	}
}

func TestCampaignQuotaCooldown_NeverWaitsPastTheMaxCooldown(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := clihealth.NewStore(root, nil).Bench(clihealth.Entry{Family: "codex", Reason: "rate_limit", BenchedAt: time.Now(), BenchedUntil: time.Now().Add(48 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if got := campaignQuotaCooldown(root)(); got != clihealth.MaxCooldown {
		t.Fatalf("cooldown = %v, want the cap %v", got, clihealth.MaxCooldown)
	}
}
