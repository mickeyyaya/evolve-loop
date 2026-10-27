package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

func TestCLIRoutingExplain_PrintsTheShareAndThePaceOfASharedTier(t *testing.T) {
	root := routingFixture(t, `{"cli_routing":{"clis":["agy","claude"],"default":["agy","claude"],"tiers":{"deep":{"clis":["agy","claude"],"share_pct":40}}}}`)

	rc, out, _ := runRoutingVerb(t, "explain", "scout", "--static", "--project-root", root)

	if rc != 0 || !strings.Contains(out, "note:    share: when the agent dispatches at deep, agy takes 40% of the seats; ") {
		t.Fatalf("explain scout: rc=%d\n%s", rc, out)
	}
}

func TestProjectTierWeek_ReadsTheWindowOfTheTierModelOfTheFamily(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	reset := now.Add(100 * time.Hour)
	if err := usageprobe.RecordObservation(evolveDir, usageprobe.Observation{CLI: "agy", ObservedAt: now, Windows: []quotastate.UsageWindow{
		{Scope: "Gemini 4 Argon", Kind: quotastate.KindWeek, PercentUsed: 22, ResetsAt: &reset, Family: "agy", Model: "Gemini 4 Argon"},
	}}); err != nil {
		t.Fatal(err)
	}
	var asked []string
	tierModel := func(cli, tier string) (string, bool) {
		asked = append(asked, cli+"@"+tier)
		return "Gemini 4 Argon (Medium)", tier == "deep"
	}
	week := projectTierWeek(func() time.Time { return now }, tierModel)

	w, ok, err := week(root, "agy", "deep")
	if err != nil || !ok || w.PercentUsed != 22 {
		t.Fatalf("week = %+v, %v, %v; want the Argon window", w, ok, err)
	}
	if _, ok, err := week(root, "agy", "fast"); ok || err != nil {
		t.Fatalf("a tier with no model in the manifest has no window: %v %v", ok, err)
	}
	if strings.Join(asked, ",") != "agy@deep,agy@fast" {
		t.Fatalf("the tier model was asked as %v", asked)
	}
}

func TestResolveModelTier_TheAgyDeepSeatIsArgon(t *testing.T) {
	t.Parallel()
	model, ok := resolveModelTier("agy", "deep")
	if !ok || !strings.HasPrefix(model, "Gemini 4 Argon") {
		t.Fatalf("resolveModelTier(agy, deep) = %q, %v; the share pace reads the Argon window through this model", model, ok)
	}
}

const shareTable = `{"cli_routing": {"clis": ["agy", "agy-claude", "claude"], "default": ["agy", "agy-claude", "claude"], "tiers": {"deep": ["agy", "agy-claude", "claude"]}}}`

func TestCLIRoutingSet_TheShareOfATierIsWrittenAndUnset(t *testing.T) {
	root := routingWriteProject(t, shareTable)

	if rc, _, stderr := runRoutingVerb(t, inRoot(root, "set", "tiers.deep", "--share-pct", "50")...); rc != 0 {
		t.Fatalf("set --share-pct: rc=%d stderr=%s", rc, stderr)
	}
	pol, raw := readRoutingPolicy(t, root)
	if got := pol.CLIRouting.Tiers["deep"]; got.SharePct != 50 || strings.Join(got.CLIs, ",") != "agy,agy-claude,claude" {
		t.Fatalf("tiers.deep = %+v, want the chain kept and share_pct 50\n%s", got, raw)
	}
	if rc, _, stderr := runRoutingVerb(t, inRoot(root, "unset", "tiers.deep.share_pct")...); rc != 0 {
		t.Fatalf("unset share_pct: rc=%d stderr=%s", rc, stderr)
	}
	if pol, _ := readRoutingPolicy(t, root); pol.CLIRouting.Tiers["deep"].SharePct != 0 || len(pol.CLIRouting.Tiers["deep"].CLIs) != 3 {
		t.Fatalf("after unset tiers.deep = %+v, want the chain without a share", pol.CLIRouting.Tiers["deep"])
	}
}

func TestCLIRoutingSet_AShareOutsideATierOrOutOfRangeIsRefused(t *testing.T) {
	root := routingWriteProject(t, shareTable)
	for _, args := range [][]string{
		{"set", "agents.builder", "agy", "--share-pct", "50"},
		{"set", "tiers.deep", "--share-pct", "100"},
		{"unset", "agents.builder.share_pct"},
		{"unset", "tiers.top.share_pct"},
	} {
		if rc, _, _ := runRoutingVerb(t, inRoot(root, args...)...); rc == 0 {
			t.Errorf("%v: rc=0, want a refusal", args)
		}
	}
	if pol, _ := readRoutingPolicy(t, root); pol.CLIRouting.Tiers["deep"].SharePct != 0 {
		t.Fatalf("a refused edit wrote %+v", pol.CLIRouting.Tiers["deep"])
	}
}

func TestRoutingHost_CarriesTheTierWeekReaderTheClockAndABenchThatLogs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	if err := clihealth.NewStore(root, func() time.Time { return now }).Bench(clihealth.Entry{Family: "agy", Reason: "rate_limit", BenchedAt: now, BenchedUntil: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer

	host := routingHost(&log, func() time.Time { return now })

	if host.TierWeek == nil || host.Now == nil || !host.Now().Equal(now) {
		t.Fatalf("the production host must carry the tier week reader and the clock: %+v", host)
	}
	host.Bench(root, "build", llmroute.Plan{Candidates: []string{"agy-tmux", "claude-tmux"}})
	if !strings.Contains(log.String(), "[cliroute] ") {
		t.Fatalf("a bench move must log through the host, got %q", log.String())
	}
}
