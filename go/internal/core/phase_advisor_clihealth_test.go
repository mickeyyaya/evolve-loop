package core

// Slice-4 contract: the routing advisor SEES the environment (cycle-283 — the
// advisor kept planning codex-routed inserts all night while codex was
// quota-walled, because RouteInput carried zero CLI state). An active bench
// renders as a deterministic "CLI health" section in the routing context;
// no benches → no section.

import (
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/clihealth"
	"github.com/mickeyyaya/evolveloop/go/internal/router"
)

func TestWriteRoutingContext_CLIHealthSection(t *testing.T) {
	t.Parallel()
	until := time.Date(2026, 6, 11, 6, 13, 0, 0, time.UTC)
	var b strings.Builder
	writeRoutingContext(&b, router.RouteInput{
		BenchedCLIs: []router.BenchedCLI{
			{Family: "codex", Reason: "rate_limit", Until: until},
		},
	})
	out := b.String()
	if !strings.Contains(out, "## CLI health (environmental)") {
		t.Fatalf("routing context missing CLI-health section:\n%s", out)
	}
	for _, want := range []string{"codex", "rate_limit", "06:13Z", "fallback"} {
		if !strings.Contains(out, want) {
			t.Errorf("CLI-health section missing %q", want)
		}
	}
}

func TestWriteRoutingContext_NoBenchesNoSection(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	writeRoutingContext(&b, router.RouteInput{})
	if strings.Contains(b.String(), "CLI health") {
		t.Error("empty BenchedCLIs must not render a CLI-health section (prompt stays stable)")
	}
}

func TestBenchedCLIsForRouting_ProjectsActiveSorted(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Now()
	st := clihealth.NewStore(root, nil)
	_ = st.Bench(clihealth.Entry{Family: "codex", Reason: "rate_limit",
		BenchedAt: now, BenchedUntil: now.Add(time.Hour)})
	_ = st.Bench(clihealth.Entry{Family: "agy", Reason: "rate_limit",
		BenchedAt: now, BenchedUntil: now.Add(time.Hour)})
	_ = st.Bench(clihealth.Entry{Family: "claude", Reason: "rate_limit",
		BenchedAt: now.Add(-2 * time.Hour), BenchedUntil: now.Add(-time.Hour)}) // expired — excluded

	got := benchedCLIsForRouting(root)
	if len(got) != 2 || got[0].Family != "agy" || got[1].Family != "codex" {
		t.Errorf("got %+v, want [agy codex] (active only, family-sorted)", got)
	}
	if benchedCLIsForRouting(t.TempDir()) != nil {
		t.Error("empty store must project nil (no prompt section)")
	}
}
