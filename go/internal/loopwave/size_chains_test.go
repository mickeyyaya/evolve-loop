package loopwave

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func benchFamily(t *testing.T, h *harness, family string) {
	t.Helper()
	now := time.Now()
	if err := clihealth.NewStore(h.root, nil).Bench(clihealth.Entry{Family: family, Reason: "rate_limit", BenchedAt: now, BenchedUntil: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
}

func writePolicy(t *testing.T, h *harness, body string) {
	t.Helper()
	if err := os.WriteFile(paths.PolicyPath(h.evolveDir), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSize_ABenchWithAFallbackInTheCallersRoutingKeepsTheLaneAndReadsNoPolicy(t *testing.T) {
	h := newHarness(t)
	benchFamily(t, h, "agy-claude")
	writePolicy(t, h, "{not json")
	routing := &policy.CLIRouting{CLIs: []string{"agy-claude", "claude"}}
	var seen *policy.CLIRouting
	var chains ChainsFn = func(block *policy.CLIRouting) [][]string {
		seen = block
		return [][]string{{"agy-claude", "claude"}}
	}
	h.ports.Chains = chains
	e := New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, h.stderr)

	got, _ := e.Size(policy.FleetConfig{Count: 3, MinLanes: 2}, routing, nil, budgethistory.Throughput{}, time.Now())

	if got.Count != 3 {
		t.Fatalf("count = %d, want 3: claude follows agy-claude, so the bench costs no lane", got.Count)
	}
	if seen != routing {
		t.Errorf("the chains port received %p, want the caller's routing block %p: Size must not load policy.json again", seen, routing)
	}
	if strings.Contains(h.stderr.String(), "unknown") {
		t.Errorf("stderr = %q: the unreadable policy.json must not be touched by Size", h.stderr.String())
	}
	if !strings.Contains(h.stderr.String(), `INFO: fleet: quota bench on CLI family "agy-claude" (rate_limit) costs no lane: a lane walks to "claude"`) {
		t.Errorf("stderr = %q, want the INFO line that names the fallback", h.stderr.String())
	}
}

func TestSize_WithoutAChainsPortEveryBenchCostsALane(t *testing.T) {
	h := newHarness(t)
	benchFamily(t, h, "agy-claude")
	e := New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, h.stderr)

	got, _ := e.Size(policy.FleetConfig{Count: 3, MinLanes: 2}, &policy.CLIRouting{}, nil, budgethistory.Throughput{}, time.Now())

	if got.Count != 2 {
		t.Fatalf("count = %d, want 2: unknown chains keep the old shrink", got.Count)
	}
}

func TestLoadRouting_ReturnsTheDeclaredBlock(t *testing.T) {
	h := newHarness(t)
	writePolicy(t, h, `{"cli_routing":{"clis":["agy-claude","claude"]}}`)

	got := LoadRouting(h.evolveDir, h.stderr)

	if got == nil || len(got.CLIs) != 2 {
		t.Fatalf("LoadRouting = %+v, want the cli_routing block", got)
	}
	if h.stderr.Len() != 0 {
		t.Errorf("stderr = %q, want silence", h.stderr.String())
	}
}

func TestLoadRouting_AnUnreadablePolicyWarnsAndYieldsNoBlock(t *testing.T) {
	h := newHarness(t)
	writePolicy(t, h, "{not json")

	got := LoadRouting(h.evolveDir, h.stderr)

	if got != nil {
		t.Errorf("LoadRouting = %+v, want nil", got)
	}
	if !strings.Contains(h.stderr.String(), "WARN: fleet: the routing chains are unknown (policy: parse") {
		t.Errorf("stderr = %q, want the WARN line for the unreadable policy", h.stderr.String())
	}
}

func TestQuotaCount_NamesTheFamiliesWhoseBenchCostsALaneAndSizeCountsTheSame(t *testing.T) {
	h := newHarness(t)
	benchFamily(t, h, "claude")
	benchFamily(t, h, "agy-claude")
	h.ports.Chains = func(*policy.CLIRouting) [][]string { return [][]string{{"agy-claude", "claude"}, {"agy-claude"}} }
	e := New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, h.stderr)
	fc := policy.FleetConfig{Count: 4, MinLanes: 2}

	count, paused := e.QuotaCount(fc, &policy.CLIRouting{})

	if count != 2 || strings.Join(paused, ",") != "agy-claude,claude" {
		t.Errorf("QuotaCount = %d, %v, want 2 and both families: each chain ends in a benched family", count, paused)
	}
	if sized, _ := e.Size(fc, &policy.CLIRouting{}, nil, budgethistory.Throughput{}, time.Now()); sized.Count != count {
		t.Errorf("Size count = %d, want the QuotaCount %d", sized.Count, count)
	}
}

func TestQuotaCount_WithNoBenchKeepsTheWidthAndPausesNothing(t *testing.T) {
	h := newHarness(t)
	e := New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, h.stderr)

	count, paused := e.QuotaCount(policy.FleetConfig{Count: 4, MinLanes: 2}, nil)

	if count != 4 || len(paused) != 0 {
		t.Errorf("QuotaCount = %d, %v, want 4 and nothing paused", count, paused)
	}
}
