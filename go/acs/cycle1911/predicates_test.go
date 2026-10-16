//go:build acs

package cycle1911

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

const (
	argonGroup       = "Gemini 4 Argon"
	agyArgonWallLine = "Individual quota reached for this model. Resets in 148h8m53s."
	unrelatedPaneTop = "  ⣾ Reading go/internal/clihealth/clihealth.go"
)

var agyChain = []string{"agy-tmux", "agy-claude-tmux", "claude-tmux"}

func TestMain(m *testing.M) {
	emptyCatalogDir, err := os.MkdirTemp("", "acs-cycle1911-catalog-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "mktemp for the empty model catalog: %v\n", err)
		os.Exit(1)
	}
	bridge.SetModelCatalogDirFn(func() string { return emptyCatalogDir })
	code := m.Run()
	if err := os.RemoveAll(emptyCatalogDir); err != nil {
		fmt.Fprintf(os.Stderr, "remove the empty model catalog %s: %v\n", emptyCatalogDir, err)
	}
	os.Exit(code)
}

func fixedNow() time.Time { return time.Date(2026, 10, 17, 2, 17, 0, 0, time.UTC) }

func seatPlan(tier string) llmroute.Plan {
	return llmroute.Plan{Candidates: slices.Clone(agyChain), Triggers: llmroute.DefaultTriggers(), Model: tier, Tiers: []string{tier}}
}

func agyTierGroups(t *testing.T) map[string]string {
	t.Helper()
	m, err := bridge.LoadManifest("agy-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(agy-tmux): %v", err)
	}
	groups := make(map[string]string, 4)
	for _, tier := range []string{"fast", "balanced", "deep", "top"} {
		groups[tier] = clihealth.ModelGroup(m.ModelTierMap[tier])
	}
	if groups["deep"] == "" || groups["deep"] == groups["balanced"] || groups["deep"] == groups["fast"] {
		t.Fatalf("agy tier map %v puts deep in the same model group as a Flash seat; a scoped bench cannot be told apart from a family bench", m.ModelTierMap)
	}
	return groups
}

func expectedChainsForWalledGroup(groups map[string]string, walled string) map[string][]string {
	walkedOn := []string{"agy-claude-tmux", "claude-tmux", "agy-tmux"}
	want := make(map[string][]string, len(groups))
	for tier, group := range groups {
		want[tier] = agyChain
		if group == walled {
			want[tier] = walkedOn
		}
	}
	return want
}

func chainFor(t *testing.T, root, tier string) []string {
	t.Helper()
	return bridgechain.ApplyCLIHealthBench(root, "build", seatPlan(tier), map[string]string{}, fixedNow, nil).Candidates
}

func TestC1911_001_AgyCompactResetKeepsTheWallLineAndCapsAt24h(t *testing.T) {
	pane := unrelatedPaneTop + "\n" + agyArgonWallLine + "\n? for shortcuts"

	entry := clihealth.NewBenchEntry(clihealth.Entry{}, "agy", clihealth.QuotaExhaustedPattern, pane, fixedNow())

	if entry.Evidence != agyArgonWallLine {
		t.Errorf("Evidence = %q, want the 429 line %q, not the pane's first line", entry.Evidence, agyArgonWallLine)
	}
	if want := fixedNow().Add(24 * time.Hour); !entry.BenchedUntil.Equal(want) {
		t.Errorf("BenchedUntil = %s, want %s: the 148h reset hint is capped at resetHintCap, not left to the 30m strike cooldown", entry.BenchedUntil, want)
	}
}

func TestC1911_002_AgyCompactResetParsesShortAndRejectsGarbage(t *testing.T) {
	cases := []struct {
		name       string
		pane       string
		wantParsed bool
		min, max   time.Duration
	}{
		{"hours-minutes-seconds", "Individual quota reached. Resets in 1h30m0s.", true, 90 * time.Minute, 95 * time.Minute},
		{"minutes-only", "Individual quota reached. Resets in 45m.", true, 45 * time.Minute, 50 * time.Minute},
		{"seconds-only", "Individual quota reached. Resets in 30s.", true, 30 * time.Second, 5*time.Minute + 30*time.Second},
		{"no-duration", "Individual quota reached. Resets in soon.", false, 0, 0},
		{"zero-duration", "Individual quota reached. Resets in 0s.", false, 0, 0},
		{"no-hint", "Individual quota reached.", false, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			at, parsed := clihealth.ParseResetHint(tc.pane, fixedNow())
			if parsed != tc.wantParsed {
				t.Fatalf("ParseResetHint(%q) parsed = %v (at %s), want %v", tc.pane, parsed, at, tc.wantParsed)
			}
			if !parsed {
				return
			}
			if d := at.Sub(fixedNow()); d < tc.min || d > tc.max {
				t.Errorf("ParseResetHint(%q) = now+%s, want within [%s, %s]", tc.pane, d, tc.min, tc.max)
			}
		})
	}
}

func TestC1911_003_ArgonScopedBenchLeavesBalancedFlashDispatchable(t *testing.T) {
	root := t.TempDir()
	groups := agyTierGroups(t)
	deepGroup := groups["deep"]
	store := clihealth.NewStore(root, fixedNow)
	if err := store.Bench(clihealth.Entry{
		Family: "agy", Scope: deepGroup, Reason: clihealth.QuotaExhaustedPattern,
		BenchedAt: fixedNow(), BenchedUntil: fixedNow().Add(24 * time.Hour), Evidence: agyArgonWallLine, Strikes: 1,
	}); err != nil {
		t.Fatal(err)
	}

	want := expectedChainsForWalledGroup(groups, deepGroup)
	if slices.Equal(want["deep"], agyChain) || !slices.Equal(want["balanced"], agyChain) || !slices.Equal(want["fast"], agyChain) {
		t.Fatalf("expected chains %v must walk the deep seat and keep the fast and balanced seats on agy-tmux", want)
	}
	for _, tier := range []string{"fast", "balanced", "deep", "top"} {
		if got := chainFor(t, root, tier); !slices.Equal(got, want[tier]) {
			t.Errorf("%s seat (%s) with a bench scoped to %q: chain = %v, want %v", tier, groups[tier], deepGroup, got, want[tier])
		}
	}
}

func TestC1911_004_FamilyWideEntryStillBenchesEveryAgyTier(t *testing.T) {
	root := t.TempDir()
	legacy := fmt.Sprintf(`{"schema_version":1,"benches":{"agy":{"family":"agy","reason":"quota_exhausted","benched_at":%q,"benched_until":%q,"evidence":"RESOURCE_EXHAUSTED","strikes":1}}}`,
		fixedNow().Format(time.RFC3339), fixedNow().Add(30*time.Minute).Format(time.RFC3339))
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "cli-health.json"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	active := clihealth.NewStore(root, fixedNow).Active()
	if len(active) != 1 {
		t.Fatalf("legacy family-wide file: Active = %v, want one entry", active)
	}
	for _, e := range active {
		if e.Family != "agy" || e.Scope != "" || e.Reason != clihealth.QuotaExhaustedPattern {
			t.Errorf("legacy entry read as %+v, want family agy with no scope", e)
		}
	}
	for _, tier := range []string{"balanced", "deep"} {
		if got, want := chainFor(t, root, tier), []string{"agy-claude-tmux", "claude-tmux", "agy-tmux"}; !slices.Equal(got, want) {
			t.Errorf("%s seat with a family-wide agy bench: chain = %v, want %v", tier, got, want)
		}
	}
}

func TestC1911_005_ScopedAndFamilyEntriesCoexistInTheStore(t *testing.T) {
	root := t.TempDir()
	store := clihealth.NewStore(root, fixedNow)
	scoped := clihealth.Entry{Family: "agy", Scope: argonGroup, Reason: clihealth.QuotaExhaustedPattern,
		BenchedAt: fixedNow(), BenchedUntil: fixedNow().Add(24 * time.Hour), Strikes: 1}
	familyWide := clihealth.Entry{Family: "agy", Reason: "rate_limit",
		BenchedAt: fixedNow(), BenchedUntil: fixedNow().Add(30 * time.Minute), Strikes: 1}
	for _, e := range []clihealth.Entry{scoped, familyWide} {
		if err := store.Bench(e); err != nil {
			t.Fatal(err)
		}
	}

	benches, err := clihealth.NewStore(root, fixedNow).Load()
	if err != nil {
		t.Fatal(err)
	}
	var sawScoped, sawFamily bool
	for _, e := range benches {
		switch {
		case e.Family == "agy" && e.Scope == argonGroup && e.Reason == clihealth.QuotaExhaustedPattern:
			sawScoped = true
		case e.Family == "agy" && e.Scope == "" && e.Reason == "rate_limit":
			sawFamily = true
		}
	}
	if !sawScoped || !sawFamily || len(benches) != 2 {
		t.Errorf("store after a scoped and a family-wide agy bench = %v, want both entries kept (scoped=%v family=%v)", benches, sawScoped, sawFamily)
	}
}

type argonWalledBridge struct {
	calls []string
}

func (b *argonWalledBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.calls = append(b.calls, req.CLI+"@"+req.Model)
	if req.CLI != "agy-tmux" {
		return core.BridgeResponse{ExitCode: 0}, nil
	}
	report, err := json.Marshal(map[string]any{"captured_at": fixedNow(), "cli": req.CLI, "pattern_name": clihealth.QuotaExhaustedPattern,
		"pane_tail": unrelatedPaneTop + "\n" + agyArgonWallLine})
	if err != nil {
		return core.BridgeResponse{}, err
	}
	if err := os.WriteFile(filepath.Join(req.Workspace, "escalation-report.json"), report, 0o644); err != nil {
		return core.BridgeResponse{}, err
	}
	return core.BridgeResponse{ExitCode: 85}, fmt.Errorf("bridge: exit 85")
}

func (b *argonWalledBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{Version: "argon-walled"}, nil
}

func TestC1911_006_DeepArgonWallThroughTheWalkBenchesOnlyTheArgonGroup(t *testing.T) {
	root, ws := t.TempDir(), t.TempDir()
	inner := &argonWalledBridge{}
	resolve := func(core.BridgeRequest) (llmroute.Plan, error) {
		return llmroute.Plan{Candidates: []string{"agy-tmux", "claude-tmux"}, Triggers: llmroute.DefaultTriggers(), Model: "deep", Tiers: []string{"deep"}}, nil
	}
	walk := bridgechain.New(inner, resolve, bridgechain.WithClock(fixedNow),
		bridgechain.WithBench(func(e bridgechain.Escalation) { bridgechain.BenchOnEscalation(e, fixedNow, nil) }))

	resp, err := walk.Launch(context.Background(), core.BridgeRequest{CLI: "agy-tmux", Model: "deep", Agent: "auditor",
		ProjectRoot: root, Workspace: ws, Env: map[string]string{}})
	if err != nil || resp.ExitCode != 0 {
		t.Fatalf("walk must end on claude-tmux after the Argon wall: resp=%+v err=%v calls=%v", resp, err, inner.calls)
	}

	active := clihealth.NewStore(root, fixedNow).Active()
	if len(active) != 1 {
		t.Fatalf("after one Argon wall: Active = %v, want exactly one bench", active)
	}
	groups := agyTierGroups(t)
	for _, e := range active {
		if e.Family != "agy" || e.Scope != groups["deep"] {
			t.Errorf("Argon wall wrote %+v, want an agy entry scoped to the deep model group %q, not the whole family", e, groups["deep"])
		}
		if e.Evidence != agyArgonWallLine {
			t.Errorf("bench evidence = %q, want %q", e.Evidence, agyArgonWallLine)
		}
	}
	for tier, want := range expectedChainsForWalledGroup(groups, groups["deep"]) {
		if got := chainFor(t, root, tier); !slices.Equal(got, want) {
			t.Errorf("%s seat after a deep Argon wall: chain = %v, want %v", tier, got, want)
		}
	}
}
